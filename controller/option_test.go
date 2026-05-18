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
