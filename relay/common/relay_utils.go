package common

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

const MaxTaskDurationSeconds = 3600

func taskDurationMetadataValue(metadata map[string]interface{}) (int, bool, error) {
	if metadata == nil {
		return 0, false, nil
	}
	value, exists := metadata["durationSeconds"]
	if !exists {
		return 0, false, nil
	}
	switch v := value.(type) {
	case int:
		return v, true, nil
	case int64:
		if v > int64(MaxTaskDurationSeconds) {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		return int(v), true, nil
	case float64:
		if v != v {
			return 0, true, fmt.Errorf("durationSeconds must be an integer")
		}
		if v < 0 {
			return -1, true, nil
		}
		if v > MaxTaskDurationSeconds {
			return MaxTaskDurationSeconds + 1, true, nil
		}
		if v != float64(int(v)) {
			return 0, true, fmt.Errorf("durationSeconds must be an integer")
		}
		return int(v), true, nil
	case string:
		seconds, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, true, err
		}
		return seconds, true, nil
	default:
		return 0, true, fmt.Errorf("durationSeconds must be an integer")
	}
}

func taskImageCountMetadataValue(metadata map[string]interface{}) (int, bool, error) {
	if metadata == nil {
		return 0, false, nil
	}
	value, exists := metadata["n"]
	if !exists {
		return 0, false, nil
	}
	switch v := value.(type) {
	case int:
		return v, true, nil
	case int64:
		if v > int64(dto.MaxImageN) {
			return dto.MaxImageN + 1, true, nil
		}
		return int(v), true, nil
	case float64:
		if v != v {
			return 0, true, fmt.Errorf("n must be an integer")
		}
		if v < 0 {
			return -1, true, nil
		}
		if v > dto.MaxImageN {
			return dto.MaxImageN + 1, true, nil
		}
		if v != float64(int(v)) {
			return 0, true, fmt.Errorf("n must be an integer")
		}
		return int(v), true, nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, true, err
		}
		return n, true, nil
	default:
		return 0, true, fmt.Errorf("n must be an integer")
	}
}

func validateTaskQuantityBounds(req TaskSubmitReq) *dto.TaskError {
	if req.Duration < 0 || req.Duration > MaxTaskDurationSeconds {
		return createTaskError(fmt.Errorf("seconds must be between 1 and %d", MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest, true)
	}
	if secondsValue := strings.TrimSpace(req.Seconds); secondsValue != "" {
		seconds, err := strconv.Atoi(secondsValue)
		if err != nil || seconds < 0 || seconds > MaxTaskDurationSeconds {
			return createTaskError(fmt.Errorf("seconds must be between 1 and %d", MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest, true)
		}
	}
	if seconds, exists, err := taskDurationMetadataValue(req.Metadata); err != nil || (exists && (seconds < 0 || seconds > MaxTaskDurationSeconds)) {
		return createTaskError(fmt.Errorf("durationSeconds must be between 1 and %d", MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest, true)
	}
	if n, exists, err := taskImageCountMetadataValue(req.Metadata); err != nil || (exists && (n < 0 || n > dto.MaxImageN)) {
		return createTaskError(fmt.Errorf("n must be an integer between 1 and %d", dto.MaxImageN), "invalid_n", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, *dto.TaskError) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:   formData.Get("prompt"),
		Model:    formData.Get("model"),
		Mode:     formData.Get("mode"),
		Image:    formData.Get("image"),
		Size:     formData.Get("size"),
		Metadata: make(map[string]interface{}),
	}

	if durationStr := strings.TrimSpace(formData.Get("seconds")); durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return req, createTaskError(fmt.Errorf("seconds must be an integer"), "invalid_seconds", http.StatusBadRequest, true)
		}
		req.Seconds = durationStr
		req.Duration = duration
	} else if durationStr := strings.TrimSpace(formData.Get("duration")); durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return req, createTaskError(fmt.Errorf("duration must be an integer"), "invalid_seconds", http.StatusBadRequest, true)
		}
		req.Duration = duration
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskQuantityBounds(req); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":          true,
		"model":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"seconds":         true,
		"duration":        true,
		"input_reference": true, // Sora 特有字段
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		var taskErr *dto.TaskError
		req, taskErr = validateMultipartTaskRequest(c, info, action)
		if taskErr != nil {
			return taskErr
		}
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if taskErr := validateTaskQuantityBounds(req); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}
