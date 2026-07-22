package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newFunctionNameCompatInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-test",
		},
		OpenAIReservedFunctionNameRestores: map[string]string{"run_python": "python"},
	}
}

func TestOpenaiHandlerRestoresReservedFunctionName(t *testing.T) {
	c, recorder := newCacheWriteUsageTestContext("/v1/chat/completions")
	info := newFunctionNameCompatInfo()
	body := `{"id":"chatcmpl_test","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"run_python","tool_calls":[{"id":"call_1","type":"function","function":{"name":"run_python","arguments":"{\"name\":\"run_python\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := OpenaiHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.Equal(t, 15, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"name":"python"`)
	require.Contains(t, recorder.Body.String(), `"content":"run_python"`)
	require.Contains(t, recorder.Body.String(), `\"name\":\"run_python\"`)
}

func TestSendStreamDataRestoresReservedFunctionName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := newFunctionNameCompatInfo()
	chunk := `{"id":"chatcmpl_test","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"run_python","tool_calls":[{"index":0,"function":{"name":"run_python","arguments":"run_python"}}]},"finish_reason":null}]}`

	err := sendStreamData(c, info, chunk, false, false)

	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"name":"python"`)
	require.Contains(t, recorder.Body.String(), `"content":"run_python"`)
	require.Contains(t, recorder.Body.String(), `"arguments":"run_python"`)
}
