package adobevideo

import (
	"context"
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
	assert.Equal(t, "frame", payload["reference_mode"])
	assert.NotContains(t, payload, "reference_images")
}

func TestPrepareNormalizedVideoRequestMapsReferenceImagesInPublicOrder(t *testing.T) {
	duration := 4
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-fast-480p",
		Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt:        "keep the subject",
			ReferenceMode: "media",
			Image:         &dto.VideoTaskSource{URL: "https://example.com/subject.png", Name: "subject"},
			ReferenceImages: []dto.VideoTaskSource{
				{URL: "https://example.com/style.webp", Name: "style"},
				{URL: "https://example.com/background.png"},
			},
			ReferenceVideos: []dto.VideoTaskSource{{URL: "https://example.com/motion.mp4", Name: "motion"}},
			ReferenceAudios: []dto.VideoTaskSource{{URL: "https://example.com/music.m4a", Name: "music"}},
		},
		Output: dto.VideoTaskOutputRequest{Duration: &duration},
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
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "media", payload.ReferenceMode)
	require.Len(t, payload.ReferenceImages, 3)
	assert.Equal(t, "https://example.com/subject.png", payload.ReferenceImages[0].URL)
	assert.Equal(t, "subject", payload.ReferenceImages[0].Name)
	assert.Equal(t, "https://example.com/style.webp", payload.ReferenceImages[1].URL)
	assert.Equal(t, "https://example.com/background.png", payload.ReferenceImages[2].URL)
	require.Len(t, payload.ReferenceVideos, 1)
	assert.Equal(t, "motion", payload.ReferenceVideos[0].Name)
	require.Len(t, payload.ReferenceAudios, 1)
	assert.Equal(t, "music", payload.ReferenceAudios[0].Name)

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 4, estimate.Seconds)
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
			name:    "resolution must come from exact model",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration, Resolution: &resolution}},
			code:    "invalid_video_parameter",
		},
		{
			name:    "video input unsupported",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat", Video: &dto.VideoTaskSource{URL: "https://example.com/cat.mp4"}}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration}},
			code:    "unsupported_video_input",
		},
		{
			name:    "file reference unsupported",
			request: dto.VideoTaskCreateRequest{Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat", Image: &dto.VideoTaskSource{Provider: "adobe_video", FileID: "file-1"}}, Output: dto.VideoTaskOutputRequest{Duration: &validDuration}},
			code:    "unsupported_file_provider",
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
			name: "public and provider reference mode conflict",
			request: dto.VideoTaskCreateRequest{
				Model: "alias", Operation: "generation",
				Input:  dto.VideoTaskInputRequest{Prompt: "cat", ReferenceMode: "media"},
				Output: dto.VideoTaskOutputRequest{Duration: &validDuration},
				ProviderOptions: map[string]map[string]any{
					ProviderOptionsNamespace: {"reference_mode": "media"},
				},
			},
			code: "invalid_provider_options",
		},
		{
			name: "invalid reference mode",
			request: dto.VideoTaskCreateRequest{
				Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &validDuration},
				ProviderOptions: map[string]map[string]any{
					ProviderOptionsNamespace: {"reference_mode": "style"},
				},
			},
			code: "invalid_provider_options",
		},
		{
			name: "reference override rejected",
			request: dto.VideoTaskCreateRequest{
				Model: "alias", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &validDuration},
				ProviderOptions: map[string]map[string]any{
					ProviderOptionsNamespace: {"reference_images": []any{}},
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

func TestValidateNormalizedVideoModelAppliesProviderCapabilityBeforeBilling(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		duration    int
		aspectRatio string
		mode        string
		images      int
		videos      int
		audios      int
		wantCode    string
	}{
		{name: "kling 3 three second frame", model: "kling_3.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame", images: 2},
		{name: "kling 3 frame requires an image", model: "kling_3.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame", wantCode: "reference_image_required"},
		{name: "kling 3 below minimum", model: "kling_3.0_1080p", duration: 2, aspectRatio: "9:16", mode: "frame", wantCode: "invalid_video_duration"},
		{name: "kling 3 images rejected", model: "kling_3.0_1080p", duration: 3, aspectRatio: "9:16", mode: "images", wantCode: "unsupported_reference_mode"},
		{name: "kling omni images", model: "kling_3.0_omni_720p", duration: 15, aspectRatio: "9:16", mode: "images", images: 3},
		{name: "kling omni duration above maximum", model: "kling_3.0_omni_1080p", duration: 16, aspectRatio: "16:9", mode: "frame", wantCode: "invalid_video_duration"},
		{name: "kling omni image limit", model: "kling_3.0_omni_1080p", duration: 4, aspectRatio: "16:9", mode: "images", images: 4, wantCode: "reference_image_limit_exceeded"},
		{name: "veo standard images", model: "veo_3.1_standard_720p", duration: 8, aspectRatio: "16:9", mode: "images", images: 3},
		{name: "veo standard images duration constraint", model: "veo_3.1_standard_1080p", duration: 6, aspectRatio: "16:9", mode: "images", wantCode: "invalid_video_parameter"},
		{name: "veo standard duration set", model: "veo_3.1_standard_1080p", duration: 5, aspectRatio: "16:9", mode: "frame", wantCode: "invalid_video_duration"},
		{name: "veo fast frame", model: "veo_3.1_fast_720p", duration: 4, aspectRatio: "9:16", mode: "frame", images: 2},
		{name: "veo fast images rejected", model: "veo_3.1_fast_1080p", duration: 8, aspectRatio: "16:9", mode: "images", wantCode: "unsupported_reference_mode"},
		{name: "stable ratio rejected", model: "kling_3.0_720p", duration: 4, aspectRatio: "1:1", mode: "frame", wantCode: "invalid_video_aspect_ratio"},
		{name: "stable media rejected", model: "kling_3.0_720p", duration: 4, aspectRatio: "16:9", mode: "frame", images: 1, videos: 1, wantCode: "reference_video_limit_exceeded"},
		{name: "seedance mixed media", model: "seedance_2.0_fast_480p", duration: 4, aspectRatio: "1:1", mode: "media", images: 6, videos: 3, audios: 3},
		{name: "seedance below minimum", model: "seedance_2.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame", wantCode: "invalid_video_duration"},
		{name: "seedance frame media rejected", model: "seedance_2.0_1080p", duration: 4, aspectRatio: "16:9", mode: "frame", audios: 1, wantCode: "unsupported_reference_mode"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			imageSources := make([]dto.VideoTaskSource, test.images)
			for i := range imageSources {
				imageSources[i].URL = fmt.Sprintf("https://example.com/image-%d.png", i)
			}
			videoSources := make([]dto.VideoTaskSource, test.videos)
			for i := range videoSources {
				videoSources[i].URL = fmt.Sprintf("https://example.com/video-%d.mp4", i)
			}
			audioSources := make([]dto.VideoTaskSource, test.audios)
			for i := range audioSources {
				audioSources[i].URL = fmt.Sprintf("https://example.com/audio-%d.m4a", i)
			}
			request := dto.VideoTaskCreateRequest{
				Model: "public-model", Operation: "generation",
				Input: dto.VideoTaskInputRequest{
					Prompt: "test", ReferenceMode: test.mode,
					ReferenceImages: imageSources, ReferenceVideos: videoSources, ReferenceAudios: audioSources,
				},
				Output: dto.VideoTaskOutputRequest{Duration: &test.duration, AspectRatio: &test.aspectRatio},
			}
			info := &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
				ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: test.model},
			}
			c := adobeVideoTestContext()
			require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, request))
			taskErr := (&TaskAdaptor{}).ValidateNormalizedVideoModel(c, info)
			if test.wantCode != "" {
				require.NotNil(t, taskErr)
				assert.Equal(t, test.wantCode, taskErr.Code)
				return
			}
			require.Nil(t, taskErr)
			estimate, billingErr := (&TaskAdaptor{}).ResolveVideoBilling(c, info)
			require.Nil(t, billingErr)
			assert.Equal(t, test.duration, estimate.Seconds)
		})
	}
}

func TestValidateNormalizedVideoModelRequiresExactProviderSKU(t *testing.T) {
	adaptor := &TaskAdaptor{}
	validContext := adobeVideoTestContext()
	validContext.Set(videoRequestContextKey, &requestPayload{
		Duration: 4, AspectRatio: "16:9", ReferenceMode: "frame",
	})
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(
		validContext,
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

	directURL := "https://pre-signed-firefly-prod.s3.amazonaws.com/generated/provider-task-1.mp4?X-Amz-Signature=secret&X-Amz-Expires=3600"
	completed, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"completed","progress":100,"duration":4,"video_url":"` + directURL + `"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, completed.Status)
	require.Len(t, completed.VideoOutputs, 1)
	assert.Equal(t, directURL, completed.VideoOutputs[0].URL)
	assert.Empty(t, completed.VideoOutputs[0].ProviderReference)
	assert.Empty(t, completed.VideoOutputs[0].Resolver)
	assert.EqualValues(t, 4000, completed.VideoOutputs[0].DurationMS)

	legacy, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-legacy","status":"completed","progress":100,"duration":4}`))
	require.NoError(t, err)
	require.Len(t, legacy.VideoOutputs, 1)
	assert.Empty(t, legacy.VideoOutputs[0].URL)
	assert.Equal(t, "provider-task-legacy", legacy.VideoOutputs[0].ProviderReference)
	assert.Equal(t, videoContentResolver, legacy.VideoOutputs[0].Resolver)

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
