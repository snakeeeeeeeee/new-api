package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
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
	ProgressKnown            *bool                `json:"progress_known,omitempty"`
	ProgressSource           string               `json:"progress_source,omitempty"`
	Stage                    string               `json:"stage,omitempty"`
	Sequence                 int64                `json:"sequence,omitempty"`
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
	TotalTokens              int                     `json:"total_tokens"`
	InputTokens              int                     `json:"input_tokens"`
	OutputTokens             int                     `json:"output_tokens"`
	PromptTokens             int                     `json:"prompt_tokens"`
	CompletionTokens         int                     `json:"completion_tokens"`
	CachedTokens             int                     `json:"cached_tokens"`
	CacheReadTokens          int                     `json:"cache_read_tokens"`
	PromptCacheHitTokens     int                     `json:"prompt_cache_hit_tokens"`
	CacheCreationTokens      int                     `json:"cache_creation_tokens"`
	CacheCreationInputTokens int                     `json:"cache_creation_input_tokens"`
	CacheCreation5mTokens    int                     `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens    int                     `json:"cache_creation_1h_tokens"`
	ImageTokens              int                     `json:"image_tokens"`
	AudioTokens              int                     `json:"audio_tokens"`
	ActualQuota              int                     `json:"actual_quota"`
	CompletionTokensDetails  *dto.OutputTokenDetails `json:"completion_tokens_details,omitempty"`
}

type imageCallbackError struct {
	Code                 string          `json:"code"`
	Message              string          `json:"message"`
	Retryable            bool            `json:"retryable"`
	UpstreamStatus       int             `json:"upstream_status,omitempty"`
	ProviderErrorCode    string          `json:"provider_error_code,omitempty"`
	ProviderErrorType    string          `json:"provider_error_type,omitempty"`
	ProviderErrorMessage string          `json:"provider_error_message,omitempty"`
	ProviderErrorParam   string          `json:"provider_error_param,omitempty"`
	UpstreamError        json.RawMessage `json:"upstream_error,omitempty"`
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
	task, exist, err := model.GetPublicImageTask(userID, taskID, imageHandleTaskPlatform())
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to load task", "")
		return
	}
	if !exist {
		writeImageTaskAPIError(c, http.StatusNotFound, "task_not_found", "Task not found", "task_id")
		return
	}
	publicTask, err := service.BuildPublicImageTask(task)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task response", "")
		return
	}
	c.JSON(http.StatusOK, publicTask)
}

func QueryImageTasks(c *gin.Context) {
	var req imageTaskQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "Invalid JSON request body", "")
		return
	}
	if len(req.TaskIDs) == 0 || len(req.TaskIDs) > 100 {
		writeImageTaskAPIError(c, http.StatusBadRequest, "invalid_request", "task_ids must contain between 1 and 100 IDs", "task_ids")
		return
	}
	userID := c.GetInt("id")
	tasks, err := model.GetPublicImageTasksByIDs(userID, req.TaskIDs, imageHandleTaskPlatform())
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to query tasks", "")
		return
	}
	publicTasks, err := service.BuildPublicImageTasks(tasks)
	if err != nil {
		writeImageTaskAPIError(c, http.StatusInternalServerError, "server_error", "Failed to build task response", "")
		return
	}
	byID := make(map[string]*dto.ImageTaskPublic, len(publicTasks))
	for _, task := range publicTasks {
		byID[task.ID] = task
	}
	ordered := make([]*dto.ImageTaskPublic, 0, len(req.TaskIDs))
	missing := make([]string, 0)
	for _, taskID := range req.TaskIDs {
		if task, ok := byID[taskID]; ok {
			ordered = append(ordered, task)
		} else {
			missing = append(missing, taskID)
		}
	}
	c.JSON(http.StatusOK, dto.ImageTaskListResponse{Object: "list", Data: ordered, Missing: missing})
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
	attempt, attemptExists, attemptErr := model.GetImageTaskAttemptByClientTaskID(event.ClientTaskID)
	if attemptErr != nil {
		result.Status = "not_found"
		result.Message = attemptErr.Error()
		return result
	}
	if attemptExists {
		return handleImageAttemptCallbackEvent(c, event, attempt, result)
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
	if event.Sequence > 0 && task.PrivateData.ProgressMetadataSet &&
		event.Sequence <= task.PrivateData.ProgressSequence {
		result.Status = "ignored_stale"
		return result
	}
	taskInfo := imageCallbackEventToTaskInfo(event)
	if taskInfo.Status == "" {
		result.Status = "invalid_status"
		return result
	}
	if model.TaskStatus(taskInfo.Status) == model.TaskStatusFailure && event.Error != nil && event.Error.Retryable {
		attempt, _, adoptErr := relay.AdoptLegacyAsyncImageTaskForRetry(task, event.ProviderTaskID)
		if adoptErr != nil {
			result.Status = "retry_state_error"
			result.Message = adoptErr.Error()
			return result
		}
		if attempt != nil {
			return handleImageAttemptCallbackEvent(c, event, attempt, result)
		}
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

func handleImageAttemptCallbackEvent(c *gin.Context, event imageCallbackEvent, attempt *model.ImageTaskAttempt, result imageCallbackResultItem) imageCallbackResultItem {
	if attempt == nil {
		result.Status = "not_found"
		return result
	}
	parent, err := relay.GetAsyncImageAttemptParent(attempt)
	if err != nil || parent == nil {
		result.Status = "not_found"
		if err != nil {
			result.Message = err.Error()
		}
		return result
	}
	if parent.Platform != imageHandleTaskPlatform() {
		result.Status = "invalid_status"
		result.Message = "task is not an image-handle task"
		return result
	}
	if callbackChannelID := c.GetInt(imageCallbackChannelIDContextKey); callbackChannelID > 0 && attempt.ChannelID != callbackChannelID {
		result.Status = "channel_mismatch"
		return result
	}
	if event.ProviderTaskID != "" && attempt.ProviderTaskID != "" && event.ProviderTaskID != attempt.ProviderTaskID {
		result.Status = "provider_task_mismatch"
		return result
	}
	if model.ImageTaskAttemptIsTerminal(attempt.Status) || parent.Status == model.TaskStatusSuccess || parent.Status == model.TaskStatusFailure {
		result.Status = "ignored_terminal"
		relay.LogAsyncImageAttemptIgnored(c.Request.Context(), parent.TaskID, attempt.AttemptNumber, "terminal")
		return result
	}
	if event.Sequence > 0 && event.Sequence <= attempt.ProgressSequence {
		result.Status = "ignored_stale"
		relay.LogAsyncImageAttemptIgnored(c.Request.Context(), parent.TaskID, attempt.AttemptNumber, "stale_sequence")
		return result
	}
	taskInfo := imageCallbackEventToTaskInfo(event)
	if taskInfo.Status == "" {
		result.Status = "invalid_status"
		return result
	}
	sanitizeImageCallbackEvent(&event)
	raw, _ := common.Marshal(event)
	taskInfo.Data = raw

	switch model.TaskStatus(taskInfo.Status) {
	case model.TaskStatusFailure:
		return handleImageAttemptFailure(c, event, attempt, parent, taskInfo, raw, result)
	case model.TaskStatusSuccess:
		preparedParent, ignored, prepareErr := relay.PrepareAsyncImageAttemptSuccess(attempt.ID, event.ProviderTaskID, taskInfo, raw)
		if prepareErr != nil {
			result.Status = "retry_state_error"
			result.Message = prepareErr.Error()
			return result
		}
		if ignored || preparedParent == nil {
			result.Status = "ignored_stale"
			return result
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"async image attempt succeeded: task_id=%s attempt=%d channel_id=%d route_group=%q",
			preparedParent.TaskID, attempt.AttemptNumber, attempt.ChannelID, attempt.RouteGroup,
		))
		adaptor := relay.GetTaskAdaptor(preparedParent.Platform)
		if adaptor == nil {
			result.Status = "invalid_status"
			result.Message = "task adaptor not found"
			return result
		}
		updated, _ := service.ApplyTaskResult(c.Request.Context(), adaptor, preparedParent, taskInfo)
		if updated {
			recordAsyncImageAttemptRouteSuccess(c, preparedParent, attempt)
		}
		result.Status = "accepted"
		return result
	default:
		ignored, updateErr := relay.ApplyAsyncImageAttemptProgress(attempt.ID, event.ProviderTaskID, taskInfo, raw)
		if updateErr != nil {
			result.Status = "retry_state_error"
			result.Message = updateErr.Error()
			return result
		}
		if ignored {
			result.Status = "ignored_stale"
			return result
		}
		result.Status = "accepted"
		return result
	}
}

func handleImageAttemptFailure(c *gin.Context, event imageCallbackEvent, attempt *model.ImageTaskAttempt, parent *model.Task, taskInfo *relaycommon.TaskInfo, raw []byte, result imageCallbackResultItem) imageCallbackResultItem {
	errorCode := "image_task_failed"
	errorMessage := strings.TrimSpace(taskInfo.Reason)
	retryable := false
	if event.Error != nil {
		if strings.TrimSpace(event.Error.Code) != "" {
			errorCode = strings.TrimSpace(event.Error.Code)
		}
		if strings.TrimSpace(event.Error.Message) != "" {
			errorMessage = strings.TrimSpace(event.Error.Message)
		}
		retryable = event.Error.Retryable
	}
	outcome, err := relay.TransitionAsyncImageAttemptFailure(c, attempt.ID, errorCode, errorMessage, retryable, raw)
	if err != nil {
		// A callback is acknowledged with HTTP 200 by protocol. If constructing a
		// later attempt fails, close the same attempt non-retryably so the parent
		// cannot remain queued forever.
		fallback, fallbackErr := relay.TransitionAsyncImageAttemptFailure(c, attempt.ID, "retry_dispatch_failed", err.Error(), false, raw)
		if fallbackErr != nil {
			result.Status = "retry_state_error"
			result.Message = fallbackErr.Error()
			return result
		}
		outcome = fallback
		errorCode = "retry_dispatch_failed"
		retryable = false
		taskInfo.Reason = "failed to schedule image retry: " + err.Error()
	}
	if outcome.Ignored {
		result.Status = "ignored_stale"
		return result
	}
	recordAsyncImageAttemptRouteFailure(c, parent, attempt, event)
	processAsyncImageAttemptChannelFailure(attempt, event)
	logger.LogInfo(c.Request.Context(), fmt.Sprintf(
		"async image attempt failed: task_id=%s attempt=%d channel_id=%d route_group=%q retryable=%t error_code=%q",
		parent.TaskID, attempt.AttemptNumber, attempt.ChannelID, attempt.RouteGroup, retryable, errorCode,
	))
	if outcome.Retrying {
		if outcome.NextAttempt != nil && outcome.NextAttempt.RouteGroup != attempt.RouteGroup {
			logger.LogInfo(c.Request.Context(), fmt.Sprintf(
				"async image route switched: task_id=%s from_group=%q to_group=%q",
				parent.TaskID, attempt.RouteGroup, outcome.NextAttempt.RouteGroup,
			))
		}
		result.Status = "retry_scheduled"
		return result
	}
	if outcome.Exhausted {
		attemptCount := attempt.AttemptNumber
		if state, exists, stateErr := model.GetImageTaskRetryStateByTaskRecordID(parent.ID); stateErr == nil && exists && state != nil {
			attemptCount = state.AttemptCount
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"async image retry exhausted: task_id=%s attempts=%d error_code=%q",
			parent.TaskID, attemptCount, errorCode,
		))
		latestParent, loadErr := model.GetTaskByRecordID(parent.ID)
		if loadErr != nil {
			result.Status = "retry_state_error"
			result.Message = loadErr.Error()
			return result
		}
		adaptor := relay.GetTaskAdaptor(latestParent.Platform)
		service.ApplyTaskResult(c.Request.Context(), adaptor, latestParent, taskInfo)
		result.Status = "accepted"
		return result
	}
	result.Status = "accepted"
	return result
}

func recordAsyncImageAttemptRouteFailure(c *gin.Context, parent *model.Task, attempt *model.ImageTaskAttempt, event imageCallbackEvent) {
	if c == nil || parent == nil || attempt == nil {
		return
	}
	state, exists, err := model.GetImageTaskRetryStateByTaskRecordID(parent.ID)
	if err != nil || !exists || state == nil || state.AggregateGroup == "" {
		return
	}
	statusCode := 0
	if event.Error != nil {
		statusCode = event.Error.UpstreamStatus
	}
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, state.AggregateGroup)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutingMode, state.RoutingMode)
	common.SetContextKey(c, constant.ContextKeyRouteGroup, attempt.RouteGroup)
	common.SetContextKey(c, constant.ContextKeyRouteGroupIndex, attempt.RouteIndex)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutePool, attempt.RoutePool)
	if aggregateGroup, ok := service.GetAggregateGroup(state.AggregateGroup, true); ok {
		common.SetContextKey(c, constant.ContextKeyAggregateSmartRouting, service.IsAggregateSmartRoutingEnabled(aggregateGroup))
	}
	service.RecordAggregateRouteRPMFailure(c, attempt.OriginModel)
	service.RecordAggregateRouteSmartFailure(c, attempt.OriginModel, attempt.RouteGroup, statusCode)
}

func processAsyncImageAttemptChannelFailure(attempt *model.ImageTaskAttempt, event imageCallbackEvent) {
	if attempt == nil || event.Error == nil {
		return
	}
	channel, err := model.GetChannelById(attempt.ChannelID, true)
	if err != nil || channel == nil || !channel.GetAutoBan() {
		return
	}
	code := firstNonEmptyCallbackErrorValue(event.Error.ProviderErrorCode, event.Error.Code, "upstream_error")
	message := firstNonEmptyCallbackErrorValue(event.Error.ProviderErrorMessage, event.Error.Message, code)
	newAPIError := types.WithOpenAIError(types.OpenAIError{
		Message: message,
		Type:    event.Error.ProviderErrorType,
		Param:   event.Error.ProviderErrorParam,
		Code:    code,
	}, event.Error.UpstreamStatus)
	if !service.ShouldDisableChannel(channel.Type, newAPIError) {
		return
	}
	usingKey := channel.Key
	if channel.ChannelInfo.IsMultiKey {
		lease, exists, leaseErr := model.GetImageCredentialLeaseByAttemptRecordID(attempt.ID)
		if leaseErr != nil || !exists || lease == nil || lease.ResolvedKeyIndex == nil {
			return
		}
		keys := channel.GetKeys()
		if *lease.ResolvedKeyIndex < 0 || *lease.ResolvedKeyIndex >= len(keys) {
			return
		}
		usingKey = keys[*lease.ResolvedKeyIndex]
	}
	channelError := types.NewChannelError(
		channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, usingKey, channel.GetAutoBan(),
	)
	gopool.Go(func() {
		service.DisableChannel(*channelError, newAPIError.ErrorWithStatusCode())
	})
}

func firstNonEmptyCallbackErrorValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func recordAsyncImageAttemptRouteSuccess(c *gin.Context, parent *model.Task, attempt *model.ImageTaskAttempt) {
	state, exists, err := model.GetImageTaskRetryStateByTaskRecordID(parent.ID)
	if err != nil || !exists || state == nil || state.AggregateGroup == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, state.AggregateGroup)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutingMode, state.RoutingMode)
	common.SetContextKey(c, constant.ContextKeyRouteGroup, attempt.RouteGroup)
	common.SetContextKey(c, constant.ContextKeyRouteGroupIndex, attempt.RouteIndex)
	common.SetContextKey(c, constant.ContextKeyAggregateRoutePool, attempt.RoutePool)
	if aggregateGroup, ok := service.GetAggregateGroup(state.AggregateGroup, true); ok {
		common.SetContextKey(c, constant.ContextKeyAggregateSmartRouting, service.IsAggregateSmartRoutingEnabled(aggregateGroup))
	}
	service.RecordAggregateRouteSuccess(c, attempt.OriginModel)
}

func imageHandleTaskPlatform() constant.TaskPlatform {
	return constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeImageHandle))
}

func imageCallbackEventToTaskInfo(event imageCallbackEvent) *relaycommon.TaskInfo {
	info := &relaycommon.TaskInfo{
		TaskID:   event.ProviderTaskID,
		Progress: event.Progress,
	}
	if event.ProgressKnown != nil || event.ProgressSource != "" || event.Stage != "" || event.Sequence > 0 {
		info.ProgressMetadataSet = true
		if event.ProgressKnown != nil {
			info.ProgressKnown = *event.ProgressKnown
		}
		info.ProgressSource = event.ProgressSource
		info.Stage = event.Stage
		info.Sequence = event.Sequence
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
		CompletionTokenDetails:      callbackOutputTokenDetails(usage.CompletionTokensDetails),
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
	if usage.CompletionTokenDetails.ReasoningTokens == 0 {
		usage.CompletionTokenDetails.ReasoningTokens = fromRaw.CompletionTokenDetails.ReasoningTokens
	}
	if usage.CompletionTokenDetails.TextTokens == 0 {
		usage.CompletionTokenDetails.TextTokens = fromRaw.CompletionTokenDetails.TextTokens
	}
	if usage.CompletionTokenDetails.AudioTokens == 0 {
		usage.CompletionTokenDetails.AudioTokens = fromRaw.CompletionTokenDetails.AudioTokens
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
	completionDetails := dto.OutputTokenDetails{
		TextTokens:      nestedUsageInt(usage, "completion_tokens_details", "text_tokens"),
		AudioTokens:     nestedUsageInt(usage, "completion_tokens_details", "audio_tokens"),
		ReasoningTokens: nestedUsageInt(usage, "completion_tokens_details", "reasoning_tokens"),
	}
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
		CompletionTokenDetails: completionDetails,
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

func callbackOutputTokenDetails(details *dto.OutputTokenDetails) dto.OutputTokenDetails {
	if details == nil {
		return dto.OutputTokenDetails{}
	}
	return *details
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
