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
