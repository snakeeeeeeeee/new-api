package higgsfieldvideo

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/task/adobevideo"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
		Output: dto.VideoTaskOutputRequest{Duration: intPointer(4)},
		ProviderOptions: map[string]map[string]any{
			ProviderOptionsNamespace: {"generate_audio": false},
		},
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

func intPointer(value int) *int {
	return &value
}
