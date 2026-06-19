package claude

import (
	"testing"

	commonpkg "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
		settings.OpenAIToolCallCompatEnabled = false
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

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatPreservesNullContentToolCalls(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "check"},
			openAIToolCallMessage(nil, "call_1", "lookup", `{"q":"status"}`),
			{Role: "tool", ToolCallId: "call_1", Content: "ok"},
			{Role: "user", Content: "done"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.Nil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))
	require.Len(t, claudeReq.Messages, 4)

	contents, err := claudeReq.Messages[1].ParseContent()
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "tool_use", contents[0].Type)
	require.Equal(t, "call_1", contents[0].Id)
	require.Equal(t, map[string]any{"q": "status"}, contents[0].Input)
}

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatMergesAssistantTextAndToolCalls(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "check"},
			{Role: "assistant", Content: "Let me check."},
			openAIToolCallMessage(nil, "call_1", "lookup", `{"q":"port"}`),
			{Role: "tool", ToolCallId: "call_1", Content: "3000"},
			{Role: "user", Content: "done"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.Nil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))
	require.Len(t, claudeReq.Messages, 4)
	require.Equal(t, "assistant", claudeReq.Messages[1].Role)

	contents, err := claudeReq.Messages[1].ParseContent()
	require.NoError(t, err)
	require.Len(t, contents, 2)
	require.Equal(t, "text", contents[0].Type)
	require.Equal(t, "Let me check.", contents[0].GetText())
	require.Equal(t, "tool_use", contents[1].Type)
	require.Equal(t, "call_1", contents[1].Id)
}

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatWrapsInvalidArguments(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	rawArguments := `{"code" title="Open page": "bad json"}`
	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "check"},
			openAIToolCallMessage(nil, "call_1", "exec", rawArguments),
			{Role: "tool", ToolCallId: "call_1", Content: "ok"},
			{Role: "user", Content: "done"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.Nil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))

	contents, err := claudeReq.Messages[1].ParseContent()
	require.NoError(t, err)
	require.Equal(t, "tool_use", contents[0].Type)
	require.Equal(t, map[string]any{"_raw_arguments": rawArguments}, contents[0].Input)
}

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatWrapsNonObjectArguments(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	for name, arguments := range map[string]string{
		"array":  `[1,2]`,
		"string": `"abc"`,
		"number": `123`,
		"bool":   `true`,
		"null":   `null`,
	} {
		t.Run(name, func(t *testing.T) {
			req := dto.GeneralOpenAIRequest{
				Model: "claude-sonnet-4-6",
				Messages: []dto.Message{
					{Role: "user", Content: "check"},
					openAIToolCallMessage(nil, "call_1", "exec", arguments),
					{Role: "tool", ToolCallId: "call_1", Content: "ok"},
					{Role: "user", Content: "done"},
				},
			}

			claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
			require.NoError(t, err)
			require.Nil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))
			contents, err := claudeReq.Messages[1].ParseContent()
			require.NoError(t, err)
			input, ok := contents[0].Input.(map[string]any)
			require.True(t, ok)
			require.Contains(t, input, "value")
		})
	}
}

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatMergesContinuousToolResults(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = true
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "check"},
			openAIToolCallMessage(nil, "call_1", "lookup", `{"q":"a"}`, extraOpenAIToolCall("call_2", "lookup", `{"q":"b"}`)),
			{Role: "tool", ToolCallId: "call_1", Content: "a"},
			{Role: "tool", ToolCallId: "call_2", Content: "b"},
			{Role: "user", Content: "done"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.Nil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))
	require.Len(t, claudeReq.Messages, 4)

	resultContents, err := claudeReq.Messages[2].ParseContent()
	require.NoError(t, err)
	require.Len(t, resultContents, 2)
	require.Equal(t, "call_1", resultContents[0].ToolUseId)
	require.Equal(t, "call_2", resultContents[1].ToolUseId)
}

func TestRequestOpenAI2ClaudeMessageOpenAIToolCallCompatDisabledKeepsOldInvalidArgumentBehavior(t *testing.T) {
	withClaudeRequestCompatSettings(t, func(settings *model_setting.ClaudeSettings) {
		settings.OpenAIToolCallCompatEnabled = false
		settings.MergeAdjacentSameRoleEnabled = true
		settings.ToolProtocolValidationMode = model_setting.ClaudeValidationModeReject
	})

	req := dto.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-6",
		Messages: []dto.Message{
			{Role: "user", Content: "check"},
			openAIToolCallMessage(nil, "call_1", "exec", `{"code" title="Open page": "bad json"}`),
			{Role: "tool", ToolCallId: "call_1", Content: "ok"},
		},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.NotNil(t, relaycommon.NormalizeClaudeRequestCompat(claudeReq, nil))
}

func openAIToolCallMessage(content any, id string, name string, arguments string, extra ...map[string]any) dto.Message {
	toolCalls := []map[string]any{extraOpenAIToolCall(id, name, arguments)}
	toolCalls = append(toolCalls, extra...)
	body, _ := commonpkg.Marshal(toolCalls)
	return dto.Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: body,
	}
}

func extraOpenAIToolCall(id string, name string, arguments string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	}
}

func TestRequestOpenAI2ClaudeMessagePassesTopLevelThinking(t *testing.T) {
	maxTokens := uint(4000)
	req := dto.GeneralOpenAIRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: &maxTokens,
		THINKING:  []byte(`{"type":"enabled","budget_tokens":2000}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	require.Equal(t, "enabled", claudeReq.Thinking.Type)
	require.NotNil(t, claudeReq.Thinking.BudgetTokens)
	require.Equal(t, 2000, *claudeReq.Thinking.BudgetTokens)
}

func TestRequestOpenAI2ClaudeMessagePassesAdaptiveThinking(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:    "claude-sonnet-4-6",
		THINKING: []byte(`{"type":"adaptive"}`),
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	require.Equal(t, "adaptive", claudeReq.Thinking.Type)
	require.Nil(t, claudeReq.Thinking.BudgetTokens)
}

func TestRequestOpenAI2ClaudeMessageTopLevelThinkingOverridesReasoning(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:     "claude-sonnet-4-6",
		Reasoning: []byte(`{"max_tokens":2048}`),
		THINKING:  []byte(`{"type":"enabled","budget_tokens":1024}`),
		Messages:  []dto.Message{{Role: "user", Content: "hello"}},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	require.Equal(t, "enabled", claudeReq.Thinking.Type)
	require.NotNil(t, claudeReq.Thinking.BudgetTokens)
	require.Equal(t, 1024, *claudeReq.Thinking.BudgetTokens)
}

func TestRequestOpenAI2ClaudeMessagePassesOutputConfig(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:        "claude-fable-5",
		OutputConfig: []byte(`{"effort":"max"}`),
		Messages:     []dto.Message{{Role: "user", Content: "hello"}},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), req)
	require.NoError(t, err)
	require.JSONEq(t, `{"effort":"max"}`, string(claudeReq.OutputConfig))
}

func TestResponseClaude2OpenAIIncludesReasoningContent(t *testing.T) {
	thinking := "careful reasoning"
	signature := "signed-thinking"
	resp := ResponseClaude2OpenAI(&dto.ClaudeResponse{
		Id:         "msg_test",
		Model:      "claude-sonnet-4-6",
		StopReason: "end_turn",
		Content: []dto.ClaudeMediaMessage{
			{Type: "thinking", Thinking: commonpkg.GetPointer(thinking), Signature: signature},
			{Type: "text", Text: commonpkg.GetPointer("final answer")},
		},
	})

	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	require.Equal(t, "final answer", resp.Choices[0].Message.StringContent())
	require.Equal(t, thinking, resp.Choices[0].Message.ReasoningContent)
	require.Equal(t, signature, resp.Choices[0].Message.ReasoningSignature)
	require.Equal(t, thinking, resp.Choices[0].Message.Thinking)
	require.Equal(t, signature, resp.Choices[0].Message.Signature)
}

func TestStreamResponseClaude2OpenAIIncludesReasoningSignature(t *testing.T) {
	signature := "stream-signature"
	resp := StreamResponseClaude2OpenAI(&dto.ClaudeResponse{
		Id:    "msg_test",
		Model: "claude-sonnet-4-6",
		Type:  "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Type:      "signature_delta",
			Signature: signature,
		},
	})

	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Delta.ReasoningSignature)
	require.Equal(t, signature, *resp.Choices[0].Delta.ReasoningSignature)
	require.NotNil(t, resp.Choices[0].Delta.Signature)
	require.Equal(t, signature, *resp.Choices[0].Delta.Signature)
}

func TestStreamResponseClaude2OpenAIIncludesThinkingAlias(t *testing.T) {
	thinking := "stream-thinking"
	resp := StreamResponseClaude2OpenAI(&dto.ClaudeResponse{
		Id:    "msg_test",
		Model: "claude-sonnet-4-6",
		Type:  "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Type:     "thinking_delta",
			Thinking: commonpkg.GetPointer(thinking),
		},
	})

	require.NotNil(t, resp)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Delta.ReasoningContent)
	require.Equal(t, thinking, *resp.Choices[0].Delta.ReasoningContent)
	require.NotNil(t, resp.Choices[0].Delta.Thinking)
	require.Equal(t, thinking, *resp.Choices[0].Delta.Thinking)
}
