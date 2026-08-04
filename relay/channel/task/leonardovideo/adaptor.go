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

func (a *TaskAdaptor) OpenAIVideoCompatibility() channel.OpenAIVideoCompatibility {
	return channel.OpenAIVideoCompatibility{Generation: true, ModelBoundResolution: true}
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
	modelName := providerModel(info)
	isH3 := isMiniMaxH3Model(modelName) || isMiniMaxH3Model(request.Model)
	duration := *request.Output.Duration
	minimumDuration := 4
	if isH3 {
		minimumDuration = 5
	}
	if duration < minimumDuration || duration > 15 {
		return requestError(fmt.Sprintf("duration must be an integer between %d and 15 seconds", minimumDuration), "invalid_video_duration", http.StatusBadRequest)
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

	generateAudio := common.GetPointer(true)
	if request.Output.GenerateAudio != nil {
		generateAudio = request.Output.GenerateAudio
	}
	if isH3 && generateAudio != nil && !*generateAudio {
		return requestError("MiniMax H3 always generates native audio", "invalid_video_parameter", http.StatusBadRequest)
	}
	for namespace, options := range request.ProviderOptions {
		if strings.TrimSpace(namespace) != ProviderOptionsNamespace {
			return requestError(fmt.Sprintf("provider_options.%s is not supported", namespace), "invalid_provider_options", http.StatusBadRequest)
		}
		for key := range options {
			if strings.EqualFold(strings.TrimSpace(key), "generate_audio") {
				return requestError("provider_options.leonardo_video.generate_audio is no longer supported; use output.generate_audio", "invalid_provider_options", http.StatusBadRequest)
			}
			if strings.EqualFold(strings.TrimSpace(key), "reference_mode") {
				return requestError("provider_options.leonardo_video.reference_mode is no longer supported; use input.reference_mode", "invalid_provider_options", http.StatusBadRequest)
			}
			return requestError(fmt.Sprintf("provider_options.%s.%s is not supported", namespace, key), "invalid_provider_options", http.StatusBadRequest)
		}
	}

	images := make([]dto.VideoTaskSource, 0, 1+len(request.Input.ReferenceImages))
	if request.Input.Image != nil {
		images = append(images, *request.Input.Image)
	}
	images = append(images, request.Input.ReferenceImages...)
	videos := request.Input.ReferenceVideos
	audios := request.Input.ReferenceAudios
	mode := strings.ToLower(strings.TrimSpace(request.Input.ReferenceMode))
	if isH3 {
		if len(videos) > 0 {
			return requestError("MiniMax H3 does not support video references", "unsupported_reference_video", http.StatusBadRequest)
		}
		if len(images) > 5 {
			return requestError("at most 5 image references are supported", "reference_image_limit_exceeded", http.StatusBadRequest)
		}
		if len(audios) > 3 {
			return requestError("at most 3 audio references are supported", "reference_audio_limit_exceeded", http.StatusBadRequest)
		}
		if mode != "" && mode != "frame" && mode != "images" && mode != "media" {
			return requestError("reference_mode must be frame, images, or media for MiniMax H3", "unsupported_reference_mode", http.StatusBadRequest)
		}
		if (len(images) > 0 || len(audios) > 0) && mode == "" {
			return requestError("reference_mode is required when H3 references are provided", "unsupported_reference_mode", http.StatusBadRequest)
		}
		switch mode {
		case "frame":
			if len(audios) > 0 {
				return requestError("MiniMax H3 frame mode does not support audio references", "unsupported_reference_audio", http.StatusBadRequest)
			}
			if len(images) == 0 {
				return requestError("MiniMax H3 frame mode requires a start frame", "invalid_video_parameter", http.StatusBadRequest)
			}
			if len(images) > 2 {
				return requestError("MiniMax H3 frame mode accepts at most 2 images", "reference_image_limit_exceeded", http.StatusBadRequest)
			}
		case "images":
			if len(audios) > 0 {
				return requestError("MiniMax H3 images mode does not support audio references", "unsupported_reference_audio", http.StatusBadRequest)
			}
			if len(images) == 0 {
				return requestError("MiniMax H3 images mode requires at least 1 image", "invalid_video_parameter", http.StatusBadRequest)
			}
		case "media":
			if len(images) == 0 || len(audios) == 0 {
				return requestError("MiniMax H3 media mode requires image and audio references", "invalid_video_parameter", http.StatusBadRequest)
			}
		}
	} else {
		if len(images) > 4 {
			return requestError("at most 4 image references are supported", "reference_image_limit_exceeded", http.StatusBadRequest)
		}
		if len(videos) > 3 {
			return requestError("at most 3 video references are supported", "reference_video_limit_exceeded", http.StatusBadRequest)
		}
		if len(audios) > 1 {
			return requestError("at most 1 audio reference is supported", "reference_audio_limit_exceeded", http.StatusBadRequest)
		}
		if len(images)+len(videos) > 7 {
			return requestError("at most 7 image and video references are supported", "reference_limit_exceeded", http.StatusBadRequest)
		}
		if len(audios) > 0 && len(images) == 0 && len(videos) == 0 {
			return requestError("Seedance audio references require at least one image or video reference", "invalid_video_parameter", http.StatusBadRequest)
		}
		if mode != "" && mode != "media" {
			return requestError("reference_mode must be media when provided", "unsupported_reference_mode", http.StatusBadRequest)
		}
		if mode == "" && (len(images) > 0 || len(videos) > 0 || len(audios) > 0) {
			return requestError("reference_mode must be media when references are provided", "unsupported_reference_mode", http.StatusBadRequest)
		}
	}

	names := make(map[string]struct{}, len(images)+len(videos)+len(audios))
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

	payload := &normalizedRequest{Prompt: prompt, Duration: duration, AspectRatio: aspectRatio, GenerateAudio: generateAudio, ReferenceMode: mode}
	for _, source := range images {
		item, taskErr := normalize(source, "image")
		if taskErr != nil {
			return taskErr
		}
		payload.ReferenceImages = append(payload.ReferenceImages, item)
	}
	for _, source := range videos {
		item, taskErr := normalize(source, "video")
		if taskErr != nil {
			return taskErr
		}
		payload.ReferenceVideos = append(payload.ReferenceVideos, item)
	}
	for _, source := range audios {
		item, taskErr := normalize(source, "audio")
		if taskErr != nil {
			return taskErr
		}
		payload.ReferenceAudios = append(payload.ReferenceAudios, item)
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
	minimumDuration := 4
	if modelName == "minimax-h3-1440p" {
		minimumDuration = 5
	}
	if payload.Duration < minimumDuration || payload.Duration > 15 {
		return requestError(fmt.Sprintf("duration must be an integer between %d and 15 seconds", minimumDuration), "invalid_video_duration", http.StatusBadRequest)
	}
	if modelName == "minimax-h3-1440p" && payload.GenerateAudio != nil && !*payload.GenerateAudio {
		return requestError("MiniMax H3 always generates native audio", "invalid_video_parameter", http.StatusBadRequest)
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
	audios := make([]referenceMedia, len(payload.ReferenceAudios))
	copy(audios, payload.ReferenceAudios)
	requestPayload := upstreamRequest{
		Model: modelName, Prompt: payload.Prompt, Duration: payload.Duration,
		AspectRatio: payload.AspectRatio, GenerateAudio: payload.GenerateAudio,
		Public: false, ImageReferences: images, VideoReferences: videos, AudioReferences: audios,
	}
	if modelName == "minimax-h3-1440p" {
		requestPayload.ReferenceMode = payload.ReferenceMode
	} else {
		seed := -1
		requestPayload.Seed = &seed
	}
	body, err := common.Marshal(requestPayload)
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
		result.ProgressMetadataSet = true
		result.ProgressKnown = true
		result.ProgressSource = "upstream_percent"
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

func isMiniMaxH3Model(value string) bool {
	switch strings.TrimSpace(value) {
	case "minimax-h3-1440p", "leonardo-minimax-h3-1440p":
		return true
	default:
		return false
	}
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
		case "content_moderated", "invalid_reference_media_duration", "reference_media_duration_exceeded", "private_generation_unavailable":
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
