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
			Duration:      &duration,
			AspectRatio:   &aspectRatio,
			GenerateAudio: common.GetPointer(false),
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
	require.NotNil(t, payload.GenerateAudio)
	assert.True(t, *payload.GenerateAudio)
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
	assert.Equal(t, true, payload["generate_audio"])
}

func TestPrepareNormalizedVideoRequestMapsPublicGenerateAudio(t *testing.T) {
	tests := []struct {
		name     string
		value    *bool
		expected bool
	}{
		{name: "omitted defaults true", expected: true},
		{name: "explicit true", value: common.GetPointer(true), expected: true},
		{name: "explicit false", value: common.GetPointer(false), expected: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			duration := 4
			request := dto.VideoTaskCreateRequest{
				Model: "seedance-2.0-fast-480p", Operation: "generation",
				Input:  dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &duration, GenerateAudio: test.value},
			}
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance_2.0_fast_480p"}}
			c := adobeVideoTestContext()
			require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, request))
			body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			assert.Equal(t, test.expected, payload["generate_audio"])
		})
	}
}

func TestPrepareNormalizedVideoRequestEnforcesPromptLimit(t *testing.T) {
	duration := 4
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "seedance_2.0_480p"},
	}
	request := dto.VideoTaskCreateRequest{
		Model:     "adobe-seedance-2.0-480p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: strings.Repeat("界", maxVideoPromptRunes)},
		Output:    dto.VideoTaskOutputRequest{Duration: &duration},
	}

	require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(adobeVideoTestContext(), info, request))

	request.Input.Prompt += "界"
	taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(adobeVideoTestContext(), info, request)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_video_parameter", taskErr.Code)
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

func TestValidateNormalizedVideoModelDelegatesProviderCapabilitiesBeforeBilling(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		duration    int
		aspectRatio string
		mode        string
		images      int
		videos      int
		audios      int
	}{
		{name: "kling 3 three second frame", model: "kling_3.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame", images: 2},
		{name: "kling 3 text to video", model: "kling_3.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame"},
		{name: "kling 3 duration delegated", model: "kling_3.0_1080p", duration: 2, aspectRatio: "9:16", mode: "frame"},
		{name: "kling 3 images delegated", model: "kling_3.0_1080p", duration: 3, aspectRatio: "9:16", mode: "images"},
		{name: "kling omni images", model: "kling_3.0_omni_720p", duration: 15, aspectRatio: "9:16", mode: "images", images: 3},
		{name: "kling omni duration delegated", model: "kling_3.0_omni_1080p", duration: 16, aspectRatio: "16:9", mode: "frame"},
		{name: "kling omni image limit delegated", model: "kling_3.0_omni_1080p", duration: 4, aspectRatio: "16:9", mode: "images", images: 4},
		{name: "veo standard images", model: "veo_3.1_standard_720p", duration: 8, aspectRatio: "16:9", mode: "images", images: 3},
		{name: "veo standard images constraint delegated", model: "veo_3.1_standard_1080p", duration: 6, aspectRatio: "16:9", mode: "images"},
		{name: "veo standard duration delegated", model: "veo_3.1_standard_1080p", duration: 5, aspectRatio: "16:9", mode: "frame"},
		{name: "veo fast frame", model: "veo_3.1_fast_720p", duration: 4, aspectRatio: "9:16", mode: "frame", images: 2},
		{name: "veo fast images delegated", model: "veo_3.1_fast_1080p", duration: 8, aspectRatio: "16:9", mode: "images"},
		{name: "stable ratio delegated", model: "kling_3.0_720p", duration: 4, aspectRatio: "1:1", mode: "frame"},
		{name: "stable media delegated", model: "kling_3.0_720p", duration: 4, aspectRatio: "16:9", mode: "frame", images: 1, videos: 1},
		{name: "seedance mixed media", model: "seedance_2.0_fast_480p", duration: 4, aspectRatio: "1:1", mode: "media", images: 6, videos: 3, audios: 3},
		{name: "seedance duration delegated", model: "seedance_2.0_720p", duration: 3, aspectRatio: "16:9", mode: "frame"},
		{name: "seedance frame media delegated", model: "seedance_2.0_1080p", duration: 4, aspectRatio: "16:9", mode: "frame", audios: 1},
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
			require.Nil(t, taskErr)
			estimate, billingErr := (&TaskAdaptor{}).ResolveVideoBilling(c, info)
			require.Nil(t, billingErr)
			assert.Equal(t, test.duration, estimate.Seconds)
		})
	}
}

func TestValidateNormalizedVideoModelRequiresNonEmptyProviderSKU(t *testing.T) {
	adaptor := &TaskAdaptor{}
	validContext := adobeVideoTestContext()
	validContext.Set(videoRequestContextKey, &requestPayload{
		Duration: 4, AspectRatio: "16:9", ReferenceMode: "frame",
	})
	for _, modelName := range []string{"seedance_2.0_fast_480p", "future_vendor_sku_2160p"} {
		require.Nil(t, adaptor.ValidateNormalizedVideoModel(
			validContext,
			&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}},
		))
	}

	taskErr := adaptor.ValidateNormalizedVideoModel(
		validContext,
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}},
	)
	require.NotNil(t, taskErr)
	assert.Equal(t, "unsupported_video_model", taskErr.Code)
}

func TestAdobeArbitraryMappedSKUDelegatesCapabilitiesUpstream(t *testing.T) {
	duration := 37
	aspectRatio := "2:1"
	request := dto.VideoTaskCreateRequest{
		Model:     "public-future-video",
		Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt:          "future provider contract",
			ReferenceMode:   "storyboard",
			ReferenceImages: []dto.VideoTaskSource{{URL: "https://example.com/image.png"}},
			ReferenceVideos: []dto.VideoTaskSource{{URL: "https://example.com/video.mp4"}},
			ReferenceAudios: []dto.VideoTaskSource{{URL: "https://example.com/audio.wav"}},
		},
		Output: dto.VideoTaskOutputRequest{
			Duration:      &duration,
			AspectRatio:   &aspectRatio,
			GenerateAudio: common.GetPointer(false),
		},
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "future_vendor_sku_2160p"},
	}
	c := adobeVideoTestContext()
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(c, info))
	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, duration, estimate.Seconds)
	assert.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "future_vendor_sku_2160p", payload["model"])
	assert.EqualValues(t, duration, payload["duration"])
	assert.Equal(t, aspectRatio, payload["aspect_ratio"])
	assert.Equal(t, false, payload["generate_audio"])
	assert.Equal(t, "storyboard", payload["reference_mode"])
	assert.Len(t, payload["reference_images"], 1)
	assert.Len(t, payload["reference_videos"], 1)
	assert.Len(t, payload["reference_audios"], 1)
}

func TestAdobeReferenceCountBoundariesRemainProviderNeutral(t *testing.T) {
	makeSources := func(count int, prefix string) []dto.VideoTaskSource {
		result := make([]dto.VideoTaskSource, count)
		for index := range result {
			result[index].URL = fmt.Sprintf("https://example.com/%s-%d", prefix, index)
		}
		return result
	}
	duration := 4
	base := func() dto.VideoTaskCreateRequest {
		return dto.VideoTaskCreateRequest{
			Model: "public-model", Operation: "generation",
			Input:  dto.VideoTaskInputRequest{Prompt: "reference", ReferenceMode: "future-mode"},
			Output: dto.VideoTaskOutputRequest{Duration: &duration},
		}
	}
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	valid := base()
	valid.Input.ReferenceImages = makeSources(dto.VideoTaskMaxReferenceImages, "image")
	valid.Input.ReferenceVideos = makeSources(dto.VideoTaskMaxReferenceVideos, "video")
	valid.Input.ReferenceAudios = makeSources(dto.VideoTaskMaxReferenceAudios, "audio")
	require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(adobeVideoTestContext(), info, valid))

	for _, test := range []struct {
		name   string
		mutate func(*dto.VideoTaskCreateRequest)
		code   string
	}{
		{name: "images", mutate: func(request *dto.VideoTaskCreateRequest) {
			request.Input.ReferenceImages = makeSources(dto.VideoTaskMaxReferenceImages+1, "image")
		}, code: "reference_image_limit_exceeded"},
		{name: "videos", mutate: func(request *dto.VideoTaskCreateRequest) {
			request.Input.ReferenceVideos = makeSources(dto.VideoTaskMaxReferenceVideos+1, "video")
		}, code: "reference_video_limit_exceeded"},
		{name: "audios", mutate: func(request *dto.VideoTaskCreateRequest) {
			request.Input.ReferenceAudios = makeSources(dto.VideoTaskMaxReferenceAudios+1, "audio")
		}, code: "reference_audio_limit_exceeded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := base()
			test.mutate(&request)
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(adobeVideoTestContext(), info, request)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
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

	submitting, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"submitting","progress":0,"stage":"submitting"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSubmitted, submitting.Status)
	assert.True(t, submitting.ProgressMetadataSet)
	assert.Equal(t, "submitting", submitting.Stage)

	inProgress, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-1","status":"in_progress","progress":47,"progress_known":true,"progress_source":"upstream_percent","stage":"generating"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, inProgress.Status)
	assert.Equal(t, "47%", inProgress.Progress)
	assert.True(t, inProgress.ProgressMetadataSet)
	assert.True(t, inProgress.ProgressKnown)
	assert.Equal(t, "upstream_percent", inProgress.ProgressSource)
	assert.Equal(t, "generating", inProgress.Stage)

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

	failedWithoutMessage, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-2","status":"failed","error":{"code":"submission_unknown"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, failedWithoutMessage.Status)
	assert.Equal(t, "Video task failed", failedWithoutMessage.Reason)
	assert.NotContains(t, failedWithoutMessage.Reason, "Adobe")

	submissionUnknown, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-task-3","status":"submission_unknown"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, submissionUnknown.Status)
	assert.Equal(t, "Submission result could not be confirmed", submissionUnknown.Reason)
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
