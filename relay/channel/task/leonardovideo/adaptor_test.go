package leonardovideo

import (
	"context"
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
	}, (&TaskAdaptor{}).GetModelList())
	assert.NotContains(t, ModelList, "seedance-2.0-2160p")
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
	request.ProviderOptions = map[string]map[string]any{ProviderOptionsNamespace: {"generate_audio": false}}

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
		{name: "audio reference", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.ReferenceMode = "media"
			r.Input.ReferenceAudios = []dto.VideoTaskSource{{URL: "https://example.com/a.mp3"}}
		}, code: "unsupported_reference_audio"},
		{name: "input video", mutate: func(r *dto.VideoTaskCreateRequest) {
			r.Input.Video = &dto.VideoTaskSource{URL: "https://example.com/a.mp4"}
		}, code: "unsupported_video_input"},
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
	assert.Equal(t, "Video task failed", unknown.Reason)
	internal, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"upstream_failed","message":"field publicJsonSchemaRegistry not found"}}`))
	require.NoError(t, err)
	assert.Equal(t, "Video task failed", internal.Reason)
	authFailure, err := adaptor.ParseTaskResult([]byte(`{"task_id":"provider-1","status":"failed","error":{"code":"auth_invalid","message":"account cookie expired"}}`))
	require.NoError(t, err)
	assert.Equal(t, "Video task failed", authFailure.Reason)
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
