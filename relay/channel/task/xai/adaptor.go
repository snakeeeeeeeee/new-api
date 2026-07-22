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
	"grok-imagine-video-1.5",
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

func (a *TaskAdaptor) PrepareNormalizedVideoRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.VideoTaskCreateRequest) *dto.TaskError {
	if info == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("relay info is nil"), "invalid_request", http.StatusBadRequest)
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	for namespace := range request.ProviderOptions {
		if !strings.EqualFold(strings.TrimSpace(namespace), ChannelName) {
			return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.%s is not supported by xAI", namespace), "invalid_provider_options", http.StatusBadRequest)
		}
	}

	payload := requestPayload{"model": request.Model, "prompt": request.Input.Prompt}
	switch request.Operation {
	case "generation":
		info.Action = constant.TaskActionVideoGeneration
		if request.Input.Video != nil {
			return xaiUnsupportedVideoInput("xAI generation does not accept input.video")
		}
		if request.Input.Image != nil && len(request.Input.ReferenceImages) > 0 {
			return xaiUnsupportedVideoInput("xAI image and reference_images generation modes are mutually exclusive")
		}
		if len(request.Input.ReferenceImages) > 7 {
			return xaiUnsupportedVideoInput("xAI reference_images supports at most 7 images")
		}
		if request.Output.Duration != nil {
			if *request.Output.Duration < 1 || *request.Output.Duration > 15 {
				return xaiNormalizedVideoError("duration must be between 1 and 15 seconds")
			}
			if len(request.Input.ReferenceImages) > 0 && *request.Output.Duration > 10 {
				return xaiNormalizedVideoError("xAI reference-image generation duration must be between 1 and 10 seconds")
			}
			payload["duration"] = *request.Output.Duration
		}
		if request.Output.AspectRatio != nil {
			if !validXAIAspectRatio(*request.Output.AspectRatio) {
				return xaiNormalizedVideoError("aspect_ratio is not supported by xAI")
			}
			payload["aspect_ratio"] = *request.Output.AspectRatio
		}
		if request.Output.Resolution != nil {
			if !validXAIResolution(*request.Output.Resolution) {
				return xaiNormalizedVideoError("resolution must be 480p, 720p, or 1080p")
			}
			payload["resolution"] = *request.Output.Resolution
		}
		if request.Input.Image != nil {
			source, taskErr := normalizedXAISource(*request.Input.Image)
			if taskErr != nil {
				return taskErr
			}
			payload["image"] = source
		}
		if len(request.Input.ReferenceImages) > 0 {
			references := make([]map[string]any, 0, len(request.Input.ReferenceImages))
			for _, input := range request.Input.ReferenceImages {
				source, taskErr := normalizedXAISource(input)
				if taskErr != nil {
					return taskErr
				}
				references = append(references, source)
			}
			payload["reference_images"] = references
		}
	case "edit":
		info.Action = constant.TaskActionVideoEdit
		if request.Input.Video == nil {
			return xaiUnsupportedVideoInput("xAI edit requires input.video")
		}
		if request.Input.Image != nil || len(request.Input.ReferenceImages) > 0 {
			return xaiUnsupportedVideoInput("xAI edit does not accept image inputs")
		}
		if request.Output.Duration != nil || request.Output.AspectRatio != nil || request.Output.Resolution != nil {
			return xaiNormalizedVideoError("xAI edit inherits duration, aspect ratio, and resolution from the input video")
		}
		source, taskErr := normalizedXAISource(*request.Input.Video)
		if taskErr != nil {
			return taskErr
		}
		payload["video"] = source
	case "extension":
		info.Action = constant.TaskActionVideoExtension
		if request.Input.Video == nil {
			return xaiUnsupportedVideoInput("xAI extension requires input.video")
		}
		if request.Input.Image != nil || len(request.Input.ReferenceImages) > 0 {
			return xaiUnsupportedVideoInput("xAI extension does not accept image inputs")
		}
		if request.Output.AspectRatio != nil || request.Output.Resolution != nil {
			return xaiNormalizedVideoError("extension inherits aspect ratio and resolution from the input video")
		}
		if request.Output.Duration != nil {
			if *request.Output.Duration < 2 || *request.Output.Duration > 10 {
				return xaiNormalizedVideoError("extension duration must be between 2 and 10 seconds")
			}
			payload["duration"] = *request.Output.Duration
		}
		source, taskErr := normalizedXAISource(*request.Input.Video)
		if taskErr != nil {
			return taskErr
		}
		payload["video"] = source
	case "remix":
		return service.TaskErrorWrapperLocal(fmt.Errorf("xAI adaptor does not support normalized remix"), "unsupported_video_operation", http.StatusBadRequest)
	default:
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported normalized video operation: %s", request.Operation), "unsupported_video_operation", http.StatusBadRequest)
	}

	if options := request.ProviderOptions[ChannelName]; options != nil {
		for key, value := range options {
			switch key {
			case "model", "prompt", "image", "reference_images", "video", "duration", "aspect_ratio", "resolution":
				return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.xai.%s duplicates a public field", key), "invalid_provider_options", http.StatusBadRequest)
			default:
				payload[key] = value
			}
		}
	}
	info.OriginModelName = request.Model
	c.Set("xai_video_request", payload)
	return nil
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	payload, err := getPayloadFromContext(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	modelName := ""
	if info != nil {
		modelName = firstNonEmpty(info.UpstreamModelName, info.OriginModelName)
	}
	if _, hasReferences := payload["reference_images"]; hasReferences && isXAI15VideoModel(modelName) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("xAI model %s does not support reference-image generation", modelName), "unsupported_video_model_capability", http.StatusBadRequest)
	}

	resolution := strings.ToLower(strings.TrimSpace(getString(payload, "resolution")))
	if resolution != "1080p" {
		return nil
	}
	if !strings.Contains(strings.ToLower(modelName), "1.5") {
		return service.TaskErrorWrapperLocal(fmt.Errorf("1080p requires an xAI 1.5 video model"), "unsupported_video_resolution", http.StatusBadRequest)
	}
	if info == nil || info.Action != constant.TaskActionVideoGeneration {
		return service.TaskErrorWrapperLocal(fmt.Errorf("1080p is only supported for xAI image-to-video generation"), "unsupported_video_resolution", http.StatusBadRequest)
	}
	if image, ok := payload["image"]; !ok || image == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("1080p is only supported for xAI image-to-video generation"), "unsupported_video_resolution", http.StatusBadRequest)
	}
	return nil
}

func normalizedXAISource(source dto.VideoTaskSource) (map[string]any, *dto.TaskError) {
	if source.URL != "" {
		return map[string]any{"url": source.URL}, nil
	}
	if !strings.EqualFold(source.Provider, ChannelName) {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("file reference provider %q is not supported by xAI", source.Provider), "unsupported_file_provider", http.StatusBadRequest)
	}
	return map[string]any{"file_id": source.FileID}, nil
}

func validXAIAspectRatio(value string) bool {
	switch value {
	case "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3":
		return true
	default:
		return false
	}
}

func validXAIResolution(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480p", "720p", "1080p":
		return true
	default:
		return false
	}
}

func xaiNormalizedVideoError(message string) *dto.TaskError {
	return service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), "invalid_video_parameter", http.StatusBadRequest)
}

func xaiUnsupportedVideoInput(message string) *dto.TaskError {
	return service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), "unsupported_video_input", http.StatusBadRequest)
}

func isXAI15VideoModel(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "grok-imagine-video-1.5")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
	if _, normalized := c.Get(relaycommon.VideoTaskPublicRequestContextKey); !normalized {
		c.JSON(http.StatusOK, publicResponse)
	}
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
		taskResult.VideoOutputs = []relaycommon.VideoOutput{{
			Index: 0, URL: res.Video.URL, MimeType: "video/mp4",
			DurationMS: int64(res.Video.Duration) * 1000,
		}}
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
	if task.Status == model.TaskStatusSuccess {
		openAIVideo.SetMetadata("url", taskcommon.BuildProxyURL(task.TaskID))
	}
	if task.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: task.FailReason,
			Code:    "video_generation_failed",
		}
	}
	return common.Marshal(openAIVideo)
}
