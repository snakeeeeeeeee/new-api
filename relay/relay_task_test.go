package relay

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskModel2DtoOnlyReturnsResultURLForSuccessfulTask(t *testing.T) {
	success := &model.Task{
		TaskID: "task_success",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://vidgen.x.ai/video.mp4",
		},
	}
	assert.Equal(t, "https://vidgen.x.ai/video.mp4", TaskModel2Dto(success).ResultURL)

	failure := &model.Task{
		TaskID:     "task_failure",
		Status:     model.TaskStatusFailure,
		FailReason: "Generated video rejected by content moderation.",
	}
	assert.Empty(t, TaskModel2Dto(failure).ResultURL)
}

func TestRecalcQuotaFromRatiosSaturatesHugeRatio(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota:       1,
			OtherRatios: map[string]float64{"old": 1},
		},
	}

	quota := recalcQuotaFromRatios(info, map[string]float64{"huge": math.MaxFloat64})

	require.Equal(t, common.MaxQuota, quota)
}

func TestAsyncImagePrechargeQuotaPerImageSaturatesHugeAmount(t *testing.T) {
	quota := asyncImagePrechargeQuotaPerImage(service.ImageHandleExecutorConfig{
		PrechargeAmountPerImage: math.MaxFloat64,
	})

	require.Equal(t, common.MaxQuota, quota)
}

func setupRelayTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.ImageCredentialLease{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}))
	t.Cleanup(func() {
		model.DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestRelayTaskSubmitImageHandleCreatesTaskAndLeaseBeforeSubmit(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		_ = ratio_setting.UpdateModelPriceByJSONString(originalModelPrice)
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.0001}`))
	require.NoError(t, db.Create(&model.User{
		Id:       7,
		Username: "u7",
		Quota:    100000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             77,
		UserId:         7,
		Key:            "relay-task-test-token",
		Status:         common.TokenStatusEnabled,
		Name:           "relay-task-test-token",
		RemainQuota:    100000,
		UnlimitedQuota: false,
		Group:          "default",
	}).Error)

	var upstreamPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/image/tasks", r.URL.Path)
		assert.Equal(t, "Bearer provider-key", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &upstreamPayload))
		var count int64
		require.NoError(t, db.Model(&model.ImageCredentialLease{}).Count(&count).Error)
		assert.EqualValues(t, 1, count)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"provider_task_id":"imgtask_lease","client_task_id":"task_lease_submit","status":"queued"}`))
	}))
	defer server.Close()

	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:                 server.URL,
		APIKey:                  "provider-key",
		InternalBaseURL:         "http://new-api:3000",
		InternalSecretID:        "image_handle_1",
		InternalSecret:          "internal-secret",
		CallbackSecret:          "callback-secret",
		UsagePrechargeEnabled:   true,
		PrechargeAmountPerImage: 0.002468,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_lease_submit",
		"model":"gpt-image-2",
		"prompt":"lease task",
		"size":"1024x1024",
		"metadata":{"n":2}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", "58")
	c.Set("model_mapping", "{}")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 123)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://real.example/v1")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "real-upstream-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2")

	result, taskErr := RelayTaskSubmit(c, &relaycommon.RelayInfo{
		UserId:        7,
		TokenId:       77,
		TokenKey:      "relay-task-test-token",
		UsingGroup:    "default",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.NotNil(t, result.CreatedTask)
	assert.Equal(t, "task_lease_submit", result.CreatedTask.TaskID)
	assert.Equal(t, "imgtask_lease", result.UpstreamTaskID)
	assert.Equal(t, 2468, result.Quota)
	assert.Equal(t, 2468, result.CreatedTask.Quota)
	require.NotNil(t, result.CreatedTask.PrivateData.BillingContext)
	assert.Equal(t, "async_image_usage_billing", result.CreatedTask.PrivateData.BillingContext.BillingMode)
	assert.Equal(t, "per_image_x_n", result.CreatedTask.PrivateData.BillingContext.PrechargeStrategy)
	assert.Equal(t, 1234, result.CreatedTask.PrivateData.BillingContext.PrechargePerImage)
	assert.InDelta(t, 0.002468, result.CreatedTask.PrivateData.BillingContext.PrechargeAmountPerImage, 0.000001)
	assert.Equal(t, 2, result.CreatedTask.PrivateData.BillingContext.ImageCount)
	executor := upstreamPayload["executor"].(map[string]any)
	assert.Equal(t, "provider_direct_lease", executor["type"])
	assert.NotEmpty(t, executor["lease_id"])
	assert.Contains(t, executor["resolve_url"], "/api/internal/image/credential-leases/")
	assert.Nil(t, executor["execute_url"])
	assert.NotContains(t, string(mustMarshalForTest(t, upstreamPayload)), "real-upstream-key")
}

func TestRelayTaskSubmitImageHandleMarksTaskAndLeaseFailedWhenSubmitFails(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		_ = ratio_setting.UpdateModelPriceByJSONString(originalModelPrice)
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.0001}`))
	require.NoError(t, db.Create(&model.User{
		Id:       7,
		Username: "u7",
		Quota:    100000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          77,
		UserId:      7,
		Key:         "relay-task-fail-token",
		Status:      common.TokenStatusEnabled,
		Name:        "relay-task-fail-token",
		RemainQuota: 100000,
		Group:       "default",
	}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"image-handle unavailable"}`))
	}))
	defer server.Close()

	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:          server.URL,
		APIKey:           "provider-key",
		InternalBaseURL:  "http://new-api:3000",
		InternalSecretID: "image_handle_1",
		InternalSecret:   "internal-secret",
		CallbackSecret:   "callback-secret",
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_lease_fail",
		"model":"gpt-image-2",
		"prompt":"lease task",
		"size":"1024x1024"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", "58")
	c.Set("model_mapping", "{}")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 123)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://real.example/v1")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "real-upstream-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2")

	result, taskErr := RelayTaskSubmit(c, &relaycommon.RelayInfo{
		UserId:        7,
		TokenId:       77,
		TokenKey:      "relay-task-fail-token",
		UsingGroup:    "default",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_lease_fail").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusFailure, task.Status)
	assert.Contains(t, task.FailReason, "image-handle unavailable")
	var lease model.ImageCredentialLease
	require.NoError(t, db.Where("task_id = ?", "task_lease_fail").First(&lease).Error)
	assert.Equal(t, model.ImageCredentialLeaseStatusFailed, lease.Status)
}

func mustMarshalForTest(t *testing.T, v any) []byte {
	t.Helper()
	data, err := common.Marshal(v)
	require.NoError(t, err)
	return data
}

func TestTaskModel2DtoDisplaysRealChannelPlatformForImageHandleTask(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:   321,
		Type: constant.ChannelTypeOpenAI,
		Key:  "test-key",
		Name: "openai-image-channel",
	}).Error)

	task := &model.Task{
		TaskID:    "task_image_handle",
		Platform:  constant.TaskPlatform("58"),
		ChannelId: 321,
		Status:    model.TaskStatusQueued,
	}

	result := TaskModel2Dto(task)

	assert.Equal(t, "58", result.Platform)
	assert.Equal(t, strconv.Itoa(constant.ChannelTypeOpenAI), result.DisplayPlatform)
}

func TestRelayTaskSubmitImageHandleClientTaskIDIdempotency(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_external_id",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		UserId:     7,
		ChannelId:  123,
		Quota:      42,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_external_id",
		"model":"gpt-image-2",
		"prompt":"already queued"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", "58")
	c.Set("model_mapping", "{}")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeImageHandle)
	common.SetContextKey(c, constant.ContextKeyChannelId, 123)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:8787")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "provider-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2")

	result, taskErr := RelayTaskSubmit(c, &relaycommon.RelayInfo{
		UserId:        7,
		UsingGroup:    "default",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.NotNil(t, result.ExistingTask)
	assert.Equal(t, "task_external_id", result.ExistingTask.TaskID)
	assert.Equal(t, 42, result.Quota)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"queued","task_id":"task_external_id"}`, recorder.Body.String())
}
