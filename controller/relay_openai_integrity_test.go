package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmptyResponseUsesExistingRetryPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	apiErr := types.NewErrorWithStatusCode(
		errors.New("upstream returned no usable output"),
		types.ErrorCodeEmptyResponse,
		http.StatusBadGateway,
		types.ErrOptionWithClientSafe(),
	)

	require.True(t, shouldRetry(c, apiErr, 1))
	require.False(t, shouldRetry(c, apiErr, 0))

	c.Set("specific_channel_id", 7)
	require.False(t, shouldRetry(c, apiErr, 1))
}

func TestOpenAIEmptyResponseHonorsAggregateRetryStatusCodes(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	group := &model.AggregateGroup{
		Name:                    "openai-integrity-retry-status",
		DisplayName:             "OpenAI integrity retry status",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 30,
		RetryStatusCodes:        "500-501,503-599",
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"default"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}, {RealGroup: "vip"}}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, group.Name)
	apiErr := types.NewErrorWithStatusCode(errors.New("empty output"), types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	require.False(t, shouldRetry(c, apiErr, 1))

	group.RetryStatusCodes = "500-599"
	require.NoError(t, group.UpdateWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}, {RealGroup: "vip"}}))
	require.True(t, shouldRetry(c, apiErr, 1))
}

func TestOpenAIIntegrityClientDisconnectSkipsRetry(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	disconnectErr := types.NewErrorWithStatusCode(
		errors.New("client canceled request"),
		types.ErrorCodeDoRequestFailed,
		499,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
	require.False(t, shouldRetry(c, disconnectErr, 1))
}

func TestOpenAIIntegrityRetriesEmptyFirstUpstreamWithoutDownstreamContamination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type streamHandler func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	tests := []struct {
		name      string
		path      string
		mode      int
		stream    bool
		emptyBody string
		validBody string
		handler   streamHandler
	}{
		{
			name:      "chat non-stream",
			path:      "/v1/chat/completions",
			mode:      relayconstant.RelayModeChatCompletions,
			emptyBody: `{"id":"first_empty","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":0,"total_tokens":2}}`,
			validBody: `{"id":"second_valid","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"second answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`,
			handler:   openaichannel.OpenaiHandler,
		},
		{
			name:   "chat stream",
			path:   "/v1/chat/completions",
			mode:   relayconstant.RelayModeChatCompletions,
			stream: true,
			emptyBody: "data: {\"id\":\"first_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"first_empty\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
			validBody: "data: {\"id\":\"second_valid\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"second_valid\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"second answer\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"second_valid\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
			handler: openaichannel.OaiStreamHandler,
		},
		{
			name:      "responses non-stream",
			path:      "/v1/responses",
			mode:      relayconstant.RelayModeResponses,
			emptyBody: openAIEmptyResponsesBody("first_empty"),
			validBody: openAIValidResponsesBody("second_valid"),
			handler:   openaichannel.OaiResponsesHandler,
		},
		{
			name:      "responses stream",
			path:      "/v1/responses",
			mode:      relayconstant.RelayModeResponses,
			stream:    true,
			emptyBody: openAIEmptyResponsesStream("first_empty"),
			validBody: openAIValidResponsesStream("second_valid"),
			handler:   openaichannel.OaiResponsesStreamHandler,
		},
		{
			name:      "responses to chat non-stream",
			path:      "/v1/chat/completions",
			mode:      relayconstant.RelayModeResponses,
			emptyBody: openAIEmptyResponsesBody("first_empty"),
			validBody: openAIValidResponsesBody("second_valid"),
			handler:   openaichannel.OaiResponsesToChatHandler,
		},
		{
			name:      "responses to chat stream",
			path:      "/v1/chat/completions",
			mode:      relayconstant.RelayModeResponses,
			stream:    true,
			emptyBody: openAIEmptyResponsesStream("first_empty"),
			validBody: openAIValidResponsesStream("second_valid"),
			handler:   openaichannel.OaiResponsesToChatStreamHandler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentType := "application/json"
			if tt.stream {
				contentType = "text/event-stream"
			}
			servers := []*httptest.Server{
				newOpenAIIntegrityUpstream(tt.emptyBody, contentType),
				newOpenAIIntegrityUpstream(tt.validBody, contentType),
			}
			defer servers[0].Close()
			defer servers[1].Close()

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{}`))
			info := &relaycommon.RelayInfo{
				RelayMode:                      tt.mode,
				RelayFormat:                    types.RelayFormatOpenAI,
				IsStream:                       tt.stream,
				OriginModelName:                "gpt-test",
				OpenAIResponseIntegrityEnabled: true,
				OpenAIResponseIntegrityFirstOutputTimeout: time.Second,
				StartTime: time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "gpt-test",
				},
			}
			info.SetEstimatePromptTokens(2)

			var finalErr *types.NewAPIError
			for index, upstream := range servers {
				c.Set("channel_name", []string{"empty", "valid"}[index])
				addUsedChannel(c, index+101)
				info.BeginOpenAIResponseIntegrityAttempt(c.Request.Context())
				response, err := http.Post(upstream.URL, "application/json", strings.NewReader(`{}`))
				require.NoError(t, err)
				_, finalErr = tt.handler(c, info, response)
				info.EndOpenAIResponseIntegrityAttempt()
				if finalErr == nil {
					break
				}
				require.Equal(t, 0, index)
				require.Equal(t, http.StatusBadGateway, finalErr.StatusCode)
				require.Equal(t, types.ErrorCodeEmptyResponse, finalErr.GetErrorCode())
				require.Empty(t, recorder.Body.String())
				require.True(t, shouldRetry(c, finalErr, 1))
			}

			require.Nil(t, finalErr)
			require.Contains(t, recorder.Body.String(), "second answer")
			require.NotContains(t, recorder.Body.String(), "first_empty")
			require.Equal(t, []string{"101", "102"}, c.GetStringSlice("use_channel"))
		})
	}
}

func newOpenAIIntegrityUpstream(body string, contentType string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
}

func openAIEmptyResponsesBody(id string) string {
	return `{"id":"` + id + `","status":"completed","model":"gpt-test","output":[],"usage":{"input_tokens":2,"output_tokens":0,"total_tokens":2}}`
}

func openAIValidResponsesBody(id string) string {
	return `{"id":"` + id + `","status":"completed","model":"gpt-test","output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"second answer"}]}],"usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}`
}

func openAIEmptyResponsesStream(id string) string {
	return "data: {\"type\":\"response.created\",\"response\":{\"id\":\"" + id + "\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"" + id + "\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":0,\"total_tokens\":2}}}\n\n"
}

func openAIValidResponsesStream(id string) string {
	return "data: {\"type\":\"response.output_text.delta\",\"delta\":\"second answer\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":" + openAIValidResponsesBody(id) + "}\n\n"
}
