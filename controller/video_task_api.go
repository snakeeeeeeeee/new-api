package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxVideoTaskIdempotencyKeyLength = 128
const maxVideoTaskRequestBytes = 256 << 10

type videoTaskAPIProblem struct {
	status  int
	code    string
	message string
	param   string
}

func PrepareVideoTaskRequest(c *gin.Context) {
	mediaType, _, mediaErr := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{
			status: http.StatusUnsupportedMediaType, code: "unsupported_media_type",
			message: "Content-Type must be application/json", param: "Content-Type",
		})
		return
	}
	if c.Request.ContentLength > maxVideoTaskRequestBytes {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{
			status: http.StatusRequestEntityTooLarge, code: "video_task_request_too_large",
			message: "Video task request must not exceed 256 KiB",
		})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVideoTaskRequestBytes)
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > maxVideoTaskIdempotencyKeyLength {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{
			status: http.StatusBadRequest, code: "invalid_idempotency_key",
			message: "Idempotency-Key must not exceed 128 characters", param: "Idempotency-Key",
		})
		return
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_request"
		message := "Invalid JSON request body"
		if common.IsRequestBodyTooLargeError(err) || strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			status = http.StatusRequestEntityTooLarge
			code = "video_task_request_too_large"
			message = "Video task request must not exceed 256 KiB"
		}
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: status, code: code, message: message})
		return
	}
	if storage.Size() > maxVideoTaskRequestBytes {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{
			status: http.StatusRequestEntityTooLarge, code: "video_task_request_too_large",
			message: "Video task request must not exceed 256 KiB",
		})
		return
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "Invalid JSON request body"})
		return
	}
	var request dto.VideoTaskCreateRequest
	if err := common.UnmarshalStrict(rawBody, &request); err != nil {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: "Invalid JSON request body"})
		return
	}
	normalizeVideoTaskCreateRequest(&request)
	if param, message := validateVideoTaskCreateRequest(&request); message != "" {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: http.StatusBadRequest, code: "invalid_request", message: message, param: param})
		return
	}
	canonical, err := common.Marshal(request)
	if err != nil {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to normalize request"})
		return
	}
	fingerprintBytes := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	if replayVideoTaskRequest(c, idempotencyKey, fingerprint) {
		return
	}

	snapshot, err := videoTaskRequestSnapshot(request)
	if err != nil {
		abortVideoTaskAPIProblem(c, &videoTaskAPIProblem{status: http.StatusInternalServerError, code: "server_error", message: "Failed to sanitize request"})
		return
	}
	c.Set(relaycommon.VideoTaskPublicRequestContextKey, request)
	c.Set(relaycommon.VideoTaskPublicRequestJSONContextKey, snapshot)
	c.Set(relaycommon.VideoTaskFingerprintContextKey, fingerprint)
	c.Set(relaycommon.VideoTaskIdempotencyKeyContextKey, idempotencyKey)
	common.CleanupBodyStorage(c)
	c.Set(common.KeyRequestBody, nil)
	c.Request.Body = io.NopCloser(bytes.NewReader(canonical))
	c.Request.ContentLength = int64(len(canonical))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Next()
}

func normalizeVideoTaskCreateRequest(request *dto.VideoTaskCreateRequest) {
	request.Model = strings.TrimSpace(request.Model)
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	request.Input.Prompt = strings.TrimSpace(request.Input.Prompt)
	request.Input.ReferenceMode = strings.ToLower(strings.TrimSpace(request.Input.ReferenceMode))
	request.ClientReferenceID = strings.TrimSpace(request.ClientReferenceID)
	normalizeVideoTaskSource(request.Input.Image)
	normalizeVideoTaskSource(request.Input.Video)
	for i := range request.Input.ReferenceImages {
		normalizeVideoTaskSource(&request.Input.ReferenceImages[i])
	}
	for i := range request.Input.ReferenceVideos {
		normalizeVideoTaskSource(&request.Input.ReferenceVideos[i])
	}
	for i := range request.Input.ReferenceAudios {
		normalizeVideoTaskSource(&request.Input.ReferenceAudios[i])
	}
	if request.Output.AspectRatio != nil {
		value := strings.TrimSpace(*request.Output.AspectRatio)
		request.Output.AspectRatio = &value
	}
	if request.Output.Resolution != nil {
		value := strings.ToLower(strings.TrimSpace(*request.Output.Resolution))
		request.Output.Resolution = &value
	}
	if len(request.ProviderOptions) > 0 {
		normalized := make(map[string]map[string]any, len(request.ProviderOptions))
		for namespace, options := range request.ProviderOptions {
			key := strings.ToLower(strings.TrimSpace(namespace))
			if existing, ok := normalized[key]; ok && existing != nil {
				// Preserve the duplicate for validation instead of silently choosing one.
				normalized[""] = nil
				continue
			}
			normalized[key] = options
		}
		request.ProviderOptions = normalized
	}
}

func normalizeVideoTaskSource(source *dto.VideoTaskSource) {
	if source == nil {
		return
	}
	source.URL = strings.TrimSpace(source.URL)
	source.Provider = strings.ToLower(strings.TrimSpace(source.Provider))
	source.FileID = strings.TrimSpace(source.FileID)
	source.Name = strings.TrimSpace(source.Name)
}

func validateVideoTaskCreateRequest(request *dto.VideoTaskCreateRequest) (string, string) {
	if request.Model == "" {
		return "model", "model is required"
	}
	if len(request.ClientReferenceID) > 191 {
		return "client_reference_id", "client_reference_id must not exceed 191 characters"
	}
	switch request.Operation {
	case "generation", "edit", "extension", "remix":
	default:
		return "operation", "operation must be generation, edit, extension, or remix"
	}
	if request.Input.Prompt == "" {
		return "input.prompt", "input.prompt is required"
	}
	referenceMode := request.Input.ReferenceMode
	if referenceMode == "" {
		referenceMode = "frame"
	}
	if referenceMode != "frame" && referenceMode != "media" {
		return "input.reference_mode", "input.reference_mode must be frame or media"
	}
	if request.Input.Image != nil {
		if message := validateVideoReferenceSource(request.Input.Image); message != "" {
			return "input.image", message
		}
	}
	if request.Input.ReferenceImages != nil && len(request.Input.ReferenceImages) == 0 {
		return "input.reference_images", "input.reference_images must contain at least one image when provided"
	}
	for i := range request.Input.ReferenceImages {
		if message := validateVideoReferenceSource(&request.Input.ReferenceImages[i]); message != "" {
			return fmt.Sprintf("input.reference_images[%d]", i), message
		}
	}
	if request.Input.ReferenceVideos != nil && len(request.Input.ReferenceVideos) == 0 {
		return "input.reference_videos", "input.reference_videos must contain at least one video when provided"
	}
	for i := range request.Input.ReferenceVideos {
		if message := validateVideoReferenceSource(&request.Input.ReferenceVideos[i]); message != "" {
			return fmt.Sprintf("input.reference_videos[%d]", i), message
		}
	}
	if request.Input.ReferenceAudios != nil && len(request.Input.ReferenceAudios) == 0 {
		return "input.reference_audios", "input.reference_audios must contain at least one audio file when provided"
	}
	for i := range request.Input.ReferenceAudios {
		if message := validateVideoReferenceSource(&request.Input.ReferenceAudios[i]); message != "" {
			return fmt.Sprintf("input.reference_audios[%d]", i), message
		}
	}
	imageCount := len(request.Input.ReferenceImages)
	if request.Input.Image != nil {
		imageCount++
	}
	if referenceMode == "frame" {
		if imageCount > 2 {
			return "input.reference_images", "frame mode supports at most 2 reference images"
		}
		if len(request.Input.ReferenceVideos) > 0 || len(request.Input.ReferenceAudios) > 0 {
			return "input.reference_mode", "frame mode accepts image references only"
		}
	} else {
		if imageCount > 9 {
			return "input.reference_images", "media mode supports at most 9 reference images"
		}
		if len(request.Input.ReferenceVideos) > 3 {
			return "input.reference_videos", "media mode supports at most 3 reference videos"
		}
		if len(request.Input.ReferenceAudios) > 3 {
			return "input.reference_audios", "media mode supports at most 3 reference audio files"
		}
		if imageCount+len(request.Input.ReferenceVideos)+len(request.Input.ReferenceAudios) > 12 {
			return "input", "media mode supports at most 12 total references"
		}
	}
	names := make(map[string]struct{})
	for _, source := range videoTaskReferenceSources(request) {
		if len(source.Name) > 100 {
			return "input", "reference names must not exceed 100 characters"
		}
		if source.Name == "" {
			continue
		}
		if _, exists := names[source.Name]; exists {
			return "input", "reference names must be unique"
		}
		names[source.Name] = struct{}{}
	}
	if request.Input.Video != nil {
		if message := validateVideoTaskSource(request.Input.Video); message != "" {
			return "input.video", message
		}
	}
	if request.Operation == "generation" {
		if request.Input.Video != nil {
			return "input.video", "input.video is only valid for edit, extension, or remix"
		}
	} else {
		if request.Input.Video == nil {
			return "input.video", "input.video is required for edit, extension, and remix"
		}
	}
	for namespace, options := range request.ProviderOptions {
		if strings.TrimSpace(namespace) == "" || options == nil {
			return "provider_options", "provider_options must use non-empty provider namespaces with object values"
		}
	}
	return "", ""
}

func validateVideoTaskSource(source *dto.VideoTaskSource) string {
	if source == nil {
		return "source is required"
	}
	hasURL := source.URL != ""
	hasProviderFile := source.Provider != "" || source.FileID != ""
	if hasURL == hasProviderFile {
		return "source must contain either url or provider with file_id"
	}
	if hasProviderFile {
		if source.Provider == "" || source.FileID == "" {
			return "provider and file_id must be provided together"
		}
		return ""
	}
	if strings.HasPrefix(strings.ToLower(source.URL), "data:") {
		return ""
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "url must be an absolute HTTP(S) URL or data URL"
	}
	return ""
}

func validateVideoReferenceSource(source *dto.VideoTaskSource) string {
	if source == nil {
		return "source is required"
	}
	if source.Provider != "" || source.FileID != "" || source.URL == "" {
		return "reference source must contain exactly one HTTP(S) URL"
	}
	if len(source.URL) > 8192 {
		return "reference URL must not exceed 8192 characters"
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "reference URL must be an absolute HTTP(S) URL"
	}
	return ""
}

func videoTaskReferenceSources(request *dto.VideoTaskCreateRequest) []dto.VideoTaskSource {
	sources := make([]dto.VideoTaskSource, 0, 1+len(request.Input.ReferenceImages)+len(request.Input.ReferenceVideos)+len(request.Input.ReferenceAudios))
	if request.Input.Image != nil {
		sources = append(sources, *request.Input.Image)
	}
	sources = append(sources, request.Input.ReferenceImages...)
	sources = append(sources, request.Input.ReferenceVideos...)
	sources = append(sources, request.Input.ReferenceAudios...)
	return sources
}

func videoTaskRequestSnapshot(request dto.VideoTaskCreateRequest) ([]byte, error) {
	snapshot := request
	snapshot.Input.Image = cloneVideoTaskSource(request.Input.Image)
	snapshot.Input.Video = cloneVideoTaskSource(request.Input.Video)
	snapshot.Input.ReferenceImages = cloneVideoTaskSources(request.Input.ReferenceImages)
	snapshot.Input.ReferenceVideos = cloneVideoTaskSources(request.Input.ReferenceVideos)
	snapshot.Input.ReferenceAudios = cloneVideoTaskSources(request.Input.ReferenceAudios)
	sanitizeSource := func(source *dto.VideoTaskSource) {
		if source == nil {
			return
		}
		if source.URL != "" {
			digest := sha256.Sum256([]byte(source.URL))
			source.URL = "sha256:" + hex.EncodeToString(digest[:])
		}
		if source.FileID != "" {
			digest := sha256.Sum256([]byte(source.FileID))
			source.FileID = "sha256:" + hex.EncodeToString(digest[:])
		}
	}
	sanitizeSource(snapshot.Input.Image)
	sanitizeSource(snapshot.Input.Video)
	for i := range snapshot.Input.ReferenceImages {
		sanitizeSource(&snapshot.Input.ReferenceImages[i])
	}
	for i := range snapshot.Input.ReferenceVideos {
		sanitizeSource(&snapshot.Input.ReferenceVideos[i])
	}
	for i := range snapshot.Input.ReferenceAudios {
		sanitizeSource(&snapshot.Input.ReferenceAudios[i])
	}
	return common.Marshal(snapshot)
}

func cloneVideoTaskSource(source *dto.VideoTaskSource) *dto.VideoTaskSource {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}

func cloneVideoTaskSources(sources []dto.VideoTaskSource) []dto.VideoTaskSource {
	if sources == nil {
		return nil
	}
	return append([]dto.VideoTaskSource(nil), sources...)
}

func replayVideoTaskRequest(c *gin.Context, idempotencyKey, fingerprint string) bool {
	if idempotencyKey == "" {
		return false
	}
	existing, exists, err := model.GetVideoTaskRequestByIdempotencyKey(c.GetInt("id"), idempotencyKey)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to check idempotency key", "")
		c.Abort()
		return true
	}
	if !exists {
		return false
	}
	if existing.RequestFingerprint != fingerprint {
		writeVideoTaskAPIError(c, http.StatusConflict, "idempotency_key_conflict", "Idempotency-Key was already used with a different request", "Idempotency-Key")
		c.Abort()
		return true
	}
	task, exists, err := model.GetPublicVideoTask(c.GetInt("id"), existing.TaskID)
	if err != nil || !exists {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to replay task", "")
		c.Abort()
		return true
	}
	publicTask, err := service.BuildPublicVideoTask(task)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to replay task", "")
		c.Abort()
		return true
	}
	c.Header("Idempotent-Replayed", "true")
	c.Header("Location", "/v1/video/tasks/"+task.TaskID)
	c.Header("Retry-After", "2")
	c.JSON(http.StatusAccepted, publicTask)
	c.Abort()
	return true
}

func ListVideoTasks(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100", "limit")
		return
	}
	createdAfter, afterErr := parseOptionalUnixQuery(c, "created_after")
	createdBefore, beforeErr := parseOptionalUnixQuery(c, "created_before")
	if afterErr != nil || beforeErr != nil {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "created_after and created_before must be Unix timestamps", "")
		return
	}
	statuses, statusErr := internalImageTaskStatuses(c.Query("status"))
	if statusErr != nil {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", statusErr.Error(), "status")
		return
	}
	operation, operationErr := publicVideoTaskOperation(c.Query("operation"))
	if operationErr != nil {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", operationErr.Error(), "operation")
		return
	}
	tasks, hasMore, queryErr := model.ListPublicVideoTasks(c.GetInt("id"), model.VideoTaskListQuery{
		Statuses: statuses, Operation: operation, ClientReferenceID: strings.TrimSpace(c.Query("client_reference_id")),
		CreatedAfter: createdAfter, CreatedBefore: createdBefore, AfterTaskID: strings.TrimSpace(c.Query("after")), Limit: limit,
	})
	if queryErr != nil {
		if c.Query("after") != "" {
			writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_cursor", "after task was not found", "after")
			return
		}
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to list tasks", "")
		return
	}
	publicTasks, err := service.BuildPublicVideoTasks(tasks)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task list", "")
		return
	}
	response := dto.VideoTaskListResponse{Object: "list", Data: publicTasks, HasMore: hasMore}
	if len(publicTasks) > 0 {
		response.FirstID = publicTasks[0].ID
		response.LastID = publicTasks[len(publicTasks)-1].ID
	}
	c.JSON(http.StatusOK, response)
}

func GetVideoTask(c *gin.Context) {
	task, exists, err := model.GetPublicVideoTask(c.GetInt("id"), c.Param("task_id"))
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to load task", "")
		return
	}
	if !exists {
		writeVideoTaskAPIError(c, http.StatusNotFound, "task_not_found", "Task not found", "task_id")
		return
	}
	publicTask, err := service.BuildPublicVideoTask(task)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task response", "")
		return
	}
	c.JSON(http.StatusOK, publicTask)
}

func QueryVideoTasks(c *gin.Context) {
	var request dto.VideoTaskBatchQueryRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "Invalid JSON request body", "")
		return
	}
	if len(request.TaskIDs) == 0 || len(request.TaskIDs) > 100 {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "task_ids must contain between 1 and 100 IDs", "task_ids")
		return
	}
	tasks, err := model.GetPublicVideoTasksByIDs(c.GetInt("id"), request.TaskIDs)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to query tasks", "")
		return
	}
	publicTasks, err := service.BuildPublicVideoTasks(tasks)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task response", "")
		return
	}
	byID := make(map[string]*dto.VideoTaskPublic, len(publicTasks))
	for _, task := range publicTasks {
		byID[task.ID] = task
	}
	ordered := make([]*dto.VideoTaskPublic, 0, len(request.TaskIDs))
	missing := make([]string, 0)
	for _, taskID := range request.TaskIDs {
		if task, ok := byID[taskID]; ok {
			ordered = append(ordered, task)
		} else {
			missing = append(missing, taskID)
		}
	}
	c.JSON(http.StatusOK, dto.VideoTaskListResponse{Object: "list", Data: ordered, Missing: missing})
}

func publicVideoTaskOperation(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "generation", "edit", "extension", "remix":
		return value, nil
	default:
		return "", fmt.Errorf("operation is invalid")
	}
}

func abortVideoTaskAPIProblem(c *gin.Context, problem *videoTaskAPIProblem) {
	writeVideoTaskAPIError(c, problem.status, problem.code, problem.message, problem.param)
	c.Abort()
}

func writeVideoTaskAPIError(c *gin.Context, status int, code, message, param string) {
	c.JSON(status, dto.VideoTaskAPIErrorResponse{Error: dto.VideoTaskAPIError{
		Type: "invalid_request_error", Code: code, Message: message, Param: param,
		RequestID: c.GetString(common.RequestIdKey),
	}})
}
