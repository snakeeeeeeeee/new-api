package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildClientFacingOpenAIError(t *testing.T) {
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
}

func assertErr(msg string) error {
	return &staticErr{msg: msg}
}

type staticErr struct {
	msg string
}

func (e *staticErr) Error() string {
	return e.msg
}
