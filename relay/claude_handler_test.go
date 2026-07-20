package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeHelperPassthroughAppliesSamplingCleanupIndependently(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","max_tokens":16,"messages":[{"role":"developer","content":"keep"}],"temperature":0.2,"top_p":0.5,"top_k":42,"unknown_beta":true}`)

	upstreamBody := runClaudeHelperPassthrough(t, body, false, true)

	require.NotContains(t, upstreamBody, `"temperature"`)
	require.NotContains(t, upstreamBody, `"top_p"`)
	require.NotContains(t, upstreamBody, `"top_k"`)
	require.Contains(t, upstreamBody, `"role":"developer"`)
	require.Contains(t, upstreamBody, `"unknown_beta":true`)
}

func TestClaudeHelperPassthroughKeepsSamplingWhenCleanupDisabled(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"temperature":0.2,"top_p":0.5,"top_k":42}`)

	upstreamBody := runClaudeHelperPassthrough(t, body, false, false)

	require.Contains(t, upstreamBody, `"temperature":0.2`)
	require.Contains(t, upstreamBody, `"top_p":0.5`)
	require.Contains(t, upstreamBody, `"top_k":42`)
}

func runClaudeHelperPassthrough(t *testing.T, body []byte, applyCompat bool, cleanupSampling bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	oldGlobalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	oldClaudeSettings := *model_setting.GetClaudeSettings()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled = applyCompat
	model_setting.GetClaudeSettings().DropDefaultSamplingForOpusEnabled = cleanupSampling
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldGlobalPassThrough
		*model_setting.GetClaudeSettings() = oldClaudeSettings
	})

	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"fake upstream"}}`))
	}))
	t.Cleanup(upstream.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage(body)
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-fable-5")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})

	var req dto.ClaudeRequest
	require.NoError(t, common.Unmarshal(body, &req))
	info := &relaycommon.RelayInfo{
		Request:         &req,
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "claude-fable-5",
	}
	info.InitRequestConversionChain()

	newAPIError := ClaudeHelper(c, info)
	require.NotNil(t, newAPIError)
	require.NotEmpty(t, upstreamBody)
	return string(upstreamBody)
}
