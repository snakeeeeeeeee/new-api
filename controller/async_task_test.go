package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
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

func resetOptionMapForConfigControllerTest(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})
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

func TestUpdateImageHandleConfigPersistsAndEchoesSecrets(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	resetOptionMapForConfigControllerTest(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	image_handle_setting.ApplyEnvFallback()

	body := []byte(`{
		"base_url":" http://image-handle:8787/ ",
		"api_key":"provider-key",
		"internal_base_url":" http://new-api:3000/ ",
		"internal_secret_id":"image_handle_1",
		"internal_secret":"internal-secret",
		"callback_secret":"fallback-callback-secret"
	}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/task/async/image-handle/config", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)

	UpdateImageHandleConfig(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"base_url":"http://image-handle:8787"`)
	require.Contains(t, string(resp.Data), `"internal_base_url":"http://new-api:3000"`)
	require.Contains(t, string(resp.Data), `"api_key":"provider-key"`)
	require.Contains(t, string(resp.Data), `"internal_secret":"internal-secret"`)
	require.Contains(t, string(resp.Data), `"callback_secret":"fallback-callback-secret"`)

	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", "image_handle_setting.internal_secret").Error)
	assert.Equal(t, "internal-secret", option.Value)
	assert.Equal(t, "provider-key", image_handle_setting.GetImageHandleSetting().APIKey)
}

func TestUpdateImageHandleConfigRejectsSharedSecrets(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	resetOptionMapForConfigControllerTest(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})

	body := []byte(`{
		"base_url":"http://image-handle:8787",
		"api_key":"provider-key",
		"internal_base_url":"http://new-api:3000",
		"internal_secret_id":"image_handle_1",
		"internal_secret":"same-secret",
		"callback_secret":"same-secret"
	}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/task/async/image-handle/config", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateImageHandleConfig(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "不能和 callback")
}
