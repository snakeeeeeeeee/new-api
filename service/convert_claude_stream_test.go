package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func newClaudeStreamConvertInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
}

func toolChunk(index int, id string, name string, args string, finishReason *string, usage *dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl-test",
		Model: "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: finishReason,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							Index: common.GetPointer(index),
							ID:    id,
							Type:  "function",
							Function: dto.FunctionResponse{
								Name:      name,
								Arguments: args,
							},
						},
					},
				},
			},
		},
		Usage: usage,
	}
}

func textChunk(text string, finishReason *string, usage *dto.Usage) *dto.ChatCompletionsStreamResponse {
	delta := dto.ChatCompletionsStreamResponseChoiceDelta{}
	if text != "" {
		delta.SetContentString(text)
	}
	return &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl-test",
		Model: "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: finishReason,
				Delta:        delta,
			},
		},
		Usage: usage,
	}
}

func thinkingChunk(thinking string, finishReason *string, usage *dto.Usage) *dto.ChatCompletionsStreamResponse {
	delta := dto.ChatCompletionsStreamResponseChoiceDelta{}
	if thinking != "" {
		delta.SetReasoningContent(thinking)
	}
	return &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl-test",
		Model: "test-model",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: finishReason,
				Delta:        delta,
			},
		},
		Usage: usage,
	}
}

func countClaudeResponses(responses []*dto.ClaudeResponse, typ string, index int) int {
	count := 0
	for _, response := range responses {
		if response == nil || response.Type != typ {
			continue
		}
		if response.Index == nil || *response.Index != index {
			continue
		}
		count++
	}
	return count
}

func collectClaudeResponses(info *relaycommon.RelayInfo, chunks ...*dto.ChatCompletionsStreamResponse) []*dto.ClaudeResponse {
	responses := make([]*dto.ClaudeResponse, 0)
	for _, chunk := range chunks {
		info.SendResponseCount++
		responses = append(responses, StreamResponseOpenAI2Claude(chunk, info)...)
	}
	return responses
}

func TestStreamResponseOpenAI2ClaudeToolCallStartOnlyOnceForRepeatedName(t *testing.T) {
	info := newClaudeStreamConvertInfo()
	responses := collectClaudeResponses(
		info,
		toolChunk(0, "call_1", "shell_exec", `{"command":"ec`, nil, nil),
		toolChunk(0, "call_1", "shell_exec", `ho hi"}`, nil, nil),
	)

	if got := countClaudeResponses(responses, "content_block_start", 0); got != 1 {
		t.Fatalf("expected one content_block_start for tool index 0, got %d", got)
	}
	if got := countClaudeResponses(responses, "content_block_delta", 0); got != 2 {
		t.Fatalf("expected two input_json_delta chunks for tool index 0, got %d", got)
	}
}

func TestStreamResponseOpenAI2ClaudeToolCallFinalArgumentsBeforeStop(t *testing.T) {
	info := newClaudeStreamConvertInfo()
	finishReason := "tool_calls"
	usage := &dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}
	responses := collectClaudeResponses(
		info,
		toolChunk(0, "call_1", "shell_exec", `{"command":"ec`, nil, nil),
		toolChunk(0, "call_1", "", `ho hi"}`, &finishReason, usage),
	)

	var finalDeltaPos = -1
	var stopPos = -1
	for i, response := range responses {
		if response == nil {
			continue
		}
		if response.Type == "content_block_delta" && response.Index != nil && *response.Index == 0 &&
			response.Delta != nil && response.Delta.PartialJson != nil && *response.Delta.PartialJson == `ho hi"}` {
			finalDeltaPos = i
		}
		if response.Type == "content_block_stop" && response.Index != nil && *response.Index == 0 {
			stopPos = i
		}
	}
	if finalDeltaPos == -1 {
		t.Fatalf("expected final tool arguments delta to be emitted")
	}
	if stopPos == -1 {
		t.Fatalf("expected content_block_stop to be emitted")
	}
	if finalDeltaPos > stopPos {
		t.Fatalf("expected final arguments delta before stop, got delta at %d and stop at %d", finalDeltaPos, stopPos)
	}
	if got := countClaudeResponses(responses, "content_block_stop", 0); got != 1 {
		t.Fatalf("expected one content_block_stop for tool index 0, got %d", got)
	}
	if !info.ClaudeConvertInfo.Done {
		t.Fatalf("expected conversion to be marked done")
	}
}

func TestStreamResponseOpenAI2ClaudeParallelToolCallsStartAndStopOnce(t *testing.T) {
	info := newClaudeStreamConvertInfo()
	finishReason := "tool_calls"
	usage := &dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}
	responses := collectClaudeResponses(
		info,
		&dto.ChatCompletionsStreamResponse{
			Id:    "chatcmpl-test",
			Model: "test-model",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								Index: common.GetPointer(0),
								ID:    "call_1",
								Type:  "function",
								Function: dto.FunctionResponse{
									Name:      "first_tool",
									Arguments: `{"a":`,
								},
							},
							{
								Index: common.GetPointer(1),
								ID:    "call_2",
								Type:  "function",
								Function: dto.FunctionResponse{
									Name:      "second_tool",
									Arguments: `{"b":`,
								},
							},
						},
					},
				},
			},
		},
		&dto.ChatCompletionsStreamResponse{
			Id:    "chatcmpl-test",
			Model: "test-model",
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index:        0,
					FinishReason: &finishReason,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						ToolCalls: []dto.ToolCallResponse{
							{
								Index: common.GetPointer(0),
								ID:    "call_1",
								Type:  "function",
								Function: dto.FunctionResponse{
									Name:      "first_tool",
									Arguments: `1}`,
								},
							},
							{
								Index: common.GetPointer(1),
								ID:    "call_2",
								Type:  "function",
								Function: dto.FunctionResponse{
									Name:      "second_tool",
									Arguments: `2}`,
								},
							},
						},
					},
				},
			},
			Usage: usage,
		},
	)

	for _, idx := range []int{0, 1} {
		if got := countClaudeResponses(responses, "content_block_start", idx); got != 1 {
			t.Fatalf("expected one content_block_start for tool index %d, got %d", idx, got)
		}
		if got := countClaudeResponses(responses, "content_block_stop", idx); got != 1 {
			t.Fatalf("expected one content_block_stop for tool index %d, got %d", idx, got)
		}
	}
}

func TestStreamResponseOpenAI2ClaudeTextAndThinkingStartStopOnce(t *testing.T) {
	tests := []struct {
		name      string
		first     *dto.ChatCompletionsStreamResponse
		second    *dto.ChatCompletionsStreamResponse
		blockType string
		deltaType string
	}{
		{
			name:      "text",
			first:     textChunk("hello ", nil, nil),
			second:    textChunk("world", nil, nil),
			blockType: "text",
			deltaType: "text_delta",
		},
		{
			name:      "thinking",
			first:     thinkingChunk("reason ", nil, nil),
			second:    thinkingChunk("more", nil, nil),
			blockType: "thinking",
			deltaType: "thinking_delta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newClaudeStreamConvertInfo()
			finishReason := "stop"
			usage := &dto.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}
			responses := collectClaudeResponses(
				info,
				tt.first,
				tt.second,
				textChunk("", &finishReason, usage),
			)

			starts := 0
			deltas := 0
			for _, response := range responses {
				if response == nil {
					continue
				}
				if response.Type == "content_block_start" && response.ContentBlock != nil && response.ContentBlock.Type == tt.blockType {
					starts++
				}
				if response.Type == "content_block_delta" && response.Delta != nil && response.Delta.Type == tt.deltaType {
					deltas++
				}
			}
			if starts != 1 {
				t.Fatalf("expected one %s start, got %d", tt.blockType, starts)
			}
			if deltas != 2 {
				t.Fatalf("expected two %s deltas, got %d", tt.deltaType, deltas)
			}
			if got := countClaudeResponses(responses, "content_block_stop", 0); got != 1 {
				t.Fatalf("expected one stop for %s block, got %d", tt.blockType, got)
			}
		})
	}
}
