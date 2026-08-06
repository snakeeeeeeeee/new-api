package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetCanvasAdminConfig(c *gin.Context) {
	config, err := service.GetCanvasAdminConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func UpdateCanvasAdminConfig(c *gin.Context) {
	var config service.CanvasConfig
	if err := common.DecodeJson(c.Request.Body, &config); err != nil {
		common.ApiErrorMsg(c, "Canvas 配置参数无效")
		return
	}
	result, err := service.SaveCanvasConfig(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetCanvasAuthorizationContext(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	request := canvasAuthorizationRequestFromQuery(c)
	context, err := service.GetCanvasAuthorizationContext(c.GetInt("id"), request)
	if err != nil {
		canvasAuthorizationError(c, http.StatusOK, err)
		return
	}
	common.ApiSuccess(c, context)
}

func CreateCanvasAuthorizationCode(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request service.CanvasAuthorizationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		canvasAuthorizationError(c, http.StatusOK, &service.CanvasAuthorizationError{Code: "invalid_request", Message: "授权请求参数无效"})
		return
	}
	result, err := service.IssueCanvasAuthorizationCode(c.GetInt("id"), request)
	if err != nil {
		canvasAuthorizationError(c, http.StatusOK, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ExchangeCanvasAuthorizationCode(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request service.CanvasTokenExchangeRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		canvasOAuthError(c, &service.CanvasAuthorizationError{Code: "invalid_request", Message: "授权兑换参数无效"})
		return
	}
	result, err := service.ExchangeCanvasAuthorizationCode(request)
	if err != nil {
		canvasOAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func canvasAuthorizationRequestFromQuery(c *gin.Context) service.CanvasAuthorizationRequest {
	return service.CanvasAuthorizationRequest{
		ClientId: c.Query("client_id"), RedirectUri: c.Query("redirect_uri"), State: c.Query("state"),
		CodeChallenge: c.Query("code_challenge"), CodeChallengeMethod: c.Query("code_challenge_method"),
	}
}

func canvasAuthorizationError(c *gin.Context, status int, err error) {
	var typed *service.CanvasAuthorizationError
	if errors.As(err, &typed) {
		c.JSON(status, gin.H{"success": false, "code": typed.Code, "message": typed.Message, "missing_groups": typed.MissingGroups})
		return
	}
	c.JSON(status, gin.H{"success": false, "code": "server_error", "message": err.Error()})
}

func canvasOAuthError(c *gin.Context, err error) {
	var typed *service.CanvasAuthorizationError
	if errors.As(err, &typed) {
		c.JSON(http.StatusBadRequest, gin.H{"error": typed.Code, "error_description": typed.Message, "missing_groups": typed.MissingGroups})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "error_description": "Canvas 授权兑换失败"})
}
