package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpenAIResponseDiagnosticSnapshots(t *testing.T) {
	t.Helper()
	testRoot := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(testRoot, "snapshots.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ErrorSnapshot{}))
	oldLogDB := model.LOG_DB
	model.LOG_DB = db

	t.Setenv("ERROR_SNAPSHOT_DIR", filepath.Join(testRoot, "snapshots"))
	cfg := config.GlobalConfig.Get("error_snapshot")
	original, err := config.ConfigToMap(cfg)
	require.NoError(t, err)
	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"enabled":              "true",
		"ttl_minutes":          "5",
		"max_storage_mib":      "16",
		"max_files":            "20",
		"priority_user_ids":    "",
		"priority_channel_ids": "",
	}))
	error_snapshot_setting.RefreshSnapshot()
	service.StartErrorSnapshotManager()
	t.Cleanup(func() {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		require.NoError(t, config.UpdateConfigFromMap(cfg, original))
		error_snapshot_setting.RefreshSnapshot()
		model.LOG_DB = oldLogDB
	})
}

func newOpenAIResponseDiagnosticContext(path, requestBody string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-openai-response-diagnostic")
	c.Set("id", 23)
	c.Set("username", "diagnostic-user")
	c.Set("channel_id", 41)
	c.Set("channel_name", "diagnostic-channel")
	c.Set("channel_type", 1)
	c.Set("original_model", "gpt-test")
	service.BeginErrorSnapshotAttempt(c, 0)
	service.MarkErrorSnapshotChannelSelected(c)
	service.CaptureRelayDiagnosticUpstreamRequestIfNeeded(c, []byte(requestBody))
	return c, recorder
}

func openAIResponseDiagnosticInfo(stream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		IsStream:    stream,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}
}

func responseDiagnosticHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func requireSingleOpenAIResponseDiagnosticSnapshot(t *testing.T, errorCode string) (map[string]any, *model.ErrorSnapshot) {
	t.Helper()
	var snapshots []*model.ErrorSnapshot
	require.Eventually(t, func() bool {
		var err error
		snapshots, _, err = service.ListErrorSnapshots(model.ErrorSnapshotQuery{Limit: 20})
		return err == nil && len(snapshots) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, errorCode, snapshots[0].ErrorCode)
	payload, err := service.ReadErrorSnapshot(snapshots[0].ID)
	require.NoError(t, err)
	return payload, snapshots[0]
}

func requireNoOpenAIResponseDiagnosticSnapshots(t *testing.T) {
	t.Helper()
	require.Never(t, func() bool {
		_, total, err := service.ListErrorSnapshots(model.ErrorSnapshotQuery{Limit: 20})
		return err == nil && total != 0
	}, 150*time.Millisecond, 10*time.Millisecond)
}

func TestOpenAIHandlersPersistResponseAnomalyEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupOpenAIResponseDiagnosticSnapshots(t)

	t.Run("chat empty output with one token", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		requestBody := `{"model":"gpt-test","messages":[{"role":"user","content":"why empty"}]}`
		c, _ := newOpenAIResponseDiagnosticContext("/v1/chat/completions", requestBody)
		responseBody := `{"id":"chatcmpl_test","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`

		usage, apiErr := OpenaiHandler(c, openAIResponseDiagnosticInfo(false), responseDiagnosticHTTPResponse(responseBody))

		require.Nil(t, apiErr)
		require.Equal(t, 1, usage.CompletionTokens)
		payload, snapshot := requireSingleOpenAIResponseDiagnosticSnapshot(t, service.RelayResponseAnomalyOutputTokensOne)
		require.Equal(t, model.ErrorSnapshotCaptureLevelDiagnostic, snapshot.CaptureLevel)
		require.NotNil(t, payload["client_request"])
		require.NotNil(t, payload["upstream_request"])
		require.NotNil(t, payload["upstream_response"])
		require.NotNil(t, payload["downstream_response"])
	})

	t.Run("responses delta disagrees with terminal output", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		requestBody := `{"model":"gpt-test","input":"test mismatch","stream":true}`
		c, _ := newOpenAIResponseDiagnosticContext("/v1/responses", requestBody)
		streamBody := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial answer\"}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"total_tokens\":12}}}\n\n"

		usage, apiErr := OaiResponsesStreamHandler(c, openAIResponseDiagnosticInfo(true), responseDiagnosticHTTPResponse(streamBody))

		require.Nil(t, apiErr)
		require.Equal(t, 12, usage.TotalTokens)
		payload, _ := requireSingleOpenAIResponseDiagnosticSnapshot(t, service.RelayResponseAnomalyTerminalOutputMismatch)
		stream := payload["stream"].(map[string]any)
		require.Equal(t, "openai_responses", stream["protocol"])
		require.NotNil(t, stream["upstream"])
		require.NotNil(t, stream["downstream"])
	})

	t.Run("chat stream missing terminal", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		requestBody := `{"model":"gpt-test","messages":[{"role":"user","content":"test eof"}],"stream":true}`
		c, _ := newOpenAIResponseDiagnosticContext("/v1/chat/completions", requestBody)
		streamBody := "data: {\"id\":\"chatcmpl_test\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"

		_, apiErr := OaiStreamHandler(c, openAIResponseDiagnosticInfo(true), responseDiagnosticHTTPResponse(streamBody))

		require.Nil(t, apiErr)
		payload, _ := requireSingleOpenAIResponseDiagnosticSnapshot(t, service.RelayResponseAnomalyTerminalMissing)
		response := payload["response"].(map[string]any)
		require.Equal(t, false, response["terminal_seen"])
	})

	t.Run("explicit stream error uses one ordinary failure snapshot", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		requestBody := `{"model":"gpt-test","messages":[{"role":"user","content":"test failure"}],"stream":true}`
		c, _ := newOpenAIResponseDiagnosticContext("/v1/chat/completions", requestBody)
		streamBody := "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"status\":\"failed\",\"model\":\"gpt-test\",\"error\":{\"message\":\"provider failed\",\"type\":\"server_error\",\"code\":\"upstream_failed\"}}}\n\n"

		_, apiErr := OaiResponsesToChatStreamHandler(c, openAIResponseDiagnosticInfo(true), responseDiagnosticHTTPResponse(streamBody))

		require.NotNil(t, apiErr)
		service.CaptureRelayErrorSnapshot(c, apiErr, false)
		var snapshots []*model.ErrorSnapshot
		require.Eventually(t, func() bool {
			var err error
			snapshots, _, err = service.ListErrorSnapshots(model.ErrorSnapshotQuery{Limit: 20})
			return err == nil && len(snapshots) == 1
		}, 2*time.Second, 10*time.Millisecond)
		require.NotEqual(t, "relay_response_anomaly", snapshots[0].ErrorType)
		require.Equal(t, model.ErrorSnapshotCaptureLevelDiagnostic, snapshots[0].CaptureLevel)
		require.NotEqual(t, service.ErrorSnapshotOutcomeSuspiciousSuccess, snapshots[0].FinalOutcome)
		payload, err := service.ReadErrorSnapshot(snapshots[0].ID)
		require.NoError(t, err)
		require.NotNil(t, payload["client_request"])
		require.NotNil(t, payload["upstream_request"])
		stream := payload["stream"].(map[string]any)
		require.Equal(t, "openai_responses", stream["protocol"])
		require.NotNil(t, stream["upstream"])
	})

	t.Run("non-stream response error uses one full failure snapshot", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		requestBody := `{"model":"gpt-test","input":"test failure"}`
		c, _ := newOpenAIResponseDiagnosticContext("/v1/responses", requestBody)
		responseBody := `{"id":"resp_failed","status":"failed","model":"gpt-test","output":[],"error":{"message":"provider failed","type":"server_error","code":"upstream_failed"}}`

		_, apiErr := OaiResponsesHandler(c, openAIResponseDiagnosticInfo(false), responseDiagnosticHTTPResponse(responseBody))

		require.NotNil(t, apiErr)
		service.CaptureRelayErrorSnapshot(c, apiErr, false)
		var snapshots []*model.ErrorSnapshot
		require.Eventually(t, func() bool {
			var err error
			snapshots, _, err = service.ListErrorSnapshots(model.ErrorSnapshotQuery{Limit: 20})
			return err == nil && len(snapshots) == 1
		}, 2*time.Second, 10*time.Millisecond)
		require.NotEqual(t, "relay_response_anomaly", snapshots[0].ErrorType)
		require.Equal(t, model.ErrorSnapshotCaptureLevelDiagnostic, snapshots[0].CaptureLevel)
		payload, err := service.ReadErrorSnapshot(snapshots[0].ID)
		require.NoError(t, err)
		require.NotNil(t, payload["client_request"])
		require.NotNil(t, payload["upstream_request"])
		require.NotNil(t, payload["upstream_response"])
	})

	t.Run("normal text and tool only are not captured", func(t *testing.T) {
		cases := []string{
			`{"id":"chatcmpl_text","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"normal answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			`{"id":"chatcmpl_tool","model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		}
		for _, responseBody := range cases {
			require.NoError(t, service.DeleteAllErrorSnapshots())
			c, _ := newOpenAIResponseDiagnosticContext("/v1/chat/completions", `{"model":"gpt-test"}`)
			_, apiErr := OpenaiHandler(c, openAIResponseDiagnosticInfo(false), responseDiagnosticHTTPResponse(responseBody))
			require.Nil(t, apiErr)
			requireNoOpenAIResponseDiagnosticSnapshots(t)
		}
	})
}
