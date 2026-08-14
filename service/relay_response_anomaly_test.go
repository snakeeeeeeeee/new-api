package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClassifyRelayResponseAnomaly(t *testing.T) {
	tests := []struct {
		name     string
		evidence RelayResponseAnomalyEvidence
		matched  bool
		reason   string
	}{
		{
			name: "normal text",
			evidence: RelayResponseAnomalyEvidence{
				VisibleText:  "answer",
				IsStream:     true,
				TerminalSeen: true,
			},
		},
		{
			name: "normal tool call",
			evidence: RelayResponseAnomalyEvidence{
				HasToolCall:  true,
				IsStream:     true,
				TerminalSeen: true,
			},
		},
		{
			name: "usage zero zero",
			evidence: RelayResponseAnomalyEvidence{
				EffectiveUsage: &dto.Usage{},
			},
			matched: true,
			reason:  RelayResponseAnomalyUsageZeroZero,
		},
		{
			name: "normal input output zero",
			evidence: RelayResponseAnomalyEvidence{
				EffectiveUsage: &dto.Usage{PromptTokens: 17, TotalTokens: 17},
			},
			matched: true,
			reason:  RelayResponseAnomalyOutputTokensZero,
		},
		{
			name: "normal input output one",
			evidence: RelayResponseAnomalyEvidence{
				EffectiveUsage: &dto.Usage{PromptTokens: 17, CompletionTokens: 1, TotalTokens: 18},
			},
			matched: true,
			reason:  RelayResponseAnomalyOutputTokensOne,
		},
		{
			name: "reasoning only",
			evidence: RelayResponseAnomalyEvidence{
				ReasoningText:  "hidden work",
				EffectiveUsage: &dto.Usage{PromptTokens: 17, CompletionTokens: 32, TotalTokens: 49},
			},
			matched: true,
			reason:  RelayResponseAnomalyReasoningOnly,
		},
		{
			name: "failed terminal with partial text",
			evidence: RelayResponseAnomalyEvidence{
				VisibleText:   "partial",
				IsStream:      true,
				TerminalSeen:  true,
				TerminalEvent: "response.failed",
			},
			matched: true,
			reason:  RelayResponseAnomalyFailed,
		},
		{
			name: "missing stream terminal with tool call",
			evidence: RelayResponseAnomalyEvidence{
				HasToolCall: true,
				IsStream:    true,
			},
			matched: true,
			reason:  RelayResponseAnomalyTerminalMissing,
		},
		{
			name: "terminal output mismatch",
			evidence: RelayResponseAnomalyEvidence{
				ForcedReason:  RelayResponseAnomalyTerminalOutputMismatch,
				VisibleText:   "stream text",
				IsStream:      true,
				TerminalSeen:  true,
				TerminalEvent: "response.completed",
			},
			matched: true,
			reason:  RelayResponseAnomalyTerminalOutputMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, reason := ClassifyRelayResponseAnomaly(test.evidence)
			require.Equal(t, test.matched, matched)
			require.Equal(t, test.reason, reason)
		})
	}
}

func TestCaptureRelayResponseAnomalySnapshotIncludesFullBoundedEvidence(t *testing.T) {
	setupErrorSnapshotTest(t)
	oldQueue := errorSnapshotManager.queue
	errorSnapshotManager.queue = make(chan errorSnapshotWork, 2)
	t.Cleanup(func() { errorSnapshotManager.queue = oldQueue })

	clientBody := `{"model":"gpt-test","messages":[{"role":"user","content":"CLIENT_FINAL_MESSAGE"}],"api_key":"client-secret"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(clientBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-relay-anomaly")
	c.Set("id", 8)
	c.Set("username", "bob")
	c.Set("channel_id", 12)
	c.Set("channel_name", "compatible-provider")
	c.Set("channel_type", 1)
	c.Set("original_model", "gpt-test")
	BeginErrorSnapshotAttempt(c, 1)
	MarkErrorSnapshotChannelSelected(c)
	CaptureRelayDiagnosticUpstreamRequestIfNeeded(c, []byte(`{"model":"mapped-model","messages":[{"role":"system","content":"UPSTREAM_FINAL_MESSAGE"}],"access_token":"upstream-secret"}`))

	rawUsage := &dto.Usage{}
	effectiveUsage := &dto.Usage{PromptTokens: 23, TotalTokens: 23}
	require.True(t, CaptureRelayResponseAnomalySnapshot(c, RelayResponseAnomalyEvidence{
		Protocol:               "openai_chat",
		UpstreamModel:          "mapped-model",
		ResponseModel:          "reported-model",
		RelayFormat:            "openai",
		VisibleText:            "",
		ReasoningText:          "hidden reasoning",
		FinishReasons:          []string{"stop"},
		ResponseStatus:         "completed",
		TerminalEvent:          "stop",
		TerminalSeen:           true,
		RawUsage:               rawUsage,
		EffectiveUsage:         effectiveUsage,
		UsageSource:            "local_estimate",
		UpstreamResponseBody:   []byte(`{"choices":[],"api_key":"response-secret"}`),
		DownstreamResponseBody: []byte(`{"choices":[],"access_token":"downstream-secret"}`),
	}))

	work := <-errorSnapshotManager.queue
	require.Equal(t, "relay_response_anomaly", work.index.ErrorType)
	require.Equal(t, RelayResponseAnomalyReasoningOnly, work.index.ErrorCode)
	require.Equal(t, ErrorSnapshotOutcomeSuspiciousSuccess, work.index.FinalOutcome)
	require.False(t, c.GetBool(errorSnapshotAnyCapturedKey))

	var envelope errorSnapshotEnvelope
	require.NoError(t, common.Unmarshal(work.payload, &envelope))
	require.NotNil(t, envelope.ClientRequest)
	require.Contains(t, envelope.ClientRequest.Body, "CLIENT_FINAL_MESSAGE")
	require.NotNil(t, envelope.UpstreamRequest)
	require.Contains(t, envelope.UpstreamRequest.Body, "UPSTREAM_FINAL_MESSAGE")
	require.NotNil(t, envelope.UpstreamResponse)
	require.NotNil(t, envelope.DownstreamResponse)
	require.NotContains(t, string(work.payload), "client-secret")
	require.NotContains(t, string(work.payload), "upstream-secret")
	require.NotContains(t, string(work.payload), "response-secret")
	require.NotContains(t, string(work.payload), "downstream-secret")
	require.Equal(t, "openai_chat", envelope.Response["protocol"])
	require.Equal(t, "local_estimate", envelope.Response["usage_source"])
	require.Equal(t, RelayResponseAnomalyReasoningOnly, envelope.Response["match_reason"])
	require.NotNil(t, envelope.Response["raw_usage"])
	require.NotNil(t, envelope.Response["effective_usage"])
}

func TestCaptureRelayResponseAnomalySnapshotSkipsMeaningfulResponse(t *testing.T) {
	setupErrorSnapshotTest(t)
	oldQueue := errorSnapshotManager.queue
	errorSnapshotManager.queue = make(chan errorSnapshotWork, 1)
	t.Cleanup(func() { errorSnapshotManager.queue = oldQueue })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))

	require.False(t, CaptureRelayResponseAnomalySnapshot(c, RelayResponseAnomalyEvidence{
		Protocol:      "openai_responses",
		VisibleText:   "complete answer",
		TerminalSeen:  true,
		TerminalEvent: "response.completed",
	}))
	require.Empty(t, errorSnapshotManager.queue)
}

func TestRelayResponseDiagnosticsRetainsBothDirections(t *testing.T) {
	setupErrorSnapshotTest(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	BeginRelayResponseDiagnostics(c, "openai_responses")
	RecordRelayResponseUpstream(c, "response.created", `{"type":"response.created"}`)
	RecordRelayResponseUpstream(c, "response.completed", `{"type":"response.completed","api_key":"stream-secret"}`)
	RecordRelayResponseDownstream(c, "chat.chunk", `{"choices":[]}`)

	summary := RelayResponseDiagnosticsSummary(c)
	require.Equal(t, "openai_responses", summary["protocol"])
	upstream := summary["upstream"].(map[string]any)
	downstream := summary["downstream"].(map[string]any)
	require.Equal(t, 2, upstream["total_events"])
	require.Equal(t, 1, downstream["total_events"])
	require.Len(t, upstream["events"], 2)
	require.Len(t, downstream["events"], 1)
}
