package doubao

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func doubaoTestContext(path, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestResolveVideoBillingNormalizesDoubaoDuration(t *testing.T) {
	c := doubaoTestContext("/v1/video/generations", `{"model":"seedance-model","prompt":"cat","duration":7}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, 7, estimate.Seconds)
	require.Equal(t, types.VideoPricingBasisGeneration, estimate.Basis)

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Equal(t, float64(7), payload["duration"])
}

func TestResolveVideoBillingRejectsMissingDoubaoDuration(t *testing.T) {
	c := doubaoTestContext("/v1/video/generations", `{"model":"seedance-model","prompt":"cat"}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	_, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.NotNil(t, taskErr)
	require.Equal(t, "video_duration_required", taskErr.Code)
}

func TestPrepareNormalizedSeedanceRequestSharesBillingAndUpstreamDuration(t *testing.T) {
	duration := 9
	resolution := "720p"
	request := dto.VideoTaskCreateRequest{
		Model:     "seedance-model-720p",
		Operation: "generation",
		Input: dto.VideoTaskInputRequest{
			Prompt: "animate",
			Image:  &dto.VideoTaskSource{URL: "https://example.com/start.png"},
			ReferenceImages: []dto.VideoTaskSource{
				{URL: "https://example.com/reference.png"},
			},
		},
		Output: dto.VideoTaskOutputRequest{Duration: &duration, Resolution: &resolution, GenerateAudio: common.GetPointer(false)},
		ProviderOptions: map[string]map[string]any{
			"doubao": {"watermark": true},
		},
	}
	c := doubaoTestContext("/v1/video/tasks", `{}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "upstream-seedance"}}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	require.Equal(t, constant.TaskActionVideoGeneration, info.Action)

	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, duration, estimate.Seconds)

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Equal(t, float64(duration), payload["duration"])
	require.Equal(t, "720p", payload["resolution"])
	require.Equal(t, true, payload["watermark"])
	require.Equal(t, false, payload["generate_audio"])
	content := payload["content"].([]interface{})
	require.Len(t, content, 3)
	require.Equal(t, "first_frame", content[1].(map[string]interface{})["role"])
	require.Equal(t, "reference_image", content[2].(map[string]interface{})["role"])
}

func TestPrepareNormalizedSeedanceMapsGenerateAudio(t *testing.T) {
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
				Model: "seedance", Operation: "generation",
				Input:  dto.VideoTaskInputRequest{Prompt: "cat"},
				Output: dto.VideoTaskOutputRequest{Duration: &duration, GenerateAudio: test.value},
			}
			c := doubaoTestContext("/v1/video/tasks", `{}`)
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{}}
			require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, request))
			body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(data, &payload))
			require.Equal(t, test.expected, payload["generate_audio"])
		})
	}
}

func TestPrepareNormalizedSeedanceRejectsUnsupportedOperationsAndOverrides(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	c := doubaoTestContext("/v1/video/tasks", `{}`)
	request := dto.VideoTaskCreateRequest{Model: "seedance", Operation: "edit", Input: dto.VideoTaskInputRequest{Prompt: "edit", Video: &dto.VideoTaskSource{URL: "https://example.com/video.mp4"}}}
	taskErr := adaptor.PrepareNormalizedVideoRequest(c, info, request)
	require.NotNil(t, taskErr)
	require.Equal(t, "unsupported_video_operation", taskErr.Code)

	request = dto.VideoTaskCreateRequest{Model: "seedance", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, ProviderOptions: map[string]map[string]any{"doubao": {"duration": 5}}}
	taskErr = adaptor.PrepareNormalizedVideoRequest(c, info, request)
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_provider_options", taskErr.Code)

	for _, key := range []string{"generate_audio", "reference_mode"} {
		request = dto.VideoTaskCreateRequest{Model: "seedance", Operation: "generation", Input: dto.VideoTaskInputRequest{Prompt: "cat"}, ProviderOptions: map[string]map[string]any{"doubao": {key: false}}}
		taskErr = adaptor.PrepareNormalizedVideoRequest(c, info, request)
		require.NotNil(t, taskErr)
		require.Equal(t, "invalid_provider_options", taskErr.Code)
	}
}
