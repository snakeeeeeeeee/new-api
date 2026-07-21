package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetAccountWebhook(c *gin.Context) {
	config, err := service.GetAccountWebhookConfig(c.GetInt("id"))
	if err != nil {
		writeAccountWebhookError(c, err)
		return
	}
	c.JSON(http.StatusOK, config)
}

func PutAccountWebhook(c *gin.Context) {
	var request dto.AccountWebhookUpdateRequest
	if err := common.DecodeJsonStrict(c.Request.Body, &request); err != nil {
		writeWebhookAPIError(c, http.StatusBadRequest, "invalid_request", "Invalid JSON request body", "")
		return
	}
	config, err := service.PutAccountWebhookConfig(c.GetInt("id"), request)
	if err != nil {
		writeAccountWebhookError(c, err)
		return
	}
	c.JSON(http.StatusOK, config)
}

func DeleteAccountWebhook(c *gin.Context) {
	if err := service.DisableAccountWebhookConfig(c.GetInt("id")); err != nil {
		writeAccountWebhookError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func TestAccountWebhook(c *gin.Context) {
	result, err := service.CreateAccountWebhookTestDelivery(c.GetInt("id"))
	if err != nil {
		writeAccountWebhookError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func writeAccountWebhookError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrWebhookConfigNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		writeWebhookAPIError(c, http.StatusNotFound, "webhook_not_configured", "Webhook is not configured", "")
		return
	}
	if errors.Is(err, service.ErrWebhookStoredKeyUnavailable) {
		writeWebhookAPIError(c, http.StatusBadRequest, "webhook_key_regeneration_required", err.Error(), "regenerate_key")
		return
	}
	message := err.Error()
	param := ""
	if strings.Contains(message, "URL") || strings.Contains(message, "HTTPS") || strings.Contains(message, "DNS") || strings.Contains(message, "IP") {
		param = "url"
	} else if strings.Contains(strings.ToLower(message), "key") {
		param = "regenerate_key"
	}
	writeWebhookAPIError(c, http.StatusBadRequest, "invalid_request", message, param)
}

func writeWebhookAPIError(c *gin.Context, status int, code, message, param string) {
	c.JSON(status, dto.ImageTaskAPIErrorResponse{Error: dto.ImageTaskAPIError{
		Type: "invalid_request_error", Code: code, Message: message, Param: param,
		RequestID: c.GetString(common.RequestIdKey),
	}})
}
