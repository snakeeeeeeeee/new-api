package main

import (
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
	ImageStatus    int `json:"image_status"`
	ImageDelayMS   int `json:"image_delay_ms"`
	WebhookStatus  int `json:"webhook_status"`
	WebhookDelayMS int `json:"webhook_delay_ms"`
}

type controlUpdate struct {
	ImageStatus    *int `json:"image_status"`
	ImageDelayMS   *int `json:"image_delay_ms"`
	WebhookStatus  *int `json:"webhook_status"`
	WebhookDelayMS *int `json:"webhook_delay_ms"`
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
	Webhook    int            `json:"webhook"`
	ByEndpoint map[string]int `json:"by_endpoint"`
}

type metricsResponse struct {
	StartedAt int64             `json:"started_at"`
	Current   concurrencyCounts `json:"current_in_flight"`
	Peak      concurrencyCounts `json:"peak_in_flight"`
	Requests  requestCounts     `json:"requests"`
	Config    mockConfig        `json:"config"`
}

type serverState struct {
	mu        sync.Mutex
	startedAt int64
	current   concurrencyCounts
	peak      concurrencyCounts
	requests  requestCounts
	config    mockConfig
	sequence  atomic.Uint64
}

func newServerState() *serverState {
	return &serverState{
		startedAt: time.Now().Unix(),
		current:   concurrencyCounts{ByEndpoint: make(map[string]int)},
		peak:      concurrencyCounts{ByEndpoint: make(map[string]int)},
		config:    mockConfig{ImageStatus: http.StatusAccepted, WebhookStatus: http.StatusNoContent},
	}
}

func (s *serverState) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/reset", s.handleReset)
	mux.HandleFunc("/control", s.handleControl)
	mux.HandleFunc("/v1/image/tasks", s.handleImageTask)
	mux.HandleFunc("/webhook/", s.handleWebhook)
	return mux
}

func (s *serverState) begin(kind, endpoint string, hasAuthorization bool) func(bool) {
	s.mu.Lock()
	s.current.Total++
	s.current.ByEndpoint[endpoint]++
	if kind == "image" {
		s.current.Image++
	} else {
		s.current.Webhook++
	}
	s.peak.Total = max(s.peak.Total, s.current.Total)
	s.peak.Image = max(s.peak.Image, s.current.Image)
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
		if kind == "image" {
			s.current.Image--
		} else {
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
	return metricsResponse{
		StartedAt: s.startedAt,
		Current:   cloneConcurrency(s.current),
		Peak:      cloneConcurrency(s.peak),
		Requests:  s.requests,
		Config:    s.config,
	}
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
