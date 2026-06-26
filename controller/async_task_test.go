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
		"callback_secret":"fallback-callback-secret",
		"debug_upstream":true,
		"sync_image_enabled":true,
		"sync_image_result_policy":"force_base64",
		"sync_image_default_format":"base64",
		"usage_precharge_enabled":true,
		"precharge_amount_per_image":1.25
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
	require.Contains(t, string(resp.Data), `"debug_upstream":true`)
	require.Contains(t, string(resp.Data), `"sync_image_enabled":true`)
	require.Contains(t, string(resp.Data), `"sync_image_result_policy":"force_base64"`)
	require.Contains(t, string(resp.Data), `"sync_image_default_format":"base64"`)
	require.Contains(t, string(resp.Data), `"usage_precharge_enabled":true`)
	require.Contains(t, string(resp.Data), `"precharge_amount_per_image":1.25`)
	require.Contains(t, string(resp.Data), `"precharge_quota_per_image":625000`)

	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", "image_handle_setting.internal_secret").Error)
	assert.Equal(t, "internal-secret", option.Value)
	var debugOption model.Option
	require.NoError(t, db.First(&debugOption, "key = ?", "image_handle_setting.debug_upstream").Error)
	assert.Equal(t, "true", debugOption.Value)
	var syncOption model.Option
	require.NoError(t, db.First(&syncOption, "key = ?", "image_handle_setting.sync_image_enabled").Error)
	assert.Equal(t, "true", syncOption.Value)
	var syncPolicyOption model.Option
	require.NoError(t, db.First(&syncPolicyOption, "key = ?", "image_handle_setting.sync_image_result_policy").Error)
	assert.Equal(t, "force_base64", syncPolicyOption.Value)
	var syncDefaultFormatOption model.Option
	require.NoError(t, db.First(&syncDefaultFormatOption, "key = ?", "image_handle_setting.sync_image_default_format").Error)
	assert.Equal(t, "base64", syncDefaultFormatOption.Value)
	var prechargeAmountOption model.Option
	require.NoError(t, db.First(&prechargeAmountOption, "key = ?", "image_handle_setting.precharge_amount_per_image").Error)
	assert.Equal(t, "1.25", prechargeAmountOption.Value)
	var prechargeQuotaOption model.Option
	require.NoError(t, db.First(&prechargeQuotaOption, "key = ?", "image_handle_setting.precharge_quota_per_image").Error)
	assert.Equal(t, "625000", prechargeQuotaOption.Value)
	assert.Equal(t, "provider-key", image_handle_setting.GetImageHandleSetting().APIKey)
	assert.True(t, image_handle_setting.GetImageHandleSetting().DebugUpstream)
	assert.True(t, image_handle_setting.GetImageHandleSetting().SyncImageEnabled)
	assert.Equal(t, "force_base64", image_handle_setting.GetImageHandleSetting().SyncImageResultPolicy)
	assert.Equal(t, "base64", image_handle_setting.GetImageHandleSetting().SyncImageDefaultFormat)
	assert.True(t, image_handle_setting.GetImageHandleSetting().UsagePrechargeEnabled)
	assert.InDelta(t, 1.25, image_handle_setting.GetImageHandleSetting().PrechargeAmountPerImage, 0.000001)
	assert.Equal(t, 625000, image_handle_setting.GetImageHandleSetting().PrechargeQuotaPerImage)
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

func TestUpdateImageHandleConfigRequiresPrechargeAmountWhenEnabled(t *testing.T) {
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
		"internal_secret":"internal-secret",
		"callback_secret":"callback-secret",
		"usage_precharge_enabled":true,
		"precharge_amount_per_image":0
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
	assert.Contains(t, resp.Message, "每张图预扣费用必须大于 0")
}

func TestGetImageHandleConfigBackfillsAmountFromLegacyQuota(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		common.QuotaPerUnit = originalQuotaPerUnit
	})
	common.QuotaPerUnit = 500000
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:                "http://image-handle:8787",
		APIKey:                 "provider-key",
		InternalBaseURL:        "http://new-api:3000",
		InternalSecretID:       "image_handle_1",
		InternalSecret:         "internal-secret",
		CallbackSecret:         "callback-secret",
		UsagePrechargeEnabled:  true,
		PrechargeQuotaPerImage: 250000,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/task/async/image-handle/config", nil)

	GetImageHandleConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"precharge_amount_per_image":0.5`)
	assert.Contains(t, recorder.Body.String(), `"precharge_quota_per_image":250000`)
}
