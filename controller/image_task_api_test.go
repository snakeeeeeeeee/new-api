package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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

func testUploadPNG(t *testing.T) []byte {
	t.Helper()
	value, err := hex.DecodeString("89504e470d0a1a0a0000000d494844520000000100000001")
	require.NoError(t, err)
	return value
}
