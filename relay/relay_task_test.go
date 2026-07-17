package relay

import (
	"errors"
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
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
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
	service.InitHttpClient()
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
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.ImageCredentialLease{}, &model.ImageTaskRequest{}, &model.ImageTaskDispatch{}, &model.Asset{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}))
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

func TestRelayTaskSubmitImagePricingNormalizesBeforeMappingAndPersistsSnapshot(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalImagePricing := ratio_setting.ImagePricing2JSONString()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalImagePricing))
	})

	const publicModel = "adobe-gpt-image-2-count"
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(`{
		"version": 1,
		"profiles": {
			"adobe-quality-v1": {
				"name": "ADOBE quality per image",
				"parameter": "quality",
				"default_tier": "economy",
				"tiers": [
					{"key":"economy","upstream_value":"low","aliases":["auto"],"unit_price":0.04},
					{"key":"high","upstream_value":"high","aliases":[],"unit_price":0.15}
				]
			}
		},
		"model_bindings": {
			"adobe-gpt-image-2-count": {"profile":"adobe-quality-v1","max_n":10}
		}
	}`))
	require.NoError(t, db.Create(&model.User{
		Id:       17,
		Username: "u17",
		Quota:    100000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          177,
		UserId:      17,
		Key:         "relay-task-image-pricing-token",
		Status:      common.TokenStatusEnabled,
		Name:        "relay-task-image-pricing-token",
		RemainQuota: 100000,
		Group:       "default",
	}).Error)

	var upstreamPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &upstreamPayload))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"provider_task_id":"imgtask_image_pricing","client_task_id":"task_image_pricing","status":"queued"}`))
	}))
	defer server.Close()

	// A deliberately different global precharge proves that bound image pricing
	// remains authoritative for the async image-handle path.
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:                 server.URL,
		APIKey:                  "provider-key",
		InternalBaseURL:         "http://new-api:3000",
		InternalSecretID:        "image_handle_1",
		InternalSecret:          "internal-secret",
		CallbackSecret:          "callback-secret",
		UsagePrechargeEnabled:   true,
		PrechargeAmountPerImage: 0.123456,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_image_pricing",
		"model":"adobe-gpt-image-2-count",
		"prompt":"mapped and normalized",
		"size":"2048x2048",
		"response_format":"url",
		"metadata":{"resolution":"2k"}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", "58")
	c.Set("model_mapping", `{"adobe-gpt-image-2-count":"gpt-image-2"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 223)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://real.example/v1")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "real-upstream-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, publicModel)

	result, taskErr := RelayTaskSubmit(c, &relaycommon.RelayInfo{
		UserId:        17,
		TokenId:       177,
		TokenKey:      "relay-task-image-pricing-token",
		UsingGroup:    "default",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	expectedQuota := common.QuotaFromFloat(0.04 * common.QuotaPerUnit)
	assert.Equal(t, expectedQuota, result.Quota)
	assert.Equal(t, expectedQuota, result.CreatedTask.Quota)

	require.Equal(t, "gpt-image-2", upstreamPayload["model"])
	parameters, ok := upstreamPayload["parameters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "low", parameters["quality"])
	assert.Equal(t, "2048x2048", parameters["size"])
	assert.Equal(t, "2k", parameters["resolution"])
	assert.Equal(t, "url", parameters["response_format"])
	assert.Equal(t, float64(1), parameters["n"])

	var persisted model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_pricing").First(&persisted).Error)
	require.NotNil(t, persisted.PrivateData.BillingContext)
	billingContext := persisted.PrivateData.BillingContext
	assert.Equal(t, types.ImagePricingBillingMode, billingContext.BillingMode)
	assert.Equal(t, types.ImagePricingBillingMode, billingContext.PrechargeStrategy)
	assert.True(t, billingContext.PerCallBilling)
	assert.Zero(t, billingContext.PrechargePerImage)
	assert.Zero(t, billingContext.PrechargeAmountPerImage)
	assert.NotContains(t, billingContext.OtherRatios, "async_image_precharge_quota_per_image")
	require.NotNil(t, billingContext.ImagePricing)
	assert.Equal(t, publicModel, billingContext.ImagePricing.PublicModel)
	assert.Equal(t, "adobe-quality-v1", billingContext.ImagePricing.ProfileID)
	assert.NotEmpty(t, billingContext.ImagePricing.ProfileHash)
	assert.Equal(t, "", billingContext.ImagePricing.RawValue)
	assert.Equal(t, "economy", billingContext.ImagePricing.EffectiveTier)
	assert.Equal(t, "low", billingContext.ImagePricing.UpstreamValue)
	assert.Equal(t, types.ImagePricingValueSourceDefault, billingContext.ImagePricing.ValueSource)
	assert.Equal(t, 1, billingContext.ImagePricing.N)
	assert.Equal(t, expectedQuota, billingContext.ImagePricing.FinalQuota)

	var imageRequest map[string]any
	require.NoError(t, common.Unmarshal(persisted.PrivateData.ImageRequest, &imageRequest))
	assert.Equal(t, "gpt-image-2", imageRequest["model"])
	assert.Equal(t, "low", imageRequest["quality"])
	assert.Equal(t, "2048x2048", imageRequest["size"])
	assert.Equal(t, "2k", imageRequest["resolution"])
	assert.Equal(t, "url", imageRequest["response_format"])
	assert.Equal(t, float64(1), imageRequest["n"])
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

func newAsyncImageSubmitFailureTask(t *testing.T, db *gorm.DB, status model.TaskStatus, upstreamTaskID string) *model.Task {
	t.Helper()
	task := &model.Task{
		TaskID:    "task_submit_failure_" + strings.ToLower(string(status)),
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeImageHandle)),
		Status:    status,
		Progress:  taskcommon.ProgressQueued,
		Quota:     42,
		CreatedAt: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamTaskID,
		},
	}
	require.NoError(t, db.Create(task).Error)
	return task
}

func asyncImageSubmitFailureError() *dto.TaskError {
	return service.TaskErrorWrapperLocal(errors.New("submit response lost"), "submit_failed", http.StatusBadGateway)
}

func TestResolveAsyncImageSubmitFailureRequiresCallbackTakeoverEvidence(t *testing.T) {
	tests := []struct {
		name             string
		status           model.TaskStatus
		upstreamTaskID   string
		callbackOwnsTask bool
	}{
		{name: "queued without upstream id", status: model.TaskStatusQueued},
		{name: "queued with whitespace upstream id", status: model.TaskStatusQueued, upstreamTaskID: "   "},
		{name: "not start without upstream id", status: model.TaskStatusNotStart},
		{name: "submitted without upstream id", status: model.TaskStatusSubmitted},
		{name: "queued with upstream id", status: model.TaskStatusQueued, upstreamTaskID: "imgtask_queued", callbackOwnsTask: true},
		{name: "submitted with upstream id", status: model.TaskStatusSubmitted, upstreamTaskID: "imgtask_submitted", callbackOwnsTask: true},
		{name: "in progress without upstream id", status: model.TaskStatusInProgress, callbackOwnsTask: true},
		{name: "success without upstream id", status: model.TaskStatusSuccess, callbackOwnsTask: true},
		{name: "failure without upstream id", status: model.TaskStatusFailure, callbackOwnsTask: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupRelayTaskTestDB(t)
			task := newAsyncImageSubmitFailureTask(t, db, test.status, test.upstreamTaskID)
			taskErr := asyncImageSubmitFailureError()

			result, gotErr := resolveAsyncImageSubmitFailure(nil, task, nil, taskErr)
			var persisted model.Task
			require.NoError(t, db.First(&persisted, task.ID).Error)
			if test.callbackOwnsTask {
				require.Nil(t, gotErr)
				require.NotNil(t, result)
				assert.EqualValues(t, test.status, persisted.Status)
				assert.Equal(t, test.upstreamTaskID, result.UpstreamTaskID)
				return
			}

			require.Same(t, taskErr, gotErr)
			require.Nil(t, result)
			assert.EqualValues(t, model.TaskStatusFailure, persisted.Status)
			assert.Equal(t, taskErr.Message, persisted.FailReason)
		})
	}
}

func TestResolveAsyncImageSubmitFailureRetriesAfterStatusCASLoss(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	persisted := newAsyncImageSubmitFailureTask(t, db, model.TaskStatusQueued, "")
	const callbackName = "test:race_async_image_submit_status"
	raced := false
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != "tasks" || raced {
			return
		}
		raced = true
		if err := tx.Session(&gorm.Session{NewDB: true}).Exec(
			"UPDATE tasks SET status = ? WHERE id = ?",
			model.TaskStatusSubmitted,
			persisted.ID,
		).Error; err != nil {
			tx.AddError(err)
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})
	taskErr := asyncImageSubmitFailureError()

	result, gotErr := resolveAsyncImageSubmitFailure(nil, persisted, nil, taskErr)

	require.True(t, raced)
	require.Same(t, taskErr, gotErr)
	require.Nil(t, result)
	require.NoError(t, db.First(persisted, persisted.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, persisted.Status)
	assert.Equal(t, taskErr.Message, persisted.FailReason)
}

func TestResolveAsyncImageSubmitFailureDoesNotOverwriteQueuedCallbackID(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	persisted := newAsyncImageSubmitFailureTask(t, db, model.TaskStatusQueued, "imgtask_callback")
	stale := *persisted
	stale.PrivateData.UpstreamTaskID = ""
	taskErr := asyncImageSubmitFailureError()

	result, gotErr := resolveAsyncImageSubmitFailure(nil, &stale, nil, taskErr)

	require.Nil(t, gotErr)
	require.NotNil(t, result)
	require.NoError(t, db.First(persisted, persisted.ID).Error)
	assert.EqualValues(t, model.TaskStatusQueued, persisted.Status)
	assert.Equal(t, "imgtask_callback", persisted.PrivateData.UpstreamTaskID)
	assert.Equal(t, "imgtask_callback", result.UpstreamTaskID)
}

func TestResolveAsyncImageSubmitFailureReturnsOriginalErrorWhenUpdateFails(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	task := newAsyncImageSubmitFailureTask(t, db, model.TaskStatusQueued, "")
	injectedErr := errors.New("injected task update failure")
	const callbackName = "test:fail_async_image_submit_update"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "tasks" {
			tx.AddError(injectedErr)
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})
	taskErr := asyncImageSubmitFailureError()

	result, gotErr := resolveAsyncImageSubmitFailure(nil, task, nil, taskErr)

	require.Same(t, taskErr, gotErr)
	require.Nil(t, result)
	var persisted model.Task
	require.NoError(t, db.First(&persisted, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusQueued, persisted.Status)
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
