package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildClientFacingOpenAIError(t *testing.T) {
	withRelayErrorSetting(t, false, "400,422", "", true)
	apiErr := types.NewOpenAIError(assertErr("upstream claude provider returned 429"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)

	got := buildClientFacingOpenAIError(apiErr)

	require.Equal(t, clientFacingRelayErrorMessage, got.Message)
	require.Equal(t, clientFacingRelayErrorType, got.Type)
	require.Equal(t, clientFacingRelayErrorCode, got.Code)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "upstream claude provider returned 429")
	require.True(t, shouldWrapClientFacingRelayError(apiErr))
}

func TestBuildClientFacingClaudeError(t *testing.T) {
	withRelayErrorSetting(t, false, "400,422", "", true)
	apiErr := types.WithClaudeError(types.ClaudeError{
		Message: "upstream vendor example.com failed",
		Type:    "upstream_error",
	}, http.StatusServiceUnavailable)

	got := buildClientFacingClaudeError(apiErr)

	require.Equal(t, clientFacingRelayErrorMessage, got.Message)
	require.Equal(t, clientFacingRelayErrorType, got.Type)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "example.com")
	require.True(t, shouldWrapClientFacingRelayError(apiErr))
}

func TestShouldWrapClientFacingRelayError_FalseForLocalErrors(t *testing.T) {
	apiErr := types.NewErrorWithStatusCode(assertErr("model ratio not set"), types.ErrorCodeModelPriceError, http.StatusInternalServerError)

	require.False(t, shouldWrapClientFacingRelayError(apiErr))
}

func TestShouldWrapClientFacingRelayError_DefaultDisabledWraps400(t *testing.T) {
	setting := operation_setting.GetRelayErrorSetting()
	original := *setting
	*setting = operation_setting.RelayErrorSetting{
		PassthroughEnabled:     false,
		PassthroughStatusCodes: "400,422",
		MaskSensitive:          true,
	}
	t.Cleanup(func() {
		*setting = original
	})

	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "prompt is too long: 202805 tokens > 200000 maximum",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)

	require.True(t, shouldWrapClientFacingRelayError(apiErr))
	require.Equal(t, clientFacingRelayErrorMessage, buildClientFacingOpenAIError(apiErr).Message)
	require.False(t, operation_setting.ShouldPassthroughRelayErrorStatusCode(http.StatusBadRequest))
}

func TestShouldWrapClientFacingRelayError_PassthroughEnabled400And422(t *testing.T) {
	withRelayErrorSetting(t, true, "400,422", "", true)

	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "prompt is too long: 202805 tokens > 200000 maximum",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)

	require.False(t, shouldWrapClientFacingRelayError(apiErr))
	got := buildClientFacingRelayOpenAIError(apiErr)
	require.Equal(t, "prompt is too long: 202805 tokens > 200000 maximum", got.Message)
	require.Equal(t, "invalid_request_error", got.Type)
	require.Equal(t, "invalid_request", got.Code)

	claudeErr := types.WithClaudeError(types.ClaudeError{
		Message: "messages.46: tool_use ids were found without tool_result blocks immediately after",
		Type:    "invalid_request_error",
	}, http.StatusBadRequest)

	require.False(t, shouldWrapClientFacingRelayError(claudeErr))
	claudeGot := buildClientFacingRelayClaudeError(claudeErr)
	require.Contains(t, claudeGot.Message, "tool_use ids were found without tool_result")
	require.Equal(t, "invalid_request_error", claudeGot.Type)

	parsedClaudeUpstreamErr := types.WithOpenAIError(types.OpenAIError{
		Message: "unexpected `tool_use_id` found in `tool_result` blocks",
		Type:    "invalid_request_error",
		Code:    nil,
	}, http.StatusBadRequest)
	parsedClaudeGot := buildClientFacingRelayClaudeError(parsedClaudeUpstreamErr)
	require.Contains(t, parsedClaudeGot.Message, "tool_use_id")
	require.Equal(t, "invalid_request_error", parsedClaudeGot.Type)

	unprocessableErr := types.WithOpenAIError(types.OpenAIError{
		Message: "invalid JSON schema for tool",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusUnprocessableEntity)
	require.False(t, shouldWrapClientFacingRelayError(unprocessableErr))
}

func TestShouldWrapClientFacingRelayError_WrapsWhenDisabledOrStatusNotAllowed(t *testing.T) {
	withRelayErrorSetting(t, true, "400,422", "", true)

	rateLimitErr := types.NewOpenAIError(assertErr("upstream capacity exceeded"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.True(t, shouldWrapClientFacingRelayError(rateLimitErr))
	require.Equal(t, clientFacingRelayErrorMessage, buildClientFacingOpenAIError(rateLimitErr).Message)

	serverErr := types.NewOpenAIError(assertErr("upstream internal error"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	require.True(t, shouldWrapClientFacingRelayError(serverErr))

	withRelayErrorSetting(t, false, "400,422", "", true)
	badRequestErr := types.NewOpenAIError(assertErr("prompt is too long"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.True(t, shouldWrapClientFacingRelayError(badRequestErr))
}

func TestShouldWrapClientFacingRelayError_BlocksConfiguredKeywords(t *testing.T) {
	withRelayErrorSetting(t, true, "400,422", "settings/usage\nthird-party apps now draw from your extra usage", true)

	blockedErr := types.WithOpenAIError(types.OpenAIError{
		Message: "Third-party apps now draw from your extra usage, not your plan limits. Add more at https://console.anthropic.com/settings/usage and keep going.",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)
	require.True(t, shouldWrapClientFacingRelayError(blockedErr))
	require.Equal(t, clientFacingRelayErrorMessage, buildClientFacingOpenAIError(blockedErr).Message)

	unblockedErr := types.WithOpenAIError(types.OpenAIError{
		Message: "messages.46: tool_use ids were found without tool_result blocks immediately after",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)
	require.False(t, shouldWrapClientFacingRelayError(unblockedErr))
	require.Contains(t, buildClientFacingRelayOpenAIError(unblockedErr).Message, "tool_use ids")

	claudeBlockedErr := types.WithClaudeError(types.ClaudeError{
		Message: "Add more at https://console.anthropic.com/SETTINGS/USAGE and keep going.",
		Type:    "invalid_request_error",
	}, http.StatusBadRequest)
	require.True(t, shouldWrapClientFacingRelayError(claudeBlockedErr))
	require.Equal(t, clientFacingRelayErrorMessage, buildClientFacingClaudeError(claudeBlockedErr).Message)
}

func TestBuildClientFacingRelayErrorHonorsMaskSensitiveSetting(t *testing.T) {
	withRelayErrorSetting(t, true, "400", "", true)
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream https://api.vendor.example/v1 failed",
		Type:    "invalid_request_error",
		Code:    "invalid_request",
	}, http.StatusBadRequest)
	require.Equal(t, "upstream https://***.example/*** failed", buildClientFacingRelayOpenAIError(apiErr).Message)

	withRelayErrorSetting(t, true, "400", "", false)
	require.Equal(t, "upstream https://api.vendor.example/v1 failed", buildClientFacingRelayOpenAIError(apiErr).Message)
}

func TestProcessChannelErrorRecordsStreamState(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		constant.ErrorLogEnabled = originalErrorLogEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 1001)
	ctx.Set("username", "stream-user")
	ctx.Set("token_name", "stream-token")
	ctx.Set("original_model", "claude-opus-4-6")
	ctx.Set("token_id", 2002)
	ctx.Set("group", "default")
	ctx.Set("channel_id", 3003)
	ctx.Set("channel_name", "stream-channel")
	ctx.Set("channel_type", 1)
	ctx.Set(common.RequestIdKey, "req-stream-error")
	common.SetContextKey(ctx, constant.ContextKeyRelayIsStream, true)

	apiErr := types.NewOpenAIError(assertErr("upstream capacity exceeded"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	processChannelError(ctx, *types.NewChannelError(3003, 1, "stream-channel", false, "", false), apiErr)

	var logItem model.Log
	require.NoError(t, db.Where("request_id = ?", "req-stream-error").First(&logItem).Error)
	require.Equal(t, model.LogTypeError, logItem.Type)
	require.True(t, logItem.IsStream)
	other, err := common.StrToMap(logItem.Other)
	require.NoError(t, err)
	require.True(t, other["user_safe"].(bool))
	require.NotContains(t, other, "internal_retry")
}

func TestProcessChannelErrorMarksInternalRetryLog(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		constant.ErrorLogEnabled = originalErrorLogEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 1002)
	ctx.Set("username", "retry-user")
	ctx.Set("token_name", "retry-token")
	ctx.Set("original_model", "claude-opus-4-6")
	ctx.Set("token_id", 2003)
	ctx.Set("group", "default")
	ctx.Set("channel_id", 3004)
	ctx.Set("channel_name", "retry-channel")
	ctx.Set("channel_type", 1)
	ctx.Set(common.RequestIdKey, "req-internal-retry")

	apiErr := types.NewOpenAIError(assertErr("upstream capacity exceeded"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	processChannelError(ctx, *types.NewChannelError(3004, 1, "retry-channel", false, "", false), apiErr, true)

	var logItem model.Log
	require.NoError(t, db.Where("request_id = ?", "req-internal-retry").First(&logItem).Error)
	require.Contains(t, logItem.Content, "upstream capacity exceeded")
	other, err := common.StrToMap(logItem.Other)
	require.NoError(t, err)
	require.True(t, other["internal_retry"].(bool))
	require.NotContains(t, other, "user_safe")
}

func assertErr(msg string) error {
	return &staticErr{msg: msg}
}

func withRelayErrorSetting(t *testing.T, passthroughEnabled bool, passthroughStatusCodes string, passthroughBlockKeywords string, maskSensitive bool) {
	t.Helper()
	setting := operation_setting.GetRelayErrorSetting()
	original := *setting
	*setting = operation_setting.RelayErrorSetting{
		PassthroughEnabled:       passthroughEnabled,
		PassthroughStatusCodes:   passthroughStatusCodes,
		PassthroughBlockKeywords: passthroughBlockKeywords,
		MaskSensitive:            maskSensitive,
	}
	t.Cleanup(func() {
		*setting = original
	})
}

type staticErr struct {
	msg string
}

func (e *staticErr) Error() string {
	return e.msg
}
