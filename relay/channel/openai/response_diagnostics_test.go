package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResponseObservationSeparatesVisibleReasoningAndTools(t *testing.T) {
	observation := &openAIResponseObservation{}
	observation.observeChatStreamData(`{"model":"gpt-test","choices":[{"delta":{"reasoning_content":"hidden"},"finish_reason":null}],"usage":null}`)
	observation.observeChatStreamData(`{"model":"gpt-test","choices":[{"delta":{"content":"visible"},"finish_reason":null}]}`)
	observation.observeChatStreamData(`{"model":"gpt-test","choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9,"completion_tokens":1,"total_tokens":10}}`)

	require.Equal(t, "visible", observation.VisibleText.String())
	require.Equal(t, "hidden", observation.ReasoningText.String())
	require.True(t, observation.HasToolCall)
	require.True(t, observation.TerminalSeen)
	require.Equal(t, "tool_calls", observation.TerminalEvent)
	require.Equal(t, 1, observation.RawUsage.CompletionTokens)
}

func TestOpenAIResponseObservationSeparatesStreamAndTerminalOutput(t *testing.T) {
	observation := &openAIResponseObservation{}
	observation.VisibleText.WriteString("delta text")

	var completed dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal([]byte(`{"status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":8,"output_tokens":1,"total_tokens":9}}`), &completed))
	observation.observeResponsesResponse(&completed, true)
	observation.markResponsesTerminal("response.completed")

	require.Equal(t, "delta text", observation.VisibleText.String())
	require.Empty(t, observation.TerminalVisible.String())
	require.True(t, observation.TerminalSeen)
	require.Equal(t, "completed", observation.ResponseStatus)
	require.Equal(t, 1, observation.RawUsage.OutputTokens)
}

func TestOpenAIResponseObservationDoesNotTreatEmptyToolArrayAsOutput(t *testing.T) {
	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal([]byte(`{"choices":[{"message":{"content":"","tool_calls":[]},"finish_reason":"stop"}]}`), &response))

	observation := &openAIResponseObservation{}
	observation.observeChatResponse(&response)

	require.False(t, observation.HasToolCall)
}

func TestOpenAIResponseObservationTreatsReasoningAsNonFinalAndFunctionAsMeaningful(t *testing.T) {
	observation := &openAIResponseObservation{}
	reasoning := dto.ResponsesOutput{Type: "reasoning"}
	observation.observeResponsesOutput(&reasoning, false)
	require.NotEmpty(t, observation.ReasoningText.String())
	require.False(t, observation.HasOtherOutput)

	functionCall := dto.ResponsesOutput{Type: "function_call"}
	observation.observeResponsesOutput(&functionCall, false)
	require.True(t, observation.HasToolCall)

	otherOutput := dto.ResponsesOutput{
		Type:    "message",
		Content: []dto.ResponsesOutputContent{{Type: "output_image"}},
	}
	observation.observeResponsesOutput(&otherOutput, false)
	require.True(t, observation.HasOtherOutput)
}
