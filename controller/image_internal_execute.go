package controller

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type imageInternalExecuteRequest struct {
	ProviderTaskID string `json:"provider_task_id"`
	Attempt        int    `json:"attempt"`
}

type imageInternalExecuteResponse struct {
	Status string                      `json:"status"`
	Images []imageInternalExecuteImage `json:"images,omitempty"`
	Usage  *imageInternalExecuteUsage  `json:"usage,omitempty"`
	Error  *imageInternalExecuteError  `json:"error,omitempty"`
}

type imageInternalExecuteImage struct {
	URL      string `json:"url,omitempty"`
	B64JSON  string `json:"b64_json,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

type imageInternalExecuteUsage struct {
	ActualQuota int `json:"actual_quota,omitempty"`
	TotalTokens int `json:"total_tokens,omitempty"`
}

type imageInternalExecuteError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func ImageTaskInternalExecute(c *gin.Context) {
	rawBody, ok := verifyImageHandleInternalExecute(c)
	if !ok {
		return
	}
	var req imageInternalExecuteRequest
	if len(rawBody) > 0 {
		if err := common.Unmarshal(rawBody, &req); err != nil {
			c.JSON(http.StatusBadRequest, imageInternalExecuteFailed("invalid_request", "invalid execute body", false))
			return
		}
	}
	taskID := c.Param("task_id")
	task, exists, err := model.GetByOnlyTaskId(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, imageInternalExecuteFailed("task_query_failed", err.Error(), true))
		return
	}
	if !exists || task == nil || task.Platform != imageHandleTaskPlatform() {
		c.JSON(http.StatusNotFound, imageInternalExecuteFailed("task_not_found", "task not found", false))
		return
	}
	if task.PrivateData.ImageRequest == nil || len(task.PrivateData.ImageRequest) == 0 {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("missing_image_request", "task image request is missing", false))
		return
	}

	if req.ProviderTaskID != "" && task.PrivateData.ImageHandleProviderTask != "" && req.ProviderTaskID != task.PrivateData.ImageHandleProviderTask {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("provider_task_mismatch", "provider_task_id does not match task", false))
		return
	}
	if len(task.PrivateData.ImageExecuteResponse) > 0 {
		var cached imageInternalExecuteResponse
		if err := common.Unmarshal(task.PrivateData.ImageExecuteResponse, &cached); err == nil && cached.Status != "" {
			c.JSON(http.StatusOK, cached)
			return
		}
	}
	if task.Status == model.TaskStatusSuccess {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("task_terminal", "task already succeeded", false))
		return
	}
	if task.Status == model.TaskStatusFailure {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("task_terminal", "task already failed", false))
		return
	}
	if task.Status == model.TaskStatusInProgress {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("task_already_running", "task execution is already claimed", false))
		return
	}
	if req.ProviderTaskID != "" && task.PrivateData.ImageHandleProviderTask == "" {
		task.PrivateData.ImageHandleProviderTask = req.ProviderTaskID
	}
	eventID := strings.TrimSpace(c.GetHeader("X-ImageHandle-Event-Id"))
	if eventID != "" && task.PrivateData.ExecuteEventID == eventID {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("duplicate_execute_event", "execute event already accepted", false))
		return
	}
	if eventID != "" {
		task.PrivateData.ExecuteEventID = eventID
	}

	previousStatus := task.Status
	task.Status = model.TaskStatusInProgress
	task.Progress = "1%"
	if task.StartTime == 0 {
		task.StartTime = time.Now().Unix()
	}
	won, err := task.UpdateWithStatus(previousStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, imageInternalExecuteFailed("task_update_failed", err.Error(), true))
		return
	}
	if !won {
		c.JSON(http.StatusConflict, imageInternalExecuteFailed("task_already_running", "task execution is already claimed", false))
		return
	}

	resp, usage, execErr := executeImageTaskWithLockedChannel(c, task)
	if execErr != nil {
		if execErr.Retryable {
			releaseImageTaskExecutionClaim(task, previousStatus)
		} else {
			cacheImageTaskExecuteResponse(task, imageInternalExecuteFailed(execErr.Code, execErr.Message, execErr.Retryable))
		}
		c.JSON(http.StatusOK, imageInternalExecuteFailed(execErr.Code, execErr.Message, execErr.Retryable))
		return
	}
	result := imageInternalExecuteResponse{
		Status: "succeeded",
		Images: resp,
	}
	if usage != nil {
		result.Usage = usage
	}
	cacheImageTaskExecuteResponse(task, &result)
	c.JSON(http.StatusOK, result)
}

func cacheImageTaskExecuteResponse(task *model.Task, response *imageInternalExecuteResponse) {
	if task == nil || response == nil {
		return
	}
	data, err := common.Marshal(response)
	if err != nil {
		common.SysError("marshal image task execute response error: " + err.Error())
		return
	}
	task.PrivateData.ImageExecuteResponse = data
	if _, err := task.UpdateWithStatus(task.Status); err != nil {
		common.SysError("cache image task execute response error: " + err.Error())
	}
}

func releaseImageTaskExecutionClaim(task *model.Task, previousStatus model.TaskStatus) {
	if task == nil {
		return
	}
	currentStatus := task.Status
	task.Status = previousStatus
	task.PrivateData.ExecuteEventID = ""
	if previousStatus == model.TaskStatusSubmitted {
		task.Progress = "0%"
	} else if previousStatus == model.TaskStatusQueued {
		task.Progress = "0%"
	}
	_, err := task.UpdateWithStatus(currentStatus)
	if err != nil {
		common.SysError("release image task execution claim error: " + err.Error())
	}
}

func verifyImageHandleInternalExecute(c *gin.Context) ([]byte, bool) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read execute body failed"})
		return nil, false
	}
	timestamp := strings.TrimSpace(c.GetHeader("X-ImageHandle-Timestamp"))
	signature := strings.TrimSpace(c.GetHeader("X-ImageHandle-Signature"))
	secretID := strings.TrimSpace(c.GetHeader("X-ImageHandle-Secret-Id"))
	if timestamp == "" || signature == "" || secretID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing execute signature headers"})
		return nil, false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid execute timestamp"})
		return nil, false
	}
	now := time.Now().Unix()
	if now-ts > callbackTimestampWindowSeconds || ts-now > callbackTimestampWindowSeconds {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "execute timestamp expired"})
		return nil, false
	}
	secret, ok := service.ResolveImageHandleInternalSecret(secretID)
	if !ok || secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "execute secret not found"})
		return nil, false
	}
	expected := signCallbackPayload(timestamp, rawBody, secret)
	if !constantTimeEqualHex(signature, expected) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid execute signature"})
		return nil, false
	}
	return rawBody, true
}

func constantTimeEqualHex(got, expected string) bool {
	got = strings.ToLower(strings.TrimSpace(got))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func executeImageTaskWithLockedChannel(c *gin.Context, task *model.Task) ([]imageInternalExecuteImage, *imageInternalExecuteUsage, *imageInternalExecuteError) {
	channelModel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("channel_not_found", err.Error(), true)
	}
	if channelModel.Status != common.ChannelStatusEnabled {
		return nil, nil, newImageInternalExecuteError("channel_disabled", "locked channel is disabled", false)
	}
	key, index, apiErr := channelModel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, nil, newImageInternalExecuteError("channel_no_available_key", apiErr.Error(), true)
	}
	setupImageExecuteContext(c, task, channelModel, key, index)

	var imageReq dto.ImageRequest
	if err := common.Unmarshal(task.PrivateData.ImageRequest, &imageReq); err != nil {
		return nil, nil, newImageInternalExecuteError("invalid_image_request", err.Error(), false)
	}
	if task.Action == constant.TaskActionImageEdit {
		if err := prepareImageEditMultipartRequest(c, task, imageReq); err != nil {
			return nil, nil, newImageInternalExecuteError("prepare_image_edit_failed", err.Error(), false)
		}
	}
	relayInfo := relaycommon.GenRelayInfoImage(c, &imageReq)
	relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	relayInfo.InitChannelMeta(c)
	relayInfo.OriginModelName = task.Properties.OriginModelName
	if relayInfo.OriginModelName == "" {
		relayInfo.OriginModelName = imageReq.Model
	}
	relayInfo.UpstreamModelName = task.Properties.UpstreamModelName
	if relayInfo.UpstreamModelName == "" {
		relayInfo.UpstreamModelName = imageReq.Model
	}
	imageReq.Model = relayInfo.UpstreamModelName
	if err := helper.ModelMappedHelper(c, relayInfo, &imageReq); err != nil {
		return nil, nil, newImageInternalExecuteError("model_mapping_failed", err.Error(), false)
	}

	adaptor := relay.GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		return nil, nil, newImageInternalExecuteError("invalid_api_type", fmt.Sprintf("invalid api type: %d", relayInfo.ApiType), false)
	}
	adaptor.Init(relayInfo)
	convertedRequest, err := adaptor.ConvertImageRequest(c, relayInfo, imageReq)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("convert_request_failed", err.Error(), false)
	}
	requestBody, err := buildImageExecuteRequestBody(convertedRequest, relayInfo)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("build_request_failed", err.Error(), false)
	}
	httpRespAny, err := adaptor.DoRequest(c, relayInfo, requestBody)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("do_request_failed", err.Error(), true)
	}
	httpResp, ok := httpRespAny.(*http.Response)
	if !ok || httpResp == nil {
		return nil, nil, newImageInternalExecuteError("invalid_upstream_response", "upstream response is empty", true)
	}
	defer httpResp.Body.Close()
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("read_response_failed", err.Error(), true)
	}
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		retryable := httpResp.StatusCode == http.StatusTooManyRequests || httpResp.StatusCode/100 == 5
		return nil, nil, newImageInternalExecuteError("upstream_error", string(responseBody), retryable)
	}
	images, usage, err := parseInternalExecuteImageResponse(responseBody)
	if err != nil {
		return nil, nil, newImageInternalExecuteError("parse_response_failed", err.Error(), false)
	}
	if len(images) == 0 {
		return nil, nil, newImageInternalExecuteError("empty_image_result", "upstream returned no images", false)
	}
	return images, usage, nil
}

func setupImageExecuteContext(c *gin.Context, task *model.Task, channelModel *model.Channel, key string, keyIndex int) {
	common.SetContextKey(c, constant.ContextKeyChannelId, channelModel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channelModel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channelModel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channelModel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channelModel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channelModel.GetOtherSettings())
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, channelModel.GetParamOverride())
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, channelModel.GetHeaderOverride())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channelModel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channelModel.GetStatusCodeMapping())
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channelModel.GetBaseURL())
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "")
	if channelModel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, keyIndex)
	} else {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	relayMode := relayconstant.RelayModeImagesGenerations
	requestPath := "/v1/images/generations"
	contentType := "application/json"
	if task != nil && task.Action == constant.TaskActionImageEdit {
		relayMode = relayconstant.RelayModeImagesEdits
		requestPath = "/v1/images/edits"
		contentType = c.Request.Header.Get("Content-Type")
	}
	c.Set("relay_mode", relayMode)
	c.Request.URL.Path = requestPath
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
}

func prepareImageEditMultipartRequest(c *gin.Context, task *model.Task, imageReq dto.ImageRequest) error {
	if len(task.PrivateData.ImageInputURLs) == 0 {
		return fmt.Errorf("image edit requires input.images")
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	writeMultipartField(writer, "model", imageReq.Model)
	writeMultipartField(writer, "prompt", imageReq.Prompt)
	writeMultipartField(writer, "size", imageReq.Size)
	writeMultipartField(writer, "quality", imageReq.Quality)
	writeMultipartField(writer, "response_format", imageReq.ResponseFormat)
	if imageReq.N != nil {
		writeMultipartField(writer, "n", strconv.FormatUint(uint64(*imageReq.N), 10))
	}
	writeMultipartRawField(writer, "background", imageReq.Background)
	writeMultipartRawField(writer, "moderation", imageReq.Moderation)
	writeMultipartRawField(writer, "output_format", imageReq.OutputFormat)
	writeMultipartRawField(writer, "output_compression", imageReq.OutputCompression)

	for i, imageURL := range task.PrivateData.ImageInputURLs {
		fieldName := "image"
		if len(task.PrivateData.ImageInputURLs) > 1 {
			fieldName = "image[]"
		}
		if err := addRemoteImageToMultipart(writer, fieldName, imageURL, fmt.Sprintf("image_%d", i)); err != nil {
			return err
		}
	}
	if task.PrivateData.ImageMaskURL != "" {
		if err := addRemoteImageToMultipart(writer, "mask", task.PrivateData.ImageMaskURL, "mask"); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(requestBody.Bytes()))
	c.Request.ContentLength = int64(requestBody.Len())
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := c.Request.ParseMultipartForm(int64(requestBody.Len()) + 1024); err != nil {
		return err
	}
	return nil
}

func writeMultipartField(writer *multipart.Writer, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	_ = writer.WriteField(key, value)
}

func writeMultipartRawField(writer *multipart.Writer, key string, raw []byte) {
	value := rawFormValue(raw)
	if value == "" {
		return
	}
	_ = writer.WriteField(key, value)
}

func rawFormValue(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func addRemoteImageToMultipart(writer *multipart.Writer, fieldName, imageURL, fallbackName string) error {
	resp, err := service.DoDownloadRequest(imageURL, "image_handle_internal_execute_edit")
	if err != nil {
		return fmt.Errorf("download %s failed: %w", fieldName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s returned HTTP %d", fieldName, resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "image/png"
	}
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("download %s content type %s is not image/*", fieldName, contentType)
	}
	filename := filenameFromImageURL(imageURL, fallbackName, contentType)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	maxImageSize := int64(constant.MaxFileDownloadMB * 1024 * 1024)
	if maxImageSize <= 0 {
		maxImageSize = 64 * 1024 * 1024
	}
	written, err := io.Copy(part, io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return err
	}
	if written > maxImageSize {
		return fmt.Errorf("download %s exceeds maximum allowed size of %d bytes", fieldName, maxImageSize)
	}
	return nil
}

func filenameFromImageURL(imageURL, fallbackName, contentType string) string {
	parsed, err := url.Parse(imageURL)
	if err == nil {
		base := path.Base(parsed.Path)
		if base != "." && base != "/" && base != "" {
			return base
		}
	}
	ext := ".png"
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	return fallbackName + ext
}

func buildImageExecuteRequestBody(convertedRequest any, info *relaycommon.RelayInfo) (io.Reader, error) {
	switch body := convertedRequest.(type) {
	case *bytes.Buffer:
		return body, nil
	case io.Reader:
		return body, nil
	default:
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return nil, err
		}
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return nil, err
			}
		}
		return bytes.NewReader(jsonData), nil
	}
}

func parseInternalExecuteImageResponse(body []byte) ([]imageInternalExecuteImage, *imageInternalExecuteUsage, error) {
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(body, &imageResp); err != nil {
		return nil, nil, err
	}
	images := make([]imageInternalExecuteImage, 0, len(imageResp.Data))
	for _, item := range imageResp.Data {
		images = append(images, imageInternalExecuteImage{
			URL:      item.Url,
			B64JSON:  item.B64Json,
			MimeType: mimeTypeFromImageResponse(imageResp),
		})
	}
	var usage *imageInternalExecuteUsage
	if imageResp.Usage != nil {
		usage = &imageInternalExecuteUsage{
			ActualQuota: imageResp.Usage.TotalTokens,
			TotalTokens: imageResp.Usage.TotalTokens,
		}
	}
	return images, usage, nil
}

func mimeTypeFromImageResponse(resp dto.ImageResponse) string {
	switch strings.ToLower(strings.TrimSpace(resp.OutputFormat)) {
	case "webp":
		return "image/webp"
	case "jpeg", "jpg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

func imageInternalExecuteFailed(code, message string, retryable bool) *imageInternalExecuteResponse {
	return &imageInternalExecuteResponse{
		Status: "failed",
		Error:  newImageInternalExecuteError(code, message, retryable),
	}
}

func newImageInternalExecuteError(code, message string, retryable bool) *imageInternalExecuteError {
	return &imageInternalExecuteError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}
