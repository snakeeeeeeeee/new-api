package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestChatCompletionsRequestToResponsesRequestPreservesServiceTier(t *testing.T) {
	stream := false
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
		Stream:      &stream,
		ServiceTier: json.RawMessage(`"priority"`),
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	if err != nil {
		t.Fatalf("ChatCompletionsRequestToResponsesRequest returned error: %v", err)
	}
	if responsesReq.ServiceTier != "priority" {
		t.Fatalf("expected service_tier to be preserved as priority, got %q", responsesReq.ServiceTier)
	}
}

func TestChatCompletionsRequestToResponsesRequestIgnoresInvalidServiceTier(t *testing.T) {
	stream := false
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
		Stream:      &stream,
		ServiceTier: json.RawMessage(`{"tier":"priority"}`),
	}

	responsesReq, err := ChatCompletionsRequestToResponsesRequest(req)
	if err != nil {
		t.Fatalf("ChatCompletionsRequestToResponsesRequest returned error: %v", err)
	}
	if responsesReq.ServiceTier != "" {
		t.Fatalf("expected invalid service_tier shape to be ignored, got %q", responsesReq.ServiceTier)
	}
}
