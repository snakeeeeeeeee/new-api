package claude

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestFormatClaudeResponseInfoSeparatesVisibleTextFromThinking(t *testing.T) {
	claudeInfo := &ClaudeResponseInfo{
		Diagnostics: &claudeResponseDiagnostics{},
		Usage:       &dto.Usage{},
	}
	startText := "I'm ready. "
	text := "What would you like me to work on?"
	thinking := "internal reasoning must not be matched"
	stopReason := "end_turn"
	index := 0

	require.True(t, FormatClaudeResponseInfo(&dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: &index,
		ContentBlock: &dto.ClaudeMediaMessage{
			Type: "text",
			Text: &startText,
		},
	}, nil, claudeInfo))
	require.True(t, FormatClaudeResponseInfo(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &index,
		Delta: &dto.ClaudeMediaMessage{
			Type:     "text_delta",
			Text:     &text,
			Thinking: &thinking,
		},
	}, nil, claudeInfo))
	require.True(t, FormatClaudeResponseInfo(&dto.ClaudeResponse{
		Type: "message_delta",
		Delta: &dto.ClaudeMediaMessage{
			StopReason: &stopReason,
		},
	}, nil, claudeInfo))

	require.Equal(t, startText+text, claudeInfo.VisibleResponseText.String())
	require.Equal(t, text+thinking, claudeInfo.ResponseText.String())
	require.Equal(t, "end_turn", claudeInfo.StopReason)
	require.Equal(t, []string{"text"}, claudeInfo.ContentBlockTypes)
}

func TestObserveClaudeNonStreamResponseUsesActuallyDeliveredText(t *testing.T) {
	first := "intro"
	last := "I'm ready. What would you like me to work on?"
	response := &dto.ClaudeResponse{
		Model:      "reported-model",
		StopReason: "end_turn",
		Content: []dto.ClaudeMediaMessage{
			{Type: "text", Text: &first},
			{Type: "thinking", Thinking: common.GetPointer("reasoning")},
			{Type: "text", Text: &last},
		},
	}

	native := &ClaudeResponseInfo{Diagnostics: &claudeResponseDiagnostics{}}
	observeClaudeNonStreamResponse(&relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude}, native, response)
	require.Equal(t, first+last, native.VisibleResponseText.String())
	require.Equal(t, []string{"text", "thinking"}, native.ContentBlockTypes)

	openAI := &ClaudeResponseInfo{Diagnostics: &claudeResponseDiagnostics{}}
	observeClaudeNonStreamResponse(&relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}, openAI, response)
	require.Equal(t, last, openAI.VisibleResponseText.String())
	require.Equal(t, "reported-model", openAI.Model)
	require.Equal(t, "end_turn", openAI.StopReason)
}

func TestClaudeDiagnosticTraceIsBounded(t *testing.T) {
	diagnostics := &claudeResponseDiagnostics{}
	diagnostics.recordUpstream("content_block_delta", strings.Repeat("a", claudeDiagnosticMaxBytesPerSide*2))
	for index := 0; index < claudeDiagnosticMaxEvents+10; index++ {
		diagnostics.recordDownstream("content_block_delta", "small")
	}

	require.True(t, diagnostics.Upstream.Truncated)
	require.Len(t, diagnostics.Upstream.Events, 1)
	require.LessOrEqual(t, diagnostics.Upstream.StoredBytes, claudeDiagnosticMaxBytesPerSide)
	require.Equal(t, claudeDiagnosticMaxBytesPerSide*2, diagnostics.Upstream.Events[0].DataBytes)
	require.NotEmpty(t, diagnostics.Upstream.Events[0].SHA256)

	require.True(t, diagnostics.Downstream.Truncated)
	require.Len(t, diagnostics.Downstream.Events, claudeDiagnosticMaxEvents)
	require.Equal(t, claudeDiagnosticMaxEvents+10, diagnostics.Downstream.TotalEvents)
	require.LessOrEqual(t, diagnostics.Downstream.StoredBytes, claudeDiagnosticMaxBytesPerSide)
}
