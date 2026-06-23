package imagehandle

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyMatchesImageHandleContract(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	originalServerAddress := system_setting.ServerAddress
	originalCallbackAddress := operation_setting.CustomCallbackAddress
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
		system_setting.ServerAddress = originalServerAddress
		operation_setting.CustomCallbackAddress = originalCallbackAddress
	})
	system_setting.ServerAddress = "https://new-api.example"
	operation_setting.CustomCallbackAddress = ""
	t.Setenv("IMAGE_HANDLE_BASE_URL", "http://127.0.0.1:8787")
	t.Setenv("IMAGE_HANDLE_API_KEY", "provider-key")
	t.Setenv("IMAGE_HANDLE_INTERNAL_BASE_URL", "http://new-api:3000")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", "internal-secret")
	image_handle_setting.ApplyEnvFallback()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_external_id",
		"model":"gpt-image-2",
		"prompt":"a clean product photo",
		"size":"1024x1024",
		"metadata":{"quality":"high","n":1,"output_format":"webp","provider":"openai"}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req_test")
	c.Set("image_credential_lease_id", "lease_test")

	info := &relaycommon.RelayInfo{
		UserId:          11,
		OriginModelName: "gpt-image-2",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            123,
			UpstreamModelName:    "gpt-image-2",
			ChannelOtherSettings: dto.ChannelOtherSettings{CallbackSecret: "callback-secret"},
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "http://wrong-channel-url", ApiKey: "wrong-key"}})
	assert.Equal(t, "http://127.0.0.1:8787", adaptor.baseURL)
	assert.Equal(t, "provider-key", adaptor.apiKey)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, constant.TaskActionImageGeneration, info.Action)
	assert.Equal(t, "gpt-image-2", info.OriginModelName)
	assert.Equal(t, "task_external_id", info.PublicTaskID)

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.Equal(t, "req_test", payload["request_id"])
	assert.Equal(t, "task_external_id", payload["client_task_id"])
	assert.Equal(t, "gpt-image-2", payload["model"])
	assert.Equal(t, "generation", payload["operation"])
	input := payload["input"].(map[string]any)
	assert.Equal(t, "a clean product photo", input["text"])
	parameters := payload["parameters"].(map[string]any)
	assert.Equal(t, "1024x1024", parameters["size"])
	assert.Equal(t, "high", parameters["quality"])
	callback := payload["callback"].(map[string]any)
	assert.Equal(t, "https://new-api.example/api/task/callback/external-image/task_external_id", callback["url"])
	assert.Equal(t, "https://new-api.example/api/task/callback/external-image/batch", callback["batch_url"])
	assert.Equal(t, "channel_123", callback["secret_id"])
	executor := payload["executor"].(map[string]any)
	assert.Equal(t, "provider_direct_lease", executor["type"])
	assert.Equal(t, "lease_test", executor["lease_id"])
	assert.Equal(t, "http://new-api:3000/api/internal/image/credential-leases/lease_test/resolve", executor["resolve_url"])
	assert.Equal(t, "image_handle_1", executor["secret_id"])
}

func TestValidateExecutorConfigUsesGlobalCallbackSecretFallback(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	t.Setenv("IMAGE_HANDLE_BASE_URL", "http://127.0.0.1:8787")
	t.Setenv("IMAGE_HANDLE_API_KEY", "provider-key")
	t.Setenv("IMAGE_HANDLE_INTERNAL_BASE_URL", "http://new-api:3000")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", "internal-secret")
	t.Setenv("IMAGE_HANDLE_CALLBACK_SECRET", "fallback-callback-secret")
	image_handle_setting.ApplyEnvFallback()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            123,
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	err := (&TaskAdaptor{}).ValidateExecutorConfig(info)

	require.NoError(t, err)
	assert.Equal(t, "fallback-callback-secret", resolveImageHandleSubmitCallbackSecret(info))
}

func TestValidateExecutorConfigRejectsMissingCallbackSecret(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	t.Setenv("IMAGE_HANDLE_BASE_URL", "http://127.0.0.1:8787")
	t.Setenv("IMAGE_HANDLE_API_KEY", "provider-key")
	t.Setenv("IMAGE_HANDLE_INTERNAL_BASE_URL", "http://new-api:3000")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", "internal-secret")
	t.Setenv("IMAGE_HANDLE_CALLBACK_SECRET", "")
	image_handle_setting.ApplyEnvFallback()

	err := (&TaskAdaptor{}).ValidateExecutorConfig(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 123},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback_secret is required")
}

func TestValidateExecutorConfigRejectsGlobalCallbackSecretMatchingInternalSecret(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	})
	t.Setenv("IMAGE_HANDLE_BASE_URL", "http://127.0.0.1:8787")
	t.Setenv("IMAGE_HANDLE_API_KEY", "provider-key")
	t.Setenv("IMAGE_HANDLE_INTERNAL_BASE_URL", "http://new-api:3000")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET_ID", "image_handle_1")
	t.Setenv("IMAGE_HANDLE_INTERNAL_SECRET", "same-secret")
	t.Setenv("IMAGE_HANDLE_CALLBACK_SECRET", "same-secret")
	image_handle_setting.ApplyEnvFallback()

	err := (&TaskAdaptor{}).ValidateExecutorConfig(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 123},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be different")
}

func TestValidateRequestRejectsUnsafeClientTaskID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
		"client_task_id":"task_../bad",
		"model":"gpt-image-2",
		"prompt":"a clean product photo"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_request", taskErr.Code)
}

func TestParseTaskResultMapsImageHandleStatusAndUsage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info, err := adaptor.ParseTaskResult([]byte(`{
		"provider_task_id":"imgtask_1",
		"client_task_id":"task_1",
		"status":"succeeded",
		"progress":"100%",
		"result":{"images":[{"url":"https://cdn.example.com/a.webp"}]},
		"usage":{"total_tokens":12,"actual_quota":34}
	}`))
	require.NoError(t, err)
	assert.Equal(t, "imgtask_1", info.TaskID)
	assert.Equal(t, model.TaskStatusSuccess, info.Status)
	assert.Equal(t, "https://cdn.example.com/a.webp", info.Url)
	assert.Equal(t, 12, info.TotalTokens)
	assert.Equal(t, 34, info.CompletionTokens)
}

func TestParseBatchTaskResultIndexesByProviderTaskID(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseBatchTaskResult([]byte(`{"data":[
		{"provider_task_id":"imgtask_1","status":"processing","progress":"30%"},
		{"task_id":"imgtask_2","status":"failed","error":{"message":"upstream failed"}}
	]}`))
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, model.TaskStatusInProgress, result["imgtask_1"].Status)
	assert.Equal(t, "30%", result["imgtask_1"].Progress)
	assert.Equal(t, model.TaskStatusFailure, result["imgtask_2"].Status)
	assert.Equal(t, "upstream failed", result["imgtask_2"].Reason)
}

func TestDoResponseDoesNotExposeProviderTaskID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"provider_task_id":"imgtask_secret","client_task_id":"task_public","status":"queued"}`)),
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	upstreamTaskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "imgtask_secret", upstreamTaskID)
	assert.JSONEq(t, `{"provider_task_id":"imgtask_secret","client_task_id":"task_public","status":"queued"}`, string(taskData))
	assert.JSONEq(t, `{"status":"queued","task_id":"task_public"}`, w.Body.String())
}

func TestDoResponseAcceptsAcceptedStatusResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(strings.NewReader(`{"provider_task_id":"imgtask_accepted","client_task_id":"task_public","status":"queued"}`)),
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	upstreamTaskID, _, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "imgtask_accepted", upstreamTaskID)
	assert.JSONEq(t, `{"status":"queued","task_id":"task_public"}`, w.Body.String())
}
