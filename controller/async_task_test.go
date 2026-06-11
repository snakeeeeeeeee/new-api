package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAsyncTaskControllerSettingForTest(t *testing.T) {
	t.Helper()
	setting := async_task_setting.GetAsyncTaskSetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
		async_task_setting.ApplyNormalization()
	})
	*setting = async_task_setting.AsyncTaskSetting{
		DefaultTimeoutMinutes: 30,
		QueryLimit:            1000,
		TimeoutOverrides:      []async_task_setting.TimeoutOverride{},
	}
	async_task_setting.ApplyNormalization()
}

func TestGetAsyncTaskStatsReturnsAdminMonitoringData(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	resetAsyncTaskControllerSettingForTest(t)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_async_stats",
		Platform:   constant.TaskPlatform("48"),
		Action:     constant.TaskActionVideoGeneration,
		ChannelId:  3,
		Status:     model.TaskStatusSubmitted,
		Progress:   "0%",
		SubmitTime: now - 31*60,
		CreatedAt:  now - 31*60,
		UpdatedAt:  now - 31*60,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_async_done",
		Platform:   constant.TaskPlatform("48"),
		Action:     constant.TaskActionVideoGeneration,
		ChannelId:  3,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: now - 90*60,
		CreatedAt:  now - 90*60,
		UpdatedAt:  now - 90*60,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/task/async/stats", nil)

	GetAsyncTaskStats(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"total_unfinished":1`)
	require.Contains(t, string(resp.Data), `"timeout_pending":1`)
	require.Contains(t, string(resp.Data), `"platform":"48"`)
	require.Contains(t, string(resp.Data), `"channel_id":3`)
}
