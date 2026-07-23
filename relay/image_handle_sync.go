package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	Output      map[string]any         `json:"output,omitempty"`
	Metadata    map[string]any         `json:"metadata,omitempty"`
	RawResponse any                    `json:"raw_response,omitempty"`
}

type imageHandleSyncImage struct {
	URL           string `json:"url"`
	B64Json       string `json:"b64_json,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	Format        string `json:"format,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Filename      string `json:"filename,omitempty"`
	SizeBytes     int64  `json:"size_bytes,omitempty"`
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
	Type                 string `json:"type,omitempty"`
	Param                string `json:"param,omitempty"`
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

type imageHandleUploadResponse struct {
	Images []string `json:"images,omitempty"`
	Mask   *string  `json:"mask,omitempty"`
}

type imageHandleBase64UploadRequest struct {
	Uploads []imageHandleBase64UploadItem `json:"uploads"`
}

type imageHandleBase64UploadItem struct {
	Field    string `json:"field"`
	Filename string `json:"filename,omitempty"`
	B64Json  string `json:"b64_json"`
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
	return info != nil && request != nil
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
	common.SetContextKey(c, constant.ContextKeyImageHandleSyncClientTaskID, clientTaskID)
	action := constant.TaskActionImageGeneration
	operation := "generation"
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		action = constant.TaskActionImageEdit
		operation = "edit"
	}
	request, err := applyImageHandleSyncParamOverride(info, request)
	if err != nil {
		return nil, newAPIErrorFromParamOverride(err)
	}
	task := initImageHandleSyncTask(info, request, clientTaskID, action)
	lease := model.NewImageCredentialLease(task, operation, info.UpstreamModelName, 1800)
	lease.TaskRecordID = 0
	common.SetContextKey(c, constant.ContextKeyImageHandleSyncCredentialLeaseID, lease.LeaseID)
	if err := model.CreateImageCredentialLease(lease); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeUpdateDataError, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}

	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		normalizedRequest, err := normalizeImageHandleSyncEditRequest(c, request)
		if err != nil {
			_ = model.MarkImageCredentialLeaseFailed(lease.LeaseID)
			return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
		}
		request = normalizedRequest
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
	recordImageHandleSyncTrace(c, syncResp)
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
	service.CaptureImageExecutionAuditFromJSON(c, responseBody, imageResp.Usage)
	imageN := uint(1)
	if request.N != nil {
		imageN = *request.N
	}
	normalizeImageUsage(imageResp.Usage, imageN)
	jsonResponse, err := marshalImageHandleSyncClientResponse(imageResp)
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
	return usage, nil
}

func applyImageHandleSyncParamOverride(info *relaycommon.RelayInfo, request dto.ImageRequest) (dto.ImageRequest, error) {
	if info == nil || info.ChannelMeta == nil || len(info.ParamOverride) == 0 {
		return request, nil
	}

	requestJSON, err := common.Marshal(request)
	if err != nil {
		return request, err
	}
	var requestFields map[string]json.RawMessage
	if err := common.Unmarshal(requestJSON, &requestFields); err != nil {
		return request, err
	}
	for key, value := range request.Extra {
		if len(value) == 0 {
			continue
		}
		if _, exists := requestFields[key]; !exists {
			requestFields[key] = value
		}
	}
	requestJSON, err = common.Marshal(requestFields)
	if err != nil {
		return request, err
	}
	requestJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(requestJSON, info)
	if err != nil {
		return request, err
	}
	requestJSON, err = restoreImagePricingParameters(requestJSON, info.PriceData.ImagePricing)
	if err != nil {
		return request, err
	}

	overridden := dto.ImageRequest{}
	if err := common.Unmarshal(requestJSON, &overridden); err != nil {
		return request, err
	}
	return overridden, nil
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
	parameters := imageHandleSyncParameters(request)
	cfg := service.GetImageHandleExecutorConfig()
	payload := imageHandleSyncRequest{
		RequestID:        c.GetString(common.RequestIdKey),
		ClientTaskID:     clientTaskID,
		Model:            info.UpstreamModelName,
		Operation:        operation,
		ResultDataFormat: imageHandleSyncResultDataFormat(request),
		Input:            imageHandleSyncInput{Text: request.Prompt, Images: images, Mask: mask},
		Parameters:       parameters,
		ProviderOptions:  imageHandleSyncProviderOptions(request, info.UpstreamModelName),
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
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func normalizeImageHandleSyncEditRequest(c *gin.Context, request dto.ImageRequest) (dto.ImageRequest, error) {
	if imageHandleSyncEditInputsAreURLs(request) {
		return request, nil
	}
	if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		uploadResp, err := uploadImageHandleSyncEditMultipart(c)
		if err != nil {
			return request, err
		}
		return applyImageHandleUploadResponseToRequest(request, uploadResp)
	}
	uploadReq, ok, err := imageHandleBase64UploadsFromRequest(request)
	if err != nil {
		return request, err
	}
	if !ok {
		return request, fmt.Errorf("image-handle sync image edits require URL, multipart file, or base64 image input")
	}
	uploadResp, err := uploadImageHandleSyncEditBase64(c, uploadReq)
	if err != nil {
		return request, err
	}
	return applyImageHandleUploadResponseToRequest(request, uploadResp)
}

func uploadImageHandleSyncEditMultipart(c *gin.Context) (imageHandleUploadResponse, error) {
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return imageHandleUploadResponse{}, fmt.Errorf("parse image edit multipart for image-handle upload failed: %w", err)
	}
	if formData == nil || len(formData.File) == 0 {
		return imageHandleUploadResponse{}, fmt.Errorf("image-handle sync image edits require at least one multipart image file")
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	fileCount := 0
	for fieldName, files := range formData.File {
		normalizedField := imageHandleUploadFieldName(fieldName)
		if normalizedField == "" {
			continue
		}
		for _, fileHeader := range files {
			if fileHeader == nil {
				continue
			}
			file, err := fileHeader.Open()
			if err != nil {
				_ = writer.Close()
				return imageHandleUploadResponse{}, fmt.Errorf("open image-handle upload file %s failed: %w", fileHeader.Filename, err)
			}
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, normalizedField, fileHeader.Filename))
			contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			header.Set("Content-Type", contentType)
			part, err := writer.CreatePart(header)
			if err != nil {
				_ = file.Close()
				_ = writer.Close()
				return imageHandleUploadResponse{}, fmt.Errorf("create image-handle upload part failed: %w", err)
			}
			if _, err := io.Copy(part, file); err != nil {
				_ = file.Close()
				_ = writer.Close()
				return imageHandleUploadResponse{}, fmt.Errorf("copy image-handle upload file failed: %w", err)
			}
			_ = file.Close()
			fileCount++
		}
	}
	if fileCount == 0 {
		_ = writer.Close()
		return imageHandleUploadResponse{}, fmt.Errorf("image-handle sync image edits require image or mask multipart files")
	}
	if err := writer.Close(); err != nil {
		return imageHandleUploadResponse{}, fmt.Errorf("close image-handle upload multipart body failed: %w", err)
	}
	return postImageHandleUpload(c, "/v1/image/uploads", writer.FormDataContentType(), requestBody.Bytes())
}

func uploadImageHandleSyncEditBase64(c *gin.Context, uploadReq imageHandleBase64UploadRequest) (imageHandleUploadResponse, error) {
	body, err := common.Marshal(uploadReq)
	if err != nil {
		return imageHandleUploadResponse{}, err
	}
	return postImageHandleUpload(c, "/v1/image/uploads/base64", "application/json", body)
}

func postImageHandleUpload(c *gin.Context, path string, contentType string, body []byte) (imageHandleUploadResponse, error) {
	cfg := service.GetImageHandleExecutorConfig()
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return imageHandleUploadResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", contentType)
	client, err := service.GetHttpClientWithProxy(c.GetString("channel_proxy"))
	if err != nil {
		return imageHandleUploadResponse{}, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return imageHandleUploadResponse{}, err
	}
	defer service.CloseResponseBodyGracefully(resp)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return imageHandleUploadResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		recordImageHandleUploadErrorDetail(c, resp.StatusCode, respBody)
		return imageHandleUploadResponse{}, fmt.Errorf("image-handle upload failed: status_code=%d, body=%s", resp.StatusCode, common.MaskSensitiveInfo(string(respBody)))
	}
	var uploadResp imageHandleUploadResponse
	if err := common.Unmarshal(respBody, &uploadResp); err != nil {
		return imageHandleUploadResponse{}, fmt.Errorf("unmarshal image-handle upload response failed: %w", err)
	}
	if len(uploadResp.Images) == 0 && (uploadResp.Mask == nil || strings.TrimSpace(*uploadResp.Mask) == "") {
		return imageHandleUploadResponse{}, fmt.Errorf("image-handle upload returned empty result")
	}
	return uploadResp, nil
}

func imageHandleBase64UploadsFromRequest(request dto.ImageRequest) (imageHandleBase64UploadRequest, bool, error) {
	uploads := make([]imageHandleBase64UploadItem, 0)
	if len(request.Image) > 0 {
		items, err := imageHandleBase64UploadItemsFromRaw(request.Image, "image")
		if err != nil {
			return imageHandleBase64UploadRequest{}, false, err
		}
		uploads = append(uploads, items...)
	}
	if len(request.Extra) > 0 {
		if raw, ok := request.Extra["images"]; ok && len(raw) > 0 {
			items, err := imageHandleBase64UploadItemsFromRaw(raw, "image")
			if err != nil {
				return imageHandleBase64UploadRequest{}, false, err
			}
			uploads = append(uploads, items...)
		}
		if raw, ok := request.Extra["mask"]; ok && len(raw) > 0 {
			items, err := imageHandleBase64UploadItemsFromRaw(raw, "mask")
			if err != nil {
				return imageHandleBase64UploadRequest{}, false, err
			}
			uploads = append(uploads, items...)
		}
	}
	if len(uploads) == 0 {
		return imageHandleBase64UploadRequest{}, false, nil
	}
	return imageHandleBase64UploadRequest{Uploads: uploads}, true, nil
}

func imageHandleBase64UploadItemsFromRaw(raw json.RawMessage, fallbackField string) ([]imageHandleBase64UploadItem, error) {
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("parse image edit base64 input failed: %w", err)
	}
	return imageHandleBase64UploadItemsFromValue(value, fallbackField, fallbackField)
}

func imageHandleBase64UploadItemsFromValue(value any, fallbackField string, fallbackFilenamePrefix string) ([]imageHandleBase64UploadItem, error) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" || isImageHandleSyncURL(typed) {
			return nil, nil
		}
		return []imageHandleBase64UploadItem{{
			Field:    fallbackField,
			Filename: imageHandleBase64UploadFilename(fallbackFilenamePrefix, 1),
			B64Json:  strings.TrimSpace(typed),
		}}, nil
	case []any:
		items := make([]imageHandleBase64UploadItem, 0, len(typed))
		for idx, item := range typed {
			nestedItems, err := imageHandleBase64UploadItemsFromValue(item, fallbackField, fmt.Sprintf("%s-%d", fallbackFilenamePrefix, idx+1))
			if err != nil {
				return nil, err
			}
			items = append(items, nestedItems...)
		}
		return items, nil
	case map[string]any:
		if url := imageHandleStringMapValue(typed, "url"); isImageHandleSyncURL(url) {
			return nil, nil
		}
		data := firstNonEmpty(
			imageHandleStringMapValue(typed, "b64_json"),
			imageHandleStringMapValue(typed, "base64"),
			imageHandleStringMapValue(typed, "data"),
			imageHandleStringMapValue(typed, "image"),
			imageHandleStringMapValue(typed, "url"),
		)
		if data == "" {
			return nil, fmt.Errorf("image edit base64 upload item must include b64_json, base64, data, or image")
		}
		field := firstNonEmpty(imageHandleStringMapValue(typed, "field"), fallbackField)
		return []imageHandleBase64UploadItem{{
			Field:    imageHandleUploadFieldName(field),
			Filename: firstNonEmpty(imageHandleStringMapValue(typed, "filename"), imageHandleBase64UploadFilename(fallbackFilenamePrefix, 1)),
			B64Json:  data,
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported image edit base64 input type")
	}
}

func imageHandleStringMapValue(payload map[string]any, key string) string {
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

func imageHandleBase64UploadFilename(prefix string, index int) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "image"
	}
	return fmt.Sprintf("%s-%d.png", prefix, index)
}

func imageHandleUploadFieldName(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	switch {
	case fieldName == "mask":
		return "mask"
	case fieldName == "", fieldName == "image", fieldName == "image[]", strings.HasPrefix(fieldName, "image["):
		return "image"
	default:
		return fieldName
	}
}

func applyImageHandleUploadResponseToRequest(request dto.ImageRequest, uploadResp imageHandleUploadResponse) (dto.ImageRequest, error) {
	existingImages, existingMask, _ := imageHandleSyncInputsFromRequest(request)
	cleanImages := make([]string, 0, len(existingImages)+len(uploadResp.Images))
	for _, image := range existingImages {
		if !isImageHandleSyncURL(image) {
			continue
		}
		cleanImages = append(cleanImages, strings.TrimSpace(image))
	}
	for _, image := range uploadResp.Images {
		image = strings.TrimSpace(image)
		if image != "" {
			cleanImages = append(cleanImages, image)
		}
	}
	if len(cleanImages) == 0 {
		return request, fmt.Errorf("image-handle upload returned blank image URLs")
	}
	imageBytes, err := common.Marshal(cleanImages)
	if err != nil {
		return request, err
	}
	request.Image = imageBytes
	if request.Extra == nil {
		request.Extra = map[string]json.RawMessage{}
	}
	delete(request.Extra, "images")
	maskValue := ""
	if existingMask != nil && isImageHandleSyncURL(*existingMask) {
		maskValue = strings.TrimSpace(*existingMask)
	}
	if uploadResp.Mask != nil && strings.TrimSpace(*uploadResp.Mask) != "" {
		maskValue = strings.TrimSpace(*uploadResp.Mask)
	}
	if maskValue != "" {
		maskBytes, err := common.Marshal(maskValue)
		if err != nil {
			return request, err
		}
		request.Extra["mask"] = maskBytes
	} else {
		delete(request.Extra, "mask")
	}
	return request, nil
}

func recordImageHandleUploadErrorDetail(c *gin.Context, statusCode int, responseBody []byte) {
	if len(responseBody) == 0 {
		return
	}
	var body any
	if err := common.Unmarshal(responseBody, &body); err != nil {
		body = string(responseBody)
	}
	detail := map[string]any{
		"code":            "image_handle_upload_failed",
		"upstream_status": statusCode,
		"upstream_error":  body,
	}
	common.SetContextKey(c, constant.ContextKeyImageHandleSyncErrorDetail, detail)
	if detailBytes, err := common.Marshal(detail); err == nil {
		logger.LogError(c, "image-handle upload error detail: "+common.MaskSensitiveInfo(string(detailBytes)))
	}
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

func imageHandleSyncParameters(request dto.ImageRequest) map[string]any {
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
	if len(request.Extra) > 0 {
		addRawImageParam(params, "resolution", request.Extra["resolution"])
	}
	addRawImageParam(params, "output_format", request.OutputFormat)
	addRawImageParam(params, "output_compression", request.OutputCompression)
	addRawImageParam(params, "background", request.Background)
	addRawImageParam(params, "moderation", request.Moderation)
	if len(request.Extra) > 0 {
		addRawImageParam(params, "input_fidelity", request.Extra["input_fidelity"])
	}
	if responseFormat := strings.TrimSpace(request.ResponseFormat); responseFormat != "" {
		params["response_format"] = responseFormat
	}
	return params
}

func imageHandleSyncProviderOptions(request dto.ImageRequest, upstreamModelName string) map[string]any {
	if len(request.ExtraFields) == 0 {
		return nil
	}
	var options map[string]any
	if err := common.Unmarshal(request.ExtraFields, &options); err != nil {
		return nil
	}
	if imageHandleSyncIsGPTImageModel(firstNonEmpty(upstreamModelName, request.Model)) {
		delete(options, "seed")
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func imageHandleSyncIsGPTImageModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-")
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

type imageHandleSyncOpenAIImageResponse struct {
	Data         []dto.ImageData                       `json:"data"`
	Created      int64                                 `json:"created"`
	Background   string                                `json:"background,omitempty"`
	OutputFormat string                                `json:"output_format,omitempty"`
	Quality      string                                `json:"quality,omitempty"`
	Size         string                                `json:"size,omitempty"`
	Usage        *imageHandleSyncOpenAICompatibleUsage `json:"usage,omitempty"`
	Metadata     json.RawMessage                       `json:"metadata,omitempty"`
}

type imageHandleSyncOpenAICompatibleUsage struct {
	TotalTokens                 int                    `json:"total_tokens,omitempty"`
	InputTokens                 int                    `json:"input_tokens,omitempty"`
	OutputTokens                int                    `json:"output_tokens,omitempty"`
	UsageSource                 string                 `json:"usage_source,omitempty"`
	InputTokensDetails          *dto.InputTokenDetails `json:"input_tokens_details,omitempty"`
	ClaudeCacheCreation5mTokens int                    `json:"claude_cache_creation_5_m_tokens,omitempty"`
	ClaudeCacheCreation1hTokens int                    `json:"claude_cache_creation_1_h_tokens,omitempty"`
}

func imageHandleSyncClientImageResponse(resp *dto.ImageResponse) imageHandleSyncOpenAIImageResponse {
	if resp == nil {
		return imageHandleSyncOpenAIImageResponse{}
	}
	return imageHandleSyncOpenAIImageResponse{
		Data:         resp.Data,
		Created:      resp.Created,
		Background:   resp.Background,
		OutputFormat: resp.OutputFormat,
		Quality:      resp.Quality,
		Size:         resp.Size,
		Usage:        imageHandleSyncClientUsage(resp.Usage),
		Metadata:     resp.Metadata,
	}
}

func marshalImageHandleSyncClientResponse(resp *dto.ImageResponse) ([]byte, error) {
	return common.MarshalNoEscapeHTML(imageHandleSyncClientImageResponse(resp))
}

func imageHandleSyncClientUsage(usage *dto.Usage) *imageHandleSyncOpenAICompatibleUsage {
	if usage == nil {
		return nil
	}
	inputTokens := firstPositiveSyncInt(usage.InputTokens, usage.PromptTokens)
	outputTokens := firstPositiveSyncInt(usage.OutputTokens, usage.CompletionTokens)
	totalTokens := firstPositiveSyncInt(usage.TotalTokens, inputTokens+outputTokens)
	details := usage.InputTokensDetails
	if details == nil && inputTokenDetailsHasUsage(usage.PromptTokensDetails) {
		promptDetails := usage.PromptTokensDetails
		details = &promptDetails
	}
	return &imageHandleSyncOpenAICompatibleUsage{
		TotalTokens:                 totalTokens,
		InputTokens:                 inputTokens,
		OutputTokens:                outputTokens,
		UsageSource:                 usage.UsageSource,
		InputTokensDetails:          details,
		ClaudeCacheCreation5mTokens: usage.ClaudeCacheCreation5mTokens,
		ClaudeCacheCreation1hTokens: usage.ClaudeCacheCreation1hTokens,
	}
}

func imageHandleSyncOpenAITopFieldsFromResponse(syncResp imageHandleSyncResponse) imageHandleSyncOpenAITopFields {
	fields := imageHandleSyncOpenAITopFields{}
	if syncResp.Result != nil {
		mergeImageHandleSyncOpenAITopFields(&fields, imageHandleSyncOpenAITopFieldsFromMap(syncResp.Result.Output))
		mergeImageHandleSyncOpenAITopFields(&fields, imageHandleSyncOpenAITopFieldsFromRaw(syncResp.Result.RawResponse))
	}
	mergeImageHandleSyncOpenAITopFields(&fields, imageHandleSyncOpenAITopFieldsFromRaw(syncResp.RawResponse))
	return fields
}

func imageHandleSyncOpenAITopFieldsFromMap(payload map[string]any) imageHandleSyncOpenAITopFields {
	fields := imageHandleSyncOpenAITopFields{}
	if payload == nil {
		return fields
	}
	fields.Created = imageHandleSyncInt64Field(payload, "created")
	fields.Background = imageHandleSyncStringField(payload, "background")
	fields.OutputFormat = imageHandleSyncStringField(payload, "output_format")
	fields.Quality = imageHandleSyncStringField(payload, "quality")
	fields.Size = imageHandleSyncStringField(payload, "size")
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
			"images":   syncResp.ResultImagesForData(),
			"output":   syncResp.ResultOutputForData(),
			"metadata": syncResp.ResultMetadataForData(),
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
		if image.Format != "" {
			item["format"] = image.Format
		}
		if image.Filename != "" {
			item["filename"] = image.Filename
		}
		if image.SizeBytes > 0 {
			item["size_bytes"] = image.SizeBytes
		}
		if image.Width > 0 {
			item["width"] = image.Width
		}
		if image.Height > 0 {
			item["height"] = image.Height
		}
		if image.RevisedPrompt != "" {
			item["revised_prompt"] = image.RevisedPrompt
		}
		images = append(images, item)
	}
	return images
}

func (r imageHandleSyncResponse) ResultOutputForData() map[string]any {
	if r.Result == nil || len(r.Result.Output) == 0 {
		return nil
	}
	return r.Result.Output
}

func (r imageHandleSyncResponse) ResultMetadataForData() map[string]any {
	if r.Result == nil || len(r.Result.Metadata) == 0 {
		return nil
	}
	return r.Result.Metadata
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
			SizeBytes:  image.SizeBytes,
			Width:      image.Width,
			Height:     image.Height,
			Metadata:   imageHandleSyncAssetMetadata(syncResp, image),
		})
	}
	return model.CreateAssetsForTask(inputs)
}

func imageHandleSyncAssetMetadata(syncResp imageHandleSyncResponse, image imageHandleSyncImage) model.AssetMetadata {
	metadata := model.AssetMetadata{
		"source":           "image_handle_sync",
		"provider_task_id": firstNonEmpty(syncResp.ProviderTaskID, syncResp.TaskID),
		"client_task_id":   syncResp.ClientTaskID,
		"execution_mode":   "image_handle_sync",
	}
	if image.Format != "" {
		metadata["format"] = image.Format
	}
	if image.RevisedPrompt != "" {
		metadata["revised_prompt"] = image.RevisedPrompt
	}
	if syncResp.Result != nil {
		if len(syncResp.Result.Output) > 0 {
			metadata["output"] = syncResp.Result.Output
		}
		if len(syncResp.Result.Metadata) > 0 {
			metadata["execution"] = syncResp.Result.Metadata
		}
	}
	return metadata
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
		} else if errBody.Type != "" {
			errorType = errBody.Type
		}
		param = firstNonEmpty(errBody.ProviderErrorParam, errBody.Param)
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
		if apiErr := imageHandleSyncOpenAIErrorFromHTTPBody(statusCode, responseBody); apiErr != nil {
			return apiErr
		}
		return types.NewOpenAIError(fmt.Errorf("image-handle sync returned %d: %s", statusCode, string(responseBody)), types.ErrorCodeBadResponseStatusCode, statusCode)
	}
	return nil
}

func imageHandleSyncOpenAIErrorFromHTTPBody(statusCode int, responseBody []byte) *types.NewAPIError {
	if len(responseBody) == 0 {
		return nil
	}
	var body struct {
		Error *imageHandleSyncError `json:"error"`
	}
	if err := common.Unmarshal(responseBody, &body); err != nil || body.Error == nil {
		return nil
	}
	errBody := body.Error
	message := imageHandleSyncBestErrorMessage(errBody)
	if message == "" {
		message = fmt.Sprintf("image-handle sync returned %d", statusCode)
	}
	code := firstNonEmpty(errBody.ProviderErrorCode, errBody.Code, string(types.ErrorCodeBadResponseStatusCode))
	errorType := firstNonEmpty(errBody.ProviderErrorType, errBody.Type, string(types.ErrorTypeUpstreamError))
	openAIError := types.OpenAIError{
		Message: message,
		Type:    errorType,
		Param:   firstNonEmpty(errBody.ProviderErrorParam, errBody.Param),
		Code:    code,
	}
	apiErr := types.WithOpenAIError(openAIError, statusCode, types.ErrOptionWithSkipRetry())
	if metadata := imageHandleSyncHTTPErrorMetadata(statusCode, responseBody); len(metadata) > 0 {
		apiErr.Metadata = metadata
	}
	return apiErr
}

func imageHandleSyncHTTPErrorMetadata(statusCode int, responseBody []byte) json.RawMessage {
	var body any
	if err := common.Unmarshal(responseBody, &body); err != nil {
		body = string(responseBody)
	}
	metadata, err := common.Marshal(map[string]any{
		"image_handle_status": statusCode,
		"image_handle_body":   common.MaskSensitiveValue(body),
	})
	if err != nil {
		return nil
	}
	return metadata
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
	recordImageHandleSyncTrace(c, syncResp)
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
	if err := common.Unmarshal(responseBody, &syncResp); err != nil {
		return
	}
	recordImageHandleSyncTrace(c, syncResp)
	if syncResp.Error == nil {
		return
	}
	recordImageHandleSyncErrorDetail(c, syncResp)
}

func recordImageHandleSyncTrace(c *gin.Context, syncResp imageHandleSyncResponse) {
	if providerTaskID := firstNonEmpty(syncResp.ProviderTaskID, syncResp.TaskID); providerTaskID != "" {
		common.SetContextKey(c, constant.ContextKeyImageHandleSyncProviderTaskID, providerTaskID)
	}
	if common.GetContextKeyString(c, constant.ContextKeyImageHandleSyncClientTaskID) == "" {
		if clientTaskID := strings.TrimSpace(syncResp.ClientTaskID); clientTaskID != "" {
			common.SetContextKey(c, constant.ContextKeyImageHandleSyncClientTaskID, clientTaskID)
		}
	}
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
			return common.MaskSensitiveValue(value)
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
	masked, _ := common.MaskSensitiveValue(result).(map[string]any)
	return masked
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
