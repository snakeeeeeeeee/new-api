package openai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupOpenAIStreamTerminalTest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *io.PipeReader, *io.PipeWriter) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}

	return c, recorder, info, pr, pw
}

func writeResponsesCompletedEvent(t *testing.T, pw *io.PipeWriter) {
	t.Helper()

	event := `data: {"type":"response.completed","response":{"id":"resp_test","created_at":123,"model":"gpt-test","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}}` + "\n"
	writeDone := make(chan error, 1)
	go func() {
		_, err := fmt.Fprint(pw, event)
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		_ = pw.Close()
		t.Fatal("timed out writing response.completed event")
	}
}

func TestOaiResponsesStreamHandlerCompletedOpenUpstreamReturns(t *testing.T) {
	c, recorder, info, pr, pw := setupOpenAIStreamTerminalTest(t)
	resp := &http.Response{Body: pr}

	type result struct {
		totalTokens int
		err         error
	}
	done := make(chan result, 1)
	go func() {
		usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
		res := result{}
		if usage != nil {
			res.totalTokens = usage.TotalTokens
		}
		if apiErr != nil {
			res.err = apiErr
		}
		done <- res
	}()

	writeResponsesCompletedEvent(t, pw)

	select {
	case res := <-done:
		require.NoError(t, res.err)
		require.Equal(t, 7, res.totalTokens)
		require.Contains(t, recorder.Body.String(), "response.completed")
	case <-time.After(500 * time.Millisecond):
		_ = pr.Close()
		_ = pw.Close()
		t.Fatal("OaiResponsesStreamHandler did not return after response.completed")
	}
}

func TestOaiResponsesToChatStreamHandlerCompletedOpenUpstreamReturns(t *testing.T) {
	c, recorder, info, pr, pw := setupOpenAIStreamTerminalTest(t)
	resp := &http.Response{Body: pr}

	type result struct {
		totalTokens int
		err         error
	}
	done := make(chan result, 1)
	go func() {
		usage, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
		res := result{}
		if usage != nil {
			res.totalTokens = usage.TotalTokens
		}
		if apiErr != nil {
			res.err = apiErr
		}
		done <- res
	}()

	writeResponsesCompletedEvent(t, pw)

	select {
	case res := <-done:
		require.NoError(t, res.err)
		require.Equal(t, 7, res.totalTokens)
		body := recorder.Body.String()
		require.Contains(t, body, `"finish_reason":"stop"`)
		require.True(t, strings.Contains(body, "data: [DONE]"), "expected final [DONE], got %q", body)
	case <-time.After(500 * time.Millisecond):
		_ = pr.Close()
		_ = pw.Close()
		t.Fatal("OaiResponsesToChatStreamHandler did not return after response.completed")
	}
}
