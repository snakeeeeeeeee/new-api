package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMatchSuspiciousClaudeResponse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "reported phrase",
			text: "I'm ready. What would you like me to work on?",
			want: true,
		},
		{
			name: "curly apostrophe and prelude",
			text: "Great — thanks. I’m ready. What would you like me to do next?",
			want: true,
		},
		{
			name: "ordinary answer",
			text: "I'm ready. Here is the implementation you requested.",
		},
		{
			name: "quoted phrase inside answer",
			text: `The upstream returned "I'm ready. What would you like me to work on?" while I was testing it.`,
		},
		{
			name: "ready only",
			text: "I'm ready.",
		},
		{
			name: "long response",
			text: "I'm ready. What would you like me to work on? " + strings.Repeat("detail ", 100),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, reason := MatchSuspiciousClaudeResponse(test.text)
			require.Equal(t, test.want, matched)
			if test.want {
				require.Equal(t, SuspiciousResponseTypeClaudeIdleGreeting, reason)
			} else {
				require.Empty(t, reason)
			}
		})
	}
}

func TestBuildSuspiciousClaudeResponseSnapshotPreservesDiagnosticEvidence(t *testing.T) {
	setupErrorSnapshotTest(t)
	clientBody := `{"model":"claude-test","system":"` + strings.Repeat("context ", 5000) +
		`","messages":[{"role":"user","content":"FINAL_CLIENT_TASK"}],"api_key":"client-secret"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(clientBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer header-secret")
	c.Set(common.RequestIdKey, "req-suspicious")
	c.Set("id", 8)
	c.Set("username", "diagnostic-user")
	c.Set("channel_id", 12)
	c.Set("channel_name", "diagnostic-channel")
	c.Set("channel_type", 14)
	c.Set("original_model", "claude-fable-5")
	BeginErrorSnapshotAttempt(c, 2)
	MarkErrorSnapshotChannelSelected(c)

	upstreamBody := []byte(`{"model":"mapped-model","system":"` + strings.Repeat("upstream ", 5000) +
		`","messages":[{"role":"user","content":"FINAL_UPSTREAM_TASK"}],"access_token":"upstream-secret"}`)
	CaptureClaudeDiagnosticUpstreamRequestIfNeeded(c, upstreamBody)

	evidence := SuspiciousClaudeResponseEvidence{
		VisibleText:          "I'm ready. What would you like me to work on?",
		UpstreamModel:        "mapped-model",
		ResponseModel:        "reported-model",
		StopReason:           "end_turn",
		RelayFormat:          "claude",
		IsStream:             true,
		ContentBlockTypes:    []string{"text"},
		Usage:                map[string]any{"prompt_tokens": 30000, "completion_tokens": 12},
		UpstreamResponseBody: []byte(`{"content":[{"type":"text","text":"I'm ready. What would you like me to work on?"}],"api_key":"response-secret"}`),
		Stream: map[string]any{
			"upstream": map[string]any{
				"events": []any{
					map[string]any{"data": `{"type":"message_start","api_key":"stream-secret"}`},
				},
			},
		},
	}
	work, err := buildSuspiciousClaudeResponseSnapshot(c, evidence, SuspiciousResponseTypeClaudeIdleGreeting)
	require.NoError(t, err)
	require.Equal(t, model.ErrorSnapshotCaptureLevelDiagnostic, work.index.CaptureLevel)
	require.Equal(t, ErrorSnapshotOutcomeSuspiciousSuccess, work.index.FinalOutcome)
	require.Equal(t, http.StatusOK, work.index.StatusCode)
	require.Equal(t, 2, work.index.RetryIndex)
	require.Equal(t, "req-suspicious", work.index.RequestID)
	require.False(t, work.index.InternalRetry)

	var envelope errorSnapshotEnvelope
	require.NoError(t, common.Unmarshal(work.payload, &envelope))
	require.NotNil(t, envelope.ClientRequest)
	require.True(t, envelope.ClientRequest.Truncated)
	require.Contains(t, envelope.ClientRequest.Body, "FINAL_CLIENT_TASK")
	require.NotContains(t, envelope.ClientRequest.Body, "client-secret")
	require.NotNil(t, envelope.UpstreamRequest)
	require.True(t, envelope.UpstreamRequest.Truncated)
	require.Contains(t, envelope.UpstreamRequest.Body, "FINAL_UPSTREAM_TASK")
	require.NotContains(t, envelope.UpstreamRequest.Body, "upstream-secret")
	require.NotNil(t, envelope.UpstreamResponse)
	require.NotContains(t, envelope.UpstreamResponse.Body, "response-secret")
	require.Equal(t, "mapped-model", envelope.Response["requested_upstream_model"])
	require.Equal(t, "reported-model", envelope.Response["response_model"])
	require.Equal(t, SuspiciousResponseTypeClaudeIdleGreeting, envelope.Response["match_reason"])
	streamJSON, err := common.Marshal(envelope.Stream)
	require.NoError(t, err)
	require.NotContains(t, string(streamJSON), "stream-secret")
}

func TestCaptureSuspiciousClaudeResponseSnapshotDoesNotMarkRelayFailure(t *testing.T) {
	setupErrorSnapshotTest(t)
	oldQueue := errorSnapshotManager.queue
	errorSnapshotManager.queue = make(chan errorSnapshotWork, 2)
	t.Cleanup(func() { errorSnapshotManager.queue = oldQueue })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-passive-only")
	c.Set("original_model", "claude-test")
	BeginErrorSnapshotAttempt(c, 0)

	require.False(t, CaptureSuspiciousClaudeResponseSnapshot(c, SuspiciousClaudeResponseEvidence{
		VisibleText: "A normal answer.",
	}))
	require.Empty(t, errorSnapshotManager.queue)

	require.True(t, CaptureSuspiciousClaudeResponseSnapshot(c, SuspiciousClaudeResponseEvidence{
		VisibleText: "I'm ready. What would you like me to work on?",
	}))
	work := <-errorSnapshotManager.queue
	require.Equal(t, ErrorSnapshotOutcomeSuspiciousSuccess, work.index.FinalOutcome)
	require.False(t, c.GetBool(errorSnapshotAnyCapturedKey))
	require.False(t, c.GetBool(errorSnapshotCurrentCapturedKey))
	require.Empty(t, c.GetString(errorSnapshotTerminalOutcomeKey))
}
