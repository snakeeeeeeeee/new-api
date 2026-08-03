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
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskModel2DtoReturnsPublicProxyPathForSuccessfulVideoTasks(t *testing.T) {
	for _, action := range constant.TaskActionsByAssetType(constant.TaskAssetTypeVideo) {
		t.Run(action, func(t *testing.T) {
			task := &model.Task{
				TaskID: "task_success",
				Action: action,
				Status: model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: "/v1/videos/upstream-uuid/content",
				},
			}

			assert.Equal(t, taskcommon.BuildProxyPath(task.TaskID), TaskModel2Dto(task).ResultURL)
			assert.Equal(t, "/v1/videos/upstream-uuid/content", task.GetResultURL())
		})
	}
}

func TestTaskModel2DtoPreservesSuccessfulNonVideoResultURL(t *testing.T) {
	success := &model.Task{
		TaskID: "task_success",
		Action: constant.TaskActionImageGeneration,
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.example.com/image.png",
		},
	}
	assert.Equal(t, "https://cdn.example.com/image.png", TaskModel2Dto(success).ResultURL)
}

func TestTaskModel2DtoOmitsFailedTaskResultURL(t *testing.T) {
	failure := &model.Task{
		TaskID:     "task_failure",
		Action:     constant.TaskActionVideoGeneration,
		Status:     model.TaskStatusFailure,
		FailReason: "Generated video rejected by content moderation.",
	}
	assert.Empty(t, TaskModel2Dto(failure).ResultURL)
}

func TestTaskModel2DtoNormalizesTerminalProgress(t *testing.T) {
	for _, status := range []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure} {
		t.Run(string(status), func(t *testing.T) {
			task := &model.Task{Status: status, Progress: "1%"}
			assert.Equal(t, taskcommon.ProgressComplete, TaskModel2Dto(task).Progress)
		})
	}
}

func TestTaskModel2DtoExposesTruthfulProgressMetadata(t *testing.T) {
	task := &model.Task{
		Status:   model.TaskStatusInProgress,
		Progress: "47%",
		PrivateData: model.TaskPrivateData{
			ProgressMetadataSet: true,
			ProgressKnown:       true,
			ProgressSource:      "upstream_percent",
			ProgressStage:       "generating",
		},
	}

	result := TaskModel2Dto(task)
	assert.True(t, result.ProgressKnown)
	assert.Equal(t, "upstream_percent", result.ProgressSource)
	assert.Equal(t, "generating", result.Stage)

	legacy := TaskModel2Dto(&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"})
	assert.False(t, legacy.ProgressKnown)
	assert.Equal(t, "local_status", legacy.ProgressSource)
	assert.Equal(t, "processing", legacy.Stage)
}

func TestApplyOpenAIVideoCompatibilityRequestValidatesMappedResolutionSKU(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	adaptor, ok := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeAdobeVideo))).(channel.OpenAIVideoCompatibilityAdaptor)
	require.True(t, ok)
	originalResolution := "1080p"
	request := dto.VideoTaskCreateRequest{Output: dto.VideoTaskOutputRequest{Resolution: &originalResolution}}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "adobe-seedance-2.0-fast-480p"}}

	normalized, taskErr := applyOpenAIVideoCompatibilityRequest(c, info, adaptor, request, dto.OpenAIVideoCompatibilityMetadata{ResolutionName: "480p"})
	require.Nil(t, taskErr)
	assert.Nil(t, normalized.Output.Resolution, "model-bound providers must not receive a duplicate resolution field")

	_, taskErr = applyOpenAIVideoCompatibilityRequest(c, info, adaptor, request, dto.OpenAIVideoCompatibilityMetadata{ResolutionName: "720p"})
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_video_resolution", taskErr.Code)

	xaiAdaptor, ok := GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeXai))).(channel.OpenAIVideoCompatibilityAdaptor)
	require.True(t, ok)
	normalized, taskErr = applyOpenAIVideoCompatibilityRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video"}}, xaiAdaptor, dto.VideoTaskCreateRequest{}, dto.OpenAIVideoCompatibilityMetadata{ResolutionName: "720p"})
	require.Nil(t, taskErr)
	require.NotNil(t, normalized.Output.Resolution)
	assert.Equal(t, "720p", *normalized.Output.Resolution)
}

func TestImageCredentialLeaseTTLUsesAsyncTaskTimeoutOnlyForAdobeDriver(t *testing.T) {
	task := &model.Task{Platform: constant.TaskPlatform("58"), Action: constant.TaskActionImageGeneration}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	assert.EqualValues(t, imageCredentialLeaseTTLSeconds, imageCredentialLeaseTTL(info, task))

	info.ChannelOtherSettings.ImageHandleExecutionDriver = dto.ImageHandleExecutionDriverAdobeAsyncImage
	expected := int64(async_task_setting.ResolveTimeoutMinutes(task.Platform, task.Action)+adobeAsyncImageLeaseBufferMinutes) * 60
	assert.Equal(t, expected, imageCredentialLeaseTTL(info, task))
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

func TestRelayTaskSubmitRejectsBoundVideoProviderWithoutBillingEstimator(t *testing.T) {
	original := ratio_setting.VideoPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(`{
		"version":1,
		"profiles":{"video":{"name":"Video","billing_mode":"per_second","unit_price":0.03}},
		"model_bindings":{"unsupported-video-model":{"profile":"video","subscription_enabled":false}}
	}`))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(`{"model":"unsupported-video-model","prompt":"test","duration":5}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", strconv.Itoa(constant.ChannelTypeKling))
	c.Set("model_mapping", `{}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeKling)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "unsupported-video-model")

	info := &relaycommon.RelayInfo{
		OriginModelName: "unsupported-video-model",
		UsingGroup:      "default",
		UserGroup:       "default",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}
	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Equal(t, "video_per_second_billing_unsupported", taskErr.Code)
	require.Equal(t, "wallet_only", info.BillingPreferenceOverride)
}

func TestRelayTaskSubmitBoundVideoIgnoresLegacyDurationAndResolutionRatios(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	original := ratio_setting.VideoPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(original))
	})
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(`{
		"version":1,
		"profiles":{"video":{"name":"Video","billing_mode":"per_second","unit_price":0.03}},
		"model_bindings":{"veo-public-1080p":{"profile":"video","subscription_enabled":false}}
	}`))
	require.NoError(t, db.Create(&model.User{
		Id:       27,
		Username: "u27",
		Quota:    0,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          277,
		UserId:      27,
		Key:         "relay-task-video-pricing-token",
		Status:      common.TokenStatusEnabled,
		Name:        "relay-task-video-pricing-token",
		RemainQuota: 0,
		Group:       "default",
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{
		"model":"veo-public-1080p",
		"prompt":"test",
		"duration":7,
		"size":"1080p",
		"metadata":{"durationSeconds":7,"resolution":"1080p"}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("platform", strconv.Itoa(constant.ChannelTypeGemini))
	c.Set("model_mapping", `{}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "veo-public-1080p")

	info := &relaycommon.RelayInfo{
		UserId:          27,
		TokenId:         277,
		OriginModelName: "veo-public-1080p",
		UsingGroup:      "default",
		UserGroup:       "default",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}
	_, taskErr := RelayTaskSubmit(c, info)
	require.NotNil(t, taskErr, "the fixture has no billable user and should stop at preconsume")
	require.NotNil(t, info.PriceData.VideoPricing)
	require.Equal(t, 7, info.PriceData.VideoPricing.Seconds)
	require.Equal(t, 0.03, info.PriceData.VideoPricing.UnitPrice)
	require.Empty(t, info.PriceData.OtherRatios)
	require.Equal(t, info.PriceData.VideoPricing.FinalQuota, info.PriceData.Quota)
}

func TestRelayTaskSubmitAdobeVideoUsesExactMappedModelAndPerSecondWalletQuota(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	originalVideoPricing := ratio_setting.VideoPricing2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(originalVideoPricing))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(`{
		"version":1,
		"profiles":{"seedance-fast-480p":{"name":"Seedance Fast 480p","billing_mode":"per_second","unit_price":0.03}},
		"model_bindings":{"seedance-2.0-fast-480p":{"profile":"seedance-fast-480p","subscription_enabled":false}}
	}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1.5}`))
	require.NoError(t, db.Create(&model.User{
		Id:       29,
		Username: "adobe-video-user",
		Quota:    0,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          299,
		UserId:      29,
		Key:         "adobe-video-token",
		Status:      common.TokenStatusEnabled,
		Name:        "adobe-video-token",
		RemainQuota: 0,
		Group:       "default",
	}).Error)

	duration := 4
	aspectRatio := "16:9"
	generateAudio := false
	normalizedRequest := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-fast-480p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "ocean sunrise"},
		Output: dto.VideoTaskOutputRequest{
			Duration:      &duration,
			AspectRatio:   &aspectRatio,
			GenerateAudio: &generateAudio,
		},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/tasks", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(relaycommon.VideoTaskPublicRequestContextKey, normalizedRequest)
	c.Set("platform", strconv.Itoa(constant.ChannelTypeAdobeVideo))
	c.Set("model_mapping", `{"seedance-2.0-fast-480p":"seedance_2.0_fast_480p"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAdobeVideo)
	common.SetContextKey(c, constant.ContextKeyChannelId, 590)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://adobe-video.invalid")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "provider-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, normalizedRequest.Model)

	info := &relaycommon.RelayInfo{
		UserId:          29,
		TokenId:         299,
		OriginModelName: normalizedRequest.Model,
		UsingGroup:      "default",
		UserGroup:       "default",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}
	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr, "the zero-balance fixture must stop before upstream dispatch")
	require.NotNil(t, info.PriceData.VideoPricing)
	assert.Equal(t, "seedance_2.0_fast_480p", info.UpstreamModelName)
	assert.Equal(t, 4, info.PriceData.VideoPricing.Seconds)
	assert.Equal(t, 0.03, info.PriceData.VideoPricing.UnitPrice)
	assert.Equal(t, 1.5, info.PriceData.VideoPricing.GroupRatio)
	assert.Equal(t, 90000, info.PriceData.Quota)
	assert.Equal(t, info.PriceData.VideoPricing.FinalQuota, info.PriceData.Quota)
	assert.Empty(t, info.PriceData.OtherRatios)
	assert.Equal(t, "wallet_only", info.BillingPreferenceOverride)
}

func TestGeminiProFixedPriceSkipsGlobalAsyncImageUsagePrecharge(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		UsagePrechargeEnabled:   true,
		PrechargeAmountPerImage: 0.1,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  service.GeminiImageModelPro,
		Prompt: "draw",
	})
	expectedQuota := common.QuotaFromFloat(0.5 * common.QuotaPerUnit)
	info := &relaycommon.RelayInfo{
		UserId:          7,
		TokenId:         77,
		OriginModelName: service.GeminiImageModelPro,
		ChannelMeta:     &relaycommon.ChannelMeta{},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_gemini_pro_fixed_price"},
		PriceData: types.PriceData{
			ModelPrice: 0.5,
			Quota:      expectedQuota,
			UsePrice:   true,
		},
	}

	applyAsyncImageUsagePrecharge(ctx, info)
	assert.Equal(t, expectedQuota, info.PriceData.Quota)
	assert.Empty(t, info.PriceData.OtherRatios)

	task, _, err := newAsyncImageTask(ctx, info, constant.TaskPlatform("58"), info.PriceData.Quota)
	require.NoError(t, err)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.True(t, task.PrivateData.BillingContext.UsePrice)
	assert.Equal(t, 0.5, task.PrivateData.BillingContext.ModelPrice)
	assert.Empty(t, task.PrivateData.BillingContext.BillingMode)
	assert.Empty(t, task.PrivateData.BillingContext.PrechargeStrategy)
	assert.Zero(t, task.PrivateData.BillingContext.PrechargePerImage)
	assert.Zero(t, task.PrivateData.BillingContext.ImageCount)
}

func TestAsyncImageTokenPricingKeepsGlobalUsagePrecharge(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		UsagePrechargeEnabled:   true,
		PrechargeAmountPerImage: 0.1,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	count := 2
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "token-priced-image-model",
		Prompt: "draw",
		N:      &count,
		Metadata: map[string]interface{}{
			"n": count,
		},
	})
	info := &relaycommon.RelayInfo{
		UserId:          7,
		TokenId:         77,
		OriginModelName: "token-priced-image-model",
		ChannelMeta:     &relaycommon.ChannelMeta{},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_token_priced_image"},
		PriceData: types.PriceData{
			ModelPrice: -1,
			ModelRatio: 2,
			UsePrice:   false,
		},
	}

	applyAsyncImageUsagePrecharge(ctx, info)
	expectedPerImage := common.QuotaFromFloat(0.1 * common.QuotaPerUnit)
	assert.Equal(t, expectedPerImage*count, info.PriceData.Quota)
	assert.Equal(t, float64(expectedPerImage), info.PriceData.OtherRatios["async_image_precharge_quota_per_image"])

	task, _, err := newAsyncImageTask(ctx, info, constant.TaskPlatform("58"), info.PriceData.Quota)
	require.NoError(t, err)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.False(t, task.PrivateData.BillingContext.UsePrice)
	assert.Equal(t, "async_image_usage_billing", task.PrivateData.BillingContext.BillingMode)
	assert.Equal(t, "per_image_x_n", task.PrivateData.BillingContext.PrechargeStrategy)
	assert.Equal(t, expectedPerImage, task.PrivateData.BillingContext.PrechargePerImage)
	assert.Equal(t, count, task.PrivateData.BillingContext.ImageCount)
}

func TestValidateAndNormalizeGeminiAsyncImageRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "gemini-3.1-flash-image",
		Prompt: "draw",
		Size:   "1024x1536",
		Metadata: map[string]interface{}{
			"provider_options": map[string]any{
				"google": map[string]any{
					"generation_config": map[string]any{"top_p": 0.8, "seed": float64(13)},
				},
			},
		},
	})

	taskErr := validateAndNormalizeGeminiAsyncImageRequest(context, service.GeminiImageModelFlash)

	require.Nil(t, taskErr)
	request, err := relaycommon.GetTaskRequest(context)
	require.NoError(t, err)
	options := request.Metadata["provider_options"].(map[string]any)
	generationConfig := options["google"].(map[string]any)["generationConfig"].(map[string]any)
	assert.Equal(t, 0.8, generationConfig["topP"])
	assert.Equal(t, int64(13), generationConfig["seed"])
}

func TestValidateAndNormalizeGeminiAsyncImageRequestRejectsUnsupportedCount(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	count := 2
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "gemini-3-pro-image-count",
		Prompt: "draw",
		N:      &count,
	})

	taskErr := validateAndNormalizeGeminiAsyncImageRequest(context, service.GeminiImageModelFlash)

	require.NotNil(t, taskErr)
	assert.Equal(t, "unsupported_image_count", taskErr.Code)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
}

func TestValidateAndNormalizeGeminiAsyncImageRequestNormalizesOutputControls(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	aspectRatio := "21:9"
	resolution := "4k"
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model:       service.GeminiImageModelPro,
		Prompt:      "draw",
		AspectRatio: &aspectRatio,
		Resolution:  &resolution,
		Metadata:    map[string]interface{}{},
	})

	taskErr := validateAndNormalizeGeminiAsyncImageRequest(context, service.GeminiImageModelPro)

	require.Nil(t, taskErr)
	request, err := relaycommon.GetTaskRequest(context)
	require.NoError(t, err)
	require.NotNil(t, request.AspectRatio)
	require.NotNil(t, request.Resolution)
	assert.Equal(t, "21:9", *request.AspectRatio)
	assert.Equal(t, "4K", *request.Resolution)
	assert.Equal(t, "21:9", request.Metadata["aspect_ratio"])
	assert.Equal(t, "4K", request.Metadata["resolution"])
}

func TestNewAsyncImageTaskPreservesEffectiveRouteRatioSnapshot(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "snapshot"})
	info := &relaycommon.RelayInfo{
		UserId:          7,
		TokenId:         77,
		UsingGroup:      "aggregate-image",
		OriginModelName: "gpt-image-2",
		ChannelMeta:     &relaycommon.ChannelMeta{},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_ratio_snapshot"},
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:                    0,
			OriginalGroupRatio:            1.5,
			RouteModelGroupRatio:          0,
			HasRouteModelGroupRatio:       true,
			RouteModelRatioAggregateGroup: "aggregate-image",
			RouteModelRatioRealGroup:      "default",
			RouteModelRatioModelName:      "gpt-image-2",
			RouteModelGroupRatioSource:    types.RouteModelGroupRatioSourceUser,
		}},
	}

	task, _, err := newAsyncImageTask(ctx, info, constant.TaskPlatform("58"), 0)
	require.NoError(t, err)
	require.NotNil(t, task.PrivateData.BillingContext)
	billingContext := task.PrivateData.BillingContext
	assert.Zero(t, billingContext.GroupRatio)
	assert.Zero(t, billingContext.RouteModelGroupRatio)
	assert.True(t, billingContext.HasRouteModelGroupRatio)
	assert.Equal(t, "aggregate-image", billingContext.RouteModelAggregateGroup)
	assert.Equal(t, "default", billingContext.RouteModelRealGroup)
	assert.Equal(t, "gpt-image-2", billingContext.RouteModelName)
	assert.Equal(t, types.RouteModelGroupRatioSourceUser, billingContext.RouteModelRatioSource)
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
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.VideoTaskRequest{}, &model.ImageCredentialLease{}, &model.ImageTaskRequest{}, &model.ImageTaskDispatch{}, &model.Asset{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}))
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

func TestCreateDurableVideoTaskStartsSubmittedAndProjectsQueued(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	duration := 4
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-fast-480p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "ocean sunrise"},
		Output:    dto.VideoTaskOutputRequest{Duration: &duration},
	}
	requestJSON, err := common.Marshal(request)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(relaycommon.VideoTaskPublicRequestContextKey, request)
	c.Set(relaycommon.VideoTaskPublicRequestJSONContextKey, requestJSON)
	c.Set(relaycommon.VideoTaskFingerprintContextKey, "normalized-video-submit-fingerprint")

	info := &relaycommon.RelayInfo{
		UserId:          41,
		TokenId:         42,
		UsingGroup:      "default",
		OriginModelName: request.Model,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         61,
			UpstreamModelName: "seedance-2.0-fast-480p",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action:       constant.TaskActionVideoGeneration,
			PublicTaskID: "task_waiting_for_provider_id",
		},
	}

	task, existing, err := createDurableVideoTask(c, info, constant.TaskPlatform("61"), 1200)
	require.NoError(t, err)
	require.Nil(t, existing)
	require.NotNil(t, task)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSubmitted), task.Status)
	assert.Equal(t, taskcommon.ProgressSubmitted, task.Progress)
	assert.Equal(t, "queued", service.BuildPublicVideoTaskFromRequest(task, &request).Status)

	var persisted model.Task
	require.NoError(t, db.First(&persisted, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSubmitted), persisted.Status)
	assert.Equal(t, taskcommon.ProgressSubmitted, persisted.Progress)
}

func TestRelayTaskSubmitImageHandlePreservesFixedModelPriceBeforeSubmit(t *testing.T) {
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
	c.Set("model_mapping", `{"gpt-image-2":"vendor-gpt-image-v2"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 123)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://real.example/v1")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "real-upstream-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2")
	c.Set(common.RequestIdKey, "req-task-lease-submit")

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
	expectedQuota := common.QuotaFromFloat(0.0001 * common.QuotaPerUnit)
	assert.Equal(t, expectedQuota, result.Quota)
	assert.Equal(t, expectedQuota, result.CreatedTask.Quota)
	require.NotNil(t, result.CreatedTask.PrivateData.BillingContext)
	assert.True(t, result.CreatedTask.PrivateData.BillingContext.UsePrice)
	assert.Equal(t, 0.0001, result.CreatedTask.PrivateData.BillingContext.ModelPrice)
	assert.Empty(t, result.CreatedTask.PrivateData.BillingContext.BillingMode)
	assert.Empty(t, result.CreatedTask.PrivateData.BillingContext.PrechargeStrategy)
	assert.Zero(t, result.CreatedTask.PrivateData.BillingContext.PrechargePerImage)
	assert.Zero(t, result.CreatedTask.PrivateData.BillingContext.PrechargeAmountPerImage)
	assert.Zero(t, result.CreatedTask.PrivateData.BillingContext.ImageCount)
	assert.Equal(t, "req-task-lease-submit", result.CreatedTask.PrivateData.BillingContext.RequestId)
	assert.Equal(t, "gpt-image-2", upstreamPayload["model"])
	assert.Equal(t, "gpt-image-2", result.CreatedTask.Properties.OriginModelName)
	assert.Equal(t, "vendor-gpt-image-v2", result.CreatedTask.Properties.UpstreamModelName)
	var lease model.ImageCredentialLease
	require.NoError(t, db.Where("task_id = ?", result.CreatedTask.TaskID).First(&lease).Error)
	assert.Equal(t, "vendor-gpt-image-v2", lease.Model)
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

	require.Equal(t, publicModel, upstreamPayload["model"])
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
	assert.Equal(t, publicModel, imageRequest["model"])
	assert.Equal(t, "low", imageRequest["quality"])
	assert.Equal(t, "2048x2048", imageRequest["size"])
	assert.Equal(t, "2k", imageRequest["resolution"])
	assert.Equal(t, "url", imageRequest["response_format"])
	assert.Equal(t, float64(1), imageRequest["n"])
	var lease model.ImageCredentialLease
	require.NoError(t, db.Where("task_id = ?", persisted.TaskID).First(&lease).Error)
	assert.Equal(t, "gpt-image-2", lease.Model)
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
