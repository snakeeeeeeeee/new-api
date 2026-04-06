package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestFetchHealthDashboardUsesCache(t *testing.T) {
	InitHttpClient()
	common.RedisEnabled = false

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"totalChecks":1,"pendingChecks":0,"healthyChecks":1,"degradedChecks":0,"failedChecks":0,"overallHealthScore":99.9,"averageLatencyMs":120,"lastUpdatedAt":"2026-04-06T13:00:00.000Z"},
			"providers":[{"id":"claude","name":"Anthropic","count":1}],
			"groups":[{"id":"claude-re-kiro","name":"claude-re-kiro","providerId":"claude","count":1}],
			"checks":[{"id":"claude__claude-re-kiro__claude-sonnet-4-6","name":"claude-sonnet-4-6","providerId":"claude","providerName":"Anthropic","groupId":"claude-re-kiro","groupName":"claude-re-kiro","endpoint":"https://example.com/v1/messages","model":"claude-sonnet-4-6","description":"test","status":"healthy","statusLabel":"运行正常","latencyMs":123,"pingLatencyMs":30,"averageLatencyMs":140,"healthScore":98.3,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received.","history":[{"status":"healthy","latencyMs":123,"pingLatencyMs":30,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received."}] }]
		}`))
	}))
	defer server.Close()

	restore := SetHealthDashboardURLForTest(server.URL)
	defer restore()

	first, cacheHit, err := FetchHealthDashboard(nil)
	require.NoError(t, err)
	require.False(t, cacheHit)
	require.NotNil(t, first)
	require.Equal(t, 1, len(first.Checks))
	require.Equal(t, "claude-re-kiro", first.Checks[0].GroupName)

	second, cacheHit, err := FetchHealthDashboard(nil)
	require.NoError(t, err)
	require.True(t, cacheHit)
	require.NotNil(t, second)
	require.Equal(t, 1, len(second.Checks))
	require.Equal(t, 1, callCount)
}

func TestFetchHealthDashboardDoesNotReuseCacheAcrossDifferentURLs(t *testing.T) {
	InitHttpClient()
	common.RedisEnabled = false

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"totalChecks":1,"pendingChecks":0,"healthyChecks":1,"degradedChecks":0,"failedChecks":0,"overallHealthScore":99.9,"averageLatencyMs":120,"lastUpdatedAt":"2026-04-06T13:00:00.000Z"},
			"providers":[{"id":"claude","name":"Anthropic","count":1}],
			"groups":[{"id":"group-a","name":"group-a","providerId":"claude","count":1}],
			"checks":[{"id":"a","name":"claude-sonnet-4-6","providerId":"claude","providerName":"Anthropic","groupId":"group-a","groupName":"group-a","endpoint":"https://example.com/v1/messages","model":"claude-sonnet-4-6","description":"test","status":"healthy","statusLabel":"运行正常","latencyMs":123,"pingLatencyMs":30,"averageLatencyMs":140,"healthScore":98.3,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received.","history":[{"status":"healthy","latencyMs":123,"pingLatencyMs":30,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received."}] }]
		}`))
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"totalChecks":1,"pendingChecks":0,"healthyChecks":1,"degradedChecks":0,"failedChecks":0,"overallHealthScore":99.9,"averageLatencyMs":120,"lastUpdatedAt":"2026-04-06T13:00:00.000Z"},
			"providers":[{"id":"claude","name":"Anthropic","count":1}],
			"groups":[{"id":"group-b","name":"group-b","providerId":"claude","count":1}],
			"checks":[{"id":"b","name":"claude-sonnet-4-6","providerId":"claude","providerName":"Anthropic","groupId":"group-b","groupName":"group-b","endpoint":"https://example.com/v1/messages","model":"claude-sonnet-4-6","description":"test","status":"healthy","statusLabel":"运行正常","latencyMs":123,"pingLatencyMs":30,"averageLatencyMs":140,"healthScore":98.3,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received.","history":[{"status":"healthy","latencyMs":123,"pingLatencyMs":30,"checkedAt":"2026-04-06T13:00:00.000Z","message":"Healthy response received."}] }]
		}`))
	}))
	defer serverB.Close()

	restoreA := SetHealthDashboardURLForTest(serverA.URL)
	first, cacheHit, err := FetchHealthDashboard(nil)
	require.NoError(t, err)
	require.False(t, cacheHit)
	require.Equal(t, "group-a", first.Checks[0].GroupName)
	restoreA()

	restoreB := SetHealthDashboardURLForTest(serverB.URL)
	defer restoreB()
	second, cacheHit, err := FetchHealthDashboard(nil)
	require.NoError(t, err)
	require.False(t, cacheHit)
	require.Equal(t, "group-b", second.Checks[0].GroupName)
}

func TestFetchHealthDashboardReturnsErrorWhenURLNotConfigured(t *testing.T) {
	restore := SetHealthDashboardURLForTest("")
	defer restore()

	data, cacheHit, err := FetchHealthDashboard(nil)
	require.Error(t, err)
	require.Nil(t, data)
	require.False(t, cacheHit)
	require.Contains(t, err.Error(), "not configured")
}
