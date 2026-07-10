package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const responsesCacheWriteUsageJSON = `{"id":"resp_test","created_at":123,"model":"gpt-test","usage":{"input_tokens":2000,"output_tokens":100,"total_tokens":2100,"input_tokens_details":{"cached_tokens":800,"cached_creation_tokens":999,"cache_write_tokens":400}}}`

func newCacheWriteUsageTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, recorder
}

func newCacheWriteUsageResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func requireMappedCacheWriteUsage(t *testing.T, usage *dto.Usage, want int) {
	t.Helper()
	require.NotNil(t, usage)
	require.Equal(t, 2000, usage.InputTokens)
	require.Equal(t, "openai_responses", usage.UsageSource)
	require.Equal(t, 800, usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, want, usage.PromptTokensDetails.CachedCreationTokens)
	require.NotNil(t, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, want, *usage.PromptTokensDetails.CacheWriteTokens)
}

func TestOaiResponsesHandlerMapsCacheWriteUsage(t *testing.T) {
	c, recorder := newCacheWriteUsageTestContext("/v1/responses")

	usage, apiErr := OaiResponsesHandler(c, nil, newCacheWriteUsageResponse(responsesCacheWriteUsageJSON))

	require.Nil(t, apiErr)
	requireMappedCacheWriteUsage(t, usage, 400)
	require.JSONEq(t, responsesCacheWriteUsageJSON, recorder.Body.String())
}

func TestOaiResponsesCompactionHandlerMapsCacheWriteUsage(t *testing.T) {
	c, recorder := newCacheWriteUsageTestContext("/v1/responses/compact")

	usage, apiErr := OaiResponsesCompactionHandler(c, newCacheWriteUsageResponse(responsesCacheWriteUsageJSON))

	require.Nil(t, apiErr)
	requireMappedCacheWriteUsage(t, usage, 400)
	require.JSONEq(t, responsesCacheWriteUsageJSON, recorder.Body.String())
}

func TestOaiResponsesStreamHandlerMapsCacheWriteUsage(t *testing.T) {
	c, _ := newCacheWriteUsageTestContext("/v1/responses")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	body := "data: {\"type\":\"response.completed\",\"response\":" + responsesCacheWriteUsageJSON + "}\n\n"

	usage, apiErr := OaiResponsesStreamHandler(c, info, newCacheWriteUsageResponse(body))

	require.Nil(t, apiErr)
	requireMappedCacheWriteUsage(t, usage, 400)
}

func TestOaiResponsesToChatStreamHandlerPreservesExplicitZeroCacheWriteUsage(t *testing.T) {
	c, _ := newCacheWriteUsageTestContext("/v1/chat/completions")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	body := `data: {"type":"response.completed","response":{"id":"resp_test","created_at":123,"model":"gpt-test","usage":{"input_tokens":2000,"output_tokens":100,"total_tokens":2100,"input_tokens_details":{"cached_tokens":800,"cached_creation_tokens":999,"cache_write_tokens":0}}}}` + "\n\n"

	usage, apiErr := OaiResponsesToChatStreamHandler(c, info, newCacheWriteUsageResponse(body))

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2000, usage.InputTokens)
	require.Equal(t, "openai_responses", usage.UsageSource)
	require.Equal(t, 800, usage.PromptTokensDetails.CachedTokens)
	require.Zero(t, usage.PromptTokensDetails.CachedCreationTokens)
	require.NotNil(t, usage.PromptTokensDetails.CacheWriteTokens)
	require.Zero(t, *usage.PromptTokensDetails.CacheWriteTokens)
}

func TestOpenaiHandlerNormalizesNativeChatCacheWriteUsage(t *testing.T) {
	c, recorder := newCacheWriteUsageTestContext("/v1/chat/completions")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	}
	body := `{"id":"chatcmpl_test","model":"gpt-test","choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":100,"total_tokens":2100,"prompt_tokens_details":{"cached_tokens":800,"cached_creation_tokens":999,"cache_write_tokens":400}}}`

	usage, apiErr := OpenaiHandler(c, info, newCacheWriteUsageResponse(body))

	require.Nil(t, apiErr)
	require.Equal(t, 400, usage.PromptTokensDetails.CachedCreationTokens)
	require.NotNil(t, usage.PromptTokensDetails.CacheWriteTokens)
	require.Equal(t, 400, *usage.PromptTokensDetails.CacheWriteTokens)
	require.Empty(t, usage.UsageSource)
	require.JSONEq(t, body, recorder.Body.String())
}

func TestOaiStreamHandlerNormalizesNativeChatExplicitZeroCacheWriteUsage(t *testing.T) {
	c, _ := newCacheWriteUsageTestContext("/v1/chat/completions")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-test",
		},
	}
	body := `data: {"id":"chatcmpl_test","object":"chat.completion.chunk","created":123,"model":"gpt-test","choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":100,"total_tokens":2100,"prompt_tokens_details":{"cached_tokens":800,"cached_creation_tokens":999,"cache_write_tokens":0}}}` + "\n\n"

	usage, apiErr := OaiStreamHandler(c, info, newCacheWriteUsageResponse(body))

	require.Nil(t, apiErr)
	require.Zero(t, usage.PromptTokensDetails.CachedCreationTokens)
	require.NotNil(t, usage.PromptTokensDetails.CacheWriteTokens)
	require.Zero(t, *usage.PromptTokensDetails.CacheWriteTokens)
	require.Empty(t, usage.UsageSource)
}

func TestApplyUsagePostProcessingNormalizesNativeChatCacheWriteUsage(t *testing.T) {
	cacheWriteZero := 0
	cacheWriteNonZero := 41
	tests := []struct {
		name             string
		cacheWriteTokens *int
		legacyTokens     int
		wantTokens       int
	}{
		{name: "legacy", legacyTokens: 37, wantTokens: 37},
		{name: "explicit zero", cacheWriteTokens: &cacheWriteZero, legacyTokens: 37, wantTokens: 0},
		{name: "official", cacheWriteTokens: &cacheWriteNonZero, legacyTokens: 37, wantTokens: 41},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &dto.Usage{
				PromptTokensDetails: dto.InputTokenDetails{
					CachedCreationTokens: tt.legacyTokens,
					CacheWriteTokens:     tt.cacheWriteTokens,
				},
				UsageSemantic: "anthropic",
			}

			applyUsagePostProcessing(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, usage, nil)

			require.Equal(t, tt.wantTokens, usage.PromptTokensDetails.CachedCreationTokens)
			require.Empty(t, usage.UsageSource)
			require.Equal(t, "anthropic", usage.UsageSemantic)
		})
	}
}
