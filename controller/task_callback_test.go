package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
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
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "gpt-image-2",
			},
		},
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}).Error)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "u", Quota: 1000, Status: common.UserStatusEnabled}).Error)

	body := []byte(`{"events":[{"event_id":"evt_1","client_task_id":"task_image_success","provider_task_id":"imgtask_success","status":"succeeded","progress":"100%","result":{"images":[{"url":"https://cdn.example.com/a.webp"}]},"usage":{"actual_quota":100}}]}`)
	ctx, recorder := makeCallbackRequest(t, body, secret)

	ImageTaskCallbackBatch(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"accepted"`)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_image_success").First(&task).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, "https://cdn.example.com/a.webp", task.PrivateData.ResultURL)
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
	assert.NotEmpty(t, resolveResp.ExpiresAt)
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
	require.NoError(t, db.Create(&model.Task{
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
	}).Error)
	require.NoError(t, db.Create(&model.Task{
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
	}).Error)
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
	assert.Contains(t, recorder.Body.String(), `"task_id":"task_current_user"`)
	assert.Contains(t, recorder.Body.String(), `"result_url":"https://cdn.example.com/current.webp"`)
	assert.NotContains(t, recorder.Body.String(), "task_other_user")
	assert.NotContains(t, recorder.Body.String(), "other.webp")
	assert.NotContains(t, recorder.Body.String(), "task_video_same_user")
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

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_ids max size is 100")
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

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_not_exist")
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
