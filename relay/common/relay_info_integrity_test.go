package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	basecommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenRelayInfoSnapshotsClaudeResponseIntegritySettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := model_setting.GetClaudeSettings()
	original := *settings
	t.Cleanup(func() {
		*model_setting.GetClaudeSettings() = original
		model_setting.RefreshClaudeResponseIntegritySettingsSnapshot()
	})

	settings.ResponseIntegrityFallbackEnabled = false
	settings.ResponseIntegrityFirstBlockTimeoutSec = 30
	model_setting.RefreshClaudeResponseIntegritySettingsSnapshot()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	request := &dto.ClaudeRequest{Stream: basecommon.GetPointer(true)}
	first := GenRelayInfoClaude(ctx, request)

	settings.ResponseIntegrityFallbackEnabled = true
	settings.ResponseIntegrityFirstBlockTimeoutSec = 45
	model_setting.RefreshClaudeResponseIntegritySettingsSnapshot()
	second := GenRelayInfoClaude(ctx, request)
	second.BeginClaudeResponseIntegrityAttempt(ctx.Request.Context())
	t.Cleanup(second.EndClaudeResponseIntegrityAttempt)

	settings.ResponseIntegrityFallbackEnabled = false
	settings.ResponseIntegrityFirstBlockTimeoutSec = 60
	model_setting.RefreshClaudeResponseIntegritySettingsSnapshot()
	third := GenRelayInfoClaude(ctx, request)

	require.False(t, first.ClaudeResponseIntegrityEnabled)
	require.Equal(t, 30*time.Second, first.ClaudeResponseIntegrityFirstBlockTimeout)
	require.True(t, second.ClaudeResponseIntegrityEnabled)
	require.Equal(t, 45*time.Second, second.ClaudeResponseIntegrityFirstBlockTimeout)
	require.NotNil(t, second.ClaudeResponseIntegrityAttemptDone(), "in-flight request must keep its enabled snapshot")
	require.False(t, third.ClaudeResponseIntegrityEnabled)
	require.Equal(t, 60*time.Second, third.ClaudeResponseIntegrityFirstBlockTimeout)
}
