package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMediaUploadSessionsForwardsMetadataAndOwner(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/internal/v1/media/uploads", request.URL.Path)
		assert.Equal(t, "Bearer internal-upload-key", request.Header.Get("Authorization"))
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))

		var body internalMediaUploadCreateRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		assert.Equal(t, "42", body.OwnerID)
		require.Len(t, body.Files, 1)
		assert.Equal(t, "motion", body.Files[0].ClientID)
		assert.Equal(t, "video", body.Files[0].Kind)
		assert.Equal(t, "video/mp4", body.Files[0].MimeType)
		assert.EqualValues(t, 4096, body.Files[0].SizeBytes)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"object":"media.upload.session.list",
			"data":[{
				"id":"upload_1",
				"client_id":"motion",
				"kind":"video",
				"method":"PUT",
				"upload_url":"https://r2.example/upload",
				"headers":{"Content-Type":"video/mp4"},
				"expires_at":123
			}]
		}`))
	}))
	defer upstream.Close()
	configureImageTaskUploadTest(t, upstream.URL)

	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.POST("/v1/media/uploads", func(c *gin.Context) {
		c.Set("id", 42)
	}, CreateMediaUploadSessions)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/media/uploads",
		bytes.NewBufferString(`{
			"files":[{
				"client_id":"motion",
				"kind":"video",
				"filename":"motion.mp4",
				"mime_type":"video/mp4",
				"size_bytes":4096
			}]
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(1), calls.Load())
	assert.JSONEq(t, `{
		"object":"media.upload.session.list",
		"data":[{
			"id":"upload_1",
			"client_id":"motion",
			"kind":"video",
			"method":"PUT",
			"upload_url":"https://r2.example/upload",
			"headers":{"Content-Type":"video/mp4"},
			"expires_at":123
		}]
	}`, recorder.Body.String())
}

func TestCreateMediaUploadSessionsRejectsNonJSONAndUnknownFields(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer upstream.Close()
	configureImageTaskUploadTest(t, upstream.URL)

	engine := gin.New()
	engine.POST("/v1/media/uploads", CreateMediaUploadSessions)

	multipartRecorder := httptest.NewRecorder()
	multipartRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/media/uploads",
		bytes.NewBufferString("file-bytes"),
	)
	multipartRequest.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	engine.ServeHTTP(multipartRecorder, multipartRequest)
	assert.Equal(t, http.StatusUnsupportedMediaType, multipartRecorder.Code)

	unknownRecorder := httptest.NewRecorder()
	unknownRequest := httptest.NewRequest(
		http.MethodPost,
		"/v1/media/uploads",
		bytes.NewBufferString(`{"files":[],"data":"base64"}`),
	)
	unknownRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(unknownRecorder, unknownRequest)
	assert.Equal(t, http.StatusBadRequest, unknownRecorder.Code)
	assert.Contains(t, unknownRecorder.Body.String(), `"code":"invalid_request"`)
	assert.Zero(t, calls.Load())
}

func TestCompleteMediaUploadsForwardsProviderError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/internal/v1/media/uploads/complete", request.URL.Path)
		var body internalMediaUploadCompleteRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		assert.Equal(t, "7", body.OwnerID)
		assert.Equal(t, []string{"upload_missing"}, body.UploadIDs)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{
			"error":{
				"type":"invalid_request_error",
				"code":"media_upload_not_found",
				"message":"Media upload session was not found",
				"param":"upload_ids"
			}
		}`))
	}))
	defer upstream.Close()
	configureImageTaskUploadTest(t, upstream.URL)

	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.POST("/v1/media/uploads/complete", func(c *gin.Context) {
		c.Set("id", 7)
	}, CompleteMediaUploads)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/media/uploads/complete",
		bytes.NewBufferString(`{"upload_ids":["upload_missing"]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"media_upload_not_found"`)
	assert.Contains(t, recorder.Body.String(), `"param":"upload_ids"`)
}
