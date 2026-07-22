package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const videoProxyTimeout = 10 * time.Minute

var getVideoContentResolver = func(platform constant.TaskPlatform) channel.VideoContentResolver {
	adaptor := relay.GetTaskAdaptor(platform)
	resolver, _ := adaptor.(channel.VideoContentResolver)
	return resolver
}

// videoProxyError returns a standardized OpenAI-style error response.
func videoProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func VideoProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := getVideoProxyTask(c, userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}
	assets, assetErr := model.GetUserAssetsByTaskIDs(task.UserId, []string{task.TaskID})
	if assetErr != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query video assets for task %s: %s", taskID, assetErr.Error()))
	}
	for _, asset := range assets {
		if asset.AssetType == model.AssetTypeVideo {
			streamVideoContent(c, task, asset.URL, asset)
			return
		}
	}
	streamVideoContent(c, task, "")
}

func VideoAssetContent(c *gin.Context) {
	assetID := strings.TrimSpace(c.Param("asset_id"))
	if assetID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "asset_id is required")
		return
	}
	userID := c.GetInt("id")
	var asset *model.Asset
	var exists bool
	var err error
	if isVideoProxyAdmin(c) {
		asset, exists, err = model.GetAssetByAssetID(assetID, false)
	} else {
		asset, exists, err = model.GetUserAssetByAssetID(userID, assetID)
	}
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query asset %s: %s", assetID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query asset")
		return
	}
	if !exists || asset == nil || asset.AssetType != model.AssetTypeVideo {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Video asset not found")
		return
	}
	task, taskExists, err := getVideoProxyTask(c, userID, asset.TaskID)
	if err != nil || !taskExists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Video task not found")
		return
	}
	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "Video task is not completed")
		return
	}
	streamVideoContent(c, task, asset.URL, asset)
}

func streamVideoContent(c *gin.Context, task *model.Task, sourceOverride string, assetOverride ...*model.Asset) {
	taskID := task.TaskID
	var asset *model.Asset
	if len(assetOverride) > 0 {
		asset = assetOverride[0]
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	var videoURL string
	requestHeaders := make(http.Header)
	proxy := channel.GetSetting().Proxy
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create proxy client for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), videoProxyTimeout)
	defer cancel()

	useOverride := strings.TrimSpace(sourceOverride) != "" && !isTaskContentProxyURL(sourceOverride, task.TaskID)
	if useOverride {
		videoURL = sourceOverride
	} else {
		switch channel.Type {
		case constant.ChannelTypeGemini:
			apiKey := task.PrivateData.Key
			if apiKey == "" {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for Gemini task %s", taskID))
				videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
				return
			}
			videoURL, err = getGeminiVideoURL(channel, task, apiKey)
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Gemini video URL for task %s: %s", taskID, err.Error()))
				videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Gemini video URL")
				return
			}
			requestHeaders.Set("x-goog-api-key", apiKey)
		case constant.ChannelTypeVertexAi:
			videoURL, err = getVertexVideoURL(channel, task)
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Vertex video URL for task %s: %s", taskID, err.Error()))
				videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Vertex video URL")
				return
			}
		case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
			videoURL = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID())
		default:
			// Video URL is stored in PrivateData.ResultURL (fallback to FailReason for old data)
			videoURL = task.GetResultURL()
		}
	}

	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL is empty for task %s", taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	if asset != nil && videoAssetNeedsResolver(asset) {
		resp, resolveErr := resolveVideoContentWithAdaptor(ctx, channel, task, asset, c.Request.Header)
		if resolveErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve provider video content for task %s: %s", taskID, resolveErr.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve provider video content")
			return
		}
		writeVideoProxyResponse(c, taskID, resp)
		return
	}

	if strings.HasPrefix(videoURL, "data:") {
		if err := writeVideoDataURL(c, videoURL); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	resolvedURL, sameOrigin, err := resolveVideoProxyURL(videoURL, baseURL)
	if err != nil {
		if asset != nil && getVideoContentResolver(task.Platform) != nil {
			resp, resolveErr := resolveVideoContentWithAdaptor(ctx, channel, task, asset, c.Request.Header)
			if resolveErr == nil {
				writeVideoProxyResponse(c, taskID, resp)
				return
			}
			err = fmt.Errorf("generic URL resolution failed: %w; provider resolver failed: %v", err, resolveErr)
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse URL %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve video URL")
		return
	}
	if sameOrigin && channel.Type != constant.ChannelTypeGemini && channel.Type != constant.ChannelTypeVertexAi {
		requestHeaders.Set("Authorization", "Bearer "+channel.Key)
		client = videoProxyClientForOrigin(client, resolvedURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL.String(), nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create request: %s", err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	req.Header = requestHeaders
	for _, header := range []string{"Range", "If-Range"} {
		if value := c.GetHeader(header); value != "" {
			req.Header.Set(header, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", resolvedURL.Redacted(), err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	writeVideoProxyResponse(c, taskID, resp)
}

func videoAssetNeedsResolver(asset *model.Asset) bool {
	if asset == nil || len(asset.Metadata) == 0 {
		return false
	}
	resolver, _ := asset.Metadata["resolver"].(string)
	return strings.TrimSpace(resolver) != ""
}

func resolveVideoContentWithAdaptor(ctx context.Context, providerChannel *model.Channel, task *model.Task, asset *model.Asset, incomingHeaders http.Header) (*http.Response, error) {
	resolver := getVideoContentResolver(task.Platform)
	if resolver == nil {
		return nil, fmt.Errorf("video content resolver is not implemented for platform %s", task.Platform)
	}
	output := relaycommon.VideoOutput{Index: asset.AssetIndex, URL: asset.URL}
	if asset.Metadata != nil {
		output.ProviderReference, _ = asset.Metadata["provider_reference"].(string)
		output.Resolver, _ = asset.Metadata["resolver"].(string)
	}
	headers := make(http.Header)
	for _, name := range []string{"Range", "If-Range"} {
		if value := incomingHeaders.Get(name); value != "" {
			headers.Set(name, value)
		}
	}
	resp, err := resolver.ResolveVideoContent(ctx, providerChannel, task, output, headers)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("video content resolver returned an empty response")
	}
	return resp, nil
}

func writeVideoProxyResponse(c *gin.Context, taskID string, resp *http.Response) {
	if resp == nil || resp.Body == nil {
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for video task %s", resp.StatusCode, taskID))
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			videoProxyError(c, http.StatusGone, "resource_expired", "Video resource has expired upstream")
			return
		}
		videoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	c.Writer.Header().Set("Cache-Control", "private, max-age=600")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func isTaskContentProxyURL(rawURL, taskID string) bool {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return target.Path == fmt.Sprintf("/v1/videos/%s/content", taskID)
}

func resolveVideoProxyURL(rawURL, baseURL string) (*url.URL, bool, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, false, fmt.Errorf("video URL is empty")
	}
	if strings.HasPrefix(rawURL, "//") {
		return nil, false, fmt.Errorf("scheme-relative video URL is not allowed")
	}

	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, false, fmt.Errorf("parse channel base URL: %w", err)
	}
	if err := validateVideoProxyHTTPURL(base); err != nil {
		return nil, false, fmt.Errorf("invalid channel base URL: %w", err)
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, false, fmt.Errorf("parse video URL: %w", err)
	}
	if target.User != nil {
		return nil, false, fmt.Errorf("video URL credentials are not allowed")
	}
	if !target.IsAbs() {
		if target.Host != "" {
			return nil, false, fmt.Errorf("scheme-relative video URL is not allowed")
		}
		base.RawQuery = ""
		base.Fragment = ""
		if !strings.HasSuffix(base.Path, "/") {
			base.Path += "/"
		}
		target = base.ResolveReference(target)
	}
	if err := validateVideoProxyHTTPURL(target); err != nil {
		return nil, false, fmt.Errorf("invalid video URL: %w", err)
	}

	return target, sameVideoProxyOrigin(base, target), nil
}

func validateVideoProxyHTTPURL(target *url.URL) error {
	if target == nil {
		return fmt.Errorf("URL is nil")
	}
	if target.User != nil {
		return fmt.Errorf("URL credentials are not allowed")
	}
	switch strings.ToLower(target.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported URL scheme %q", target.Scheme)
	}
	if strings.TrimSpace(target.Hostname()) == "" {
		return fmt.Errorf("URL host is required")
	}
	return nil
}

func sameVideoProxyOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		videoProxyPort(left) == videoProxyPort(right)
}

func videoProxyPort(target *url.URL) string {
	if port := target.Port(); port != "" {
		return port
	}
	if strings.EqualFold(target.Scheme, "http") {
		return "80"
	}
	if strings.EqualFold(target.Scheme, "https") {
		return "443"
	}
	return ""
}

func videoProxyClientForOrigin(client *http.Client, authorizedOrigin *url.URL) *http.Client {
	cloned := *client
	originalCheckRedirect := client.CheckRedirect
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if originalCheckRedirect != nil {
			if err := originalCheckRedirect(req, via); err != nil {
				return err
			}
		}
		if !sameVideoProxyOrigin(authorizedOrigin, req.URL) {
			req.Header.Del("Authorization")
		}
		return nil
	}
	return &cloned
}

func getVideoProxyTask(c *gin.Context, userID int, taskID string) (*model.Task, bool, error) {
	if isVideoProxyAdmin(c) {
		return model.GetByOnlyTaskId(taskID)
	}
	return model.GetByTaskId(userID, taskID)
}

func isVideoProxyAdmin(c *gin.Context) bool {
	if role, ok := c.Get("role"); ok {
		if roleInt, ok := role.(int); ok && roleInt >= common.RoleAdminUser {
			return true
		}
	}
	if _, ok := c.Get(sessions.DefaultKey); !ok {
		return false
	}
	role, ok := sessions.Default(c).Get("role").(int)
	return ok && role >= common.RoleAdminUser
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return err
		}
	}

	start, end, partial, rangeErr := parseVideoByteRange(c.GetHeader("Range"), int64(len(videoBytes)))
	if rangeErr != nil {
		c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(videoBytes)))
		c.Writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	body := videoBytes
	status := http.StatusOK
	if partial {
		body = videoBytes[start : end+1]
		status = http.StatusPartialContent
		c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(videoBytes)))
	}
	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Accept-Ranges", "bytes")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	c.Writer.Header().Set("Cache-Control", "private, max-age=600")
	c.Writer.WriteHeader(status)
	_, err = c.Writer.Write(body)
	return err
}

func parseVideoByteRange(value string, size int64) (int64, int64, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, size - 1, false, nil
	}
	if size <= 0 || !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return 0, 0, false, fmt.Errorf("invalid byte range")
	}
	startText, endText, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(value, "bytes=")), "-")
	if !ok || (startText == "" && endText == "") {
		return 0, 0, false, fmt.Errorf("invalid byte range")
	}
	if startText == "" {
		suffix, err := strconv.ParseInt(endText, 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false, fmt.Errorf("invalid byte range")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, fmt.Errorf("invalid byte range")
	}
	end := size - 1
	if endText != "" {
		end, err = strconv.ParseInt(endText, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, fmt.Errorf("invalid byte range")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true, nil
}
