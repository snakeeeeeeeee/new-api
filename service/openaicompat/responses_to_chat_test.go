package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

func TestResponsesResponseToChatCompletionsResponseExtractsToolCallsWithText(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_test",
		CreatedAt: 123,
		Model:     "gpt-test",
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type: "output_text",
						Text: "I will call a tool.",
					},
				},
			},
			{
				Type:      "function_call",
				ID:        "fc_1",
				CallId:    "call_1",
				Name:      "lookup",
				Arguments: `{"query":"codex"}`,
			},
		},
	}

	chatResp, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_test")
	require.NoError(t, err)
	require.Len(t, chatResp.Choices, 1)
	require.Equal(t, "tool_calls", chatResp.Choices[0].FinishReason)
	require.Equal(t, "I will call a tool.", chatResp.Choices[0].Message.Content)

	var toolCalls []dto.ToolCallResponse
	require.NoError(t, common.Unmarshal(chatResp.Choices[0].Message.ToolCalls, &toolCalls))
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "lookup", toolCalls[0].Function.Name)
	require.Equal(t, `{"query":"codex"}`, toolCalls[0].Function.Arguments)
}

func TestNormalizeResponsesInputUsageCacheWritePrecedence(t *testing.T) {
	cacheWriteZero := 0
	cacheWriteNonZero := 41

	tests := []struct {
		name                    string
		details                 *dto.InputTokenDetails
		wantCacheCreationTokens int
		wantCacheWriteTokens    *int
		wantCacheWriteReported  bool
	}{
		{
			name: "legacy fallback",
			details: &dto.InputTokenDetails{
				CachedTokens:         12,
				CachedCreationTokens: 37,
			},
			wantCacheCreationTokens: 37,
		},
		{
			name: "official explicit zero overrides legacy",
			details: &dto.InputTokenDetails{
				CachedTokens:         12,
				CachedCreationTokens: 37,
				CacheWriteTokens:     &cacheWriteZero,
			},
			wantCacheCreationTokens: 0,
			wantCacheWriteTokens:    &cacheWriteZero,
			wantCacheWriteReported:  true,
		},
		{
			name: "official non-zero overrides legacy",
			details: &dto.InputTokenDetails{
				CachedTokens:         12,
				CachedCreationTokens: 37,
				CacheWriteTokens:     &cacheWriteNonZero,
			},
			wantCacheCreationTokens: 41,
			wantCacheWriteTokens:    &cacheWriteNonZero,
			wantCacheWriteReported:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &dto.Usage{
				InputTokens:        2000,
				InputTokensDetails: tt.details,
			}
			target := &dto.Usage{}

			NormalizeResponsesInputUsage(target, source)

			require.Equal(t, 2000, target.InputTokens)
			require.Equal(t, "openai_responses", target.UsageSource)
			require.Equal(t, 12, target.PromptTokensDetails.CachedTokens)
			require.Equal(t, tt.wantCacheCreationTokens, target.PromptTokensDetails.CachedCreationTokens)
			if tt.wantCacheWriteReported {
				require.NotNil(t, target.PromptTokensDetails.CacheWriteTokens)
				require.Equal(t, *tt.wantCacheWriteTokens, *target.PromptTokensDetails.CacheWriteTokens)
			} else {
				require.Nil(t, target.PromptTokensDetails.CacheWriteTokens)
			}
		})
	}
}

func TestNormalizeResponsesInputUsagePreservesAnthropicMetadata(t *testing.T) {
	target := &dto.Usage{UsageSemantic: "anthropic", UsageSource: "anthropic"}
	source := &dto.Usage{InputTokens: 123}

	NormalizeResponsesInputUsage(target, source)

	require.Equal(t, "anthropic", target.UsageSemantic)
	require.Equal(t, "anthropic", target.UsageSource)
}

func TestNormalizeResponsesInputUsageDoesNotRetagLegacyClaudeCacheUsage(t *testing.T) {
	target := &dto.Usage{}
	source := &dto.Usage{
		InputTokens:                 123,
		ClaudeCacheCreation5mTokens: 17,
	}

	NormalizeResponsesInputUsage(target, source)

	require.Empty(t, target.UsageSemantic)
	require.Empty(t, target.UsageSource)
}

func TestResponsesResponseToChatCompletionsResponsePreservesCacheWriteUsage(t *testing.T) {
	cacheWriteTokens := 400
	resp := &dto.OpenAIResponsesResponse{
		Model: "gpt-test",
		Usage: &dto.Usage{
			InputTokens:  2000,
			OutputTokens: 100,
			TotalTokens:  2100,
			InputTokensDetails: &dto.InputTokenDetails{
				CachedTokens:         800,
				CachedCreationTokens: 999,
				CacheWriteTokens:     &cacheWriteTokens,
			},
		},
	}

	chatResp, usage, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_test")
	require.NoError(t, err)
	require.Equal(t, 2000, usage.InputTokens)
	require.Equal(t, "openai_responses", usage.UsageSource)
	require.Equal(t, 800, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 400, usage.PromptTokensDetails.CachedCreationTokens)
	require.NotNil(t, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, 400, *usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, usage.PromptTokensDetails, chatResp.Usage.PromptTokensDetails)
}
