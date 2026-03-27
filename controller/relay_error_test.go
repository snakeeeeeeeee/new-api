package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
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

func assertErr(msg string) error {
	return &staticErr{msg: msg}
}

type staticErr struct {
	msg string
}

func (e *staticErr) Error() string {
	return e.msg
}
