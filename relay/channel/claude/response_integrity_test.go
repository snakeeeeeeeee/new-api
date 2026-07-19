package claude

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newClaudeIntegrityTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c, recorder
}

func newClaudeIntegrityTestInfo(c *gin.Context, stream bool, maxTokens uint) *relaycommon.RelayInfo {
	start := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:                                start,
		IsStream:                                 stream,
		RelayFormat:                              types.RelayFormatClaude,
		OriginModelName:                          "claude-test",
		ChannelMeta:                              &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
		ClaudeResponseIntegrityEnabled:           true,
		ClaudeResponseIntegrityFirstBlockTimeout: time.Second,
		Request:                                  &dto.ClaudeRequest{MaxTokens: common.GetPointer(maxTokens)},
	}
	info.MarkFinalRequestRelayFormat(types.RelayFormatClaude)
	info.BeginClaudeResponseIntegrityAttempt(c.Request.Context())
	return info
}

func claudeIntegrityHTTPResponse(body io.Reader, stream bool) *http.Response {
	contentType := "application/json"
	if stream {
		contentType = "text/event-stream"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(body),
	}
}

type claudeIntegrityErrorReader struct {
	data []byte
	err  error
}

func (r *claudeIntegrityErrorReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

type claudeIntegritySignalRecorder struct {
	*httptest.ResponseRecorder
	wrote chan struct{}
	once  sync.Once
}

type claudeIntegrityFlushErrorRecorder struct {
	*httptest.ResponseRecorder
	err error
}

type claudeIntegrityTimingRecorder struct {
	*httptest.ResponseRecorder
	startedAt  time.Time
	firstWrite time.Duration
	once       sync.Once
}

func (r *claudeIntegrityTimingRecorder) Write(p []byte) (int, error) {
	r.once.Do(func() { r.firstWrite = time.Since(r.startedAt) })
	return r.ResponseRecorder.Write(p)
}

func (r *claudeIntegrityFlushErrorRecorder) FlushError() error {
	return r.err
}

func (r *claudeIntegritySignalRecorder) Write(p []byte) (int, error) {
	r.once.Do(func() { close(r.wrote) })
	return r.ResponseRecorder.Write(p)
}

func TestClaudeIntegrityHandlerRejectsEmptyContent(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, false, 32)
	resp := claudeIntegrityHTTPResponse(strings.NewReader(`{"type":"message","content":[],"usage":{"input_tokens":2,"output_tokens":0}}`), false)

	usage, apiErr := ClaudeIntegrityHandler(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Equal(t, string(types.ErrorCodeClaudeContentBlockMissing), apiErr.ToClaudeError().Code)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.ToOpenAIError().Code)
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityHandlerAllowsExplicitZeroMaxTokens(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, false, 0)
	resp := claudeIntegrityHTTPResponse(strings.NewReader(`{"type":"message","content":[],"usage":{"input_tokens":2,"output_tokens":0}}`), false)

	usage, apiErr := ClaudeIntegrityHandler(c, resp, info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), `"content":[]`)
}

func TestClaudeIntegrityHandlerStillRejectsInvalidBlockForExplicitZeroMaxTokens(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, false, 0)
	resp := claudeIntegrityHTTPResponse(strings.NewReader(`{"type":"message","content":[{"text":"missing type"}],"usage":{"input_tokens":1,"output_tokens":0}}`), false)

	usage, apiErr := ClaudeIntegrityHandler(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityHandlerAllowsUnknownTypedBlock(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, false, 32)
	resp := claudeIntegrityHTTPResponse(strings.NewReader(`{"type":"message","content":[{"type":"future_block"}],"usage":{"input_tokens":2,"output_tokens":1}}`), false)

	usage, apiErr := ClaudeIntegrityHandler(c, resp, info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), `"type":"future_block"`)
}

func TestClaudeIntegrityHandlerValidatesContentBlocks(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError bool
	}{
		{name: "text", body: `{"type":"message","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":1}}`},
		{name: "tool use", body: `{"type":"message","content":[{"type":"tool_use","id":"tool_1","name":"lookup","input":{}}],"usage":{"input_tokens":2,"output_tokens":1}}`},
		{name: "thinking", body: `{"type":"message","content":[{"type":"thinking","thinking":"work"}],"usage":{"input_tokens":2,"output_tokens":1}}`},
		{name: "refusal", body: `{"type":"message","content":[{"type":"refusal","text":"no"}],"stop_reason":"refusal","usage":{"input_tokens":2,"output_tokens":1}}`},
		{name: "missing type", body: `{"type":"message","content":[{"text":"bad"}],"usage":{"input_tokens":2,"output_tokens":1}}`, wantError: true},
		{name: "malformed json", body: `{"type":"message","content":[`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := newClaudeIntegrityTestContext(t)
			info := newClaudeIntegrityTestInfo(c, false, 32)

			usage, apiErr := ClaudeIntegrityHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(test.body), false), info)

			if test.wantError {
				require.Nil(t, usage)
				require.NotNil(t, apiErr)
				require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
				require.Empty(t, recorder.Body.String())
				return
			}
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.NotEmpty(t, recorder.Body.String())
		})
	}
}

func TestClaudeIntegrityStreamBuffersUntilFirstBlockAndPreservesOrder(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"ping"}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	output := recorder.Body.String()
	messageStart := strings.Index(output, "event: message_start")
	ping := strings.Index(output, "event: ping")
	blockStart := strings.Index(output, "event: content_block_start")
	require.GreaterOrEqual(t, messageStart, 0)
	require.Greater(t, ping, messageStart)
	require.Greater(t, blockStart, ping)
	require.Contains(t, output, "event: message_stop")
	require.True(t, info.HasSendResponse())
	require.Equal(t, 7, info.ReceivedResponseCount)
	require.Equal(t, 7, info.SendResponseCount)
	require.False(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
}

func TestClaudeIntegrityStreamRejectsUnknownDeltaBeforeCommit(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"message_start\",\"message\":{}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"bad\"}}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityStreamEmitsErrorForUnknownDeltaAfterCommit(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"bad\"}}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
	require.Contains(t, recorder.Body.String(), "event: content_block_start")
	require.Contains(t, recorder.Body.String(), "event: error")
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeClaudeStreamIncomplete))
}

func TestClaudeIntegrityStreamEmitsErrorWhenMessageStopIsMissing(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
	require.Contains(t, recorder.Body.String(), "event: error")
}

func TestClaudeIntegrityStreamConvertsClaudeToOpenAI(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	info.RelayFormat = types.RelayFormatOpenAI
	info.ShouldIncludeUsage = true
	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	output := recorder.Body.String()
	require.Contains(t, output, `"content":"hello"`)
	require.Contains(t, output, "data: [DONE]")
	require.NotContains(t, output, "event: content_block_start")
}

func TestClaudeIntegrityStreamUsesOpenAIErrorShapeAfterCommit(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	info.RelayFormat = types.RelayFormatOpenAI
	body := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":1}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), `"error":{"message":"Upstream Claude stream ended before completion."`)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeClaudeStreamIncomplete))
}

func TestClaudeIntegrityStreamAllowsEmptySequenceForExplicitZeroMaxTokens(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 0)
	body := "data: {\"type\":\"message_start\",\"message\":{}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), "event: message_start")
	require.Contains(t, recorder.Body.String(), "event: message_stop")
}

func TestClaudeIntegrityStreamFirstBlockTimeout(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	info.EndClaudeResponseIntegrityAttempt()
	info.ClaudeResponseIntegrityFirstBlockTimeout = 20 * time.Millisecond
	info.BeginClaudeResponseIntegrityAttempt(c.Request.Context())
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(reader, true), info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "first_block_timeout")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityStreamRejectsMalformedJSONBeforeCommit(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"message_start\",\"message\":{}}\n\n" +
		"data: {not-json}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "malformed_json_before_first_block")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityStreamEmitsErrorForMalformedJSONAfterCommit(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"future_block\"}}\n\n" +
		"data: {not-json}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
	require.Contains(t, recorder.Body.String(), "event: error")
}

func TestClaudeIntegrityStreamNeverFallsBackAfterWriteWhenFlushFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	flushErr := errors.New("synthetic downstream flush failure")
	recorder := &claudeIntegrityFlushErrorRecorder{ResponseRecorder: httptest.NewRecorder(), err: flushErr}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.True(t, c.Writer.Written())
	require.Zero(t, info.SendResponseCount, "failed flush must not be counted as a successfully sent event")
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
	require.Equal(t, "downstream_write_error", common.GetContextKeyString(c, constant.ContextKeyClaudeStreamIncompleteReason))
}

func TestClaudeIntegrityStreamMapsReaderErrorByCommitState(t *testing.T) {
	sentinel := errors.New("synthetic stream read failure")
	tests := []struct {
		name      string
		data      string
		committed bool
	}{
		{name: "before commit", data: "data: {\"type\":\"message_start\",\"message\":{}}\n\n"},
		{name: "after commit", data: "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n", committed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, recorder := newClaudeIntegrityTestContext(t)
			info := newClaudeIntegrityTestInfo(c, true, 32)
			reader := &claudeIntegrityErrorReader{data: []byte(test.data), err: sentinel}

			usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(reader, true), info)

			if !test.committed {
				require.Nil(t, usage)
				require.NotNil(t, apiErr)
				require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
				require.Empty(t, recorder.Body.String())
				return
			}
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
			require.Contains(t, recorder.Body.String(), "event: error")
		})
	}
}

func TestClaudeIntegrityStreamAttemptStateDoesNotPolluteFallbackSuccess(t *testing.T) {
	c, _ := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	common.SetContextKey(c, constant.ContextKeyUpstreamFirstEventMs, 999)
	failedBody := "data: {\"type\":\"message_start\",\"message\":{\"model\":\"failed-model\",\"usage\":{\"input_tokens\":999,\"output_tokens\":0}}}\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(failedBody), true), info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())

	info.ChannelMeta.UpstreamModelName = "claude-test"
	info.UpstreamModelName = "claude-test"
	info.BeginClaudeResponseIntegrityAttempt(c.Request.Context())
	successBody := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_final","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")
	usage, apiErr = ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(successBody), true), info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.PromptTokens)
	require.Equal(t, 1, usage.CompletionTokens)
	require.Equal(t, "claude-test", info.UpstreamModelName)
	require.Equal(t, 6, info.ReceivedResponseCount)
	require.Less(t, common.GetContextKeyInt(c, constant.ContextKeyUpstreamFirstEventMs), 999)
}

func TestClaudeIntegrityStreamClientDisconnectBeforeCommitDoesNotEmitOrReturnUsage(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	clientCtx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(clientCtx)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	done := make(chan struct {
		usage  *dto.Usage
		apiErr *types.NewAPIError
	}, 1)
	go func() {
		usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(reader, true), info)
		done <- struct {
			usage  *dto.Usage
			apiErr *types.NewAPIError
		}{usage: usage, apiErr: apiErr}
	}()

	cancel()
	result := <-done
	require.Nil(t, result.usage)
	require.NotNil(t, result.apiErr)
	require.Equal(t, 499, result.apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(result.apiErr))
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityStreamClientDisconnectAfterCommitReturnsUsageWithoutFallbackError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &claudeIntegritySignalRecorder{ResponseRecorder: httptest.NewRecorder(), wrote: make(chan struct{})}
	c, _ := gin.CreateTestContext(recorder)
	clientCtx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(clientCtx)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	done := make(chan struct {
		usage  *dto.Usage
		apiErr *types.NewAPIError
	}, 1)
	go func() {
		usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(reader, true), info)
		done <- struct {
			usage  *dto.Usage
			apiErr *types.NewAPIError
		}{usage: usage, apiErr: apiErr}
	}()

	_, err := io.WriteString(writer, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
	require.NoError(t, err)
	select {
	case <-recorder.wrote:
	case <-time.After(time.Second):
		t.Fatal("protected stream did not commit first block")
	}
	cancel()
	result := <-done
	require.Nil(t, result.apiErr)
	require.NotNil(t, result.usage)
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyClaudeStreamIncomplete))
	require.Equal(t, "client_disconnected", common.GetContextKeyString(c, constant.ContextKeyClaudeStreamIncompleteReason))
	require.NotContains(t, recorder.Body.String(), "event: error")
}

func TestClaudeIntegrityDoRequestCancelsUpstreamBeforeHeadersOnFirstBlockTimeout(t *testing.T) {
	service.InitHttpClient()
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	c, _ := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	info.EndClaudeResponseIntegrityAttempt()
	info.ClaudeResponseIntegrityFirstBlockTimeout = 500 * time.Millisecond
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: server.URL, UpstreamModelName: "claude-test"}
	adaptor := &Adaptor{}

	startedAt := time.Now()
	_, err := adaptor.DoRequest(c, info, strings.NewReader(`{"model":"claude-test","stream":true}`))
	elapsed := time.Since(startedAt)

	require.Error(t, err)
	var apiErr *types.NewAPIError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request was not started")
	}
	require.Less(t, elapsed, 1500*time.Millisecond)
}

func TestClaudeIntegrityStreamRejectsBufferOverflow(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	var builder strings.Builder
	for i := 0; i <= claudeIntegrityMaxBufferedEvents; i++ {
		builder.WriteString("data: {\"type\":\"ping\"}\n\n")
	}

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(builder.String()), true), info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "first_block_buffer_limit")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegrityStreamRejectsByteBufferOverflow(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := newClaudeIntegrityTestInfo(c, true, 32)
	body := `data: {"type":"ping","padding":"` + strings.Repeat("x", claudeIntegrityMaxBufferedBytes) + `"}` + "\n\n"

	usage, apiErr := ClaudeIntegrityStreamHandler(c, claudeIntegrityHTTPResponse(strings.NewReader(body), true), info)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeClaudeContentBlockMissing, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "first_block_buffer_limit")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeIntegritySwitchOffUsesLegacyNonStreamHandler(t *testing.T) {
	c, recorder := newClaudeIntegrityTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:                    types.RelayFormatClaude,
		ChannelMeta:                    &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
		ClaudeResponseIntegrityEnabled: false,
	}
	adaptor := &Adaptor{}
	resp := claudeIntegrityHTTPResponse(strings.NewReader(`{"type":"message","content":[],"usage":{"input_tokens":2,"output_tokens":0}}`), false)

	usage, apiErr := adaptor.DoResponse(c, resp, info)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), `"content":[]`)
}

func TestClaudeSwitchOffUsesLegacyStreamByteForByte(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_legacy","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")
	newInfo := func() *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			StartTime:                      time.Now(),
			IsStream:                       true,
			RelayFormat:                    types.RelayFormatClaude,
			OriginModelName:                "claude-test",
			ChannelMeta:                    &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
			ClaudeResponseIntegrityEnabled: false,
		}
	}

	legacyContext, legacyRecorder := newClaudeIntegrityTestContext(t)
	legacyUsage, legacyErr := ClaudeStreamHandler(
		legacyContext,
		claudeIntegrityHTTPResponse(strings.NewReader(body), true),
		newInfo(),
	)
	require.Nil(t, legacyErr)

	dispatchContext, dispatchRecorder := newClaudeIntegrityTestContext(t)
	dispatchUsage, dispatchErr := (&Adaptor{}).DoResponse(
		dispatchContext,
		claudeIntegrityHTTPResponse(strings.NewReader(body), true),
		newInfo(),
	)
	require.Nil(t, dispatchErr)
	require.Equal(t, legacyRecorder.Body.Bytes(), dispatchRecorder.Body.Bytes())
	require.Equal(t, legacyUsage, dispatchUsage)
}

func TestClaudeIntegrityStateRejectsDuplicateAndUnclosedBlocks(t *testing.T) {
	state := newClaudeIntegrityState(false)
	index := 0
	_, err := state.observe(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        &index,
		ContentBlock: &dto.ClaudeMediaMessage{Type: "future_block"},
	})
	require.NoError(t, err)
	_, err = state.observe(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        &index,
		ContentBlock: &dto.ClaudeMediaMessage{Type: "text"},
	})
	require.ErrorContains(t, err, "duplicate")
	_, err = state.observe(&dto.ClaudeResponse{Type: "message_stop"})
	require.ErrorContains(t, err, "open content block")
}

func BenchmarkClaudeStreamIntegrity(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	common.LogWriterMu.Unlock()
	b.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		gin.DefaultErrorWriter = oldErrorWriter
		common.LogWriterMu.Unlock()
	})
	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")
	run := func(b *testing.B, protected bool) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			start := time.Now()
			info := &relaycommon.RelayInfo{
				StartTime:                                start,
				IsStream:                                 true,
				RelayFormat:                              types.RelayFormatClaude,
				OriginModelName:                          "claude-test",
				ChannelMeta:                              &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
				ClaudeResponseIntegrityEnabled:           protected,
				ClaudeResponseIntegrityFirstBlockTimeout: time.Second,
				Request:                                  &dto.ClaudeRequest{MaxTokens: common.GetPointer[uint](32)},
			}
			resp := claudeIntegrityHTTPResponse(strings.NewReader(body), true)
			if protected {
				info.MarkFinalRequestRelayFormat(types.RelayFormatClaude)
				info.BeginClaudeResponseIntegrityAttempt(c.Request.Context())
				_, _ = ClaudeIntegrityStreamHandler(c, resp, info)
			} else {
				_, _ = ClaudeStreamHandler(c, resp, info)
			}
		}
	}
	b.Run("legacy_switch_off", func(b *testing.B) { run(b, false) })
	b.Run("integrity_switch_on", func(b *testing.B) { run(b, true) })
}

func BenchmarkClaudeStreamFirstBlockLatency(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	common.LogWriterMu.Unlock()
	b.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		gin.DefaultErrorWriter = oldErrorWriter
		common.LogWriterMu.Unlock()
	})
	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-test","usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n\n")
	run := func(b *testing.B, protected bool) {
		const maxSamples = 4096
		sampleEvery := b.N / maxSamples
		if sampleEvery < 1 {
			sampleEvery = 1
		}
		samples := make([]int64, 0, min(b.N, maxSamples))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recorder := &claudeIntegrityTimingRecorder{ResponseRecorder: httptest.NewRecorder()}
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			start := time.Now()
			info := &relaycommon.RelayInfo{
				StartTime:                                start,
				IsStream:                                 true,
				RelayFormat:                              types.RelayFormatClaude,
				OriginModelName:                          "claude-test",
				ChannelMeta:                              &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
				ClaudeResponseIntegrityEnabled:           protected,
				ClaudeResponseIntegrityFirstBlockTimeout: time.Second,
				Request:                                  &dto.ClaudeRequest{MaxTokens: common.GetPointer[uint](32)},
			}
			recorder.startedAt = time.Now()
			resp := claudeIntegrityHTTPResponse(strings.NewReader(body), true)
			if protected {
				info.MarkFinalRequestRelayFormat(types.RelayFormatClaude)
				info.BeginClaudeResponseIntegrityAttempt(c.Request.Context())
				_, _ = ClaudeIntegrityStreamHandler(c, resp, info)
			} else {
				_, _ = ClaudeStreamHandler(c, resp, info)
			}
			if recorder.firstWrite <= 0 {
				b.Fatal("stream handler did not write a first response")
			}
			if i%sampleEvery == 0 && len(samples) < maxSamples {
				samples = append(samples, recorder.firstWrite.Nanoseconds())
			}
		}
		b.StopTimer()
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		p95Index := (len(samples)*95 + 99) / 100
		if p95Index > 0 {
			p95Index--
		}
		b.ReportMetric(float64(samples[p95Index]), "first-block-p95-ns")
	}
	b.Run("legacy_switch_off", func(b *testing.B) { run(b, false) })
	b.Run("integrity_switch_on", func(b *testing.B) { run(b, true) })
}

func TestClaudeIntegrityStateAcceptsKnownAndFutureBlockTypes(t *testing.T) {
	for _, blockType := range []string{"text", "tool_use", "thinking", "refusal", "future_block"} {
		t.Run(blockType, func(t *testing.T) {
			state := newClaudeIntegrityState(false)
			index := 0
			observation, err := state.observe(&dto.ClaudeResponse{
				Type:         "content_block_start",
				Index:        &index,
				ContentBlock: &dto.ClaudeMediaMessage{Type: blockType},
			})
			require.NoError(t, err)
			require.True(t, observation.firstBlock)
			_, err = state.observe(&dto.ClaudeResponse{Type: "content_block_stop", Index: &index})
			require.NoError(t, err)
			observation, err = state.observe(&dto.ClaudeResponse{Type: "message_stop"})
			require.NoError(t, err)
			require.True(t, observation.terminal)
		})
	}
}

func TestClaudeIntegrityStateRejectsBlockWithoutType(t *testing.T) {
	state := newClaudeIntegrityState(false)
	index := 0
	_, err := state.observe(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        &index,
		ContentBlock: &dto.ClaudeMediaMessage{},
	})
	require.ErrorContains(t, err, "no block type")
}

func TestClaudeIntegrityStateRejectsMissingEventAndDeltaTypes(t *testing.T) {
	state := newClaudeIntegrityState(false)
	_, err := state.observe(&dto.ClaudeResponse{})
	require.ErrorContains(t, err, "event has no type")

	index := 0
	_, err = state.observe(&dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        &index,
		ContentBlock: &dto.ClaudeMediaMessage{Type: "future_block"},
	})
	require.NoError(t, err)
	_, err = state.observe(&dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &index,
		Delta: &dto.ClaudeMediaMessage{},
	})
	require.ErrorContains(t, err, "no delta type")
}
