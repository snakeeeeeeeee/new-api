package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"

	"github.com/gin-gonic/gin"
)

const (
	RelayResponseAnomalyUsageZeroZero          = "relay_response_usage_0_0"
	RelayResponseAnomalyOutputTokensZero       = "relay_response_output_tokens_0"
	RelayResponseAnomalyOutputTokensOne        = "relay_response_output_tokens_1"
	RelayResponseAnomalyReasoningOnly          = "relay_response_reasoning_only"
	RelayResponseAnomalyEmptyOutput            = "relay_response_empty_visible_output"
	RelayResponseAnomalyFailed                 = "relay_response_failed"
	RelayResponseAnomalyIncomplete             = "relay_response_incomplete"
	RelayResponseAnomalyCancelled              = "relay_response_cancelled"
	RelayResponseAnomalyTerminalMissing        = "relay_stream_terminal_missing"
	RelayResponseAnomalyTerminalOutputMismatch = "relay_response_terminal_output_mismatch"
)

const (
	relayResponseDiagnosticsContextKey = "_relay_response_diagnostics"
	relayDiagnosticMaxEvents           = 256
	relayDiagnosticMaxBytesPerSide     = 16 << 10
)

type relayDiagnosticTraceEvent struct {
	Sequence  int    `json:"sequence"`
	EventType string `json:"event_type,omitempty"`
	Data      string `json:"data,omitempty"`
	DataBytes int    `json:"data_bytes"`
	SHA256    string `json:"sha256"`
	Truncated bool   `json:"truncated,omitempty"`
}

type relayDiagnosticTraceSide struct {
	Events      []relayDiagnosticTraceEvent
	TotalEvents int
	StoredBytes int
	Truncated   bool
}

type relayResponseDiagnostics struct {
	Protocol   string
	Upstream   relayDiagnosticTraceSide
	Downstream relayDiagnosticTraceSide
}

type RelayResponseAnomalyEvidence struct {
	ForcedReason           string
	Protocol               string
	UpstreamModel          string
	ResponseModel          string
	RelayFormat            string
	IsStream               bool
	VisibleText            string
	ReasoningText          string
	HasToolCall            bool
	HasOtherOutput         bool
	FinishReasons          []string
	ResponseStatus         string
	TerminalEvent          string
	TerminalSeen           bool
	RawUsage               *dto.Usage
	EffectiveUsage         *dto.Usage
	UsageSource            string
	UpstreamResponseBody   []byte
	DownstreamResponseBody []byte
	Stream                 map[string]any
	Details                map[string]any
}

type diagnosticResponseSnapshotEvidence struct {
	StatusCode             int
	ErrorType              string
	ErrorCode              string
	ErrorMessage           string
	IsStream               bool
	Response               map[string]any
	UpstreamResponseBody   []byte
	DownstreamResponseBody []byte
	Stream                 map[string]any
}

func RelayResponseDiagnosticsEnabled() bool {
	return error_snapshot_setting.GetSnapshot().Enabled
}

func BeginRelayResponseDiagnostics(c *gin.Context, protocol string) {
	if c == nil || !RelayResponseDiagnosticsEnabled() {
		return
	}
	c.Set(relayResponseDiagnosticsContextKey, &relayResponseDiagnostics{Protocol: strings.TrimSpace(protocol)})
}

func RecordRelayResponseUpstream(c *gin.Context, eventType, data string) {
	diagnostics := getRelayResponseDiagnostics(c)
	if diagnostics == nil {
		return
	}
	recordRelayDiagnosticTrace(&diagnostics.Upstream, eventType, data)
}

func RecordRelayResponseDownstream(c *gin.Context, eventType, data string) {
	diagnostics := getRelayResponseDiagnostics(c)
	if diagnostics == nil {
		return
	}
	recordRelayDiagnosticTrace(&diagnostics.Downstream, eventType, data)
}

func RelayResponseDiagnosticsSummary(c *gin.Context) map[string]any {
	diagnostics := getRelayResponseDiagnostics(c)
	if diagnostics == nil {
		return nil
	}
	return map[string]any{
		"protocol":   diagnostics.Protocol,
		"upstream":   relayDiagnosticTraceSummary(diagnostics.Upstream),
		"downstream": relayDiagnosticTraceSummary(diagnostics.Downstream),
	}
}

func getRelayResponseDiagnostics(c *gin.Context) *relayResponseDiagnostics {
	if c == nil {
		return nil
	}
	value, exists := c.Get(relayResponseDiagnosticsContextKey)
	if !exists {
		return nil
	}
	diagnostics, _ := value.(*relayResponseDiagnostics)
	return diagnostics
}

func relayDiagnosticTraceSummary(side relayDiagnosticTraceSide) map[string]any {
	return map[string]any{
		"events":       side.Events,
		"total_events": side.TotalEvents,
		"stored_bytes": side.StoredBytes,
		"truncated":    side.Truncated,
	}
}

func recordRelayDiagnosticTrace(side *relayDiagnosticTraceSide, eventType, data string) {
	if side == nil {
		return
	}
	side.TotalEvents++
	if len(side.Events) >= relayDiagnosticMaxEvents || side.StoredBytes >= relayDiagnosticMaxBytesPerSide {
		side.Truncated = true
		return
	}
	remaining := relayDiagnosticMaxBytesPerSide - side.StoredBytes
	storedData := data
	truncated := false
	if len(storedData) > remaining {
		storedData = preserveRelayDiagnosticHeadTail(storedData, remaining)
		truncated = true
		side.Truncated = true
	}
	hash := sha256.Sum256([]byte(data))
	side.Events = append(side.Events, relayDiagnosticTraceEvent{
		Sequence:  side.TotalEvents,
		EventType: strings.TrimSpace(eventType),
		Data:      storedData,
		DataBytes: len(data),
		SHA256:    hex.EncodeToString(hash[:]),
		Truncated: truncated,
	})
	side.StoredBytes += len(storedData)
}

func preserveRelayDiagnosticHeadTail(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	const separator = "\n...[truncated]...\n"
	if maxBytes <= len(separator) {
		return validRelayDiagnosticPrefix(value, maxBytes)
	}
	available := maxBytes - len(separator)
	headBytes := available / 2
	tailBytes := available - headBytes
	head := validRelayDiagnosticPrefix(value, headBytes)
	tailStart := len(value) - tailBytes
	for tailStart < len(value) && !utf8.RuneStart(value[tailStart]) {
		tailStart++
	}
	return head + separator + value[tailStart:]
}

func validRelayDiagnosticPrefix(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}

func ClassifyRelayResponseAnomaly(evidence RelayResponseAnomalyEvidence) (bool, string) {
	terminal := strings.ToLower(strings.TrimSpace(evidence.TerminalEvent))
	status := strings.ToLower(strings.Trim(strings.TrimSpace(evidence.ResponseStatus), `"`))
	combinedTerminal := terminal + " " + status

	switch {
	case strings.Contains(combinedTerminal, "failed") || strings.Contains(combinedTerminal, "error"):
		return true, RelayResponseAnomalyFailed
	case strings.Contains(combinedTerminal, "incomplete"):
		return true, RelayResponseAnomalyIncomplete
	case strings.Contains(combinedTerminal, "cancelled") || strings.Contains(combinedTerminal, "canceled"):
		return true, RelayResponseAnomalyCancelled
	case evidence.IsStream && !evidence.TerminalSeen:
		return true, RelayResponseAnomalyTerminalMissing
	}
	if forcedReason := strings.TrimSpace(evidence.ForcedReason); forcedReason != "" {
		return true, forcedReason
	}

	if strings.TrimSpace(evidence.VisibleText) != "" || evidence.HasToolCall || evidence.HasOtherOutput {
		return false, ""
	}

	if strings.TrimSpace(evidence.ReasoningText) != "" || relayUsageReasoningTokens(evidence.EffectiveUsage) > 0 {
		return true, RelayResponseAnomalyReasoningOnly
	}

	promptTokens, completionTokens := relayUsagePromptCompletion(evidence.EffectiveUsage)
	switch {
	case promptTokens == 0 && completionTokens == 0:
		return true, RelayResponseAnomalyUsageZeroZero
	case completionTokens == 0:
		return true, RelayResponseAnomalyOutputTokensZero
	case completionTokens == 1:
		return true, RelayResponseAnomalyOutputTokensOne
	default:
		return true, RelayResponseAnomalyEmptyOutput
	}
}

func CaptureRelayResponseAnomalySnapshot(c *gin.Context, evidence RelayResponseAnomalyEvidence) bool {
	if c == nil || !error_snapshot_setting.GetSnapshot().Enabled {
		return false
	}
	matched, reason := ClassifyRelayResponseAnomaly(evidence)
	if !matched {
		return false
	}

	protocol := strings.TrimSpace(evidence.Protocol)
	if protocol == "" {
		protocol = "unknown"
	}
	usageSource := strings.TrimSpace(evidence.UsageSource)
	if usageSource == "" {
		if common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens) {
			usageSource = "local_estimate"
		} else {
			usageSource = "upstream"
		}
	}
	response := map[string]any{
		"match_reason":             reason,
		"protocol":                 protocol,
		"requested_upstream_model": strings.TrimSpace(evidence.UpstreamModel),
		"response_model":           strings.TrimSpace(evidence.ResponseModel),
		"relay_format":             strings.TrimSpace(evidence.RelayFormat),
		"visible_text":             limitRequestDumpString(strings.TrimSpace(evidence.VisibleText), 4000),
		"visible_text_bytes":       len(evidence.VisibleText),
		"visible_text_sha256":      hashErrorSnapshotString(evidence.VisibleText),
		"reasoning_text":           limitRequestDumpString(strings.TrimSpace(evidence.ReasoningText), 4000),
		"reasoning_text_bytes":     len(evidence.ReasoningText),
		"reasoning_text_sha256":    hashErrorSnapshotString(evidence.ReasoningText),
		"has_tool_call":            evidence.HasToolCall,
		"has_other_output":         evidence.HasOtherOutput,
		"finish_reasons":           evidence.FinishReasons,
		"response_status":          strings.TrimSpace(evidence.ResponseStatus),
		"terminal_event":           strings.TrimSpace(evidence.TerminalEvent),
		"terminal_seen":            evidence.TerminalSeen,
		"raw_usage":                evidence.RawUsage,
		"effective_usage":          evidence.EffectiveUsage,
		"usage_source":             usageSource,
		"details":                  evidence.Details,
	}

	work, err := buildDiagnosticResponseSnapshot(c, diagnosticResponseSnapshotEvidence{
		StatusCode:             http.StatusOK,
		ErrorType:              "relay_response_anomaly",
		ErrorCode:              reason,
		ErrorMessage:           "Relay response anomaly (" + protocol + "): " + reason,
		IsStream:               evidence.IsStream,
		Response:               response,
		UpstreamResponseBody:   evidence.UpstreamResponseBody,
		DownstreamResponseBody: evidence.DownstreamResponseBody,
		Stream:                 evidence.Stream,
	})
	if err != nil {
		errorSnapshotManager.dropped.Add(1)
		recordErrorSnapshotManagerError(err)
		return false
	}
	return enqueueErrorSnapshotWork(work)
}

func buildDiagnosticResponseSnapshot(c *gin.Context, evidence diagnosticResponseSnapshotEvidence) (errorSnapshotWork, error) {
	now := time.Now()
	snapshotID := common.GetUUID()
	channelID := 0
	channelName := ""
	channelType := 0
	if c.GetBool(errorSnapshotChannelSelectedKey) {
		channelID = c.GetInt("channel_id")
		channelName = c.GetString("channel_name")
		channelType = c.GetInt("channel_type")
	}
	requestPath := ""
	method := ""
	contentType := ""
	if c.Request != nil {
		method = c.Request.Method
		contentType = c.Request.Header.Get("Content-Type")
		if c.Request.URL != nil {
			requestPath = c.Request.URL.Path
		}
	}
	statusCode := evidence.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	index := model.ErrorSnapshot{
		ID:             snapshotID,
		CreatedAt:      now.Unix(),
		RequestID:      c.GetString(common.RequestIdKey),
		RequestPath:    requestPath,
		UserID:         c.GetInt("id"),
		Username:       c.GetString("username"),
		ChannelID:      channelID,
		ChannelName:    channelName,
		ModelName:      c.GetString("original_model"),
		AggregateGroup: common.GetContextKeyString(c, constant.ContextKeyAggregateGroup),
		RouteGroup:     common.GetContextKeyString(c, constant.ContextKeyRouteGroup),
		RetryIndex:     c.GetInt(errorSnapshotRetryIndexKey),
		StatusCode:     statusCode,
		ErrorType:      strings.TrimSpace(evidence.ErrorType),
		ErrorCode:      strings.TrimSpace(evidence.ErrorCode),
		ErrorMessage:   limitRequestDumpString(sanitizeErrorSnapshotText(evidence.ErrorMessage), 4000),
		CaptureLevel:   model.ErrorSnapshotCaptureLevelDiagnostic,
		IsStream:       evidence.IsStream,
		InternalRetry:  false,
		FinalOutcome:   ErrorSnapshotOutcomeSuspiciousSuccess,
	}
	envelope := errorSnapshotEnvelope{
		SchemaVersion: 1,
		SnapshotID:    snapshotID,
		CreatedAt:     index.CreatedAt,
		Request: map[string]any{
			"request_id":   index.RequestID,
			"method":       method,
			"path":         requestPath,
			"content_type": contentType,
			"user_id":      index.UserID,
			"username":     index.Username,
			"token_id":     c.GetInt("token_id"),
			"token_name":   c.GetString("token_name"),
			"model":        index.ModelName,
			"is_stream":    index.IsStream,
			"headers":      filterRequestDumpHeaders(requestHeaders(c)),
		},
		Route: map[string]any{
			"channel_id":        index.ChannelID,
			"channel_name":      index.ChannelName,
			"channel_type":      channelType,
			"aggregate_group":   index.AggregateGroup,
			"route_group":       index.RouteGroup,
			"route_group_index": common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex),
			"retry_index":       index.RetryIndex,
		},
		Error: map[string]any{
			"status_code": index.StatusCode,
			"type":        index.ErrorType,
			"code":        index.ErrorCode,
			"message":     index.ErrorMessage,
		},
		Timing: map[string]any{
			"elapsed_ms":              elapsedErrorSnapshotMilliseconds(c, now),
			"first_response_ms":       common.GetContextKeyInt(c, constant.ContextKeyFirstResponseMs),
			"upstream_first_event_ms": common.GetContextKeyInt(c, constant.ContextKeyUpstreamFirstEventMs),
		},
		Response: sanitizeDiagnosticMap(evidence.Response),
	}
	envelope.ClientRequest = captureDiagnosticClientRequestBody(c, contentType)
	if value, ok := c.Get(errorSnapshotUpstreamRequestKey); ok {
		if capture, captureOK := value.(errorSnapshotBodyCapture); captureOK {
			envelope.UpstreamRequest = buildDiagnosticBodyFragment(capture.Body, capture.OriginalSize, capture.ContentType, errorSnapshotMaxBodyFragment)
			envelope.UpstreamRequest.SHA256 = capture.SHA256
			envelope.UpstreamRequest.Truncated = envelope.UpstreamRequest.Truncated || capture.Truncated
			if capture.SkipReason != "" {
				envelope.UpstreamRequest.SkipReason = capture.SkipReason
			}
		}
	}
	if envelope.UpstreamRequest == nil && c.GetBool(errorSnapshotUpstreamPassthroughKey) {
		envelope.UpstreamRequest = captureDiagnosticClientRequestBody(c, contentType)
	}
	if len(evidence.UpstreamResponseBody) > 0 {
		envelope.UpstreamResponse = buildDiagnosticBodyFragment(evidence.UpstreamResponseBody, int64(len(evidence.UpstreamResponseBody)), "application/json", errorSnapshotMaxBodyFragment)
	}
	if len(evidence.DownstreamResponseBody) > 0 {
		envelope.DownstreamResponse = buildDiagnosticBodyFragment(evidence.DownstreamResponseBody, int64(len(evidence.DownstreamResponseBody)), "application/json", errorSnapshotMaxBodyFragment)
	}
	if len(evidence.Stream) > 0 {
		envelope.Stream = sanitizeDiagnosticMap(evidence.Stream)
	}
	payload, err := common.Marshal(envelope)
	if err != nil {
		return errorSnapshotWork{}, err
	}
	boundedPayload, originalSize, truncated, err := boundErrorSnapshotPayload(snapshotID, payload)
	if err != nil {
		return errorSnapshotWork{}, err
	}
	index.OriginalSize = originalSize
	index.PayloadTruncated = truncated
	return errorSnapshotWork{kind: errorSnapshotWorkCapture, index: index, payload: boundedPayload}, nil
}

func relayUsagePromptCompletion(usage *dto.Usage) (int, int) {
	if usage == nil {
		return 0, 0
	}
	promptTokens := usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = usage.InputTokens
	}
	completionTokens := usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = usage.OutputTokens
	}
	return promptTokens, completionTokens
}

func relayUsageReasoningTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.CompletionTokenDetails.ReasoningTokens
}
