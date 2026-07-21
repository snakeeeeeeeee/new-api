package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupImageDispatchTest(t *testing.T, handler http.HandlerFunc) (*model.Task, *model.ImageTaskDispatch) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.ImageTaskRequest{}, &model.ImageTaskDispatch{}, &model.Asset{},
		&model.WebhookEndpoint{}, &model.WebhookEvent{}, &model.WebhookDelivery{}, &model.WebhookDeliveryAttempt{},
	))
	originalClient := httpClient
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	server := httptest.NewServer(handler)
	httpClient = server.Client()
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL: server.URL,
		APIKey:  "dispatch-test-key",
	})
	t.Cleanup(func() {
		httpClient = originalClient
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		server.Close()
		model.DB.Where("task_id LIKE ?", "task_dispatch_test_%").Delete(&model.ImageTaskDispatch{})
		model.DB.Where("task_id LIKE ?", "task_dispatch_test_%").Delete(&model.ImageTaskRequest{})
		model.DB.Where("object_id LIKE ?", "task_dispatch_test_%").Delete(&model.WebhookEvent{})
		model.DB.Where("task_id LIKE ?", "task_dispatch_test_%").Delete(&model.Task{})
	})

	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_dispatch_test_" + time.Now().Format("150405.000000000"), UserId: 1, ChannelId: 1,
		Platform: constant.TaskPlatform("58"), Action: constant.TaskActionImageGeneration,
		Status: model.TaskStatusQueued, Progress: "0%", SubmitTime: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)
	dispatch := model.NewImageTaskDispatch(task, []byte(`{"client_task_id":"`+task.TaskID+`"}`))
	dispatch.Status = model.ImageTaskDispatchProcessing
	dispatch.Attempts = 1
	dispatch.LockToken = "dispatch-lock"
	dispatch.LockedUntil = now + 60
	require.NoError(t, model.DB.Create(dispatch).Error)
	return task, dispatch
}

func attachPublicImageTaskRequest(t *testing.T, task *model.Task) {
	t.Helper()
	requestJSON, err := common.Marshal(dto.ImageTaskCreateRequest{
		Model: "gpt-image-2", Operation: "generation",
		Input: dto.ImageTaskInputRequest{Prompt: "draw"},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(model.NewImageTaskRequest(
		task, task.UserId, nil, "dispatch-fingerprint", "", requestJSON,
	)).Error)
}

func TestProcessImageTaskDispatchPersistsSuccessfulSubmission(t *testing.T) {
	var taskID string
	task, dispatch := setupImageDispatchTest(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer dispatch-test-key", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"provider_task_id":"imgtask_dispatch_success","client_task_id":"` + taskID + `"}`))
	})
	taskID = task.TaskID

	processImageTaskDispatch(context.Background(), dispatch)

	var storedDispatch model.ImageTaskDispatch
	require.NoError(t, model.DB.First(&storedDispatch, dispatch.ID).Error)
	assert.Equal(t, model.ImageTaskDispatchDelivered, storedDispatch.Status)
	assert.Empty(t, storedDispatch.LockToken)
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.Equal(t, "imgtask_dispatch_success", storedTask.PrivateData.UpstreamTaskID)
}

func TestProcessImageTaskDispatchReschedulesRetryableFailure(t *testing.T) {
	_, dispatch := setupImageDispatchTest(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"error":{"message":"temporary outage"}}`))
	})
	before := time.Now().Unix()

	processImageTaskDispatch(context.Background(), dispatch)

	var stored model.ImageTaskDispatch
	require.NoError(t, model.DB.First(&stored, dispatch.ID).Error)
	assert.Equal(t, model.ImageTaskDispatchPending, stored.Status)
	assert.Empty(t, stored.LockToken)
	assert.Equal(t, http.StatusInternalServerError, stored.LastHTTPStatus)
	assert.Equal(t, "temporary outage", stored.LastError)
	assert.GreaterOrEqual(t, stored.NextAttemptAt, before+5)
}

func TestProcessImageTaskDispatchUsesDedicatedTimeout(t *testing.T) {
	_, dispatch := setupImageDispatchTest(t, func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		writer.WriteHeader(http.StatusAccepted)
	})
	result := processImageTaskDispatchWithTimeout(context.Background(), dispatch, 1)
	assert.True(t, result.timedOut)

	var stored model.ImageTaskDispatch
	require.NoError(t, model.DB.First(&stored, dispatch.ID).Error)
	assert.Equal(t, model.ImageTaskDispatchPending, stored.Status)
	assert.Contains(t, stored.LastError, "deadline exceeded")
	assert.Equal(t, async_task_setting.DefaultImageDispatchTimeoutSeconds, async_task_setting.GetSnapshot().ImageDispatchTimeoutSeconds)
}

func TestProcessImageTaskDispatchPermanentFailureRefundsAndCreatesWebhookEvent(t *testing.T) {
	truncate(t)
	seedUser(t, 1, 10_000)
	seedToken(t, 1, 1, "dispatch-refund-token", 5_000)
	seedChannel(t, 1)
	task, dispatch := setupImageDispatchTest(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"message":"invalid provider request"}}`))
	})
	task.Quota = 3_000
	task.PrivateData.BillingSource = BillingSourceWallet
	task.PrivateData.TokenId = 1
	task.PrivateData.BillingContext = &model.TaskBillingContext{OriginModelName: "gpt-image-2"}
	require.NoError(t, model.DB.Save(task).Error)
	attachPublicImageTaskRequest(t, task)

	processImageTaskDispatch(context.Background(), dispatch)

	var storedDispatch model.ImageTaskDispatch
	require.NoError(t, model.DB.First(&storedDispatch, dispatch.ID).Error)
	assert.Equal(t, model.ImageTaskDispatchFailed, storedDispatch.Status)
	assert.Equal(t, http.StatusBadRequest, storedDispatch.LastHTTPStatus)
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, storedTask.Status)
	assert.Equal(t, "invalid provider request", storedTask.FailReason)
	assert.Equal(t, 13_000, getUserQuota(t, 1))
	assert.Equal(t, 8_000, getTokenRemainQuota(t, 1))
	var event model.WebhookEvent
	require.NoError(t, model.DB.Where("object_id = ?", task.TaskID).First(&event).Error)
	assert.Equal(t, WebhookEventImageTaskFailed, event.EventType)
	assert.Contains(t, event.Payload, `"status":"failed"`)
}

func TestProcessImageTaskDispatchRetriesWhenTerminalTransactionFails(t *testing.T) {
	truncate(t)
	task, dispatch := setupImageDispatchTest(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
	})
	attachPublicImageTaskRequest(t, task)
	injectedErr := errors.New("injected terminal update failure")
	const callbackName = "test:fail_dispatch_terminal_update"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "tasks" {
			tx.AddError(injectedErr)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove(callbackName) })

	processImageTaskDispatch(context.Background(), dispatch)

	var storedDispatch model.ImageTaskDispatch
	require.NoError(t, model.DB.First(&storedDispatch, dispatch.ID).Error)
	assert.Equal(t, model.ImageTaskDispatchPending, storedDispatch.Status)
	assert.Greater(t, storedDispatch.NextAttemptAt, time.Now().Unix())
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusQueued, storedTask.Status)
}
