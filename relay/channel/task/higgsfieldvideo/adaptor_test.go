package higgsfieldvideo

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/task/adobevideo"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareNormalizedVideoRequestUsesHiggsfieldOptionsAndMediaContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &TaskAdaptor{}
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-480p",
		Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt:        "a camera move from @start to @motion",
			ReferenceMode: "media",
			ReferenceImages: []dto.VideoTaskSource{
				{URL: "https://assets.example/start.png", Name: "start"},
			},
			ReferenceVideos: []dto.VideoTaskSource{
				{URL: "https://assets.example/motion.mp4", Name: "motion"},
			},
			ReferenceAudios: []dto.VideoTaskSource{
				{URL: "https://assets.example/music.mp3", Name: "music"},
			},
		},
		Output: dto.VideoTaskOutputRequest{Duration: intPointer(4), GenerateAudio: common.GetPointer(false)},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-480p"},
	}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(context, info, request))
	body, err := adaptor.BuildRequestBody(context, info)
	require.NoError(t, err)
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(raw, &payload))

	assert.Equal(t, "seedance-2.0-480p", payload["model"])
	assert.Equal(t, "media", payload["reference_mode"])
	assert.Equal(t, false, payload["generate_audio"])
	assert.Len(t, payload["reference_images"], 1)
	assert.Len(t, payload["reference_videos"], 1)
	assert.Len(t, payload["reference_audios"], 1)
}

func TestHiggsfieldGenerateAudioMapping(t *testing.T) {
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
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			request := dto.VideoTaskCreateRequest{
				Model: "seedance-2.0-480p", Operation: "generation",
				Input:  dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: intPointer(4), GenerateAudio: test.value},
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-480p"}}
			require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(context, info, request))
			body, err := (&TaskAdaptor{}).BuildRequestBody(context, info)
			require.NoError(t, err)
			raw, err := io.ReadAll(body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(raw, &payload))
			assert.Equal(t, test.expected, payload["generate_audio"])
		})
	}
}

func TestBuildRequestURLUsesHiggsfieldVideoLifecycle(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://higgsfield.example",
			ApiKey:         "test-key",
		},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://higgsfield.example/v1/videos", requestURL)
}

func TestParseTaskResultNormalizesFractionalProgress(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		progress      string
		progressKnown bool
	}{
		{
			name:          "fractional upstream progress",
			body:          `{"task_id":"provider-1","status":"in_progress","progress":0.479}`,
			progress:      "47%",
			progressKnown: true,
		},
		{
			name:          "integer upstream progress",
			body:          `{"task_id":"provider-1","status":"in_progress","progress":47}`,
			progress:      "47%",
			progressKnown: true,
		},
		{
			name:          "explicit unknown progress remains unknown",
			body:          `{"task_id":"provider-1","status":"in_progress","progress":0.47,"progress_known":false,"progress_source":"upstream_status"}`,
			progress:      "47%",
			progressKnown: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(test.body))
			require.NoError(t, err)
			assert.Equal(t, test.progress, result.Progress)
			assert.True(t, result.ProgressMetadataSet)
			assert.Equal(t, test.progressKnown, result.ProgressKnown)
			if test.progressKnown {
				assert.Equal(t, "upstream_percent", result.ProgressSource)
			}
		})
	}
}

func TestPrepareNormalizedVideoRequestRejectsAdobeProviderOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	duration := 4
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-480p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "test"},
		Output:    dto.VideoTaskOutputRequest{Duration: &duration},
		ProviderOptions: map[string]map[string]any{
			adobevideo.ProviderOptionsNamespace: {"generate_audio": false},
		},
	}

	taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
		context,
		&relaycommon.RelayInfo{
			TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-480p"},
		},
		request,
	)

	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_provider_options", taskErr.Code)
}

func TestPrepareNormalizedVideoRequestRejectsLegacyHiggsfieldOptions(t *testing.T) {
	for _, key := range []string{"generate_audio", "reference_mode"} {
		t.Run(key, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			duration := 4
			request := dto.VideoTaskCreateRequest{
				Model: "seedance-2.0-480p", Operation: "generation",
				Input:           dto.VideoTaskInputRequest{Prompt: "test"},
				Output:          dto.VideoTaskOutputRequest{Duration: &duration},
				ProviderOptions: map[string]map[string]any{ProviderOptionsNamespace: {key: false}},
			}
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(context, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, request)
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_provider_options", taskErr.Code)
		})
	}
}

func TestSeedanceCapabilityAndBillingUseHiggsfieldProviderSKU(t *testing.T) {
	gin.SetMode(gin.TestMode)
	duration := 4
	aspectRatio := "1:1"
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-480p",
		Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt:        "mixed media",
			ReferenceMode: "media",
			ReferenceVideos: []dto.VideoTaskSource{
				{URL: "https://assets.example/motion.mp4"},
			},
		},
		Output: dto.VideoTaskOutputRequest{
			Duration:    &duration,
			AspectRatio: &aspectRatio,
		},
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-480p"},
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(context, info, request))
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(context, info))
	estimate, taskErr := adaptor.ResolveVideoBilling(context, info)
	require.Nil(t, taskErr)
	assert.Equal(t, 4, estimate.Seconds)
	assert.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)
}

func TestSeedanceCapabilityRejectsInvalidDurationBeforeBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	duration := 3
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-2.0-720p",
		Operation: "generation",
		Input:     dto.VideoTaskInputRequest{Prompt: "too short"},
		Output:    dto.VideoTaskOutputRequest{Duration: &duration},
	}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2.0-720p"},
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptor := &TaskAdaptor{}

	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(context, info, request))
	taskErr := adaptor.ValidateNormalizedVideoModel(context, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_video_duration", taskErr.Code)
}

func intPointer(value int) *int {
	return &value
}
