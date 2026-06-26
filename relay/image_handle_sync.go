package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	imageHandleSyncModeInherit  = "inherit"
	imageHandleSyncModeForceOn  = "force_on"
	imageHandleSyncModeForceOff = "force_off"

	imageHandleResultFormatURL    = "url"
	imageHandleResultFormatBase64 = "base64"
)

type imageHandleSyncRequest struct {
	RequestID        string                  `json:"request_id"`
	ClientTaskID     string                  `json:"client_task_id"`
	Model            string                  `json:"model"`
	Operation        string                  `json:"operation"`
	ResultDataFormat string                  `json:"result_data_format,omitempty"`
	Input            imageHandleSyncInput    `json:"input"`
	Parameters       map[string]any          `json:"parameters,omitempty"`
	ProviderOptions  map[string]any          `json:"provider_options,omitempty"`
	Executor         imageHandleSyncExecutor `json:"executor"`
	Metadata         map[string]interface{}  `json:"metadata,omitempty"`
}

type imageHandleSyncInput struct {
	Text   string   `json:"text"`
	Images []string `json:"images,omitempty"`
	Mask   *string  `json:"mask,omitempty"`
}

type imageHandleSyncExecutor struct {
	Type       string `json:"type"`
	LeaseID    string `json:"lease_id"`
	ResolveURL string `json:"resolve_url"`
	SecretID   string `json:"secret_id"`
}

type imageHandleSyncResponse struct {
	TaskID                   string                 `json:"task_id"`
	ProviderTaskID           string                 `json:"provider_task_id"`
	ClientTaskID             string                 `json:"client_task_id"`
	Status                   string                 `json:"status"`
	Progress                 string                 `json:"progress"`
	ResultDataFormat         string                 `json:"result_data_format,omitempty"`
	Result                   *imageHandleSyncResult `json:"result,omitempty"`
	Usage                    *imageHandleSyncUsage  `json:"usage,omitempty"`
	Error                    *imageHandleSyncError  `json:"error,omitempty"`
	SyncWait                 *imageHandleSyncWait   `json:"sync_wait,omitempty"`
	RawResponse              any                    `json:"raw_response,omitempty"`
	RawResponseTruncated     bool                   `json:"raw_response_truncated,omitempty"`
	RawResponseOmittedFields []string               `json:"raw_response_omitted_fields,omitempty"`
}

type imageHandleSyncResult struct {
	Images      []imageHandleSyncImage `json:"images,omitempty"`
	RawResponse any                    `json:"raw_response,omitempty"`
}

type imageHandleSyncImage struct {
	URL           string `json:"url"`
	B64Json       string `json:"b64_json,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Filename      string `json:"filename,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
}

type imageHandleSyncUsage struct {
	TotalTokens              int                    `json:"total_tokens,omitempty"`
	InputTokens              int                    `json:"input_tokens,omitempty"`
	OutputTokens             int                    `json:"output_tokens,omitempty"`
	PromptTokens             int                    `json:"prompt_tokens,omitempty"`
	CompletionTokens         int                    `json:"completion_tokens,omitempty"`
	CachedTokens             int                    `json:"cached_tokens,omitempty"`
	CacheReadTokens          int                    `json:"cache_read_tokens,omitempty"`
	PromptCacheHitTokens     int                    `json:"prompt_cache_hit_tokens,omitempty"`
	CacheCreationTokens      int                    `json:"cache_creation_tokens,omitempty"`
	CacheCreationInputTokens int                    `json:"cache_creation_input_tokens,omitempty"`
	CacheCreation5mTokens    int                    `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1hTokens    int                    `json:"cache_creation_1h_tokens,omitempty"`
	ImageTokens              int                    `json:"image_tokens,omitempty"`
	AudioTokens              int                    `json:"audio_tokens,omitempty"`
	PromptTokensDetails      *dto.InputTokenDetails `json:"prompt_tokens_details,omitempty"`
	InputTokensDetails       *dto.InputTokenDetails `json:"input_tokens_details,omitempty"`
}

type imageHandleSyncError struct {
	Code                 string `json:"code,omitempty"`
	Message              string `json:"message,omitempty"`
	Retryable            bool   `json:"retryable,omitempty"`
	UpstreamStatus       int    `json:"upstream_status,omitempty"`
	ProviderErrorCode    string `json:"provider_error_code,omitempty"`
	ProviderErrorType    string `json:"provider_error_type,omitempty"`
	ProviderErrorMessage string `json:"provider_error_message,omitempty"`
	ProviderErrorParam   string `json:"provider_error_param,omitempty"`
	UpstreamError        any    `json:"upstream_error,omitempty"`
}

type imageHandleSyncErrorDetail struct {
	Code                     string   `json:"code,omitempty"`
	Message                  string   `json:"message,omitempty"`
	Retryable                bool     `json:"retryable,omitempty"`
	UpstreamStatus           int      `json:"upstream_status,omitempty"`
	ProviderErrorCode        string   `json:"provider_error_code,omitempty"`
	ProviderErrorType        string   `json:"provider_error_type,omitempty"`
	ProviderErrorMessage     string   `json:"provider_error_message,omitempty"`
	ProviderErrorParam       string   `json:"provider_error_param,omitempty"`
	UpstreamError            any      `json:"upstream_error,omitempty"`
	RawResponseSummary       any      `json:"raw_response_summary,omitempty"`
	RawResponseTruncated     bool     `json:"raw_response_truncated,omitempty"`
	RawResponseOmittedFields []string `json:"raw_response_omitted_fields,omitempty"`
	ProviderTaskID           string   `json:"provider_task_id,omitempty"`
	ClientTaskID             string   `json:"client_task_id,omitempty"`
	TaskID                   string   `json:"task_id,omitempty"`
	ResultDataFormat         string   `json:"result_data_format,omitempty"`
}

type imageHandleSyncWait struct {
	Completed bool `json:"completed"`
	TimeoutMS int  `json:"timeout_ms"`
}

func shouldUseImageHandleSync(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelMeta == nil {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeImagesGenerations && info.RelayMode != relayconstant.RelayModeImagesEdits {
		return false
	}
	channelType := info.ChannelType
	if info.ChannelMeta != nil && info.ChannelMeta.ChannelType != 0 {
		channelType = info.ChannelMeta.ChannelType
	}
	if !imageHandleSyncChannelSupported(channelType) {
		return false
	}
	mode := strings.TrimSpace(info.ChannelOtherSettings.ImageHandleSyncMode)
	switch mode {
	case imageHandleSyncModeForceOff:
		return false
	case imageHandleSyncModeForceOn:
		return true
	case "", imageHandleSyncModeInherit:
		return service.GetImageHandleExecutorConfig().SyncImageEnabled
	default:
		return service.GetImageHandleExecutorConfig().SyncImageEnabled
	}
}

func canUseImageHandleSyncForRequest(info *relaycommon.RelayInfo, request *dto.ImageRequest) bool {
	if !shouldUseImageHandleSync(info) {
		return false
	}
	if info == nil || request == nil {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeImagesEdits {
		return true
	}
	return imageHandleSyncEditInputsAreURLs(*request)
}

func imageHandleSyncChannelSupported(channelType int) bool {
	return channelType == constant.ChannelTypeOpenAI || channelType == constant.ChannelTypeXai
}

func relayImageHandleSync(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (*dto.Usage, *types.NewAPIError) {
	common.SetContextKey(c, constant.ContextKeyExecutionMode, "image_handle_sync")
	if err := service.ValidateImageHandleExecutorConfig(); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	clientTaskID := model.GenerateTaskID()
	action := constant.TaskActionImageGeneration
	operation := "generation"
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		action = constant.TaskActionImageEdit
		operation = "edit"
	}
	task := initImageHandleSyncTask(info, request, clientTaskID, action)
	lease := model.NewImageCredentialLease(task, operation, info.UpstreamModelName, 1800)
	lease.TaskRecordID = 0
	if err := model.CreateImageCredentialLease(lease); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeUpdateDataError, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}

	body, err := buildImageHandleSyncPayload(c, info, request, clientTaskID, lease.LeaseID, operation)
	if err != nil {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	resp, err := postImageHandleSync(c, body)
	if err != nil {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if statusErr := imageHandleSyncStatusError(c, resp.StatusCode, responseBody); statusErr != nil {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, statusErr
	}

	var syncResp imageHandleSyncResponse
	if err := common.Unmarshal(responseBody, &syncResp); err != nil {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, types.NewOpenAIError(fmt.Errorf("unmarshal image-handle sync response failed: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	status := strings.ToLower(strings.TrimSpace(syncResp.Status))
	if status == "failed" {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		recordImageHandleSyncErrorDetail(c, syncResp)
		return nil, imageHandleSyncFailedError(syncResp)
	}
	if status != "succeeded" {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, imageHandleSyncErrorResponse("image_handle_sync_not_completed", "image-handle sync did not complete", http.StatusGatewayTimeout)
	}
	imageResp := imageHandleSyncToOpenAIResponse(syncResp, info, request)
	if len(imageResp.Data) == 0 {
		_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
		return nil, imageHandleSyncErrorResponse("image_handle_empty_result", "image-handle sync returned empty result", http.StatusBadGateway)
	}
	imageN := uint(1)
	if request.N != nil {
		imageN = *request.N
	}
	normalizeImageUsage(imageResp.Usage, imageN)
	jsonResponse, err := common.Marshal(imageResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody, types.ErrOptionWithSkipRetry())
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	usage := imageResp.Usage
	if usage == nil {
		usage = &dto.Usage{UsageSource: "image_handle_sync"}
	}
	task.PrivateData.UpstreamTaskID = firstNonEmpty(syncResp.ProviderTaskID, syncResp.TaskID)
	resultFormat := imageHandleSyncResultDataFormat(request)
	if resultFormat == imageHandleResultFormatURL && strings.TrimSpace(imageResp.Data[0].Url) != "" {
		task.PrivateData.ResultURL = imageResp.Data[0].Url
	}
	task.Data = imageHandleSyncTaskData(syncResp)
	if resultFormat == imageHandleResultFormatURL && imageHandleSyncResponseHasURL(syncResp) {
		if err := createImageHandleSyncAssets(c, task, syncResp); err != nil {
			logger.LogError(c, fmt.Sprintf("create image-handle sync assets failed: %s", err.Error()))
		}
	}
	c.Set("image_handle_sync_provider_task_id", task.PrivateData.UpstreamTaskID)
	c.Set("image_handle_sync_lease_id", lease.LeaseID)
	return usage, nil
}

func buildImageHandleSyncPayload(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest, clientTaskID string, leaseID string, operation string) ([]byte, error) {
	metadata := map[string]interface{}{
		"tenant_id":      fmt.Sprintf("user_%d", info.UserId),
		"channel_id":     fmt.Sprintf("channel_%d", info.ChannelId),
		"sync":           true,
		"execution_mode": "image_handle_sync",
	}
	if service.GetImageHandleExecutorConfig().DebugUpstream {
		metadata["debug_upstream"] = true
	}
	images, mask, err := imageHandleSyncInputsFromRequest(request)
	if err != nil {
		return nil, err
	}
	parameters := imageHandleSyncParameters(request, info.UpstreamModelName)
	cfg := service.GetImageHandleExecutorConfig()
	payload := imageHandleSyncRequest{
		RequestID:        c.GetString(common.RequestIdKey),
		ClientTaskID:     clientTaskID,
		Model:            info.UpstreamModelName,
		Operation:        operation,
		ResultDataFormat: imageHandleSyncResultDataFormat(request),
		Input:            imageHandleSyncInput{Text: request.Prompt, Images: images, Mask: mask},
		Parameters:       parameters,
		ProviderOptions:  imageHandleSyncProviderOptions(request),
		Executor: imageHandleSyncExecutor{
			Type:       "provider_direct_lease",
			LeaseID:    leaseID,
			ResolveURL: service.BuildImageHandleCredentialLeaseResolveURL(leaseID),
			SecretID:   cfg.InternalSecretID,
		},
		Metadata: metadata,
	}
	return common.Marshal(payload)
}

func imageHandleSyncResultDataFormat(request dto.ImageRequest) string {
	cfg := service.GetImageHandleExecutorConfig()
	switch cfg.SyncImageResultPolicy {
	case image_handle_setting.SyncImageResultFormatPolicyForceURL:
		return imageHandleResultFormatURL
	case image_handle_setting.SyncImageResultFormatPolicyForceBase64:
		return imageHandleResultFormatBase64
	}
	responseFormat := strings.TrimSpace(request.ResponseFormat)
	if responseFormat == "" {
		if cfg.SyncImageDefaultFormat == image_handle_setting.SyncImageDefaultResultFormatBase64 {
			return imageHandleResultFormatBase64
		}
		return imageHandleResultFormatURL
	}
	if strings.EqualFold(responseFormat, "b64_json") {
		return imageHandleResultFormatBase64
	}
	return imageHandleResultFormatURL
}

func postImageHandleSync(c *gin.Context, body []byte) (*http.Response, error) {
	cfg := service.GetImageHandleExecutorConfig()
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/v1/image/tasks/sync", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client, err := service.GetHttpClientWithProxy(c.GetString("channel_proxy"))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func imageHandleSyncInputsFromRequest(request dto.ImageRequest) ([]string, *string, error) {
	var images []string
	if len(request.Image) > 0 {
		var one string
		if err := common.Unmarshal(request.Image, &one); err == nil && strings.TrimSpace(one) != "" {
			images = append(images, strings.TrimSpace(one))
		} else {
			var many []string
			if err := common.Unmarshal(request.Image, &many); err == nil {
				for _, image := range many {
					if strings.TrimSpace(image) != "" {
						images = append(images, strings.TrimSpace(image))
					}
				}
			} else if err != nil {
				return nil, nil, fmt.Errorf("image-handle sync image edits only support image URL input")
			}
		}
	}
	if len(request.Extra) > 0 {
		if raw, ok := request.Extra["images"]; ok && len(raw) > 0 {
			var many []string
			if err := common.Unmarshal(raw, &many); err == nil {
				for _, image := range many {
					if strings.TrimSpace(image) != "" {
						images = append(images, strings.TrimSpace(image))
					}
				}
			}
		}
		if raw, ok := request.Extra["mask"]; ok && len(raw) > 0 {
			var mask string
			if err := common.Unmarshal(raw, &mask); err == nil && strings.TrimSpace(mask) != "" {
				mask = strings.TrimSpace(mask)
				return images, &mask, nil
			}
		}
	}
	return images, nil, nil
}

func imageHandleSyncEditInputsAreURLs(request dto.ImageRequest) bool {
	images, mask, err := imageHandleSyncInputsFromRequest(request)
	if err != nil {
		return false
	}
	if len(images) == 0 {
		return false
	}
	for _, image := range images {
		if !isImageHandleSyncURL(image) {
			return false
		}
	}
	return mask == nil || isImageHandleSyncURL(*mask)
}

func isImageHandleSyncURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func imageHandleSyncParameters(request dto.ImageRequest, upstreamModelName string) map[string]any {
	params := map[string]any{}
	if request.Size != "" {
		params["size"] = request.Size
	}
	if request.Quality != "" {
		params["quality"] = request.Quality
	}
	if request.N != nil {
		params["n"] = *request.N
	}
	addRawImageParam(params, "output_format", request.OutputFormat)
	addRawImageParam(params, "output_compression", request.OutputCompression)
	addRawImageParam(params, "background", request.Background)
	addRawImageParam(params, "moderation", request.Moderation)
	if imageHandleSyncShouldForwardResponseFormat(request, upstreamModelName) {
		switch imageHandleSyncResultDataFormat(request) {
		case imageHandleResultFormatBase64:
			params["response_format"] = "b64_json"
		case imageHandleResultFormatURL:
			params["response_format"] = "url"
		}
	}
	return params
}

func imageHandleSyncShouldForwardResponseFormat(request dto.ImageRequest, upstreamModelName string) bool {
	modelName := firstNonEmpty(upstreamModelName, request.Model)
	return !strings.HasPrefix(strings.ToLower(modelName), "gpt-image-")
}

func imageHandleSyncProviderOptions(request dto.ImageRequest) map[string]any {
	if len(request.ExtraFields) == 0 {
		return nil
	}
	var options map[string]any
	if err := common.Unmarshal(request.ExtraFields, &options); err != nil {
		return nil
	}
	return options
}

func addRawImageParam(params map[string]any, key string, raw any) {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			params[key] = v
		}
	case json.RawMessage:
		addRawJSONParam(params, key, []byte(v))
	case []byte:
		addRawJSONParam(params, key, v)
	default:
		if b, ok := v.([]byte); ok {
			addRawJSONParam(params, key, b)
		}
	}
}

func addRawJSONParam(params map[string]any, key string, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var value any
	if err := common.Unmarshal(raw, &value); err == nil {
		params[key] = value
	}
}

func imageHandleSyncToOpenAIResponse(syncResp imageHandleSyncResponse, info *relaycommon.RelayInfo, request dto.ImageRequest) *dto.ImageResponse {
	data := make([]dto.ImageData, 0)
	resultFormat := imageHandleSyncResultDataFormat(request)
	if syncResp.Result != nil {
		for _, image := range syncResp.Result.Images {
			switch resultFormat {
			case imageHandleResultFormatBase64:
				if strings.TrimSpace(image.B64Json) == "" {
					continue
				}
				data = append(data, dto.ImageData{
					B64Json:       strings.TrimSpace(image.B64Json),
					RevisedPrompt: image.RevisedPrompt,
				})
			default:
				if strings.TrimSpace(image.URL) == "" {
					continue
				}
				data = append(data, dto.ImageData{
					Url:           strings.TrimSpace(image.URL),
					RevisedPrompt: image.RevisedPrompt,
				})
			}
		}
	}
	resp := &dto.ImageResponse{
		Created: common.GetTimestamp(),
		Data:    data,
		Usage:   imageHandleSyncUsageToDTO(imageHandleSyncUsageFromResponse(syncResp)),
	}
	if info != nil && !info.StartTime.IsZero() {
		resp.Created = info.StartTime.Unix()
	}
	topFields := imageHandleSyncOpenAITopFieldsFromResponse(syncResp)
	if topFields.Created > 0 {
		resp.Created = topFields.Created
	}
	resp.Background = firstNonEmpty(topFields.Background, imageHandleSyncStringFromRaw(request.Background))
	resp.OutputFormat = firstNonEmpty(topFields.OutputFormat, imageHandleSyncStringFromRaw(request.OutputFormat))
	resp.Quality = firstNonEmpty(topFields.Quality, request.Quality)
	resp.Size = firstNonEmpty(topFields.Size, request.Size)
	return resp
}

type imageHandleSyncOpenAITopFields struct {
	Created      int64
	Background   string
	OutputFormat string
	Quality      string
	Size         string
}

func imageHandleSyncOpenAITopFieldsFromResponse(syncResp imageHandleSyncResponse) imageHandleSyncOpenAITopFields {
	fields := imageHandleSyncOpenAITopFields{}
	if syncResp.Result != nil {
		mergeImageHandleSyncOpenAITopFields(&fields, imageHandleSyncOpenAITopFieldsFromRaw(syncResp.Result.RawResponse))
	}
	mergeImageHandleSyncOpenAITopFields(&fields, imageHandleSyncOpenAITopFieldsFromRaw(syncResp.RawResponse))
	return fields
}

func imageHandleSyncOpenAITopFieldsFromRaw(raw any) imageHandleSyncOpenAITopFields {
	fields := imageHandleSyncOpenAITopFields{}
	if raw == nil {
		return fields
	}
	data, err := common.Marshal(raw)
	if err != nil || len(data) == 0 {
		return fields
	}
	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		return fields
	}
	fields.Created = imageHandleSyncInt64Field(payload, "created")
	fields.Background = imageHandleSyncStringField(payload, "background")
	fields.OutputFormat = imageHandleSyncStringField(payload, "output_format")
	fields.Quality = imageHandleSyncStringField(payload, "quality")
	fields.Size = imageHandleSyncStringField(payload, "size")
	return fields
}

func mergeImageHandleSyncOpenAITopFields(target *imageHandleSyncOpenAITopFields, source imageHandleSyncOpenAITopFields) {
	if target == nil {
		return
	}
	if target.Created == 0 && source.Created > 0 {
		target.Created = source.Created
	}
	if target.Background == "" {
		target.Background = source.Background
	}
	if target.OutputFormat == "" {
		target.OutputFormat = source.OutputFormat
	}
	if target.Quality == "" {
		target.Quality = source.Quality
	}
	if target.Size == "" {
		target.Size = source.Size
	}
}

func imageHandleSyncStringField(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func imageHandleSyncInt64Field(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int64(typed)
		}
	case int64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return int64(typed)
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func imageHandleSyncStringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func imageHandleSyncResponseHasURL(syncResp imageHandleSyncResponse) bool {
	if syncResp.Result == nil {
		return false
	}
	for _, image := range syncResp.Result.Images {
		if strings.TrimSpace(image.URL) != "" {
			return true
		}
	}
	return false
}

func imageHandleSyncUsageFromResponse(syncResp imageHandleSyncResponse) *imageHandleSyncUsage {
	merged := cloneImageHandleSyncUsage(syncResp.Usage)
	mergeImageHandleSyncUsage(merged, imageHandleSyncUsageFromRaw(syncResp.RawResponse))
	if syncResp.Result != nil {
		mergeImageHandleSyncUsage(merged, imageHandleSyncUsageFromRaw(syncResp.Result.RawResponse))
	}
	if hasImageHandleSyncUsage(merged) {
		return merged
	}
	return syncResp.Usage
}

func imageHandleSyncUsageFromRaw(raw any) *imageHandleSyncUsage {
	if raw == nil {
		return nil
	}
	data, err := common.Marshal(raw)
	if err != nil || len(data) == 0 {
		return nil
	}
	var direct imageHandleSyncUsage
	if err := common.Unmarshal(data, &direct); err == nil && hasImageHandleSyncUsage(&direct) {
		return &direct
	}
	var payload struct {
		Usage  *imageHandleSyncUsage `json:"usage,omitempty"`
		Result struct {
			Usage *imageHandleSyncUsage `json:"usage,omitempty"`
		} `json:"result,omitempty"`
		Output struct {
			Usage *imageHandleSyncUsage `json:"usage,omitempty"`
		} `json:"output,omitempty"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil
	}
	if payload.Usage != nil {
		return payload.Usage
	}
	if payload.Result.Usage != nil {
		return payload.Result.Usage
	}
	if payload.Output.Usage != nil {
		return payload.Output.Usage
	}
	return nil
}

func cloneImageHandleSyncUsage(usage *imageHandleSyncUsage) *imageHandleSyncUsage {
	if usage == nil {
		return &imageHandleSyncUsage{}
	}
	cloned := *usage
	if usage.PromptTokensDetails != nil {
		details := *usage.PromptTokensDetails
		cloned.PromptTokensDetails = &details
	}
	if usage.InputTokensDetails != nil {
		details := *usage.InputTokensDetails
		cloned.InputTokensDetails = &details
	}
	return &cloned
}

func mergeImageHandleSyncUsage(target *imageHandleSyncUsage, source *imageHandleSyncUsage) {
	if target == nil || source == nil {
		return
	}
	fillPositiveSyncInt(&target.TotalTokens, source.TotalTokens)
	fillPositiveSyncInt(&target.InputTokens, source.InputTokens)
	fillPositiveSyncInt(&target.OutputTokens, source.OutputTokens)
	fillPositiveSyncInt(&target.PromptTokens, source.PromptTokens)
	fillPositiveSyncInt(&target.CompletionTokens, source.CompletionTokens)
	fillPositiveSyncInt(&target.CachedTokens, source.CachedTokens)
	fillPositiveSyncInt(&target.CacheReadTokens, source.CacheReadTokens)
	fillPositiveSyncInt(&target.PromptCacheHitTokens, source.PromptCacheHitTokens)
	fillPositiveSyncInt(&target.CacheCreationTokens, source.CacheCreationTokens)
	fillPositiveSyncInt(&target.CacheCreationInputTokens, source.CacheCreationInputTokens)
	fillPositiveSyncInt(&target.CacheCreation5mTokens, source.CacheCreation5mTokens)
	fillPositiveSyncInt(&target.CacheCreation1hTokens, source.CacheCreation1hTokens)
	fillPositiveSyncInt(&target.ImageTokens, source.ImageTokens)
	fillPositiveSyncInt(&target.AudioTokens, source.AudioTokens)
	if source.PromptTokensDetails != nil {
		if target.PromptTokensDetails == nil {
			target.PromptTokensDetails = &dto.InputTokenDetails{}
		}
		mergeInputTokenDetails(target.PromptTokensDetails, source.PromptTokensDetails)
	}
	if source.InputTokensDetails != nil {
		if target.InputTokensDetails == nil {
			target.InputTokensDetails = &dto.InputTokenDetails{}
		}
		mergeInputTokenDetails(target.InputTokensDetails, source.InputTokensDetails)
	}
}

func mergeInputTokenDetails(target *dto.InputTokenDetails, source *dto.InputTokenDetails) {
	if target == nil || source == nil {
		return
	}
	fillPositiveSyncInt(&target.CachedTokens, source.CachedTokens)
	fillPositiveSyncInt(&target.CachedCreationTokens, source.CachedCreationTokens)
	fillPositiveSyncInt(&target.TextTokens, source.TextTokens)
	fillPositiveSyncInt(&target.AudioTokens, source.AudioTokens)
	fillPositiveSyncInt(&target.ImageTokens, source.ImageTokens)
}

func fillPositiveSyncInt(target *int, source int) {
	if target != nil && *target == 0 && source > 0 {
		*target = source
	}
}

func hasImageHandleSyncUsage(usage *imageHandleSyncUsage) bool {
	if usage == nil {
		return false
	}
	if firstPositiveSyncInt(
		usage.TotalTokens,
		usage.InputTokens,
		usage.OutputTokens,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.CachedTokens,
		usage.CacheReadTokens,
		usage.PromptCacheHitTokens,
		usage.CacheCreationTokens,
		usage.CacheCreationInputTokens,
		usage.CacheCreation5mTokens,
		usage.CacheCreation1hTokens,
		usage.ImageTokens,
		usage.AudioTokens,
	) > 0 {
		return true
	}
	if usage.PromptTokensDetails != nil && inputTokenDetailsHasUsage(*usage.PromptTokensDetails) {
		return true
	}
	return usage.InputTokensDetails != nil && inputTokenDetailsHasUsage(*usage.InputTokensDetails)
}

func inputTokenDetailsHasUsage(details dto.InputTokenDetails) bool {
	return firstPositiveSyncInt(
		details.CachedTokens,
		details.CachedCreationTokens,
		details.TextTokens,
		details.AudioTokens,
		details.ImageTokens,
	) > 0
}

func imageHandleSyncUsageToDTO(usage *imageHandleSyncUsage) *dto.Usage {
	if usage == nil {
		return &dto.Usage{UsageSource: "image_handle_sync"}
	}
	inputTokens := firstPositiveSyncInt(usage.InputTokens, usage.PromptTokens)
	outputTokens := firstPositiveSyncInt(usage.OutputTokens, usage.CompletionTokens)
	totalTokens := firstPositiveSyncInt(usage.TotalTokens, inputTokens+outputTokens)
	promptDetails := syncUsageInputDetails(usage.PromptTokensDetails)
	inputDetails := syncUsageInputDetails(usage.InputTokensDetails)
	cachedTokens := firstPositiveSyncInt(usage.CachedTokens, usage.CacheReadTokens, usage.PromptCacheHitTokens, promptDetails.CachedTokens, inputDetails.CachedTokens)
	cacheCreationTokens := firstPositiveSyncInt(usage.CacheCreationTokens, usage.CacheCreationInputTokens, promptDetails.CachedCreationTokens, inputDetails.CachedCreationTokens)
	textTokens := firstPositiveSyncInt(promptDetails.TextTokens, inputDetails.TextTokens)
	imageTokens := firstPositiveSyncInt(usage.ImageTokens, promptDetails.ImageTokens, inputDetails.ImageTokens)
	audioTokens := firstPositiveSyncInt(usage.AudioTokens, promptDetails.AudioTokens, inputDetails.AudioTokens)
	normalizedInputDetails := dto.InputTokenDetails{
		CachedTokens:         cachedTokens,
		CachedCreationTokens: cacheCreationTokens,
		TextTokens:           textTokens,
		ImageTokens:          imageTokens,
		AudioTokens:          audioTokens,
	}
	return &dto.Usage{
		PromptTokens:                inputTokens,
		CompletionTokens:            outputTokens,
		TotalTokens:                 totalTokens,
		InputTokens:                 inputTokens,
		OutputTokens:                outputTokens,
		UsageSource:                 "image_handle_sync",
		PromptTokensDetails:         normalizedInputDetails,
		InputTokensDetails:          &normalizedInputDetails,
		ClaudeCacheCreation5mTokens: usage.CacheCreation5mTokens,
		ClaudeCacheCreation1hTokens: usage.CacheCreation1hTokens,
	}
}

func syncUsageInputDetails(details *dto.InputTokenDetails) dto.InputTokenDetails {
	if details == nil {
		return dto.InputTokenDetails{}
	}
	return *details
}

func initImageHandleSyncTask(info *relaycommon.RelayInfo, request dto.ImageRequest, taskID string, action string) *model.Task {
	now := common.GetTimestamp()
	task := &model.Task{
		TaskID:     taskID,
		Platform:   constant.TaskPlatform(fmt.Sprintf("%d", info.ChannelType)),
		UserId:     info.UserId,
		Group:      info.UsingGroup,
		ChannelId:  info.ChannelId,
		Action:     action,
		Status:     model.TaskStatusSuccess,
		SubmitTime: now,
		StartTime:  now,
		FinishTime: now,
		Progress:   "100%",
		Properties: model.Properties{
			Input:             request.Prompt,
			OriginModelName:   info.OriginModelName,
			UpstreamModelName: info.UpstreamModelName,
		},
		PrivateData: model.TaskPrivateData{},
	}
	return task
}

func imageHandleSyncTaskData(syncResp imageHandleSyncResponse) []byte {
	payload := map[string]any{
		"result": map[string]any{
			"images": syncResp.ResultImagesForData(),
		},
		"provider_task_id":   syncResp.ProviderTaskID,
		"status":             syncResp.Status,
		"result_data_format": syncResp.ResultDataFormat,
	}
	data, _ := common.Marshal(payload)
	return data
}

func (r imageHandleSyncResponse) ResultImagesForData() []map[string]any {
	if r.Result == nil {
		return nil
	}
	images := make([]map[string]any, 0, len(r.Result.Images))
	for _, image := range r.Result.Images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		item := map[string]any{"url": strings.TrimSpace(image.URL)}
		if image.MimeType != "" {
			item["mime_type"] = image.MimeType
		}
		if image.Filename != "" {
			item["filename"] = image.Filename
		}
		if image.Width > 0 {
			item["width"] = image.Width
		}
		if image.Height > 0 {
			item["height"] = image.Height
		}
		images = append(images, item)
	}
	return images
}

func createImageHandleSyncAssets(ctx *gin.Context, task *model.Task, syncResp imageHandleSyncResponse) error {
	if syncResp.Result == nil || len(syncResp.Result.Images) == 0 {
		return nil
	}
	inputs := make([]model.AssetCreateInput, 0, len(syncResp.Result.Images))
	for _, image := range syncResp.Result.Images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		inputs = append(inputs, model.AssetCreateInput{
			Task:       task,
			AssetIndex: len(inputs),
			AssetType:  model.AssetTypeImage,
			URL:        image.URL,
			MimeType:   image.MimeType,
			Filename:   image.Filename,
			Width:      image.Width,
			Height:     image.Height,
			Metadata: model.AssetMetadata{
				"source":           "image_handle_sync",
				"provider_task_id": firstNonEmpty(syncResp.ProviderTaskID, syncResp.TaskID),
				"client_task_id":   syncResp.ClientTaskID,
				"execution_mode":   "image_handle_sync",
			},
		})
	}
	return model.CreateAssetsForTask(inputs)
}

func imageHandleSyncFailedError(syncResp imageHandleSyncResponse) *types.NewAPIError {
	errBody := syncResp.Error
	message := "image-handle sync task failed"
	code := "image_handle_sync_failed"
	errorType := string(types.ErrorTypeUpstreamError)
	param := ""
	if errBody != nil {
		if value := imageHandleSyncBestErrorMessage(errBody); value != "" {
			message = value
		}
		if errBody.Code != "" {
			code = errBody.Code
		}
		if errBody.ProviderErrorType != "" {
			errorType = errBody.ProviderErrorType
		}
		param = errBody.ProviderErrorParam
	}
	openAIError := types.OpenAIError{
		Message: message,
		Type:    errorType,
		Param:   param,
		Code:    code,
	}
	if detail := imageHandleSyncErrorDetailFromResponse(syncResp); detail != nil {
		if metadata, err := common.Marshal(detail); err == nil {
			apiErr := types.WithOpenAIError(openAIError, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
			apiErr.Metadata = metadata
			return apiErr
		}
	}
	return types.WithOpenAIError(openAIError, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
}

func imageHandleSyncErrorResponse(code string, message string, statusCode int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(fmt.Errorf("%s", message), types.ErrorCode(code), statusCode, types.ErrOptionWithSkipRetry())
}

func imageHandleSyncStatusError(c *gin.Context, statusCode int, responseBody []byte) *types.NewAPIError {
	if statusCode == http.StatusAccepted {
		recordImageHandleSyncErrorDetailFromBody(c, responseBody)
		return imageHandleSyncErrorResponse("image_handle_sync_timeout", "image-handle sync wait timeout", http.StatusGatewayTimeout)
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		recordImageHandleSyncErrorDetailFromBody(c, responseBody)
		return types.NewOpenAIError(fmt.Errorf("image-handle sync returned %d: %s", statusCode, string(responseBody)), types.ErrorCodeBadResponseStatusCode, statusCode)
	}
	return nil
}

func imageHandleSyncBestErrorMessage(errBody *imageHandleSyncError) string {
	if errBody == nil {
		return ""
	}
	if strings.TrimSpace(errBody.ProviderErrorMessage) != "" {
		return strings.TrimSpace(errBody.ProviderErrorMessage)
	}
	return strings.TrimSpace(errBody.Message)
}

func recordImageHandleSyncErrorDetail(c *gin.Context, syncResp imageHandleSyncResponse) {
	detail := imageHandleSyncErrorDetailFromResponse(syncResp)
	if detail == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyImageHandleSyncErrorDetail, imageHandleSyncErrorDetailToMap(*detail))
	if detailBytes, err := common.Marshal(detail); err == nil {
		logger.LogError(c, "image-handle sync error detail: "+common.MaskSensitiveInfo(string(detailBytes)))
	}
}

func recordImageHandleSyncErrorDetailFromBody(c *gin.Context, responseBody []byte) {
	if len(responseBody) == 0 {
		return
	}
	var syncResp imageHandleSyncResponse
	if err := common.Unmarshal(responseBody, &syncResp); err != nil || syncResp.Error == nil {
		return
	}
	recordImageHandleSyncErrorDetail(c, syncResp)
}

func imageHandleSyncErrorDetailFromResponse(syncResp imageHandleSyncResponse) *imageHandleSyncErrorDetail {
	if syncResp.Error == nil {
		return nil
	}
	errBody := syncResp.Error
	detail := &imageHandleSyncErrorDetail{
		Code:                     errBody.Code,
		Message:                  errBody.Message,
		Retryable:                errBody.Retryable,
		UpstreamStatus:           errBody.UpstreamStatus,
		ProviderErrorCode:        errBody.ProviderErrorCode,
		ProviderErrorType:        errBody.ProviderErrorType,
		ProviderErrorMessage:     errBody.ProviderErrorMessage,
		ProviderErrorParam:       errBody.ProviderErrorParam,
		UpstreamError:            errBody.UpstreamError,
		RawResponseTruncated:     syncResp.RawResponseTruncated,
		RawResponseOmittedFields: syncResp.RawResponseOmittedFields,
		ProviderTaskID:           firstNonEmpty(syncResp.ProviderTaskID, syncResp.TaskID),
		ClientTaskID:             syncResp.ClientTaskID,
		TaskID:                   syncResp.TaskID,
		ResultDataFormat:         syncResp.ResultDataFormat,
	}
	detail.RawResponseSummary = imageHandleSyncRawResponseSummary(syncResp.RawResponse)
	return detail
}

func imageHandleSyncRawResponseSummary(raw any) any {
	if raw == nil {
		return nil
	}
	data, err := common.Marshal(raw)
	if err != nil || len(data) == 0 {
		return nil
	}
	const maxBytes = 8192
	if len(data) <= maxBytes {
		var value any
		if err := common.Unmarshal(data, &value); err == nil {
			return value
		}
		return string(data)
	}
	return map[string]any{
		"omitted": true,
		"bytes":   len(data),
		"reason":  "raw_response_summary_too_large",
	}
}

func imageHandleSyncErrorDetailToMap(detail imageHandleSyncErrorDetail) map[string]any {
	data, err := common.Marshal(detail)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := common.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func firstPositiveSyncInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
