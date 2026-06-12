package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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

const ChannelName = "xai"

var ModelList = []string{
	"grok-imagine-video",
}

type requestPayload map[string]any

type submitResponse struct {
	RequestID string `json:"request_id"`
	ID        string `json:"id,omitempty"`
}

type taskResponse struct {
	Status string `json:"status"`
	Code   string `json:"code,omitempty"`
	Video  *struct {
		URL               string `json:"url"`
		Duration          int    `json:"duration"`
		RespectModeration *bool  `json:"respect_moderation"`
	} `json:"video,omitempty"`
	Model string `json:"model,omitempty"`
	Usage *struct {
		CostInUSDTicks int64 `json:"cost_in_usd_ticks"`
	} `json:"usage,omitempty"`
	Progress *int   `json:"progress,omitempty"`
	Error    any    `json:"error,omitempty"`
	Msg      string `json:"message,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil || info.ChannelMeta == nil {
		return
	}
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	if a.baseURL == "" {
		a.baseURL = constant.ChannelBaseURLs[constant.ChannelTypeXai]
	}
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if info == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("relay info is nil"), "invalid_request", http.StatusBadRequest)
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	payload, err := readJSONPayload(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if modelName, ok := payload["model"].(string); ok && strings.TrimSpace(modelName) != "" {
		info.OriginModelName = strings.TrimSpace(modelName)
	}

	switch c.Request.URL.Path {
	case "/v1/videos/generations", "/v1/video/generations", "/v1/videos":
		info.Action = constant.TaskActionVideoGeneration
	case "/v1/videos/edits":
		info.Action = constant.TaskActionVideoEdit
	case "/v1/videos/extensions":
		info.Action = constant.TaskActionVideoExtension
	default:
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported xAI video path: %s", c.Request.URL.Path), "invalid_request", http.StatusBadRequest)
	}

	c.Set("xai_video_request", payload)
	return nil
}

func readJSONPayload(c *gin.Context) (requestPayload, error) {
	contentType := c.GetHeader("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("xAI video task only supports application/json requests")
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}
	var payload requestPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(getString(payload, "model")) == "" {
		return nil, fmt.Errorf("model field is required")
	}
	return payload, nil
}

func getPayloadFromContext(c *gin.Context) (requestPayload, error) {
	if v, ok := c.Get("xai_video_request"); ok {
		if payload, ok := v.(requestPayload); ok {
			return payload, nil
		}
	}
	return readJSONPayload(c)
}

func getString(payload requestPayload, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return value
}

func normalizeXAIVideoModel(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if strings.HasPrefix(modelName, "grok-imagine-video-1.5-preview") ||
		strings.HasPrefix(modelName, "grok-imagine-video-1.5-2026-05-30") {
		return "grok-imagine-video-1.5-preview"
	}
	if strings.HasPrefix(modelName, "grok-imagine-video") {
		return "grok-imagine-video"
	}
	return modelName
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	action := ""
	if info != nil && info.TaskRelayInfo != nil {
		action = info.Action
	}
	switch action {
	case constant.TaskActionVideoGeneration:
		return fmt.Sprintf("%s/v1/videos/generations", a.baseURL), nil
	case constant.TaskActionVideoEdit:
		return fmt.Sprintf("%s/v1/videos/edits", a.baseURL), nil
	case constant.TaskActionVideoExtension:
		return fmt.Sprintf("%s/v1/videos/extensions", a.baseURL), nil
	default:
		return "", fmt.Errorf("unsupported xAI video action: %s", action)
	}
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	payload, err := getPayloadFromContext(c)
	if err != nil {
		return nil, err
	}
	upstreamModel := ""
	if info != nil && info.ChannelMeta != nil {
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	if upstreamModel == "" && info != nil {
		upstreamModel = info.OriginModelName
	}
	upstreamModel = normalizeXAIVideoModel(upstreamModel)
	if upstreamModel != "" {
		payload["model"] = upstreamModel
	}

	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
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

	var submit submitResponse
	if err := common.Unmarshal(responseBody, &submit); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	upstreamID := strings.TrimSpace(submit.RequestID)
	if upstreamID == "" {
		upstreamID = strings.TrimSpace(submit.ID)
	}
	if upstreamID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	publicTaskID := ""
	if info != nil && info.TaskRelayInfo != nil {
		publicTaskID = info.PublicTaskID
	}
	publicResponse := submitResponse{
		RequestID: publicTaskID,
		ID:        publicTaskID,
	}
	c.JSON(http.StatusOK, publicResponse)
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var res taskResponse
	if err := common.Unmarshal(respBody, &res); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{Code: 0}
	if res.Status == "" {
		if reason := xaiTaskErrorMessage(res); reason != "" {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Reason = reason
			if res.Progress != nil {
				taskResult.Progress = fmt.Sprintf("%d%%", *res.Progress)
			}
			return &taskResult, nil
		}
	}
	switch strings.ToLower(res.Status) {
	case "pending":
		taskResult.Status = model.TaskStatusInProgress
	case "done", "completed":
		if res.Video == nil || strings.TrimSpace(res.Video.URL) == "" {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Reason = "video url is empty"
			if res.Video != nil && res.Video.RespectModeration != nil && !*res.Video.RespectModeration {
				taskResult.Reason = "video rejected by moderation"
			}
			break
		}
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = res.Video.URL
	case "failed", "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Reason = "task failed"
		if strings.ToLower(res.Status) == "expired" {
			taskResult.Reason = "task expired"
		}
		if reason := xaiTaskErrorMessage(res); reason != "" {
			taskResult.Reason = reason
		}
	default:
		return &taskResult, nil
	}
	if res.Progress != nil {
		taskResult.Progress = fmt.Sprintf("%d%%", *res.Progress)
	}
	return &taskResult, nil
}

func xaiTaskErrorMessage(res taskResponse) string {
	if strings.TrimSpace(res.Msg) != "" {
		return strings.TrimSpace(res.Msg)
	}
	switch err := res.Error.(type) {
	case string:
		return strings.TrimSpace(err)
	case map[string]any:
		for _, key := range []string{"message", "error", "detail", "msg"} {
			if value, ok := err[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	if strings.TrimSpace(res.Code) != "" && res.Error != nil {
		return strings.TrimSpace(res.Code)
	}
	return ""
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	openAIVideo := task.ToOpenAIVideo()
	if task.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: task.FailReason,
			Code:    "video_generation_failed",
		}
	}
	return common.Marshal(openAIVideo)
}
