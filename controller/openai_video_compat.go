package controller

import (
	"bytes"
	"crypto/sha256"
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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	openAIVideoCandidateContextKey = "openai_video_candidate"
	maxOpenAIVideoUploadBytes      = 64 << 20
)

type openAIVideoJSONRequest struct {
	Model          string           `json:"model,omitempty"`
	Prompt         string           `json:"prompt"`
	Seconds        any              `json:"seconds,omitempty"`
	Size           string           `json:"size,omitempty"`
	InputReference any              `json:"input_reference,omitempty"`
	Video          any              `json:"video,omitempty"`
	Characters     []map[string]any `json:"characters,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
	ResolutionName string           `json:"resolution_name,omitempty"`
	Preset         string           `json:"preset,omitempty"`
}

type openAIVideoCandidate struct {
	request             dto.VideoTaskCreateRequest
	compatibility       dto.OpenAIVideoCompatibilityMetadata
	fingerprint         string
	form                *multipart.Form
	singleImageFile     *multipart.FileHeader
	referenceImageFiles []*multipart.FileHeader
	referenceFileStart  int
	videoFile           *multipart.FileHeader
}

type openAIVideoCaptureState struct {
	candidate openAIVideoCandidate
	problem   *videoTaskAPIProblem
}

type openAIVideoFingerprint struct {
	Transport     string                               `json:"transport"`
	Request       dto.VideoTaskCreateRequest           `json:"request"`
	Compatibility dto.OpenAIVideoCompatibilityMetadata `json:"compatibility"`
}

func CaptureOpenAIVideoRequest(c *gin.Context) {
	candidate, problem := parseOpenAIVideoCandidate(c)
	if problem == nil && candidate.compatibility.SourceVideoID != "" {
		problem = lockOpenAIVideoSourceChannel(c, candidate.compatibility.SourceVideoID, &candidate)
	}
	if problem == nil {
		problem = sealOpenAIVideoCandidate(&candidate)
	}
	modelName := strings.TrimSpace(candidate.request.Model)
	if modelName == "" {
		var request struct {
			Model string `json:"model"`
		}
		if common.UnmarshalBodyReusable(c, &request) == nil {
			modelName = strings.TrimSpace(request.Model)
		}
	}
	if modelName == "" && c.Request.URL.Path == "/v1/videos" {
		modelName = "sora-2"
	}
	c.Set(openAIVideoCandidateContextKey, openAIVideoCaptureState{candidate: candidate, problem: problem})
	c.Set(relaycommon.OpenAIVideoModelContextKey, modelName)
	c.Next()
	if candidate.form != nil {
		_ = candidate.form.RemoveAll()
	}
}

func parseOpenAIVideoCandidate(c *gin.Context) (openAIVideoCandidate, *videoTaskAPIProblem) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil {
		return openAIVideoCandidate{}, videoCompatibilityProblem(http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json or multipart/form-data", "Content-Type")
	}
	operation := openAIVideoOperation(c)
	if operation == "" {
		return openAIVideoCandidate{}, videoCompatibilityProblem(http.StatusBadRequest, "unsupported_video_endpoint", "Unsupported OpenAI video endpoint", "")
	}
	var candidate openAIVideoCandidate
	switch mediaType {
	case "application/json":
		candidate, err = parseOpenAIVideoJSONCandidate(c, operation)
	case "multipart/form-data":
		candidate, err = parseOpenAIVideoMultipartCandidate(c, operation)
	default:
		return candidate, videoCompatibilityProblem(http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json or multipart/form-data", "Content-Type")
	}
	if err != nil {
		return candidate, videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", err.Error(), "")
	}
	if operation == "remix" {
		sourceID := strings.TrimSpace(c.Param("video_id"))
		candidate.compatibility.SourceVideoID = sourceID
		candidate.compatibility.RemixedFromVideo = sourceID
	}
	if problem := finalizeOpenAIVideoCandidate(&candidate, operation, mediaType); problem != nil {
		if candidate.form != nil {
			_ = candidate.form.RemoveAll()
		}
		return candidate, problem
	}
	return candidate, nil
}

func parseOpenAIVideoJSONCandidate(c *gin.Context, operation string) (openAIVideoCandidate, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return openAIVideoCandidate{}, fmt.Errorf("invalid JSON request body")
	}
	if storage.Size() > maxVideoTaskRequestBytes {
		return openAIVideoCandidate{}, fmt.Errorf("video request must not exceed 256 KiB")
	}
	body, err := storage.Bytes()
	if err != nil {
		return openAIVideoCandidate{}, fmt.Errorf("invalid JSON request body")
	}
	var raw openAIVideoJSONRequest
	if err := common.UnmarshalStrict(body, &raw); err != nil {
		return openAIVideoCandidate{}, fmt.Errorf("invalid JSON request body")
	}
	candidate := openAIVideoCandidate{
		request: dto.VideoTaskCreateRequest{
			Model: strings.TrimSpace(raw.Model), Operation: operation,
			Input:    dto.VideoTaskInputRequest{Prompt: strings.TrimSpace(raw.Prompt)},
			Metadata: raw.Metadata,
		},
		compatibility: dto.OpenAIVideoCompatibilityMetadata{
			Version: dto.OpenAIVideoCompatibilityVersion, Size: strings.TrimSpace(raw.Size),
			ResolutionName: strings.ToLower(strings.TrimSpace(raw.ResolutionName)), Preset: strings.ToLower(strings.TrimSpace(raw.Preset)),
		},
	}
	seconds, provided, err := parseOpenAIVideoSeconds(raw.Seconds)
	if err != nil {
		return candidate, err
	}
	if provided {
		candidate.compatibility.Seconds = seconds
	}
	if raw.InputReference != nil {
		source, sourceErr := openAIVideoImageReference(raw.InputReference)
		if sourceErr != nil {
			return candidate, sourceErr
		}
		candidate.request.Input.Image = source
		candidate.request.Input.ReferenceMode = "frame"
	}
	if raw.Video != nil {
		source, sourceID, sourceErr := openAIVideoSource(raw.Video)
		if sourceErr != nil {
			return candidate, sourceErr
		}
		candidate.request.Input.Video = source
		candidate.compatibility.SourceVideoID = sourceID
	}
	if len(raw.Characters) > 0 {
		return candidate, fmt.Errorf("characters are not supported by the selected normalized video providers")
	}
	return candidate, nil
}

func parseOpenAIVideoMultipartCandidate(c *gin.Context, operation string) (openAIVideoCandidate, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return openAIVideoCandidate{}, fmt.Errorf("invalid multipart request body")
	}
	if storage.Size() > maxImageUploadRequestBytes {
		return openAIVideoCandidate{}, fmt.Errorf("multipart request exceeds 100 MiB")
	}
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return openAIVideoCandidate{}, fmt.Errorf("invalid multipart request body")
	}
	allowedValues := map[string]bool{
		"model": true, "prompt": true, "seconds": true, "size": true,
		"resolution_name": true, "preset": true, "metadata": true,
		"input_reference": true, "input_reference[]": true,
	}
	for field, values := range form.Value {
		if !allowedValues[field] {
			_ = form.RemoveAll()
			return openAIVideoCandidate{}, fmt.Errorf("unknown multipart field %s", field)
		}
		if field != "input_reference[]" && len(values) > 1 {
			_ = form.RemoveAll()
			return openAIVideoCandidate{}, fmt.Errorf("multipart field %s must not be repeated", field)
		}
	}
	for field := range form.File {
		if field != "input_reference" && field != "input_reference[]" && field != "video" {
			_ = form.RemoveAll()
			return openAIVideoCandidate{}, fmt.Errorf("unknown multipart file field %s", field)
		}
	}
	first := func(name string) string {
		if values := form.Value[name]; len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
		return ""
	}
	candidate := openAIVideoCandidate{
		form: form,
		request: dto.VideoTaskCreateRequest{
			Model: first("model"), Operation: operation,
			Input: dto.VideoTaskInputRequest{Prompt: first("prompt")},
		},
		compatibility: dto.OpenAIVideoCompatibilityMetadata{
			Version: dto.OpenAIVideoCompatibilityVersion, Size: first("size"),
			ResolutionName: strings.ToLower(first("resolution_name")), Preset: strings.ToLower(first("preset")),
		},
	}
	if value := first("seconds"); value != "" {
		seconds, parseErr := strconv.Atoi(value)
		if parseErr != nil || seconds <= 0 {
			return candidate, fmt.Errorf("seconds must be a positive integer")
		}
		candidate.compatibility.Seconds = seconds
	}
	if value := first("metadata"); value != "" {
		if common.UnmarshalStrict([]byte(value), &candidate.request.Metadata) != nil || candidate.request.Metadata == nil {
			return candidate, fmt.Errorf("metadata must be a JSON object")
		}
	}
	if operation == "generation" {
		if files := form.File["input_reference"]; len(files) > 1 {
			return candidate, fmt.Errorf("input_reference accepts one image")
		} else if len(files) == 1 {
			candidate.singleImageFile = files[0]
		}
		if values := form.Value["input_reference"]; len(values) > 0 {
			if candidate.singleImageFile != nil {
				return candidate, fmt.Errorf("input_reference cannot contain both a file and URL")
			}
			source, sourceErr := openAIVideoImageReference(values[0])
			if sourceErr != nil {
				return candidate, sourceErr
			}
			candidate.request.Input.Image = source
		}
		for _, value := range form.Value["input_reference[]"] {
			source, sourceErr := openAIVideoImageReference(value)
			if sourceErr != nil {
				return candidate, sourceErr
			}
			candidate.request.Input.ReferenceImages = append(candidate.request.Input.ReferenceImages, *source)
		}
		candidate.referenceFileStart = len(candidate.request.Input.ReferenceImages)
		candidate.referenceImageFiles = append(candidate.referenceImageFiles, form.File["input_reference[]"]...)
	} else if files := form.File["video"]; len(files) != 0 {
		if len(files) != 1 {
			return candidate, fmt.Errorf("video accepts one uploaded file")
		}
		candidate.videoFile = files[0]
		if candidate.request.Model == "" {
			return candidate, fmt.Errorf("model is required when video is uploaded")
		}
	} else {
		return candidate, fmt.Errorf("video file is required")
	}
	return candidate, nil
}

func finalizeOpenAIVideoCandidate(candidate *openAIVideoCandidate, operation, transport string) *videoTaskAPIProblem {
	if candidate == nil {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", "Invalid video request", "")
	}
	request := &candidate.request
	request.Model = strings.TrimSpace(request.Model)
	request.Input.Prompt = strings.TrimSpace(request.Input.Prompt)
	if request.Input.Prompt == "" {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", "prompt is required", "prompt")
	}
	if candidate.compatibility.Preset != "" && candidate.compatibility.Preset != "normal" {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_parameter", "preset must be normal when provided", "preset")
	}
	if operation == "generation" && request.Model == "" {
		request.Model = "sora-2"
	}
	if operation == "remix" {
		sourceID := strings.TrimSpace(candidate.compatibility.SourceVideoID)
		if sourceID == "" {
			sourceID = strings.TrimSpace(candidate.compatibility.RemixedFromVideo)
		}
		candidate.compatibility.SourceVideoID = sourceID
		candidate.compatibility.RemixedFromVideo = sourceID
	}
	if candidate.singleImageFile != nil {
		if request.Input.Image != nil || len(request.Input.ReferenceImages) > 0 || len(candidate.referenceImageFiles) > 0 {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_parameter", "input_reference and input_reference[] cannot be combined", "input_reference")
		}
		digest, err := openAIVideoFileDigest(candidate.singleImageFile, maxImageUploadFileBytes)
		if err != nil {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_image", err.Error(), "input_reference")
		}
		request.Input.Image = &dto.VideoTaskSource{URL: "https://multipart.invalid/image/" + digest}
	}
	for _, file := range candidate.referenceImageFiles {
		digest, err := openAIVideoFileDigest(file, maxImageUploadFileBytes)
		if err != nil {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_image", err.Error(), "input_reference[]")
		}
		request.Input.ReferenceImages = append(request.Input.ReferenceImages, dto.VideoTaskSource{URL: "https://multipart.invalid/image/" + digest})
	}
	if request.Input.Image != nil {
		request.Input.ReferenceMode = "frame"
	}
	if len(request.Input.ReferenceImages) > 0 {
		if request.Input.Image != nil {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_parameter", "input_reference and input_reference[] cannot be combined", "input_reference")
		}
		request.Input.ReferenceMode = "media"
	}
	if candidate.videoFile != nil {
		digest, err := openAIVideoFileDigest(candidate.videoFile, maxOpenAIVideoUploadBytes)
		if err != nil {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_video", err.Error(), "video")
		}
		request.Input.Video = &dto.VideoTaskSource{URL: "https://multipart.invalid/video/" + digest}
	}
	if operation == "generation" {
		seconds := candidate.compatibility.Seconds
		if seconds == 0 {
			seconds = 4
		}
		candidate.compatibility.Seconds = seconds
		request.Output.Duration = &seconds
		size := strings.TrimSpace(candidate.compatibility.Size)
		if size == "" {
			size = "720x1280"
		}
		aspectRatio, err := openAIVideoSizeAspectRatio(size)
		if err != nil {
			return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_size", err.Error(), "size")
		}
		candidate.compatibility.Size = size
		request.Output.AspectRatio = &aspectRatio
	} else if operation == "extension" {
		seconds := candidate.compatibility.Seconds
		if seconds == 0 {
			seconds = 4
		}
		candidate.compatibility.Seconds = seconds
		request.Output.Duration = &seconds
	} else if candidate.compatibility.Seconds > 0 || candidate.compatibility.Size != "" {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_parameter", "seconds and size are not supported for video edits or remixes", "")
	}
	if candidate.compatibility.SourceVideoID == "" && operation != "generation" && candidate.videoFile == nil && request.Input.Video == nil {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_video_parameter", "video is required", "video")
	}
	return nil
}

func sealOpenAIVideoCandidate(candidate *openAIVideoCandidate) *videoTaskAPIProblem {
	if candidate == nil {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", "Invalid video request", "")
	}
	normalizeVideoTaskCreateRequest(&candidate.request)
	if param, message := validateVideoTaskCreateRequest(&candidate.request); message != "" {
		return videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", message, openAIVideoParam(param))
	}
	transport := "json"
	if candidate.form != nil {
		transport = "multipart"
	}
	canonical, err := common.Marshal(openAIVideoFingerprint{
		Transport: transport, Request: candidate.request, Compatibility: candidate.compatibility,
	})
	if err != nil {
		return videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to normalize video request", "")
	}
	digest := sha256.Sum256(canonical)
	candidate.fingerprint = hex.EncodeToString(digest[:])
	return nil
}

func lockOpenAIVideoSourceChannel(c *gin.Context, sourceID string, candidate *openAIVideoCandidate) *videoTaskAPIProblem {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), strings.TrimSpace(sourceID))
	if err != nil {
		return videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to load source video", "video")
	}
	if !exists || task == nil || constant.TaskActionAssetType(task.Action) != constant.TaskAssetTypeVideo {
		return videoCompatibilityProblem(http.StatusNotFound, "video_not_found", "Source video was not found", "video")
	}
	if task.Status != model.TaskStatusSuccess {
		return videoCompatibilityProblem(http.StatusConflict, "video_not_completed", "Source video must be completed", "video")
	}
	modelName := strings.TrimSpace(task.Properties.OriginModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(task.Properties.UpstreamModelName)
	}
	if candidate.request.Model != "" && modelName != "" && candidate.request.Model != modelName {
		return videoCompatibilityProblem(http.StatusBadRequest, "video_model_conflict", "model must match the source video model", "model")
	}
	if modelName == "" {
		return videoCompatibilityProblem(http.StatusBadGateway, "source_video_model_unavailable", "Source video model is unavailable", "video")
	}
	candidate.request.Model = modelName
	assets, assetErr := model.GetUserAssetsByTaskIDs(task.UserId, []string{task.TaskID})
	if assetErr != nil {
		return videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to load source video asset", "video")
	}
	sourceURL := ""
	for _, asset := range assets {
		if asset.AssetType == model.AssetTypeVideo && absoluteHTTPURL(asset.URL) {
			sourceURL = asset.URL
			break
		}
	}
	if sourceURL == "" && absoluteHTTPURL(task.GetResultURL()) {
		sourceURL = task.GetResultURL()
	}
	if sourceURL == "" {
		return videoCompatibilityProblem(http.StatusConflict, "source_video_unavailable", "Source video does not expose a reusable URL", "video")
	}
	candidate.request.Input.Video = &dto.VideoTaskSource{URL: sourceURL}
	channelID := strconv.Itoa(task.ChannelId)
	if existing, ok := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId); ok && fmt.Sprint(existing) != channelID {
		return videoCompatibilityProblem(http.StatusForbidden, "source_video_channel_conflict", "Source video channel is not allowed by this token", "video")
	}
	common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, channelID)
	return nil
}

func openAIVideoOperation(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	path := c.Request.URL.Path
	switch path {
	case "/v1/videos":
		return "generation"
	case "/v1/videos/edits":
		return "edit"
	case "/v1/videos/extensions":
		return "extension"
	}
	if strings.HasSuffix(path, "/remix") {
		return "remix"
	}
	return ""
}

func openAIVideoParam(param string) string {
	switch param {
	case "input.prompt":
		return "prompt"
	case "input.image", "input.reference_images":
		return "input_reference"
	case "input.video":
		return "video"
	case "output.duration":
		return "seconds"
	case "output.aspect_ratio":
		return "size"
	default:
		return param
	}
}

func openAIVideoFileDigest(header *multipart.FileHeader, limit int64) (string, error) {
	if header == nil {
		return "", fmt.Errorf("uploaded file is missing")
	}
	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file")
	}
	defer file.Close()
	hasher := sha256.New()
	written, err := io.Copy(hasher, io.LimitReader(file, limit+1))
	if err != nil {
		return "", fmt.Errorf("failed to read uploaded file")
	}
	if written == 0 {
		return "", fmt.Errorf("uploaded file is empty")
	}
	if written > limit {
		return "", fmt.Errorf("uploaded file exceeds %d MiB", limit>>20)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func openAIVideoSizeAspectRatio(size string) (string, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return "", fmt.Errorf("size must use WIDTHxHEIGHT format")
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return "", fmt.Errorf("size must contain valid positive dimensions")
	}
	divisor := width
	other := height
	for other != 0 {
		divisor, other = other, divisor%other
	}
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor), nil
}

func parseOpenAIVideoSeconds(value any) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || parsed <= 0 {
			return 0, true, fmt.Errorf("seconds must be a positive integer")
		}
		return parsed, true, nil
	case float64:
		parsed := int(typed)
		if typed != float64(parsed) || parsed <= 0 {
			return 0, true, fmt.Errorf("seconds must be a positive integer")
		}
		return parsed, true, nil
	default:
		return 0, true, fmt.Errorf("seconds must be a positive integer")
	}
}

func openAIVideoImageReference(value any) (*dto.VideoTaskSource, error) {
	switch typed := value.(type) {
	case string:
		if !absoluteHTTPURL(typed) {
			return nil, fmt.Errorf("input_reference must be an absolute HTTP(S) URL")
		}
		return &dto.VideoTaskSource{URL: strings.TrimSpace(typed)}, nil
	case map[string]any:
		if fileID, _ := typed["file_id"].(string); strings.TrimSpace(fileID) != "" {
			return nil, fmt.Errorf("input_reference.file_id is not supported by the selected normalized video providers")
		}
		imageURL := typed["image_url"]
		if imageMap, ok := imageURL.(map[string]any); ok {
			imageURL = imageMap["url"]
		}
		if rawURL, _ := imageURL.(string); absoluteHTTPURL(rawURL) {
			return &dto.VideoTaskSource{URL: strings.TrimSpace(rawURL)}, nil
		}
	}
	return nil, fmt.Errorf("input_reference must contain image_url or file_id")
}

func openAIVideoSource(value any) (*dto.VideoTaskSource, string, error) {
	switch typed := value.(type) {
	case string:
		if absoluteHTTPURL(typed) {
			return &dto.VideoTaskSource{URL: strings.TrimSpace(typed)}, "", nil
		}
		if strings.TrimSpace(typed) != "" {
			return nil, strings.TrimSpace(typed), nil
		}
	case map[string]any:
		if id, _ := typed["id"].(string); strings.TrimSpace(id) != "" {
			return nil, strings.TrimSpace(id), nil
		}
		if rawURL, _ := typed["url"].(string); absoluteHTTPURL(rawURL) {
			return &dto.VideoTaskSource{URL: strings.TrimSpace(rawURL)}, "", nil
		}
		if fileID, _ := typed["file_id"].(string); strings.TrimSpace(fileID) != "" {
			return nil, "", fmt.Errorf("video.file_id is not supported; upload the video or use a video id")
		}
	}
	return nil, "", fmt.Errorf("video must contain id or an absolute HTTP(S) URL")
}

func absoluteHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.User == nil && parsed.Hostname() != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func videoCompatibilityProblem(status int, code, message, param string) *videoTaskAPIProblem {
	return &videoTaskAPIProblem{status: status, code: code, message: message, param: param}
}

func PrepareOpenAIVideoCompatibility(c *gin.Context) {
	value, exists := c.Get(openAIVideoCandidateContextKey)
	state, ok := value.(openAIVideoCaptureState)
	if !exists || !ok {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", "OpenAI video request context is missing", ""))
		return
	}
	adaptor := relay.GetTaskAdaptor(relay.GetTaskPlatform(c))
	compatibilityAdaptor, supported := adaptor.(channel.OpenAIVideoCompatibilityAdaptor)
	if !supported {
		c.Next()
		return
	}
	if state.problem != nil {
		abortVideoTaskAPIProblem(c, state.problem)
		return
	}
	candidate := state.candidate
	capabilities := compatibilityAdaptor.OpenAIVideoCompatibility()
	if !capabilities.Supports(candidate.request.Operation) {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusBadRequest, "unsupported_video_capability", "Selected provider does not support this OpenAI video operation", ""))
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > maxVideoTaskIdempotencyKeyLength {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must not exceed 128 characters", "Idempotency-Key"))
		return
	}
	c.Set(relaycommon.OpenAIVideoCompatibilityContextKey, candidate.compatibility)
	if replayVideoTaskRequest(c, idempotencyKey, candidate.fingerprint) {
		return
	}
	if problem := materializeOpenAIVideoUploads(c, &candidate); problem != nil {
		abortVideoTaskAPIProblem(c, problem)
		return
	}
	normalizeVideoTaskCreateRequest(&candidate.request)
	if param, message := validateVideoTaskCreateRequest(&candidate.request); message != "" {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusBadRequest, "invalid_request", message, openAIVideoParam(param)))
		return
	}
	canonical, err := common.Marshal(candidate.request)
	if err != nil {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to normalize video request", ""))
		return
	}
	snapshot, err := videoTaskRequestSnapshot(candidate.request)
	if err != nil {
		abortVideoTaskAPIProblem(c, videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to sanitize video request", ""))
		return
	}
	c.Set(relaycommon.VideoTaskPublicRequestContextKey, candidate.request)
	c.Set(relaycommon.VideoTaskPublicRequestJSONContextKey, snapshot)
	c.Set(relaycommon.VideoTaskFingerprintContextKey, candidate.fingerprint)
	c.Set(relaycommon.VideoTaskIdempotencyKeyContextKey, idempotencyKey)
	c.Set(relaycommon.OpenAIVideoCompatibilityContextKey, candidate.compatibility)
	common.CleanupBodyStorage(c)
	c.Set(common.KeyRequestBody, nil)
	c.Request.Body = io.NopCloser(bytes.NewReader(canonical))
	c.Request.ContentLength = int64(len(canonical))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Next()
}

func materializeOpenAIVideoUploads(c *gin.Context, candidate *openAIVideoCandidate) *videoTaskAPIProblem {
	if candidate.singleImageFile != nil || len(candidate.referenceImageFiles) > 0 {
		files := make([]*multipart.FileHeader, 0, 1+len(candidate.referenceImageFiles))
		if candidate.singleImageFile != nil {
			files = append(files, candidate.singleImageFile)
		}
		for _, file := range candidate.referenceImageFiles {
			files = append(files, file)
		}
		uploaded, problem := uploadOpenAIVideoImages(c, files)
		if problem != nil {
			return problem
		}
		cursor := 0
		if candidate.singleImageFile != nil {
			candidate.request.Input.Image = &dto.VideoTaskSource{URL: uploaded[cursor]}
			cursor++
		}
		for index := candidate.referenceFileStart; index < len(candidate.request.Input.ReferenceImages); index++ {
			if cursor >= len(uploaded) {
				return videoCompatibilityProblem(http.StatusBadGateway, "invalid_upload_response", "Image upload service returned incomplete inputs", "input_reference[]")
			}
			candidate.request.Input.ReferenceImages[index].URL = uploaded[cursor]
			cursor++
		}
	}
	if candidate.videoFile != nil {
		videoURL, problem := uploadOpenAIVideoFile(c, candidate.videoFile)
		if problem != nil {
			return problem
		}
		candidate.request.Input.Video = &dto.VideoTaskSource{URL: videoURL}
	}
	return nil
}

func uploadOpenAIVideoImages(c *gin.Context, files []*multipart.FileHeader) ([]string, *videoTaskAPIProblem) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := range files {
		header := files[index]
		file, err := header.Open()
		if err != nil {
			return nil, videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_image", "Failed to open uploaded image", "input_reference")
		}
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "image", "filename": header.Filename}))
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		partHeader.Set("Content-Type", contentType)
		part, createErr := writer.CreatePart(partHeader)
		if createErr != nil {
			_ = file.Close()
			return nil, videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to prepare image upload", "")
		}
		written, copyErr := io.Copy(part, io.LimitReader(file, maxImageUploadFileBytes+1))
		_ = file.Close()
		if copyErr != nil || written > maxImageUploadFileBytes {
			return nil, videoCompatibilityProblem(http.StatusRequestEntityTooLarge, "upload_file_too_large", "Uploaded image exceeds 20 MiB", "input_reference")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to prepare image upload", "")
	}
	result, problem := forwardImageTaskUpload(c.Request.Context(), writer.FormDataContentType(), "/v1/image/uploads", body.Bytes())
	if problem != nil {
		return nil, videoCompatibilityProblem(problem.status, problem.code, problem.message, problem.param)
	}
	if len(result.Images) != len(files) {
		return nil, videoCompatibilityProblem(http.StatusBadGateway, "invalid_upload_response", "Image upload service returned incomplete inputs", "input_reference")
	}
	return result.Images, nil
}

func uploadOpenAIVideoFile(c *gin.Context, header *multipart.FileHeader) (string, *videoTaskAPIProblem) {
	if header == nil {
		return "", videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_video", "Uploaded video is missing", "video")
	}
	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	sessions, problem := forwardMediaUploadControl[dto.MediaUploadSessionListResponse](
		c.Request.Context(), "/internal/v1/media/uploads",
		internalMediaUploadCreateRequest{OwnerID: strconv.Itoa(c.GetInt("id")), Files: []dto.MediaUploadFileRequest{{
			ClientID: "openai-video", Kind: "video", Filename: header.Filename, MimeType: mimeType, SizeBytes: header.Size,
		}}},
	)
	if problem != nil {
		return "", problem
	}
	if len(sessions.Data) != 1 || strings.TrimSpace(sessions.Data[0].UploadURL) == "" {
		return "", videoCompatibilityProblem(http.StatusBadGateway, "invalid_upload_response", "Media upload service returned an invalid session", "video")
	}
	session := sessions.Data[0]
	file, err := header.Open()
	if err != nil {
		return "", videoCompatibilityProblem(http.StatusBadRequest, "invalid_upload_video", "Failed to open uploaded video", "video")
	}
	defer file.Close()
	method := strings.ToUpper(strings.TrimSpace(session.Method))
	if method == "" {
		method = http.MethodPut
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), method, session.UploadURL, io.LimitReader(file, maxOpenAIVideoUploadBytes+1))
	if err != nil {
		return "", videoCompatibilityProblem(http.StatusInternalServerError, "server_error", "Failed to create media upload request", "video")
	}
	request.ContentLength = header.Size
	for key, value := range session.Headers {
		request.Header.Set(key, value)
	}
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", videoCompatibilityProblem(http.StatusBadGateway, "media_upload_failed", "Failed to upload video", "video")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	_ = response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", videoCompatibilityProblem(http.StatusBadGateway, "media_upload_failed", "Video upload was rejected", "video")
	}
	completed, completeProblem := forwardMediaUploadControl[dto.MediaUploadListResponse](
		c.Request.Context(), "/internal/v1/media/uploads/complete",
		internalMediaUploadCompleteRequest{OwnerID: strconv.Itoa(c.GetInt("id")), UploadIDs: []string{session.ID}},
	)
	if completeProblem != nil {
		return "", completeProblem
	}
	if len(completed.Data) != 1 || !absoluteHTTPURL(completed.Data[0].URL) {
		return "", videoCompatibilityProblem(http.StatusBadGateway, "invalid_upload_response", "Media upload service returned an invalid video URL", "video")
	}
	return completed.Data[0].URL, nil
}

func GetOpenAIVideo(c *gin.Context) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), strings.TrimSpace(c.Param("task_id")))
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to load video", "")
		return
	}
	if !exists || task == nil || constant.TaskActionAssetType(task.Action) != constant.TaskAssetTypeVideo || service.IsOpenAIVideoCompatibilityDeleted(task) {
		writeVideoTaskAPIError(c, http.StatusNotFound, "video_not_found", "Video not found", "video_id")
		return
	}
	if service.IsOpenAIVideoCompatibilityTask(task) {
		video, buildErr := service.BuildOpenAIVideoCompatibilityTask(task)
		if buildErr != nil {
			writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build video response", "")
			return
		}
		c.JSON(http.StatusOK, video)
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func ListOpenAIVideos(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100", "limit")
		return
	}
	order := strings.ToLower(strings.TrimSpace(c.DefaultQuery("order", "desc")))
	if order != "asc" && order != "desc" {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_request", "order must be asc or desc", "order")
		return
	}
	after := strings.TrimSpace(c.Query("after"))
	videos := make([]*dto.OpenAIVideo, 0, limit+1)
	for len(videos) <= limit {
		tasks, hasMore, queryErr := model.ListPublicVideoTasks(c.GetInt("id"), model.VideoTaskListQuery{
			AfterTaskID: after, Order: order, Limit: 100,
		})
		if queryErr != nil {
			if after != "" {
				writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_cursor", "after video was not found", "after")
				return
			}
			writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to list videos", "")
			return
		}
		for _, task := range tasks {
			if service.IsOpenAIVideoCompatibilityDeleted(task) {
				continue
			}
			video, projectionErr := projectOpenAIVideo(task)
			if projectionErr != nil {
				writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build video list", "")
				return
			}
			if video != nil {
				videos = append(videos, video)
				if len(videos) > limit {
					break
				}
			}
		}
		if len(videos) > limit || !hasMore || len(tasks) == 0 {
			break
		}
		after = tasks[len(tasks)-1].TaskID
	}
	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}
	response := dto.OpenAIVideoList{Object: "list", Data: videos, HasMore: hasMore}
	if len(videos) > 0 {
		response.FirstID = videos[0].ID
		response.LastID = videos[len(videos)-1].ID
	}
	c.JSON(http.StatusOK, response)
}

func projectOpenAIVideo(task *model.Task) (*dto.OpenAIVideo, error) {
	if service.IsOpenAIVideoCompatibilityTask(task) {
		return service.BuildOpenAIVideoCompatibilityTask(task)
	}
	adaptor := relay.GetTaskAdaptor(task.Platform)
	converter, ok := adaptor.(channel.OpenAIVideoConverter)
	if !ok {
		return nil, nil
	}
	body, err := converter.ConvertToOpenAIVideo(task)
	if err != nil {
		return nil, err
	}
	var video dto.OpenAIVideo
	if err := common.Unmarshal(body, &video); err != nil {
		return nil, err
	}
	return &video, nil
}

func DeleteOpenAIVideo(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	task, exists, deleted, err := model.TombstoneOpenAIVideoTask(c.GetInt("id"), taskID, time.Now().Unix())
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to delete video", "")
		return
	}
	if !exists || task == nil || !service.IsOpenAIVideoCompatibilityTask(task) {
		writeVideoTaskAPIError(c, http.StatusNotFound, "video_not_found", "Video not found", "video_id")
		return
	}
	if !deleted {
		writeVideoTaskAPIError(c, http.StatusConflict, "video_not_terminal", "Video can only be deleted after it reaches a terminal state", "video_id")
		return
	}
	c.JSON(http.StatusOK, dto.OpenAIVideoDeleted{ID: taskID, Object: "video.deleted", Deleted: true})
}

func OpenAIVideoContent(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to load video", "")
		return
	}
	if !exists || task == nil || constant.TaskActionAssetType(task.Action) != constant.TaskAssetTypeVideo || service.IsOpenAIVideoCompatibilityDeleted(task) {
		writeVideoTaskAPIError(c, http.StatusNotFound, "video_not_found", "Video not found", "video_id")
		return
	}
	variant := strings.ToLower(strings.TrimSpace(c.DefaultQuery("variant", "video")))
	if variant == "video" {
		VideoProxy(c)
		return
	}
	if variant != "thumbnail" && variant != "spritesheet" {
		writeVideoTaskAPIError(c, http.StatusBadRequest, "invalid_video_variant", "variant must be video, thumbnail, or spritesheet", "variant")
		return
	}
	if task.Status != model.TaskStatusSuccess {
		writeVideoTaskAPIError(c, http.StatusConflict, "video_not_completed", "Video is not completed", "video_id")
		return
	}
	assets, assetErr := model.GetUserAssetsByTaskIDs(task.UserId, []string{task.TaskID})
	if assetErr != nil {
		writeVideoTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to load video assets", "")
		return
	}
	for _, asset := range assets {
		if asset.AssetType != model.AssetTypeVideo {
			continue
		}
		assetURL := ""
		if variant == "thumbnail" {
			assetURL = strings.TrimSpace(asset.ThumbnailURL)
			if assetURL == "" {
				assetURL, _ = asset.Metadata["thumbnail_url"].(string)
			}
		} else {
			assetURL, _ = asset.Metadata["spritesheet_url"].(string)
		}
		if strings.TrimSpace(assetURL) != "" {
			streamVideoContent(c, task, assetURL)
			return
		}
	}
	writeVideoTaskAPIError(c, http.StatusNotFound, "video_variant_unavailable", "Requested video variant is unavailable", "variant")
}

func OpenAIVideoCharacterCapability(c *gin.Context) {
	writeVideoTaskAPIError(c, http.StatusBadRequest, "unsupported_video_capability", "Video characters are not supported by the selected normalized video providers", "")
}
