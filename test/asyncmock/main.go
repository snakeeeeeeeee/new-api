package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultPort = "8080"
	maxDelayMS  = 300_000
	maxBodySize = 2 << 20
)

type mockConfig struct {
	ImageStatus            int    `json:"image_status"`
	ImageDelayMS           int    `json:"image_delay_ms"`
	WebhookStatus          int    `json:"webhook_status"`
	WebhookDelayMS         int    `json:"webhook_delay_ms"`
	VideoSubmitStatus      int    `json:"video_submit_status"`
	VideoTerminalStatus    string `json:"video_terminal_status"`
	VideoTerminalAfterPoll int    `json:"video_terminal_after_poll"`
	VideoTerminalErrorCode string `json:"video_terminal_error_code"`
	VideoTerminalErrorMsg  string `json:"video_terminal_error_message"`
	VideoDisconnectFirst   bool   `json:"video_disconnect_first_response"`
}

type controlUpdate struct {
	ImageStatus            *int    `json:"image_status"`
	ImageDelayMS           *int    `json:"image_delay_ms"`
	WebhookStatus          *int    `json:"webhook_status"`
	WebhookDelayMS         *int    `json:"webhook_delay_ms"`
	VideoSubmitStatus      *int    `json:"video_submit_status"`
	VideoTerminalStatus    *string `json:"video_terminal_status"`
	VideoTerminalAfterPoll *int    `json:"video_terminal_after_poll"`
	VideoTerminalErrorCode *string `json:"video_terminal_error_code"`
	VideoTerminalErrorMsg  *string `json:"video_terminal_error_message"`
	VideoDisconnectFirst   *bool   `json:"video_disconnect_first_response"`
}

type requestCounts struct {
	Total                int64 `json:"total"`
	Succeeded            int64 `json:"succeeded"`
	Failed               int64 `json:"failed"`
	AuthorizationPresent int64 `json:"authorization_present"`
}

type concurrencyCounts struct {
	Total      int            `json:"total"`
	Image      int            `json:"image"`
	Video      int            `json:"video"`
	Webhook    int            `json:"webhook"`
	ByEndpoint map[string]int `json:"by_endpoint"`
}

type videoTaskRequest struct {
	Model           string                `json:"model"`
	Prompt          string                `json:"prompt"`
	Duration        int                   `json:"duration"`
	AspectRatio     string                `json:"aspect_ratio"`
	GenerateAudio   *bool                 `json:"generate_audio,omitempty"`
	Public          *bool                 `json:"public,omitempty"`
	Seed            *int                  `json:"seed,omitempty"`
	ReferenceMode   string                `json:"reference_mode,omitempty"`
	ReferenceImages []videoReferenceMedia `json:"reference_images,omitempty"`
	ReferenceVideos []videoReferenceMedia `json:"reference_videos,omitempty"`
	ReferenceAudios []videoReferenceMedia `json:"reference_audios,omitempty"`
	ImageReferences []videoReferenceMedia `json:"image_references,omitempty"`
	VideoReferences []videoReferenceMedia `json:"video_references,omitempty"`
	AudioReferences []videoReferenceMedia `json:"audio_references,omitempty"`
}

type videoReferenceMedia struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

type videoRequestCounts struct {
	Submit         int64 `json:"submit"`
	SubmitAttempts int64 `json:"submit_attempts"`
	Poll           int64 `json:"poll"`
	Content        int64 `json:"content"`
}

type metricsResponse struct {
	StartedAt       int64              `json:"started_at"`
	Current         concurrencyCounts  `json:"current_in_flight"`
	Peak            concurrencyCounts  `json:"peak_in_flight"`
	Requests        requestCounts      `json:"requests"`
	VideoRequests   videoRequestCounts `json:"video_requests"`
	LastVideoSubmit *videoTaskRequest  `json:"last_video_submit,omitempty"`
	Config          mockConfig         `json:"config"`
}

type videoJob struct {
	Request videoTaskRequest
	Polls   int
}

type serverState struct {
	mu              sync.Mutex
	startedAt       int64
	current         concurrencyCounts
	peak            concurrencyCounts
	requests        requestCounts
	videoRequests   videoRequestCounts
	lastVideoSubmit *videoTaskRequest
	videoJobs       map[string]*videoJob
	idempotencyJobs map[string]string
	disconnectSeen  map[string]bool
	config          mockConfig
	sequence        atomic.Uint64
}

func newServerState() *serverState {
	return &serverState{
		startedAt:       time.Now().Unix(),
		current:         concurrencyCounts{ByEndpoint: make(map[string]int)},
		peak:            concurrencyCounts{ByEndpoint: make(map[string]int)},
		videoJobs:       make(map[string]*videoJob),
		idempotencyJobs: make(map[string]string),
		disconnectSeen:  make(map[string]bool),
		config: mockConfig{
			ImageStatus:            http.StatusAccepted,
			WebhookStatus:          http.StatusNoContent,
			VideoSubmitStatus:      http.StatusAccepted,
			VideoTerminalStatus:    "completed",
			VideoTerminalAfterPoll: 3,
			VideoTerminalErrorCode: "mock_video_failed",
			VideoTerminalErrorMsg:  "async-test mock forced video failure",
		},
	}
}

func (s *serverState) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/reset", s.handleReset)
	mux.HandleFunc("/control", s.handleControl)
	mux.HandleFunc("/v1/image/tasks", s.handleImageTask)
	mux.HandleFunc("/v1/videos", s.handleVideoSubmit)
	mux.HandleFunc("/v1/videos/", s.handleVideoTask)
	mux.HandleFunc("/webhook/", s.handleWebhook)
	return mux
}

func (s *serverState) begin(kind, endpoint string, hasAuthorization bool) func(bool) {
	s.mu.Lock()
	s.current.Total++
	s.current.ByEndpoint[endpoint]++
	switch kind {
	case "image":
		s.current.Image++
	case "video":
		s.current.Video++
	default:
		s.current.Webhook++
	}
	s.peak.Total = max(s.peak.Total, s.current.Total)
	s.peak.Image = max(s.peak.Image, s.current.Image)
	s.peak.Video = max(s.peak.Video, s.current.Video)
	s.peak.Webhook = max(s.peak.Webhook, s.current.Webhook)
	s.peak.ByEndpoint[endpoint] = max(s.peak.ByEndpoint[endpoint], s.current.ByEndpoint[endpoint])
	s.requests.Total++
	if hasAuthorization {
		s.requests.AuthorizationPresent++
	}
	s.mu.Unlock()

	return func(succeeded bool) {
		s.mu.Lock()
		s.current.Total--
		s.current.ByEndpoint[endpoint]--
		if s.current.ByEndpoint[endpoint] == 0 {
			delete(s.current.ByEndpoint, endpoint)
		}
		switch kind {
		case "image":
			s.current.Image--
		case "video":
			s.current.Video--
		default:
			s.current.Webhook--
		}
		if succeeded {
			s.requests.Succeeded++
		} else {
			s.requests.Failed++
		}
		s.mu.Unlock()
	}
}

func (s *serverState) snapshot() metricsResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := metricsResponse{
		StartedAt:     s.startedAt,
		Current:       cloneConcurrency(s.current),
		Peak:          cloneConcurrency(s.peak),
		Requests:      s.requests,
		VideoRequests: s.videoRequests,
		Config:        s.config,
	}
	if s.lastVideoSubmit != nil {
		last := *s.lastVideoSubmit
		result.LastVideoSubmit = &last
	}
	return result
}

func cloneConcurrency(value concurrencyCounts) concurrencyCounts {
	result := value
	result.ByEndpoint = make(map[string]int, len(value.ByEndpoint))
	for endpoint, count := range value.ByEndpoint {
		result.ByEndpoint[endpoint] = count
	}
	return result
}

func (s *serverState) resetMetrics() metricsResponse {
	s.mu.Lock()
	s.startedAt = time.Now().Unix()
	s.peak = cloneConcurrency(s.current)
	s.requests = requestCounts{}
	s.videoRequests = videoRequestCounts{}
	s.lastVideoSubmit = nil
	s.disconnectSeen = make(map[string]bool)
	s.mu.Unlock()
	return s.snapshot()
}

func (s *serverState) getConfig() mockConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *serverState) updateConfig(update controlUpdate) (mockConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.config
	if update.ImageStatus != nil {
		next.ImageStatus = *update.ImageStatus
	}
	if update.ImageDelayMS != nil {
		next.ImageDelayMS = *update.ImageDelayMS
	}
	if update.WebhookStatus != nil {
		next.WebhookStatus = *update.WebhookStatus
	}
	if update.WebhookDelayMS != nil {
		next.WebhookDelayMS = *update.WebhookDelayMS
	}
	if update.VideoSubmitStatus != nil {
		next.VideoSubmitStatus = *update.VideoSubmitStatus
	}
	if update.VideoTerminalStatus != nil {
		next.VideoTerminalStatus = strings.ToLower(strings.TrimSpace(*update.VideoTerminalStatus))
	}
	if update.VideoTerminalAfterPoll != nil {
		next.VideoTerminalAfterPoll = *update.VideoTerminalAfterPoll
	}
	if update.VideoTerminalErrorCode != nil {
		next.VideoTerminalErrorCode = strings.TrimSpace(*update.VideoTerminalErrorCode)
	}
	if update.VideoTerminalErrorMsg != nil {
		next.VideoTerminalErrorMsg = strings.TrimSpace(*update.VideoTerminalErrorMsg)
	}
	if update.VideoDisconnectFirst != nil {
		next.VideoDisconnectFirst = *update.VideoDisconnectFirst
	}
	if err := validateConfig(next); err != nil {
		return s.config, err
	}
	s.config = next
	return next, nil
}

func validateConfig(config mockConfig) error {
	if config.ImageStatus < 100 || config.ImageStatus > 599 {
		return errors.New("image_status must be between 100 and 599")
	}
	if config.WebhookStatus < 100 || config.WebhookStatus > 599 {
		return errors.New("webhook_status must be between 100 and 599")
	}
	if config.VideoSubmitStatus < 100 || config.VideoSubmitStatus > 599 {
		return errors.New("video_submit_status must be between 100 and 599")
	}
	if config.VideoTerminalStatus != "completed" && config.VideoTerminalStatus != "failed" {
		return errors.New("video_terminal_status must be completed or failed")
	}
	if config.VideoTerminalAfterPoll < 1 || config.VideoTerminalAfterPoll > 100 {
		return errors.New("video_terminal_after_poll must be between 1 and 100")
	}
	if len(config.VideoTerminalErrorCode) > 100 {
		return errors.New("video_terminal_error_code must not exceed 100 characters")
	}
	if len(config.VideoTerminalErrorMsg) > 500 {
		return errors.New("video_terminal_error_message must not exceed 500 characters")
	}
	if config.ImageDelayMS < 0 || config.ImageDelayMS > maxDelayMS {
		return fmt.Errorf("image_delay_ms must be between 0 and %d", maxDelayMS)
	}
	if config.WebhookDelayMS < 0 || config.WebhookDelayMS > maxDelayMS {
		return fmt.Errorf("webhook_delay_ms must be between 0 and %d", maxDelayMS)
	}
	return nil
}

func (s *serverState) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
}

func (s *serverState) handleMetrics(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, s.snapshot())
}

func (s *serverState) handleReset(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer)
		return
	}
	writeJSON(writer, http.StatusOK, s.resetMetrics())
}

func (s *serverState) handleControl(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(writer, http.StatusOK, s.getConfig())
	case http.MethodPost, http.MethodPut:
		var update controlUpdate
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxBodySize))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&update); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		config, err := s.updateConfig(update)
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, config)
	default:
		writeMethodNotAllowed(writer)
	}
}

type imageTaskRequest struct {
	ClientTaskID string         `json:"client_task_id"`
	Metadata     map[string]any `json:"metadata"`
}

func (s *serverState) handleImageTask(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer)
		return
	}
	var payload imageTaskRequest
	if err := json.NewDecoder(io.LimitReader(request.Body, maxBodySize)).Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid JSON"}})
		return
	}
	config := s.getConfig()
	status := metadataInt(payload.Metadata, "async_test_status", config.ImageStatus)
	delayMS := metadataInt(payload.Metadata, "async_test_delay_ms", config.ImageDelayMS)
	if status < 100 || status > 599 || delayMS < 0 || delayMS > maxDelayMS {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid async-test metadata"}})
		return
	}
	finish := s.begin("image", "image:/v1/image/tasks", request.Header.Get("Authorization") != "")
	succeeded := status >= 200 && status < 300
	defer finish(succeeded)
	if !waitForDelay(request.Context(), delayMS) {
		return
	}
	if !succeeded {
		writeJSON(writer, status, map[string]any{"error": map[string]any{"message": "async-test mock forced image failure"}})
		return
	}
	writeJSON(writer, status, map[string]any{
		"provider_task_id": fmt.Sprintf("mock_image_%d", s.sequence.Add(1)),
		"client_task_id":   payload.ClientTaskID,
		"status":           "queued",
	})
}

func metadataInt(metadata map[string]any, key string, fallback int) int {
	value, exists := metadata[key]
	if !exists {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return -1
}

func (s *serverState) handleVideoSubmit(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer)
		return
	}
	rawBody, err := io.ReadAll(io.LimitReader(request.Body, maxBodySize))
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid video JSON"}})
		return
	}
	var payload videoTaskRequest
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid video JSON"}})
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid video JSON"}})
		return
	}
	leonardo := payload.Public != nil || payload.Seed != nil || fields["image_references"] != nil || fields["video_references"] != nil || fields["audio_references"] != nil
	if leonardo {
		h3 := payload.Model == "minimax-h3-1440p"
		seedance25 := isLeonardoSeedance25Model(payload.Model)
		legacyFields := []string{"reference_images", "reference_videos", "reference_audios"}
		if !h3 && !seedance25 {
			legacyFields = append(legacyFields, "reference_mode")
		}
		for _, legacyField := range legacyFields {
			if fields[legacyField] != nil {
				writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "Leonardo mock received a legacy reference field"}})
				return
			}
		}
		validLeonardo := payload.Public != nil && !*payload.Public
		if h3 {
			mode := strings.ToLower(strings.TrimSpace(payload.ReferenceMode))
			validLeonardo = validLeonardo && payload.Seed == nil && len(payload.VideoReferences) == 0 &&
				(payload.GenerateAudio == nil || *payload.GenerateAudio) &&
				len(payload.ImageReferences) <= 5 && len(payload.AudioReferences) <= 3 &&
				((mode == "" && len(payload.ImageReferences) == 0 && len(payload.AudioReferences) == 0) ||
					(mode == "frame" && len(payload.ImageReferences) >= 1 && len(payload.ImageReferences) <= 2 && len(payload.AudioReferences) == 0) ||
					(mode == "images" && len(payload.ImageReferences) >= 1 && len(payload.AudioReferences) == 0) ||
					(mode == "media" && len(payload.ImageReferences) >= 1 && len(payload.AudioReferences) >= 1)) &&
				validVideoReferencesForLeonardo(payload.ImageReferences, nil, payload.AudioReferences)
		} else if seedance25 {
			mode := strings.ToLower(strings.TrimSpace(payload.ReferenceMode))
			validLeonardo = validLeonardo && payload.Seed != nil && *payload.Seed == -1 &&
				len(payload.ImageReferences) <= 4 && len(payload.VideoReferences) <= 3 && len(payload.AudioReferences) <= 1 &&
				len(payload.ImageReferences)+len(payload.VideoReferences) <= 7 &&
				(len(payload.AudioReferences) == 0 || len(payload.ImageReferences)+len(payload.VideoReferences) > 0) &&
				((mode == "" && len(payload.ImageReferences) == 0 && len(payload.VideoReferences) == 0 && len(payload.AudioReferences) == 0) ||
					mode == "media" ||
					(mode == "frame" && len(payload.ImageReferences) >= 1 && len(payload.ImageReferences) <= 2 &&
						len(payload.VideoReferences) == 0 && len(payload.AudioReferences) == 0)) &&
				validVideoReferencesForLeonardo(payload.ImageReferences, payload.VideoReferences, payload.AudioReferences)
		} else {
			validLeonardo = validLeonardo && payload.Seed != nil && *payload.Seed == -1 &&
				len(payload.ImageReferences) <= 4 && len(payload.VideoReferences) <= 3 && len(payload.AudioReferences) <= 1 &&
				len(payload.ImageReferences)+len(payload.VideoReferences) <= 7 &&
				(len(payload.AudioReferences) == 0 || len(payload.ImageReferences)+len(payload.VideoReferences) > 0) &&
				validVideoReferencesForLeonardo(payload.ImageReferences, payload.VideoReferences, payload.AudioReferences)
		}
		if !validLeonardo {
			writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid Leonardo video request"}})
			return
		}
	} else {
		payload.ReferenceMode = strings.ToLower(strings.TrimSpace(payload.ReferenceMode))
		if payload.ReferenceMode == "" {
			payload.ReferenceMode = "frame"
		}
	}
	maximumDuration := 15
	if isLeonardoSeedance25Model(payload.Model) {
		maximumDuration = 30
	}
	if strings.TrimSpace(payload.Model) == "" || strings.TrimSpace(payload.Prompt) == "" ||
		payload.Duration < 4 || payload.Duration > maximumDuration ||
		(payload.Model == "minimax-h3-1440p" && payload.Duration < 5) ||
		!validVideoAspectRatio(payload.AspectRatio) ||
		(!leonardo && !validVideoReferences(
			payload.ReferenceMode,
			payload.ReferenceImages,
			payload.ReferenceVideos,
			payload.ReferenceAudios,
		)) {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid video request"}})
		return
	}

	config := s.getConfig()
	finish := s.begin("video", "video:/v1/videos", request.Header.Get("Authorization") != "")
	s.mu.Lock()
	s.videoRequests.SubmitAttempts++
	s.mu.Unlock()
	succeeded := config.VideoSubmitStatus >= 200 && config.VideoSubmitStatus < 300
	defer finish(succeeded)
	if !succeeded {
		writeJSON(writer, config.VideoSubmitStatus, map[string]any{"error": map[string]any{"message": "async-test mock forced video submit failure"}})
		return
	}

	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	s.mu.Lock()
	if idempotencyKey != "" {
		if existingTaskID := s.idempotencyJobs[idempotencyKey]; existingTaskID != "" {
			job := s.videoJobs[existingTaskID]
			s.mu.Unlock()
			if job == nil {
				writeJSON(writer, http.StatusConflict, map[string]any{"error": map[string]any{"message": "idempotency task missing"}})
				return
			}
			writeJSON(writer, config.VideoSubmitStatus, videoTaskResponse(existingTaskID, job.Request, "queued", 0, nil))
			return
		}
	}
	taskID := fmt.Sprintf("mock_video_%d", s.sequence.Add(1))
	s.videoJobs[taskID] = &videoJob{Request: payload}
	if idempotencyKey != "" {
		s.idempotencyJobs[idempotencyKey] = taskID
	}
	s.videoRequests.Submit++
	last := payload
	s.lastVideoSubmit = &last
	disconnect := config.VideoDisconnectFirst && idempotencyKey != "" && !s.disconnectSeen[idempotencyKey]
	if disconnect {
		s.disconnectSeen[idempotencyKey] = true
	}
	s.mu.Unlock()
	if disconnect {
		if hijacker, ok := writer.(http.Hijacker); ok {
			connection, _, hijackErr := hijacker.Hijack()
			if hijackErr == nil {
				_ = connection.Close()
				return
			}
		}
		return
	}
	writeJSON(writer, config.VideoSubmitStatus, videoTaskResponse(taskID, payload, "queued", 0, nil))
}

func isLeonardoSeedance25Model(value string) bool {
	switch strings.TrimSpace(value) {
	case "seedance-2.5-480p", "seedance-2.5-720p":
		return true
	default:
		return false
	}
}

func (s *serverState) handleVideoTask(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/v1/videos/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(writer, request)
		return
	}
	taskID := parts[0]
	if len(parts) == 2 && parts[1] == "content" {
		s.handleVideoContent(writer, request, taskID)
		return
	}
	if len(parts) != 1 {
		http.NotFound(writer, request)
		return
	}

	finish := s.begin("video", "video:/v1/videos/{id}", request.Header.Get("Authorization") != "")
	defer finish(true)
	config := s.getConfig()
	s.mu.Lock()
	job := s.videoJobs[taskID]
	if job != nil {
		job.Polls++
		s.videoRequests.Poll++
	}
	var payload videoTaskRequest
	polls := 0
	if job != nil {
		payload = job.Request
		polls = job.Polls
	}
	s.mu.Unlock()
	if job == nil {
		writeJSON(writer, http.StatusNotFound, map[string]any{"detail": "video task not found"})
		return
	}

	status := "in_progress"
	progress := 50
	if polls == 1 && config.VideoTerminalAfterPoll > 1 {
		status = "queued"
		progress = 0
	}
	var responseErr map[string]any
	if polls >= config.VideoTerminalAfterPoll {
		status = config.VideoTerminalStatus
		progress = 100
		if status == "failed" {
			responseErr = map[string]any{"code": config.VideoTerminalErrorCode, "message": config.VideoTerminalErrorMsg}
		}
	}
	writeJSON(writer, http.StatusOK, videoTaskResponse(taskID, payload, status, progress, responseErr))
}

func (s *serverState) handleVideoContent(writer http.ResponseWriter, request *http.Request, taskID string) {
	finish := s.begin("video", "video:/v1/videos/{id}/content", request.Header.Get("Authorization") != "")
	defer finish(true)
	s.mu.Lock()
	job := s.videoJobs[taskID]
	if job != nil {
		s.videoRequests.Content++
	}
	s.mu.Unlock()
	if job == nil {
		http.NotFound(writer, request)
		return
	}
	content := []byte("mock-video-content")
	writer.Header().Set("Content-Type", "video/mp4")
	if request.Header.Get("Range") == "bytes=0-3" {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes 0-3/%d", len(content)))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(content[:4])
		return
	}
	writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(content)
}

func videoTaskResponse(taskID string, payload videoTaskRequest, status string, progress int, responseErr map[string]any) map[string]any {
	response := map[string]any{
		"id":             taskID,
		"task_id":        taskID,
		"object":         "video",
		"model":          payload.Model,
		"status":         status,
		"progress":       progress,
		"duration":       payload.Duration,
		"aspect_ratio":   payload.AspectRatio,
		"resolution":     "480p",
		"generate_audio": payload.GenerateAudio == nil || *payload.GenerateAudio,
		"created_at":     time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}
	if status == "completed" {
		response["completed_at"] = time.Now().Unix()
		response["video_url"] = "http://async-test-mock:8080/v1/videos/" + taskID + "/content"
	}
	if responseErr != nil {
		response["error"] = responseErr
		response["completed_at"] = time.Now().Unix()
	}
	return response
}

func validVideoReferences(
	mode string,
	images []videoReferenceMedia,
	videos []videoReferenceMedia,
	audios []videoReferenceMedia,
) bool {
	switch mode {
	case "frame":
		if len(images) > 2 || len(videos) != 0 || len(audios) != 0 {
			return false
		}
	case "media":
		if len(images) > 9 || len(videos) > 3 || len(audios) > 3 ||
			len(images)+len(videos)+len(audios) > 12 {
			return false
		}
	default:
		return false
	}

	names := make(map[string]struct{}, len(images)+len(videos)+len(audios))
	references := make([]videoReferenceMedia, 0, len(images)+len(videos)+len(audios))
	references = append(references, images...)
	references = append(references, videos...)
	references = append(references, audios...)
	for _, reference := range references {
		sourceURL := strings.ToLower(strings.TrimSpace(reference.URL))
		if !strings.HasPrefix(sourceURL, "http://") &&
			!strings.HasPrefix(sourceURL, "https://") {
			return false
		}
		name := strings.TrimSpace(reference.Name)
		if name == "" {
			continue
		}
		if _, exists := names[name]; exists {
			return false
		}
		names[name] = struct{}{}
	}
	return true
}

func validVideoReferencesForLeonardo(
	images []videoReferenceMedia,
	videos []videoReferenceMedia,
	audios []videoReferenceMedia,
) bool {
	names := make(map[string]struct{}, len(images)+len(videos)+len(audios))
	references := make([]videoReferenceMedia, 0, len(images)+len(videos)+len(audios))
	references = append(references, images...)
	references = append(references, videos...)
	references = append(references, audios...)
	for _, reference := range references {
		sourceURL := strings.ToLower(strings.TrimSpace(reference.URL))
		if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
			return false
		}
		name := strings.TrimSpace(reference.Name)
		if name == "" {
			continue
		}
		if _, exists := names[name]; exists {
			return false
		}
		names[name] = struct{}{}
	}
	return true
}

func validVideoAspectRatio(value string) bool {
	switch value {
	case "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
		return true
	default:
		return false
	}
}

func (s *serverState) handleWebhook(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer)
		return
	}
	config := s.getConfig()
	status := config.WebhookStatus
	delayMS := config.WebhookDelayMS
	mode := strings.TrimPrefix(request.URL.Path, "/webhook/")
	switch strings.Split(mode, "/")[0] {
	case "success":
		status = http.StatusNoContent
		delayMS = 0
	case "failure":
		status = http.StatusInternalServerError
		delayMS = 0
	case "delay":
		if delayMS == 0 {
			delayMS = 2_000
		}
	default:
		http.NotFound(writer, request)
		return
	}
	if value, ok := queryInt(request, "status"); ok {
		status = value
	}
	if value, ok := queryInt(request, "delay_ms"); ok {
		delayMS = value
	}
	if status < 100 || status > 599 || delayMS < 0 || delayMS > maxDelayMS {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "invalid status or delay_ms"})
		return
	}
	endpoint := "webhook:" + request.URL.Path
	finish := s.begin("webhook", endpoint, request.Header.Get("Authorization") != "")
	succeeded := status >= 200 && status < 300
	defer finish(succeeded)
	_, _ = io.Copy(io.Discard, io.LimitReader(request.Body, maxBodySize))
	if !waitForDelay(request.Context(), delayMS) {
		return
	}
	if status == http.StatusNoContent {
		writer.WriteHeader(status)
		return
	}
	writeJSON(writer, status, map[string]any{"ok": succeeded, "status": status})
}

func queryInt(request *http.Request, key string) (int, bool) {
	value := strings.TrimSpace(request.URL.Query().Get(key))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1, true
	}
	return parsed, true
}

func waitForDelay(ctx context.Context, delayMS int) bool {
	if delayMS <= 0 {
		return true
	}
	timer := time.NewTimer(time.Duration(delayMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeMethodNotAllowed(writer http.ResponseWriter) {
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func runHealthcheck(target string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func main() {
	healthcheck := flag.String("healthcheck", "", "check an HTTP health endpoint and exit")
	flag.Parse()
	if *healthcheck != "" {
		if err := runHealthcheck(*healthcheck); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newServerState().handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("async-test mock listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
