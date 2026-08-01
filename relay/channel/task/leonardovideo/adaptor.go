package leonardovideo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
	proxy   string
}

const submissionTimeout = 10 * time.Minute

var (
	_ channel.TaskAdaptor                = (*TaskAdaptor)(nil)
	_ channel.NormalizedVideoTaskAdaptor = (*TaskAdaptor)(nil)
	_ channel.VideoBillingEstimator      = (*TaskAdaptor)(nil)
	_ channel.VideoContentResolver       = (*TaskAdaptor)(nil)
)

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	a.apiKey = strings.TrimSpace(info.ApiKey)
	a.baseURL = strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	a.proxy = info.ChannelSetting.Proxy
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(_ *gin.Context, _ *relaycommon.RelayInfo) *dto.TaskError {
	return requestError("only the normalized /v1/video/tasks endpoint is supported", "unsupported_video_endpoint", http.StatusBadRequest)
}

func (a *TaskAdaptor) PrepareNormalizedVideoRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.VideoTaskCreateRequest) *dto.TaskError {
	if info == nil {
		return requestError("relay info is required", "invalid_request", http.StatusBadRequest)
	}
	if strings.ToLower(strings.TrimSpace(request.Operation)) != "generation" {
		return requestError("only generation is supported", "unsupported_video_operation", http.StatusBadRequest)
	}
	if request.Input.Video != nil {
		return requestError("input.video is not supported for generation", "unsupported_video_input", http.StatusBadRequest)
	}
	prompt := strings.TrimSpace(request.Input.Prompt)
	if prompt == "" {
		return requestError("prompt is required", "invalid_video_parameter", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(prompt) > maxVideoPromptRunes {
		return requestError("prompt must not exceed 1200 characters", "invalid_video_parameter", http.StatusBadRequest)
	}
	if request.Output.Duration == nil {
		return requestError("duration is required", "video_duration_required", http.StatusBadRequest)
	}
	duration := *request.Output.Duration
	if duration < 4 || duration > 15 {
		return requestError("duration must be an integer between 4 and 15 seconds", "invalid_video_duration", http.StatusBadRequest)
	}
	if request.Output.Resolution != nil {
		return requestError("resolution is selected by the exact model mapping", "invalid_video_parameter", http.StatusBadRequest)
	}
	aspectRatio := defaultAspectRatio
	if request.Output.AspectRatio != nil {
		aspectRatio = strings.TrimSpace(*request.Output.AspectRatio)
	}
	if _, ok := supportedAspectRatios[aspectRatio]; !ok {
		return requestError(fmt.Sprintf("aspect_ratio %q is not supported", aspectRatio), "invalid_video_aspect_ratio", http.StatusBadRequest)
	}

	var generateAudio *bool
	for namespace, options := range request.ProviderOptions {
		if strings.TrimSpace(namespace) != ProviderOptionsNamespace {
			return requestError(fmt.Sprintf("provider_options.%s is not supported", namespace), "invalid_provider_options", http.StatusBadRequest)
		}
		for key, value := range options {
			if key != "generate_audio" {
				return requestError(fmt.Sprintf("provider_options.%s.%s is not supported", namespace, key), "invalid_provider_options", http.StatusBadRequest)
			}
			enabled, ok := value.(bool)
			if !ok {
				return requestError("provider_options.leonardo_video.generate_audio must be a boolean", "invalid_provider_options", http.StatusBadRequest)
			}
			generateAudio = &enabled
		}
	}

	images := make([]dto.VideoTaskSource, 0, 1+len(request.Input.ReferenceImages))
	if request.Input.Image != nil {
		images = append(images, *request.Input.Image)
	}
	images = append(images, request.Input.ReferenceImages...)
	if len(images) > 4 {
		return requestError("at most 4 image references are supported", "reference_image_limit_exceeded", http.StatusBadRequest)
	}
	if len(request.Input.ReferenceVideos) > 3 {
		return requestError("at most 3 video references are supported", "reference_video_limit_exceeded", http.StatusBadRequest)
	}
	if len(images)+len(request.Input.ReferenceVideos) > 7 {
		return requestError("at most 7 image and video references are supported", "reference_limit_exceeded", http.StatusBadRequest)
	}
	if len(request.Input.ReferenceAudios) > 0 {
		return requestError("audio references are not supported", "unsupported_reference_audio", http.StatusBadRequest)
	}
	mode := strings.ToLower(strings.TrimSpace(request.Input.ReferenceMode))
	if mode != "" && mode != "media" {
		return requestError("reference_mode must be media when provided", "unsupported_reference_mode", http.StatusBadRequest)
	}
	if mode == "" && (len(images) > 0 || len(request.Input.ReferenceVideos) > 0) {
		return requestError("reference_mode must be media when references are provided", "unsupported_reference_mode", http.StatusBadRequest)
	}

	names := make(map[string]struct{}, len(images)+len(request.Input.ReferenceVideos))
	normalize := func(source dto.VideoTaskSource, kind string) (referenceMedia, *dto.TaskError) {
		if strings.TrimSpace(source.Provider) != "" || strings.TrimSpace(source.FileID) != "" {
			return referenceMedia{}, requestError(fmt.Sprintf("%s references must use HTTP(S) URLs", kind), "unsupported_file_provider", http.StatusBadRequest)
		}
		value := strings.TrimSpace(source.URL)
		parsed, err := url.Parse(value)
		if err != nil || parsed.User != nil || parsed.Hostname() == "" {
			return referenceMedia{}, requestError(fmt.Sprintf("%s reference URL must be an absolute HTTP(S) URL", kind), "invalid_video_parameter", http.StatusBadRequest)
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return referenceMedia{}, requestError(fmt.Sprintf("%s reference URL must be an absolute HTTP(S) URL", kind), "invalid_video_parameter", http.StatusBadRequest)
		}
		if portText := parsed.Port(); portText != "" {
			port, portErr := strconv.Atoi(portText)
			if portErr != nil || port < 1 || port > 65535 {
				return referenceMedia{}, requestError(fmt.Sprintf("%s reference URL contains an invalid port", kind), "invalid_video_parameter", http.StatusBadRequest)
			}
		}
		name := strings.TrimSpace(source.Name)
		if len(name) > 100 {
			return referenceMedia{}, requestError("reference names must not exceed 100 characters", "invalid_video_parameter", http.StatusBadRequest)
		}
		if name != "" {
			if _, exists := names[name]; exists {
				return referenceMedia{}, requestError("reference names must be unique", "invalid_video_parameter", http.StatusBadRequest)
			}
			names[name] = struct{}{}
		}
		return referenceMedia{URL: value, Name: name}, nil
	}

	payload := &normalizedRequest{Prompt: prompt, Duration: duration, AspectRatio: aspectRatio, GenerateAudio: generateAudio}
	for _, source := range images {
		item, taskErr := normalize(source, "image")
		if taskErr != nil {
			return taskErr
		}
		payload.ReferenceImages = append(payload.ReferenceImages, item)
	}
	for _, source := range request.Input.ReferenceVideos {
		item, taskErr := normalize(source, "video")
		if taskErr != nil {
			return taskErr
		}
		payload.ReferenceVideos = append(payload.ReferenceVideos, item)
	}

	info.Action = constant.TaskActionVideoGeneration
	info.OriginModelName = request.Model
	c.Set(videoRequestContextKey, payload)
	return nil
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	modelName := providerModel(info)
	if _, ok := supportedModels[modelName]; !ok {
		return requestError(fmt.Sprintf("unsupported video model mapping %q", modelName), "unsupported_video_model", http.StatusBadRequest)
	}
	payload, err := normalizedPayload(c)
	if err != nil {
		return requestError(err.Error(), "invalid_request", http.StatusBadRequest)
	}
	if payload.Duration < 4 || payload.Duration > 15 {
		return requestError("duration must be an integer between 4 and 15 seconds", "invalid_video_duration", http.StatusBadRequest)
	}
	if _, ok := supportedAspectRatios[payload.AspectRatio]; !ok {
		return requestError(fmt.Sprintf("aspect_ratio %q is not supported", payload.AspectRatio), "invalid_video_aspect_ratio", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) ResolveVideoBilling(c *gin.Context, info *relaycommon.RelayInfo) (channel.VideoBillingEstimate, *dto.TaskError) {
	if taskErr := a.ValidateNormalizedVideoModel(c, info); taskErr != nil {
		return channel.VideoBillingEstimate{}, taskErr
	}
	payload, err := normalizedPayload(c)
	if err != nil {
		return channel.VideoBillingEstimate{}, requestError(err.Error(), "invalid_request", http.StatusBadRequest)
	}
	return channel.VideoBillingEstimate{Seconds: payload.Duration, Basis: types.VideoPricingBasisGeneration}, nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", fmt.Errorf("Leonardo video channel base URL is required")
	}
	return a.baseURL + "/v1/videos", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if info != nil && info.TaskRelayInfo != nil && strings.TrimSpace(info.TaskRelayInfo.PublicTaskID) != "" {
		req.Header.Set("Idempotency-Key", strings.TrimSpace(info.TaskRelayInfo.PublicTaskID))
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	payload, err := normalizedPayload(c)
	if err != nil {
		return nil, err
	}
	modelName := providerModel(info)
	if _, ok := supportedModels[modelName]; !ok {
		return nil, fmt.Errorf("unsupported Leonardo video provider model %q", modelName)
	}
	images := make([]referenceMedia, len(payload.ReferenceImages))
	copy(images, payload.ReferenceImages)
	videos := make([]referenceMedia, len(payload.ReferenceVideos))
	copy(videos, payload.ReferenceVideos)
	body, err := common.Marshal(upstreamRequest{
		Model: modelName, Prompt: payload.Prompt, Duration: payload.Duration,
		AspectRatio: payload.AspectRatio, GenerateAudio: payload.GenerateAudio,
		Public: false, Seed: -1, ImageReferences: images, VideoReferences: videos,
	})
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}
	endpoint, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	client, err := service.GetHttpClientWithProxy(a.proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	parent := context.Background()
	if c != nil && c.Request != nil {
		parent = context.WithoutCancel(c.Request.Context())
	}
	ctx, cancel := context.WithTimeout(parent, submissionTimeout)
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestErr
		}
		if requestErr = a.BuildRequestHeader(c, req, info); requestErr != nil {
			return nil, requestErr
		}
		resp, requestErr := client.Do(req)
		if requestErr == nil || resp != nil || attempt == 1 || ctx.Err() != nil {
			return resp, requestErr
		}
	}
	return nil, fmt.Errorf("Leonardo video request failed")
}

func (a *TaskAdaptor) DoResponse(_ *gin.Context, resp *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()
	var response responsePayload
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrap(err, "unmarshal video response failed"), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	taskID := firstNonEmpty(response.TaskID, response.ID)
	if taskID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("video task id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	return taskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("Leonardo video channel base URL is required")
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/videos/"+url.PathEscape(strings.TrimSpace(taskID)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var response responsePayload
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, errors.Wrap(err, "unmarshal Leonardo video task result failed")
	}
	result := &relaycommon.TaskInfo{Code: 0}
	if response.Progress > 0 {
		result.Progress = fmt.Sprintf("%d%%", max(0, min(100, response.Progress)))
	}
	taskID := firstNonEmpty(response.TaskID, response.ID)
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "submitting", "queued", "pending":
		result.Status = model.TaskStatusQueued
	case "in_progress", "processing", "running":
		result.Status = model.TaskStatusInProgress
	case "completed", "succeeded", "success":
		if taskID == "" {
			result.Status = model.TaskStatusFailure
			result.Reason = "Video task completed without a task reference"
			break
		}
		result.Status = model.TaskStatusSuccess
		// Leonardo2API stores progress internally as a 0-1 fraction. A
		// terminal success is complete even when an older upstream response
		// still reports that fraction as the public progress value.
		result.Progress = taskcommon.ProgressComplete
		output := relaycommon.VideoOutput{Index: 0, MimeType: "video/mp4", Filename: taskID + ".mp4", DurationMS: int64(response.Duration) * 1000}
		if directURL := validCDNURL(response.VideoURL); directURL != "" {
			output.URL = directURL
		} else {
			output.ProviderReference = taskID
			output.Resolver = videoContentResolver
		}
		result.VideoOutputs = []relaycommon.VideoOutput{output}
	case "failed", "failure", "cancelled", "canceled":
		result.Status = model.TaskStatusFailure
		result.Reason = responseErrorMessage(response.Error)
	case "submission_unknown":
		result.Status = model.TaskStatusFailure
		result.Reason = "The submission result could not be confirmed. Review the original task before submitting another request."
	default:
		if response.Error != nil {
			result.Status = model.TaskStatusFailure
			result.Reason = responseErrorMessage(response.Error)
			break
		}
		return nil, fmt.Errorf("unknown video task status %q", response.Status)
	}
	return result, nil
}

func (a *TaskAdaptor) ResolveVideoContent(ctx context.Context, providerChannel *model.Channel, task *model.Task, output relaycommon.VideoOutput, headers http.Header) (*http.Response, error) {
	if providerChannel == nil || task == nil {
		return nil, fmt.Errorf("video content context is incomplete")
	}
	if output.Resolver != "" && output.Resolver != videoContentResolver {
		return nil, fmt.Errorf("unsupported video content resolver %q", output.Resolver)
	}
	taskID := strings.TrimSpace(output.ProviderReference)
	if taskID == "" {
		taskID = strings.TrimSpace(task.GetUpstreamTaskID())
	}
	if taskID == "" {
		return nil, fmt.Errorf("video provider task reference is missing")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(providerChannel.GetBaseURL()), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("Leonardo video channel base URL is required")
	}
	key := strings.TrimSpace(task.PrivateData.Key)
	if key == "" {
		key = strings.TrimSpace(providerChannel.Key)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/videos/"+url.PathEscape(taskID)+"/content", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "video/*")
	for _, name := range []string{"Range", "If-Range"} {
		if value := headers.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	client, err := service.GetHttpClientWithProxy(providerChannel.GetSetting().Proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string { return append([]string(nil), ModelList...) }

func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

func normalizedPayload(c *gin.Context) (*normalizedRequest, error) {
	value, ok := c.Get(videoRequestContextKey)
	if !ok {
		return nil, fmt.Errorf("normalized Leonardo video request is missing")
	}
	payload, ok := value.(*normalizedRequest)
	if !ok || payload == nil {
		return nil, fmt.Errorf("normalized Leonardo video request is invalid")
	}
	return payload, nil
}

func providerModel(info *relaycommon.RelayInfo) string {
	if info != nil {
		if value := strings.TrimSpace(info.UpstreamModelName); value != "" {
			return value
		}
		return strings.TrimSpace(info.OriginModelName)
	}
	return ""
}

func requestError(message, code string, status int) *dto.TaskError {
	return service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), code, status)
}

func responseErrorMessage(response *responseError) string {
	if response == nil {
		return "Video task failed"
	}
	code := strings.TrimSpace(response.Code)
	message := strings.TrimSpace(response.Message)
	if message != "" {
		switch code {
		case "content_moderated", "invalid_reference_media_duration", "reference_media_duration_exceeded":
			return message
		}
	}
	return "Video task failed"
}

func validCDNURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "cdn.leonardo.ai") {
		return ""
	}
	if net.ParseIP(parsed.Hostname()) != nil || (parsed.Port() != "" && parsed.Port() != "443") {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
