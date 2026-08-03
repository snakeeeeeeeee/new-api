package controller

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAIVideoJSONDefaultsAndOfficialReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{
		"prompt":"animate the still",
		"input_reference":{"image_url":{"url":"https://cdn.example/input.png"}}
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	candidate, problem := parseOpenAIVideoCandidate(ctx)
	require.Nil(t, problem)
	assert.Equal(t, "sora-2", candidate.request.Model)
	assert.Equal(t, "generation", candidate.request.Operation)
	require.NotNil(t, candidate.request.Output.Duration)
	assert.Equal(t, 4, *candidate.request.Output.Duration)
	require.NotNil(t, candidate.request.Output.AspectRatio)
	assert.Equal(t, "9:16", *candidate.request.Output.AspectRatio)
	assert.Equal(t, "720x1280", candidate.compatibility.Size)
	assert.Equal(t, 4, candidate.compatibility.Seconds)
	require.NotNil(t, candidate.request.Input.Image)
	assert.Equal(t, "https://cdn.example/input.png", candidate.request.Input.Image.URL)
	assert.Equal(t, "frame", candidate.request.Input.ReferenceMode)
}

func TestParseOpenAIVideoCanvasMultipartReferences(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "adobe-seedance-2.0-fast-480p"))
	require.NoError(t, writer.WriteField("prompt", "combine references"))
	require.NoError(t, writer.WriteField("seconds", "6"))
	require.NoError(t, writer.WriteField("size", "1280x720"))
	require.NoError(t, writer.WriteField("resolution_name", "480p"))
	require.NoError(t, writer.WriteField("preset", "normal"))
	require.NoError(t, writer.WriteField("input_reference[]", "https://cdn.example/first.png"))
	part, err := writer.CreateFormFile("input_reference[]", "second.png")
	require.NoError(t, err)
	_, err = part.Write(testUploadPNG(t))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	candidate, problem := parseOpenAIVideoCandidate(ctx)
	require.Nil(t, problem)
	t.Cleanup(func() {
		if candidate.form != nil {
			_ = candidate.form.RemoveAll()
		}
	})

	assert.Equal(t, "media", candidate.request.Input.ReferenceMode)
	require.Len(t, candidate.request.Input.ReferenceImages, 2)
	assert.Equal(t, "https://cdn.example/first.png", candidate.request.Input.ReferenceImages[0].URL)
	assert.True(t, strings.HasPrefix(candidate.request.Input.ReferenceImages[1].URL, "https://multipart.invalid/image/"))
	require.NotNil(t, candidate.request.Output.Duration)
	assert.Equal(t, 6, *candidate.request.Output.Duration)
	require.NotNil(t, candidate.request.Output.AspectRatio)
	assert.Equal(t, "16:9", *candidate.request.Output.AspectRatio)
	assert.Equal(t, "480p", candidate.compatibility.ResolutionName)
}

func TestParseOpenAIVideoRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		code  string
		param string
	}{
		{name: "seconds", body: `{"prompt":"test","seconds":0}`, code: "invalid_request"},
		{name: "size", body: `{"prompt":"test","size":"landscape"}`, code: "invalid_video_size", param: "size"},
		{name: "preset", body: `{"prompt":"test","preset":"turbo"}`, code: "invalid_video_parameter", param: "preset"},
		{name: "file id", body: `{"prompt":"test","input_reference":{"file_id":"file_123"}}`, code: "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			_, problem := parseOpenAIVideoCandidate(ctx)
			require.NotNil(t, problem)
			assert.Equal(t, test.code, problem.code)
			assert.Equal(t, test.param, problem.param)
		})
	}
}

func TestPrepareOpenAIVideoCompatibilityOnlyActivatesExplicitAdaptors(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
		path        string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{
			name: "legacy adaptor keeps original invalid body", channelType: constant.ChannelTypeKling,
			path: "/v1/videos", body: `{"unexpected":true}`, wantStatus: http.StatusNoContent,
		},
		{
			name: "adobe validates compatibility body", channelType: constant.ChannelTypeAdobeVideo,
			path: "/v1/videos", body: `{"unexpected":true}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
		},
		{
			name: "adobe rejects edit capability", channelType: constant.ChannelTypeAdobeVideo,
			path: "/v1/videos/edits", body: `{"model":"adobe-seedance-2.0-fast-480p","prompt":"edit","video":"https://cdn.example/source.mp4"}`,
			wantStatus: http.StatusBadRequest, wantCode: "unsupported_video_capability",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST(test.path,
				CaptureOpenAIVideoRequest,
				func(c *gin.Context) {
					c.Set("platform", strconv.Itoa(test.channelType))
				},
				PrepareOpenAIVideoCompatibility,
				func(c *gin.Context) { c.Status(http.StatusNoContent) },
			)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
			if test.wantCode != "" {
				assert.Contains(t, recorder.Body.String(), `"code":"`+test.wantCode+`"`)
			}
		})
	}
}

func TestMaterializeOpenAIVideoImageUploadsUsesExistingImageUploadPath(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/v1/image/uploads", request.URL.Path)
		assert.Equal(t, "Bearer internal-upload-key", request.Header.Get("Authorization"))
		reader, err := request.MultipartReader()
		require.NoError(t, err)
		part, err := reader.NextPart()
		require.NoError(t, err)
		assert.Equal(t, "image", part.FormName())
		payload, err := io.ReadAll(part)
		require.NoError(t, err)
		assert.Equal(t, testUploadPNG(t), payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uploads":[],"images":["https://cdn.example/uploaded.png"]}`))
	}))
	defer upstream.Close()
	configureImageTaskUploadTest(t, upstream.URL)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "adobe-seedance-2.0-fast-480p"))
	require.NoError(t, writer.WriteField("prompt", "animate"))
	part, err := writer.CreateFormFile("input_reference", "input.png")
	require.NoError(t, err)
	_, err = part.Write(testUploadPNG(t))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	candidate, problem := parseOpenAIVideoCandidate(ctx)
	require.Nil(t, problem)
	t.Cleanup(func() { _ = candidate.form.RemoveAll() })

	require.Nil(t, materializeOpenAIVideoUploads(ctx, &candidate))
	assert.EqualValues(t, 1, calls.Load())
	require.NotNil(t, candidate.request.Input.Image)
	assert.Equal(t, "https://cdn.example/uploaded.png", candidate.request.Input.Image.URL)
}

func TestMaterializeOpenAIVideoFileUsesMediaUploadSessionFlow(t *testing.T) {
	var createCalls, putCalls, completeCalls atomic.Int32
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/internal/v1/media/uploads":
			createCalls.Add(1)
			var payload internalMediaUploadCreateRequest
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			assert.Equal(t, "42", payload.OwnerID)
			require.Len(t, payload.Files, 1)
			assert.Equal(t, "video", payload.Files[0].Kind)
			assert.Equal(t, "video/mp4", payload.Files[0].MimeType)
			_, _ = w.Write([]byte(`{"object":"media.upload.session.list","data":[{"id":"upload_video_1","kind":"video","method":"PUT","upload_url":"` + upstream.URL + `/upload","headers":{"Content-Type":"video/mp4"}}]}`))
		case "/upload":
			putCalls.Add(1)
			assert.Equal(t, http.MethodPut, request.Method)
			assert.Equal(t, "video/mp4", request.Header.Get("Content-Type"))
			payload, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.Equal(t, []byte("video-bytes"), payload)
			w.WriteHeader(http.StatusNoContent)
		case "/internal/v1/media/uploads/complete":
			completeCalls.Add(1)
			var payload internalMediaUploadCompleteRequest
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			assert.Equal(t, "42", payload.OwnerID)
			assert.Equal(t, []string{"upload_video_1"}, payload.UploadIDs)
			_, _ = w.Write([]byte(`{"object":"media.upload.list","data":[{"id":"upload_video_1","kind":"video","url":"https://cdn.example/source.mp4","mime_type":"video/mp4","size_bytes":11,"temporary":true}]}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer upstream.Close()
	configureImageTaskUploadTest(t, upstream.URL)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5"))
	require.NoError(t, writer.WriteField("prompt", "edit source"))
	header := make(textproto.MIMEHeader)
	header["Content-Disposition"] = []string{`form-data; name="video"; filename="source.mp4"`}
	header["Content-Type"] = []string{"video/mp4"}
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte("video-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/edits", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Set("id", 42)
	candidate, problem := parseOpenAIVideoCandidate(ctx)
	require.Nil(t, problem)
	t.Cleanup(func() { _ = candidate.form.RemoveAll() })

	require.Nil(t, materializeOpenAIVideoUploads(ctx, &candidate))
	assert.EqualValues(t, 1, createCalls.Load())
	assert.EqualValues(t, 1, putCalls.Load())
	assert.EqualValues(t, 1, completeCalls.Load())
	require.NotNil(t, candidate.request.Input.Video)
	assert.Equal(t, "https://cdn.example/source.mp4", candidate.request.Input.Video.URL)
}

func TestOpenAIVideoResourcesEnforceVideoTypeAndVariantAvailability(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	image := &model.Task{
		TaskID: "task_image_not_video", UserId: 17, Action: constant.TaskActionImageGeneration,
		Status: model.TaskStatusSuccess, Properties: model.Properties{AssetType: constant.TaskAssetTypeImage},
	}
	require.NoError(t, db.Create(image).Error)
	video := &model.Task{
		TaskID: "task_video_without_thumbnail", UserId: 17, Action: constant.TaskActionVideoGeneration,
		Status: model.TaskStatusSuccess, Properties: model.Properties{AssetType: constant.TaskAssetTypeVideo},
		PrivateData: model.TaskPrivateData{OpenAIVideoCompatibility: &dto.OpenAIVideoCompatibilityMetadata{Version: dto.OpenAIVideoCompatibilityVersion}},
	}
	require.NoError(t, db.Create(video).Error)

	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set("id", 17) })
	engine.GET("/v1/videos/:task_id", GetOpenAIVideo)
	engine.GET("/v1/videos/:task_id/content", OpenAIVideoContent)

	imageRecorder := httptest.NewRecorder()
	engine.ServeHTTP(imageRecorder, httptest.NewRequest(http.MethodGet, "/v1/videos/task_image_not_video", nil))
	assert.Equal(t, http.StatusNotFound, imageRecorder.Code)
	assert.Contains(t, imageRecorder.Body.String(), `"code":"video_not_found"`)

	variantRecorder := httptest.NewRecorder()
	engine.ServeHTTP(variantRecorder, httptest.NewRequest(http.MethodGet, "/v1/videos/task_video_without_thumbnail/content?variant=thumbnail", nil))
	assert.Equal(t, http.StatusNotFound, variantRecorder.Code)
	assert.Contains(t, variantRecorder.Body.String(), `"code":"video_variant_unavailable"`)
}

func TestReplayOpenAIVideoUsesCompatibilityStatusAndLocation(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.VideoTaskRequest{}))
	compatibility := &dto.OpenAIVideoCompatibilityMetadata{
		Version: dto.OpenAIVideoCompatibilityVersion, Seconds: 4, Size: "720x1280",
	}
	task := &model.Task{
		TaskID: "task_openai_replay", UserId: 73, Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeAdobeVideo)),
		Action: constant.TaskActionVideoGeneration, Status: model.TaskStatusQueued, Progress: "0%",
		Properties:  model.Properties{OriginModelName: "adobe-seedance-2.0-fast-480p", AssetType: constant.TaskAssetTypeVideo, Operation: "generation"},
		PrivateData: model.TaskPrivateData{OpenAIVideoCompatibility: compatibility},
	}
	require.NoError(t, db.Create(task).Error)
	key := "openai-replay"
	require.NoError(t, db.Create(model.NewVideoTaskRequest(task, 73, &key, "same-fingerprint", "", []byte(`{"model":"adobe"}`))).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	ctx.Set("id", 73)
	ctx.Set(relaycommon.OpenAIVideoCompatibilityContextKey, *compatibility)

	assert.True(t, replayVideoTaskRequest(ctx, key, "same-fingerprint"))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/v1/videos/task_openai_replay", recorder.Header().Get("Location"))
	assert.Equal(t, "true", recorder.Header().Get("Idempotent-Replayed"))
	assert.Contains(t, recorder.Body.String(), `"object":"video"`)
	assert.NotContains(t, recorder.Body.String(), "openai_video_compatibility")
}
