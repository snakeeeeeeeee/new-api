package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWebhookDelayTracksEndpointConcurrency(t *testing.T) {
	state := newServerState()
	server := httptest.NewServer(state.handler())
	defer server.Close()

	const requests = 6
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := 0; i < requests; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			request, err := http.NewRequest(http.MethodPost, server.URL+"/webhook/delay/a?delay_ms=80", bytes.NewBufferString(`{}`))
			if err != nil {
				t.Error(err)
				return
			}
			request.Header.Set("Authorization", "Bearer test")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Error(err)
				return
			}
			_ = response.Body.Close()
		}()
	}
	close(start)
	group.Wait()

	metrics := state.snapshot()
	if metrics.Peak.Webhook != requests {
		t.Fatalf("peak webhook concurrency = %d, want %d", metrics.Peak.Webhook, requests)
	}
	if metrics.Peak.ByEndpoint["webhook:/webhook/delay/a"] != requests {
		t.Fatalf("endpoint peak = %d, want %d", metrics.Peak.ByEndpoint["webhook:/webhook/delay/a"], requests)
	}
	if metrics.Requests.AuthorizationPresent != requests {
		t.Fatalf("authorization count = %d, want %d", metrics.Requests.AuthorizationPresent, requests)
	}
}

func TestImageControlAndMetadataOverrides(t *testing.T) {
	state := newServerState()
	status := http.StatusServiceUnavailable
	delay := 1
	if _, err := state.updateConfig(controlUpdate{ImageStatus: &status, ImageDelayMS: &delay}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(state.handler())
	defer server.Close()

	requestBody := map[string]any{
		"client_task_id": "task_mock",
		"metadata":       map[string]any{"async_test_status": http.StatusAccepted, "async_test_delay_ms": 0},
	}
	body, _ := json.Marshal(requestBody)
	response, err := http.Post(server.URL+"/v1/image/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["client_task_id"] != "task_mock" {
		t.Fatalf("client_task_id = %v", result["client_task_id"])
	}
}

func TestResetPreservesActiveRequestsAsPeakFloor(t *testing.T) {
	state := newServerState()
	finish := state.begin("webhook", "webhook:/active", false)
	metrics := state.resetMetrics()
	if metrics.Peak.Total != 1 || metrics.Current.Total != 1 || metrics.Requests.Total != 0 {
		t.Fatalf("unexpected reset snapshot: %+v", metrics)
	}
	finish(true)
	if state.snapshot().Current.Total != 0 {
		t.Fatal("active request did not finish")
	}
}

func TestControlRejectsInvalidDelay(t *testing.T) {
	state := newServerState()
	delay := int((5*time.Minute)/time.Millisecond) + 1
	if _, err := state.updateConfig(controlUpdate{WebhookDelayMS: &delay}); err == nil {
		t.Fatal("expected invalid delay error")
	}
}

func TestVideoLifecycleAndRangeContent(t *testing.T) {
	state := newServerState()
	server := httptest.NewServer(state.handler())
	defer server.Close()

	audio := false
	body, err := json.Marshal(videoTaskRequest{
		Model: "seedance_2.0_fast_480p", Prompt: "ocean sunrise",
		Duration: 4, AspectRatio: "16:9", GenerateAudio: &audio,
		ReferenceMode: "media",
		ReferenceImages: []videoReferenceMedia{
			{URL: "https://media.example.com/character.png", Name: "character"},
		},
		ReferenceVideos: []videoReferenceMedia{
			{URL: "https://media.example.com/motion.mp4", Name: "motion"},
		},
		ReferenceAudios: []videoReferenceMedia{
			{URL: "https://media.example.com/music.mp3", Name: "music"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	submitRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	submitRequest.Header.Set("Authorization", "Bearer mock-key")
	submitResponse, err := http.DefaultClient.Do(submitRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer submitResponse.Body.Close()
	if submitResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d", submitResponse.StatusCode)
	}
	var submitted map[string]any
	if err := json.NewDecoder(submitResponse.Body).Decode(&submitted); err != nil {
		t.Fatal(err)
	}
	taskID, _ := submitted["task_id"].(string)
	if taskID == "" {
		t.Fatal("submit response omitted task_id")
	}

	for index, expected := range []string{"queued", "in_progress", "completed"} {
		request, err := http.NewRequest(http.MethodGet, server.URL+"/v1/videos/"+taskID, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer mock-key")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var task map[string]any
		if err := json.NewDecoder(response.Body).Decode(&task); err != nil {
			response.Body.Close()
			t.Fatal(err)
		}
		response.Body.Close()
		if task["status"] != expected {
			t.Fatalf("poll %d status = %v, want %s", index+1, task["status"], expected)
		}
	}

	contentRequest, err := http.NewRequest(http.MethodGet, server.URL+"/v1/videos/"+taskID+"/content", nil)
	if err != nil {
		t.Fatal(err)
	}
	contentRequest.Header.Set("Authorization", "Bearer mock-key")
	contentRequest.Header.Set("Range", "bytes=0-3")
	contentResponse, err := http.DefaultClient.Do(contentRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer contentResponse.Body.Close()
	content, err := io.ReadAll(contentResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if contentResponse.StatusCode != http.StatusPartialContent || string(content) != "mock" {
		t.Fatalf("unexpected ranged content: status=%d body=%q", contentResponse.StatusCode, content)
	}

	metrics := state.snapshot()
	if metrics.VideoRequests.Submit != 1 || metrics.VideoRequests.Poll != 3 || metrics.VideoRequests.Content != 1 {
		t.Fatalf("unexpected video request metrics: %+v", metrics.VideoRequests)
	}
	if metrics.LastVideoSubmit == nil || metrics.LastVideoSubmit.Model != "seedance_2.0_fast_480p" ||
		metrics.LastVideoSubmit.Duration != 4 || metrics.LastVideoSubmit.GenerateAudio == nil ||
		*metrics.LastVideoSubmit.GenerateAudio || metrics.LastVideoSubmit.ReferenceMode != "media" ||
		len(metrics.LastVideoSubmit.ReferenceImages) != 1 ||
		len(metrics.LastVideoSubmit.ReferenceVideos) != 1 ||
		len(metrics.LastVideoSubmit.ReferenceAudios) != 1 ||
		metrics.LastVideoSubmit.ReferenceImages[0].Name != "character" ||
		metrics.LastVideoSubmit.ReferenceVideos[0].URL != "https://media.example.com/motion.mp4" ||
		metrics.LastVideoSubmit.ReferenceAudios[0].Name != "music" {
		t.Fatalf("unexpected last video submit: %+v", metrics.LastVideoSubmit)
	}
}

func TestVideoSubmitRejectsInvalidReferenceContract(t *testing.T) {
	state := newServerState()
	server := httptest.NewServer(state.handler())
	defer server.Close()

	tests := []string{
		`{
			"model":"seedance_2.0_fast_480p",
			"prompt":"too many frames",
			"duration":4,
			"aspect_ratio":"16:9",
			"reference_mode":"frame",
			"reference_images":[
				{"url":"https://example.com/1.png"},
				{"url":"https://example.com/2.png"},
				{"url":"https://example.com/3.png"}
			]
		}`,
		`{
			"model":"seedance_2.0_fast_480p",
			"prompt":"frame cannot include video",
			"duration":4,
			"aspect_ratio":"16:9",
			"reference_mode":"frame",
			"reference_videos":[{"url":"https://example.com/clip.mp4"}]
		}`,
		`{
			"model":"seedance_2.0_fast_480p",
			"prompt":"reject non URL data",
			"duration":4,
			"aspect_ratio":"16:9",
			"reference_mode":"media",
			"reference_audios":[{"url":"data:audio/mpeg;base64,YXVkaW8="}]
		}`,
		`{
			"model":"seedance_2.0_fast_480p",
			"prompt":"names must be unique",
			"duration":4,
			"aspect_ratio":"16:9",
			"reference_mode":"media",
			"reference_images":[{"url":"https://example.com/image.png","name":"subject"}],
			"reference_videos":[{"url":"https://example.com/clip.mp4","name":"subject"}]
		}`,
	}
	for _, body := range tests {
		response, err := http.Post(
			server.URL+"/v1/videos",
			"application/json",
			bytes.NewBufferString(body),
		)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusBadRequest {
			response.Body.Close()
			t.Fatalf("status = %d, want %d for %s", response.StatusCode, http.StatusBadRequest, body)
		}
		response.Body.Close()
	}
}

func TestVideoFailureControl(t *testing.T) {
	state := newServerState()
	terminal := "failed"
	after := 1
	if _, err := state.updateConfig(controlUpdate{
		VideoTerminalStatus:    &terminal,
		VideoTerminalAfterPoll: &after,
	}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(state.handler())
	defer server.Close()

	body := bytes.NewBufferString(`{"model":"seedance_2.0_fast_480p","prompt":"fail","duration":4,"aspect_ratio":"16:9"}`)
	response, err := http.Post(server.URL+"/v1/videos", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	var submitted map[string]any
	if err := json.NewDecoder(response.Body).Decode(&submitted); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	taskID, _ := submitted["task_id"].(string)
	response, err = http.Get(server.URL + "/v1/videos/" + taskID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var task map[string]any
	if err := json.NewDecoder(response.Body).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task["status"] != "failed" {
		t.Fatalf("status = %v, want failed", task["status"])
	}
}

func TestLeonardoVideoPayloadContractAndIdempotency(t *testing.T) {
	state := newServerState()
	server := httptest.NewServer(state.handler())
	defer server.Close()

	audio := false
	public := false
	seed := -1
	payload := videoTaskRequest{
		Model: "seedance-2.0-fast-480p", Prompt: "a cinematic cat", Duration: 4,
		AspectRatio: "16:9", GenerateAudio: &audio, Public: &public, Seed: &seed,
		ImageReferences: []videoReferenceMedia{
			{URL: "https://media.example.com/1.png"},
			{URL: "https://media.example.com/2.png"},
			{URL: "https://media.example.com/3.png"},
			{URL: "https://media.example.com/4.png"},
		},
		VideoReferences: []videoReferenceMedia{
			{URL: "https://media.example.com/1.mp4"},
			{URL: "https://media.example.com/2.mp4"},
			{URL: "https://media.example.com/3.mp4"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	post := func() (map[string]any, int) {
		request, requestErr := http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Authorization", "Bearer mock-key")
		request.Header.Set("Idempotency-Key", "public-task-1")
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		defer response.Body.Close()
		var result map[string]any
		if decodeErr := json.NewDecoder(response.Body).Decode(&result); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return result, response.StatusCode
	}
	first, status := post()
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", status, http.StatusAccepted)
	}
	firstID, _ := first["task_id"].(string)
	if firstID == "" {
		t.Fatal("first response omitted task_id")
	}
	second, status := post()
	if status != http.StatusAccepted || second["task_id"] != firstID {
		t.Fatalf("idempotent replay = status %d task %v, want %d task %s", status, second["task_id"], http.StatusAccepted, firstID)
	}

	metrics := state.snapshot()
	if metrics.VideoRequests.Submit != 1 || metrics.VideoRequests.SubmitAttempts != 2 {
		t.Fatalf("unexpected submit metrics: %+v", metrics.VideoRequests)
	}
	if metrics.LastVideoSubmit == nil || metrics.LastVideoSubmit.Public == nil || *metrics.LastVideoSubmit.Public ||
		metrics.LastVideoSubmit.Seed == nil || *metrics.LastVideoSubmit.Seed != -1 ||
		len(metrics.LastVideoSubmit.ImageReferences) != 4 || len(metrics.LastVideoSubmit.VideoReferences) != 3 ||
		metrics.LastVideoSubmit.ReferenceMode != "" || len(metrics.LastVideoSubmit.ReferenceImages) != 0 ||
		len(metrics.LastVideoSubmit.ReferenceVideos) != 0 || len(metrics.LastVideoSubmit.ReferenceAudios) != 0 {
		t.Fatalf("unexpected Leonardo payload: %+v", metrics.LastVideoSubmit)
	}
}

func TestLeonardoMiniMaxH3PayloadContract(t *testing.T) {
	state := newServerState()
	server := httptest.NewServer(state.handler())
	defer server.Close()

	audio := true
	public := false
	payload := videoTaskRequest{
		Model: "minimax-h3-1440p", Prompt: "transition between frames", Duration: 5,
		AspectRatio: "16:9", GenerateAudio: &audio, Public: &public, ReferenceMode: "frame",
		ImageReferences: []videoReferenceMedia{
			{URL: "https://media.example.com/start.png"},
			{URL: "http://media.example.com/end.png"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer mock-key")
	request.Header.Set("Idempotency-Key", "h3-frame-1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	metrics := state.snapshot()
	if metrics.LastVideoSubmit == nil || metrics.LastVideoSubmit.Seed != nil ||
		metrics.LastVideoSubmit.ReferenceMode != "frame" ||
		len(metrics.LastVideoSubmit.ImageReferences) != 2 {
		t.Fatalf("unexpected H3 payload: %+v", metrics.LastVideoSubmit)
	}

	payload.ImageReferences = nil
	body, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err = http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty frame status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}

func TestLeonardoMockSupportsModerationFailure(t *testing.T) {
	state := newServerState()
	terminal := "failed"
	after := 1
	code := "content_moderated"
	message := "The generation was rejected by content moderation."
	if _, err := state.updateConfig(controlUpdate{
		VideoTerminalStatus:    &terminal,
		VideoTerminalAfterPoll: &after,
		VideoTerminalErrorCode: &code,
		VideoTerminalErrorMsg:  &message,
	}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(state.handler())
	defer server.Close()

	body := `{"model":"seedance-2.0-fast-480p","prompt":"moderated","duration":4,"aspect_ratio":"16:9","public":false,"seed":-1}`
	response, err := http.Post(server.URL+"/v1/videos", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var submitted map[string]any
	if err := json.NewDecoder(response.Body).Decode(&submitted); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	taskID, _ := submitted["task_id"].(string)
	response, err = http.Get(server.URL + "/v1/videos/" + taskID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var task map[string]any
	if err := json.NewDecoder(response.Body).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task["status"] != "failed" {
		t.Fatalf("status = %v, want failed", task["status"])
	}
	errorObject, ok := task["error"].(map[string]any)
	if !ok || errorObject["code"] != code || errorObject["message"] != message {
		t.Fatalf("unexpected moderation error: %v", task["error"])
	}
}

func TestLeonardoMockDisconnectsFirstResponseWithoutCreatingSecondJob(t *testing.T) {
	state := newServerState()
	disconnect := true
	if _, err := state.updateConfig(controlUpdate{VideoDisconnectFirst: &disconnect}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(state.handler())
	defer server.Close()

	body := []byte(`{"model":"seedance-2.0-fast-480p","prompt":"retry transport","duration":4,"aspect_ratio":"16:9","public":false,"seed":-1}`)
	firstRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	firstRequest.Header.Set("Idempotency-Key", "transport-task-1")
	if response, requestErr := http.DefaultClient.Do(firstRequest); requestErr == nil {
		response.Body.Close()
		t.Fatal("first response unexpectedly succeeded")
	}

	secondRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	secondRequest.Header.Set("Idempotency-Key", "transport-task-1")
	response, err := http.DefaultClient.Do(secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("second status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	metrics := state.snapshot()
	if metrics.VideoRequests.Submit != 1 || metrics.VideoRequests.SubmitAttempts != 2 || len(state.videoJobs) != 1 {
		t.Fatalf("transport replay created duplicate job: metrics=%+v jobs=%d", metrics.VideoRequests, len(state.videoJobs))
	}
}
