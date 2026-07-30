package service

import (
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"

	"github.com/gin-gonic/gin"
)

const (
	SuspiciousResponseTypeClaudeIdleGreeting = "claude_idle_greeting"
	suspiciousClaudeResponseMaxRunes         = 320
)

var suspiciousClaudeIdleGreetingPattern = regexp.MustCompile(
	`^(?:(?:great|sure|okay|ok|thanks|thank you)[\s,.!:\-]*)*i(?:'m| am) ready[\s,.!:\-]*what would you like me to (?:(?:work on)|(?:do))(?: next)?[\s?.!]*$`,
)

type SuspiciousClaudeResponseEvidence struct {
	VisibleText          string
	UpstreamModel        string
	ResponseModel        string
	StopReason           string
	RelayFormat          string
	IsStream             bool
	ContentBlockTypes    []string
	Usage                map[string]any
	UpstreamResponseBody []byte
	Stream               map[string]any
}

func SuspiciousResponseCaptureEnabled() bool {
	return error_snapshot_setting.GetSnapshot().Enabled
}

func MatchSuspiciousClaudeResponse(text string) (bool, string) {
	normalized := normalizeSuspiciousClaudeResponse(text)
	if normalized == "" || utf8.RuneCountInString(normalized) > suspiciousClaudeResponseMaxRunes {
		return false, ""
	}
	if suspiciousClaudeIdleGreetingPattern.MatchString(normalized) {
		return true, SuspiciousResponseTypeClaudeIdleGreeting
	}
	return false, ""
}

func CaptureSuspiciousClaudeResponseSnapshot(c *gin.Context, evidence SuspiciousClaudeResponseEvidence) bool {
	if c == nil || !SuspiciousResponseCaptureEnabled() {
		return false
	}
	matched, reason := MatchSuspiciousClaudeResponse(evidence.VisibleText)
	if !matched {
		return false
	}
	work, err := buildSuspiciousClaudeResponseSnapshot(c, evidence, reason)
	if err != nil {
		errorSnapshotManager.dropped.Add(1)
		recordErrorSnapshotManagerError(err)
		return false
	}
	return enqueueErrorSnapshotWork(work)
}

func normalizeSuspiciousClaudeResponse(text string) string {
	text = strings.NewReplacer(
		"’", "'",
		"‘", "'",
		"—", "-",
		"–", "-",
	).Replace(strings.TrimSpace(text))
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func buildSuspiciousClaudeResponseSnapshot(c *gin.Context, evidence SuspiciousClaudeResponseEvidence, reason string) (errorSnapshotWork, error) {
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
	responseText := limitRequestDumpString(strings.TrimSpace(evidence.VisibleText), 1000)
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
		StatusCode:     http.StatusOK,
		ErrorType:      "suspicious_success",
		ErrorCode:      reason,
		ErrorMessage:   limitRequestDumpString(sanitizeErrorSnapshotText("Suspicious successful Claude response: "+responseText), 4000),
		CaptureLevel:   model.ErrorSnapshotCaptureLevelDiagnostic,
		IsStream:       evidence.IsStream,
		InternalRetry:  false,
		FinalOutcome:   ErrorSnapshotOutcomeSuspiciousSuccess,
	}
	response := sanitizeDiagnosticMap(map[string]any{
		"match_reason":             reason,
		"visible_text":             responseText,
		"visible_text_sha256":      hashErrorSnapshotString(evidence.VisibleText),
		"visible_text_bytes":       len(evidence.VisibleText),
		"requested_upstream_model": strings.TrimSpace(evidence.UpstreamModel),
		"response_model":           strings.TrimSpace(evidence.ResponseModel),
		"stop_reason":              strings.TrimSpace(evidence.StopReason),
		"relay_format":             strings.TrimSpace(evidence.RelayFormat),
		"content_block_types":      evidence.ContentBlockTypes,
		"usage":                    evidence.Usage,
	})
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
			"status_code": http.StatusOK,
			"type":        index.ErrorType,
			"code":        index.ErrorCode,
			"message":     index.ErrorMessage,
		},
		Timing: map[string]any{
			"elapsed_ms":              elapsedErrorSnapshotMilliseconds(c, now),
			"first_response_ms":       common.GetContextKeyInt(c, constant.ContextKeyFirstResponseMs),
			"upstream_first_event_ms": common.GetContextKeyInt(c, constant.ContextKeyUpstreamFirstEventMs),
		},
		Response: response,
	}
	envelope.ClientRequest = captureDiagnosticClientRequestBody(c, contentType)
	if value, ok := c.Get(errorSnapshotUpstreamRequestKey); ok {
		if capture, captureOK := value.(errorSnapshotBodyCapture); captureOK {
			envelope.UpstreamRequest = buildDiagnosticBodyFragment(capture.Body, capture.OriginalSize, capture.ContentType, errorSnapshotMaxBodyFragment)
			envelope.UpstreamRequest.SHA256 = capture.SHA256
			envelope.UpstreamRequest.Truncated = envelope.UpstreamRequest.Truncated || capture.Truncated
		}
	}
	if envelope.UpstreamRequest == nil && c.GetBool(errorSnapshotUpstreamPassthroughKey) {
		envelope.UpstreamRequest = captureDiagnosticClientRequestBody(c, contentType)
	}
	if len(evidence.UpstreamResponseBody) > 0 {
		envelope.UpstreamResponse = buildDiagnosticBodyFragment(
			evidence.UpstreamResponseBody,
			int64(len(evidence.UpstreamResponseBody)),
			"application/json",
			errorSnapshotMaxResponseFragment,
		)
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

func captureDiagnosticClientRequestBody(c *gin.Context, contentType string) *errorSnapshotBodyFragment {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return &errorSnapshotBodyFragment{ContentType: contentType, SkipReason: "read_body_failed"}
	}
	if isUnsupportedRequestDumpContentType(contentType) {
		return captureClientRequestBody(c, contentType)
	}
	body, err := storage.Bytes()
	if err != nil {
		return &errorSnapshotBodyFragment{ContentType: contentType, OriginalSize: storage.Size(), SkipReason: "read_body_failed"}
	}
	diagnosticBody, truncated := buildClaudeDiagnosticRequestBody(body, errorSnapshotMaxBodyFragment)
	fragment := buildDiagnosticBodyFragment(diagnosticBody, storage.Size(), contentType, errorSnapshotMaxBodyFragment)
	fragment.SHA256 = hashErrorSnapshotBytes(body)
	fragment.Truncated = fragment.Truncated || truncated
	return fragment
}

func buildDiagnosticBodyFragment(body []byte, originalSize int64, contentType string, maxBytes int) *errorSnapshotBodyFragment {
	fragment := &errorSnapshotBodyFragment{
		ContentType:  contentType,
		OriginalSize: originalSize,
		SHA256:       hashErrorSnapshotBytes(body),
		Truncated:    len(body) > maxBytes,
	}
	if len(body) == 0 {
		return fragment
	}
	if !utf8.Valid(body) {
		fragment.SkipReason = "binary_content"
		return fragment
	}
	sanitized := sanitizeErrorSnapshotBody(body)
	fragment.Body = string(preserveBodyHeadTail(sanitized, maxBytes))
	fragment.Truncated = fragment.Truncated || len(sanitized) > maxBytes
	return fragment
}

func sanitizeDiagnosticMap(value map[string]any) map[string]any {
	data, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	var sanitized map[string]any
	if err = common.Unmarshal(sanitizeErrorSnapshotBody(data), &sanitized); err != nil {
		return nil
	}
	return sanitizeDiagnosticStrings(sanitized).(map[string]any)
}

type claudeDiagnosticRequest struct {
	Model      any            `json:"model,omitempty"`
	System     any            `json:"system,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Tools      any            `json:"tools,omitempty"`
	Messages   any            `json:"messages,omitempty"`
}

func buildClaudeDiagnosticRequestBody(body []byte, maxBytes int) ([]byte, bool) {
	if len(body) <= maxBytes {
		return sanitizeErrorSnapshotBody(body), false
	}
	var request map[string]any
	if err := common.Unmarshal(body, &request); err != nil {
		sanitized := []byte(sanitizeErrorSnapshotText(string(body)))
		return preserveBodyHeadTail(sanitized, maxBytes), true
	}
	masked, ok := maskErrorSnapshotValue(request, "").(map[string]any)
	if !ok {
		sanitized := sanitizeErrorSnapshotBody(body)
		return preserveBodyHeadTail(sanitized, maxBytes), true
	}
	parameters := make(map[string]any)
	for key, value := range masked {
		switch key {
		case "model", "system", "messages", "tools":
			continue
		}
		if data, err := common.Marshal(value); err == nil && len(data) <= 2048 {
			parameters[key] = value
		}
	}
	diagnostic := claudeDiagnosticRequest{
		Model:      masked["model"],
		System:     summarizeDiagnosticJSONValue(masked["system"], 6<<10),
		Parameters: parameters,
		Tools:      summarizeDiagnosticJSONValue(masked["tools"], 4<<10),
		Messages:   summarizeDiagnosticJSONValue(masked["messages"], 16<<10),
	}
	data, err := common.Marshal(diagnostic)
	if err != nil {
		sanitized := sanitizeErrorSnapshotBody(body)
		return preserveBodyHeadTail(sanitized, maxBytes), true
	}
	return preserveBodyHeadTail(data, maxBytes), true
}

func summarizeDiagnosticJSONValue(value any, maxBytes int) any {
	if value == nil {
		return nil
	}
	data, err := common.Marshal(value)
	if err != nil {
		return map[string]any{"skip_reason": "marshal_failed"}
	}
	if len(data) <= maxBytes {
		return value
	}
	return map[string]any{
		"original_size": len(data),
		"sha256":        hashErrorSnapshotBytes(data),
		"fragment":      string(preserveBodyHeadTail(data, maxBytes)),
		"truncated":     true,
	}
}

func sanitizeDiagnosticStrings(value any) any {
	switch typed := value.(type) {
	case string:
		return sanitizeErrorSnapshotText(typed)
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, child := range typed {
			sanitized[key] = sanitizeDiagnosticStrings(child)
		}
		return sanitized
	case []any:
		sanitized := make([]any, len(typed))
		for index, child := range typed {
			sanitized[index] = sanitizeDiagnosticStrings(child)
		}
		return sanitized
	default:
		return value
	}
}
