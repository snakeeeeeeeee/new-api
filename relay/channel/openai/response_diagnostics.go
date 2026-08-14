package openai

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	openAIChatDiagnosticProtocol      = "openai_chat"
	openAIResponsesDiagnosticProtocol = "openai_responses"
)

type openAIResponseObservation struct {
	VisibleText     strings.Builder
	ReasoningText   strings.Builder
	HasToolCall     bool
	HasOtherOutput  bool
	FinishReasons   []string
	ResponseStatus  string
	TerminalEvent   string
	TerminalSeen    bool
	ResponseModel   string
	RawUsage        *dto.Usage
	UsageSource     string
	TerminalVisible strings.Builder
}

func beginOpenAIResponseDiagnostics(c *gin.Context, protocol string) {
	service.BeginRelayResponseDiagnostics(c, protocol)
}

func recordOpenAIResponseUpstream(c *gin.Context, data string) {
	service.RecordRelayResponseUpstream(c, openAIDiagnosticEventType(data), data)
}

func recordOpenAIResponseDownstream(c *gin.Context, data string) {
	service.RecordRelayResponseDownstream(c, openAIDiagnosticEventType(data), data)
}

func recordOpenAIResponseDownstreamObject(c *gin.Context, value any) {
	data, err := common.Marshal(value)
	if err != nil {
		return
	}
	recordOpenAIResponseDownstream(c, string(data))
}

func openAIDiagnosticEventType(data string) string {
	trimmed := strings.TrimSpace(data)
	if strings.HasPrefix(trimmed, "[DONE]") {
		return "done"
	}
	var event struct {
		Type   string `json:"type"`
		Object string `json:"object"`
	}
	if err := common.UnmarshalJsonStr(trimmed, &event); err != nil {
		return "unknown"
	}
	if event.Type != "" {
		return event.Type
	}
	if event.Object != "" {
		return event.Object
	}
	return "unknown"
}

func (observation *openAIResponseObservation) observeChatStreamData(data string) {
	if observation == nil {
		return
	}
	var response dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &response); err != nil {
		return
	}
	if response.Model != "" {
		observation.ResponseModel = response.Model
	}
	if response.Usage != nil {
		observation.RawUsage = cloneDiagnosticUsage(response.Usage)
	}
	for _, choice := range response.Choices {
		observation.VisibleText.WriteString(choice.Delta.GetContentString())
		observation.ReasoningText.WriteString(choice.Delta.GetReasoningContent())
		if choice.Delta.Thinking != nil {
			observation.ReasoningText.WriteString(*choice.Delta.Thinking)
		}
		if len(choice.Delta.ToolCalls) > 0 {
			observation.HasToolCall = true
		}
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			reason := strings.TrimSpace(*choice.FinishReason)
			observation.FinishReasons = appendUniqueDiagnosticString(observation.FinishReasons, reason)
			observation.TerminalEvent = reason
			observation.TerminalSeen = true
		}
	}
}

func (observation *openAIResponseObservation) observeChatResponse(response *dto.OpenAITextResponse) {
	if observation == nil || response == nil {
		return
	}
	observation.ResponseModel = response.Model
	observation.RawUsage = cloneDiagnosticUsage(&response.Usage)
	for _, choice := range response.Choices {
		observation.VisibleText.WriteString(choice.Message.StringContent())
		observation.ReasoningText.WriteString(choice.Message.ReasoningContent)
		observation.ReasoningText.WriteString(choice.Message.Reasoning)
		if hasOpenAIResponseToolCalls(choice.Message.ToolCalls) {
			observation.HasToolCall = true
		}
		if reason := strings.TrimSpace(choice.FinishReason); reason != "" {
			observation.FinishReasons = appendUniqueDiagnosticString(observation.FinishReasons, reason)
			observation.TerminalEvent = reason
			observation.TerminalSeen = true
		}
	}
}

func (observation *openAIResponseObservation) observeResponsesResponse(response *dto.OpenAIResponsesResponse, terminal bool) {
	if observation == nil || response == nil {
		return
	}
	if response.Model != "" {
		observation.ResponseModel = response.Model
	}
	if response.Usage != nil {
		observation.RawUsage = cloneDiagnosticUsage(response.Usage)
	}
	observation.ResponseStatus = strings.Trim(string(response.Status), `" `)
	for i := range response.Output {
		observation.observeResponsesOutput(&response.Output[i], terminal)
	}
}

func (observation *openAIResponseObservation) observeResponsesOutput(output *dto.ResponsesOutput, terminal bool) {
	if observation == nil || output == nil {
		return
	}
	outputType := strings.TrimSpace(output.Type)
	switch {
	case outputType == "reasoning":
		if len(output.Content) == 0 {
			observation.ReasoningText.WriteString("[reasoning output item]")
		}
		for _, content := range output.Content {
			observation.ReasoningText.WriteString(content.Text)
		}
	case strings.Contains(outputType, "call"):
		observation.HasToolCall = true
	case outputType == "message":
		for _, content := range output.Content {
			contentType := strings.TrimSpace(content.Type)
			if strings.Contains(contentType, "reasoning") {
				observation.ReasoningText.WriteString(content.Text)
				continue
			}
			if content.Text != "" {
				observation.VisibleText.WriteString(content.Text)
				if terminal {
					observation.TerminalVisible.WriteString(content.Text)
				}
			} else if contentType == "refusal" {
				observation.HasOtherOutput = true
			} else if contentType != "" && contentType != "text" && contentType != "output_text" {
				observation.HasOtherOutput = true
			}
		}
	case outputType != "":
		observation.HasOtherOutput = true
	}
}

func (observation *openAIResponseObservation) markResponsesTerminal(eventType string) {
	if observation == nil {
		return
	}
	observation.TerminalEvent = strings.TrimSpace(eventType)
	observation.TerminalSeen = isResponsesTerminalStreamType(eventType)
}

func (observation *openAIResponseObservation) capture(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	protocol string,
	isStream bool,
	effectiveUsage *dto.Usage,
	upstreamResponseBody []byte,
	downstreamResponseBody []byte,
) bool {
	if observation == nil || info == nil {
		return false
	}
	if strings.Contains(strings.ToLower(info.UpstreamModelName), "audio") {
		return false
	}
	forcedReason := ""
	details := map[string]any{}
	if isStream && observation.VisibleText.Len() > 0 && observation.TerminalSeen && observation.TerminalVisible.Len() == 0 && protocol == openAIResponsesDiagnosticProtocol {
		forcedReason = service.RelayResponseAnomalyTerminalOutputMismatch
		details["stream_visible_text_bytes"] = observation.VisibleText.Len()
		details["terminal_visible_text_bytes"] = observation.TerminalVisible.Len()
	}
	if len(details) == 0 {
		details = nil
	}
	usageSource := strings.TrimSpace(observation.UsageSource)
	if usageSource == "" && common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens) {
		usageSource = "local_estimate"
	} else if usageSource == "" && observation.RawUsage == nil {
		usageSource = "unavailable"
	} else if usageSource == "" {
		usageSource = "upstream"
	}
	return service.CaptureRelayResponseAnomalySnapshot(c, service.RelayResponseAnomalyEvidence{
		ForcedReason:           forcedReason,
		Protocol:               protocol,
		UpstreamModel:          info.UpstreamModelName,
		ResponseModel:          observation.ResponseModel,
		RelayFormat:            string(info.RelayFormat),
		IsStream:               isStream,
		VisibleText:            observation.VisibleText.String(),
		ReasoningText:          observation.ReasoningText.String(),
		HasToolCall:            observation.HasToolCall,
		HasOtherOutput:         observation.HasOtherOutput,
		FinishReasons:          observation.FinishReasons,
		ResponseStatus:         observation.ResponseStatus,
		TerminalEvent:          observation.TerminalEvent,
		TerminalSeen:           observation.TerminalSeen,
		RawUsage:               observation.RawUsage,
		EffectiveUsage:         cloneDiagnosticUsage(effectiveUsage),
		UsageSource:            usageSource,
		UpstreamResponseBody:   upstreamResponseBody,
		DownstreamResponseBody: downstreamResponseBody,
		Stream:                 service.RelayResponseDiagnosticsSummary(c),
		Details:                details,
	})
}

func cloneDiagnosticUsage(usage *dto.Usage) *dto.Usage {
	if usage == nil {
		return nil
	}
	cloned := *usage
	return &cloned
}

func hasOpenAIResponseToolCalls(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var calls []any
	if err := common.Unmarshal(raw, &calls); err != nil {
		return false
	}
	return len(calls) > 0
}

func appendUniqueDiagnosticString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func attachOpenAIResponseErrorDiagnostic(apiErr *types.NewAPIError, upstreamBody []byte) *types.NewAPIError {
	if apiErr == nil || len(upstreamBody) == 0 {
		return apiErr
	}
	if apiErr.Diagnostic == nil {
		apiErr.Diagnostic = &types.RelayErrorDiagnostic{}
	}
	apiErr.Diagnostic.UpstreamResponseBody = append([]byte(nil), upstreamBody...)
	apiErr.Diagnostic.UpstreamBodySize = int64(len(upstreamBody))
	return apiErr
}
