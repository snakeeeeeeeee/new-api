package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareImageTaskRequestRejectsUnknownFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.POST("/v1/image/tasks", PrepareImageTaskRequest, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader([]byte(`{
		"model":"gpt-image-2","operation":"generation","input":{"prompt":"draw"},
		"callback_url":"https://example.com/hook"
	}`)))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"invalid_request"`)
}

func TestPrepareImageTaskRequestKeepsPublicMetadataOutOfExecutorPayload(t *testing.T) {
	recorder := httptest.NewRecorder()
	engine := gin.New()
	var internal relaycommon.TaskSubmitReq
	var public dto.ImageTaskCreateRequest
	engine.POST("/v1/image/tasks", PrepareImageTaskRequest, func(c *gin.Context) {
		require.NoError(t, common.DecodeJson(c.Request.Body, &internal))
		value, exists := c.Get(relaycommon.ImageTaskPublicRequestContextKey)
		require.True(t, exists)
		public = value.(dto.ImageTaskCreateRequest)
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader([]byte(`{
		"model":"gpt-image-2","operation":"generation","input":{"prompt":"draw"},
		"output":{"count":1,"compression":0},"metadata":{"tenant":"acme","callback":{"url":"https://bad.example"}}
	}`)))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "acme", public.Metadata["tenant"])
	assert.NotContains(t, internal.Metadata, "tenant")
	assert.NotContains(t, internal.Metadata, "callback")
	assert.EqualValues(t, 0, internal.Metadata["output_compression"])
}

func TestValidateBase64ImageUploadsAcceptsPNGAndRejectsNonImage(t *testing.T) {
	png := testUploadPNG(t)
	body, err := common.Marshal(map[string]any{"images": []any{base64.StdEncoding.EncodeToString(png)}})
	require.NoError(t, err)
	code, _, validationErr := validateImageUploadRequest("application/json", "/v1/image/uploads/base64", body)
	assert.Empty(t, code)
	require.NoError(t, validationErr)

	badBody, err := common.Marshal(map[string]any{"images": []any{base64.StdEncoding.EncodeToString([]byte("plain text"))}})
	require.NoError(t, err)
	code, _, validationErr = validateImageUploadRequest("application/json", "/v1/image/uploads/base64", badBody)
	assert.Equal(t, "invalid_upload_image", code)
	require.Error(t, validationErr)
}

func TestValidateMultipartImageUploadsAllowsRepeatedImagesAndOneMask(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range []string{"image", "image", "mask"} {
		part, err := writer.CreateFormFile(field, field+".png")
		require.NoError(t, err)
		_, err = part.Write(testUploadPNG(t))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	code, _, validationErr := validateImageUploadRequest(writer.FormDataContentType(), "/v1/image/uploads", body.Bytes())
	assert.Empty(t, code)
	require.NoError(t, validationErr)
}

func TestParseMultipartImageTaskMapsSyncEditFields(t *testing.T) {
	body, contentType := multipartImageTaskBody(t, [][2]string{
		{"model", " gpt-image-2 "},
		{"prompt", " Replace the background "},
		{"operation", "EDIT"},
		{"n", "2"},
		{"size", "1024x1024"},
		{"quality", "high"},
		{"output_format", "png"},
		{"output_compression", "0"},
		{"background", "transparent"},
		{"client_reference_id", "order_123"},
		{"metadata", `{"tenant":"acme"}`},
	}, []multipartImageTaskTestFile{
		{field: "image", filename: "first.png", data: testUploadPNG(t)},
		{field: "image", filename: "second.png", data: append(testUploadPNG(t), 1)},
		{field: "mask", filename: "mask.png", data: append(testUploadPNG(t), 2)},
	})
	ctx := multipartImageTaskContext(body, contentType)
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

	preparation, problem := parseMultipartImageTaskRequest(ctx)
	require.Nil(t, problem)
	require.NotEmpty(t, preparation.fingerprint)
	assert.Equal(t, "gpt-image-2", preparation.request.Model)
	assert.Equal(t, "edit", preparation.request.Operation)
	assert.Equal(t, "Replace the background", preparation.request.Input.Prompt)
	require.Len(t, preparation.request.Input.Images, 2)
	require.NotNil(t, preparation.request.Input.Mask)
	require.NotNil(t, preparation.request.Output.Count)
	assert.Equal(t, 2, *preparation.request.Output.Count)
	require.NotNil(t, preparation.request.Output.Compression)
	assert.Equal(t, 0, *preparation.request.Output.Compression)
	assert.Equal(t, "acme", preparation.request.Metadata["tenant"])

	mediaType, params, err := mime.ParseMediaType(preparation.uploadContentType)
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(bytes.NewReader(preparation.uploadBody), params["boundary"])
	fields := make([]string, 0, 3)
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		require.NoError(t, nextErr)
		fields = append(fields, part.FormName())
		assert.NotEmpty(t, part.FileName())
		_, err = io.Copy(io.Discard, part)
		require.NoError(t, err)
		_ = part.Close()
	}
	assert.Equal(t, []string{"image", "image", "mask"}, fields)
}

func TestMultipartImageTaskFingerprintUsesFileContentsNotFilenames(t *testing.T) {
	fields := [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}
	bodyA, contentTypeA := multipartImageTaskBody(t, fields, []multipartImageTaskTestFile{{field: "image", filename: "a.png", data: testUploadPNG(t)}})
	bodyB, contentTypeB := multipartImageTaskBody(t, fields, []multipartImageTaskTestFile{{field: "image", filename: "renamed.png", data: testUploadPNG(t)}})
	bodyC, contentTypeC := multipartImageTaskBody(t, fields, []multipartImageTaskTestFile{{field: "image", filename: "a.png", data: append(testUploadPNG(t), 1)}})

	preparationA, problemA := parseMultipartImageTaskRequest(multipartImageTaskContext(bodyA, contentTypeA))
	preparationB, problemB := parseMultipartImageTaskRequest(multipartImageTaskContext(bodyB, contentTypeB))
	preparationC, problemC := parseMultipartImageTaskRequest(multipartImageTaskContext(bodyC, contentTypeC))
	require.Nil(t, problemA)
	require.Nil(t, problemB)
	require.Nil(t, problemC)
	assert.Equal(t, preparationA.fingerprint, preparationB.fingerprint)
	assert.NotEqual(t, preparationA.fingerprint, preparationC.fingerprint)
}

func TestPrepareMultipartImageTaskUploadsThenUsesNormalizedTaskFlow(t *testing.T) {
	var uploadCalls atomic.Int32
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		uploadCalls.Add(1)
		assert.Equal(t, "/v1/image/uploads", request.URL.Path)
		assert.Equal(t, "Bearer internal-upload-key", request.Header.Get("Authorization"))
		form, err := request.MultipartReader()
		require.NoError(t, err)
		fileFields := make([]string, 0, 2)
		for {
			part, nextErr := form.NextPart()
			if nextErr == io.EOF {
				break
			}
			require.NoError(t, nextErr)
			fileFields = append(fileFields, part.FormName())
			_, err = io.Copy(io.Discard, part)
			require.NoError(t, err)
			_ = part.Close()
		}
		assert.Equal(t, []string{"image", "mask"}, fileFields)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"uploads":[],"images":["https://cdn.example.com/input.png"],"mask":"https://cdn.example.com/mask.png"}`))
	}))
	defer uploadServer.Close()
	configureImageTaskUploadTest(t, uploadServer.URL)

	body, contentType := multipartImageTaskBody(t, [][2]string{
		{"model", "gpt-image-2"}, {"prompt", "replace background"}, {"output_compression", "0"},
	}, []multipartImageTaskTestFile{
		{field: "image", filename: "input.png", data: testUploadPNG(t)},
		{field: "mask", filename: "mask.png", data: testUploadPNG(t)},
	})
	var internal relaycommon.TaskSubmitReq
	var public dto.ImageTaskCreateRequest
	engine := gin.New()
	engine.POST("/v1/image/tasks", func(c *gin.Context) { c.Set("id", 7) }, PrepareImageTaskRequest, func(c *gin.Context) {
		require.NoError(t, common.DecodeJson(c.Request.Body, &internal))
		value, exists := c.Get(relaycommon.ImageTaskPublicRequestContextKey)
		require.True(t, exists)
		public = value.(dto.ImageTaskCreateRequest)
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	assert.EqualValues(t, 1, uploadCalls.Load())
	assert.Equal(t, "edit", internal.Mode)
	assert.Equal(t, []string{"https://cdn.example.com/input.png"}, internal.Images)
	assert.Equal(t, "https://cdn.example.com/mask.png", internal.Metadata["mask"])
	assert.EqualValues(t, 0, internal.Metadata["output_compression"])
	assert.Equal(t, "https://cdn.example.com/input.png", public.Input.Images[0].URL)
}

func TestParseMultipartImageTaskRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		fields    [][2]string
		files     []multipartImageTaskTestFile
		wantCode  string
		wantParam string
	}{
		{name: "missing model", fields: [][2]string{{"prompt", "edit"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "model"},
		{name: "missing prompt", fields: [][2]string{{"model", "gpt-image-2"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "prompt"},
		{name: "missing image", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}, wantCode: "missing_upload_file", wantParam: "image"},
		{name: "generation operation", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"operation", "generation"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "operation"},
		{name: "invalid n", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"n", "zero"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "n"},
		{name: "zero n", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"n", "0"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "n"},
		{name: "invalid compression", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"output_compression", "101"}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "output_compression"},
		{name: "invalid metadata", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"metadata", `[]`}}, files: validMultipartTaskImage(t), wantCode: "invalid_request", wantParam: "metadata"},
		{name: "unknown scalar", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"provider_options", `{}`}}, files: validMultipartTaskImage(t), wantCode: "invalid_multipart_field", wantParam: "provider_options"},
		{name: "unknown file", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}, files: []multipartImageTaskTestFile{{field: "photo", filename: "photo.png", data: testUploadPNG(t)}}, wantCode: "invalid_upload_field", wantParam: "photo"},
		{name: "invalid image", fields: [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}, files: []multipartImageTaskTestFile{{field: "image", filename: "input.txt", data: []byte("not an image")}}, wantCode: "invalid_upload_image", wantParam: "image"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, contentType := multipartImageTaskBody(t, test.fields, test.files)
			ctx := multipartImageTaskContext(body, contentType)
			_, problem := parseMultipartImageTaskRequest(ctx)
			require.NotNil(t, problem)
			assert.Equal(t, test.wantCode, problem.code)
			assert.Equal(t, test.wantParam, problem.param)
			common.CleanupBodyStorage(ctx)
		})
	}
}

func TestParseMultipartImageTaskEnforcesUploadSizeLimits(t *testing.T) {
	oversizedImage := make([]byte, maxImageUploadFileBytes+1)
	copy(oversizedImage, testUploadPNG(t))
	body, contentType := multipartImageTaskBody(t, [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}, []multipartImageTaskTestFile{{field: "image", filename: "large.png", data: oversizedImage}})
	ctx := multipartImageTaskContext(body, contentType)
	_, problem := parseMultipartImageTaskRequest(ctx)
	require.NotNil(t, problem)
	assert.Equal(t, http.StatusRequestEntityTooLarge, problem.status)
	assert.Equal(t, "upload_file_too_large", problem.code)
	common.CleanupBodyStorage(ctx)

	requestCtx := multipartImageTaskContext(nil, "multipart/form-data; boundary=test")
	requestCtx.Set(common.KeyBodyStorage, oversizedImageTaskBodyStorage{})
	_, problem = parseMultipartImageTaskRequest(requestCtx)
	require.NotNil(t, problem)
	assert.Equal(t, http.StatusRequestEntityTooLarge, problem.status)
	assert.Equal(t, "upload_request_too_large", problem.code)
}

func TestPrepareMultipartImageTaskPropagatesUploadFailure(t *testing.T) {
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"storage_unavailable","message":"storage unavailable"}}`))
	}))
	defer uploadServer.Close()
	configureImageTaskUploadTest(t, uploadServer.URL)
	body, contentType := multipartImageTaskBody(t, [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}}, validMultipartTaskImage(t))
	engine := gin.New()
	engine.POST("/v1/image/tasks", PrepareImageTaskRequest, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"storage_unavailable"`)
}

func TestPrepareMultipartImageTaskIdempotencyPrecedesUpload(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	var uploadCalls atomic.Int32
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		uploadCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer uploadServer.Close()
	configureImageTaskUploadTest(t, uploadServer.URL)

	fields := [][2]string{{"model", "gpt-image-2"}, {"prompt", "edit"}, {"metadata", `{"order":123}`}}
	originalImage := testUploadPNG(t)
	body, contentType := multipartImageTaskBody(t, fields, []multipartImageTaskTestFile{{field: "image", filename: "input.png", data: originalImage}})
	preparation, problem := parseMultipartImageTaskRequest(multipartImageTaskContext(body, contentType))
	require.Nil(t, problem)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_multipart_replay", Platform: imageHandleTaskPlatform(), UserId: 71,
		Action: constant.TaskActionImageEdit, Status: model.TaskStatusQueued, Progress: "0%",
		CreatedAt: now, UpdatedAt: now, SubmitTime: now, Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}
	require.NoError(t, db.Create(task).Error)
	storedRequest := dto.ImageTaskCreateRequest{Model: "gpt-image-2", Operation: "edit", Input: dto.ImageTaskInputRequest{Prompt: "edit", Images: []dto.ImageTaskSource{{URL: "https://cdn.example.com/previous.png"}}}, Metadata: map[string]any{"order": float64(123)}}
	storedJSON, err := common.Marshal(storedRequest)
	require.NoError(t, err)
	idempotencyKey := "multipart-order-123"
	require.NoError(t, db.Create(model.NewImageTaskRequest(task, 71, &idempotencyKey, preparation.fingerprint, "", storedJSON)).Error)

	serve := func(requestBody []byte, requestContentType string) *httptest.ResponseRecorder {
		engine := gin.New()
		engine.POST("/v1/image/tasks", func(c *gin.Context) { c.Set("id", 71) }, PrepareImageTaskRequest, func(c *gin.Context) { c.Status(http.StatusNoContent) })
		request := httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader(requestBody))
		request.Header.Set("Content-Type", requestContentType)
		request.Header.Set("Idempotency-Key", idempotencyKey)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	replay := serve(body, contentType)
	require.Equal(t, http.StatusAccepted, replay.Code, replay.Body.String())
	assert.Equal(t, "true", replay.Header().Get("Idempotent-Replayed"))
	assert.EqualValues(t, 0, uploadCalls.Load())

	conflictingBody, conflictingContentType := multipartImageTaskBody(t, fields, []multipartImageTaskTestFile{{field: "image", filename: "input.png", data: append(originalImage, 1)}})
	conflict := serve(conflictingBody, conflictingContentType)
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	assert.Contains(t, conflict.Body.String(), `"code":"idempotency_key_conflict"`)
	assert.EqualValues(t, 0, uploadCalls.Load())
}

type multipartImageTaskTestFile struct {
	field    string
	filename string
	data     []byte
}

type oversizedImageTaskBodyStorage struct{}

func (oversizedImageTaskBodyStorage) Read([]byte) (int, error)       { return 0, io.EOF }
func (oversizedImageTaskBodyStorage) Seek(int64, int) (int64, error) { return 0, nil }
func (oversizedImageTaskBodyStorage) Close() error                   { return nil }
func (oversizedImageTaskBodyStorage) Bytes() ([]byte, error)         { return nil, nil }
func (oversizedImageTaskBodyStorage) Size() int64                    { return maxImageUploadRequestBytes + 1 }
func (oversizedImageTaskBodyStorage) IsDisk() bool                   { return false }

func multipartImageTaskBody(t *testing.T, fields [][2]string, files []multipartImageTaskTestFile) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		require.NoError(t, writer.WriteField(field[0], field[1]))
	}
	for _, file := range files {
		part, err := writer.CreateFormFile(file.field, file.filename)
		require.NoError(t, err)
		_, err = part.Write(file.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return body.Bytes(), writer.FormDataContentType()
}

func multipartImageTaskContext(body []byte, contentType string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", contentType)
	return ctx
}

func validMultipartTaskImage(t *testing.T) []multipartImageTaskTestFile {
	t.Helper()
	return []multipartImageTaskTestFile{{field: "image", filename: "input.png", data: testUploadPNG(t)}}
}

func configureImageTaskUploadTest(t *testing.T, baseURL string) {
	t.Helper()
	original := *image_handle_setting.GetImageHandleSetting()
	t.Cleanup(func() { *image_handle_setting.GetImageHandleSetting() = original })
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL: baseURL,
		APIKey:  "internal-upload-key",
	})
	service.InitHttpClient()
}

func testUploadPNG(t *testing.T) []byte {
	t.Helper()
	value, err := hex.DecodeString("89504e470d0a1a0a0000000d494844520000000100000001")
	require.NoError(t, err)
	return value
}
