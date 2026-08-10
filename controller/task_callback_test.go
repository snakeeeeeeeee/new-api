package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signCallbackTestBody(timestamp string, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func makeCallbackRequest(t *testing.T, body []byte, secret string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/task/callback/external-image/batch", bytes.NewReader(body))
	ctx.Request.Header.Set("X-Callback-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Callback-Signature", signCallbackTestBody(timestamp, body, secret))
	ctx.Request.Header.Set("X-Callback-Secret-Id", "channel_123")
	return ctx, recorder
}

func makeLeaseResolveRequest(t *testing.T, leaseID string, body []byte, secretID string, secret string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/internal/image/credential-leases/"+leaseID+"/resolve", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("X-ImageHandle-Timestamp", timestamp)
	ctx.Request.Header.Set("X-ImageHandle-Signature", signCallbackTestBody(timestamp, body, secret))
	ctx.Request.Header.Set("X-ImageHandle-Secret-Id", secretID)
	ctx.Request.Header.Set("X-ImageHandle-Event-Id", "evt_resolve_1")
	ctx.Params = gin.Params{{Key: "lease_id", Value: leaseID}}
	return ctx, recorder
}

func seedAttemptLeaseResolveFixture(t *testing.T, suffix string) (*model.Task, *model.ImageTaskRetryState, *model.ImageTaskAttempt, *model.ImageCredentialLease) {
	t.Helper()
	settings, err := common.Marshal(dto.ChannelOtherSettings{ImageHandleExecutionDriver: dto.ImageHandleExecutionDriverAdobeAsyncImage})
	require.NoError(t, err)
	channelID := 880 + len(suffix)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelID, Type: constant.ChannelTypeOpenAI, Name: "attempt-channel-" + suffix,
		Key: "attempt-key-" + suffix, Status: common.ChannelStatusEnabled, OtherSettings: string(settings),
	}).Error)
	now := time.Now().Unix()
	parent := &model.Task{
		TaskID: "task_attempt_lease_" + suffix, Platform: constant.TaskPlatform("58"),
		Action: constant.TaskActionImageGeneration, UserId: 1, ChannelId: 777,
		Status: model.TaskStatusQueued, Progress: "0%", SubmitTime: now,
		PrivateData: model.TaskPrivateData{ImageHandleExecutionDriver: dto.ImageHandleExecutionDriverAdobeAsyncImage},
		Properties:  model.Properties{OriginModelName: "gpt-image-2", UpstreamModelName: "initial-mapped-image"},
		CreatedAt:   now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(parent).Error)
	state := model.NewImageTaskRetryState(parent, 1, "default", "default", "gpt-image-2")
	state.CurrentRouteGroup = "fallback"
	require.NoError(t, model.DB.Create(state).Error)
	attempt := model.NewImageTaskAttempt(
		parent, 2, "task_attempt_internal_"+suffix, channelID, "fallback", 1, "primary",
		"gpt-image-2", "attempt-mapped-image", 100, &model.TaskBillingContext{OriginModelName: "gpt-image-2"}, "req_attempt_lease",
	)
	attempt.ExecutionDriver = dto.ImageHandleExecutionDriverLegacySync
	attempt.ProviderTaskID = "provider_attempt_" + suffix
	attempt.Status = model.ImageTaskAttemptSubmitted
	require.NoError(t, model.DB.Create(attempt).Error)
	state.ActiveAttemptRecordID = attempt.ID
	state.AttemptCount = 2
	require.NoError(t, model.DB.Save(state).Error)
	lease := model.NewImageCredentialLeaseForAttempt(parent, attempt, "generation", attempt.UpstreamModel, 1800)
	require.NoError(t, model.DB.Create(lease).Error)
	return parent, state, attempt, lease
}

type imageRetryCallbackFixture struct {
	Secret  string
	Parent  *model.Task
	State   *model.ImageTaskRetryState
	Attempt *model.ImageTaskAttempt
	LogID   int
}

func setupImageRetryCallbackFixture(t *testing.T, suffix string, retryLimit int) *imageRetryCallbackFixture {
	t.Helper()
	db := setupInviteCodeControllerTestDB(t)
	originalMemoryCache := common.MemoryCacheEnabled
	originalImageHandleSetting := *image_handle_setting.GetImageHandleSetting()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCache
		*image_handle_setting.GetImageHandleSetting() = originalImageHandleSetting
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
	})
	common.MemoryCacheEnabled = false
	const publicModel = "image-retry-callback-model"
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"image-retry-callback-model":0.0002}`))
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:          "http://image-handle.invalid",
		APIKey:           "image-handle-test-key",
		InternalBaseURL:  "http://new-api.internal",
		InternalSecretID: "image_handle_1",
		InternalSecret:   "internal-secret",
		CallbackSecret:   "global-callback-secret",
	})

	secret := "retry-callback-secret"
	channelSettings, err := common.Marshal(dto.ChannelOtherSettings{
		CallbackSecret: secret, ImageHandleExecutionDriver: dto.ImageHandleExecutionDriverLegacySync,
	})
	require.NoError(t, err)
	priority := int64(100)
	weight := uint(100)
	for _, channelID := range []int{123, 124} {
		channel := &model.Channel{
			Id: channelID, Type: constant.ChannelTypeOpenAI, Name: fmt.Sprintf("retry-channel-%d-%s", channelID, suffix),
			Key: fmt.Sprintf("retry-key-%d", channelID), Status: common.ChannelStatusEnabled,
			Group: "default", Models: publicModel, Priority: &priority, Weight: &weight,
			OtherSettings: string(channelSettings), CreatedTime: time.Now().Unix(),
		}
		if channelID == 123 {
			channel.UsedQuota = 100
		}
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	require.NoError(t, db.Create(&model.User{
		Id: 1, Username: "retry-user", Quota: 1000, Status: common.UserStatusEnabled, Group: "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 11, UserId: 1, Key: "retry-token", Name: "retry-token", Status: common.TokenStatusEnabled,
		RemainQuota: 1000, Group: "default",
	}).Error)

	now := time.Now().Unix()
	parent := &model.Task{
		TaskID: "task_retry_callback_" + suffix, Platform: imageHandleTaskPlatform(),
		Action: constant.TaskActionImageGeneration, UserId: 1, ChannelId: 123,
		Quota: 100, Status: model.TaskStatusQueued, Progress: "0%", Group: "default",
		SubmitTime: now, CreatedAt: now, UpdatedAt: now,
		PrivateData: model.TaskPrivateData{
			BillingSource: service.BillingSourceWallet, TokenId: 11,
			ImageHandleExecutionDriver: dto.ImageHandleExecutionDriverLegacySync,
			BillingContext: &model.TaskBillingContext{
				ModelPrice: 0.0002, GroupRatio: 1, OriginModelName: publicModel,
				UsePrice: true, PerCallBilling: true, RouteQuota: 100, RequestId: "req_retry_callback_" + suffix,
			},
		},
		Properties: model.Properties{
			OriginModelName: publicModel, UpstreamModelName: publicModel, AssetType: constant.TaskAssetTypeImage,
		},
	}
	require.NoError(t, db.Create(parent).Error)
	prechargeLog := &model.Log{
		UserId: parent.UserId, CreatedAt: now, Type: model.LogTypeConsume,
		Content: "async image precharge", ModelName: publicModel, Quota: parent.Quota,
		ChannelId: parent.ChannelId, TokenId: parent.PrivateData.TokenId, Group: parent.Group,
		RequestId: parent.PrivateData.BillingContext.RequestId,
		Other:     common.MapToJsonStr(map[string]interface{}{"task_id": parent.TaskID}),
	}
	require.NoError(t, db.Create(prechargeLog).Error)
	parent.PrivateData.BillingContext.ConsumeLogId = prechargeLog.Id
	require.NoError(t, db.Model(parent).Update("private_data", parent.PrivateData).Error)
	request := dto.ImageTaskCreateRequest{
		Model: publicModel, Operation: "generation", Input: dto.ImageTaskInputRequest{Prompt: "retry callback test"},
	}
	requestJSON, err := common.Marshal(request)
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewImageTaskRequest(parent, parent.UserId, nil, "retry-callback-fingerprint-"+suffix, "", requestJSON)).Error)
	state := model.NewImageTaskRetryState(parent, retryLimit, "default", "default", publicModel)
	state.CurrentRouteGroup = "default"
	state.CurrentRouteIndex = -1
	state.CurrentGroupAttempts = 1
	require.NoError(t, db.Create(state).Error)
	attempt := model.NewImageTaskAttempt(
		parent, 1, "task_attempt_callback_"+suffix, 123, "default", -1, "",
		publicModel, publicModel, 100, parent.PrivateData.BillingContext, "req_retry_callback_"+suffix+":attempt:1",
	)
	attempt.Status = model.ImageTaskAttemptSubmitted
	attempt.ProviderTaskID = "provider_retry_callback_" + suffix
	require.NoError(t, db.Create(attempt).Error)
	state.ActiveAttemptRecordID = attempt.ID
	state.AttemptCount = 1
	state.AppendTrace(attempt, model.ImageTaskAttemptSubmitted, "")
	require.NoError(t, db.Save(state).Error)
	lease := model.NewImageCredentialLeaseForAttempt(parent, attempt, "generation", publicModel, 1800)
	require.NoError(t, db.Create(lease).Error)
	dispatch := model.NewImageTaskDispatchForAttempt(parent, attempt, []byte(`{"client_task_id":"`+attempt.ClientTaskID+`"}`))
	dispatch.Status = model.ImageTaskDispatchDelivered
	dispatch.DeliveredAt = now
	require.NoError(t, db.Create(dispatch).Error)
	return &imageRetryCallbackFixture{Secret: secret, Parent: parent, State: state, Attempt: attempt, LogID: prechargeLog.Id}
}

func sendImageAttemptFailureCallback(t *testing.T, fixture *imageRetryCallbackFixture, eventID string, retryable bool) *httptest.ResponseRecorder {
	t.Helper()
	return sendImageAttemptFailureCallbackWithError(t, fixture, eventID, &imageCallbackError{
		Code: "upstream_unavailable", Message: "temporary provider outage", Retryable: retryable, UpstreamStatus: http.StatusServiceUnavailable,
	})
}

func sendImageAttemptFailureCallbackWithError(t *testing.T, fixture *imageRetryCallbackFixture, eventID string, callbackError *imageCallbackError) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := common.Marshal(imageCallbackBatchRequest{Events: []imageCallbackEvent{{
		EventID: eventID, ClientTaskID: fixture.Attempt.ClientTaskID, ProviderTaskID: fixture.Attempt.ProviderTaskID,
		Status: "failed", Progress: "100%", Error: callbackError,
	}}})
	require.NoError(t, err)
	ctx, recorder := makeCallbackRequest(t, payload, fixture.Secret)
	ImageTaskCallbackBatch(ctx)
	return recorder
}

func convertImageRetryFixtureToLegacy(t *testing.T, fixture *imageRetryCallbackFixture, keepNormalizedRequest bool) {
	t.Helper()
	db := model.DB
	require.NoError(t, db.Model(&model.ImageTaskDispatch{}).
		Where("task_record_id = ?", fixture.Parent.ID).
		UpdateColumn("attempt_record_id", nil).Error)
	require.NoError(t, db.Model(&model.ImageCredentialLease{}).
		Where("task_record_id = ?", fixture.Parent.ID).
		UpdateColumn("attempt_record_id", nil).Error)
	require.NoError(t, db.Where("task_record_id = ?", fixture.Parent.ID).Delete(&model.ImageTaskAttempt{}).Error)
	require.NoError(t, db.Where("task_record_id = ?", fixture.Parent.ID).Delete(&model.ImageTaskRetryState{}).Error)
	if !keepNormalizedRequest {
		require.NoError(t, db.Where("task_record_id = ?", fixture.Parent.ID).Delete(&model.ImageTaskRequest{}).Error)
	}
	fixture.Attempt = &model.ImageTaskAttempt{
		ClientTaskID:   fixture.Parent.TaskID,
		ProviderTaskID: "provider_legacy_" + fixture.Parent.TaskID,
		ChannelID:      fixture.Parent.ChannelId,
	}
}

func getUserQuotaForControllerTest(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	return user.Quota
}

func sendImageAttemptSuccessCallback(t *testing.T, fixture *imageRetryCallbackFixture, attempt *model.ImageTaskAttempt, eventID string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := common.Marshal(imageCallbackBatchRequest{Events: []imageCallbackEvent{{
		EventID: eventID, ClientTaskID: attempt.ClientTaskID, ProviderTaskID: "provider_retry_winner",
		Status: "succeeded", Progress: "100%", Result: &imageCallbackResult{Images: []imageCallbackImage{{
			URL: "https://cdn.example.test/retry-winner.webp", MimeType: "image/webp", Width: 1024, Height: 1024,
		}}},
	}}})
	require.NoError(t, err)
	ctx, recorder := makeCallbackRequest(t, payload, fixture.Secret)
	ctx.Request.Header.Set("X-Callback-Secret-Id", fmt.Sprintf("channel_%d", attempt.ChannelID))
	ImageTaskCallbackBatch(ctx)
	return recorder
}

func TestImageTaskAttemptRetryableCallbackSchedulesOneDifferentChannel(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "retryable", 1)

	recorder := sendImageAttemptFailureCallback(t, fixture, "evt_retryable_1", true)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"retry_scheduled"`)
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	assert.Equal(t, model.ImageTaskAttemptFailed, attempts[0].Status)
	assert.True(t, attempts[0].ErrorRetryable)
	assert.Equal(t, 123, attempts[0].ChannelID)
	assert.Equal(t, 124, attempts[1].ChannelID)
	assert.NotEqual(t, attempts[0].ClientTaskID, attempts[1].ClientTaskID)
	state, exists, err := model.GetImageTaskRetryStateByTaskRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, model.ImageTaskRetryStateActive, state.Status)
	assert.Equal(t, 2, state.AttemptCount)
	assert.Contains(t, state.FailedChannelIDs, 123)
	parent, err := model.GetTaskByRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	assert.EqualValues(t, model.TaskStatusQueued, parent.Status)
	assert.Equal(t, 1000, getUserQuotaForControllerTest(t, 1))

	var dispatches []model.ImageTaskDispatch
	require.NoError(t, model.DB.Where("task_record_id = ?", fixture.Parent.ID).Order("id ASC").Find(&dispatches).Error)
	require.Len(t, dispatches, 2)
	require.NotNil(t, dispatches[1].AttemptRecordID)
	assert.Equal(t, attempts[1].ID, *dispatches[1].AttemptRecordID)
	assert.Contains(t, dispatches[1].RequestBody, attempts[1].ClientTaskID)
	lease, leaseExists, err := model.GetImageCredentialLeaseByAttemptRecordID(attempts[1].ID)
	require.NoError(t, err)
	require.True(t, leaseExists)
	assert.Equal(t, 124, lease.ChannelID)
	failedLease, failedLeaseExists, err := model.GetImageCredentialLeaseByAttemptRecordID(attempts[0].ID)
	require.NoError(t, err)
	require.True(t, failedLeaseExists)
	assert.Equal(t, model.ImageCredentialLeaseStatusFailed, failedLease.Status)

	duplicate := sendImageAttemptFailureCallback(t, fixture, "evt_retryable_duplicate", true)
	assert.Contains(t, duplicate.Body.String(), `"status":"ignored_terminal"`)
	attempts, err = model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	assert.Len(t, attempts, 2)
}

func TestImageTaskAttemptNonRetryableCallbackFinalizesAndRefundsOnce(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "non_retryable", 2)

	first := sendImageAttemptFailureCallback(t, fixture, "evt_non_retryable_1", false)
	second := sendImageAttemptFailureCallback(t, fixture, "evt_non_retryable_2", false)

	require.Equal(t, http.StatusOK, first.Code)
	assert.Contains(t, first.Body.String(), `"status":"accepted"`)
	assert.Contains(t, second.Body.String(), `"status":"ignored_terminal"`)
	parent, err := model.GetTaskByRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	assert.EqualValues(t, model.TaskStatusFailure, parent.Status)
	assert.Equal(t, 1100, getUserQuotaForControllerTest(t, 1))
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.False(t, attempts[0].ErrorRetryable)
}

func TestImageTaskAttemptConcurrentRetryableCallbacksCreateOneNextAttempt(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "concurrent", 1)
	var wait sync.WaitGroup
	wait.Add(2)
	responses := make([]*httptest.ResponseRecorder, 2)
	for index := range responses {
		go func(index int) {
			defer wait.Done()
			responses[index] = sendImageAttemptFailureCallback(t, fixture, fmt.Sprintf("evt_concurrent_%d", index), true)
		}(index)
	}
	wait.Wait()

	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	retryScheduled := 0
	ignored := 0
	for _, response := range responses {
		if strings.Contains(response.Body.String(), `"status":"retry_scheduled"`) {
			retryScheduled++
		}
		if strings.Contains(response.Body.String(), `"status":"ignored_stale"`) || strings.Contains(response.Body.String(), `"status":"ignored_terminal"`) {
			ignored++
		}
	}
	assert.Equal(t, 1, retryScheduled)
	assert.Equal(t, 1, ignored)
}

func TestLegacyNormalizedImageTaskRetryableCallbackLazilyCreatesAttempts(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "legacy_lazy_retry", 1)
	convertImageRetryFixtureToLegacy(t, fixture, true)
	originalRetryTimes := common.RetryTimes
	common.RetryTimes = 1
	t.Cleanup(func() { common.RetryTimes = originalRetryTimes })

	response := sendImageAttemptFailureCallback(t, fixture, "evt_legacy_lazy_retry", true)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"retry_scheduled"`)
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	assert.Equal(t, fixture.Parent.TaskID, attempts[0].ClientTaskID)
	assert.Equal(t, model.ImageTaskAttemptFailed, attempts[0].Status)
	assert.Equal(t, 123, attempts[0].ChannelID)
	assert.Equal(t, 124, attempts[1].ChannelID)
	state, exists, err := model.GetImageTaskRetryStateByTaskRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, 1, state.RetryLimit)
	assert.False(t, state.LockedChannel)
	assert.Equal(t, 2, state.AttemptCount)

	var dispatches []model.ImageTaskDispatch
	require.NoError(t, model.DB.Where("task_record_id = ?", fixture.Parent.ID).Order("id ASC").Find(&dispatches).Error)
	require.Len(t, dispatches, 2)
	require.NotNil(t, dispatches[0].AttemptRecordID)
	assert.Equal(t, attempts[0].ID, *dispatches[0].AttemptRecordID)
	var leases []model.ImageCredentialLease
	require.NoError(t, model.DB.Where("task_record_id = ?", fixture.Parent.ID).Order("id ASC").Find(&leases).Error)
	require.Len(t, leases, 2)
	require.NotNil(t, leases[0].AttemptRecordID)
	assert.Equal(t, attempts[0].ID, *leases[0].AttemptRecordID)
}

func TestLegacyImageTaskWithoutNormalizedRequestKeepsLegacyFailurePath(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "legacy_without_request", 1)
	convertImageRetryFixtureToLegacy(t, fixture, false)

	response := sendImageAttemptFailureCallback(t, fixture, "evt_legacy_without_request", true)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"accepted"`)
	_, exists, err := model.GetImageTaskRetryStateByTaskRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	assert.False(t, exists)
	parent, err := model.GetTaskByRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	assert.EqualValues(t, model.TaskStatusFailure, parent.Status)
}

func TestLegacyAdministratorImageTaskIsConservativelyLocked(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "legacy_admin_locked", 1)
	convertImageRetryFixtureToLegacy(t, fixture, true)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", fixture.Parent.UserId).
		Update("role", common.RoleAdminUser).Error)

	response := sendImageAttemptFailureCallback(t, fixture, "evt_legacy_admin_locked", true)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"accepted"`)
	state, exists, err := model.GetImageTaskRetryStateByTaskRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.True(t, state.LockedChannel)
	assert.Equal(t, model.ImageTaskRetryStateFailed, state.Status)
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	assert.Len(t, attempts, 1)
}

func TestImageTaskAttemptRetryDoesNotReadThroughLockedTransaction(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "single_connection", 1)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.SetMaxOpenConns(10) })

	response := sendImageAttemptFailureCallback(t, fixture, "evt_single_connection", true)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"retry_scheduled"`)
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	assert.Len(t, attempts, 2)
}

func TestImageTaskAttemptFailureRecordsAggregateRPMAndSmartSignals(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "aggregate_signals", 1)
	originalSmartStrategyEnabled := setting.AggregateGroupSmartStrategyEnabled
	setting.AggregateGroupSmartStrategyEnabled = true
	t.Cleanup(func() { setting.AggregateGroupSmartStrategyEnabled = originalSmartStrategyEnabled })
	aggregateName := "image-retry-signals"
	aggregate := &model.AggregateGroup{
		Name: aggregateName, DisplayName: aggregateName, Status: model.AggregateGroupStatusEnabled,
		RoutingMode: model.AggregateGroupRoutingModeFailover, SmartRoutingEnabled: true,
		CreatedTime: time.Now().Unix(), UpdatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(aggregate).Error)
	require.NoError(t, model.DB.Create(&model.AggregateGroupTarget{
		AggregateGroupId: aggregate.Id, RealGroup: "default", OrderIndex: 0,
	}).Error)
	fixture.State.AggregateGroup = aggregateName
	fixture.State.RoutingMode = model.AggregateGroupRoutingModeFailover
	fixture.State.CurrentRouteIndex = 0
	require.NoError(t, model.DB.Save(fixture.State).Error)
	t.Cleanup(func() {
		_ = service.ResetAggregateGroupRouteStrategyState(aggregateName, fixture.Attempt.OriginModel, "default")
	})

	response := sendImageAttemptFailureCallback(t, fixture, "evt_aggregate_signals", true)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"status":"retry_scheduled"`)
	stats := service.GetAggregateRouteWindowStatsForPool(aggregateName, fixture.Attempt.OriginModel, "", "default", 60)
	assert.Equal(t, 1, stats.Attempts)
	assert.Equal(t, 1, stats.Failures)
	assert.Equal(t, 1, stats.StrategyFailures)
}

func TestImageTaskAttemptAutoDisableUsesResolvedMultiKeyIndex(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "multi_key_disable", 0)
	channel, err := model.GetChannelById(fixture.Attempt.ChannelID, true)
	require.NoError(t, err)
	autoBan := 1
	channel.AutoBan = &autoBan
	channel.Key = "first-image-key\nsecond-image-key"
	channel.ChannelInfo = model.ChannelInfo{
		IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling, MultiKeyPollingIndex: 1,
	}
	require.NoError(t, channel.Save())
	originalAutomaticDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = originalAutomaticDisable })

	body := []byte(fmt.Sprintf(
		`{"provider_task_id":%q,"client_task_id":%q,"attempt":1,"operation":"generation","model":%q}`,
		fixture.Attempt.ProviderTaskID, fixture.Attempt.ClientTaskID, fixture.Attempt.OriginModel,
	))
	resolveContext, resolveRecorder := makeLeaseResolveRequest(t, "", body, "image_handle_1", "internal-secret")
	lease, exists, err := model.GetImageCredentialLeaseByAttemptRecordID(fixture.Attempt.ID)
	require.NoError(t, err)
	require.True(t, exists)
	resolveContext.Params = gin.Params{{Key: "lease_id", Value: lease.LeaseID}}
	ResolveImageCredentialLease(resolveContext)
	require.Equal(t, http.StatusOK, resolveRecorder.Code, resolveRecorder.Body.String())
	var resolveResponse imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(resolveRecorder.Body.Bytes(), &resolveResponse))
	assert.Equal(t, "second-image-key", resolveResponse.APIKey)
	lease, exists, err = model.GetImageCredentialLeaseByAttemptRecordID(fixture.Attempt.ID)
	require.NoError(t, err)
	require.True(t, exists)
	require.NotNil(t, lease.ResolvedKeyIndex)
	assert.Equal(t, 1, *lease.ResolvedKeyIndex)

	failure := sendImageAttemptFailureCallbackWithError(t, fixture, "evt_multi_key_disable", &imageCallbackError{
		Code: "invalid_api_key", ProviderErrorCode: "invalid_api_key", Message: "invalid credential",
		ProviderErrorMessage: "invalid credential", Retryable: false, UpstreamStatus: http.StatusUnauthorized,
	})
	require.Equal(t, http.StatusOK, failure.Code)
	assert.Eventually(t, func() bool {
		stored, loadErr := model.GetChannelById(fixture.Attempt.ChannelID, true)
		return loadErr == nil && stored.ChannelInfo.MultiKeyStatusList[1] == common.ChannelStatusAutoDisabled
	}, time.Second, 10*time.Millisecond)
	stored, err := model.GetChannelById(fixture.Attempt.ChannelID, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.NotContains(t, stored.ChannelInfo.MultiKeyStatusList, 0)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[1])
}

func TestImageTaskAttemptRetryThenSuccessSettlesWinningRouteAndLog(t *testing.T) {
	fixture := setupImageRetryCallbackFixture(t, "retry_success", 1)
	failed := sendImageAttemptFailureCallback(t, fixture, "evt_retry_then_success_failure", true)
	require.Contains(t, failed.Body.String(), `"status":"retry_scheduled"`)
	attempts, err := model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)

	succeeded := sendImageAttemptSuccessCallback(t, fixture, attempts[1], "evt_retry_then_success_winner")

	require.Equal(t, http.StatusOK, succeeded.Code)
	assert.Contains(t, succeeded.Body.String(), `"status":"accepted"`)
	parent, err := model.GetTaskByRecordID(fixture.Parent.ID)
	require.NoError(t, err)
	assert.Equal(t, fixture.Parent.TaskID, parent.TaskID)
	assert.EqualValues(t, model.TaskStatusSuccess, parent.Status)
	assert.Equal(t, 124, parent.ChannelId)
	assert.Equal(t, "provider_retry_winner", parent.PrivateData.UpstreamTaskID)
	assert.Equal(t, 1000, getUserQuotaForControllerTest(t, 1))
	var assets []model.Asset
	require.NoError(t, model.DB.Where("task_record_id = ?", parent.ID).Find(&assets).Error)
	require.Len(t, assets, 1)
	assert.Equal(t, "https://cdn.example.test/retry-winner.webp", assets[0].URL)
	var finalLog model.Log
	require.NoError(t, model.LOG_DB.First(&finalLog, fixture.LogID).Error)
	assert.Equal(t, 124, finalLog.ChannelId)
	assert.Equal(t, 100, finalLog.Quota)
	other, err := common.StrToMap(finalLog.Other)
	require.NoError(t, err)
	assert.EqualValues(t, 2, other["attempt_count"])
	assert.EqualValues(t, 1, other["channel_switch_count"])
	var initialChannel, winningChannel model.Channel
	require.NoError(t, model.DB.First(&initialChannel, 123).Error)
	require.NoError(t, model.DB.First(&winningChannel, 124).Error)
	assert.Zero(t, initialChannel.UsedQuota)
	assert.EqualValues(t, 100, winningChannel.UsedQuota)

	late := sendImageAttemptFailureCallback(t, fixture, "evt_retry_then_success_late", true)
	assert.Contains(t, late.Body.String(), `"status":"ignored_terminal"`)
	attempts, err = model.ListImageTaskAttempts(fixture.Parent.ID)
	require.NoError(t, err)
	assert.Len(t, attempts, 2)
}

func TestImageTaskCallbackBatchAccepted(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_image_success",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		UserId:     1,
		ChannelId:  123,
		Quota:      100,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_success",
			BillingSource:  service.BillingSourceWallet,
			TokenId:        11,
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "gpt-image-2",
				PerCallBilling:  true,
				UsePrice:        true,
				BillingMode:     "async_image_usage_billing",
			},
		},
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}).Error)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "u", Quota: 120, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 11, UserId: 1, Key: "sk-callback-bill", Name: "callback token", Status: common.TokenStatusEnabled, RemainQuota: 50}).Error)

	const signedURL = "https://cdn.example.com/a.webp?x=1&X-Amz-Credential=AKIA%2F20260714%2Fs3%2Faws4_request&X-Amz-Signature=abc123"
	body := []byte(`{"events":[{"event_id":"evt_1","client_task_id":"task_image_success","provider_task_id":"imgtask_success","status":"succeeded","progress":"100%","result":{"images":[{"url":"https://cdn.example.com/a.webp?x=1\u0026X-Amz-Credential=AKIA%2F20260714%2Fs3%2Faws4_request\u0026X-Amz-Signature=abc123","mime_type":"image/webp","format":"webp","width":1024,"height":768,"size_bytes":123456,"filename":"a.webp","revised_prompt":"revised prompt"}],"output":{"quality":"high","output_format":"webp","size":"1024x768"},"metadata":{"image_count":1,"input_image_count":0,"mask_used":false}},"usage":{"actual_quota":300}}]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"accepted"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_success").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, signedURL, task.PrivateData.ResultURL)
	assert.Equal(t, 100, task.Quota)
	var user model.User
	require.NoError(t, db.Select("quota").Where("id = ?", 1).First(&user).Error)
	assert.Equal(t, 120, user.Quota)
	var token model.Token
	require.NoError(t, db.Select("remain_quota, used_quota").Where("id = ?", 11).First(&token).Error)
	assert.Equal(t, 50, token.RemainQuota)
	assert.Equal(t, 0, token.UsedQuota)
	assert.Contains(t, string(task.Data), `"format":"webp"`)
	assert.Contains(t, string(task.Data), `"output_format":"webp"`)
	var asset model.Asset
	require.NoError(t, db.Where("task_id = ?", "task_image_success").First(&asset).Error)
	assert.Equal(t, signedURL, asset.URL)
	assert.Equal(t, "image/webp", asset.MimeType)
	assert.Equal(t, "a.webp", asset.Filename)
	assert.EqualValues(t, 123456, asset.SizeBytes)
	assert.Equal(t, 1024, asset.Width)
	assert.Equal(t, 768, asset.Height)
	assert.Equal(t, "webp", asset.Metadata["format"])
	assert.Equal(t, "revised prompt", asset.Metadata["revised_prompt"])
}

func TestCallbackUsageToDTOKeepsGeminiReasoningTokens(t *testing.T) {
	usage := callbackUsageToDTO(&imageCallbackUsage{
		InputTokens:             27,
		OutputTokens:            1125,
		TotalTokens:             1152,
		ImageTokens:             5,
		CompletionTokensDetails: &dto.OutputTokenDetails{ReasoningTokens: 5},
	})

	require.NotNil(t, usage)
	assert.Equal(t, 27, usage.PromptTokens)
	assert.Equal(t, 1125, usage.CompletionTokens)
	assert.Equal(t, 5, usage.PromptTokensDetails.ImageTokens)
	assert.Equal(t, 5, usage.CompletionTokenDetails.ReasoningTokens)
}

func TestResolveImageCredentialLeaseAccepted(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	baseURL := "https://real.example/v1"
	require.NoError(t, db.Create(&model.Channel{
		Id:          777,
		Type:        constant.ChannelTypeOpenAI,
		Name:        "real-openai-image",
		Key:         "real-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-image-2",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_lease_resolve",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 777,
		Status:    model.TaskStatusQueued,
		Progress:  "0%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_lease",
		},
		Properties: model.Properties{
			OriginModelName:   "gpt-image-2",
			UpstreamModelName: "gpt-image-2",
		},
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_resolve",
		TaskID:       "task_lease_resolve",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    777,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"provider_task_id":"imgtask_lease","client_task_id":"task_lease_resolve","attempt":1,"operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_resolve", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resolveResp imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resolveResp))
	assert.Equal(t, "openai_compatible", resolveResp.Provider)
	assert.Equal(t, "openai_images", resolveResp.RequestFormat)
	assert.Equal(t, "https://real.example/v1", resolveResp.BaseURL)
	assert.Equal(t, "real-upstream-key", resolveResp.APIKey)
	assert.Equal(t, "gpt-image-2", resolveResp.Model)
	assert.Equal(t, "channel_777", resolveResp.ChannelID)
	assert.Equal(t, dto.ImageHandleExecutionDriverLegacySync, resolveResp.ExecutionDriver)
	assert.NotEmpty(t, resolveResp.ExpiresAt)

	var persistedTask model.Task
	require.NoError(t, db.Where("task_id = ?", "task_lease_resolve").First(&persistedTask).Error)
	persistedTask.PrivateData.ImageHandleExecutionDriver = dto.ImageHandleExecutionDriverAdobeAsyncImage
	require.NoError(t, db.Save(&persistedTask).Error)
	secondContext, secondRecorder := makeLeaseResolveRequest(t, "lease_resolve", body, "image_handle_1", secret)
	ResolveImageCredentialLease(secondContext)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	require.NoError(t, common.Unmarshal(secondRecorder.Body.Bytes(), &resolveResp))
	assert.Equal(t, dto.ImageHandleExecutionDriverAdobeAsyncImage, resolveResp.ExecutionDriver)
}

func TestResolveImageCredentialLeaseUsesActiveAttemptIdentityAndChannel(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	secret := "attempt-internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()
	_, _, attempt, lease := seedAttemptLeaseResolveFixture(t, "active")
	body := []byte(fmt.Sprintf(
		`{"provider_task_id":%q,"client_task_id":%q,"attempt":2,"operation":"generation","model":"gpt-image-2"}`,
		attempt.ProviderTaskID, attempt.ClientTaskID,
	))
	ctx, recorder := makeLeaseResolveRequest(t, lease.LeaseID, body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "attempt-key-active", response.APIKey)
	assert.Equal(t, "attempt-mapped-image", response.Model)
	assert.Equal(t, fmt.Sprintf("channel_%d", attempt.ChannelID), response.ChannelID)
	assert.Equal(t, dto.ImageHandleExecutionDriverLegacySync, response.ExecutionDriver)
}

func TestResolveImageCredentialLeaseRejectsInactiveAttempt(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	secret := "attempt-internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()
	_, state, attempt, lease := seedAttemptLeaseResolveFixture(t, "inactive")
	state.ActiveAttemptRecordID = 0
	require.NoError(t, model.DB.Save(state).Error)
	body := []byte(fmt.Sprintf(
		`{"provider_task_id":%q,"client_task_id":%q,"attempt":2,"operation":"generation","model":"gpt-image-2"}`,
		attempt.ProviderTaskID, attempt.ClientTaskID,
	))
	ctx, recorder := makeLeaseResolveRequest(t, lease.LeaseID, body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "attempt_not_active")
	storedLease, exists, err := model.GetImageCredentialLeaseByLeaseID(lease.LeaseID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, model.ImageCredentialLeaseStatusFailed, storedLease.Status)
}

func TestResolveImageCredentialLeaseBuildsGeminiGenerateContentEndpoint(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()
	originalVersions := model_setting.GetGeminiSettings().VersionSettings
	model_setting.GetGeminiSettings().VersionSettings = map[string]string{
		"default":                "v1beta",
		"gemini-3.1-flash-image": "v1",
	}
	t.Cleanup(func() {
		model_setting.GetGeminiSettings().VersionSettings = originalVersions
	})

	baseURL := "https://generativelanguage.googleapis.com"
	require.NoError(t, db.Create(&model.Channel{
		Id:          780,
		Type:        constant.ChannelTypeGemini,
		Name:        "gemini-image",
		Key:         "gemini-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gemini-3.1-flash-image",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_gemini_image",
		TaskID:       "task_gemini_image",
		TaskRecordID: 0,
		UserID:       1,
		ChannelID:    780,
		Operation:    "generation",
		Model:        "vendor-flash-image-v7",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"client_task_id":"task_gemini_image","operation":"generation","model":"gemini-3.1-flash-image"}`)
	context, recorder := makeLeaseResolveRequest(t, "lease_gemini_image", body, "image_handle_1", secret)

	ResolveImageCredentialLease(context)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "google_gemini", response.Provider)
	assert.Equal(t, "gemini_generate_content", response.RequestFormat)
	assert.Equal(t, baseURL, response.BaseURL)
	assert.Equal(t, baseURL+"/v1/models/vendor-flash-image-v7:generateContent", response.EndpointURL)
	assert.Equal(t, "vendor-flash-image-v7", response.Model)
	assert.Equal(t, "gemini-upstream-key", response.APIKey)
}

func TestResolveImageCredentialLeaseRejectsUnsupportedGeminiImageModel(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	baseURL := "https://generativelanguage.googleapis.com"
	require.NoError(t, db.Create(&model.Channel{
		Id:          781,
		Type:        constant.ChannelTypeGemini,
		Name:        "gemini-image-unsupported",
		Key:         "gemini-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gemini-3-pro-image-preview",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_gemini_unsupported",
		TaskID:       "task_gemini_unsupported",
		TaskRecordID: 0,
		UserID:       1,
		ChannelID:    781,
		Operation:    "generation",
		Model:        "gemini-3.1-flash-image",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"client_task_id":"task_gemini_unsupported","operation":"generation","model":"gemini-3-pro-image-preview"}`)
	context, recorder := makeLeaseResolveRequest(t, "lease_gemini_unsupported", body, "image_handle_1", secret)

	ResolveImageCredentialLease(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"model_not_supported"`)
	assert.NotContains(t, recorder.Body.String(), "gemini-upstream-key")
}

func TestResolveImageCredentialLeaseSyncLeaseDebugWithoutTaskRecord(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()
	image_handle_setting.GetImageHandleSetting().DebugUpstream = true
	t.Cleanup(func() {
		image_handle_setting.GetImageHandleSetting().DebugUpstream = false
	})

	baseURL := "https://real.example/v1"
	settings, err := common.Marshal(dto.ChannelOtherSettings{
		ImageHandleExecutionDriver: dto.ImageHandleExecutionDriverAdobeAsyncImage,
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            787,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "sync-openai-image",
		Key:           "real-upstream-key",
		BaseURL:       &baseURL,
		Status:        common.ChannelStatusEnabled,
		Models:        "gpt-image-2",
		Group:         "default",
		CreatedTime:   time.Now().Unix(),
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_sync_debug",
		TaskID:       "task_sync_debug",
		TaskRecordID: 0,
		UserID:       1,
		ChannelID:    787,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"provider_task_id":"imgtask_sync_debug","client_task_id":"task_sync_debug","attempt":1,"operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_sync_debug", body, "image_handle_1", secret)

	require.NotPanics(t, func() {
		ResolveImageCredentialLease(ctx)
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	var resolveResp imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resolveResp))
	assert.Equal(t, "openai_compatible", resolveResp.Provider)
	assert.Equal(t, "openai_images", resolveResp.RequestFormat)
	assert.Equal(t, dto.ImageHandleExecutionDriverAdobeAsyncImage, resolveResp.ExecutionDriver)
	assert.Equal(t, "real-upstream-key", resolveResp.APIKey)
}

func TestResolveImageCredentialLeaseNormalizesImagesEndpointBaseURL(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	baseURL := "https://real.example/v1/images/generations"
	require.NoError(t, db.Create(&model.Channel{
		Id:          778,
		Type:        constant.ChannelTypeOpenAI,
		Name:        "real-openai-image-endpoint-url",
		Key:         "real-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-image-2",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_lease_endpoint_url",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 778,
		Status:    model.TaskStatusQueued,
		Progress:  "0%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_endpoint_url",
		},
		Properties: model.Properties{
			OriginModelName:   "gpt-image-2",
			UpstreamModelName: "gpt-image-2",
		},
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_endpoint_url",
		TaskID:       "task_lease_endpoint_url",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    778,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"provider_task_id":"imgtask_endpoint_url","client_task_id":"task_lease_endpoint_url","attempt":1,"operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_endpoint_url", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resolveResp imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resolveResp))
	assert.Equal(t, "https://real.example/v1", resolveResp.BaseURL)
}

func TestResolveImageCredentialLeaseAddsV1ForOpenAICompatibleHostBaseURL(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	baseURL := "http://104.194.8.24:8008"
	require.NoError(t, db.Create(&model.Channel{
		Id:          779,
		Type:        constant.ChannelTypeOpenAI,
		Name:        "openai-compatible-host-url",
		Key:         "real-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-image-2",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_lease_host_url",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 779,
		Status:    model.TaskStatusQueued,
		Progress:  "0%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_host_url",
		},
		Properties: model.Properties{
			OriginModelName:   "gpt-image-2",
			UpstreamModelName: "gpt-image-2",
		},
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_host_url",
		TaskID:       "task_lease_host_url",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    779,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"provider_task_id":"imgtask_host_url","client_task_id":"task_lease_host_url","attempt":1,"operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_host_url", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resolveResp imageCredentialLeaseResolveResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resolveResp))
	assert.Equal(t, "http://104.194.8.24:8008/v1", resolveResp.BaseURL)
}

func TestResolveImageCredentialLeaseRejectsExpiredLease(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_expired_lease",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 777,
		Status:    model.TaskStatusQueued,
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_expired",
		TaskID:       "task_expired_lease",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    777,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(-time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"client_task_id":"task_expired_lease","operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_expired", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusGone, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "lease_expired")
	var lease model.ImageCredentialLease
	require.NoError(t, db.Where("lease_id = ?", "lease_expired").First(&lease).Error)
	assert.Equal(t, model.ImageCredentialLeaseStatusExpired, lease.Status)
}

func TestResolveImageCredentialLeaseRejectsFinishedTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_finished_lease",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 777,
		Status:    model.TaskStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_finished",
		TaskID:       "task_finished_lease",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    777,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"client_task_id":"task_finished_lease","operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_finished", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_already_finished")
}

func TestResolveImageCredentialLeaseRejectsDisabledChannel(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "internal-secret"
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", secret)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	require.NoError(t, db.Create(&model.Channel{
		Id:     777,
		Type:   constant.ChannelTypeOpenAI,
		Name:   "disabled-openai-image",
		Key:    "real-upstream-key",
		Status: common.ChannelStatusManuallyDisabled,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_disabled_channel",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 777,
		Status:    model.TaskStatusQueued,
	}).Error)
	require.NoError(t, db.Create(&model.ImageCredentialLease{
		LeaseID:      "lease_disabled_channel",
		TaskID:       "task_disabled_channel",
		TaskRecordID: 1,
		UserID:       1,
		ChannelID:    777,
		Operation:    "generation",
		Model:        "gpt-image-2",
		Status:       model.ImageCredentialLeaseStatusActive,
		ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}).Error)

	body := []byte(`{"client_task_id":"task_disabled_channel","operation":"generation","model":"gpt-image-2"}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_disabled_channel", body, "image_handle_1", secret)

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "channel_disabled")
}

func TestResolveImageCredentialLeaseRejectsBadSignature(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", "internal-secret")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	image_handle_setting.ApplyEnvFallback()

	body := []byte(`{"provider_task_id":"imgtask_internal","attempt":1}`)
	ctx, recorder := makeLeaseResolveRequest(t, "lease_resolve", body, "image_handle_1", "wrong-secret")

	ResolveImageCredentialLease(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "invalid internal signature")
}

func TestImageTaskCallbackBatchIgnoredTerminal(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_image_done",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_done",
			ResultURL:      "https://cdn.example.com/old.webp",
		},
	}).Error)

	body := []byte(`{"events":[{"event_id":"evt_2","client_task_id":"task_image_done","provider_task_id":"imgtask_done","status":"succeeded","progress":"100%","result":{"images":[{"url":"https://cdn.example.com/new.webp"}]}}]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"ignored_terminal"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_done").First(&task).Error)
	assert.Equal(t, "https://cdn.example.com/old.webp", task.PrivateData.ResultURL)
}

func TestImageTaskCallbackBatchRejectsStaleProgressSequence(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id: 123, Type: constant.ChannelTypeOpenAI, Name: "real-openai-image",
		Key: "real-upstream-key", Status: common.ChannelStatusEnabled, OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task_image_sequence", Platform: constant.TaskPlatform("58"),
		Action: constant.TaskActionImageGeneration, UserId: 1, ChannelId: 123,
		Status: model.TaskStatusInProgress, Progress: "55%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_sequence", ProgressMetadataSet: true,
			ProgressKnown: true, ProgressSource: "upstream_percent",
			ProgressStage: "generating", ProgressSequence: 5,
		},
	}).Error)

	known := true
	staleBody, err := common.Marshal(imageCallbackBatchRequest{Events: []imageCallbackEvent{{
		EventID: "evt_stale", ClientTaskID: "task_image_sequence", ProviderTaskID: "imgtask_sequence",
		Status: "processing", Progress: "40%", ProgressKnown: &known,
		ProgressSource: "upstream_percent", Stage: "generating", Sequence: 4,
	}}})
	require.NoError(t, err)
	context, recorder := makeCallbackRequest(t, staleBody, secret)
	ImageTaskCallbackBatch(context)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"ignored_stale"`)

	var persisted model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_sequence").First(&persisted).Error)
	assert.Equal(t, "55%", persisted.Progress)
	assert.EqualValues(t, 5, persisted.PrivateData.ProgressSequence)

	freshBody, err := common.Marshal(imageCallbackBatchRequest{Events: []imageCallbackEvent{{
		EventID: "evt_fresh", ClientTaskID: "task_image_sequence", ProviderTaskID: "imgtask_sequence",
		Status: "processing", Progress: "60%", ProgressKnown: &known,
		ProgressSource: "upstream_percent", Stage: "generating", Sequence: 6,
	}}})
	require.NoError(t, err)
	context, recorder = makeCallbackRequest(t, freshBody, secret)
	ImageTaskCallbackBatch(context)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"accepted"`)
	require.NoError(t, db.Where("task_id = ?", "task_image_sequence").First(&persisted).Error)
	assert.Equal(t, "60%", persisted.Progress)
	assert.EqualValues(t, 6, persisted.PrivateData.ProgressSequence)
}

func TestImageTaskCallbackBatchAcceptsProviderTaskBeforeUpstreamIDPersisted(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "u", Quota: 1000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_fast_callback",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		UserId:     1,
		ChannelId:  123,
		Quota:      100,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			BillingSource: service.BillingSourceWallet,
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "gpt-image-2",
			},
		},
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}).Error)

	body := []byte(`{"events":[{"event_id":"evt_fast","client_task_id":"task_fast_callback","provider_task_id":"imgtask_fast","status":"succeeded","progress":"100%","result":{"images":[{"url":"https://cdn.example.com/fast.webp"}]},"usage":{"actual_quota":100}}]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"accepted"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_fast_callback").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, "https://cdn.example.com/fast.webp", task.PrivateData.ResultURL)
}

type fastCallbackRelayOutcome struct {
	Task         model.Task
	User         model.User
	Token        model.Token
	Channel      model.Channel
	Logs         []model.Log
	ResponseCode int
}

func runRelayTaskWithFastCallback(t *testing.T, event imageCallbackEvent) fastCallbackRelayOutcome {
	return runRelayTaskWithFastCallbackSubmitResponse(t, event, http.StatusOK,
		`{"provider_task_id":"imgtask_controller_fast","client_task_id":"task_controller_fast","status":"queued"}`)
}

func runRelayTaskWithFastCallbackSubmitResponse(t *testing.T, event imageCallbackEvent, submitStatus int, submitBody string) fastCallbackRelayOutcome {
	t.Helper()
	db := setupInviteCodeControllerTestDB(t)
	service.InitHttpClient()

	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalImagePricing := ratio_setting.ImagePricing2JSONString()
	originalLogConsumeEnabled := common.LogConsumeEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalImagePricing))
		common.LogConsumeEnabled = originalLogConsumeEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
	})
	common.LogConsumeEnabled = true
	common.BatchUpdateEnabled = false
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(`{
		"version":1,
		"profiles":{
			"fast-callback-quality":{
				"name":"fast callback test",
				"parameter":"quality",
				"default_tier":"low",
				"tiers":[{"key":"low","upstream_value":"low","aliases":[],"unit_price":0.0002}]
			}
		},
		"model_bindings":{"public-fast-image":{"profile":"fast-callback-quality","max_n":10}}
	}`))

	const callbackSecret = "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: callbackSecret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "fast-callback-user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          11,
		UserId:      1,
		Key:         "sk-fast-callback",
		Name:        "fast callback token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 1000,
		Group:       "default",
	}).Error)

	event.EventID = "evt_controller_fast"
	event.ClientTaskID = "task_controller_fast"
	event.ProviderTaskID = "imgtask_controller_fast"
	if event.Progress == "" {
		event.Progress = "100%"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackBody, marshalErr := common.Marshal(imageCallbackBatchRequest{Events: []imageCallbackEvent{event}})
		require.NoError(t, marshalErr)
		callbackCtx, callbackRecorder := makeCallbackRequest(t, callbackBody, callbackSecret)
		ImageTaskCallbackBatch(callbackCtx)
		require.Equal(t, http.StatusOK, callbackRecorder.Code)
		require.Contains(t, callbackRecorder.Body.String(), `"status":"accepted"`)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(submitStatus)
		_, _ = w.Write([]byte(submitBody))
	}))
	defer server.Close()

	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:                 server.URL,
		APIKey:                  "provider-key",
		InternalBaseURL:         "http://new-api:3000",
		InternalSecretID:        "image_handle_1",
		InternalSecret:          "internal-secret",
		CallbackSecret:          callbackSecret,
		UsagePrechargeEnabled:   true,
		PrechargeAmountPerImage: 0.5,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader([]byte(`{
		"client_task_id":"task_controller_fast",
		"model":"public-fast-image",
		"prompt":"finish before submit response"
	}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("platform", "58")
	ctx.Set("model_mapping", `{"public-fast-image":"gpt-image-2"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserId, 1)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 1000)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenId, 11)
	common.SetContextKey(ctx, constant.ContextKeyTokenKey, "sk-fast-callback")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "public-fast-image")
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 123)
	common.SetContextKey(ctx, constant.ContextKeyChannelName, "real-openai-image")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, "https://real.example/v1")
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "real-upstream-key")
	ctx.Set(common.RequestIdKey, "req-controller-fast")

	RelayTask(ctx)

	outcome := fastCallbackRelayOutcome{ResponseCode: recorder.Code}
	require.NoError(t, db.Where("task_id = ?", "task_controller_fast").First(&outcome.Task).Error)
	require.NoError(t, db.First(&outcome.User, 1).Error)
	require.NoError(t, db.First(&outcome.Token, 11).Error)
	require.NoError(t, db.First(&outcome.Channel, 123).Error)
	require.NoError(t, db.Order("id asc").Find(&outcome.Logs).Error)
	return outcome
}

func TestRelayTaskPreservesFastCallbackResultWhenPersistingUpstreamID(t *testing.T) {
	outcome := runRelayTaskWithFastCallback(t, imageCallbackEvent{
		Status: "succeeded",
		Result: &imageCallbackResult{Images: []imageCallbackImage{{
			URL: "https://cdn.example.com/controller-fast.webp",
		}}},
		Usage: &imageCallbackUsage{ActualQuota: 999},
	})

	require.Equal(t, http.StatusOK, outcome.ResponseCode)
	assert.EqualValues(t, model.TaskStatusSuccess, outcome.Task.Status)
	assert.Equal(t, "imgtask_controller_fast", outcome.Task.PrivateData.UpstreamTaskID)
	assert.Equal(t, "https://cdn.example.com/controller-fast.webp", outcome.Task.PrivateData.ResultURL)
	assert.Contains(t, string(outcome.Task.Data), "controller-fast.webp")
	require.NotNil(t, outcome.Task.PrivateData.BillingContext)
	require.NotNil(t, outcome.Task.PrivateData.BillingContext.ImagePricing)
	assert.Equal(t, "public-fast-image", outcome.Task.PrivateData.BillingContext.ImagePricing.PublicModel)
	expectedQuota := common.QuotaFromFloat(0.0002 * common.QuotaPerUnit)
	assert.Equal(t, 1000-expectedQuota, outcome.User.Quota)
	assert.Equal(t, 1000-expectedQuota, outcome.Token.RemainQuota)
	assert.Equal(t, expectedQuota, outcome.Token.UsedQuota)
}

func TestRelayTaskFastFailureCallbackRefundsExactlyOnceBeforeSubmitSettlement(t *testing.T) {
	outcome := runRelayTaskWithFastCallback(t, imageCallbackEvent{
		Status: "failed",
		Error: &imageCallbackError{
			Code:                 "new_api_error",
			Message:              "internal upstream quota failure",
			UpstreamStatus:       http.StatusForbidden,
			ProviderErrorCode:    "insufficient_user_quota",
			ProviderErrorType:    "new_api_error",
			ProviderErrorMessage: "internal upstream quota failure",
			ProviderErrorParam:   "quota",
			UpstreamError:        json.RawMessage(`{"error":{"code":"insufficient_user_quota"}}`),
		},
		Usage: &imageCallbackUsage{ActualQuota: 999},
	})

	require.Equal(t, http.StatusOK, outcome.ResponseCode)
	assert.EqualValues(t, model.TaskStatusFailure, outcome.Task.Status)
	assert.Equal(t, "100%", outcome.Task.Progress)
	assert.Equal(t, "internal upstream quota failure", outcome.Task.FailReason)
	assert.Equal(t, "imgtask_controller_fast", outcome.Task.PrivateData.UpstreamTaskID)
	assert.Contains(t, string(outcome.Task.Data), `"provider_error_code":"insufficient_user_quota"`)
	assert.Contains(t, string(outcome.Task.Data), `"provider_error_param":"quota"`)
	assert.Contains(t, string(outcome.Task.Data), `"upstream_error":{"error":{"code":"insufficient_user_quota"}}`)
	require.NotNil(t, outcome.Task.PrivateData.BillingContext)
	require.NotNil(t, outcome.Task.PrivateData.BillingContext.ImagePricing)
	assert.Equal(t, "public-fast-image", outcome.Task.PrivateData.BillingContext.ImagePricing.PublicModel)

	publicTask, err := service.BuildPublicImageTask(&outcome.Task)
	require.NoError(t, err)
	require.NotNil(t, publicTask.Error)
	assert.Equal(t, "524", publicTask.Error.Code)
	assert.Equal(t, "Image generation service is temporarily unavailable. Please try again later.", publicTask.Error.Message)
	assert.True(t, publicTask.Error.Retryable)
	assert.NotContains(t, publicTask.Error.Message, "quota")

	expectedQuota := common.QuotaFromFloat(0.0002 * common.QuotaPerUnit)
	assert.Zero(t, outcome.Task.Quota)
	assert.Equal(t, 1000, outcome.User.Quota)
	assert.Equal(t, 1000, outcome.Token.RemainQuota)
	assert.Zero(t, outcome.Token.UsedQuota)

	require.Len(t, outcome.Logs, 1)
	assert.Equal(t, model.LogTypeError, outcome.Logs[0].Type)
	assert.Zero(t, outcome.Logs[0].Quota)
	assert.Equal(t, "req-controller-fast", outcome.Logs[0].RequestId)
	assert.Equal(t, 123, outcome.Logs[0].ChannelId)
	assert.Equal(t, "default", outcome.Logs[0].Group)
	assert.Contains(t, outcome.Logs[0].Content, "internal upstream quota failure")
	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(outcome.Logs[0].Other), &other))
	assert.Equal(t, "async_image_failed", other["billing_stage"])
	assert.Equal(t, float64(expectedQuota), other["pre_consumed_quota"])
	assert.Equal(t, "internal upstream quota failure", other["reason"])

	userLogs, total, err := model.GetUserLogs(1, model.LogTypeError, 0, 0, "", "", 0, 10, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, userLogs, 1)
	assert.Equal(t, "status_code=500, 系统异常，请稍后重试", userLogs[0].Content)
	assert.Zero(t, userLogs[0].ChannelId)
	assert.Empty(t, userLogs[0].ChannelName)
	assert.Empty(t, userLogs[0].Group)
	assert.NotContains(t, userLogs[0].Content, "quota")
	var userOther map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(userLogs[0].Other), &userOther))
	assert.Equal(t, "task_controller_fast", userOther["task_id"])
	assert.NotContains(t, userOther, "reason")
}

func TestRelayTaskFastSuccessCallbackOwnsBillingWhenSubmitReturnsError(t *testing.T) {
	outcome := runRelayTaskWithFastCallbackSubmitResponse(t, imageCallbackEvent{
		Status: "succeeded",
		Result: &imageCallbackResult{Images: []imageCallbackImage{{
			URL: "https://cdn.example.com/controller-fast-error.webp",
		}}},
	}, http.StatusBadGateway, `{"error":"response lost after callback"}`)

	require.Equal(t, http.StatusOK, outcome.ResponseCode)
	assert.EqualValues(t, model.TaskStatusSuccess, outcome.Task.Status)
	assert.Equal(t, "https://cdn.example.com/controller-fast-error.webp", outcome.Task.PrivateData.ResultURL)
	expectedQuota := common.QuotaFromFloat(0.0002 * common.QuotaPerUnit)
	assert.Equal(t, 1000-expectedQuota, outcome.User.Quota)
	assert.Equal(t, 1000-expectedQuota, outcome.Token.RemainQuota)
	assert.Equal(t, expectedQuota, outcome.Token.UsedQuota)

	for _, log := range outcome.Logs {
		assert.NotEqual(t, model.LogTypeRefund, log.Type)
	}
}

func TestRelayTaskFastFailureCallbackDoesNotDoubleRefundWhenSubmitResponseIsMalformed(t *testing.T) {
	outcome := runRelayTaskWithFastCallbackSubmitResponse(t, imageCallbackEvent{
		Status: "failed",
		Error: &imageCallbackError{
			Code:    "render_failed",
			Message: "render failed before malformed submit response",
		},
	}, http.StatusOK, `{"provider_task_id":`)

	require.Equal(t, http.StatusOK, outcome.ResponseCode)
	assert.EqualValues(t, model.TaskStatusFailure, outcome.Task.Status)
	assert.Equal(t, 1000, outcome.User.Quota)
	assert.Equal(t, 1000, outcome.Token.RemainQuota)
	assert.Zero(t, outcome.Token.UsedQuota)

	require.Len(t, outcome.Logs, 1)
	assert.Equal(t, model.LogTypeError, outcome.Logs[0].Type)
	assert.Zero(t, outcome.Logs[0].Quota)
	assert.Equal(t, "req-controller-fast", outcome.Logs[0].RequestId)
}

func TestImageTaskCallbackBatchRejectsChannelMismatch(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_image_other_channel",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 456,
		Status:    model.TaskStatusQueued,
		Progress:  "20%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_other_channel",
		},
	}).Error)

	body := []byte(`{"events":[{"event_id":"evt_channel","client_task_id":"task_image_other_channel","provider_task_id":"imgtask_other_channel","status":"succeeded","progress":"100%","result":{"images":[{"url":"https://cdn.example.com/a.webp"}]}}]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"channel_mismatch"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_other_channel").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusQueued, task.Status)
	assert.Empty(t, task.PrivateData.ResultURL)
}

func TestImageTaskCallbackBatchTruncatesOversizedRawResponse(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "u", Quota: 1000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:     "task_raw_response",
		Platform:   constant.TaskPlatform("58"),
		Action:     constant.TaskActionImageGeneration,
		UserId:     1,
		ChannelId:  123,
		Quota:      100,
		Status:     model.TaskStatusQueued,
		Progress:   "20%",
		SubmitTime: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "imgtask_raw",
			BillingSource:  service.BillingSourceWallet,
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "gpt-image-2",
			},
		},
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}).Error)

	oversized := bytes.Repeat([]byte("a"), rawResponseMaxBytes+1)
	event := imageCallbackBatchRequest{Events: []imageCallbackEvent{{
		EventID:        "evt_raw",
		ClientTaskID:   "task_raw_response",
		ProviderTaskID: "imgtask_raw",
		Status:         "succeeded",
		Progress:       "100%",
		Result:         &imageCallbackResult{Images: []imageCallbackImage{{URL: "https://cdn.example.com/raw.webp"}}},
		RawResponse:    append([]byte(`{"payload":"`), append(oversized, []byte(`"}`)...)...),
	}}}
	body, err := common.Marshal(event)
	require.NoError(t, err)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"accepted"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_raw_response").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Less(t, len(task.Data), rawResponseMaxBytes/4)
	assert.Contains(t, string(task.Data), `"raw_response_truncated":true`)
	assert.Contains(t, string(task.Data), `"original_size_bytes"`)
}

func TestQueryImageTasksReturnsOnlyCurrentUserTasks(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	currentTask := &model.Task{
		TaskID:    "task_current_user",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    1,
		ChannelId: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.example.com/current.webp",
		},
	}
	require.NoError(t, db.Create(currentTask).Error)
	currentRequest, err := common.Marshal(dto.ImageTaskCreateRequest{
		Model: "gpt-image-2", Operation: "generation", Input: dto.ImageTaskInputRequest{Prompt: "draw"},
		ClientReferenceID: "order_current",
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewImageTaskRequest(currentTask, 1, nil, "fingerprint-current", "order_current", currentRequest)).Error)
	otherTask := &model.Task{
		TaskID:    "task_other_user",
		Platform:  constant.TaskPlatform("58"),
		Action:    constant.TaskActionImageGeneration,
		UserId:    2,
		ChannelId: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.example.com/other.webp",
		},
	}
	require.NoError(t, db.Create(otherTask).Error)
	otherRequest, err := common.Marshal(dto.ImageTaskCreateRequest{
		Model: "gpt-image-2", Operation: "generation", Input: dto.ImageTaskInputRequest{Prompt: "other"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewImageTaskRequest(otherTask, 2, nil, "fingerprint-other", "", otherRequest)).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_video_same_user",
		Platform:  constant.TaskPlatform("48"),
		Action:    constant.TaskActionVideoGeneration,
		UserId:    1,
		ChannelId: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.example.com/video.mp4",
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks/query", bytes.NewReader([]byte(`{"task_ids":["task_current_user","task_other_user","task_video_same_user"]}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)

	QueryImageTasks(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"id":"task_current_user"`)
	assert.Contains(t, recorder.Body.String(), `"object":"image.task"`)
	assert.Contains(t, recorder.Body.String(), `"client_reference_id":"order_current"`)
	assert.Contains(t, recorder.Body.String(), `"missing":["task_other_user","task_video_same_user"]`)
	assert.NotContains(t, recorder.Body.String(), "channel_id")
	assert.NotContains(t, recorder.Body.String(), "user_id")
	assert.NotContains(t, recorder.Body.String(), "quota")
	assert.NotContains(t, recorder.Body.String(), "other.webp")
	assert.NotContains(t, recorder.Body.String(), "video.mp4")
}

func TestQueryImageTasksRejectsMoreThanOneHundredIDs(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	taskIDs := make([]string, 101)
	for i := range taskIDs {
		taskIDs[i] = fmt.Sprintf("task_%03d", i)
	}
	body, err := common.Marshal(imageTaskQueryRequest{TaskIDs: taskIDs})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks/query", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)

	QueryImageTasks(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"invalid_request"`)
	assert.Contains(t, recorder.Body.String(), "between 1 and 100")
}

func TestGetImageTaskRejectsNonImageHandleTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_video_same_user",
		Platform:  constant.TaskPlatform("48"),
		Action:    constant.TaskActionVideoGeneration,
		UserId:    1,
		ChannelId: 123,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/image/tasks/task_video_same_user", nil)
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_video_same_user"}}
	ctx.Set("id", 1)

	GetImageTask(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"task_not_found"`)
}

func TestVerifyImageCallbackRejectsInvalidSignature(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeOpenAI,
		Name:          "real-openai-image",
		Key:           "real-upstream-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	body := []byte(`{"events":[]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)
	ctx.Request.Header.Set("X-Callback-Signature", "bad-signature")

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "invalid callback signature")
}

func TestVerifyImageCallbackRejectsExpiredTimestamp(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	secret := "callback-secret"
	settings, err := common.Marshal(dto.ChannelOtherSettings{CallbackSecret: secret})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Channel{
		Id:            123,
		Type:          constant.ChannelTypeImageHandle,
		Name:          "image-handle",
		Key:           "provider-key",
		Status:        common.ChannelStatusEnabled,
		OtherSettings: string(settings),
	}).Error)
	body := []byte(`{"events":[]}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	timestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/task/callback/external-image/batch", bytes.NewReader(body))
	ctx.Request.Header.Set("X-Callback-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Callback-Signature", signCallbackTestBody(timestamp, body, secret))
	ctx.Request.Header.Set("X-Callback-Secret-Id", "channel_123")

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "callback timestamp expired")
}

func TestVerifyImageCallbackRejectsMissingSecretID(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	body := []byte(`{"events":[]}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/task/callback/external-image/batch", bytes.NewReader(body))
	ctx.Request.Header.Set("X-Callback-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Callback-Signature", signCallbackTestBody(timestamp, body, "callback-secret"))
	ctx.Request.Header.Set("X-Callback-Secret-Id", "channel_999")

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "callback secret not found")
}
