package openaicompat

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

const openAIResponsesUsageSource = "openai_responses"

// NormalizeResponsesInputUsage preserves Responses API input usage metadata
// while mapping its token details to the common prompt-token representation.
func NormalizeResponsesInputUsage(target *dto.Usage, source *dto.Usage) {
	if target == nil || source == nil {
		return
	}

	target.InputTokens = source.InputTokens
	if target.UsageSemantic == "" {
		target.UsageSemantic = source.UsageSemantic
	}
	if target.UsageSource == "" {
		target.UsageSource = source.UsageSource
	}
	legacyClaudeCacheUsage := source.ClaudeCacheCreation5mTokens > 0 || source.ClaudeCacheCreation1hTokens > 0
	if target.UsageSource == "" && target.UsageSemantic != "anthropic" && !legacyClaudeCacheUsage {
		target.UsageSource = openAIResponsesUsageSource
	}

	if source.InputTokensDetails == nil {
		return
	}
	cacheCreationTokens, _ := source.InputTokensDetails.ResolveCacheCreationTokens()
	target.PromptTokensDetails.CachedTokens = source.InputTokensDetails.CachedTokens
	target.PromptTokensDetails.CachedCreationTokens = cacheCreationTokens
	target.PromptTokensDetails.CacheWriteTokens = source.InputTokensDetails.CacheWriteTokens
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	text := ExtractOutputTextFromResponses(resp)

	usage := &dto.Usage{}
	if resp.Usage != nil {
		if resp.Usage.InputTokens != 0 {
			usage.PromptTokens = resp.Usage.InputTokens
			usage.InputTokens = resp.Usage.InputTokens
		}
		if resp.Usage.OutputTokens != 0 {
			usage.CompletionTokens = resp.Usage.OutputTokens
			usage.OutputTokens = resp.Usage.OutputTokens
		}
		if resp.Usage.TotalTokens != 0 {
			usage.TotalTokens = resp.Usage.TotalTokens
		} else {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		NormalizeResponsesInputUsage(usage, resp.Usage)
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.ImageTokens = resp.Usage.InputTokensDetails.ImageTokens
			usage.PromptTokensDetails.AudioTokens = resp.Usage.InputTokensDetails.AudioTokens
		}
		if resp.Usage.CompletionTokenDetails.ReasoningTokens != 0 {
			usage.CompletionTokenDetails.ReasoningTokens = resp.Usage.CompletionTokenDetails.ReasoningTokens
		}
	}

	created := resp.CreatedAt

	var toolCalls []dto.ToolCallResponse
	for _, out := range resp.Output {
		if out.Type != "function_call" {
			continue
		}
		name := strings.TrimSpace(out.Name)
		if name == "" {
			continue
		}
		callId := strings.TrimSpace(out.CallId)
		if callId == "" {
			callId = strings.TrimSpace(out.ID)
		}
		toolCalls = append(toolCalls, dto.ToolCallResponse{
			ID:   callId,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      name,
				Arguments: out.Arguments,
			},
		})
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	msg := dto.Message{
		Role:    "assistant",
		Content: text,
	}
	if len(toolCalls) > 0 {
		msg.SetToolCalls(toolCalls)
	}

	out := &dto.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   resp.Model,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			},
		},
		Usage: *usage,
	}

	return out, usage, nil
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || len(resp.Output) == 0 {
		return ""
	}

	var sb strings.Builder

	// Prefer assistant message outputs.
	for _, out := range resp.Output {
		if out.Type != "message" {
			continue
		}
		if out.Role != "" && out.Role != "assistant" {
			continue
		}
		for _, c := range out.Content {
			if c.Type == "output_text" && c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	if sb.Len() > 0 {
		return sb.String()
	}
	for _, out := range resp.Output {
		for _, c := range out.Content {
			if c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
	}
	return sb.String()
}
