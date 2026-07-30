package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	claudeDiagnosticMaxEvents       = 256
	claudeDiagnosticMaxBytesPerSide = 16 << 10
)

type claudeDiagnosticTraceEvent struct {
	Sequence  int    `json:"sequence"`
	EventType string `json:"event_type,omitempty"`
	Data      string `json:"data,omitempty"`
	DataBytes int    `json:"data_bytes"`
	SHA256    string `json:"sha256"`
	Truncated bool   `json:"truncated,omitempty"`
}

type claudeDiagnosticTraceSide struct {
	Events      []claudeDiagnosticTraceEvent
	TotalEvents int
	StoredBytes int
	Truncated   bool
}

type claudeResponseDiagnostics struct {
	RequestedUpstreamModel string
	Upstream               claudeDiagnosticTraceSide
	Downstream             claudeDiagnosticTraceSide
}

func newClaudeResponseDiagnostics(upstreamModel string) *claudeResponseDiagnostics {
	if !service.SuspiciousResponseCaptureEnabled() {
		return nil
	}
	return &claudeResponseDiagnostics{
		RequestedUpstreamModel: strings.TrimSpace(upstreamModel),
	}
}

func (d *claudeResponseDiagnostics) recordUpstream(eventType, data string) {
	if d == nil {
		return
	}
	recordClaudeDiagnosticTrace(&d.Upstream, eventType, data)
}

func (d *claudeResponseDiagnostics) recordDownstream(eventType, data string) {
	if d == nil {
		return
	}
	recordClaudeDiagnosticTrace(&d.Downstream, eventType, data)
}

func (d *claudeResponseDiagnostics) summary() map[string]any {
	if d == nil {
		return nil
	}
	return map[string]any{
		"upstream": map[string]any{
			"events":       d.Upstream.Events,
			"total_events": d.Upstream.TotalEvents,
			"stored_bytes": d.Upstream.StoredBytes,
			"truncated":    d.Upstream.Truncated,
		},
		"downstream": map[string]any{
			"events":       d.Downstream.Events,
			"total_events": d.Downstream.TotalEvents,
			"stored_bytes": d.Downstream.StoredBytes,
			"truncated":    d.Downstream.Truncated,
		},
	}
}

func recordClaudeDiagnosticTrace(side *claudeDiagnosticTraceSide, eventType, data string) {
	if side == nil {
		return
	}
	side.TotalEvents++
	if len(side.Events) >= claudeDiagnosticMaxEvents || side.StoredBytes >= claudeDiagnosticMaxBytesPerSide {
		side.Truncated = true
		return
	}
	remaining := claudeDiagnosticMaxBytesPerSide - side.StoredBytes
	storedData := data
	truncated := false
	if len(storedData) > remaining {
		storedData = preserveClaudeDiagnosticHeadTail(storedData, remaining)
		truncated = true
		side.Truncated = true
	}
	hash := sha256.Sum256([]byte(data))
	side.Events = append(side.Events, claudeDiagnosticTraceEvent{
		Sequence:  side.TotalEvents,
		EventType: strings.TrimSpace(eventType),
		Data:      storedData,
		DataBytes: len(data),
		SHA256:    hex.EncodeToString(hash[:]),
		Truncated: truncated,
	})
	side.StoredBytes += len(storedData)
}

func preserveClaudeDiagnosticHeadTail(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	const separator = "\n...[truncated]...\n"
	if maxBytes <= len(separator) {
		return validClaudeDiagnosticPrefix(value, maxBytes)
	}
	available := maxBytes - len(separator)
	headBytes := available / 2
	tailBytes := available - headBytes
	head := validClaudeDiagnosticPrefix(value, headBytes)
	tailStart := len(value) - tailBytes
	for tailStart < len(value) && !utf8.RuneStart(value[tailStart]) {
		tailStart++
	}
	return head + separator + value[tailStart:]
}

func validClaudeDiagnosticPrefix(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}

func captureSuspiciousClaudeResponse(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	claudeInfo *ClaudeResponseInfo,
	upstreamResponseBody []byte,
) {
	if c == nil || info == nil || claudeInfo == nil {
		return
	}
	if claudeInfo.Diagnostics == nil {
		return
	}
	if matched, _ := service.MatchSuspiciousClaudeResponse(claudeInfo.VisibleResponseText.String()); !matched {
		return
	}
	responseModel := strings.TrimSpace(claudeInfo.Model)
	if responseModel == "" {
		responseModel = strings.TrimSpace(info.UpstreamModelName)
	}
	upstreamModel := strings.TrimSpace(claudeInfo.Diagnostics.RequestedUpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	service.CaptureSuspiciousClaudeResponseSnapshot(c, service.SuspiciousClaudeResponseEvidence{
		VisibleText:          claudeInfo.VisibleResponseText.String(),
		UpstreamModel:        upstreamModel,
		ResponseModel:        responseModel,
		StopReason:           strings.TrimSpace(claudeInfo.StopReason),
		RelayFormat:          string(info.RelayFormat),
		IsStream:             info.IsStream,
		ContentBlockTypes:    append([]string(nil), claudeInfo.ContentBlockTypes...),
		Usage:                claudeDiagnosticUsage(claudeInfo.Usage),
		UpstreamResponseBody: append([]byte(nil), upstreamResponseBody...),
		Stream:               claudeInfo.Diagnostics.summary(),
	})
}

func observeClaudeNonStreamResponse(
	info *relaycommon.RelayInfo,
	claudeInfo *ClaudeResponseInfo,
	response *dto.ClaudeResponse,
) {
	if claudeInfo == nil || response == nil {
		return
	}
	if claudeInfo.Diagnostics == nil {
		return
	}
	claudeInfo.Model = strings.TrimSpace(response.Model)
	claudeInfo.StopReason = strings.TrimSpace(response.StopReason)
	var visibleText string
	for _, block := range response.Content {
		claudeInfo.ContentBlockTypes = appendClaudeContentBlockType(claudeInfo.ContentBlockTypes, block.Type)
		if block.Type != "text" {
			continue
		}
		if info != nil && info.RelayFormat == types.RelayFormatOpenAI {
			visibleText = block.GetText()
			continue
		}
		visibleText += block.GetText()
	}
	claudeInfo.VisibleResponseText.WriteString(visibleText)
}

func appendClaudeContentBlockType(types []string, blockType string) []string {
	blockType = strings.TrimSpace(blockType)
	if blockType == "" {
		return types
	}
	for _, existing := range types {
		if existing == blockType {
			return types
		}
	}
	return append(types, blockType)
}

func claudeDiagnosticUsage(usage *dto.Usage) map[string]any {
	if usage == nil {
		return nil
	}
	return map[string]any{
		"prompt_tokens":                  usage.PromptTokens,
		"completion_tokens":              usage.CompletionTokens,
		"total_tokens":                   usage.TotalTokens,
		"cache_read_input_tokens":        usage.PromptTokensDetails.CachedTokens,
		"cache_creation_input_tokens":    usage.PromptTokensDetails.CachedCreationTokens,
		"cache_creation_5m_input_tokens": usage.ClaudeCacheCreation5mTokens,
		"cache_creation_1h_input_tokens": usage.ClaudeCacheCreation1hTokens,
		"usage_semantic":                 usage.UsageSemantic,
	}
}
