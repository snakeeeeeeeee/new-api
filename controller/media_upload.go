package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxMediaUploadControlBodyBytes = 64 << 10

type internalMediaUploadCreateRequest struct {
	OwnerID string                       `json:"owner_id"`
	Files   []dto.MediaUploadFileRequest `json:"files"`
}

type internalMediaUploadCompleteRequest struct {
	OwnerID   string   `json:"owner_id"`
	UploadIDs []string `json:"upload_ids"`
}

func CreateMediaUploadSessions(c *gin.Context) {
	if !mediaUploadJSONRequest(c) {
		return
	}
	var request dto.MediaUploadCreateRequest
	if !decodeMediaUploadRequest(c, &request) {
		return
	}
	if len(request.Files) == 0 || len(request.Files) > 12 {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_media_files", "files must contain between 1 and 12 items", "files")
		return
	}
	result, problem := forwardMediaUploadControl[dto.MediaUploadSessionListResponse](
		c.Request.Context(),
		"/internal/v1/media/uploads",
		internalMediaUploadCreateRequest{
			OwnerID: fmt.Sprintf("%d", c.GetInt("id")),
			Files:   request.Files,
		},
	)
	if problem != nil {
		writeVideoTaskAPIError(c, problem.status, problem.code, problem.message, problem.param)
		return
	}
	c.JSON(http.StatusOK, result)
}

func CompleteMediaUploads(c *gin.Context) {
	if !mediaUploadJSONRequest(c) {
		return
	}
	var request dto.MediaUploadCompleteRequest
	if !decodeMediaUploadRequest(c, &request) {
		return
	}
	if len(request.UploadIDs) == 0 || len(request.UploadIDs) > 12 {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_upload_ids", "upload_ids must contain between 1 and 12 items", "upload_ids")
		return
	}
	result, problem := forwardMediaUploadControl[dto.MediaUploadListResponse](
		c.Request.Context(),
		"/internal/v1/media/uploads/complete",
		internalMediaUploadCompleteRequest{
			OwnerID:   fmt.Sprintf("%d", c.GetInt("id")),
			UploadIDs: request.UploadIDs,
		},
	)
	if problem != nil {
		writeVideoTaskAPIError(c, problem.status, problem.code, problem.message, problem.param)
		return
	}
	c.JSON(http.StatusOK, result)
}

func decodeMediaUploadRequest(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMediaUploadControlBodyBytes)
	if err := common.DecodeJsonStrict(c.Request.Body, target); err != nil {
		status := http.StatusBadRequest
		code := "invalid_request"
		if strings.Contains(err.Error(), "request body too large") {
			status = http.StatusRequestEntityTooLarge
			code = "upload_control_request_too_large"
		}
		writeVideoTaskAPIError(c, status, code, "Invalid media upload control request", "")
		return false
	}
	return true
}

func mediaUploadJSONRequest(c *gin.Context) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeVideoTaskAPIError(c, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json", "Content-Type")
		return false
	}
	return true
}

func forwardMediaUploadControl[T any](ctx context.Context, path string, payload any) (T, *videoTaskAPIProblem) {
	var result T
	if err := service.ValidateImageHandleSubmitConfig(); err != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusServiceUnavailable,
			code:   "media_upload_unavailable", message: "Media upload service is not configured",
		}
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusInternalServerError,
			code:   "server_error", message: "Failed to encode media upload request",
		}
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(service.GetImageHandleSubmitBaseURL(), "/")+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusInternalServerError,
			code:   "server_error", message: "Failed to create media upload request",
		}
	}
	request.Header.Set("Authorization", "Bearer "+service.GetImageHandleSubmitAPIKey())
	request.Header.Set("Content-Type", "application/json")
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusBadGateway,
			code:   "media_upload_failed", message: "Media upload service is unavailable",
		}
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if readErr != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusBadGateway,
			code:   "media_upload_failed", message: "Failed to read media upload response",
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var upstream dto.VideoTaskAPIErrorResponse
		if common.Unmarshal(responseBody, &upstream) == nil && upstream.Error.Code != "" {
			return result, &videoTaskAPIProblem{
				status: response.StatusCode,
				code:   upstream.Error.Code, message: upstream.Error.Message, param: upstream.Error.Param,
			}
		}
		return result, &videoTaskAPIProblem{
			status: response.StatusCode,
			code:   "media_upload_failed", message: "Media upload service rejected the request",
		}
	}
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return result, &videoTaskAPIProblem{
			status: http.StatusBadGateway,
			code:   "invalid_upload_response", message: "Media upload service returned an invalid response",
		}
	}
	return result, nil
}
