package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func relayInfoWithToolSchemaCompat(enabled bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 456,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ClaudeToolSchemaCompatEnabled: enabled,
			},
		},
	}
}

func requestWithNullableToolSchema() dto.GeneralOpenAIRequest {
	return dto.GeneralOpenAIRequest{
		Model: "claude-opus-4-7",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
		Tools: []dto.ToolCallRequest{
			{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        "custom",
					Description: "custom tool",
					Parameters: map[string]any{
						"type":       "object",
						"properties": nil,
					},
				},
			},
		},
	}
}

func TestRequestOpenAI2ClaudeMessageToolSchemaCompatDisabledKeepsRequiredNull(t *testing.T) {
	t.Parallel()

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(false), requestWithNullableToolSchema())
	require.NoError(t, err)

	tool := claudeReq.GetTools()[0].(*dto.Tool)
	require.Equal(t, "custom", tool.Name)
	require.Contains(t, tool.InputSchema, "required")
	require.Nil(t, tool.InputSchema["required"])
	require.Contains(t, tool.InputSchema, "properties")
	require.Nil(t, tool.InputSchema["properties"])
}

func TestRequestOpenAI2ClaudeMessageToolSchemaCompatEnabledNormalizesInputSchema(t *testing.T) {
	t.Parallel()

	req := requestWithNullableToolSchema()
	req.Tools[0].Function.Parameters = map[string]any{
		"type":       "",
		"properties": nil,
		"required":   []any{"path", float64(1), "path"},
	}

	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, relayInfoWithToolSchemaCompat(true), req)
	require.NoError(t, err)

	tool := claudeReq.GetTools()[0].(*dto.Tool)
	require.Equal(t, "object", tool.InputSchema["type"])
	require.Equal(t, map[string]any{}, tool.InputSchema["properties"])
	require.Equal(t, []any{"path"}, tool.InputSchema["required"])
}
