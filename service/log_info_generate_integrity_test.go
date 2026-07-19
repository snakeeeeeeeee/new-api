package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesClaudeIntegrityObservability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(ctx, constant.ContextKeyUpstreamFirstEventMs, 42)
	common.SetContextKey(ctx, constant.ContextKeyClaudeStreamIncomplete, true)
	common.SetContextKey(ctx, constant.ContextKeyClaudeStreamIncompleteReason, "eof_before_message_stop")
	start := time.Now()
	relayInfo := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(100 * time.Millisecond),
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 1, 0, 1)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 42, adminInfo["upstream_first_event_ms"])
	require.Equal(t, true, adminInfo["claude_stream_incomplete"])
	require.Equal(t, "eof_before_message_stop", adminInfo["claude_stream_incomplete_reason"])
}
