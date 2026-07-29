package adobevideo

import (
	"context"
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
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func adobeVideoTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/tasks", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestPrepareNormalizedVideoRequestSharesValidatedPayloadWithBillingAndUpstream(t *testing.T) {
	duration := 4
	aspectRatio := "16:9"
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-fast-480p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "  cinematic ocean sunrise  "},
		Output: dto.VideoTaskOutputRequest{
			Duration:    &duration,
			AspectRatio: &aspectRatio,
		},
		ProviderOptions: map[string]map[string]any{
			ProviderOptionsNamespace: {"generate_audio": false},
		},
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "seedance_2.0_fast_480p",
		},
	}
	c := adobeVideoTestContext()
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	assert.Equal(t, constant.TaskActionVideoGeneration, info.Action)
	assert.Equal(t, request.Model, info.OriginModelName)
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(c, info))

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 4, estimate.Seconds)
	assert.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "seedance_2.0_fast_480p", payload["model"])
	assert.Equal(t, "cinematic ocean sunrise", payload["prompt"])
	assert.EqualValues(t, 4, payload["duration"])
	assert.Equal(t, "16:9", payload["aspect_ratio"])
	assert.Equal(t, false, payload["generate_audio"])
}

func TestPrepareNormalizedVideoRequestUsesAdobeDefaultsWithoutInventingResolution(t *testing.T) {
	duration := 5
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-720p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "clouds"},
		Output:    dto.VideoTaskOutputRequest{Duration: &duration},
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "seedance_2.0_720p"},
	}
	c := adobeVideoTestContext()
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, defaultAspectRatio, payload["aspect_ratio"])
	assert.NotContains(t, payload, "resolution")
	assert.NotContains(t, payload, "generate_audio")
}

func TestPrepareNormalizedVideoRequestRejectsInvalidInputBeforeDispatch(t *testing.T) {
	validDuration := 4
	tooShort := 3
	tooLong := 16
	resolution := "480p"
	tests := []struct {
		name    string
		request dto.VideoTaskCreateRequest
		code    string
	}{
		{
			name:    "missing duration",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}},
			code:    "video_duration_required",
		},
		{
			name:    "duration below provider minimum",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, Output: dto.VideoTaskOutputRequest{Duration: &tooShort}},
			code:    "invalid_video_duration",
		},
		{
			name:    "duration above provider maximum",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, Output: dto.VideoTaskOutputRequest{Duration: &tooLong}},
			code:    "invalid_video_duration",
		},
		{
			name:    "resolution must come from exact model",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration, Resolution: &resolution}},
			code:    "invalid_video_parameter",
		},
		{
			name:    "image input unsupported",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat", Image: &dto.VideoTaskSource{URL: "https://example.com/cat.png"}}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration}},
			code:    "unsupported_video_input",
		},
		{
			name:    "operation unsupported",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "edit", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration}},
			code:    "unsupported_video_operation",
		},
		{
			name: "duration override rejected",
			request: dto.VideoTaskCreateRequest{
				Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &validDuration},
				ProviderOptions: map[string]map[string]any{
					ProviderOptionsNamespace: {"duration": 8},
				},
			},
			code: "invalid_provider_options",
		},
		{
			name: "non boolean audio option rejected",
			request: dto.VideoTaskCreateRequest{
				Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &validDuration},
				ProviderOptions: map[string]map[string]any{
					ProviderOptionsNamespace: {"generate_audio": "false"},
				},
			},
			code: "invalid_provider_options",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
				adobeVideoTestContext(),
				&relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}},
				test.request,
			)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestValidateNormalizedVideoModelRequiresExactProviderSKU(t *testing.T) {
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(
		adobeVideoTestContext(),
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance_2.0_fast_480p"}},
	))

	taskErr := adaptor.ValidateNormalizedVideoModel(
		adobeVideoTestContext(),
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-fast-480p"}},
	)
	require.NotNil(t, taskErr)
	assert.Equal(t, "unsupported_video_model", taskErr.Code)
}

func TestDoResponseAndParseTaskLifecycle(t *testing.T) {
	adaptor := &TaskAdaptor{}
	submit := `{"id":"provider-task-1","task_id":"provider-task-1","status":"queued","duration":4}`
	taskID, taskData, taskErr := adaptor.DoResponse(
		adobeVideoTestContext(),
		&http.Response{Body: io.NopCloser(strings.NewReader(submit))},
		nil,
	)
	require.Nil(t, taskErr)
	assert.Equal(t, "provider-task-1", taskID)
	assert.JSONEq(t, submit, string(taskData))

	queued, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"queued","progress":0}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusQueued, queued.Status)

	inProgress, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"in_progress","progress":47}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, inProgress.Status)
	assert.Equal(t, "47%", inProgress.Progress)

	completed, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"completed","progress":100,"duration":4,"video_url":"http://private.example/generated/provider-task-1.mp4"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, completed.Status)
	require.Len(t, completed.VideoOutputs, 1)
	assert.Empty(t, completed.VideoOutputs[0].URL)
	assert.Equal(t, "provider-task-1", completed.VideoOutputs[0].ProviderReference)
	assert.Equal(t, videoContentResolver, completed.VideoOutputs[0].Resolver)
	assert.EqualValues(t, 4000, completed.VideoOutputs[0].DurationMS)

	failed, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"failed","error":{"code":"provider_rejected","message":"request rejected"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, failed.Status)
	assert.Equal(t, "request rejected", failed.Reason)
}

func TestFetchTaskAndResolveVideoContentUseStoredCredentialAndRange(t *testing.T) {
	var statusAuth string
	var contentAuth string
	var contentRange string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos/provider-task-1":
			statusAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"provider-task-1","status":"completed"}`))
		case "/v1/videos/provider-task-1/content":
			contentAuth = r.Header.Get("Authorization")
			contentRange = r.Header.Get("Range")
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Range", "bytes 0-3/8")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	adaptor := &TaskAdaptor{}
	statusResponse, err := adaptor.FetchTask(upstream.URL, "poll-key", map[string]any{"task_id": "provider-task-1"}, "")
	require.NoError(t, err)
	require.NoError(t, statusResponse.Body.Close())
	assert.Equal(t, "Bearer poll-key", statusAuth)

	baseURL := upstream.URL
	providerChannel := &model.Channel{Key: "channel-key", BaseURL: &baseURL}
	task := &model.Task{
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "fallback-task",
			Key:            "selected-task-key",
		},
	}
	headers := make(http.Header)
	headers.Set("Range", "bytes=0-3")
	contentResponse, err := adaptor.ResolveVideoContent(
		context.Background(),
		providerChannel,
		task,
		relaycommon.VideoOutput{
			ProviderReference: "provider-task-1",
			Resolver:          videoContentResolver,
		},
		headers,
	)
	require.NoError(t, err)
	defer contentResponse.Body.Close()
	content, err := io.ReadAll(contentResponse.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusPartialContent, contentResponse.StatusCode)
	assert.Equal(t, "data", string(content))
	assert.Equal(t, "Bearer selected-task-key", contentAuth)
	assert.Equal(t, "bytes=0-3", contentRange)
}
