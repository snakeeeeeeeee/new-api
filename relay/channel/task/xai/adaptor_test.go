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
	"github.com/QuantumNous/new-api/model"
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

func TestBuildRequestBodyNormalizesXAIVideoAliasWithoutModelMapping(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "stable short alias",
			in:   "grok-imagine-video-15s-720p",
			want: "grok-imagine-video",
		},
		{
			name: "preview quality alias",
			in:   "grok-imagine-video-1.5-preview-15s-720p",
			want: "grok-imagine-video-1.5-preview",
		},
		{
			name: "preview dated alias",
			in:   "grok-imagine-video-1.5-2026-05-30-15s-480p",
			want: "grok-imagine-video-1.5-preview",
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
			}`, tt.in)
			c, _ := buildTestContext(t, "/v1/videos/generations", body)
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.in,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: tt.in,
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
	assert.Equal(t, "https://vidgen.x.ai/video.mp4", metadata["url"])
}
