package imagehandle

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

var clientTaskIDPattern = regexp.MustCompile(`^task_[A-Za-z0-9_-]{1,186}$`)

type imageHandleSubmitRequest struct {
	RequestID       string                 `json:"request_id"`
	ClientTaskID    string                 `json:"client_task_id"`
	Provider        string                 `json:"provider,omitempty"`
	Model           string                 `json:"model"`
	Operation       string                 `json:"operation"`
	Input           imageHandleInput       `json:"input"`
	Parameters      map[string]any         `json:"parameters,omitempty"`
	ProviderOptions map[string]any         `json:"provider_options,omitempty"`
	Executor        imageHandleExecutor    `json:"executor,omitempty"`
	Callback        imageHandleCallback    `json:"callback,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type imageHandleInput struct {
	Text   string   `json:"text"`
	Images []string `json:"images,omitempty"`
	Mask   *string  `json:"mask,omitempty"`
}

type imageHandleCallback struct {
	URL      string `json:"url,omitempty"`
	BatchURL string `json:"batch_url,omitempty"`
	SecretID string `json:"secret_id,omitempty"`
}

type imageHandleExecutor struct {
	Type       string `json:"type,omitempty"`
	ExecuteURL string `json:"execute_url,omitempty"`
	SecretID   string `json:"secret_id,omitempty"`
}

type imageHandleSubmitResponse struct {
	ProviderTaskID string `json:"provider_task_id"`
	ClientTaskID   string `json:"client_task_id"`
	Status         string `json:"status"`
}

type imageHandleTaskResponse struct {
	TaskID         string                 `json:"task_id"`
	ProviderTaskID string                 `json:"provider_task_id"`
	ClientTaskID   string                 `json:"client_task_id"`
	Status         string                 `json:"status"`
	Progress       string                 `json:"progress"`
	Result         *imageHandleResult     `json:"result,omitempty"`
	Usage          *imageHandleUsage      `json:"usage,omitempty"`
	Error          *imageHandleError      `json:"error,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
}

type imageHandleBatchQueryResponse struct {
	Data []imageHandleTaskResponse `json:"data"`
}

type imageHandleResult struct {
	Images []imageHandleImage `json:"images,omitempty"`
}

type imageHandleImage struct {
	URL string `json:"url"`
}

type imageHandleUsage struct {
	TotalTokens int `json:"total_tokens,omitempty"`
	ActualQuota int `json:"actual_quota,omitempty"`
}

type imageHandleError struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = service.GetImageHandleSubmitBaseURL()
	a.apiKey = service.GetImageHandleSubmitAPIKey()
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionImageGeneration)
	if taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Model) != "" {
		info.OriginModelName = strings.TrimSpace(req.Model)
		if info.ChannelMeta != nil {
			info.UpstreamModelName = info.OriginModelName
		}
	}
	if strings.TrimSpace(req.ClientTaskID) != "" {
		clientTaskID := strings.TrimSpace(req.ClientTaskID)
		if !clientTaskIDPattern.MatchString(clientTaskID) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("client_task_id must match ^task_[A-Za-z0-9_-]{1,186}$"), "invalid_request", http.StatusBadRequest)
		}
		if info.TaskRelayInfo != nil {
			info.PublicTaskID = clientTaskID
		}
		req.ClientTaskID = clientTaskID
	}
	action := constant.TaskActionImageGeneration
	if strings.EqualFold(req.Mode, "edit") || metadataString(req.Metadata, "operation", "") == "edit" || len(req.Images) > 0 || req.Image != "" {
		action = constant.TaskActionImageEdit
	}
	info.Action = action
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateExecutorConfig(info *relaycommon.RelayInfo) error {
	if err := service.ValidateImageHandleExecutorConfig(); err != nil {
		return err
	}
	return validateImageHandleChannelSecrets(info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if err := service.ValidateImageHandleExecutorConfig(); err != nil {
		return "", err
	}
	return a.baseURL + "/v1/image/tasks", nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	if info.PublicTaskID == "" {
		info.PublicTaskID = model.GenerateTaskID()
	}

	metadata := copyMetadata(taskReq.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["tenant_id"] = fmt.Sprintf("user_%d", info.UserId)
	metadata["channel_id"] = fmt.Sprintf("channel_%d", info.ChannelId)

	images := append([]string{}, taskReq.Images...)
	if len(images) == 0 && strings.TrimSpace(taskReq.Image) != "" {
		images = []string{strings.TrimSpace(taskReq.Image)}
	}
	mask := metadataStringPtr(metadata, "mask")

	parameters := extractParameters(taskReq, metadata)
	providerOptions := extractProviderOptions(metadata)
	operation := "generation"
	if info.Action == constant.TaskActionImageEdit {
		operation = "edit"
	}

	executorCfg := service.GetImageHandleExecutorConfig()
	if err := validateImageHandleChannelSecrets(info); err != nil {
		return nil, err
	}
	callbackBase := strings.TrimRight(service.ImageHandleCallbackAddress(), "/")
	callbackSecretID := fmt.Sprintf("channel_%d", info.ChannelId)
	payload := imageHandleSubmitRequest{
		RequestID:       c.GetString(common.RequestIdKey),
		ClientTaskID:    info.PublicTaskID,
		Model:           info.UpstreamModelName,
		Operation:       operation,
		Input:           imageHandleInput{Text: taskReq.Prompt, Images: images, Mask: mask},
		Parameters:      parameters,
		ProviderOptions: providerOptions,
		Executor: imageHandleExecutor{
			Type:       "new_api_internal",
			ExecuteURL: service.BuildImageHandleExecuteURL(info.PublicTaskID),
			SecretID:   executorCfg.InternalSecretID,
		},
		Callback: imageHandleCallback{
			URL:      fmt.Sprintf("%s/api/task/callback/external-image/%s", callbackBase, info.PublicTaskID),
			BatchURL: fmt.Sprintf("%s/api/task/callback/external-image/batch", callbackBase),
			SecretID: callbackSecretID,
		},
		Metadata: metadata,
	}

	body, err := common.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_image_handle_request_failed")
	}
	return bytes.NewReader(body), nil
}

func validateImageHandleChannelSecrets(info *relaycommon.RelayInfo) error {
	callbackSecret := resolveImageHandleSubmitCallbackSecret(info)
	if callbackSecret == "" {
		return fmt.Errorf("callback_secret is required for image-handle callbacks; configure it in async image executor settings or image channel settings")
	}
	if service.GetImageHandleExecutorConfig().InternalSecret == callbackSecret {
		return fmt.Errorf("image-handle internal execute secret and callback_secret must be different")
	}
	return nil
}

func resolveImageHandleSubmitCallbackSecret(info *relaycommon.RelayInfo) string {
	if info != nil && info.ChannelMeta != nil {
		if secret := strings.TrimSpace(info.ChannelOtherSettings.CallbackSecret); secret != "" {
			return secret
		}
	}
	return strings.TrimSpace(service.GetImageHandleExecutorConfig().CallbackSecret)
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var submitResp imageHandleSubmitResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if submitResp.ProviderTaskID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("provider_task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	if submitResp.ClientTaskID == "" {
		submitResp.ClientTaskID = info.PublicTaskID
	}
	if submitResp.Status == "" {
		submitResp.Status = "queued"
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id": info.PublicTaskID,
		"status":  submitResp.Status,
	})
	return submitResp.ProviderTaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	if err := service.ValidateImageHandleSubmitConfig(); err != nil {
		return nil, err
	}
	baseUrl = service.GetImageHandleSubmitBaseURL()
	key = service.GetImageHandleSubmitAPIKey()
	if ids, ok := body["task_ids"].([]string); ok && len(ids) > 0 {
		payload, err := common.Marshal(map[string]any{"task_ids": ids})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(http.MethodPost, baseUrl+"/v1/image/tasks/query", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		client, err := service.GetHttpClientWithProxy(proxy)
		if err != nil {
			return nil, fmt.Errorf("new proxy http client failed: %w", err)
		}
		return client.Do(req)
	}

	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, baseUrl+"/v1/image/tasks/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var taskResp imageHandleTaskResponse
	if err := common.Unmarshal(respBody, &taskResp); err != nil {
		var batch imageHandleBatchQueryResponse
		if batchErr := common.Unmarshal(respBody, &batch); batchErr == nil && len(batch.Data) > 0 {
			taskResp = batch.Data[0]
		} else {
			return nil, errors.Wrap(err, "unmarshal image-handle task result failed")
		}
	}
	return taskResponseToTaskInfo(taskResp), nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{}
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult != nil && taskResult.CompletionTokens > 0 {
		return taskResult.CompletionTokens
	}
	return 0
}

func (a *TaskAdaptor) ParseBatchTaskResult(respBody []byte) (map[string]*relaycommon.TaskInfo, error) {
	var batch imageHandleBatchQueryResponse
	if err := common.Unmarshal(respBody, &batch); err != nil {
		return nil, errors.Wrap(err, "unmarshal image-handle batch query response failed")
	}
	result := make(map[string]*relaycommon.TaskInfo, len(batch.Data))
	for _, item := range batch.Data {
		id := item.ProviderTaskID
		if id == "" {
			id = item.TaskID
		}
		if id == "" {
			continue
		}
		result[id] = taskResponseToTaskInfo(item)
	}
	return result, nil
}

func taskResponseToTaskInfo(item imageHandleTaskResponse) *relaycommon.TaskInfo {
	info := &relaycommon.TaskInfo{
		TaskID:   item.ProviderTaskID,
		Progress: item.Progress,
	}
	if info.TaskID == "" {
		info.TaskID = item.TaskID
	}
	switch strings.ToLower(item.Status) {
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
		info.Status = item.Status
	}
	if info.Progress == "" {
		switch info.Status {
		case model.TaskStatusSuccess, model.TaskStatusFailure:
			info.Progress = taskcommon.ProgressComplete
		case model.TaskStatusInProgress:
			info.Progress = taskcommon.ProgressInProgress
		case model.TaskStatusQueued:
			info.Progress = taskcommon.ProgressQueued
		}
	}
	if item.Result != nil && len(item.Result.Images) > 0 {
		info.Url = item.Result.Images[0].URL
	}
	if item.Usage != nil {
		info.TotalTokens = item.Usage.TotalTokens
		info.CompletionTokens = item.Usage.ActualQuota
	}
	if item.Error != nil {
		info.Reason = item.Error.Message
		if info.Reason == "" {
			info.Reason = item.Error.Code
		}
	}
	return info
}

func copyMetadata(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func extractParameters(req relaycommon.TaskSubmitReq, metadata map[string]interface{}) map[string]any {
	params := map[string]any{}
	if req.Size != "" {
		params["size"] = req.Size
	}
	for _, key := range []string{"quality", "n", "output_format", "output_compression"} {
		if v, ok := metadata[key]; ok {
			params[key] = v
		}
	}
	return params
}

func extractProviderOptions(metadata map[string]interface{}) map[string]any {
	v, ok := metadata["provider_options"]
	if !ok {
		return nil
	}
	if opts, ok := v.(map[string]any); ok {
		return opts
	}
	return nil
}

func metadataString(metadata map[string]interface{}, key string, fallback string) string {
	v, ok := metadata[key]
	if !ok {
		return fallback
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return fallback
}

func metadataStringPtr(metadata map[string]interface{}, key string) *string {
	v := metadataString(metadata, key, "")
	if v == "" {
		return nil
	}
	return &v
}
