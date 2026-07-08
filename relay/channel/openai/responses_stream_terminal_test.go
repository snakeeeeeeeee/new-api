package openai

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func setupOpenAIStreamTerminalTest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *io.PipeReader, *io.PipeWriter) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("id", 123)
	c.Set("token_id", 456)
	c.Set("token_name", "test-codex")
	c.Set("original_model", "gpt-test")

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

func TestOaiResponsesStreamHandlerDumpsResponsesStreamTrace(t *testing.T) {
	service.StopRequestDump()
	service.ClearRequestDumpEvents()
	t.Cleanup(func() {
		service.StopRequestDump()
		service.ClearRequestDumpEvents()
	})

	_, err := service.StartRequestDump(service.RequestDumpRule{
		UserIDs:                   []int{123},
		TokenNames:                []string{"test-codex"},
		Models:                    []string{"gpt-test"},
		Paths:                     []string{"/v1/responses"},
		DurationSeconds:           60,
		MaxCount:                  10,
		PrintOn:                   service.RequestDumpPrintOnAll,
		TraceResponsesStream:      true,
		MaxStreamEventsPerRequest: 10,
	})
	require.NoError(t, err)

	c, _, info, pr, pw := setupOpenAIStreamTerminalTest(t)
	resp := &http.Response{Body: pr}

	done := make(chan error, 1)
	go func() {
		_, apiErr := OaiResponsesStreamHandler(c, info, resp)
		if apiErr != nil {
			done <- apiErr
			return
		}
		done <- nil
	}()

	writeResponsesCompletedEvent(t, pw)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		_ = pr.Close()
		_ = pw.Close()
		t.Fatal("OaiResponsesStreamHandler did not return after response.completed")
	}

	events := service.GetRequestDumpEvents(0, 10)
	require.Len(t, events, 2)
	require.Equal(t, service.RequestDumpStageResponsesStreamEvent, events[0].Stage)
	require.Equal(t, "response.completed", events[0].StreamEventType)
	require.Equal(t, service.RequestDumpStageResponsesStreamSummary, events[1].Stage)
	require.Equal(t, "response.completed", events[1].StreamStopReason)
	require.Equal(t, 1, events[1].StreamReceivedCount)
}

func TestBuildResponsesStreamDumpMetaIncludesToolDetails(t *testing.T) {
	data := `{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"tool_search","arguments":"{\"query\":\"chrome tabs\"}"}}`
	var streamResp dto.ResponsesStreamResponse
	require.NoError(t, common.UnmarshalJsonStr(data, &streamResp))

	meta := buildResponsesStreamDumpMeta(data, streamResp)

	require.Equal(t, "response.output_item.done", meta.EventType)
	require.Equal(t, "function_call", meta.ItemType)
	require.Equal(t, "fc_1", meta.ItemID)
	require.Equal(t, "call_1", meta.CallID)
	require.Equal(t, "tool_search", meta.ToolName)
	require.Equal(t, `{"query":"chrome tabs"}`, meta.Arguments)
	require.Equal(t, len(`{"query":"chrome tabs"}`), meta.ArgumentsSize)
	require.Equal(t, "item.arguments", meta.Details["arguments_source"])
}

func TestBuildResponsesStreamDumpMetaIncludesCustomToolInput(t *testing.T) {
	data := `{"type":"response.output_item.added","item":{"type":"custom_tool_call","id":"ctc_1","call_id":"call_c","name":"mcp__chrome__tabs","input":{"action":"list"}}}`
	var streamResp dto.ResponsesStreamResponse
	require.NoError(t, common.UnmarshalJsonStr(data, &streamResp))

	meta := buildResponsesStreamDumpMeta(data, streamResp)

	require.Equal(t, "custom_tool_call", meta.ItemType)
	require.Equal(t, "ctc_1", meta.ItemID)
	require.Equal(t, "call_c", meta.CallID)
	require.Equal(t, "mcp__chrome__tabs", meta.ToolName)
	require.Equal(t, `{"action":"list"}`, meta.Arguments)
	require.Equal(t, len(`{"action":"list"}`), meta.ArgumentsSize)
	require.Equal(t, "item.input", meta.Details["arguments_source"])
}
