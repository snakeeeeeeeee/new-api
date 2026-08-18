package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTextHelperOpenAIIntegrityReturnsRetryable502FromRealUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "non-stream", true: "stream"}[stream], func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("data: {\"id\":\"chat_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n"))
					_, _ = w.Write([]byte("data: {\"id\":\"chat_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"chat_empty","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":0,"total_tokens":2}}`))
			}))
			defer upstream.Close()

			requestBody := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`)
			c, recorder, storage := newOpenAIIntegrityRelayContext(t, "/v1/chat/completions", requestBody, upstream.URL)
			defer storage.Close()
			request := &dto.GeneralOpenAIRequest{Model: "gpt-test", Stream: common.GetPointer(stream), Messages: []dto.Message{{Role: "user", Content: "hello"}}}
			info := newOpenAIIntegrityRelayInfo(request, relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI, stream)

			apiErr := TextHelper(c, info)

			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
			require.Empty(t, recorder.Body.String())
		})
	}
}

func TestResponsesHelperOpenAIIntegrityReturnsRetryable502FromRealUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "non-stream", true: "stream"}[stream], func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_empty\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n"))
					_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":0,\"total_tokens\":2}}}\n\n"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"resp_empty","status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`))
			}))
			defer upstream.Close()

			requestBody := []byte(`{"model":"gpt-test","input":"hello","stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`)
			c, recorder, storage := newOpenAIIntegrityRelayContext(t, "/v1/responses", requestBody, upstream.URL)
			defer storage.Close()
			request := &dto.OpenAIResponsesRequest{Model: "gpt-test", Input: []byte(`"hello"`), Stream: common.GetPointer(stream)}
			info := newOpenAIIntegrityRelayInfo(request, relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses, stream)

			apiErr := ResponsesHelper(c, info)

			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
			require.Empty(t, recorder.Body.String())
		})
	}
}

func newOpenAIIntegrityRelayContext(t *testing.T, path string, body []byte, upstreamURL string) (*gin.Context, *httptest.ResponseRecorder, common.BodyStorage) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage(body)
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	return c, recorder, storage
}

func newOpenAIIntegrityRelayInfo(request dto.Request, mode int, format types.RelayFormat, stream bool) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		Request:                        request,
		RelayFormat:                    format,
		RelayMode:                      mode,
		RequestURLPath:                 map[int]string{relayconstant.RelayModeChatCompletions: "/v1/chat/completions", relayconstant.RelayModeResponses: "/v1/responses"}[mode],
		OriginModelName:                "gpt-test",
		IsStream:                       stream,
		StartTime:                      time.Now(),
		OpenAIResponseIntegrityEnabled: true,
		OpenAIResponseIntegrityFirstOutputTimeout: time.Second,
	}
	info.InitRequestConversionChain()
	return info
}
