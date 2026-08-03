package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string          `json:"type"`                // "text", "image_url" or "video"
	Text     string          `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL       `json:"image_url,omitempty"` // for image_url type
	Video    *VideoReference `json:"video,omitempty"`     // for video (sample) type
	Role     string          `json:"role,omitempty"`      // reference_image / first_frame / last_frame
}

type ImageURL struct {
	URL string `json:"url"`
}

type VideoReference struct {
	URL string `json:"url"` // Draft video URL
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter dto.IntValue   `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Resolution            string         `json:"resolution,omitempty"`
	Ratio                 string         `json:"ratio,omitempty"`
	Duration              dto.IntValue   `json:"duration,omitempty"`
	Frames                dto.IntValue   `json:"frames,omitempty"`
	Seed                  dto.IntValue   `json:"seed,omitempty"`
	CameraFixed           *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark             *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	Usage           struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) OpenAIVideoCompatibility() channel.OpenAIVideoCompatibility {
	return channel.OpenAIVideoCompatibility{Generation: true}
}

const normalizedRequestContextKey = "doubao_video_request"

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) PrepareNormalizedVideoRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.VideoTaskCreateRequest) *dto.TaskError {
	if info == nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("relay info is nil"), "invalid_request", http.StatusBadRequest)
	}
	if request.Operation != "generation" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("Seedance only supports normalized generation"), "unsupported_video_operation", http.StatusBadRequest)
	}
	if request.Input.Video != nil {
		return service.TaskErrorWrapperLocal(fmt.Errorf("Seedance generation does not accept input.video"), "unsupported_video_input", http.StatusBadRequest)
	}

	generateAudio := dto.BoolValue(true)
	if request.Output.GenerateAudio != nil {
		generateAudio = dto.BoolValue(*request.Output.GenerateAudio)
	}
	payload := &requestPayload{Model: request.Model, Content: []ContentItem{}, GenerateAudio: &generateAudio}
	if request.Input.Prompt != "" {
		payload.Content = append(payload.Content, ContentItem{Type: "text", Text: request.Input.Prompt})
	}
	if request.Input.Image != nil {
		item, taskErr := normalizedDoubaoImage(*request.Input.Image, "first_frame")
		if taskErr != nil {
			return taskErr
		}
		payload.Content = append(payload.Content, item)
	}
	for _, source := range request.Input.ReferenceImages {
		item, taskErr := normalizedDoubaoImage(source, "reference_image")
		if taskErr != nil {
			return taskErr
		}
		payload.Content = append(payload.Content, item)
	}
	if request.Output.Duration != nil {
		if *request.Output.Duration <= 0 || *request.Output.Duration > relaycommon.MaxTaskDurationSeconds {
			return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between 1 and %d", relaycommon.MaxTaskDurationSeconds), "invalid_video_duration", http.StatusBadRequest)
		}
		payload.Duration = dto.IntValue(*request.Output.Duration)
	}
	if request.Output.Resolution != nil {
		payload.Resolution = *request.Output.Resolution
	}
	if request.Output.AspectRatio != nil {
		payload.Ratio = *request.Output.AspectRatio
	}

	var providerOptions map[string]any
	for namespace, options := range request.ProviderOptions {
		if namespace != "doubao" && namespace != ChannelName {
			return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.%s is not supported by Seedance", namespace), "invalid_provider_options", http.StatusBadRequest)
		}
		if providerOptions != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("use only one Seedance provider_options namespace"), "invalid_provider_options", http.StatusBadRequest)
		}
		providerOptions = options
	}
	for key := range providerOptions {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "generate_audio":
			return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.%s is no longer supported; use output.generate_audio", key), "invalid_provider_options", http.StatusBadRequest)
		case "reference_mode":
			return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.%s is no longer supported; use input.reference_mode", key), "invalid_provider_options", http.StatusBadRequest)
		case "model", "content", "duration", "resolution", "ratio":
			return service.TaskErrorWrapperLocal(fmt.Errorf("provider_options.%s duplicates a public field", key), "invalid_provider_options", http.StatusBadRequest)
		}
	}
	if len(providerOptions) > 0 {
		data, err := common.Marshal(providerOptions)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_provider_options", http.StatusBadRequest)
		}
		if err := common.Unmarshal(data, payload); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_provider_options", http.StatusBadRequest)
		}
	}

	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionVideoGeneration
	info.OriginModelName = request.Model
	c.Set(normalizedRequestContextKey, payload)
	return nil
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(_ *gin.Context, _ *relaycommon.RelayInfo) *dto.TaskError {
	return nil
}

func normalizedDoubaoImage(source dto.VideoTaskSource, role string) (ContentItem, *dto.TaskError) {
	if strings.TrimSpace(source.URL) == "" {
		return ContentItem{}, service.TaskErrorWrapperLocal(fmt.Errorf("Seedance normalized inputs require url sources"), "unsupported_file_provider", http.StatusBadRequest)
	}
	return ContentItem{Type: "image_url", ImageURL: &ImageURL{URL: source.URL}, Role: role}, nil
}

func (a *TaskAdaptor) ResolveVideoBilling(c *gin.Context, _ *relaycommon.RelayInfo) (channel.VideoBillingEstimate, *dto.TaskError) {
	var seconds int
	if value, ok := c.Get(normalizedRequestContextKey); ok {
		payload, valid := value.(*requestPayload)
		if !valid || payload == nil {
			return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(fmt.Errorf("invalid normalized Seedance request"), "invalid_request", http.StatusBadRequest)
		}
		seconds = int(payload.Duration)
	} else {
		req, err := relaycommon.GetTaskRequest(c)
		if err != nil {
			return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		payload, err := a.convertToRequestPayload(&req)
		if err != nil {
			return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		seconds = int(payload.Duration)
		if seconds == 0 {
			seconds = req.Duration
		}
		if seconds > 0 {
			req.Duration = seconds
			if req.Metadata == nil {
				req.Metadata = map[string]interface{}{}
			}
			req.Metadata["duration"] = seconds
			c.Set("task_request", req)
		}
	}
	if seconds <= 0 {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(fmt.Errorf("duration is required for per-second video billing"), "video_duration_required", http.StatusBadRequest)
	}
	if seconds > relaycommon.MaxTaskDurationSeconds {
		return channel.VideoBillingEstimate{}, service.TaskErrorWrapperLocal(fmt.Errorf("duration must be between 1 and %d", relaycommon.MaxTaskDurationSeconds), "invalid_video_duration", http.StatusBadRequest)
	}
	return channel.VideoBillingEstimate{Seconds: seconds, Basis: types.VideoPricingBasisGeneration}, nil
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	var body *requestPayload
	if value, ok := c.Get(normalizedRequestContextKey); ok {
		body, _ = value.(*requestPayload)
	}
	if body == nil {
		req, err := relaycommon.GetTaskRequest(c)
		if err != nil {
			return nil, err
		}
		body, err = a.convertToRequestPayload(&req)
		if err != nil {
			return nil, errors.Wrap(err, "convert request payload failed")
		}
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	if _, normalized := c.Get(relaycommon.VideoTaskPublicRequestContextKey); !normalized {
		c.JSON(http.StatusOK, ov)
	}
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add text prompt
	if req.Prompt != "" {
		r.Content = append(r.Content, ContentItem{
			Type: "text",
			Text: req.Prompt,
		})
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &ImageURL{
					URL: imgURL,
				},
			})
		}
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	if r.Duration == 0 && req.Duration > 0 {
		r.Duration = dto.IntValue(req.Duration)
	}

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		taskResult.VideoOutputs = []relaycommon.VideoOutput{{
			Index: 0, URL: resTask.Content.VideoURL, MimeType: "video/mp4",
			DurationMS: int64(resTask.Duration) * 1000,
		}}
		// 解析 usage 信息用于按倍率计费
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task failed"
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: "task failed",
			Code:    "failed",
		}
	}

	return common.Marshal(openAIVideo)
}
