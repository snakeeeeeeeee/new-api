package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPublicImageTaskErrorMasksInternalUpstreamQuotaDetails(t *testing.T) {
	publicError := buildPublicImageTaskError(&imageTaskCallbackError{
		Code:                 "new_api_error",
		Message:              "本地用户预扣费额度失败, 本地用户剩余额度: $0.020000, 需要预扣费额度: $0.060000 (request id: req-secret)",
		ProviderErrorCode:    "insufficient_user_quota",
		ProviderErrorType:    "new_api_error",
		ProviderErrorMessage: "本地用户预扣费额度失败",
		UpstreamStatus:       403,
	}, "sensitive fallback")

	assert.Equal(t, "524", publicError.Code)
	assert.Equal(t, publicImageTaskUpstreamQuotaMessage, publicError.Message)
	assert.True(t, publicError.Retryable)
	assert.NotContains(t, publicError.Message, "$0.020000")
	assert.NotContains(t, publicError.Message, "req-secret")
}

func TestBuildPublicImageTaskErrorMasksLegacyQuotaMessageWithoutProviderCode(t *testing.T) {
	publicError := buildPublicImageTaskError(&imageTaskCallbackError{
		Code:    "new_api_error",
		Message: "本地用户预扣费额度失败, 本地用户剩余额度不足",
	}, "")

	assert.Equal(t, "524", publicError.Code)
	assert.Equal(t, publicImageTaskUpstreamQuotaMessage, publicError.Message)
	assert.True(t, publicError.Retryable)
}

func TestBuildPublicImageTaskErrorPreservesNonQuotaBehavior(t *testing.T) {
	publicError := buildPublicImageTaskError(&imageTaskCallbackError{
		Code:      "unsupported_size",
		Message:   "size must be 1024x1024",
		Retryable: false,
	}, "")

	assert.Equal(t, "unsupported_size", publicError.Code)
	assert.Equal(t, "size must be 1024x1024", publicError.Message)
	assert.False(t, publicError.Retryable)
}
