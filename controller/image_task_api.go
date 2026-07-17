package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxImageTaskIdempotencyKeyLength = 128

const (
	maxImageUploadRequestBytes = 100 << 20
	maxImageUploadFileBytes    = 20 << 20
	maxImageUploadCount        = 10
)

func PrepareImageTaskRequest(c *gin.Context) {
	var request dto.ImageTaskCreateRequest
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "Invalid JSON request body", "")
		c.Abort()
		return
	}
	rawBody, err := storage.Bytes()
	if err != nil || common.UnmarshalStrict(rawBody, &request) != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "Invalid JSON request body", "")
		c.Abort()
		return
	}
	if param, message := validateImageTaskCreateRequest(&request); message != "" {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", message, param)
		c.Abort()
		return
	}
	canonical, err := common.Marshal(request)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to normalize request", "")
		c.Abort()
		return
	}
	fingerprintBytes := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > maxImageTaskIdempotencyKeyLength {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must not exceed 128 characters", "Idempotency-Key")
		c.Abort()
		return
	}
	if idempotencyKey != "" {
		existing, exists, lookupErr := model.GetImageTaskRequestByIdempotencyKey(c.GetInt("id"), idempotencyKey)
		if lookupErr != nil {
			writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to resolve Idempotency-Key", "Idempotency-Key")
			c.Abort()
			return
		}
		if exists {
			if existing.RequestFingerprint != fingerprint {
				writeImageTaskAPIError(c, http.StatusConflict, "idempotency_key_conflict", "Idempotency-Key was already used with a different request", "Idempotency-Key")
				c.Abort()
				return
			}
			task, taskExists, taskErr := model.GetPublicImageTask(c.GetInt("id"), existing.TaskID, imageHandleTaskPlatform())
			if taskErr != nil || !taskExists {
				writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Idempotent task could not be loaded", "")
				c.Abort()
				return
			}
			publicTask, buildErr := service.BuildPublicImageTask(task)
			if buildErr != nil {
				writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task response", "")
				c.Abort()
				return
			}
			c.Header("Idempotent-Replayed", "true")
			c.Header("Location", "/v1/image/tasks/"+task.TaskID)
			c.Header("Retry-After", "2")
			c.JSON(http.StatusAccepted, publicTask)
			c.Abort()
			return
		}
	}

	legacy := normalizedImageTaskToLegacy(request)
	legacyBody, err := common.Marshal(legacy)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to prepare task request", "")
		c.Abort()
		return
	}
	c.Set(relaycommon.ImageTaskPublicRequestContextKey, request)
	c.Set(relaycommon.ImageTaskPublicRequestJSONContextKey, canonical)
	c.Set(relaycommon.ImageTaskFingerprintContextKey, fingerprint)
	c.Set(relaycommon.ImageTaskIdempotencyKeyContextKey, idempotencyKey)
	common.CleanupBodyStorage(c)
	c.Set(common.KeyRequestBody, nil)
	c.Request.Body = io.NopCloser(bytes.NewReader(legacyBody))
	c.Request.ContentLength = int64(len(legacyBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Next()
}

func validateImageTaskCreateRequest(request *dto.ImageTaskCreateRequest) (string, string) {
	request.Model = strings.TrimSpace(request.Model)
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	request.Input.Prompt = strings.TrimSpace(request.Input.Prompt)
	request.ClientReferenceID = strings.TrimSpace(request.ClientReferenceID)
	if request.Model == "" {
		return "model", "model is required"
	}
	if request.Operation != "generation" && request.Operation != "edit" {
		return "operation", "operation must be generation or edit"
	}
	if request.Input.Prompt == "" {
		return "input.prompt", "input.prompt is required"
	}
	if request.Operation == "edit" && len(request.Input.Images) == 0 {
		return "input.images", "input.images is required for edit tasks"
	}
	for index := range request.Input.Images {
		request.Input.Images[index].URL = strings.TrimSpace(request.Input.Images[index].URL)
		if !validImageTaskSourceURL(request.Input.Images[index].URL) {
			return fmt.Sprintf("input.images[%d].url", index), "image URL must use http or https"
		}
	}
	if request.Input.Mask != nil {
		request.Input.Mask.URL = strings.TrimSpace(request.Input.Mask.URL)
		if !validImageTaskSourceURL(request.Input.Mask.URL) {
			return "input.mask.url", "mask URL must use http or https"
		}
	}
	if request.Output.Count != nil && (*request.Output.Count < 1 || *request.Output.Count > dto.MaxImageN) {
		return "output.count", fmt.Sprintf("output.count must be between 1 and %d", dto.MaxImageN)
	}
	if request.Output.Compression != nil && (*request.Output.Compression < 0 || *request.Output.Compression > 100) {
		return "output.compression", "output.compression must be between 0 and 100"
	}
	if len(request.ClientReferenceID) > 191 {
		return "client_reference_id", "client_reference_id must not exceed 191 characters"
	}
	if request.Metadata == nil {
		request.Metadata = map[string]any{}
	}
	return "", ""
}

func validImageTaskSourceURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func normalizedImageTaskToLegacy(request dto.ImageTaskCreateRequest) relaycommon.TaskSubmitReq {
	metadata := make(map[string]any, 8)
	metadata["operation"] = request.Operation
	metadata["result_data_format"] = "url"
	legacy := relaycommon.TaskSubmitReq{
		Prompt:   request.Input.Prompt,
		Model:    request.Model,
		Mode:     request.Operation,
		Metadata: metadata,
	}
	for _, source := range request.Input.Images {
		legacy.Images = append(legacy.Images, source.URL)
	}
	if request.Input.Mask != nil {
		metadata["mask"] = request.Input.Mask.URL
	}
	if request.Output.Count != nil {
		legacy.N = request.Output.Count
		metadata["n"] = *request.Output.Count
	}
	if request.Output.Size != nil {
		legacy.Size = *request.Output.Size
	}
	legacy.Quality = request.Output.Quality
	if request.Output.Quality != nil {
		metadata["quality"] = *request.Output.Quality
	}
	if request.Output.Format != nil {
		metadata["output_format"] = *request.Output.Format
	}
	if request.Output.Compression != nil {
		metadata["output_compression"] = *request.Output.Compression
	}
	if request.Output.Background != nil {
		metadata["background"] = *request.Output.Background
	}
	return legacy
}

func ListImageTasks(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100", "limit")
		return
	}
	createdAfter, afterErr := parseOptionalUnixQuery(c, "created_after")
	createdBefore, beforeErr := parseOptionalUnixQuery(c, "created_before")
	if afterErr != nil || beforeErr != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "created_after and created_before must be Unix timestamps", "")
		return
	}
	statuses, statusErr := internalImageTaskStatuses(c.Query("status"))
	if statusErr != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", statusErr.Error(), "status")
		return
	}
	operation, operationErr := internalImageTaskOperation(c.Query("operation"))
	if operationErr != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", operationErr.Error(), "operation")
		return
	}
	tasks, hasMore, queryErr := model.ListPublicImageTasks(c.GetInt("id"), imageHandleTaskPlatform(), model.ImageTaskListQuery{
		Statuses: statuses, Operation: operation, ClientReferenceID: strings.TrimSpace(c.Query("client_reference_id")),
		CreatedAfter: createdAfter, CreatedBefore: createdBefore, AfterTaskID: strings.TrimSpace(c.Query("after")), Limit: limit,
	})
	if queryErr != nil {
		if c.Query("after") != "" {
			writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_cursor", "after task was not found", "after")
			return
		}
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to list tasks", "")
		return
	}
	publicTasks, buildErr := service.BuildPublicImageTasks(tasks)
	if buildErr != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task list", "")
		return
	}
	response := dto.ImageTaskListResponse{Object: "list", Data: publicTasks, HasMore: hasMore}
	if len(publicTasks) > 0 {
		response.FirstID = publicTasks[0].ID
		response.LastID = publicTasks[len(publicTasks)-1].ID
	}
	c.JSON(http.StatusOK, response)
}

func parseOptionalUnixQuery(c *gin.Context, name string) (int64, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid timestamp")
	}
	return parsed, nil
}

func internalImageTaskStatuses(value string) ([]model.TaskStatus, error) {
	switch strings.TrimSpace(value) {
	case "":
		return nil, nil
	case "queued":
		return []model.TaskStatus{model.TaskStatusSubmitted, model.TaskStatusQueued}, nil
	case "in_progress":
		return []model.TaskStatus{model.TaskStatusInProgress}, nil
	case "succeeded":
		return []model.TaskStatus{model.TaskStatusSuccess}, nil
	case "failed":
		return []model.TaskStatus{model.TaskStatusFailure}, nil
	default:
		return nil, fmt.Errorf("status is invalid")
	}
}

func internalImageTaskOperation(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "":
		return "", nil
	case "generation":
		return constant.TaskActionImageGeneration, nil
	case "edit":
		return constant.TaskActionImageEdit, nil
	default:
		return "", fmt.Errorf("operation is invalid")
	}
}

func ProxyImageTaskUpload(c *gin.Context) {
	if err := service.ValidateImageHandleSubmitConfig(); err != nil {
		writeImageTaskAPIError(c, http.StatusServiceUnavailable, "image_upload_unavailable", "Image upload service is not configured", "")
		return
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) {
			status = http.StatusRequestEntityTooLarge
		}
		writeImageTaskAPIError(c, status, "invalid_upload_body", err.Error(), "")
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_upload_body", "Failed to read upload body", "")
		return
	}
	if storage.Size() > maxImageUploadRequestBytes {
		writeImageTaskAPIError(c, http.StatusRequestEntityTooLarge, "upload_request_too_large", "Upload request exceeds 100 MiB", "")
		return
	}
	path := "/v1/image/uploads"
	if strings.HasSuffix(c.Request.URL.Path, "/base64") {
		path += "/base64"
	}
	if code, param, validationErr := validateImageUploadRequest(c.GetHeader("Content-Type"), path, body); validationErr != nil {
		status := http.StatusBadRequest
		if code == "upload_file_too_large" {
			status = http.StatusRequestEntityTooLarge
		}
		writeImageTaskAPIError(c, status, code, validationErr.Error(), param)
		return
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, strings.TrimRight(service.GetImageHandleSubmitBaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to create upload request", "")
		return
	}
	request.Header.Set("Authorization", "Bearer "+service.GetImageHandleSubmitAPIKey())
	request.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	response, err := service.GetHttpClient().Do(request)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusBadGateway, "image_upload_failed", "Image upload service is unavailable", "")
		return
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var upstream dto.ImageTaskAPIErrorResponse
		if common.Unmarshal(responseBody, &upstream) == nil && upstream.Error.Code != "" {
			writeImageTaskAPIError(c, response.StatusCode, upstream.Error.Code, upstream.Error.Message, upstream.Error.Param)
			return
		}
		writeImageTaskAPIError(c, response.StatusCode, "image_upload_failed", "Image upload failed", "")
		return
	}
	var upstream struct {
		Uploads []struct {
			ID        string `json:"id"`
			Field     string `json:"field"`
			URL       string `json:"url"`
			MimeType  string `json:"mime_type"`
			Bytes     int64  `json:"bytes"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Format    string `json:"format"`
			Temporary bool   `json:"temporary"`
		} `json:"uploads"`
		Images []string `json:"images"`
		Mask   *string  `json:"mask"`
	}
	if err := common.Unmarshal(responseBody, &upstream); err != nil {
		writeImageTaskAPIError(c, http.StatusBadGateway, "invalid_upload_response", "Image upload service returned an invalid response", "")
		return
	}
	result := dto.ImageUploadListResponse{Object: "image.upload.list", Images: upstream.Images, Mask: upstream.Mask}
	for _, upload := range upstream.Uploads {
		result.Data = append(result.Data, dto.ImageUploadPublic{
			ID: upload.ID, Object: "image.upload", Field: upload.Field, URL: upload.URL, MimeType: upload.MimeType,
			SizeBytes: upload.Bytes, Width: upload.Width, Height: upload.Height, Format: upload.Format, Temporary: true,
		})
	}
	c.JSON(http.StatusOK, result)
}

func validateImageUploadRequest(contentType, path string, body []byte) (string, string, error) {
	if strings.HasSuffix(path, "/base64") {
		return validateBase64ImageUploads(body)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return "invalid_upload_body", "", fmt.Errorf("request body must be multipart/form-data")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	imageCount := 0
	maskCount := 0
	totalFiles := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "invalid_upload_body", "", fmt.Errorf("invalid multipart upload")
		}
		field := part.FormName()
		if part.FileName() == "" {
			_ = part.Close()
			continue
		}
		switch field {
		case "image":
			imageCount++
			if imageCount > maxImageUploadCount {
				_ = part.Close()
				return "too_many_uploads", "image", fmt.Errorf("at most %d images may be uploaded", maxImageUploadCount)
			}
		case "mask":
			maskCount++
			if maskCount > 1 {
				_ = part.Close()
				return "too_many_uploads", "mask", fmt.Errorf("only one mask may be uploaded")
			}
		default:
			_ = part.Close()
			return "invalid_upload_field", field, fmt.Errorf("file field must be image or mask")
		}
		fileBytes, readErr := io.ReadAll(io.LimitReader(part, maxImageUploadFileBytes+1))
		_ = part.Close()
		if readErr != nil {
			return "invalid_upload_body", field, fmt.Errorf("failed to read uploaded image")
		}
		if len(fileBytes) > maxImageUploadFileBytes {
			return "upload_file_too_large", field, fmt.Errorf("uploaded image exceeds 20 MiB")
		}
		if err := validateUploadedImageBytes(fileBytes); err != nil {
			return "invalid_upload_image", field, err
		}
		totalFiles++
	}
	if totalFiles == 0 {
		return "missing_upload_file", "image", fmt.Errorf("at least one image or mask file is required")
	}
	return "", "", nil
}

func validateBase64ImageUploads(body []byte) (string, string, error) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return "invalid_upload_body", "", fmt.Errorf("invalid JSON request body")
	}
	items := make([]struct {
		field string
		value any
	}, 0)
	if uploads, ok := payload["uploads"].([]any); ok {
		for _, item := range uploads {
			items = append(items, struct {
				field string
				value any
			}{field: "image", value: item})
		}
	} else {
		if images, ok := payload["images"].([]any); ok {
			for _, item := range images {
				items = append(items, struct {
					field string
					value any
				}{field: "image", value: item})
			}
		}
		if mask, ok := payload["mask"]; ok && mask != nil {
			items = append(items, struct {
				field string
				value any
			}{field: "mask", value: mask})
		}
	}
	if len(items) == 0 {
		return "missing_base64_uploads", "", fmt.Errorf("uploads, images, or mask is required")
	}
	imageCount := 0
	maskCount := 0
	for index, item := range items {
		encoded, field, ok := imageUploadBase64Value(item.value, item.field)
		if !ok {
			return "missing_upload_base64", fmt.Sprintf("uploads[%d]", index), fmt.Errorf("upload item must include b64_json, base64, or data")
		}
		if field == "mask" {
			maskCount++
			if maskCount > 1 {
				return "too_many_uploads", "mask", fmt.Errorf("only one mask may be uploaded")
			}
		} else if field == "image" {
			imageCount++
			if imageCount > maxImageUploadCount {
				return "too_many_uploads", "images", fmt.Errorf("at most %d images may be uploaded", maxImageUploadCount)
			}
		} else {
			return "invalid_upload_field", fmt.Sprintf("uploads[%d].field", index), fmt.Errorf("upload field must be image or mask")
		}
		decoded, err := decodeImageUploadBase64(encoded)
		if err != nil {
			return "invalid_upload_base64", fmt.Sprintf("uploads[%d]", index), err
		}
		if len(decoded) > maxImageUploadFileBytes {
			return "upload_file_too_large", fmt.Sprintf("uploads[%d]", index), fmt.Errorf("uploaded image exceeds 20 MiB")
		}
		if err := validateUploadedImageBytes(decoded); err != nil {
			return "invalid_upload_image", fmt.Sprintf("uploads[%d]", index), err
		}
	}
	return "", "", nil
}

func imageUploadBase64Value(value any, fallbackField string) (string, string, bool) {
	if text, ok := value.(string); ok {
		return text, fallbackField, strings.TrimSpace(text) != ""
	}
	item, ok := value.(map[string]any)
	if !ok {
		return "", fallbackField, false
	}
	field := fallbackField
	if value, ok := item["field"].(string); ok && strings.TrimSpace(value) != "" {
		field = strings.TrimSpace(value)
	}
	for _, key := range []string{"b64_json", "base64", "data"} {
		if encoded, ok := item[key].(string); ok && strings.TrimSpace(encoded) != "" {
			return encoded, field, true
		}
	}
	return "", field, false
}

func decodeImageUploadBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 || !strings.Contains(strings.ToLower(value[:comma]), ";base64") {
			return nil, fmt.Errorf("invalid image data URL")
		}
		value = value[comma+1:]
	}
	value = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, value)
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(decoded) == 0 {
		return nil, fmt.Errorf("uploaded image base64 is invalid")
	}
	return decoded, nil
}

func validateUploadedImageBytes(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("uploaded image is empty")
	}
	detected := http.DetectContentType(value)
	switch detected {
	case "image/png", "image/jpeg", "image/webp":
		return nil
	}
	if len(value) >= 12 && string(value[:4]) == "RIFF" && string(value[8:12]) == "WEBP" {
		return nil
	}
	return fmt.Errorf("uploaded image must be PNG, JPEG, or WebP")
}

func writeImageTaskAPIError(c *gin.Context, status int, code, message, param string) {
	requestID := c.GetString(common.RequestIdKey)
	c.JSON(status, dto.ImageTaskAPIErrorResponse{Error: dto.ImageTaskAPIError{
		Type: "invalid_request_error", Code: code, Message: message, Param: param, RequestID: requestID,
	}})
}
