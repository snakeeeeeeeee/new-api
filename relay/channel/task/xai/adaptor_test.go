package xai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestContext(t *testing.T, path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestValidateAndBuildRequestBodyPassThroughOfficialFields(t *testing.T) {
	body := `{
		"model":"grok-imagine-video-1.5-preview-15s-720p",
		"prompt":"pan out",
		"image":{"url":"data:image/png;base64,abc"},
		"image_url":"https://example.com/frame.png",
		"reference_images":[{"file_id":"file_123"},{"url":"https://example.com/ref.png"}],
		"video":{"file_id":"file_video"},
		"duration":15,
		"resolution":"720p",
		"service_tier":"priority",
		"output":{"upload_url":"https://uploads.example.com/signed"},
		"user":"user-1"
	}`
	c, _ := buildTestContext(t, "/v1/videos/generations", body)
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-1.5-preview-15s-720p",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video",
			ChannelBaseUrl:    "https://api.x.ai",
			ApiKey:            "sk-test",
		},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionVideoGeneration, info.Action)

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "grok-imagine-video", payload["model"])
	assert.Equal(t, "pan out", payload["prompt"])
	assert.Equal(t, "https://example.com/frame.png", payload["image_url"])
	assert.Equal(t, float64(15), payload["duration"])
	assert.Equal(t, "720p", payload["resolution"])
	assert.Equal(t, "priority", payload["service_tier"])
	assert.Equal(t, "user-1", payload["user"])

	image, ok := payload["image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "data:image/png;base64,abc", image["url"])
	refs, ok := payload["reference_images"].([]any)
	require.True(t, ok)
	require.Len(t, refs, 2)
	assert.Equal(t, "file_123", refs[0].(map[string]any)["file_id"])
	video, ok := payload["video"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "file_video", video["file_id"])
	output, ok := payload["output"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://uploads.example.com/signed", output["upload_url"])
}

func TestBuildRequestBodyPreservesUpstreamModelName(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		upstream string
		want     string
	}{
		{
			name:     "canonical 1.5 model",
			origin:   "grok-imagine-video-1.5",
			upstream: "grok-imagine-video-1.5",
			want:     "grok-imagine-video-1.5",
		},
		{
			name:     "configured alias without mapping",
			origin:   "grok-imagine-video-1.5-720p",
			upstream: "grok-imagine-video-1.5-720p",
			want:     "grok-imagine-video-1.5-720p",
		},
		{
			name:     "channel model mapping",
			origin:   "client-video-alias",
			upstream: "cliproxy-video-model",
			want:     "cliproxy-video-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{
				"model":%q,
				"prompt":"pan out",
				"reference_images":[{"url":"https://example.com/ref.png"}],
				"duration":10,
				"aspect_ratio":"9:16",
				"resolution":"720p"
			}`, tt.origin)
			c, _ := buildTestContext(t, "/v1/videos/generations", body)
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.origin,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: tt.upstream,
					ChannelBaseUrl:    "https://api.x.ai",
					ApiKey:            "sk-test",
				},
			}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(c, info)
			require.Nil(t, taskErr)
			reader, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			assert.Equal(t, tt.want, payload["model"])
			assert.Equal(t, "9:16", payload["aspect_ratio"])
			assert.Equal(t, "720p", payload["resolution"])
		})
	}
}

func TestBuildRequestURLByAction(t *testing.T) {
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.x.ai/"}})
	tests := []struct {
		action string
		want   string
	}{
		{constant.TaskActionVideoGeneration, "https://api.x.ai/v1/videos/generations"},
		{constant.TaskActionVideoEdit, "https://api.x.ai/v1/videos/edits"},
		{constant.TaskActionVideoExtension, "https://api.x.ai/v1/videos/extensions"},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: tt.action}})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildRequestURLUsesDefaultBaseURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})

	got, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionVideoGeneration}})

	require.NoError(t, err)
	assert.Equal(t, "https://api.x.ai/v1/videos/generations", got)
}

func TestPrepareNormalizedVideoRequests(t *testing.T) {
	duration := 8
	aspectRatio := "9:16"
	resolution := "720p"
	tests := []struct {
		name       string
		request    dto.VideoTaskCreateRequest
		wantAction string
		wantURL    string
		assertBody func(*testing.T, map[string]any)
	}{
		{
			name: "generation with image and provider options",
			request: dto.VideoTaskCreateRequest{
				Model: "grok-imagine-video-1.5", Operation: "generation",
				Input:           dto.VideoTaskInputRequest{Prompt: "animate", Image: &dto.VideoTaskSource{URL: "https://example.com/frame.png"}},
				Output:          dto.VideoTaskOutputRequest{Duration: &duration, AspectRatio: &aspectRatio, Resolution: &resolution},
				ProviderOptions: map[string]map[string]any{"xai": {"user": "customer-1"}},
			},
			wantAction: constant.TaskActionVideoGeneration,
			wantURL:    "https://api.x.ai/v1/videos/generations",
			assertBody: func(t *testing.T, payload map[string]any) {
				assert.Equal(t, float64(8), payload["duration"])
				assert.Equal(t, "9:16", payload["aspect_ratio"])
				assert.Equal(t, "customer-1", payload["user"])
				assert.Equal(t, "https://example.com/frame.png", payload["image"].(map[string]any)["url"])
			},
		},
		{
			name: "generation with reference images",
			request: dto.VideoTaskCreateRequest{
				Model: "grok-imagine-video", Operation: "generation",
				Input: dto.VideoTaskInputRequest{Prompt: "keep the character", ReferenceImages: []dto.VideoTaskSource{
					{URL: "https://example.com/reference.png"},
					{Provider: "xai", FileID: "file_reference"},
				}},
				Output: dto.VideoTaskOutputRequest{Duration: common.GetPointer(10)},
			},
			wantAction: constant.TaskActionVideoGeneration,
			wantURL:    "https://api.x.ai/v1/videos/generations",
			assertBody: func(t *testing.T, payload map[string]any) {
				references, ok := payload["reference_images"].([]any)
				require.True(t, ok)
				require.Len(t, references, 2)
				assert.Equal(t, "https://example.com/reference.png", references[0].(map[string]any)["url"])
				assert.Equal(t, "file_reference", references[1].(map[string]any)["file_id"])
				assert.Equal(t, float64(10), payload["duration"])
			},
		},
		{
			name: "edit with xai file",
			request: dto.VideoTaskCreateRequest{
				Model: "grok-imagine-video-1.5", Operation: "edit",
				Input: dto.VideoTaskInputRequest{Prompt: "add rain", Video: &dto.VideoTaskSource{Provider: "xai", FileID: "file_video"}},
			},
			wantAction: constant.TaskActionVideoEdit,
			wantURL:    "https://api.x.ai/v1/videos/edits",
			assertBody: func(t *testing.T, payload map[string]any) {
				assert.Equal(t, "file_video", payload["video"].(map[string]any)["file_id"])
			},
		},
		{
			name: "extension duration",
			request: dto.VideoTaskCreateRequest{
				Model: "grok-imagine-video-1.5", Operation: "extension",
				Input:  dto.VideoTaskInputRequest{Prompt: "continue", Video: &dto.VideoTaskSource{URL: "https://example.com/source.mp4"}},
				Output: dto.VideoTaskOutputRequest{Duration: common.GetPointer(4)},
			},
			wantAction: constant.TaskActionVideoExtension,
			wantURL:    "https://api.x.ai/v1/videos/extensions",
			assertBody: func(t *testing.T, payload map[string]any) {
				assert.Equal(t, float64(4), payload["duration"])
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := buildTestContext(t, "/v1/video/tasks", `{}`)
			info := &relaycommon.RelayInfo{OriginModelName: test.request.Model, ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: "https://api.x.ai", UpstreamModelName: test.request.Model,
			}}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, test.request))
			assert.Equal(t, test.wantAction, info.Action)
			requestURL, err := adaptor.BuildRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, test.wantURL, requestURL)
			body, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			test.assertBody(t, payload)
		})
	}
}

func TestPrepareNormalizedVideoRejectsUnsupportedCapabilities(t *testing.T) {
	c, _ := buildTestContext(t, "/v1/video/tasks", `{}`)
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	remix := dto.VideoTaskCreateRequest{Model: "grok-imagine-video-1.5", Operation: "remix",
		Input: dto.VideoTaskInputRequest{Prompt: "remix", Video: &dto.VideoTaskSource{URL: "https://example.com/source.mp4"}}}
	taskErr := adaptor.PrepareNormalizedVideoRequest(c, info, remix)
	require.NotNil(t, taskErr)
	assert.Equal(t, "unsupported_video_operation", taskErr.Code)

	badNamespace := remix
	badNamespace.Operation = "edit"
	badNamespace.ProviderOptions = map[string]map[string]any{"other": {"seed": 1}}
	taskErr = adaptor.PrepareNormalizedVideoRequest(c, info, badNamespace)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_provider_options", taskErr.Code)

	tests := []struct {
		name    string
		request dto.VideoTaskCreateRequest
		code    string
	}{
		{
			name: "generation image and references",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video", Operation: "generation", Input: dto.VideoTaskInputRequest{
				Prompt: "generate", Image: &dto.VideoTaskSource{URL: "https://example.com/start.png"},
				ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}},
			}},
			code: "unsupported_video_input",
		},
		{
			name: "too many reference images",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video", Operation: "generation", Input: dto.VideoTaskInputRequest{
				Prompt: "generate", ReferenceImages: []dto.VideoTaskSource{
					{URL: "https://example.com/1.png"}, {URL: "https://example.com/2.png"},
					{URL: "https://example.com/3.png"}, {URL: "https://example.com/4.png"},
					{URL: "https://example.com/5.png"}, {URL: "https://example.com/6.png"},
					{URL: "https://example.com/7.png"}, {URL: "https://example.com/8.png"},
				},
			}},
			code: "unsupported_video_input",
		},
		{
			name: "reference duration over ten seconds",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video", Operation: "generation", Input: dto.VideoTaskInputRequest{
				Prompt: "generate", ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}},
			}, Output: dto.VideoTaskOutputRequest{Duration: common.GetPointer(11)}},
			code: "invalid_video_parameter",
		},
		{
			name: "edit with reference image",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video-1.5", Operation: "edit", Input: dto.VideoTaskInputRequest{
				Prompt: "edit", Video: &dto.VideoTaskSource{URL: "https://example.com/video.mp4"},
				ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/ref.png"}},
			}},
			code: "unsupported_video_input",
		},
		{
			name: "edit output override",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video-1.5", Operation: "edit",
				Input:  dto.VideoTaskInputRequest{Prompt: "edit", Video: &dto.VideoTaskSource{URL: "https://example.com/video.mp4"}},
				Output: dto.VideoTaskOutputRequest{Resolution: common.GetPointer("720p")}},
			code: "invalid_video_parameter",
		},
		{
			name: "extension with primary image",
			request: dto.VideoTaskCreateRequest{Model: "grok-imagine-video-1.5", Operation: "extension", Input: dto.VideoTaskInputRequest{
				Prompt: "extend", Video: &dto.VideoTaskSource{URL: "https://example.com/video.mp4"},
				Image: &dto.VideoTaskSource{URL: "https://example.com/start.png"},
			}},
			code: "unsupported_video_input",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := buildTestContext(t, "/v1/video/tasks", `{}`)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
			taskErr := adaptor.PrepareNormalizedVideoRequest(c, info, test.request)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestValidateNormalizedVideoModelRestricts1080p(t *testing.T) {
	adaptor := &TaskAdaptor{}
	tests := []struct {
		name     string
		payload  requestPayload
		model    string
		action   string
		wantCode string
	}{
		{
			name: "1.5 image generation",
			payload: requestPayload{
				"resolution": "1080p",
				"image":      map[string]any{"url": "https://example.com/frame.png"},
			},
			model:  "grok-imagine-video-1.5",
			action: constant.TaskActionVideoGeneration,
		},
		{
			name:     "legacy model",
			payload:  requestPayload{"resolution": "1080p", "image": map[string]any{"url": "https://example.com/frame.png"}},
			model:    "grok-imagine-video",
			action:   constant.TaskActionVideoGeneration,
			wantCode: "unsupported_video_resolution",
		},
		{
			name:     "text generation",
			payload:  requestPayload{"resolution": "1080p"},
			model:    "grok-imagine-video-1.5",
			action:   constant.TaskActionVideoGeneration,
			wantCode: "unsupported_video_resolution",
		},
		{
			name: "reference image generation",
			payload: requestPayload{
				"resolution":       "1080p",
				"reference_images": []any{map[string]any{"url": "https://example.com/ref.png"}},
			},
			model:    "grok-imagine-video-1.5",
			action:   constant.TaskActionVideoGeneration,
			wantCode: "unsupported_video_model_capability",
		},
		{
			name:     "non-generation action",
			payload:  requestPayload{"resolution": "1080p", "image": map[string]any{"url": "https://example.com/frame.png"}},
			model:    "grok-imagine-video-1.5",
			action:   constant.TaskActionVideoEdit,
			wantCode: "unsupported_video_resolution",
		},
		{
			name:    "lower resolution is unaffected",
			payload: requestPayload{"resolution": "720p"},
			model:   "grok-imagine-video",
			action:  constant.TaskActionVideoGeneration,
		},
		{
			name: "1.5 reference generation is unsupported at 720p",
			payload: requestPayload{
				"resolution":       "720p",
				"reference_images": []any{map[string]any{"url": "https://example.com/ref.png"}},
			},
			model:    "grok-imagine-video-1.5",
			action:   constant.TaskActionVideoGeneration,
			wantCode: "unsupported_video_model_capability",
		},
		{
			name: "base model reference generation is supported",
			payload: requestPayload{
				"resolution":       "720p",
				"reference_images": []any{map[string]any{"url": "https://example.com/ref.png"}},
			},
			model:  "grok-imagine-video",
			action: constant.TaskActionVideoGeneration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := buildTestContext(t, "/v1/video/tasks", `{}`)
			c.Set("xai_video_request", test.payload)
			info := &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: test.action},
				ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: test.model},
			}

			taskErr := adaptor.ValidateNormalizedVideoModel(c, info)
			if test.wantCode != "" {
				require.NotNil(t, taskErr)
				assert.Equal(t, test.wantCode, taskErr.Code)
				return
			}
			assert.Nil(t, taskErr)
		})
	}
}

func TestDoResponseHidesUpstreamRequestID(t *testing.T) {
	c, w := buildTestContext(t, "/v1/videos/generations", `{}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"upstream-123"}`)),
	}
	adaptor := &TaskAdaptor{}

	upstreamID, taskData, taskErr := adaptor.DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-123", upstreamID)
	assert.JSONEq(t, `{"request_id":"upstream-123"}`, string(taskData))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"request_id":"task_public","id":"task_public"}`, w.Body.String())
}

func TestDoResponseAcceptsIDFallback(t *testing.T) {
	c, _ := buildTestContext(t, "/v1/videos/generations", `{}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"upstream-id"}`)),
	}
	adaptor := &TaskAdaptor{}

	upstreamID, _, taskErr := adaptor.DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-id", upstreamID)
}

func TestParseTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}

	pending, err := adaptor.ParseTaskResult([]byte(`{"status":"pending","progress":25}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), pending.Status)
	assert.Equal(t, "25%", pending.Progress)

	done, err := adaptor.ParseTaskResult([]byte(`{"status":"done","progress":100,"video":{"url":"https://vidgen.x.ai/video.mp4","duration":15,"respect_moderation":true},"usage":{"cost_in_usd_ticks":12100000000}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), done.Status)
	assert.Equal(t, "100%", done.Progress)
	assert.Equal(t, "https://vidgen.x.ai/video.mp4", done.Url)

	completed, err := adaptor.ParseTaskResult([]byte(`{"status":"completed","video":{"url":"https://vidgen.x.ai/video2.mp4"}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), completed.Status)
	assert.Equal(t, "https://vidgen.x.ai/video2.mp4", completed.Url)

	moderated, err := adaptor.ParseTaskResult([]byte(`{"status":"done","video":{"url":"","respect_moderation":false}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), moderated.Status)
	assert.Equal(t, "video rejected by moderation", moderated.Reason)

	failed, err := adaptor.ParseTaskResult([]byte(`{"status":"failed","error":{"code":"invalid_argument","message":"bad input"}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), failed.Status)
	assert.Equal(t, "bad input", failed.Reason)

	moderationError, err := adaptor.ParseTaskResult([]byte(`{"code":"Client specified an invalid argument","error":"Generated video rejected by content moderation.","usage":{"cost_in_usd_ticks":12100000000}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), moderationError.Status)
	assert.Equal(t, "Generated video rejected by content moderation.", moderationError.Reason)

	expired, err := adaptor.ParseTaskResult([]byte(`{"status":"expired"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), expired.Status)
	assert.Equal(t, "task expired", expired.Reason)
}

func TestConvertToOpenAIVideoUsesLocalTaskIDAndResultURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video-1.5-preview-15s-480p",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-123",
			ResultURL:      "https://vidgen.x.ai/video.mp4",
		},
	}

	data, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "task_public", payload["id"])
	assert.Equal(t, "completed", payload["status"])
	assert.Equal(t, "grok-imagine-video-1.5-preview-15s-480p", payload["model"])
	metadata := payload["metadata"].(map[string]any)
	assert.Equal(t, taskcommon.BuildProxyURL("task_public"), metadata["url"])
}
