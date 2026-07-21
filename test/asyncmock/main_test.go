package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
