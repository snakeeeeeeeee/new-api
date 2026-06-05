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
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTextHelperOpenAIToClaudePassthroughAppliesCompatWhenEnabled(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"sys"},{"role":"developer","content":"dev"},{"role":"user","content":"hi"}]}`)

	upstreamBody := runTextHelperClaudePassthrough(t, body, true)

	require.NotContains(t, upstreamBody, `"role":"developer"`)
	require.NotContains(t, upstreamBody, `"role":"system"`)
	require.Contains(t, upstreamBody, `"system"`)
	require.Contains(t, upstreamBody, `"text":"sys"`)
	require.Contains(t, upstreamBody, `"text":"dev"`)
	require.Contains(t, upstreamBody, `"role":"user"`)
}

func TestTextHelperOpenAIToClaudePassthroughKeepsBodyWhenCompatDisabled(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":16,"messages":[{"role":"system","content":"sys"},{"role":"developer","content":"dev"},{"role":"user","content":"hi"}]}`)

	upstreamBody := runTextHelperClaudePassthrough(t, body, false)

	require.Contains(t, upstreamBody, `"role":"developer"`)
	require.Contains(t, upstreamBody, `"role":"system"`)
}

func runTextHelperClaudePassthrough(t *testing.T, body []byte, applyCompat bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	oldGlobalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	oldClaudeSettings := *model_setting.GetClaudeSettings()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled = applyCompat
	model_setting.GetClaudeSettings().PromoteLeadingSystemRoleEnabled = true
	model_setting.GetClaudeSettings().MergeAdjacentSameRoleEnabled = true
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
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage(body)
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-6")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(body, &req))
	info := &relaycommon.RelayInfo{
		Request:         &req,
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "claude-sonnet-4-6",
	}
	info.InitRequestConversionChain()

	newAPIError := TextHelper(c, info)
	require.NotNil(t, newAPIError)
	require.NotEmpty(t, upstreamBody)
	return string(upstreamBody)
}
