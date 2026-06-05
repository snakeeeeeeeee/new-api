package middleware

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	errorCode := types.ErrorCodeBadRequestBody
	if len(code) > 0 {
		errorCode = code[0]
		codeStr = string(errorCode)
	}
	service.DumpRelayErrorIfNeeded(c, types.NewErrorWithStatusCode(fmt.Errorf("%s", message), errorCode, statusCode, types.ErrOptionWithSkipRetry()))
	userId := c.GetInt("id")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}

func abortWithClaudeCompatError(c *gin.Context, err *types.NewAPIError) {
	if err == nil {
		return
	}
	service.DumpRelayErrorIfNeeded(c, err)
	userId := c.GetInt("id")
	requestId := c.GetString(common.RequestIdKey)
	err.SetMessage(common.MessageWithRequestId(err.Error(), requestId))
	c.JSON(err.StatusCode, gin.H{
		"type":  "error",
		"error": err.ToClaudeError(),
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, err.Error()))
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
