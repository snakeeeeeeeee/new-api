package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetHealthDashboard(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"totalChecks":1,"pendingChecks":0,"healthyChecks":1,"degradedChecks":0,"failedChecks":0,"overallHealthScore":96.5,"averageLatencyMs":3200,"lastUpdatedAt":"2026-04-06T13:00:00.000Z"},
			"providers":[{"id":"claude","name":"Anthropic","count":1}],
			"groups":[{"id":"claude-re-kiro","name":"claude-re-kiro","providerId":"claude","count":1}],
			"checks":[{"id":"claude__claude-re-kiro__claude-sonnet-4-6","name":"claude-sonnet-4-6","providerId":"claude","providerName":"Anthropic","groupId":"claude-re-kiro","groupName":"claude-re-kiro","endpoint":"https://example.com/v1/messages","model":"claude-sonnet-4-6","description":"test","status":"healthy","statusLabel":"运行正常","latencyMs":2800,"pingLatencyMs":40,"averageLatencyMs":3200,"healthScore":96.5,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received.","history":[]}]
		}`))
	}))
	defer server.Close()

	restore := service.SetHealthDashboardURLForTest(server.URL)
	defer restore()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/health/dashboard", nil)
	GetHealthDashboard(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), "claude-sonnet-4-6")
	require.Contains(t, string(resp.Data), "cache_hit")
}
