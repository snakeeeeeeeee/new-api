package adobevideo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type requestPayload struct {
	Model         string `json:"model"`
	Prompt        string `json:"prompt"`
	Duration      int    `json:"duration"`
	AspectRatio   string `json:"aspect_ratio"`
	GenerateAudio *bool  `json:"generate_audio,omitempty"`
}

type responseError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type responsePayload struct {
	ID            string         `json:"id"`
	TaskID        string         `json:"task_id"`
	Model         string         `json:"model"`
	Status        string         `json:"status"`
	Progress      int            `json:"progress"`
	Duration      int            `json:"duration"`
	AspectRatio   string         `json:"aspect_ratio"`
	Resolution    string         `json:"resolution"`
	GenerateAudio *bool          `json:"generate_audio,omitempty"`
	VideoURL      string         `json:"video_url,omitempty"`
	Error         *responseError `json:"error,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

var (
	_ channel.TaskAdaptor                = (*TaskAdaptor)(nil)
	_ channel.NormalizedVideoTaskAdaptor = (*TaskAdaptor)(nil)
	_ channel.VideoBillingEstimator      = (*TaskAdaptor)(nil)
	_ channel.VideoContentResolver       = (*TaskAdaptor)(nil)
)

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil || info.ChannelMeta == nil {
		return
	}
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(_ *gin.Context, _ *relaycommon.RelayInfo) *dto.TaskError {
	return service.TaskErrorWrapperLocal(
		fmt.Errorf("AdobeVideo only supports the normalized /v1/video/tasks endpoint"),
		"unsupported_video_endpoint",
		http.StatusBadRequest,
	)
}

func (a *TaskAdaptor) PrepareNormalizedVideoRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.VideoTaskCreateRequest) *dto.TaskError {
	if info == nil {
		return adobeVideoRequestError("relay info is nil", "invalid_request")
	}
	if request.Operation != "generation" {
		return adobeVideoRequestError("AdobeVideo only supports generation", "unsupported_video_operation")
	}
	if request.Input.Image != nil || len(request.Input.ReferenceImages) > 0 || request.Input.Video != nil {
		return adobeVideoRequestError("AdobeVideo generation currently supports text input only", "unsupported_video_input")
	}

	prompt := strings.TrimSpace(request.Input.Prompt)
	if prompt == "" {
		return adobeVideoRequestError("prompt is required", "invalid_video_parameter")
	}
	if request.Output.Duration == nil {
		return adobeVideoRequestError("duration is required", "video_duration_required")
	}
	duration := *request.Output.Duration
	if duration < minDurationSeconds || duration > maxDurationSeconds {
		return adobeVideoRequestError(
			fmt.Sprintf("duration must be between %d and %d seconds", minDurationSeconds, maxDurationSeconds),
			"invalid_video_duration",
		)
	}
	if request.Output.Resolution != nil {
		return adobeVideoRequestError(
			"resolution is selected by the exact AdobeVideo model mapping",
			"invalid_video_parameter",
		)
	}

	aspectRatio := defaultAspectRatio
	if request.Output.AspectRatio != nil {
		aspectRatio = strings.TrimSpace(*request.Output.AspectRatio)
	}
	if !validAspectRatio(aspectRatio) {
		return adobeVideoRequestError("aspect_ratio is not supported by AdobeVideo", "invalid_video_parameter")
	}

	payload := &requestPayload{
		Model:       request.Model,
		Prompt:      prompt,
		Duration:    duration,
		AspectRatio: aspectRatio,
	}
	for namespace, options := range request.ProviderOptions {
		if strings.TrimSpace(namespace) != ProviderOptionsNamespace {
			return adobeVideoRequestError(
				fmt.Sprintf("provider_options.%s is not supported by AdobeVideo", namespace),
				"invalid_provider_options",
			)
		}
		for key, value := range options {
			switch key {
			case "generate_audio":
				enabled, ok := value.(bool)
				if !ok {
					return adobeVideoRequestError(
						"provider_options.adobe_video.generate_audio must be a boolean",
						"invalid_provider_options",
					)
				}
				payload.GenerateAudio = &enabled
			case "model", "prompt", "duration", "aspect_ratio", "resolution":
				return adobeVideoRequestError(
					fmt.Sprintf("provider_options.adobe_video.%s duplicates a public or model-bound field", key),
					"invalid_provider_options",
				)
			default:
				return adobeVideoRequestError(
					fmt.Sprintf("provider_options.adobe_video.%s is not supported", key),
					"invalid_provider_options",
				)
			}
		}
	}

	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionVideoGeneration
	info.OriginModelName = request.Model
	c.Set(videoRequestContextKey, payload)
	return nil
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(_ *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	modelName := ""
	if info != nil && info.ChannelMeta != nil {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if modelName == "" && info != nil {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	if _, ok := supportedModels[modelName]; !ok {
		return adobeVideoRequestError(
			fmt.Sprintf("AdobeVideo model mapping must target an exact supported provider SKU, got %q", modelName),
			"unsupported_video_model",
		)
	}
	return nil
}

func (a *TaskAdaptor) ResolveVideoBilling(c *gin.Context, _ *relaycommon.RelayInfo) (channel.VideoBillingEstimate, *dto.TaskError) {
	payload, err := normalizedPayload(c)
	if err != nil {
		return channel.VideoBillingEstimate{}, adobeVideoRequestError(err.Error(), "invalid_request")
	}
	if payload.Duration < minDurationSeconds || payload.Duration > maxDurationSeconds {
		return channel.VideoBillingEstimate{}, adobeVideoRequestError(
			fmt.Sprintf("duration must be between %d and %d seconds", minDurationSeconds, maxDurationSeconds),
			"invalid_video_duration",
		)
	}
	return channel.VideoBillingEstimate{
		Seconds: payload.Duration,
		Basis:   types.VideoPricingBasisGeneration,
	}, nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", fmt.Errorf("AdobeVideo channel base URL is required")
	}
	return a.baseURL + "/v1/videos", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	payload, err := normalizedPayload(c)
	if err != nil {
		return nil, err
	}
	upstreamModel := ""
	if info != nil && info.ChannelMeta != nil {
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	if upstreamModel == "" && info != nil {
		upstreamModel = strings.TrimSpace(info.OriginModelName)
	}
	if _, ok := supportedModels[upstreamModel]; !ok {
		return nil, fmt.Errorf("unsupported AdobeVideo provider SKU %q", upstreamModel)
	}
	payload.Model = upstreamModel
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var response responsePayload
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", nil, service.TaskErrorWrapper(
			errors.Wrapf(err, "body: %s", responseBody),
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}
	taskID := firstNonEmpty(response.TaskID, response.ID)
	if taskID == "" {
		return "", nil, service.TaskErrorWrapper(
			fmt.Errorf("AdobeVideo task_id is empty"),
			"invalid_response",
			http.StatusInternalServerError,
		)
	}
	return taskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("AdobeVideo channel base URL is required")
	}
	request, err := http.NewRequest(http.MethodGet, baseURL+"/v1/videos/"+url.PathEscape(taskID), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")

	client, err := adobeVideoHTTPClient(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var response responsePayload
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal AdobeVideo task result failed")
	}

	result := &relaycommon.TaskInfo{Code: 0}
	if response.Progress > 0 {
		result.Progress = fmt.Sprintf("%d%%", min(response.Progress, 100))
	}
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "queued", "pending":
		result.Status = model.TaskStatusQueued
	case "in_progress", "processing", "running":
		result.Status = model.TaskStatusInProgress
	case "completed", "succeeded", "success":
		taskID := firstNonEmpty(response.TaskID, response.ID)
		if taskID == "" {
			result.Status = model.TaskStatusFailure
			result.Reason = "AdobeVideo completed without a task reference"
			break
		}
		result.Status = model.TaskStatusSuccess
		result.VideoOutputs = []relaycommon.VideoOutput{{
			Index:             0,
			ProviderReference: taskID,
			MimeType:          "video/mp4",
			Filename:          taskID + ".mp4",
			DurationMS:        int64(response.Duration) * 1000,
			Resolver:          videoContentResolver,
		}}
	case "failed", "failure", "cancelled", "canceled":
		result.Status = model.TaskStatusFailure
		result.Reason = responseErrorMessage(response.Error)
	default:
		if response.Error != nil {
			result.Status = model.TaskStatusFailure
			result.Reason = responseErrorMessage(response.Error)
		}
	}
	return result, nil
}

func (a *TaskAdaptor) ResolveVideoContent(ctx context.Context, providerChannel *model.Channel, task *model.Task, output relaycommon.VideoOutput, headers http.Header) (*http.Response, error) {
	if providerChannel == nil || task == nil {
		return nil, fmt.Errorf("AdobeVideo content context is incomplete")
	}
	if output.Resolver != "" && output.Resolver != videoContentResolver {
		return nil, fmt.Errorf("unsupported AdobeVideo content resolver %q", output.Resolver)
	}
	taskID := strings.TrimSpace(output.ProviderReference)
	if taskID == "" {
		taskID = strings.TrimSpace(task.GetUpstreamTaskID())
	}
	if taskID == "" {
		return nil, fmt.Errorf("AdobeVideo provider task reference is missing")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(providerChannel.GetBaseURL()), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("AdobeVideo channel base URL is required")
	}
	apiKey := strings.TrimSpace(task.PrivateData.Key)
	if apiKey == "" {
		apiKey = strings.TrimSpace(providerChannel.Key)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("AdobeVideo channel API key is missing")
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		baseURL+"/v1/videos/"+url.PathEscape(taskID)+"/content",
		nil,
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "video/*")
	for _, name := range []string{"Range", "If-Range"} {
		if value := headers.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}

	client, err := adobeVideoHTTPClient(providerChannel.GetSetting().Proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(request)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func normalizedPayload(c *gin.Context) (*requestPayload, error) {
	value, ok := c.Get(videoRequestContextKey)
	if !ok {
		return nil, fmt.Errorf("normalized AdobeVideo request is missing")
	}
	payload, ok := value.(*requestPayload)
	if !ok || payload == nil {
		return nil, fmt.Errorf("normalized AdobeVideo request is invalid")
	}
	return payload, nil
}

func validAspectRatio(value string) bool {
	switch value {
	case "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
		return true
	default:
		return false
	}
}

func adobeVideoRequestError(message, code string) *dto.TaskError {
	return service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), code, http.StatusBadRequest)
}

func responseErrorMessage(response *responseError) string {
	if response == nil || strings.TrimSpace(response.Message) == "" {
		return "AdobeVideo task failed"
	}
	return strings.TrimSpace(response.Message)
}

func adobeVideoHTTPClient(proxy string) (*http.Client, error) {
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	return client, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
