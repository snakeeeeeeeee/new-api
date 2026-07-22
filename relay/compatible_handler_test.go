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

func TestTextHelperOpenAIToClaudePassthroughAppliesSamplingCleanupIndependently(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","max_tokens":16,"messages":[{"role":"developer","content":"keep"}],"temperature":0.2,"top_p":0.5,"top_k":42,"unknown_beta":true}`)

	upstreamBody := runTextHelperClaudePassthroughWithSampling(t, body, false, true)

	require.NotContains(t, upstreamBody, `"temperature"`)
	require.NotContains(t, upstreamBody, `"top_p"`)
	require.NotContains(t, upstreamBody, `"top_k"`)
	require.Contains(t, upstreamBody, `"role":"developer"`)
	require.Contains(t, upstreamBody, `"unknown_beta":true`)
}

func TestTextHelperOpenAIToClaudePassthroughKeepsSamplingWhenCleanupDisabled(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"temperature":0.2,"top_p":0.5,"top_k":42}`)

	upstreamBody := runTextHelperClaudePassthroughWithSampling(t, body, false, false)

	require.Contains(t, upstreamBody, `"temperature":0.2`)
	require.Contains(t, upstreamBody, `"top_p":0.5`)
	require.Contains(t, upstreamBody, `"top_k":42`)
}

func TestTextHelperRewritesReservedFunctionNameInSerializedAndPassthroughRequests(t *testing.T) {
	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"python"}],"tools":[{"type":"function","function":{"name":"python","parameters":{"type":"object"}}}]}`)

	for _, passThrough := range []bool{false, true} {
		t.Run(map[bool]string{false: "serialized", true: "passthrough"}[passThrough], func(t *testing.T) {
			upstreamBody := runTextHelperOpenAIReservedFunctionNameCompat(t, body, passThrough)
			require.Contains(t, upstreamBody, `"name":"run_python"`)
			require.Contains(t, upstreamBody, `"content":"python"`)
		})
	}
}

func runTextHelperClaudePassthrough(t *testing.T, body []byte, applyCompat bool) string {
	return runTextHelperClaudePassthroughWithSampling(t, body, applyCompat, false)
}

func runTextHelperClaudePassthroughWithSampling(t *testing.T, body []byte, applyCompat bool, cleanupSampling bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	oldGlobalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	oldClaudeSettings := *model_setting.GetClaudeSettings()
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	model_setting.GetClaudeSettings().ApplyCompatInPassthroughEnabled = applyCompat
	model_setting.GetClaudeSettings().DropDefaultSamplingForOpusEnabled = cleanupSampling
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

func runTextHelperOpenAIReservedFunctionNameCompat(t *testing.T, body []byte, passThrough bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	globalSettings := model_setting.GetGlobalSettings()
	oldGlobalPassThrough := globalSettings.PassThroughRequestEnabled
	oldCompatEnabled := globalSettings.OpenAIReservedFunctionNameCompatEnabled
	oldReservedNames := globalSettings.OpenAIReservedFunctionNames
	globalSettings.PassThroughRequestEnabled = false
	globalSettings.OpenAIReservedFunctionNameCompatEnabled = true
	globalSettings.OpenAIReservedFunctionNames = "python"
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = oldGlobalPassThrough
		globalSettings.OpenAIReservedFunctionNameCompatEnabled = oldCompatEnabled
		globalSettings.OpenAIReservedFunctionNames = oldReservedNames
	})

	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"fake upstream"}}`))
	}))
	t.Cleanup(upstream.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage(body)
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: passThrough})

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(body, &req))
	info := &relaycommon.RelayInfo{
		Request:         &req,
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "gpt-test",
	}
	info.InitRequestConversionChain()

	newAPIError := TextHelper(c, info)
	require.NotNil(t, newAPIError)
	require.NotEmpty(t, upstreamBody)
	return string(upstreamBody)
}
