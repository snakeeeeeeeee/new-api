package service

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	RequestDumpPrintOnAll       = "all"
	RequestDumpPrintOnErrorOnly = "error_only"

	RequestDumpLogLevelDebug = "debug"
	RequestDumpLogLevelInfo  = "info"
	RequestDumpLogLevelWarn  = "warn"
	RequestDumpLogLevelError = "error"

	RequestDumpStageRawRequest             = "raw_request"
	RequestDumpStageUpstreamRequest        = "upstream_request"
	RequestDumpStageRelayError             = "relay_error"
	RequestDumpStageResponsesStreamEvent   = "responses_stream_event"
	RequestDumpStageResponsesStreamSummary = "responses_stream_summary"

	defaultRequestDumpDurationSeconds = 300
	maxRequestDumpDurationSeconds     = 1800
	defaultRequestDumpMaxCount        = 20
	maxRequestDumpMaxCount            = 100
	defaultRequestDumpMaxBodyBytes    = 256 * 1024
	maxRequestDumpMaxBodyBytes        = 1024 * 1024
	defaultRequestDumpBufferSize      = 200
	defaultRequestDumpMaxStreamEvents = 200
	maxRequestDumpMaxStreamEvents     = 1000
)

const requestDumpRawRecordedKey = "_request_dump_raw_recorded"
const requestDumpErrorCountedKey = "_request_dump_error_counted"
const requestDumpMatchedCountedKey = "_request_dump_matched_counted"
const requestDumpResponsesStreamEventCountKey = "_request_dump_responses_stream_event_count"

type RequestDumpRule struct {
	UserIDs                           []int    `json:"user_ids"`
	TokenIDs                          []int    `json:"token_ids,omitempty"`
	TokenNames                        []string `json:"token_names,omitempty"`
	Models                            []string `json:"models,omitempty"`
	Paths                             []string `json:"paths,omitempty"`
	AggregateGroups                   []string `json:"aggregate_groups,omitempty"`
	Keywords                          []string `json:"keywords,omitempty"`
	CaseSensitive                     bool     `json:"case_sensitive,omitempty"`
	DurationSeconds                   int      `json:"duration_seconds"`
	MaxCount                          int      `json:"max_count"`
	PrintOn                           string   `json:"print_on"`
	LogLevel                          string   `json:"log_level"`
	PrintURL                          bool     `json:"print_url"`
	PrintHeaders                      bool     `json:"print_headers"`
	PrintBody                         bool     `json:"print_body"`
	PrintUpstreamBody                 bool     `json:"print_upstream_body"`
	MaxBodyBytes                      int64    `json:"max_body_bytes"`
	TraceResponsesStream              bool     `json:"trace_responses_stream,omitempty"`
	TraceResponsesStreamKeyEventsOnly bool     `json:"trace_responses_stream_key_events_only,omitempty"`
	MaxStreamEventsPerRequest         int      `json:"max_stream_events_per_request,omitempty"`
}

type RequestDumpStatus struct {
	Enabled              bool             `json:"enabled"`
	Rule                 *RequestDumpRule `json:"rule,omitempty"`
	StartedAt            int64            `json:"started_at,omitempty"`
	ExpiresAt            int64            `json:"expires_at,omitempty"`
	RemainingSeconds     int64            `json:"remaining_seconds"`
	MatchedCount         int              `json:"matched_count"`
	MaxCount             int              `json:"max_count"`
	ConsoleEventCount    int              `json:"console_event_count"`
	ConsoleOldestEventID int64            `json:"console_oldest_event_id"`
	ConsoleLatestEventID int64            `json:"console_latest_event_id"`
}

type RequestDumpEvent struct {
	ID                  int64             `json:"id"`
	CreatedAt           int64             `json:"created_at"`
	Stage               string            `json:"stage"`
	LogLevel            string            `json:"log_level,omitempty"`
	RequestID           string            `json:"request_id,omitempty"`
	UserID              int               `json:"user_id,omitempty"`
	TokenID             int               `json:"token_id,omitempty"`
	TokenName           string            `json:"token_name,omitempty"`
	Method              string            `json:"method,omitempty"`
	Host                string            `json:"host,omitempty"`
	Path                string            `json:"path,omitempty"`
	RawURL              string            `json:"raw_url,omitempty"`
	Model               string            `json:"model,omitempty"`
	AggregateGroup      string            `json:"aggregate_group,omitempty"`
	ChannelID           int               `json:"channel_id,omitempty"`
	ChannelName         string            `json:"channel_name,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	RawBody             string            `json:"raw_body,omitempty"`
	UpstreamBody        string            `json:"upstream_body,omitempty"`
	ContentType         string            `json:"content_type,omitempty"`
	BodySize            int64             `json:"body_size,omitempty"`
	SkipReason          string            `json:"skip_reason,omitempty"`
	StatusCode          int               `json:"status_code,omitempty"`
	ErrorType           string            `json:"error_type,omitempty"`
	ErrorCode           string            `json:"error_code,omitempty"`
	ErrorMessage        string            `json:"error_message,omitempty"`
	RetryIndex          int               `json:"retry_index,omitempty"`
	StreamEventType     string            `json:"stream_event_type,omitempty"`
	StreamItemType      string            `json:"stream_item_type,omitempty"`
	StreamSequence      int               `json:"stream_sequence,omitempty"`
	StreamStopReason    string            `json:"stream_stop_reason,omitempty"`
	StreamElapsedMs     int64             `json:"stream_elapsed_ms,omitempty"`
	StreamReceivedCount int               `json:"stream_received_count,omitempty"`
	StreamNote          string            `json:"stream_note,omitempty"`
}

type ResponsesStreamDumpMeta struct {
	EventType     string
	ItemType      string
	Sequence      int
	StopReason    string
	ElapsedMs     int64
	ReceivedCount int
	ErrorType     string
	ErrorCode     string
	ErrorMessage  string
	Note          string
}

type requestDumpState struct {
	rule         *RequestDumpRule
	startedAt    time.Time
	expiresAt    time.Time
	matchedCount int
	generation   int64
	nextEventID  int64
	events       []RequestDumpEvent
}

var requestDump = struct {
	mu    sync.RWMutex
	state requestDumpState
}{}

func NormalizeRequestDumpRule(rule RequestDumpRule) (RequestDumpRule, error) {
	rule.UserIDs = normalizePositiveInts(rule.UserIDs)
	rule.TokenIDs = normalizePositiveInts(rule.TokenIDs)
	rule.TokenNames = normalizeStrings(rule.TokenNames)
	rule.Models = normalizeStrings(rule.Models)
	rule.Paths = normalizeStrings(rule.Paths)
	rule.AggregateGroups = normalizeStrings(rule.AggregateGroups)
	rule.Keywords = normalizeStrings(rule.Keywords)

	if len(rule.UserIDs) == 0 {
		return rule, fmt.Errorf("user_ids is required")
	}
	if rule.DurationSeconds <= 0 {
		rule.DurationSeconds = defaultRequestDumpDurationSeconds
	}
	if rule.DurationSeconds > maxRequestDumpDurationSeconds {
		rule.DurationSeconds = maxRequestDumpDurationSeconds
	}
	if rule.MaxCount <= 0 {
		rule.MaxCount = defaultRequestDumpMaxCount
	}
	if rule.MaxCount > maxRequestDumpMaxCount {
		rule.MaxCount = maxRequestDumpMaxCount
	}
	if rule.PrintOn == "" {
		rule.PrintOn = RequestDumpPrintOnAll
	}
	switch rule.PrintOn {
	case RequestDumpPrintOnAll, RequestDumpPrintOnErrorOnly:
	default:
		return rule, fmt.Errorf("print_on must be all or error_only")
	}
	if rule.LogLevel == "" {
		rule.LogLevel = RequestDumpLogLevelInfo
	}
	rule.LogLevel = strings.ToLower(strings.TrimSpace(rule.LogLevel))
	switch rule.LogLevel {
	case RequestDumpLogLevelDebug, RequestDumpLogLevelInfo, RequestDumpLogLevelWarn, RequestDumpLogLevelError:
	default:
		return rule, fmt.Errorf("log_level must be debug, info, warn or error")
	}
	if rule.MaxBodyBytes <= 0 {
		rule.MaxBodyBytes = defaultRequestDumpMaxBodyBytes
	}
	if rule.MaxBodyBytes > maxRequestDumpMaxBodyBytes {
		rule.MaxBodyBytes = maxRequestDumpMaxBodyBytes
	}
	if rule.MaxStreamEventsPerRequest <= 0 {
		rule.MaxStreamEventsPerRequest = defaultRequestDumpMaxStreamEvents
	}
	if rule.MaxStreamEventsPerRequest > maxRequestDumpMaxStreamEvents {
		rule.MaxStreamEventsPerRequest = maxRequestDumpMaxStreamEvents
	}
	return rule, nil
}

func DefaultRequestDumpRule() RequestDumpRule {
	return RequestDumpRule{
		DurationSeconds:           defaultRequestDumpDurationSeconds,
		MaxCount:                  defaultRequestDumpMaxCount,
		PrintOn:                   RequestDumpPrintOnAll,
		LogLevel:                  RequestDumpLogLevelInfo,
		PrintURL:                  true,
		PrintHeaders:              true,
		PrintBody:                 true,
		MaxBodyBytes:              defaultRequestDumpMaxBodyBytes,
		MaxStreamEventsPerRequest: defaultRequestDumpMaxStreamEvents,
	}
}

func StartRequestDump(rule RequestDumpRule) (RequestDumpStatus, error) {
	normalized, err := NormalizeRequestDumpRule(rule)
	if err != nil {
		return RequestDumpStatus{}, err
	}
	now := time.Now()
	requestDump.mu.Lock()
	requestDump.state.generation++
	requestDump.state.rule = &normalized
	requestDump.state.startedAt = now
	requestDump.state.expiresAt = now.Add(time.Duration(normalized.DurationSeconds) * time.Second)
	requestDump.state.matchedCount = 0
	requestDump.state.events = nil
	requestDump.mu.Unlock()
	return GetRequestDumpStatus(), nil
}

func StopRequestDump() RequestDumpStatus {
	requestDump.mu.Lock()
	requestDump.state.generation++
	requestDump.state.rule = nil
	requestDump.state.startedAt = time.Time{}
	requestDump.state.expiresAt = time.Time{}
	requestDump.state.matchedCount = 0
	requestDump.mu.Unlock()
	return GetRequestDumpStatus()
}

func ClearRequestDumpEvents() RequestDumpStatus {
	requestDump.mu.Lock()
	requestDump.state.events = nil
	requestDump.mu.Unlock()
	return GetRequestDumpStatus()
}

func GetRequestDumpStatus() RequestDumpStatus {
	now := time.Now()
	requestDump.mu.Lock()
	expireLockedIfNeeded(now)
	status := buildRequestDumpStatusLocked(now)
	requestDump.mu.Unlock()
	return status
}

func GetRequestDumpEvents(afterID int64, limit int) []RequestDumpEvent {
	if limit <= 0 {
		limit = 100
	}
	if limit > defaultRequestDumpBufferSize {
		limit = defaultRequestDumpBufferSize
	}
	requestDump.mu.RLock()
	defer requestDump.mu.RUnlock()
	events := make([]RequestDumpEvent, 0, min(limit, len(requestDump.state.events)))
	for _, event := range requestDump.state.events {
		if event.ID <= afterID {
			continue
		}
		events = append(events, event)
		if len(events) >= limit {
			break
		}
	}
	return events
}

func DumpRawRequestIfNeeded(c *gin.Context) {
	safeDump(func() {
		if c == nil {
			return
		}
		if c.GetBool(requestDumpRawRecordedKey) {
			return
		}
		rule, generation, matched := getActiveRequestDumpRule(c)
		if !matched || rule.PrintOn == RequestDumpPrintOnErrorOnly {
			return
		}
		event := buildBaseRequestDumpEvent(c, RequestDumpStageRawRequest, rule)
		fillRawBody(c, rule, &event)
		if appendMatchedRequestDumpEvent(c, event, rule, generation) {
			c.Set(requestDumpRawRecordedKey, true)
		}
	})
}

func DumpUpstreamRequestIfNeeded(c *gin.Context, body []byte) {
	safeDump(func() {
		if c == nil || len(body) == 0 {
			return
		}
		rule, generation, matched := getActiveRequestDumpRule(c)
		if !matched || rule.PrintOn == RequestDumpPrintOnErrorOnly || !rule.PrintUpstreamBody {
			return
		}
		event := buildBaseRequestDumpEvent(c, RequestDumpStageUpstreamRequest, rule)
		event.BodySize = int64(len(body))
		if int64(len(body)) > rule.MaxBodyBytes {
			event.SkipReason = "body_too_large"
		} else {
			event.UpstreamBody = string(body)
		}
		appendMatchedRequestDumpEvent(c, event, rule, generation)
	})
}

func DumpRelayErrorIfNeeded(c *gin.Context, err *types.NewAPIError) {
	safeDump(func() {
		if c == nil || err == nil {
			return
		}
		rule, generation, matched := getActiveRequestDumpRule(c)
		if !matched {
			return
		}
		event := buildBaseRequestDumpEvent(c, RequestDumpStageRelayError, rule)
		event.StatusCode = err.StatusCode
		event.ErrorType = string(err.GetErrorType())
		event.ErrorCode = string(err.GetErrorCode())
		event.ErrorMessage = err.MaskSensitiveError()
		event.RetryIndex = c.GetInt("request_dump_retry_index")
		if !c.GetBool(requestDumpRawRecordedKey) {
			fillRawBody(c, rule, &event)
		}
		if appendMatchedRequestDumpEvent(c, event, rule, generation) {
			c.Set(requestDumpRawRecordedKey, true)
		}
	})
}

func DumpResponsesStreamEventIfNeeded(c *gin.Context, meta ResponsesStreamDumpMeta) {
	safeDump(func() {
		if c == nil {
			return
		}
		rule, generation, matched := getActiveRequestDumpRule(c)
		if !matched || !rule.TraceResponsesStream || rule.PrintOn == RequestDumpPrintOnErrorOnly {
			return
		}
		if rule.TraceResponsesStreamKeyEventsOnly && !isKeyResponsesStreamDumpEvent(meta) {
			return
		}
		count := c.GetInt(requestDumpResponsesStreamEventCountKey)
		if count >= rule.MaxStreamEventsPerRequest {
			return
		}
		c.Set(requestDumpResponsesStreamEventCountKey, count+1)
		event := buildBaseRequestDumpEvent(c, RequestDumpStageResponsesStreamEvent, rule)
		fillResponsesStreamDumpFields(&event, meta)
		if event.StreamSequence == 0 {
			event.StreamSequence = count + 1
		}
		appendMatchedRequestDumpEvent(c, event, rule, generation)
	})
}

func DumpResponsesStreamSummaryIfNeeded(c *gin.Context, meta ResponsesStreamDumpMeta) {
	safeDump(func() {
		if c == nil {
			return
		}
		rule, generation, matched := getActiveRequestDumpRule(c)
		if !matched || !rule.TraceResponsesStream {
			return
		}
		event := buildBaseRequestDumpEvent(c, RequestDumpStageResponsesStreamSummary, rule)
		fillResponsesStreamDumpFields(&event, meta)
		appendMatchedRequestDumpEvent(c, event, rule, generation)
	})
}

func fillResponsesStreamDumpFields(event *RequestDumpEvent, meta ResponsesStreamDumpMeta) {
	if event == nil {
		return
	}
	event.StreamEventType = strings.TrimSpace(meta.EventType)
	event.StreamItemType = strings.TrimSpace(meta.ItemType)
	event.StreamSequence = meta.Sequence
	event.StreamStopReason = strings.TrimSpace(meta.StopReason)
	event.StreamElapsedMs = meta.ElapsedMs
	event.StreamReceivedCount = meta.ReceivedCount
	event.StreamNote = limitRequestDumpString(strings.TrimSpace(meta.Note), 500)
	event.ErrorType = strings.TrimSpace(meta.ErrorType)
	event.ErrorCode = strings.TrimSpace(meta.ErrorCode)
	event.ErrorMessage = limitRequestDumpString(strings.TrimSpace(meta.ErrorMessage), 500)
}

func isKeyResponsesStreamDumpEvent(meta ResponsesStreamDumpMeta) bool {
	if strings.TrimSpace(meta.StopReason) != "" || strings.TrimSpace(meta.ErrorMessage) != "" {
		return true
	}
	eventType := strings.TrimSpace(meta.EventType)
	switch eventType {
	case "response.completed", "response.failed", "response.error", "response.incomplete", "response.cancelled":
		return true
	case "response.output_item.added", "response.output_item.done":
		return strings.TrimSpace(meta.ItemType) != ""
	}
	if strings.Contains(eventType, "function_call") {
		return true
	}
	return false
}

func getActiveRequestDumpRule(c *gin.Context) (RequestDumpRule, int64, bool) {
	now := time.Now()
	requestDump.mu.Lock()
	defer requestDump.mu.Unlock()
	expireLockedIfNeeded(now)
	if requestDump.state.rule == nil {
		return RequestDumpRule{}, 0, false
	}
	rule := *requestDump.state.rule
	if !matchesRequestDumpRule(c, &rule) {
		return RequestDumpRule{}, 0, false
	}
	return rule, requestDump.state.generation, true
}

func shouldCountRequestDumpMatch(c *gin.Context, stage string, printOn string) bool {
	if c != nil && c.GetBool(requestDumpMatchedCountedKey) {
		return false
	}
	if printOn == RequestDumpPrintOnErrorOnly {
		if c != nil && c.GetBool(requestDumpErrorCountedKey) {
			return false
		}
		return stage == RequestDumpStageRelayError
	}
	return stage == RequestDumpStageRawRequest ||
		stage == RequestDumpStageUpstreamRequest ||
		stage == RequestDumpStageRelayError ||
		stage == RequestDumpStageResponsesStreamEvent ||
		stage == RequestDumpStageResponsesStreamSummary
}

func buildBaseRequestDumpEvent(c *gin.Context, stage string, rule RequestDumpRule) RequestDumpEvent {
	event := RequestDumpEvent{
		CreatedAt:      time.Now().Unix(),
		Stage:          stage,
		LogLevel:       rule.LogLevel,
		RequestID:      c.GetString(common.RequestIdKey),
		UserID:         c.GetInt("id"),
		TokenID:        c.GetInt("token_id"),
		TokenName:      c.GetString("token_name"),
		Model:          c.GetString("original_model"),
		AggregateGroup: common.GetContextKeyString(c, constant.ContextKeyAggregateGroup),
		ChannelID:      c.GetInt("channel_id"),
		ChannelName:    c.GetString("channel_name"),
	}
	if c.Request != nil {
		event.Method = c.Request.Method
		event.Host = c.Request.Host
		event.ContentType = c.Request.Header.Get("Content-Type")
		if c.Request.URL != nil {
			event.Path = c.Request.URL.Path
			if rule.PrintURL {
				event.RawURL = c.Request.URL.String()
			}
		}
		if rule.PrintHeaders {
			event.Headers = filterRequestDumpHeaders(c.Request.Header)
		}
	}
	return event
}

func fillRawBody(c *gin.Context, rule RequestDumpRule, event *RequestDumpEvent) {
	if !rule.PrintBody || event == nil {
		return
	}
	if isUnsupportedRequestDumpContentType(event.ContentType) {
		event.SkipReason = "unsupported_content_type"
		return
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		event.SkipReason = "read_body_failed:" + err.Error()
		return
	}
	event.BodySize = storage.Size()
	if storage.Size() > rule.MaxBodyBytes {
		event.SkipReason = "body_too_large"
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		event.SkipReason = "read_body_failed:" + err.Error()
		return
	}
	event.RawBody = string(body)
}

func appendMatchedRequestDumpEvent(c *gin.Context, event RequestDumpEvent, rule RequestDumpRule, generation int64) bool {
	if (c == nil || !c.GetBool(requestDumpMatchedCountedKey)) && !matchesRequestDumpKeywords(&rule, &event) {
		return false
	}

	requestDump.mu.Lock()

	expireLockedIfNeeded(time.Now())
	if requestDump.state.rule == nil || requestDump.state.generation != generation {
		requestDump.mu.Unlock()
		return false
	}
	shouldCount := shouldCountRequestDumpMatch(c, event.Stage, rule.PrintOn)
	if shouldCount && requestDump.state.matchedCount >= rule.MaxCount {
		requestDump.state.rule = nil
		requestDump.mu.Unlock()
		return false
	}
	if shouldCount {
		requestDump.state.matchedCount++
		if c != nil {
			c.Set(requestDumpMatchedCountedKey, true)
			if event.Stage == RequestDumpStageRelayError {
				c.Set(requestDumpErrorCountedKey, true)
			}
		}
	}
	appendRequestDumpEventLocked(&event)
	if shouldCount && requestDump.state.matchedCount >= rule.MaxCount {
		requestDump.state.rule = nil
	}
	requestDump.mu.Unlock()

	logRequestDumpEvent(event, rule.LogLevel)
	return true
}

func appendRequestDumpEvent(event RequestDumpEvent, logLevel ...string) {
	requestDump.mu.Lock()
	appendRequestDumpEventLocked(&event)
	requestDump.mu.Unlock()

	level := event.LogLevel
	if len(logLevel) > 0 {
		level = logLevel[0]
	}
	logRequestDumpEvent(event, level)
}

func appendRequestDumpEventLocked(event *RequestDumpEvent) {
	requestDump.state.nextEventID++
	event.ID = requestDump.state.nextEventID
	requestDump.state.events = append(requestDump.state.events, *event)
	if len(requestDump.state.events) > defaultRequestDumpBufferSize {
		requestDump.state.events = append([]RequestDumpEvent(nil), requestDump.state.events[len(requestDump.state.events)-defaultRequestDumpBufferSize:]...)
	}
}

func logRequestDumpEvent(event RequestDumpEvent, logLevel string) {
	if data, err := common.Marshal(event); err == nil {
		ctx := contextForRequestDumpLog(event.RequestID)
		msg := "request_dump " + string(data)
		switch strings.ToLower(strings.TrimSpace(logLevel)) {
		case RequestDumpLogLevelDebug:
			logger.LogDebugForce(ctx, msg)
		case RequestDumpLogLevelWarn:
			logger.LogWarn(ctx, msg)
		case RequestDumpLogLevelError:
			logger.LogError(ctx, msg)
		default:
			logger.LogInfo(ctx, msg)
		}
	}
}

func buildRequestDumpStatusLocked(now time.Time) RequestDumpStatus {
	status := RequestDumpStatus{
		MatchedCount:      requestDump.state.matchedCount,
		ConsoleEventCount: len(requestDump.state.events),
	}
	if len(requestDump.state.events) > 0 {
		status.ConsoleOldestEventID = requestDump.state.events[0].ID
		status.ConsoleLatestEventID = requestDump.state.events[len(requestDump.state.events)-1].ID
	}
	if requestDump.state.rule == nil {
		return status
	}
	ruleCopy := *requestDump.state.rule
	status.Enabled = true
	status.Rule = &ruleCopy
	status.StartedAt = requestDump.state.startedAt.Unix()
	status.ExpiresAt = requestDump.state.expiresAt.Unix()
	status.RemainingSeconds = int64(time.Until(requestDump.state.expiresAt).Seconds())
	if status.RemainingSeconds < 0 {
		status.RemainingSeconds = 0
	}
	status.MaxCount = ruleCopy.MaxCount
	_ = now
	return status
}

func expireLockedIfNeeded(now time.Time) {
	if requestDump.state.rule == nil {
		return
	}
	if !requestDump.state.expiresAt.IsZero() && !now.Before(requestDump.state.expiresAt) {
		requestDump.state.rule = nil
	}
}

func matchesRequestDumpRule(c *gin.Context, rule *RequestDumpRule) bool {
	if c == nil || rule == nil {
		return false
	}
	if !containsInt(rule.UserIDs, c.GetInt("id")) {
		return false
	}
	if len(rule.TokenIDs) > 0 && !containsInt(rule.TokenIDs, c.GetInt("token_id")) {
		return false
	}
	if len(rule.TokenNames) > 0 && !containsString(rule.TokenNames, c.GetString("token_name")) {
		return false
	}
	model := c.GetString("original_model")
	if len(rule.Models) > 0 && !containsString(rule.Models, model) {
		return false
	}
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	if len(rule.Paths) > 0 && !matchesPath(rule.Paths, path) {
		return false
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	if len(rule.AggregateGroups) > 0 && !containsString(rule.AggregateGroups, aggregateGroup) {
		return false
	}
	return true
}

func matchesRequestDumpKeywords(rule *RequestDumpRule, event *RequestDumpEvent) bool {
	if rule == nil || event == nil || len(rule.Keywords) == 0 {
		return true
	}
	haystack := buildRequestDumpKeywordHaystack(event)
	if !rule.CaseSensitive {
		haystack = strings.ToLower(haystack)
	}
	for _, keyword := range rule.Keywords {
		if !rule.CaseSensitive {
			keyword = strings.ToLower(keyword)
		}
		if keyword != "" && strings.Contains(haystack, keyword) {
			return true
		}
	}
	return false
}

func buildRequestDumpKeywordHaystack(event *RequestDumpEvent) string {
	var builder strings.Builder
	appendKeywordPart := func(value string) {
		if value == "" {
			return
		}
		builder.WriteByte('\n')
		builder.WriteString(value)
	}
	appendKeywordInt := func(value int) {
		if value > 0 {
			appendKeywordPart(strconv.Itoa(value))
		}
	}
	appendKeywordPart(event.Stage)
	appendKeywordPart(event.RequestID)
	appendKeywordInt(event.UserID)
	appendKeywordInt(event.TokenID)
	appendKeywordPart(event.TokenName)
	appendKeywordPart(event.Method)
	appendKeywordPart(event.Host)
	appendKeywordPart(event.Path)
	appendKeywordPart(event.RawURL)
	appendKeywordPart(event.Model)
	appendKeywordPart(event.AggregateGroup)
	appendKeywordInt(event.ChannelID)
	appendKeywordPart(event.ChannelName)
	appendKeywordPart(event.ContentType)
	appendKeywordPart(event.SkipReason)
	appendKeywordInt(event.StatusCode)
	appendKeywordPart(event.ErrorType)
	appendKeywordPart(event.ErrorCode)
	appendKeywordPart(event.ErrorMessage)
	appendKeywordInt(event.RetryIndex)
	appendKeywordPart(event.StreamEventType)
	appendKeywordPart(event.StreamItemType)
	appendKeywordPart(event.StreamStopReason)
	appendKeywordPart(event.StreamNote)
	appendKeywordPart(event.RawBody)
	appendKeywordPart(event.UpstreamBody)
	if len(event.Headers) > 0 {
		keys := make([]string, 0, len(event.Headers))
		for key := range event.Headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendKeywordPart(key)
			appendKeywordPart(event.Headers[key])
		}
	}
	return builder.String()
}

func filterRequestDumpHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	keys := make([]string, 0, len(header))
	for key := range header {
		if isSensitiveRequestDumpHeader(key) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(header.Get(key))
		if value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func isSensitiveRequestDumpHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return true
	}
	switch normalized {
	case "authorization", "cookie", "set-cookie", "x-api-key", "api-key", "proxy-authorization":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "key") ||
		strings.Contains(normalized, "password")
}

func isUnsupportedRequestDumpContentType(contentType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if normalized == "" {
		return false
	}
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	return normalized == "application/octet-stream" ||
		strings.HasPrefix(normalized, "multipart/form-data") ||
		strings.HasPrefix(normalized, "audio/") ||
		strings.HasPrefix(normalized, "image/") ||
		strings.HasPrefix(normalized, "video/")
}

func normalizePositiveInts(values []int) []int {
	set := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value > 0 {
			set[value] = struct{}{}
		}
	}
	result := make([]int, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func normalizeStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func limitRequestDumpString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func matchesPath(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if pattern == path || strings.HasPrefix(path, pattern) {
			return true
		}
	}
	return false
}

func safeDump(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(contextForRequestDumpLog(""), fmt.Sprintf("request_dump panic recovered: %v", r))
		}
	}()
	fn()
}

type requestDumpLogContext string

func contextForRequestDumpLog(requestID string) requestDumpLogContext {
	if strings.TrimSpace(requestID) == "" {
		return requestDumpLogContext("SYSTEM")
	}
	return requestDumpLogContext(requestID)
}

func (c requestDumpLogContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c requestDumpLogContext) Done() <-chan struct{} {
	return nil
}

func (c requestDumpLogContext) Err() error {
	return nil
}

func (c requestDumpLogContext) Value(key any) any {
	if key == common.RequestIdKey {
		return string(c)
	}
	return nil
}

func ParseRequestDumpEventQuery(afterIDStr string, limitStr string) (int64, int) {
	var afterID int64
	if afterIDStr != "" {
		if parsed, err := strconv.ParseInt(afterIDStr, 10, 64); err == nil && parsed > 0 {
			afterID = parsed
		}
	}
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return afterID, limit
}
