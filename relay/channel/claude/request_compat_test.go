package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

func withClaudeRequestCompatSettings(t *testing.T, mutate func(*model_setting.ClaudeSettings)) {
	t.Helper()
	original := *model_setting.GetClaudeSettings()
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
	})
	mutate(model_setting.GetClaudeSettings())
}

func TestRequestOpenAI2ClaudeMessagePreservesZeroMaxTokens(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PreserveZeroMaxTokensEnabled = true
		settings.DefaultMaxTokens = map[string]int{"default": 8192}
	})
	zero := uint(0)
	req := dto.GeneralOpenAIRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: &zero,
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.MaxTokens)
	require.Equal(t, uint(0), *claudeReq.MaxTokens)
}

func TestRequestOpenAI2ClaudeMessagePromotesOnlyLeadingSystemAndDeveloper(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
	})
	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "system", Content: "leading system"},
			{Role: "developer", Content: "leading developer"},
			{Role: "user", Content: "first"},
			{Role: "system", Content: "middle system"},
			{Role: "assistant", Content: "answer"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	system, ok := claudeReq.System.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, system, 2)
	require.Equal(t, "leading system", system[0].GetText())
	require.Equal(t, "leading developer", system[1].GetText())
	require.Len(t, claudeReq.Messages, 3)
	require.Equal(t, "user", claudeReq.Messages[0].Role)
	require.Equal(t, "first", claudeReq.Messages[0].Content)
	require.Equal(t, "user", claudeReq.Messages[1].Role)
	require.Equal(t, "system: middle system", claudeReq.Messages[1].Content)
}

func TestRequestOpenAI2ClaudeMessageMergesAdjacentUserAndAssistantOnly(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.PromoteLeadingSystemRoleEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
	})
	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "a"},
			{Role: "user", Content: "b"},
			{Role: "assistant", Content: "c"},
			{Role: "assistant", Content: "d"},
			{Role: "tool", ToolCallId: "call_1", Content: "tool result"},
			{Role: "tool", ToolCallId: "call_2", Content: "tool result 2"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.Len(t, claudeReq.Messages, 4)
	require.Equal(t, "a b", claudeReq.Messages[0].Content)
	require.Equal(t, "c d", claudeReq.Messages[1].Content)
	require.Equal(t, "user", claudeReq.Messages[2].Role)
	require.Equal(t, "user", claudeReq.Messages[3].Role)
}
