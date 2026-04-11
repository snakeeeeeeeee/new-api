package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAggregateGroupStrategyOptionsCanBeReadAndUpdated(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	model.InitOptionMap()

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.smart_strategy_enabled"`)
	require.Contains(t, string(listResp.Data), `"key":"aggregate_group.consecutive_failure_threshold"`)

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
}
