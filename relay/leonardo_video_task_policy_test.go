package relay

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeLeonardoSubmitErrorMasksUpstreamDetails(t *testing.T) {
	upstream := &dto.TaskError{
		Code:       "fail_to_fetch_task",
		Message:    `{"detail":"Bearer secret and provider internals"}`,
		StatusCode: http.StatusBadGateway,
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeLeonardoVideo}}

	masked := sanitizeLeonardoSubmitError(info, upstream)

	require.NotNil(t, masked)
	assert.Equal(t, "video_task_failed", masked.Code)
	assert.Equal(t, "video task submission failed", masked.Message)
	assert.Equal(t, http.StatusBadGateway, masked.StatusCode)
	assert.True(t, masked.LocalError)
	assert.NotContains(t, masked.Message, "Bearer")
}

func TestSanitizeLeonardoSubmitErrorLeavesOtherChannelsUntouched(t *testing.T) {
	upstream := &dto.TaskError{Code: "fail_to_fetch_task", Message: "provider detail", StatusCode: http.StatusBadGateway}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeAdobeVideo}}

	assert.Same(t, upstream, sanitizeLeonardoSubmitError(info, upstream))
	assert.Nil(t, sanitizeLeonardoSubmitError(info, nil))
}

func TestProjectLeonardoSubmitHTTPErrorPreservesTrustedValidation(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeLeonardoVideo}}
	body := []byte(`{"detail":{"code":"invalid_reference_media_duration","message":"each video reference must be 3-10 seconds"}}`)

	projected := projectTaskSubmitHTTPError(info, http.StatusBadRequest, body)

	require.NotNil(t, projected)
	assert.Equal(t, "invalid_reference_media_duration", projected.Code)
	assert.Equal(t, "each video reference must be 3-10 seconds", projected.Message)
	assert.Equal(t, http.StatusBadRequest, projected.StatusCode)
}

func TestProjectLeonardoSubmitHTTPErrorPreservesNormalizationFailure(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeLeonardoVideo}}
	body := []byte(`{"detail":{"code":"reference_media_normalization_failed","message":"unable to normalize video reference duration"}}`)

	projected := projectTaskSubmitHTTPError(info, http.StatusBadRequest, body)

	require.NotNil(t, projected)
	assert.Equal(t, "reference_media_normalization_failed", projected.Code)
	assert.Equal(t, "unable to normalize video reference duration", projected.Message)
	assert.Equal(t, http.StatusBadRequest, projected.StatusCode)
}

func TestProjectAdobeSubmitHTTPErrorPreservesTrustedDurationValidation(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeAdobeVideo}}
	body := []byte(`{"detail":{"code":"reference_media_duration_exceeded","message":"reference video exceeds the 15 second limit","kind":"video","actual_duration_ms":15400}}`)

	projected := projectTaskSubmitHTTPError(info, http.StatusBadRequest, body)

	require.NotNil(t, projected)
	assert.Equal(t, "reference_media_duration_exceeded", projected.Code)
	assert.Equal(t, "reference video exceeds the 15 second limit", projected.Message)
	assert.Equal(t, http.StatusBadRequest, projected.StatusCode)
	assert.True(t, projected.LocalError)
}

func TestProjectLeonardoSubmitHTTPErrorMasksUntrustedResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeLeonardoVideo}}
	for _, test := range []struct {
		status int
		body   string
	}{
		{status: http.StatusUnauthorized, body: `{"detail":{"code":"auth_invalid","message":"Bearer secret"}}`},
		{status: http.StatusBadRequest, body: `{"detail":{"code":"provider_internal","message":"private"}}`},
		{status: http.StatusInternalServerError, body: `{"detail":{"code":"invalid_request","message":"stack trace"}}`},
		{status: http.StatusBadRequest, body: `not-json`},
	} {
		projected := projectTaskSubmitHTTPError(info, test.status, []byte(test.body))
		require.NotNil(t, projected)
		assert.Equal(t, "video_task_failed", projected.Code)
		assert.Equal(t, "video task submission failed", projected.Message)
		assert.NotContains(t, projected.Message, "secret")
	}
}
