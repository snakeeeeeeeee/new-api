package leonardovideo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func leonardoVideoTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/tasks", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func leonardoInfo(modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123"},
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: modelName, ChannelSetting: dto.ChannelSettings{}},
	}
}

func leonardoRequest(modelName string, duration int) dto.VideoTaskCreateRequest {
	return dto.VideoTaskCreateRequest{
		Model: modelName, Operation: "generation",
		Input:  dto.VideoTaskInputRequest{Prompt: "  cinematic cat  "},
		Output: dto.VideoTaskOutputRequest{Duration: &duration},
	}
}

func TestLeonardoModelListAndRegistrationContract(t *testing.T) {
	assert.Equal(t, "leonardo-video", (&TaskAdaptor{}).GetChannelName())
	assert.Equal(t, []string{
		"seedance-2.0-fast-480p", "seedance-2.0-fast-720p",
		"seedance-2.0-480p", "seedance-2.0-720p", "seedance-2.0-1080p",
		"seedance-2.5-480p", "seedance-2.5-720p",
		"minimax-h3-1440p",
	}, (&TaskAdaptor{}).GetModelList())
	assert.NotContains(t, ModelList, "seedance-2.0-2160p")
	assert.True(t, isSupportedProviderModel("seedance-3.0-ultra-1440p"))
	assert.False(t, isSupportedProviderModel("seedance-2.0-2160p"))
	assert.False(t, isSupportedProviderModel("seedance-2.5-1080p"))
	assert.False(t, isSupportedProviderModel("leonardo-seedance-3.0-720p"))
}

func TestLeonardoMiniMaxH3FramePayload(t *testing.T) {
	duration := 5
	ratio := "21:9"
	request := leonardoRequest("leonardo-minimax-h3-1440p", duration)
	request.Output.AspectRatio = &ratio
	request.Input.ReferenceMode = "frame"
	request.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/start.png", Name: "start"}
	request.Input.ReferenceImages = []dto.VideoTaskSource{{URL: "http://example.com/end.png", Name: "end"}}

	info := leonardoInfo("leonardo-minimax-h3-1440p")
	c := leonardoVideoTestContext()
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	info.UpstreamModelName = "minimax-h3-1440p"
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(c, info))
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "minimax-h3-1440p", payload["model"])
	assert.Equal(t, "frame", payload["reference_mode"])
	assert.Equal(t, false, payload["public"])
	assert.Equal(t, true, payload["generate_audio"])
	assert.NotContains(t, payload, "seed")
	assert.NotContains(t, payload, "resolution")
	assert.Len(t, payload["image_references"], 2)
	assert.NotContains(t, payload, "video_references")
}

func TestLeonardoMiniMaxH3ReferenceRules(t *testing.T) {
	makeImages := func(count int) []dto.VideoTaskSource {
		images := make([]dto.VideoTaskSource, count)
		for index := range images {
			images[index] = dto.VideoTaskSource{URL: fmt.Sprintf("https://example.com/%d.png", index)}
		}
		return images
	}
	tests := []struct {
		name   string
		mode   string
		images int
		videos int
		audios int
		code   string
	}{
		{name: "frame start", mode: "frame", images: 1},
		{name: "frame start and end", mode: "frame", images: 2},
		{name: "five image references", mode: "images", images: 5},
		{name: "media images and audio", mode: "media", images: 5, audios: 3},
		{name: "missing mode", images: 1, code: "unsupported_reference_mode"},
		{name: "empty frame mode", mode: "frame", code: "invalid_video_parameter"},
		{name: "too many frame images", mode: "frame", images: 3, code: "reference_image_limit_exceeded"},
		{name: "frame audio", mode: "frame", images: 1, audios: 1, code: "unsupported_reference_audio"},
		{name: "empty images mode", mode: "images", code: "invalid_video_parameter"},
		{name: "too many images", mode: "images", images: 6, code: "reference_image_limit_exceeded"},
		{name: "images audio", mode: "images", images: 1, audios: 1, code: "unsupported_reference_audio"},
		{name: "video reference", mode: "frame", images: 1, videos: 1, code: "unsupported_reference_video"},
		{name: "media without audio", mode: "media", images: 1, code: "invalid_video_parameter"},
		{name: "media without image", mode: "media", audios: 1, code: "invalid_video_parameter"},
		{name: "too many audios", mode: "media", images: 1, audios: 4, code: "reference_audio_limit_exceeded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := leonardoRequest("leonardo-minimax-h3-1440p", 5)
			request.Input.ReferenceMode = test.mode
			request.Input.ReferenceImages = makeImages(test.images)
			if test.videos > 0 {
				request.Input.ReferenceVideos = []dto.VideoTaskSource{{URL: "https://example.com/video.mp4"}}
			}
			if test.audios > 0 {
				request.Input.ReferenceAudios = makeImages(test.audios)
			}
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
				leonardoVideoTestContext(), leonardoInfo("minimax-h3-1440p"), request,
			)
			if test.code == "" {
				require.Nil(t, taskErr)
				return
			}
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestLeonardoMiniMaxH3DurationStartsAtFiveSeconds(t *testing.T) {
	for duration, valid := range map[int]bool{4: false, 5: true, 15: true, 16: false} {
		request := leonardoRequest("leonardo-minimax-h3-1440p", duration)
		taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
			leonardoVideoTestContext(), leonardoInfo("minimax-h3-1440p"), request,
		)
		if valid {
			require.Nil(t, taskErr)
		} else {
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_video_duration", taskErr.Code)
		}
	}
}

func TestLeonardoSeedance25DurationAndFramePayload(t *testing.T) {
	for duration, valid := range map[int]bool{3: false, 4: true, 15: true, 30: true, 31: false} {
		request := leonardoRequest("leonardo-seedance-2.5-720p", duration)
		info := leonardoInfo("seedance-2.5-720p")
		c := leonardoVideoTestContext()
		taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, request)
		if !valid {
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_video_duration", taskErr.Code)
			continue
		}
		require.Nil(t, taskErr)
		require.Nil(t, (&TaskAdaptor{}).ValidateNormalizedVideoModel(c, info))
	}

	duration := 30
	ratio := "9:16"
	request := leonardoRequest("leonardo-seedance-2.5-720p", duration)
	request.Output.AspectRatio = &ratio
	request.Input.ReferenceMode = "frame"
	request.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/start.png", Name: "start"}
	request.Input.ReferenceImages = []dto.VideoTaskSource{{URL: "https://example.com/end.png", Name: "end"}}
	info := leonardoInfo("seedance-2.5-720p")
	c := leonardoVideoTestContext()
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(c, info))
	estimate, taskErr := adaptor.ResolveVideoBilling(c, info)
	require.Nil(t, taskErr)
	assert.Equal(t, duration, estimate.Seconds)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "seedance-2.5-720p", payload["model"])
	assert.Equal(t, "frame", payload["reference_mode"])
	assert.EqualValues(t, -1, payload["seed"])
	assert.Len(t, payload["image_references"], 2)
}

func TestLeonardoSeedance25FrameReferenceRules(t *testing.T) {
	makeImages := func(count int) []dto.VideoTaskSource {
		images := make([]dto.VideoTaskSource, count)
		for index := range images {
			images[index] = dto.VideoTaskSource{URL: fmt.Sprintf("https://example.com/%d.png", index)}
		}
		return images
	}
	tests := []struct {
		name   string
		images int
		videos int
		audios int
		code   string
	}{
		{name: "start frame", images: 1},
		{name: "start and end frames", images: 2},
		{name: "missing frame", code: "invalid_video_parameter"},
		{name: "too many frames", images: 3, code: "reference_image_limit_exceeded"},
		{name: "video is unsupported", images: 1, videos: 1, code: "unsupported_reference_video"},
		{name: "audio is unsupported", images: 1, audios: 1, code: "unsupported_reference_audio"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := leonardoRequest("leonardo-seedance-2.5-480p", 4)
			request.Input.ReferenceMode = "frame"
			request.Input.ReferenceImages = makeImages(test.images)
			if test.videos > 0 {
				request.Input.ReferenceVideos = []dto.VideoTaskSource{{URL: "https://example.com/video.mp4"}}
			}
			if test.audios > 0 {
				request.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "https://example.com/audio.wav"}}
			}
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
				leonardoVideoTestContext(), leonardoInfo("seedance-2.5-480p"), request,
			)
			if test.code == "" {
				require.Nil(t, taskErr)
				return
			}
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestLeonardoFutureSeedanceMappingUsesDefaultContract(t *testing.T) {
	duration := 15
	request := leonardoRequest("leonardo-seedance-3.0-ultra-1440p", duration)
	request.Input.ReferenceMode = "media"
	request.Input.ReferenceImages = []dto.VideoTaskSource{{URL: "https://example.com/reference.png"}}
	info := leonardoInfo("seedance-3.0-ultra-1440p")
	c := leonardoVideoTestContext()
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
	require.Nil(t, adaptor.ValidateNormalizedVideoModel(c, info))
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "seedance-3.0-ultra-1440p", payload["model"])
	assert.Len(t, payload["image_references"], 1)

	request = leonardoRequest("leonardo-seedance-3.0-ultra-1440p", 16)
	taskErr := adaptor.PrepareNormalizedVideoRequest(leonardoVideoTestContext(), info, request)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_video_duration", taskErr.Code)
}

func TestPrepareAndBuildLeonardoRequestUsesOnlyUpstreamFields(t *testing.T) {
	duration := 4
	aspect := "9:16"
	request := leonardoRequest("leonardo-seedance-2.0-fast-480p", duration)
	request.Output.AspectRatio = &aspect
	request.Input.ReferenceMode = "media"
	request.Input.Image = &dto.VideoTaskSource{URL: "http://example.com/first.png", Name: "first"}
	request.Input.ReferenceImages = []dto.VideoTaskSource{{URL: "https://example.com/style.webp"}}
	request.Input.ReferenceVideos = []dto.VideoTaskSource{{URL: "https://example.com/motion.mp4"}}
	request.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "http://example.com/sound.wav", Name: "sound"}}
	request.Output.GenerateAudio = common.GetPointer(false)

	info := leonardoInfo("seedance-2.0-fast-480p")
	c := leonardoVideoTestContext()
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.PrepareNormalizedVideoRequest(c, info, request))
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
	assert.Equal(t, "seedance-2.0-fast-480p", payload["model"])
	assert.Equal(t, "cinematic cat", payload["prompt"])
	assert.EqualValues(t, 4, payload["duration"])
	assert.Equal(t, "9:16", payload["aspect_ratio"])
	assert.Equal(t, false, payload["generate_audio"])
	assert.Equal(t, false, payload["public"])
	assert.EqualValues(t, -1, payload["seed"])
	assert.NotContains(t, payload, "reference_mode")
	assert.NotContains(t, payload, "reference_images")
	assert.NotContains(t, payload, "reference_videos")
	assert.NotContains(t, payload, "reference_audios")
	assert.NotContains(t, payload, "resolution")
	images := payload["image_references"].([]any)
	assert.Equal(t, "http://example.com/first.png", images[0].(map[string]any)["url"])
	assert.Equal(t, "https://example.com/style.webp", images[1].(map[string]any)["url"])
	videos := payload["video_references"].([]any)
	assert.Equal(t, "https://example.com/motion.mp4", videos[0].(map[string]any)["url"])
	audios := payload["audio_references"].([]any)
	assert.Equal(t, "http://example.com/sound.wav", audios[0].(map[string]any)["url"])
	assert.Equal(t, "sound", audios[0].(map[string]any)["name"])
}

func TestLeonardoGenerateAudioMapping(t *testing.T) {
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
			request := leonardoRequest("leonardo-seedance-2.0-fast-480p", 4)
			request.Output.GenerateAudio = test.value
			info := leonardoInfo("seedance-2.0-fast-480p")
			c := leonardoVideoTestContext()
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

func TestLeonardoReferenceAndParameterLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*dto.VideoTaskCreateRequest)
		code   string
	}{
		{name: "duration below minimum", mutate: func(r *dto.VideoTaskCreateRequest) { d := 3; r.Output.Duration = &d }, code: "invalid_video_duration"},
		{name: "duration above maximum", mutate: func(r *dto.VideoTaskCreateRequest) { d := 16; r.Output.Duration = &d }, code: "invalid_video_duration"},
		{name: "resolution is model bound", mutate: func(r *dto.VideoTaskCreateRequest) { v := "480p"; r.Output.Resolution = &v }, code: "invalid_video_parameter"},
		{name: "edit is unsupported", mutate: func(r *dto.VideoTaskCreateRequest) { r.Operation = "edit" }, code: "unsupported_video_operation"},
		{name: "frame mode is unsupported", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "frame"
			r.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/a.png"}
		}, code: "unsupported_reference_mode"},
		{name: "missing mode with references", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/a.png"}
		}, code: "unsupported_reference_mode"},
		{name: "audio without visual reference", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "https://example.com/a.mp3"}}
		}, code: "invalid_video_parameter"},
		{name: "input video", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.Video = &dto.VideoTaskSource{URL: "https://example.com/a.mp4"}
		}, code: "unsupported_video_input"},
		{name: "legacy audio provider option", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.ProviderOptions = map[string]map[string]any{ProviderOptionsNamespace: {"generate_audio": false}}
		}, code: "invalid_provider_options"},
		{name: "legacy reference mode provider option", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.ProviderOptions = map[string]map[string]any{ProviderOptionsNamespace: {"reference_mode": "media"}}
		}, code: "invalid_provider_options"},
		{name: "data url", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{URL: "data:image/png;base64,AAAA"}
		}, code: "invalid_video_parameter"},
		{name: "ftp url", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{URL: "ftp://example.com/a.png"}
		}, code: "invalid_video_parameter"},
		{name: "port zero", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{URL: "http://example.com:0/a.png"}
		}, code: "invalid_video_parameter"},
		{name: "provider file", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{Provider: "files", FileID: "file_1"}
		}, code: "unsupported_file_provider"},
		{name: "audio data url", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/a.png"}
			r.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "data:audio/mpeg;base64,AAAA"}}
		}, code: "invalid_video_parameter"},
		{name: "audio provider file", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/a.png"}
			r.Input.ReferenceAudios = []dto.VideoTaskSource{{Provider: "files", FileID: "audio_1"}}
		}, code: "unsupported_file_provider"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			duration := 4
			request := leonardoRequest("leonardo-seedance-2.0-fast-480p", duration)
			test.mutate(&request)
			taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-fast-480p"), request)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestLeonardoReferenceCountBoundaries(t *testing.T) {
	makeSources := func(count int, prefix string) []dto.VideoTaskSource {
		result := make([]dto.VideoTaskSource, count)
		for i := range result {
			result[i] = dto.VideoTaskSource{URL: "https://example.com/" + prefix + "." + string(rune('a'+i))}
		}
		return result
	}
	duration := 4
	base := func() dto.VideoTaskCreateRequest {
		return dto.VideoTaskCreateRequest{
			Model: "leonardo-seedance-2.0-480p", Operation: "generation",
			Input:  dto.VideoTaskInputRequest{Prompt: "reference", ReferenceMode: "media"},
			Output: dto.VideoTaskOutputRequest{Duration: &duration},
		}
	}
	valid := base()
	valid.Input.ReferenceImages = makeSources(4, "image")
	valid.Input.ReferenceVideos = makeSources(3, "video")
	valid.Input.ReferenceAudios = makeSources(1, "audio")
	require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-480p"), valid))

	tooManyImages := base()
	tooManyImages.Input.ReferenceImages = makeSources(5, "image")
	err := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-480p"), tooManyImages)
	require.NotNil(t, err)
	assert.Equal(t, "reference_image_limit_exceeded", err.Code)

	tooManyVideos := base()
	tooManyVideos.Input.ReferenceVideos = makeSources(4, "video")
	err = (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-480p"), tooManyVideos)
	require.NotNil(t, err)
	assert.Equal(t, "reference_video_limit_exceeded", err.Code)

	tooManyAudios := base()
	tooManyAudios.Input.ReferenceImages = makeSources(1, "image")
	tooManyAudios.Input.ReferenceAudios = makeSources(2, "audio")
	err = (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-480p"), tooManyAudios)
	require.NotNil(t, err)
	assert.Equal(t, "reference_audio_limit_exceeded", err.Code)
}

func TestLeonardoMiniMaxH3RejectsDisabledNativeAudioBeforeBilling(t *testing.T) {
	request := leonardoRequest("leonardo-minimax-h3-1440p", 5)
	request.Output.GenerateAudio = common.GetPointer(false)
	taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
		leonardoVideoTestContext(), leonardoInfo("minimax-h3-1440p"), request,
	)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_video_parameter", taskErr.Code)
}

func TestLeonardoReferenceNamesAreUniqueAcrossMediaTypes(t *testing.T) {
	request := leonardoRequest("leonardo-seedance-2.0-fast-480p", 4)
	request.Input.ReferenceMode = "media"
	request.Input.Image = &dto.VideoTaskSource{URL: "https://example.com/image.png", Name: "subject"}
	request.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "https://example.com/audio.mp3", Name: "subject"}}
	taskErr := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(
		leonardoVideoTestContext(), leonardoInfo("seedance-2.0-fast-480p"), request,
	)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_video_parameter", taskErr.Code)
}

func TestLeonardoModelAndAspectValidation(t *testing.T) {
	duration := 4
	for _, ratio := range []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"} {
		ratio := ratio
		request := leonardoRequest("leonardo-seedance-2.0-fast-480p", duration)
		request.Output.AspectRatio = &ratio
		c := leonardoVideoTestContext()
		info := leonardoInfo("seedance-2.0-fast-480p")
		require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, request))
		require.Nil(t, (&TaskAdaptor{}).ValidateNormalizedVideoModel(c, info))
	}
	request := leonardoRequest("leonardo-seedance-2.0-fast-480p", duration)
	badRatio := "2:1"
	request.Output.AspectRatio = &badRatio
	err := (&TaskAdaptor{}).PrepareNormalizedVideoRequest(leonardoVideoTestContext(), leonardoInfo("seedance-2.0-fast-480p"), request)
	require.NotNil(t, err)
	assert.Equal(t, "invalid_video_aspect_ratio", err.Code)

	for _, modelName := range []string{"seedance-2.0", "seedance-2.0-2160p", "unknown-model"} {
		modelRequest := leonardoRequest("leonardo-seedance-2.0-fast-480p", duration)
		c := leonardoVideoTestContext()
		info := leonardoInfo(modelName)
		require.Nil(t, (&TaskAdaptor{}).PrepareNormalizedVideoRequest(c, info, modelRequest))
		err := (&TaskAdaptor{}).ValidateNormalizedVideoModel(c, info)
		require.NotNil(t, err)
		assert.Equal(t, "unsupported_video_model", err.Code)
	}
}

func TestLeonardoTaskLifecycleAndErrorMasking(t *testing.T) {
	adaptor := &TaskAdaptor{}
	queued, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"queued","progress":0}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusQueued, queued.Status)
	inProgress, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"in_progress","progress":47}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, inProgress.Status)
	assert.Equal(t, "47%", inProgress.Progress)
	assert.True(t, inProgress.ProgressMetadataSet)
	assert.True(t, inProgress.ProgressKnown)
	assert.Equal(t, "upstream_percent", inProgress.ProgressSource)
	complete, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"completed","progress":1,"duration":4,"video_url":"https://cdn.leonardo.ai/result.mp4"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, complete.Status)
	assert.Equal(t, "100%", complete.Progress)
	require.Len(t, complete.VideoOutputs, 1)
	assert.Equal(t, "https://cdn.leonardo.ai/result.mp4", complete.VideoOutputs[0].URL)
	assert.Empty(t, complete.VideoOutputs[0].Resolver)
	legacy, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"completed","duration":4}`))
	require.NoError(t, err)
	assert.Equal(t, videoContentResolver, legacy.VideoOutputs[0].Resolver)
	moderated, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"content_moderated","message":"Please revise the prompt"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, moderated.Status)
	assert.Equal(t, "Please revise the prompt", moderated.Reason)
	unknown, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"provider_secret","message":"secret provider detail"}}`))
	require.NoError(t, err)
	assert.Equal(t, "secret provider detail", unknown.Reason)
	internal, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"upstream_failed","message":"field publicJsonSchemaRegistry not found"}}`))
	require.NoError(t, err)
	assert.Equal(t, "field publicJsonSchemaRegistry not found", internal.Reason)
	privateUnavailable, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"private_generation_unavailable","message":"Private generation is unavailable for the selected account"}}`))
	require.NoError(t, err)
	assert.Equal(t, "Private generation is unavailable for the selected account", privateUnavailable.Reason)
	authFailure, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"auth_invalid","message":"account cookie expired"}}`))
	require.NoError(t, err)
	assert.Equal(t, "account cookie expired", authFailure.Reason)
	submissionUnknown, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"submission_unknown"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, submissionUnknown.Status)
	assert.Contains(t, submissionUnknown.Reason, "could not be confirmed")
}

func TestLeonardoRejectsLookalikeCDNAndSupportsContentResolver(t *testing.T) {
	for _, value := range []string{
		"https://cdn.leonardo.ai.evil.example/result.mp4",
		"https://cdn.leonardo.ai:444/result.mp4",
		"http://cdn.leonardo.ai/result.mp4",
		"https://user:pass@cdn.leonardo.ai/result.mp4",
	} {
		result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"task_id":"provider-1","status":"completed","video_url":"` + value + `"}`))
		require.NoError(t, err)
		assert.Empty(t, result.VideoOutputs[0].URL)
		assert.Equal(t, "provider-1", result.VideoOutputs[0].ProviderReference)
		assert.Equal(t, videoContentResolver, result.VideoOutputs[0].Resolver)
	}

	var auth, rangeHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		rangeHeader = r.Header.Get("Range")
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/4")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()
	base := upstream.URL
	channelModel := &model.Channel{BaseURL: &base, Key: "channel-key"}
	task := &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "provider-1", Key: "task-key"}}
	headers := make(http.Header)
	headers.Set("Range", "bytes=0-3")
	response, err := (&TaskAdaptor{}).ResolveVideoContent(context.Background(), channelModel, task, relaycommon.VideoOutput{ProviderReference: "provider-1", Resolver: videoContentResolver}, headers)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, "Bearer task-key", auth)
	assert.Equal(t, "bytes=0-3", rangeHeader)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, "data", string(body))
}

func TestLeonardoBuildRequestHeaderIncludesPublicTaskId(t *testing.T) {
	adaptor := &TaskAdaptor{apiKey: "secret"}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/v1/videos", nil)
	info := leonardoInfo("seedance-2.0-fast-480p")
	require.NoError(t, adaptor.BuildRequestHeader(leonardoVideoTestContext(), req, info))
	assert.Equal(t, "Bearer secret", req.Header.Get("Authorization"))
	assert.Equal(t, "task_public_123", req.Header.Get("Idempotency-Key"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestLeonardoSubmissionContinuesAfterClientCancellation(t *testing.T) {
	received := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"provider-1","status":"queued"}`))
	}))
	defer upstream.Close()

	c := leonardoVideoTestContext()
	cancelled, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(cancelled)
	cancel()
	adaptor := &TaskAdaptor{apiKey: "secret", baseURL: upstream.URL}
	response, err := adaptor.DoRequest(c, leonardoInfo("seedance-2.0-fast-480p"), strings.NewReader(`{}`))
	require.NoError(t, err)
	require.NotNil(t, response)
	defer response.Body.Close()
	assert.Equal(t, "task_public_123", <-received)
}
