package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateGroupStrategyOptionsCanBeReadAndUpdated(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	model.InitOptionMap()
	originalExcludedIDs := common.LogConsumeExcludedUserIDs
	t.Cleanup(func() {
		_, _ = common.SetLogConsumeExcludedUserIDs(originalExcludedIDs)
	})

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.smart_strategy_enabled"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.consecutive_failure_threshold"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.failure_rate_window_seconds"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.failure_rate_min_requests"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.failure_rate_threshold_percent"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.slow_rate_window_seconds"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.slow_rate_min_requests"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.slow_rate_threshold_percent"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.cluster_degraded_weight_percent"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.slow_first_response_threshold_seconds"`)
	require.Contains(t, string(listResp.Data), `"key":"LogConsumeExcludedUserIDs"`)

	updatePayload := []byte(`{"key":"aggregate_group.consecutive_failure_threshold","value":"4"}`)
	updateRecorder := httptest.NewRecorder()
	updateCtx, _ := gin.CreateTestContext(updateRecorder)
	updateCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(updatePayload))
	updateCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(updateCtx)

	var updateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)
	require.Equal(t, 4, setting.AggregateGroupFailureThreshold)

	percentPayload := []byte(`{"key":"aggregate_group.cluster_degraded_weight_percent","value":"35"}`)
	percentRecorder := httptest.NewRecorder()
	percentCtx, _ := gin.CreateTestContext(percentRecorder)
	percentCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(percentPayload))
	percentCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(percentCtx)

	var percentResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(percentRecorder.Body.Bytes(), &percentResp))
	require.True(t, percentResp.Success, percentResp.Message)
	require.Equal(t, 35, setting.AggregateGroupClusterDegradedWeightPct)

	rateWindowPayload := []byte(`{"key":"aggregate_group.failure_rate_window_seconds","value":"120"}`)
	rateWindowRecorder := httptest.NewRecorder()
	rateWindowCtx, _ := gin.CreateTestContext(rateWindowRecorder)
	rateWindowCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(rateWindowPayload))
	rateWindowCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(rateWindowCtx)

	var rateWindowResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(rateWindowRecorder.Body.Bytes(), &rateWindowResp))
	require.True(t, rateWindowResp.Success, rateWindowResp.Message)
	require.Equal(t, 120, setting.AggregateGroupFailureRateWindowSeconds)

	minRequestsPayload := []byte(`{"key":"aggregate_group.failure_rate_min_requests","value":"200"}`)
	minRequestsRecorder := httptest.NewRecorder()
	minRequestsCtx, _ := gin.CreateTestContext(minRequestsRecorder)
	minRequestsCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(minRequestsPayload))
	minRequestsCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(minRequestsCtx)

	var minRequestsResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(minRequestsRecorder.Body.Bytes(), &minRequestsResp))
	require.True(t, minRequestsResp.Success, minRequestsResp.Message)
	require.Equal(t, 200, setting.AggregateGroupFailureRateMinRequests)

	rateThresholdPayload := []byte(`{"key":"aggregate_group.failure_rate_threshold_percent","value":"6"}`)
	rateThresholdRecorder := httptest.NewRecorder()
	rateThresholdCtx, _ := gin.CreateTestContext(rateThresholdRecorder)
	rateThresholdCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(rateThresholdPayload))
	rateThresholdCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(rateThresholdCtx)

	var rateThresholdResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(rateThresholdRecorder.Body.Bytes(), &rateThresholdResp))
	require.True(t, rateThresholdResp.Success, rateThresholdResp.Message)
	require.Equal(t, 6, setting.AggregateGroupFailureRateThresholdPct)

	slowRatePayload := []byte(`{"key":"aggregate_group.slow_rate_threshold_percent","value":"40"}`)
	slowRateRecorder := httptest.NewRecorder()
	slowRateCtx, _ := gin.CreateTestContext(slowRateRecorder)
	slowRateCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(slowRatePayload))
	slowRateCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(slowRateCtx)

	var slowRateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(slowRateRecorder.Body.Bytes(), &slowRateResp))
	require.True(t, slowRateResp.Success, slowRateResp.Message)
	require.Equal(t, 40, setting.AggregateGroupSlowRateThresholdPct)

	invalidPercentPayload := []byte(`{"key":"aggregate_group.cluster_degraded_weight_percent","value":"101"}`)
	invalidPercentRecorder := httptest.NewRecorder()
	invalidPercentCtx, _ := gin.CreateTestContext(invalidPercentRecorder)
	invalidPercentCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidPercentPayload))
	invalidPercentCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidPercentCtx)

	var invalidPercentResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidPercentRecorder.Body.Bytes(), &invalidPercentResp))
	require.False(t, invalidPercentResp.Success)
	require.Contains(t, invalidPercentResp.Message, "1 到 100")
	require.Equal(t, 35, setting.AggregateGroupClusterDegradedWeightPct)

	invalidRateWindowPayload := []byte(`{"key":"aggregate_group.failure_rate_window_seconds","value":"3601"}`)
	invalidRateWindowRecorder := httptest.NewRecorder()
	invalidRateWindowCtx, _ := gin.CreateTestContext(invalidRateWindowRecorder)
	invalidRateWindowCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidRateWindowPayload))
	invalidRateWindowCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidRateWindowCtx)

	var invalidRateWindowResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidRateWindowRecorder.Body.Bytes(), &invalidRateWindowResp))
	require.False(t, invalidRateWindowResp.Success)
	require.Contains(t, invalidRateWindowResp.Message, "1 到 3600")
	require.Equal(t, 120, setting.AggregateGroupFailureRateWindowSeconds)

	invalidMinRequestsPayload := []byte(`{"key":"aggregate_group.slow_rate_min_requests","value":"0"}`)
	invalidMinRequestsRecorder := httptest.NewRecorder()
	invalidMinRequestsCtx, _ := gin.CreateTestContext(invalidMinRequestsRecorder)
	invalidMinRequestsCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidMinRequestsPayload))
	invalidMinRequestsCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidMinRequestsCtx)

	var invalidMinRequestsResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidMinRequestsRecorder.Body.Bytes(), &invalidMinRequestsResp))
	require.False(t, invalidMinRequestsResp.Success)
	require.Contains(t, invalidMinRequestsResp.Message, "最小样本数")

	invalidRateThresholdPayload := []byte(`{"key":"aggregate_group.slow_rate_threshold_percent","value":"101"}`)
	invalidRateThresholdRecorder := httptest.NewRecorder()
	invalidRateThresholdCtx, _ := gin.CreateTestContext(invalidRateThresholdRecorder)
	invalidRateThresholdCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidRateThresholdPayload))
	invalidRateThresholdCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidRateThresholdCtx)

	var invalidRateThresholdResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidRateThresholdRecorder.Body.Bytes(), &invalidRateThresholdResp))
	require.False(t, invalidRateThresholdResp.Success)
	require.Contains(t, invalidRateThresholdResp.Message, "百分比阈值")
	require.Equal(t, 40, setting.AggregateGroupSlowRateThresholdPct)

	firstResponsePayload := []byte(`{"key":"aggregate_group.slow_first_response_threshold_seconds","value":"0"}`)
	firstResponseRecorder := httptest.NewRecorder()
	firstResponseCtx, _ := gin.CreateTestContext(firstResponseRecorder)
	firstResponseCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(firstResponsePayload))
	firstResponseCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(firstResponseCtx)

	var firstResponseResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(firstResponseRecorder.Body.Bytes(), &firstResponseResp))
	require.True(t, firstResponseResp.Success, firstResponseResp.Message)
	require.Equal(t, 0, setting.AggregateGroupSlowFirstResponseThreshold)

	invalidFirstResponsePayload := []byte(`{"key":"aggregate_group.slow_first_response_threshold_seconds","value":"-1"}`)
	invalidFirstResponseRecorder := httptest.NewRecorder()
	invalidFirstResponseCtx, _ := gin.CreateTestContext(invalidFirstResponseRecorder)
	invalidFirstResponseCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidFirstResponsePayload))
	invalidFirstResponseCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidFirstResponseCtx)

	var invalidFirstResponseResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidFirstResponseRecorder.Body.Bytes(), &invalidFirstResponseResp))
	require.False(t, invalidFirstResponseResp.Success)
	require.Contains(t, invalidFirstResponseResp.Message, "大于等于 0")
	require.Equal(t, 0, setting.AggregateGroupSlowFirstResponseThreshold)

	switchPayload := []byte(`{"key":"aggregate_group.smart_strategy_enabled","value":true}`)
	switchRecorder := httptest.NewRecorder()
	switchCtx, _ := gin.CreateTestContext(switchRecorder)
	switchCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(switchPayload))
	switchCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(switchCtx)

	var switchResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(switchRecorder.Body.Bytes(), &switchResp))
	require.True(t, switchResp.Success, switchResp.Message)
	require.True(t, setting.AggregateGroupSmartStrategyEnabled)

	excludedPayload := []byte(`{"key":"LogConsumeExcludedUserIDs","value":"34, 17,34"}`)
	excludedRecorder := httptest.NewRecorder()
	excludedCtx, _ := gin.CreateTestContext(excludedRecorder)
	excludedCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(excludedPayload))
	excludedCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(excludedCtx)

	var excludedResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(excludedRecorder.Body.Bytes(), &excludedResp))
	require.True(t, excludedResp.Success, excludedResp.Message)
	require.Equal(t, "17,34", common.LogConsumeExcludedUserIDs)

	invalidPayload := []byte(`{"key":"LogConsumeExcludedUserIDs","value":"17,abc"}`)
	invalidRecorder := httptest.NewRecorder()
	invalidCtx, _ := gin.CreateTestContext(invalidRecorder)
	invalidCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidPayload))
	invalidCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidCtx)

	var invalidResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidRecorder.Body.Bytes(), &invalidResp))
	require.False(t, invalidResp.Success)
	require.Contains(t, invalidResp.Message, "不是正整数")
	require.Equal(t, "17,34", common.LogConsumeExcludedUserIDs)

	var stored model.Option
	require.NoError(t, model.DB.Where("key = ?", "LogConsumeExcludedUserIDs").First(&stored).Error)
	require.Equal(t, "17,34", stored.Value)
}

func TestRelayErrorSettingOptionsCanBeReadAndUpdated(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	model.InitOptionMap()
	original := *operation_setting.GetRelayErrorSetting()
	t.Cleanup(func() {
		*operation_setting.GetRelayErrorSetting() = original
	})

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"key":"relay_error_setting.passthrough_enabled"`)
	require.Contains(t, string(listResp.Data), `"key":"relay_error_setting.passthrough_status_codes"`)
	require.Contains(t, string(listResp.Data), `"key":"relay_error_setting.mask_sensitive"`)
	require.Contains(t, string(listResp.Data), `"key":"relay_error_setting.passthrough_enabled","value":"false"`)

	updatePayload := []byte(`{"key":"relay_error_setting.passthrough_status_codes","value":"422,400,400"}`)
	updateRecorder := httptest.NewRecorder()
	updateCtx, _ := gin.CreateTestContext(updateRecorder)
	updateCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(updatePayload))
	updateCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(updateCtx)

	var updateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)
	require.Equal(t, "422,400,400", operation_setting.GetRelayErrorSetting().PassthroughStatusCodes)
	require.False(t, operation_setting.GetRelayErrorSetting().PassthroughEnabled)
	require.False(t, operation_setting.ShouldPassthroughRelayErrorStatusCode(http.StatusBadRequest))

	enablePayload := []byte(`{"key":"relay_error_setting.passthrough_enabled","value":true}`)
	enableRecorder := httptest.NewRecorder()
	enableCtx, _ := gin.CreateTestContext(enableRecorder)
	enableCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(enablePayload))
	enableCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(enableCtx)

	var enableResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(enableRecorder.Body.Bytes(), &enableResp))
	require.True(t, enableResp.Success, enableResp.Message)
	require.True(t, operation_setting.GetRelayErrorSetting().PassthroughEnabled)
	require.True(t, operation_setting.ShouldPassthroughRelayErrorStatusCode(http.StatusBadRequest))
	require.True(t, operation_setting.ShouldPassthroughRelayErrorStatusCode(http.StatusUnprocessableEntity))
	require.False(t, operation_setting.ShouldPassthroughRelayErrorStatusCode(http.StatusTooManyRequests))

	switchPayload := []byte(`{"key":"relay_error_setting.passthrough_enabled","value":false}`)
	switchRecorder := httptest.NewRecorder()
	switchCtx, _ := gin.CreateTestContext(switchRecorder)
	switchCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(switchPayload))
	switchCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(switchCtx)

	var switchResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(switchRecorder.Body.Bytes(), &switchResp))
	require.True(t, switchResp.Success, switchResp.Message)
	require.False(t, operation_setting.GetRelayErrorSetting().PassthroughEnabled)

	invalidPayload := []byte(`{"key":"relay_error_setting.passthrough_status_codes","value":"400,abc"}`)
	invalidRecorder := httptest.NewRecorder()
	invalidCtx, _ := gin.CreateTestContext(invalidRecorder)
	invalidCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewReader(invalidPayload))
	invalidCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(invalidCtx)

	var invalidResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(invalidRecorder.Body.Bytes(), &invalidResp))
	require.False(t, invalidResp.Success)
	require.Contains(t, invalidResp.Message, "invalid http status code rules")
	require.Equal(t, "422,400,400", operation_setting.GetRelayErrorSetting().PassthroughStatusCodes)
}
