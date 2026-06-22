package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const callbackTimestampWindowSeconds = 5 * 60
const imageCallbackChannelIDContextKey = "image_callback_channel_id"

type imageTaskQueryRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type imageCallbackBatchRequest struct {
	Events []imageCallbackEvent `json:"events"`
}

type imageCallbackEvent struct {
	EventID        string               `json:"event_id"`
	ClientTaskID   string               `json:"client_task_id"`
	ProviderTaskID string               `json:"provider_task_id"`
	Status         string               `json:"status"`
	Progress       string               `json:"progress"`
	Result         *imageCallbackResult `json:"result"`
	Usage          *imageCallbackUsage  `json:"usage"`
	Error          *imageCallbackError  `json:"error"`
}

type imageCallbackResult struct {
	Images []imageCallbackImage `json:"images"`
}

type imageCallbackImage struct {
	URL string `json:"url"`
}

type imageCallbackUsage struct {
	TotalTokens int `json:"total_tokens"`
	ActualQuota int `json:"actual_quota"`
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
	common.ApiSuccess(c, tasksToDto(tasks, false))
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
	return ch.GetOtherSettings().CallbackSecret, channelID, nil
}

func signCallbackPayload(timestamp string, rawBody []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
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
	if event.ProviderTaskID != "" && task.GetUpstreamTaskID() != "" && event.ProviderTaskID != task.GetUpstreamTaskID() {
		result.Status = "provider_task_mismatch"
		return result
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
		info.TotalTokens = event.Usage.TotalTokens
		info.CompletionTokens = event.Usage.ActualQuota
	}
	if event.Error != nil {
		info.Reason = event.Error.Message
		if info.Reason == "" {
			info.Reason = event.Error.Code
		}
	}
	return info
}
