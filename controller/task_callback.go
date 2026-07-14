package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const callbackTimestampWindowSeconds = 5 * 60
const imageCallbackChannelIDContextKey = "image_callback_channel_id"
const rawResponseMaxBytes = 256 * 1024

type imageTaskQueryRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type imageCallbackBatchRequest struct {
	Events []imageCallbackEvent `json:"events"`
}

type imageCallbackEvent struct {
	EventID                  string               `json:"event_id"`
	ClientTaskID             string               `json:"client_task_id"`
	ProviderTaskID           string               `json:"provider_task_id"`
	Status                   string               `json:"status"`
	Progress                 string               `json:"progress"`
	Result                   *imageCallbackResult `json:"result"`
	Usage                    *imageCallbackUsage  `json:"usage"`
	Error                    *imageCallbackError  `json:"error"`
	RawResponse              json.RawMessage      `json:"raw_response,omitempty"`
	RawResponseTruncated     bool                 `json:"raw_response_truncated,omitempty"`
	RawResponseOmittedFields []string             `json:"raw_response_omitted_fields,omitempty"`
}

type imageCallbackResult struct {
	Images   []imageCallbackImage `json:"images"`
	Output   map[string]any       `json:"output,omitempty"`
	Metadata map[string]any       `json:"metadata,omitempty"`
}

type imageCallbackImage struct {
	URL           string `json:"url"`
	MimeType      string `json:"mime_type,omitempty"`
	Format        string `json:"format,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	SizeBytes     int64  `json:"size_bytes,omitempty"`
	Filename      string `json:"filename,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type imageCallbackUsage struct {
	TotalTokens              int `json:"total_tokens"`
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	CachedTokens             int `json:"cached_tokens"`
	CacheReadTokens          int `json:"cache_read_tokens"`
	PromptCacheHitTokens     int `json:"prompt_cache_hit_tokens"`
	CacheCreationTokens      int `json:"cache_creation_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheCreation5mTokens    int `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens    int `json:"cache_creation_1h_tokens"`
	ImageTokens              int `json:"image_tokens"`
	AudioTokens              int `json:"audio_tokens"`
	ActualQuota              int `json:"actual_quota"`
}

type imageCallbackError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type imageCallbackResultItem struct {
	EventID      string `json:"event_id,omitempty"`
	ClientTaskID string `json:"client_task_id,omitempty"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
}

func GetImageTask(c *gin.Context) {
	taskID := c.Param("task_id")
	userID := c.GetInt("id")
	task, exist, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exist {
		common.ApiError(c, errors.New("task_not_exist"))
		return
	}
	if task.Platform != imageHandleTaskPlatform() {
		common.ApiError(c, errors.New("task_not_exist"))
		return
	}
	common.ApiSuccess(c, relay.TaskModel2Dto(task))
}

func QueryImageTasks(c *gin.Context) {
	var req imageTaskQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.TaskIDs) > 100 {
		common.ApiError(c, errors.New("task_ids max size is 100"))
		return
	}
	userID := c.GetInt("id")
	tasks, err := model.GetByTaskIDStrings(userID, req.TaskIDs, imageHandleTaskPlatform())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, imageTasksToDto(tasks))
}

func imageTasksToDto(tasks []*model.Task) []*dto.TaskDto {
	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		result[i] = relay.TaskModel2Dto(task)
	}
	return result
}

func ImageTaskCallback(c *gin.Context) {
	rawBody, ok := verifyImageCallback(c)
	if !ok {
		return
	}
	var event imageCallbackEvent
	if err := common.Unmarshal(rawBody, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback body"})
		return
	}
	if event.ClientTaskID == "" {
		event.ClientTaskID = c.Param("task_id")
	}
	result := handleImageCallbackEvent(c, event)
	c.JSON(http.StatusOK, gin.H{
		"code":    "success",
		"results": []imageCallbackResultItem{result},
	})
}

func ImageTaskCallbackBatch(c *gin.Context) {
	rawBody, ok := verifyImageCallback(c)
	if !ok {
		return
	}
	var req imageCallbackBatchRequest
	if err := common.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback body"})
		return
	}
	results := make([]imageCallbackResultItem, 0, len(req.Events))
	for _, event := range req.Events {
		results = append(results, handleImageCallbackEvent(c, event))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    "success",
		"results": results,
	})
}

func verifyImageCallback(c *gin.Context) ([]byte, bool) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read callback body failed"})
		return nil, false
	}
	timestamp := strings.TrimSpace(c.GetHeader("X-Callback-Timestamp"))
	signature := strings.TrimSpace(c.GetHeader("X-Callback-Signature"))
	secretID := strings.TrimSpace(c.GetHeader("X-Callback-Secret-Id"))
	if timestamp == "" || signature == "" || secretID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing callback signature headers"})
		return nil, false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid callback timestamp"})
		return nil, false
	}
	now := time.Now().Unix()
	if now-ts > callbackTimestampWindowSeconds || ts-now > callbackTimestampWindowSeconds {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "callback timestamp expired"})
		return nil, false
	}
	secret, channelID, err := resolveCallbackSecret(secretID)
	if err != nil || secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "callback secret not found"})
		return nil, false
	}
	expected := signCallbackPayload(timestamp, rawBody, secret)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(signature)), []byte(expected)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid callback signature"})
		return nil, false
	}
	c.Set(imageCallbackChannelIDContextKey, channelID)
	return rawBody, true
}

func resolveCallbackSecret(secretID string) (string, int, error) {
	if !strings.HasPrefix(secretID, "channel_") {
		if secret, ok := service.ResolveImageHandleCallbackSecret(secretID); ok && secret != "" {
			return secret, 0, nil
		}
		return "", 0, fmt.Errorf("invalid secret id")
	}
	channelID, err := strconv.Atoi(strings.TrimPrefix(secretID, "channel_"))
	if err != nil || channelID <= 0 {
		return "", 0, fmt.Errorf("invalid channel id")
	}
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return "", 0, err
	}
	if secret := ch.GetOtherSettings().CallbackSecret; secret != "" {
		return secret, channelID, nil
	}
	if secret, ok := service.ResolveImageHandleCallbackSecret(secretID); ok && secret != "" {
		return secret, channelID, nil
	}
	return "", channelID, fmt.Errorf("callback secret not configured")
}

func signCallbackPayload(timestamp string, rawBody []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeEqualHex(got, expected string) bool {
	got = strings.ToLower(strings.TrimSpace(got))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func handleImageCallbackEvent(c *gin.Context, event imageCallbackEvent) imageCallbackResultItem {
	result := imageCallbackResultItem{
		EventID:      event.EventID,
		ClientTaskID: event.ClientTaskID,
	}
	if event.ClientTaskID == "" {
		result.Status = "invalid_status"
		result.Message = "client_task_id is required"
		return result
	}
	task, exist, err := model.GetByOnlyTaskId(event.ClientTaskID)
	if err != nil {
		result.Status = "not_found"
		result.Message = err.Error()
		return result
	}
	if !exist || task == nil {
		result.Status = "not_found"
		return result
	}
	if task.Platform != imageHandleTaskPlatform() {
		result.Status = "invalid_status"
		result.Message = "task is not an image-handle task"
		return result
	}
	if callbackChannelID := c.GetInt(imageCallbackChannelIDContextKey); callbackChannelID > 0 && task.ChannelId != callbackChannelID {
		result.Status = "channel_mismatch"
		return result
	}
	if event.ProviderTaskID != "" && task.PrivateData.UpstreamTaskID != "" && event.ProviderTaskID != task.PrivateData.UpstreamTaskID {
		result.Status = "provider_task_mismatch"
		return result
	}
	if task.PrivateData.UpstreamTaskID == "" {
		task.PrivateData.UpstreamTaskID = event.ProviderTaskID
	}
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		result.Status = "ignored_terminal"
		return result
	}
	taskInfo := imageCallbackEventToTaskInfo(event)
	if taskInfo.Status == "" {
		result.Status = "invalid_status"
		return result
	}
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		result.Status = "invalid_status"
		result.Message = "task adaptor not found"
		return result
	}
	sanitizeImageCallbackEvent(&event)
	raw, _ := common.Marshal(event)
	task.Data = raw
	service.ApplyTaskResult(c.Request.Context(), adaptor, task, taskInfo)
	result.Status = "accepted"
	return result
}

func imageHandleTaskPlatform() constant.TaskPlatform {
	return constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeImageHandle))
}

func imageCallbackEventToTaskInfo(event imageCallbackEvent) *relaycommon.TaskInfo {
	info := &relaycommon.TaskInfo{
		TaskID:   event.ProviderTaskID,
		Progress: event.Progress,
	}
	switch strings.ToLower(event.Status) {
	case "submitted":
		info.Status = model.TaskStatusSubmitted
	case "queued":
		info.Status = model.TaskStatusQueued
	case "processing":
		info.Status = model.TaskStatusInProgress
	case "succeeded":
		info.Status = model.TaskStatusSuccess
	case "failed":
		info.Status = model.TaskStatusFailure
	default:
		return info
	}
	if info.Progress == "" {
		if info.Status == model.TaskStatusSuccess || info.Status == model.TaskStatusFailure {
			info.Progress = taskcommon.ProgressComplete
		}
	}
	if event.Result != nil && len(event.Result.Images) > 0 {
		info.Url = event.Result.Images[0].URL
	}
	if event.Usage != nil {
		info.Usage = callbackUsageToDTO(event.Usage)
		info.TotalTokens = info.Usage.TotalTokens
		info.CompletionTokens = info.Usage.CompletionTokens
		info.ActualQuota = event.Usage.ActualQuota
	}
	if info.TotalTokens == 0 {
		info.Usage = usageFromRawResponse(event.RawResponse)
		if info.Usage != nil {
			info.TotalTokens = info.Usage.TotalTokens
			info.CompletionTokens = info.Usage.CompletionTokens
		}
	} else if info.Usage != nil {
		mergeUsageFromRawResponse(info.Usage, event.RawResponse)
	}
	if event.Error != nil {
		info.Reason = event.Error.Message
		if info.Reason == "" {
			info.Reason = event.Error.Code
		}
	}
	return info
}

func callbackUsageToDTO(usage *imageCallbackUsage) *dto.Usage {
	if usage == nil {
		return nil
	}
	inputTokens := firstPositiveInt(usage.InputTokens, usage.PromptTokens)
	outputTokens := firstPositiveInt(usage.OutputTokens, usage.CompletionTokens)
	totalTokens := firstPositiveInt(usage.TotalTokens, inputTokens+outputTokens)
	cachedTokens := firstPositiveInt(usage.CachedTokens, usage.CacheReadTokens, usage.PromptCacheHitTokens)
	cacheCreationTokens := firstPositiveInt(usage.CacheCreationTokens, usage.CacheCreationInputTokens)
	return &dto.Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      totalTokens,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		UsageSource:      "image_handle_callback",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         cachedTokens,
			CachedCreationTokens: cacheCreationTokens,
			ImageTokens:          usage.ImageTokens,
			AudioTokens:          usage.AudioTokens,
		},
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:         cachedTokens,
			CachedCreationTokens: cacheCreationTokens,
			ImageTokens:          usage.ImageTokens,
			AudioTokens:          usage.AudioTokens,
		},
		ClaudeCacheCreation5mTokens: usage.CacheCreation5mTokens,
		ClaudeCacheCreation1hTokens: usage.CacheCreation1hTokens,
	}
}

func sanitizeImageCallbackEvent(event *imageCallbackEvent) {
	if event == nil || len(event.RawResponse) == 0 {
		return
	}
	if len(event.RawResponse) <= rawResponseMaxBytes {
		return
	}
	event.RawResponse = json.RawMessage([]byte(fmt.Sprintf(`{"truncated":true,"original_size_bytes":%d}`, len(event.RawResponse))))
	event.RawResponseTruncated = true
	if len(event.RawResponseOmittedFields) == 0 {
		event.RawResponseOmittedFields = []string{"raw_response"}
	}
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func usageFromRawResponse(raw json.RawMessage) *dto.Usage {
	if len(raw) == 0 || len(raw) > rawResponseMaxBytes {
		return nil
	}
	var data map[string]interface{}
	if err := common.Unmarshal(raw, &data); err != nil {
		return nil
	}
	if usage, ok := data["usage"].(map[string]interface{}); ok {
		return usageMapToDTO(usage)
	}
	return nil
}

func mergeUsageFromRawResponse(usage *dto.Usage, raw json.RawMessage) {
	if usage == nil {
		return
	}
	fromRaw := usageFromRawResponse(raw)
	if fromRaw == nil {
		return
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = fromRaw.PromptTokens
		usage.InputTokens = fromRaw.InputTokens
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = fromRaw.CompletionTokens
		usage.OutputTokens = fromRaw.OutputTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = firstPositiveInt(fromRaw.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	}
	if usage.PromptTokensDetails.CachedTokens == 0 {
		usage.PromptTokensDetails.CachedTokens = fromRaw.PromptTokensDetails.CachedTokens
	}
	if usage.PromptTokensDetails.CachedCreationTokens == 0 {
		usage.PromptTokensDetails.CachedCreationTokens = fromRaw.PromptTokensDetails.CachedCreationTokens
	}
	if usage.PromptTokensDetails.ImageTokens == 0 {
		usage.PromptTokensDetails.ImageTokens = fromRaw.PromptTokensDetails.ImageTokens
	}
	if usage.PromptTokensDetails.AudioTokens == 0 {
		usage.PromptTokensDetails.AudioTokens = fromRaw.PromptTokensDetails.AudioTokens
	}
	if usage.InputTokensDetails == nil {
		usage.InputTokensDetails = fromRaw.InputTokensDetails
	}
	if usage.ClaudeCacheCreation5mTokens == 0 {
		usage.ClaudeCacheCreation5mTokens = fromRaw.ClaudeCacheCreation5mTokens
	}
	if usage.ClaudeCacheCreation1hTokens == 0 {
		usage.ClaudeCacheCreation1hTokens = fromRaw.ClaudeCacheCreation1hTokens
	}
}

func usageMapToDTO(usage map[string]interface{}) *dto.Usage {
	inputTokens := firstPositiveInt(
		intFromAny(usage["input_tokens"]),
		intFromAny(usage["prompt_tokens"]),
	)
	outputTokens := firstPositiveInt(
		intFromAny(usage["output_tokens"]),
		intFromAny(usage["completion_tokens"]),
	)
	totalTokens := firstPositiveInt(intFromAny(usage["total_tokens"]), inputTokens+outputTokens)
	cachedTokens := firstPositiveInt(
		intFromAny(usage["cached_tokens"]),
		intFromAny(usage["cache_read_tokens"]),
		intFromAny(usage["prompt_cache_hit_tokens"]),
	)
	cacheCreationTokens := firstPositiveInt(
		intFromAny(usage["cache_creation_tokens"]),
		intFromAny(usage["cache_creation_input_tokens"]),
	)
	if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		cachedTokens = firstPositiveInt(cachedTokens, intFromAny(details["cached_tokens"]))
		cacheCreationTokens = firstPositiveInt(cacheCreationTokens, intFromAny(details["cached_creation_tokens"]))
	}
	if details, ok := usage["input_tokens_details"].(map[string]interface{}); ok {
		cachedTokens = firstPositiveInt(cachedTokens, intFromAny(details["cached_tokens"]))
		cacheCreationTokens = firstPositiveInt(cacheCreationTokens, intFromAny(details["cached_creation_tokens"]))
	}
	imageTokens := firstPositiveInt(
		intFromAny(usage["image_tokens"]),
		nestedUsageInt(usage, "prompt_tokens_details", "image_tokens"),
		nestedUsageInt(usage, "input_tokens_details", "image_tokens"),
	)
	audioTokens := firstPositiveInt(
		intFromAny(usage["audio_tokens"]),
		nestedUsageInt(usage, "prompt_tokens_details", "audio_tokens"),
		nestedUsageInt(usage, "input_tokens_details", "audio_tokens"),
	)
	return &dto.Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      totalTokens,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		UsageSource:      "image_handle_raw_response",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         cachedTokens,
			CachedCreationTokens: cacheCreationTokens,
			ImageTokens:          imageTokens,
			AudioTokens:          audioTokens,
		},
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:         cachedTokens,
			CachedCreationTokens: cacheCreationTokens,
			ImageTokens:          imageTokens,
			AudioTokens:          audioTokens,
		},
		ClaudeCacheCreation5mTokens: firstPositiveInt(
			intFromAny(usage["cache_creation_5m_tokens"]),
			intFromAny(usage["cache_creation_tokens_5m"]),
		),
		ClaudeCacheCreation1hTokens: firstPositiveInt(
			intFromAny(usage["cache_creation_1h_tokens"]),
			intFromAny(usage["cache_creation_tokens_1h"]),
		),
	}
}

func nestedUsageInt(usage map[string]interface{}, objectKey string, valueKey string) int {
	if nested, ok := usage[objectKey].(map[string]interface{}); ok {
		return intFromAny(nested[valueKey])
	}
	return 0
}

func intFromAny(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := strconv.Atoi(v.String())
		return i
	default:
		return 0
	}
}
