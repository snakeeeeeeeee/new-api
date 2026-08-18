package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAIIntegrityTestContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, recorder
}

func newOpenAIIntegrityTestInfo(mode int, stream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:                      mode,
		RelayFormat:                    types.RelayFormatOpenAI,
		IsStream:                       stream,
		OpenAIResponseIntegrityEnabled: true,
		OpenAIResponseIntegrityFirstOutputTimeout: time.Second,
		StartTime: time.Now(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
}

func openAIIntegrityHTTPResponse(body io.Reader, contentType string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(body),
	}
}

func TestOpenAIIntegrityNonStreamEmptyResponsesReturn502BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		path string
		body string
		run  func(*gin.Context, *relaycommon.RelayInfo, *http.Response) *types.NewAPIError
	}{
		{
			name: "chat",
			path: "/v1/chat/completions",
			body: `{"id":"chat_empty","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":0,"total_tokens":2}}`,
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OpenaiHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses",
			path: "/v1/responses",
			body: `{"id":"resp_empty","status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`,
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses to chat",
			path: "/v1/chat/completions",
			body: `{"id":"resp_empty","status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`,
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesToChatHandler(c, info, resp)
				return apiErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newOpenAIIntegrityTestContext(tt.path)
			mode := relayconstant.RelayModeChatCompletions
			if tt.path == "/v1/responses" {
				mode = relayconstant.RelayModeResponses
			}
			apiErr := tt.run(c, newOpenAIIntegrityTestInfo(mode, false), openAIIntegrityHTTPResponse(strings.NewReader(tt.body), "application/json"))

			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
			require.Empty(t, recorder.Body.String())
			require.NotNil(t, apiErr.Diagnostic)
			require.Equal(t, false, apiErr.Diagnostic.StreamSummary["response_committed"])
		})
	}
}

func TestOpenAIIntegrityAllowsMeaningfulAndExplicitlyLimitedNonStreamResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("chat tool call", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
		body := `{"id":"chat_tool","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`
		_, apiErr := OpenaiHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeChatCompletions, false), openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))
		require.Nil(t, apiErr)
		require.Contains(t, recorder.Body.String(), "call_1")
	})

	t.Run("chat length", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
		body := `{"id":"chat_length","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"length"}],"usage":{"prompt_tokens":2,"completion_tokens":0,"total_tokens":2}}`
		_, apiErr := OpenaiHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeChatCompletions, false), openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))
		require.Nil(t, apiErr)
		require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
	})

	t.Run("responses tool output", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
		body := `{"id":"resp_tool","status":"completed","model":"gpt-test","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}],"usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}`
		_, apiErr := OaiResponsesHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, false), openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))
		require.Nil(t, apiErr)
		require.Contains(t, recorder.Body.String(), "function_call")
	})

	t.Run("responses to chat max output tokens", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
		body := `{"id":"resp_limited","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`
		_, apiErr := OaiResponsesToChatHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeChatCompletions, false), openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))
		require.Nil(t, apiErr)
		require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
	})
}

func TestOpenAIIntegrityEmptyStreamsReturn502BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		path string
		mode int
		body string
		run  func(*gin.Context, *relaycommon.RelayInfo, *http.Response) *types.NewAPIError
	}{
		{
			name: "chat empty stop",
			path: "/v1/chat/completions",
			mode: relayconstant.RelayModeChatCompletions,
			body: "data: {\"id\":\"chat_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"chat_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: [DONE]\n\n",
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses empty completed",
			path: "/v1/responses",
			mode: relayconstant.RelayModeResponses,
			body: "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_empty\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":0,\"total_tokens\":2}}}\n\n",
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesStreamHandler(c, info, resp)
				return apiErr
			},
		},
		{
			name: "responses to chat empty completed",
			path: "/v1/chat/completions",
			mode: relayconstant.RelayModeResponses,
			body: "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_empty\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":0,\"total_tokens\":2}}}\n\n",
			run: func(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
				_, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
				return apiErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newOpenAIIntegrityTestContext(tt.path)
			apiErr := tt.run(c, newOpenAIIntegrityTestInfo(tt.mode, true), openAIIntegrityHTTPResponse(strings.NewReader(tt.body), "text/event-stream"))
			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
			require.Empty(t, recorder.Body.String())
		})
	}
}

func TestOpenAIIntegrityStreamMeaningfulOutputAndPostCommitFailureDoNotRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("chat text", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
		body := "data: {\"id\":\"chat_text\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
			"data: {\"id\":\"chat_text\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n" +
			"data: {\"id\":\"chat_text\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"
		_, apiErr := OaiStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeChatCompletions, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))
		require.Nil(t, apiErr)
		require.Contains(t, recorder.Body.String(), "hello")
	})

	t.Run("responses failure after text", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
		body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
			"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"status\":\"failed\",\"model\":\"gpt-test\",\"error\":{\"message\":\"provider failed\",\"type\":\"server_error\",\"code\":\"upstream_failed\"}}}\n\n"
		usage, apiErr := OaiResponsesStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))
		require.Nil(t, apiErr)
		require.Positive(t, usage.CompletionTokens)
		require.Contains(t, recorder.Body.String(), "partial")
		require.Contains(t, recorder.Body.String(), "response.failed")
	})

	t.Run("responses to chat failure after text", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
		body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
			"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"status\":\"failed\",\"model\":\"gpt-test\",\"error\":{\"message\":\"provider failed\",\"type\":\"server_error\",\"code\":\"upstream_failed\"}}}\n\n"
		usage, apiErr := OaiResponsesToChatStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))
		require.Nil(t, apiErr)
		require.Positive(t, usage.CompletionTokens)
		require.Contains(t, recorder.Body.String(), "partial")
		require.NotContains(t, recorder.Body.String(), "[DONE]")
	})
}

func TestOpenAIIntegrityPreservesExplicitStreamRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	body := `data: {"type":"response.error","error":{"message":"too many requests","type":"rate_limit_error","code":"rate_limit_exceeded"}}` + "\n\n"

	_, apiErr := OaiResponsesStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.Equal(t, types.ErrorCode("rate_limit_exceeded"), apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
}

func TestOpenAIIntegrityFirstOutputTimeoutReturns502(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	info := newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true)
	info.OpenAIResponseIntegrityFirstOutputTimeout = 20 * time.Millisecond
	info.BeginOpenAIResponseIntegrityAttempt(c.Request.Context())
	defer info.EndOpenAIResponseIntegrityAttempt()
	reader, writer := io.Pipe()
	defer writer.Close()

	done := make(chan *types.NewAPIError, 1)
	go func() {
		_, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: http.StatusOK, Body: reader})
		done <- apiErr
	}()

	select {
	case apiErr := <-done:
		require.NotNil(t, apiErr)
		require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
		require.Empty(t, recorder.Body.String())
	case <-time.After(time.Second):
		t.Fatal("integrity handler did not return after first-output timeout")
	}
}

func TestOpenAIIntegrityResponsesToChatIncompleteStreamUsesLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/chat/completions")
	body := `data: {"type":"response.incomplete","response":{"id":"resp_limited","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}}` + "\n\n"

	_, apiErr := OaiResponsesToChatStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))

	require.Nil(t, apiErr)
	require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
}

func TestOpenAIIntegrityExplicitNonStreamServerErrorUses502(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	body := `{"id":"resp_failed","status":"failed","model":"gpt-test","output":[],"error":{"message":"provider failed","type":"server_error","code":"upstream_failed"}}`

	_, apiErr := OaiResponsesHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, false), openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCode("upstream_failed"), apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
}

func TestOpenAIIntegrityMalformedNonStreamResponseIsRetryableOnlyWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("enabled", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
		_, apiErr := OaiResponsesHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, false), openAIIntegrityHTTPResponse(strings.NewReader(`{"broken"`), "application/json"))
		require.NotNil(t, apiErr)
		require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
		require.Empty(t, recorder.Body.String())
	})

	t.Run("disabled legacy behavior", func(t *testing.T) {
		c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
		info := newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, false)
		info.OpenAIResponseIntegrityEnabled = false
		_, apiErr := OaiResponsesHandler(c, info, openAIIntegrityHTTPResponse(strings.NewReader(`{"broken"`), "application/json"))
		require.NotNil(t, apiErr)
		require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
		require.Empty(t, recorder.Body.String())
	})
}

func TestOpenAIIntegrityDisabledPreservesEmptySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	info := newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, false)
	info.OpenAIResponseIntegrityEnabled = false
	body := `{"id":"resp_empty","status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`

	_, apiErr := OaiResponsesHandler(c, info, openAIIntegrityHTTPResponse(strings.NewReader(body), "application/json"))

	require.Nil(t, apiErr)
	require.Contains(t, recorder.Body.String(), `"output":[]`)
}

func TestOpenAIIntegrityClassifiesProtocolSpecificMeaningfulOutput(t *testing.T) {
	tests := []struct {
		name       string
		protocol   openAIIntegrityProtocol
		data       string
		meaningful bool
	}{
		{name: "chat reasoning", protocol: openAIIntegrityChat, data: `{"choices":[{"delta":{"reasoning_content":"think"},"finish_reason":null}]}`, meaningful: true},
		{name: "chat refusal", protocol: openAIIntegrityChat, data: `{"choices":[{"delta":{"refusal":"blocked"},"finish_reason":null}]}`, meaningful: true},
		{name: "chat audio", protocol: openAIIntegrityChat, data: `{"choices":[{"delta":{"audio":{"id":"audio_1"}},"finish_reason":null}]}`, meaningful: true},
		{name: "responses refusal", protocol: openAIIntegrityResponses, data: `{"type":"response.refusal.delta","delta":"blocked"}`, meaningful: true},
		{name: "responses reasoning", protocol: openAIIntegrityResponses, data: `{"type":"response.reasoning_text.delta","delta":"think"}`, meaningful: true},
		{name: "responses image", protocol: openAIIntegrityResponses, data: `{"type":"response.output_item.added","item":{"id":"img_1","type":"image_generation_call","status":"in_progress"}}`, meaningful: true},
		{name: "responses custom tool", protocol: openAIIntegrityResponses, data: `{"type":"response.output_item.added","item":{"id":"custom_1","type":"custom_tool_call","call_id":"call_1","name":"browser"}}`, meaningful: true},
		{name: "responses to chat function", protocol: openAIIntegrityResponsesToChat, data: `{"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"lookup","arguments":""}}`, meaningful: true},
		{name: "responses to chat lifecycle", protocol: openAIIntegrityResponsesToChat, data: `{"type":"response.created","response":{"id":"resp_1","status":"in_progress","output":[]}}`, meaningful: false},
		{name: "responses to chat custom tool", protocol: openAIIntegrityResponsesToChat, data: `{"type":"response.output_item.added","item":{"id":"custom_1","type":"custom_tool_call","call_id":"call_1","name":"browser"}}`, meaningful: false},
		{name: "responses to chat unconvertible built-in", protocol: openAIIntegrityResponsesToChat, data: `{"type":"response.output_item.added","item":{"id":"web_1","type":"web_search_call","status":"in_progress"}}`, meaningful: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := classifyOpenAIIntegrityEvent(tt.data, tt.protocol)
			require.NoError(t, err)
			require.Equal(t, tt.meaningful, event.meaningful)
		})
	}
}

func TestOpenAIIntegrityBufferLimitReturns502BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	var body strings.Builder
	for i := 0; i <= openAIIntegrityMaxBufferedEvents; i++ {
		body.WriteString(`data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress","model":"gpt-test","output":[]}}`)
		body.WriteString("\n\n")
	}

	_, apiErr := OaiResponsesStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body.String()), "text/event-stream"))

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
	require.Equal(t, "first_output_buffer_limit", apiErr.Diagnostic.StreamSummary["integrity_reason"])
}

func TestOpenAIIntegrityBufferByteLimitReturns502BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	body := `data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress","model":"gpt-test","output":[]},"padding":"` + strings.Repeat("x", openAIIntegrityMaxBufferedBytes) + `"}` + "\n\n"

	_, apiErr := OaiResponsesStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), openAIIntegrityHTTPResponse(strings.NewReader(body), "text/event-stream"))

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeEmptyResponse, apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
	require.Equal(t, "first_output_buffer_limit", apiErr.Diagnostic.StreamSummary["integrity_reason"])
}

func TestOpenAIIntegrityClientDisconnectReturns499BeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newOpenAIIntegrityTestContext("/v1/responses")
	requestContext, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(requestContext)
	cancel()
	reader, writer := io.Pipe()
	defer writer.Close()

	_, apiErr := OaiResponsesStreamHandler(c, newOpenAIIntegrityTestInfo(relayconstant.RelayModeResponses, true), &http.Response{StatusCode: http.StatusOK, Body: reader})

	require.NotNil(t, apiErr)
	require.Equal(t, 499, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.Empty(t, recorder.Body.String())
}
