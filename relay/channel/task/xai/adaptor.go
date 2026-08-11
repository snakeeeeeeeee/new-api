package xai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
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

const ChannelName = "xai"

const xai2KENContentResolver = "xai-2ken-content"

var ModelList = []string{
	"grok-imagine-video",
	"grok-imagine-video-1.5",
	"grok-imagine-video-480p",
	"grok-imagine-video-720p",
	"grok-imagine-video-1.5-preview-480p",
	"grok-imagine-video-1.5-preview-720p",
}

type requestPayload map[string]any

type submitResponse struct {
	RequestID string `json:"request_id"`
	ID        string `json:"id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
}

type taskResponse struct {
	ID       string `json:"id,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
	Status   string `json:"status"`
	Code     string `json:"code,omitempty"`
	Seconds  string `json:"seconds,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
	Video    *struct {
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
	is2KEN  bool
}

func (a *TaskAdaptor) OpenAIVideoCompatibility() channel.OpenAIVideoCompatibility {
	if a.is2KEN {
		return channel.OpenAIVideoCompatibility{Generation: true}
	}
	return channel.OpenAIVideoCompatibility{Generation: true, Edit: true, Extension: true}
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
	a.baseURL = strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if a.baseURL == "" {
		a.baseURL = constant.ChannelBaseURLs[constant.ChannelTypeXai]
	}
	a.apiKey = info.ApiKey
	a.is2KEN = info.ChannelOtherSettings.IsXAI2KEN()
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
	if a.is2KEN {
		return a.prepare2KENNormalizedVideoRequest(c, info, request)
	}
	if request.Output.GenerateAudio != nil {
		return xaiNormalizedVideoError("output.generate_audio is not supported by xAI")
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
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "generate_audio":
				return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.xai.generate_audio is no longer supported; output.generate_audio is not supported by xAI"), "invalid_provider_options", http.StatusBadRequest)
			case "reference_mode":
				return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.xai.reference_mode is no longer supported; use input.reference_mode"), "invalid_provider_options", http.StatusBadRequest)
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

func (a *TaskAdaptor) prepare2KENNormalizedVideoRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.VideoTaskCreateRequest) *dto.TaskError {
	if request.Operation != "generation" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("2KEN only supports video generation"), "unsupported_video_operation", http.StatusBadRequest)
	}
	if strings.TrimSpace(request.Input.Prompt) == "" {
		return xaiNormalizedVideoError("prompt is required")
	}
	if request.Output.GenerateAudio != nil {
		return xaiNormalizedVideoError("output.generate_audio is not supported by 2KEN")
	}
	if request.Input.Video != nil {
		return xaiUnsupportedVideoInput("2KEN generation does not accept input.video")
	}
	if len(request.Input.ReferenceVideos) > 0 || len(request.Input.ReferenceAudios) > 0 {
		return xaiUnsupportedVideoInput("2KEN generation only accepts image reference media")
	}
	if request.Input.Image != nil && len(request.Input.ReferenceImages) > 0 {
		return xaiUnsupportedVideoInput("2KEN image and reference_images generation modes are mutually exclusive")
	}
	if len(request.Input.ReferenceImages) > 2 {
		return xaiUnsupportedVideoInput("2KEN reference_images supports at most 2 images")
	}
	referenceMode := strings.ToLower(strings.TrimSpace(request.Input.ReferenceMode))
	if request.Input.Image != nil && referenceMode != "" && referenceMode != "frame" {
		return xaiUnsupportedVideoInput("2KEN single image input requires reference_mode=frame")
	}
	if len(request.Input.ReferenceImages) > 0 && referenceMode != "" && referenceMode != "media" {
		return xaiUnsupportedVideoInput("2KEN reference_images requires reference_mode=media")
	}
	if request.Input.Image == nil && len(request.Input.ReferenceImages) == 0 && referenceMode != "" {
		return xaiUnsupportedVideoInput("2KEN reference_mode requires image input")
	}
	if len(request.ProviderOptions) > 0 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options are not supported by 2KEN"), "invalid_provider_options", http.StatusBadRequest)
	}

	_, modelResolution, supported := xai2KENVideoModel(request.Model)
	if !supported {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported 2KEN public video model: %s", request.Model), "unsupported_video_model", http.StatusBadRequest)
	}
	duration := 4
	if request.Output.Duration != nil {
		duration = *request.Output.Duration
	}
	if duration < 1 || duration > 15 {
		return xaiNormalizedVideoError("duration must be between 1 and 15 seconds")
	}

	payload := requestPayload{
		"model":      strings.TrimSpace(request.Model),
		"prompt":     strings.TrimSpace(request.Input.Prompt),
		"duration":   duration,
		"resolution": modelResolution,
	}
	if request.Output.Resolution != nil {
		requestedResolution := strings.ToLower(strings.TrimSpace(*request.Output.Resolution))
		if requestedResolution != modelResolution {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("resolution %s conflicts with public model resolution %s", requestedResolution, modelResolution),
				"invalid_video_resolution", http.StatusBadRequest,
			)
		}
	}
	if request.Output.AspectRatio != nil {
		aspectRatio := strings.TrimSpace(*request.Output.AspectRatio)
		if !validXAIAspectRatio(aspectRatio) {
			return xaiNormalizedVideoError("aspect_ratio is not supported by 2KEN")
		}
		payload["aspect_ratio"] = aspectRatio
	}
	if request.Input.Image != nil {
		imageURL, taskErr := normalized2KENImageURL(*request.Input.Image)
		if taskErr != nil {
			return taskErr
		}
		payload["image"] = map[string]any{"url": imageURL}
	}
	if len(request.Input.ReferenceImages) > 0 {
		references := make([]map[string]any, 0, len(request.Input.ReferenceImages))
		for _, input := range request.Input.ReferenceImages {
			imageURL, taskErr := normalized2KENImageURL(input)
			if taskErr != nil {
				return taskErr
			}
			references = append(references, map[string]any{"url": imageURL})
		}
		payload["reference_images"] = references
	}

	info.Action = constant.TaskActionVideoGeneration
	info.OriginModelName = strings.TrimSpace(request.Model)
	c.Set("xai_video_request", payload)
	return nil
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	payload, err := getPayloadFromContext(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if a.is2KEN {
		return validate2KENNormalizedVideoModel(info, payload)
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

func validate2KENNormalizedVideoModel(info *relaycommon.RelayInfo, payload requestPayload) *dto.TaskError {
	publicModel := ""
	upstreamModel := ""
	if info != nil {
		publicModel = strings.TrimSpace(info.OriginModelName)
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	expectedUpstream, expectedResolution, supported := xai2KENVideoModel(publicModel)
	if !supported {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported 2KEN public video model: %s", publicModel), "unsupported_video_model", http.StatusBadRequest)
	}
	if upstreamModel != expectedUpstream {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("2KEN model %s must map to %s, got %s", publicModel, expectedUpstream, upstreamModel),
			"unsupported_video_model_mapping", http.StatusBadRequest,
		)
	}
	if strings.ToLower(strings.TrimSpace(getString(payload, "resolution"))) != expectedResolution {
		return service.TaskErrorWrapperLocal(fmt.Errorf("2KEN public model resolution mapping is invalid"), "invalid_video_resolution", http.StatusBadRequest)
	}
	return nil
}

func xai2KENVideoModel(modelName string) (upstreamModel string, resolution string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "grok-imagine-video-480p":
		return "grok-imagine-video", "480p", true
	case "grok-imagine-video-720p":
		return "grok-imagine-video", "720p", true
	case "grok-imagine-video-1.5-preview-480p":
		return "grok-imagine-video-1.5-preview", "480p", true
	case "grok-imagine-video-1.5-preview-720p":
		return "grok-imagine-video-1.5-preview", "720p", true
	default:
		return "", "", false
	}
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

func payloadInteger(payload requestPayload, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	value, exists := payload[key]
	if !exists || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) {
			return 0, true
		}
		return int(typed), true
	default:
		return 0, true
	}
}

func (a *TaskAdaptor) ResolveVideoBilling(c *gin.Context, info *relaycommon.RelayInfo) (channel.VideoBillingEstimate, *dto.TaskError) {
	action := ""
	if info != nil && info.TaskRelayInfo != nil {
		action = info.Action
	}
	if action == constant.TaskActionVideoEdit {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(
			fmt.Errorf("xAI edit inherits the source duration and cannot use per-second billing"),
			"video_per_second_billing_unsupported",
			http.StatusBadRequest,
		)
	}
	if action != constant.TaskActionVideoGeneration && action != constant.TaskActionVideoExtension {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(
			fmt.Errorf("unsupported xAI video billing action: %s", action),
			"video_per_second_billing_unsupported",
			http.StatusBadRequest,
		)
	}
	payload, err := getPayloadFromContext(c)
	if err != nil {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	seconds, exists := payloadInteger(payload, "duration")
	if !exists {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(
			fmt.Errorf("duration is required for per-second video billing"),
			"video_duration_required",
			http.StatusBadRequest,
		)
	}
	minSeconds, maxSeconds := 1, 15
	basis := types.VideoPricingBasisGeneration
	if action == constant.TaskActionVideoExtension {
		minSeconds, maxSeconds = 2, 10
		basis = types.VideoPricingBasisExtensionDelta
	} else if _, hasReferences := payload["reference_images"]; hasReferences {
		maxSeconds = 10
	}
	if seconds < minSeconds || seconds > maxSeconds {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(
			fmt.Errorf("duration must be between %d and %d seconds", minSeconds, maxSeconds),
			"invalid_video_duration",
			http.StatusBadRequest,
		)
	}
	payload["duration"] = seconds
	c.Set("xai_video_request", payload)
	return channel.VideoBillingEstimate{Seconds: seconds, Basis: basis}, nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	action := ""
	if info != nil && info.TaskRelayInfo != nil {
		action = info.Action
	}
	switch action {
	case constant.TaskActionVideoGeneration:
		if a.is2KEN {
			return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
		}
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
	if a.is2KEN {
		seconds, exists := payloadInteger(payload, "duration")
		if !exists || seconds < 1 || seconds > 15 {
			return nil, fmt.Errorf("valid duration is required for 2KEN video generation")
		}
		upstreamPayload := requestPayload{
			"model":      upstreamModel,
			"prompt":     getString(payload, "prompt"),
			"seconds":    strconv.Itoa(seconds),
			"resolution": getString(payload, "resolution"),
		}
		if aspectRatio := strings.TrimSpace(getString(payload, "aspect_ratio")); aspectRatio != "" {
			upstreamPayload["aspect_ratio"] = aspectRatio
		}
		if image, ok := payload["image"].(map[string]any); ok {
			if imageURL, ok := image["url"].(string); ok && strings.TrimSpace(imageURL) != "" {
				upstreamPayload["image_url"] = strings.TrimSpace(imageURL)
			}
		}
		if references, ok := payload["reference_images"]; ok {
			upstreamPayload["reference_images"] = references
		}
		body, err := common.Marshal(upstreamPayload)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(body), nil
	}

	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func normalized2KENImageURL(source dto.VideoTaskSource) (string, *dto.TaskError) {
	imageURL := strings.TrimSpace(source.URL)
	if source.Provider != "" || source.FileID != "" {
		return "", xaiUnsupportedVideoInput("2KEN image input only accepts a public URL")
	}
	lowerURL := strings.ToLower(imageURL)
	if imageURL == "" || strings.HasPrefix(lowerURL, "data:") || (!strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://")) {
		return "", xaiUnsupportedVideoInput("2KEN image input must be a public HTTP(S) URL")
	}
	return imageURL, nil
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
	if a.is2KEN && upstreamID == "" {
		upstreamID = strings.TrimSpace(submit.TaskID)
	}
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

	uri := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseUrl, "/"), url.PathEscape(strings.TrimSpace(taskID)))
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
	case "queued":
		if a.is2KEN {
			taskResult.Status = model.TaskStatusQueued
		}
	case "pending":
		taskResult.Status = model.TaskStatusInProgress
	case "in_progress":
		if a.is2KEN {
			taskResult.Status = model.TaskStatusInProgress
		}
	case "done", "completed":
		providerTaskID := firstNonEmpty(res.TaskID, res.ID)
		if res.Video != nil && strings.TrimSpace(res.Video.URL) != "" {
			taskResult.Status = model.TaskStatusSuccess
			taskResult.Url = res.Video.URL
			taskResult.VideoOutputs = []relaycommon.VideoOutput{{
				Index: 0, URL: res.Video.URL, MimeType: "video/mp4",
				DurationMS: int64(res.Video.Duration) * 1000,
			}}
		} else if a.is2KEN && providerTaskID != "" {
			duration, _ := strconv.Atoi(strings.TrimSpace(res.Seconds))
			taskResult.Status = model.TaskStatusSuccess
			taskResult.VideoOutputs = []relaycommon.VideoOutput{{
				Index:             0,
				ProviderReference: providerTaskID,
				Resolver:          xai2KENContentResolver,
				MimeType:          "video/mp4",
				Filename:          providerTaskID + ".mp4",
				DurationMS:        int64(duration) * 1000,
			}}
		} else {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Reason = "video url is empty"
			if res.Video != nil && res.Video.RespectModeration != nil && !*res.Video.RespectModeration {
				taskResult.Reason = "video rejected by moderation"
			}
		}
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

func (a *TaskAdaptor) ResolveVideoContent(ctx context.Context, providerChannel *model.Channel, task *model.Task, output relaycommon.VideoOutput, headers http.Header) (*http.Response, error) {
	if providerChannel == nil || task == nil {
		return nil, fmt.Errorf("video content context is incomplete")
	}
	if output.Resolver != xai2KENContentResolver {
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
		return nil, fmt.Errorf("2KEN channel base URL is required")
	}
	key := strings.TrimSpace(task.PrivateData.Key)
	if key == "" {
		key = strings.TrimSpace(providerChannel.Key)
	}
	if key == "" {
		return nil, fmt.Errorf("2KEN channel key is required")
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
