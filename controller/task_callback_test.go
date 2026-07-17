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
	require.NoError(t, db.Create(&model.Channel{
		Id:          787,
		Type:        constant.ChannelTypeOpenAI,
		Name:        "sync-openai-image",
		Key:         "real-upstream-key",
		BaseURL:     &baseURL,
		Status:      common.ChannelStatusEnabled,
		Models:      "gpt-image-2",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
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
			Code:    "render_failed",
			Message: "render failed before submit response",
		},
		Usage: &imageCallbackUsage{ActualQuota: 999},
	})

	require.Equal(t, http.StatusOK, outcome.ResponseCode)
	assert.EqualValues(t, model.TaskStatusFailure, outcome.Task.Status)
	assert.Equal(t, "100%", outcome.Task.Progress)
	assert.Equal(t, "render failed before submit response", outcome.Task.FailReason)
	assert.Equal(t, "imgtask_controller_fast", outcome.Task.PrivateData.UpstreamTaskID)
	assert.Contains(t, string(outcome.Task.Data), "render_failed")
	require.NotNil(t, outcome.Task.PrivateData.BillingContext)
	require.NotNil(t, outcome.Task.PrivateData.BillingContext.ImagePricing)
	assert.Equal(t, "public-fast-image", outcome.Task.PrivateData.BillingContext.ImagePricing.PublicModel)

	expectedQuota := common.QuotaFromFloat(0.0002 * common.QuotaPerUnit)
	assert.Equal(t, expectedQuota, outcome.Task.Quota)
	assert.Equal(t, 1000, outcome.User.Quota)
	assert.Equal(t, 1000, outcome.Token.RemainQuota)
	assert.Zero(t, outcome.Token.UsedQuota)

	refundLogs := make([]model.Log, 0, 1)
	for _, log := range outcome.Logs {
		if log.Type == model.LogTypeRefund {
			refundLogs = append(refundLogs, log)
		}
	}
	require.Len(t, refundLogs, 1)
	assert.Equal(t, expectedQuota, refundLogs[0].Quota)
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

	refundLogs := make([]model.Log, 0, 1)
	for _, log := range outcome.Logs {
		if log.Type == model.LogTypeRefund {
			refundLogs = append(refundLogs, log)
		}
	}
	require.Len(t, refundLogs, 1)
	assert.Equal(t, common.QuotaFromFloat(0.0002*common.QuotaPerUnit), refundLogs[0].Quota)
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
