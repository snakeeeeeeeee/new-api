package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
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

type imageTaskAPIProblem struct {
	status  int
	code    string
	message string
	param   string
}

type multipartImageTaskPreparation struct {
	request           dto.ImageTaskCreateRequest
	fingerprint       string
	uploadContentType string
	uploadBody        []byte
	imageCount        int
	hasMask           bool
}

type multipartImageTaskFingerprint struct {
	Transport string                     `json:"transport"`
	Request   dto.ImageTaskCreateRequest `json:"request"`
}

type imagePrefixWriter struct {
	data []byte
}

func (w *imagePrefixWriter) Write(value []byte) (int, error) {
	remaining := 512 - len(w.data)
	if remaining > len(value) {
		remaining = len(value)
	}
	if remaining > 0 {
		w.data = append(w.data, value[:remaining]...)
	}
	return len(value), nil
}

func PrepareImageTaskRequest(c *gin.Context) {
	idempotencyKey, problem := imageTaskIdempotencyKey(c)
	if problem != nil {
		abortImageTaskAPIProblem(c, problem)
		return
	}

	var request dto.ImageTaskCreateRequest
	var canonical []byte
	var fingerprint string
	var err error
	if isMultipartImageTaskRequest(c.GetHeader("Content-Type")) {
		preparation, parseProblem := parseMultipartImageTaskRequest(c)
		if parseProblem != nil {
			abortImageTaskAPIProblem(c, parseProblem)
			return
		}
		fingerprint = preparation.fingerprint
		if replayImageTaskRequest(c, idempotencyKey, fingerprint) {
			return
		}
		upload, uploadProblem := forwardImageTaskUpload(c.Request.Context(), preparation.uploadContentType, "/v1/image/uploads", preparation.uploadBody)
		if uploadProblem != nil {
			abortImageTaskAPIProblem(c, uploadProblem)
			return
		}
		request, problem = applyMultipartImageTaskUpload(preparation, upload)
		if problem != nil {
			abortImageTaskAPIProblem(c, problem)
			return
		}
		canonical, err = common.Marshal(request)
		if err != nil {
			abortImageTaskAPIProblem(c, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to normalize request"})
			return
		}
	} else {
		var parseProblem *imageTaskAPIProblem
		request, canonical, fingerprint, parseProblem = parseJSONImageTaskRequest(c)
		if parseProblem != nil {
			abortImageTaskAPIProblem(c, parseProblem)
			return
		}
		if replayImageTaskRequest(c, idempotencyKey, fingerprint) {
			return
		}
	}
	if len(canonical) == 0 {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to normalize request", "")
		c.Abort()
		return
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

func imageTaskIdempotencyKey(c *gin.Context) (string, *imageTaskAPIProblem) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) > maxImageTaskIdempotencyKeyLength {
		return "", &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_idempotency_key", message: "Idempotency-Key must not exceed 128 characters", param: "Idempotency-Key"}
	}
	return key, nil
}

func parseJSONImageTaskRequest(c *gin.Context) (dto.ImageTaskCreateRequest, []byte, string, *imageTaskAPIProblem) {
	var request dto.ImageTaskCreateRequest
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return request, nil, "", &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "Invalid JSON request body"}
	}
	rawBody, err := storage.Bytes()
	if err != nil || common.UnmarshalStrict(rawBody, &request) != nil {
		return request, nil, "", &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "Invalid JSON request body"}
	}
	if param, message := validateImageTaskCreateRequest(&request); message != "" {
		return request, nil, "", &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: message, param: param}
	}
	canonical, err := common.Marshal(request)
	if err != nil {
		return request, nil, "", &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to normalize request"}
	}
	return request, canonical, imageTaskRequestFingerprint(canonical), nil
}

func isMultipartImageTaskRequest(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return mediaType == "multipart/form-data"
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data")
}

func parseMultipartImageTaskRequest(c *gin.Context) (multipartImageTaskPreparation, *imageTaskAPIProblem) {
	var preparation multipartImageTaskPreparation
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_multipart_body"
		if common.IsRequestBodyTooLargeError(err) {
			status = http.StatusRequestEntityTooLarge
			code = "upload_request_too_large"
		}
		return preparation, &imageTaskAPIProblem{status: status, code: code, message: "Invalid multipart request body"}
	}
	if storage.Size() > maxImageUploadRequestBytes {
		return preparation, &imageTaskAPIProblem{status: http.StatusRequestEntityTooLarge, code: "upload_request_too_large", message: "Upload request exceeds 100 MiB"}
	}
	body, err := storage.Bytes()
	if err != nil {
		return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_body", message: "Failed to read multipart request body"}
	}
	_, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || params["boundary"] == "" {
		return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_body", message: "request body must be multipart/form-data"}
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := make(map[string]string)
	allowedFields := map[string]bool{
		"model": true, "operation": true, "prompt": true, "n": true, "size": true,
		"quality": true, "output_format": true, "output_compression": true,
		"background": true, "client_reference_id": true, "metadata": true,
	}
	var uploadBody bytes.Buffer
	uploadWriter := multipart.NewWriter(&uploadBody)
	imageHashes := make([]string, 0, maxImageUploadCount)
	maskHash := ""

	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_body", message: "Invalid multipart request body"}
		}
		field := part.FormName()
		filename := part.FileName()
		if filename == "" {
			if !allowedFields[field] {
				_ = part.Close()
				if field == "image" || field == "mask" {
					return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_upload_field", message: field + " must be an uploaded file", param: field}
				}
				return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_field", message: "Unknown multipart field", param: field}
			}
			if _, exists := fields[field]; exists {
				_ = part.Close()
				return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "duplicate_multipart_field", message: "Multipart field must not be repeated", param: field}
			}
			value, readErr := io.ReadAll(part)
			_ = part.Close()
			if readErr != nil {
				return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_body", message: "Failed to read multipart field", param: field}
			}
			fields[field] = string(value)
			continue
		}

		if field != "image" && field != "mask" {
			_ = part.Close()
			return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_upload_field", message: "File field must be image or mask", param: field}
		}
		if field == "image" {
			preparation.imageCount++
			if preparation.imageCount > maxImageUploadCount {
				_ = part.Close()
				return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "too_many_uploads", message: fmt.Sprintf("At most %d images may be uploaded", maxImageUploadCount), param: "image"}
			}
		} else {
			if preparation.hasMask {
				_ = part.Close()
				return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "too_many_uploads", message: "Only one mask may be uploaded", param: "mask"}
			}
			preparation.hasMask = true
		}

		uploadHeader := make(textproto.MIMEHeader)
		uploadHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": field, "filename": filename}))
		if fileContentType := strings.TrimSpace(part.Header.Get("Content-Type")); fileContentType != "" {
			uploadHeader.Set("Content-Type", fileContentType)
		} else {
			uploadHeader.Set("Content-Type", "application/octet-stream")
		}
		uploadPart, createErr := uploadWriter.CreatePart(uploadHeader)
		if createErr != nil {
			_ = part.Close()
			return preparation, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to prepare image upload"}
		}
		hasher := sha256.New()
		prefix := &imagePrefixWriter{}
		written, copyErr := io.Copy(io.MultiWriter(uploadPart, hasher, prefix), io.LimitReader(part, maxImageUploadFileBytes+1))
		_ = part.Close()
		if copyErr != nil {
			return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_multipart_body", message: "Failed to read uploaded image", param: field}
		}
		if written > maxImageUploadFileBytes {
			return preparation, &imageTaskAPIProblem{status: http.StatusRequestEntityTooLarge, code: "upload_file_too_large", message: "Uploaded image exceeds 20 MiB", param: field}
		}
		if imageErr := validateUploadedImageBytes(prefix.data); imageErr != nil {
			return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_upload_image", message: imageErr.Error(), param: field}
		}
		digest := hex.EncodeToString(hasher.Sum(nil))
		if field == "image" {
			imageHashes = append(imageHashes, digest)
		} else {
			maskHash = digest
		}
	}
	if preparation.imageCount == 0 {
		return preparation, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "missing_upload_file", message: "At least one image file is required", param: "image"}
	}
	if err := uploadWriter.Close(); err != nil {
		return preparation, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to prepare image upload"}
	}

	request, fieldProblem := multipartImageTaskRequestFromFields(fields, imageHashes, maskHash)
	if fieldProblem != nil {
		return preparation, fieldProblem
	}
	canonicalFingerprint, err := common.Marshal(multipartImageTaskFingerprint{Transport: "multipart", Request: request})
	if err != nil {
		return preparation, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to normalize request"}
	}
	preparation.request = request
	preparation.fingerprint = imageTaskRequestFingerprint(canonicalFingerprint)
	preparation.uploadContentType = uploadWriter.FormDataContentType()
	preparation.uploadBody = uploadBody.Bytes()
	return preparation, nil
}

func multipartImageTaskRequestFromFields(fields map[string]string, imageHashes []string, maskHash string) (dto.ImageTaskCreateRequest, *imageTaskAPIProblem) {
	request := dto.ImageTaskCreateRequest{
		Model:     fields["model"],
		Operation: "edit",
		Input:     dto.ImageTaskInputRequest{Prompt: fields["prompt"]},
	}
	if operation, exists := fields["operation"]; exists {
		request.Operation = strings.ToLower(strings.TrimSpace(operation))
		if request.Operation != "edit" {
			return request, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "operation must be edit for multipart requests", param: "operation"}
		}
	}
	for _, digest := range imageHashes {
		request.Input.Images = append(request.Input.Images, dto.ImageTaskSource{URL: "https://multipart.invalid/image/" + digest})
	}
	if maskHash != "" {
		request.Input.Mask = &dto.ImageTaskSource{URL: "https://multipart.invalid/mask/" + maskHash}
	}
	request.ClientReferenceID = fields["client_reference_id"]
	if value, exists := fields["n"]; exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return request, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "n must be an integer", param: "n"}
		}
		request.Output.Count = &parsed
	}
	if value, exists := fields["size"]; exists {
		value = strings.TrimSpace(value)
		request.Output.Size = &value
	}
	if value, exists := fields["quality"]; exists {
		value = strings.TrimSpace(value)
		request.Output.Quality = &value
	}
	if value, exists := fields["output_format"]; exists {
		value = strings.TrimSpace(value)
		request.Output.Format = &value
	}
	if value, exists := fields["output_compression"]; exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return request, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "output_compression must be an integer", param: "output_compression"}
		}
		request.Output.Compression = &parsed
	}
	if value, exists := fields["background"]; exists {
		value = strings.TrimSpace(value)
		request.Output.Background = &value
	}
	if value, exists := fields["metadata"]; exists {
		if strings.TrimSpace(value) == "" || !strings.HasPrefix(strings.TrimSpace(value), "{") || common.UnmarshalStrict([]byte(value), &request.Metadata) != nil || request.Metadata == nil {
			return request, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "metadata must be a JSON object", param: "metadata"}
		}
	}
	if param, message := validateImageTaskCreateRequest(&request); message != "" {
		return request, &imageTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: message, param: multipartImageTaskParam(param)}
	}
	return request, nil
}

func multipartImageTaskParam(param string) string {
	switch param {
	case "input.prompt":
		return "prompt"
	case "input.images":
		return "image"
	case "output.count":
		return "n"
	case "output.compression":
		return "output_compression"
	default:
		return param
	}
}

func applyMultipartImageTaskUpload(preparation multipartImageTaskPreparation, upload dto.ImageUploadListResponse) (dto.ImageTaskCreateRequest, *imageTaskAPIProblem) {
	request := preparation.request
	if len(upload.Images) != preparation.imageCount || (preparation.hasMask && upload.Mask == nil) || (!preparation.hasMask && upload.Mask != nil) {
		return request, &imageTaskAPIProblem{status: http.StatusBadGateway, code: "invalid_upload_response", message: "Image upload service returned incomplete inputs"}
	}
	request.Input.Images = make([]dto.ImageTaskSource, 0, len(upload.Images))
	for _, imageURL := range upload.Images {
		request.Input.Images = append(request.Input.Images, dto.ImageTaskSource{URL: imageURL})
	}
	request.Input.Mask = nil
	if upload.Mask != nil {
		request.Input.Mask = &dto.ImageTaskSource{URL: *upload.Mask}
	}
	if param, message := validateImageTaskCreateRequest(&request); message != "" {
		return request, &imageTaskAPIProblem{status: http.StatusBadGateway, code: "invalid_upload_response", message: message, param: multipartImageTaskParam(param)}
	}
	return request, nil
}

func imageTaskRequestFingerprint(canonical []byte) string {
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func replayImageTaskRequest(c *gin.Context, idempotencyKey, fingerprint string) bool {
	if idempotencyKey == "" {
		return false
	}
	existing, exists, lookupErr := model.GetImageTaskRequestByIdempotencyKey(c.GetInt("id"), idempotencyKey)
	if lookupErr != nil {
		abortImageTaskAPIProblem(c, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to resolve Idempotency-Key", param: "Idempotency-Key"})
		return true
	}
	if !exists {
		return false
	}
	if existing.RequestFingerprint != fingerprint {
		abortImageTaskAPIProblem(c, &imageTaskAPIProblem{status: http.StatusConflict, code: "idempotency_key_conflict", message: "Idempotency-Key was already used with a different request", param: "Idempotency-Key"})
		return true
	}
	task, taskExists, taskErr := model.GetPublicImageTask(c.GetInt("id"), existing.TaskID, imageHandleTaskPlatform())
	if taskErr != nil || !taskExists {
		abortImageTaskAPIProblem(c, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Idempotent task could not be loaded"})
		return true
	}
	publicTask, buildErr := service.BuildPublicImageTask(task)
	if buildErr != nil {
		abortImageTaskAPIProblem(c, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to build task response"})
		return true
	}
	c.Header("Idempotent-Replayed", "true")
	c.Header("Location", "/v1/image/tasks/"+task.TaskID)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, publicTask)
	c.Abort()
	return true
}

func abortImageTaskAPIProblem(c *gin.Context, problem *imageTaskAPIProblem) {
	writeImageTaskAPIError(c, problem.status, problem.code, problem.message, problem.param)
	c.Abort()
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
	result, problem := forwardImageTaskUpload(c.Request.Context(), c.GetHeader("Content-Type"), path, body)
	if problem != nil {
		writeImageTaskAPIError(c, problem.status, problem.code, problem.message, problem.param)
		return
	}
	c.JSON(http.StatusOK, result)
}

func forwardImageTaskUpload(ctx context.Context, contentType, path string, body []byte) (dto.ImageUploadListResponse, *imageTaskAPIProblem) {
	var result dto.ImageUploadListResponse
	if err := service.ValidateImageHandleSubmitConfig(); err != nil {
		return result, &imageTaskAPIProblem{status: http.StatusServiceUnavailable, code: "image_upload_unavailable", message: "Image upload service is not configured"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(service.GetImageHandleSubmitBaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return result, &imageTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to create upload request"}
	}
	request.Header.Set("Authorization", "Bearer "+service.GetImageHandleSubmitAPIKey())
	request.Header.Set("Content-Type", contentType)
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return result, &imageTaskAPIProblem{status: http.StatusBadGateway, code: "image_upload_failed", message: "Image upload service is unavailable"}
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var upstream dto.ImageTaskAPIErrorResponse
		if common.Unmarshal(responseBody, &upstream) == nil && upstream.Error.Code != "" {
			return result, &imageTaskAPIProblem{status: response.StatusCode, code: upstream.Error.Code, message: upstream.Error.Message, param: upstream.Error.Param}
		}
		return result, &imageTaskAPIProblem{status: response.StatusCode, code: "image_upload_failed", message: "Image upload failed"}
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
		return result, &imageTaskAPIProblem{status: http.StatusBadGateway, code: "invalid_upload_response", message: "Image upload service returned an invalid response"}
	}
	result = dto.ImageUploadListResponse{Object: "image.upload.list", Images: upstream.Images, Mask: upstream.Mask}
	for _, upload := range upstream.Uploads {
		result.Data = append(result.Data, dto.ImageUploadPublic{
			ID: upload.ID, Object: "image.upload", Field: upload.Field, URL: upload.URL, MimeType: upload.MimeType,
			SizeBytes: upload.Bytes, Width: upload.Width, Height: upload.Height, Format: upload.Format, Temporary: true,
		})
	}
	return result, nil
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
