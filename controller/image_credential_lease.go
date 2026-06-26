package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type imageCredentialLeaseResolveRequest struct {
	ProviderTaskID string `json:"provider_task_id"`
	ClientTaskID   string `json:"client_task_id"`
	Attempt        int    `json:"attempt"`
	Operation      string `json:"operation"`
	Model          string `json:"model"`
}

type imageCredentialLeaseResolveResponse struct {
	Provider      string `json:"provider"`
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	ChannelID     string `json:"channel_id"`
	RequestFormat string `json:"request_format"`
	ExpiresAt     string `json:"expires_at"`
}

type imageCredentialLeaseErrorBody struct {
	Error imageCredentialLeaseError `json:"error"`
}

type imageCredentialLeaseError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func ResolveImageCredentialLease(c *gin.Context) {
	rawBody, ok := verifyImageHandleInternalRequest(c)
	if !ok {
		return
	}
	req := imageCredentialLeaseResolveRequest{}
	if len(rawBody) > 0 {
		if err := common.Unmarshal(rawBody, &req); err != nil {
			writeImageCredentialLeaseError(c, http.StatusBadRequest, "invalid_request", "invalid resolve body", false)
			return
		}
	}
	leaseID := strings.TrimSpace(c.Param("lease_id"))
	if leaseID == "" {
		writeImageCredentialLeaseError(c, http.StatusBadRequest, "invalid_request", "lease_id is required", false)
		return
	}
	lease, exists, err := model.GetImageCredentialLeaseByLeaseID(leaseID)
	if err != nil || !exists || lease == nil {
		writeImageCredentialLeaseError(c, http.StatusNotFound, "lease_not_found", "credential lease not found", false)
		return
	}
	if lease.ExpiresAt > 0 && time.Now().Unix() > lease.ExpiresAt {
		_ = model.MarkImageCredentialLeaseExpired(lease.LeaseID)
		writeImageCredentialLeaseError(c, http.StatusGone, "lease_expired", "credential lease expired", false)
		return
	}
	if lease.Status == model.ImageCredentialLeaseStatusFailed || lease.Status == model.ImageCredentialLeaseStatusExpired {
		writeImageCredentialLeaseError(c, http.StatusGone, "lease_expired", "credential lease is not active", false)
		return
	}
	var task *model.Task
	if lease.TaskRecordID > 0 {
		var exists bool
		task, exists, err = model.GetByOnlyTaskId(lease.TaskID)
		if err != nil {
			writeImageCredentialLeaseError(c, http.StatusInternalServerError, "task_query_failed", err.Error(), true)
			return
		}
		if !exists || task == nil || task.Platform != imageHandleTaskPlatform() {
			writeImageCredentialLeaseError(c, http.StatusNotFound, "task_not_found", "task not found", false)
			return
		}
		if req.ClientTaskID != "" && req.ClientTaskID != task.TaskID {
			writeImageCredentialLeaseError(c, http.StatusConflict, "task_mismatch", "client_task_id does not match lease", false)
			return
		}
		if req.ProviderTaskID != "" && task.PrivateData.UpstreamTaskID != "" && req.ProviderTaskID != task.PrivateData.UpstreamTaskID {
			writeImageCredentialLeaseError(c, http.StatusConflict, "task_mismatch", "provider_task_id does not match task", false)
			return
		}
		if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
			writeImageCredentialLeaseError(c, http.StatusConflict, "task_already_finished", "task already finished", false)
			return
		}
		if task.ID != lease.TaskRecordID {
			writeImageCredentialLeaseError(c, http.StatusConflict, "task_mismatch", "task record does not match lease", false)
			return
		}
		if task.ChannelId != lease.ChannelID {
			writeImageCredentialLeaseError(c, http.StatusConflict, "channel_mismatch", "task channel does not match lease", false)
			return
		}
	} else if req.ClientTaskID != "" && lease.TaskID != "" && req.ClientTaskID != lease.TaskID {
		writeImageCredentialLeaseError(c, http.StatusConflict, "task_mismatch", "client_task_id does not match lease", false)
		return
	}
	if req.Operation != "" && lease.Operation != "" && req.Operation != lease.Operation {
		writeImageCredentialLeaseError(c, http.StatusConflict, "model_not_supported", "request operation does not match lease", false)
		return
	}
	channel, err := model.GetChannelById(lease.ChannelID, true)
	if err != nil {
		writeImageCredentialLeaseError(c, http.StatusNotFound, "channel_not_found", err.Error(), true)
		return
	}
	if channel.Status != common.ChannelStatusEnabled {
		writeImageCredentialLeaseError(c, http.StatusForbidden, "channel_disabled", "channel is disabled", false)
		return
	}
	if !channelSupportsOpenAIImages(channel) {
		writeImageCredentialLeaseError(c, http.StatusBadRequest, "model_not_supported", "channel does not support openai images resolve format", false)
		return
	}
	key, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil || strings.TrimSpace(key) == "" {
		message := "channel credential unavailable"
		if apiErr != nil {
			message = apiErr.Error()
		}
		writeImageCredentialLeaseError(c, http.StatusServiceUnavailable, "credential_unavailable", message, true)
		return
	}
	modelName := lease.Model
	if modelName == "" && task != nil {
		modelName = task.Properties.UpstreamModelName
	}
	if modelName == "" && task != nil {
		modelName = task.Properties.OriginModelName
	}
	if req.Model != "" && modelName != "" && req.Model != modelName {
		writeImageCredentialLeaseError(c, http.StatusConflict, "model_not_supported", "request model does not match lease", false)
		return
	}
	_ = model.MarkImageCredentialLeaseResolved(lease)
	baseURL := resolveChannelBaseURL(channel)
	if service.GetImageHandleExecutorConfig().DebugUpstream {
		taskID := lease.TaskID
		imageRequest := ""
		if task != nil && task.PrivateData.ImageRequest != nil {
			taskID = task.TaskID
			imageRequest = string(task.PrivateData.ImageRequest)
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"image-handle resolve lease debug: task_id=%s provider_task_id=%s lease_id=%s channel_id=%d channel_type=%d model=%s operation=%s base_url=%s request=%s",
			taskID,
			req.ProviderTaskID,
			lease.LeaseID,
			channel.Id,
			channel.Type,
			modelName,
			lease.Operation,
			baseURL,
			imageRequest,
		))
	}
	c.JSON(http.StatusOK, imageCredentialLeaseResolveResponse{
		Provider:      "openai_compatible",
		BaseURL:       baseURL,
		APIKey:        key,
		Model:         modelName,
		ChannelID:     fmt.Sprintf("channel_%d", channel.Id),
		RequestFormat: "openai_images",
		ExpiresAt:     time.Unix(lease.ExpiresAt, 0).UTC().Format(time.RFC3339),
	})
}

func verifyImageHandleInternalRequest(c *gin.Context) ([]byte, bool) {
	rawBody, err := readRequestBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read internal body failed"})
		return nil, false
	}
	timestamp := strings.TrimSpace(c.GetHeader("X-ImageHandle-Timestamp"))
	signature := strings.TrimSpace(c.GetHeader("X-ImageHandle-Signature"))
	secretID := strings.TrimSpace(c.GetHeader("X-ImageHandle-Secret-Id"))
	if timestamp == "" || signature == "" || secretID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing internal signature headers"})
		return nil, false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid internal timestamp"})
		return nil, false
	}
	now := time.Now().Unix()
	if now-ts > callbackTimestampWindowSeconds || ts-now > callbackTimestampWindowSeconds {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "internal timestamp expired"})
		return nil, false
	}
	secret, ok := service.ResolveImageHandleInternalSecret(secretID)
	if !ok || secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "internal secret not found"})
		return nil, false
	}
	expected := signCallbackPayload(timestamp, rawBody, secret)
	if !constantTimeEqualHex(signature, expected) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid internal signature"})
		return nil, false
	}
	return rawBody, true
}

func readRequestBody(c *gin.Context) ([]byte, error) {
	return c.GetRawData()
}

func writeImageCredentialLeaseError(c *gin.Context, statusCode int, code string, message string, retryable bool) {
	c.JSON(statusCode, imageCredentialLeaseErrorBody{
		Error: imageCredentialLeaseError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	})
}

func channelSupportsOpenAIImages(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	return channel.Type == constant.ChannelTypeOpenAI || channel.Type == constant.ChannelTypeXai
}

func resolveChannelBaseURL(channel *model.Channel) string {
	if channel == nil {
		return ""
	}
	if baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/"); baseURL != "" {
		return normalizeOpenAIImagesBaseURL(baseURL, channel.Type)
	}
	return normalizeOpenAIImagesBaseURL(constant.ChannelBaseURLs[channel.Type], channel.Type)
}

func normalizeOpenAIImagesBaseURL(baseURL string, channelType int) string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for {
		lower := strings.ToLower(normalized)
		switch {
		case strings.HasSuffix(lower, "/images/generations"):
			normalized = strings.TrimRight(normalized[:len(normalized)-len("/images/generations")], "/")
		case strings.HasSuffix(lower, "/images/edits"):
			normalized = strings.TrimRight(normalized[:len(normalized)-len("/images/edits")], "/")
		default:
			if channelType == constant.ChannelTypeOpenAI && !strings.HasSuffix(strings.ToLower(normalized), "/v1") {
				return normalized + "/v1"
			}
			return normalized
		}
	}
}
