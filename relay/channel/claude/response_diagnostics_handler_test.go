package claude

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

func setupClaudeResponseDiagnosticSnapshots(t *testing.T) {
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

func newClaudeResponseDiagnosticContext() (*gin.Context, *httptest.ResponseRecorder) {
	requestBody := `{"model":"claude-test","messages":[{"role":"user","content":"why empty"}],"stream":true}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(requestBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-claude-response-diagnostic")
	c.Set("id", 24)
	c.Set("username", "diagnostic-user")
	c.Set("channel_id", 42)
	c.Set("channel_name", "claude-diagnostic-channel")
	c.Set("channel_type", 14)
	c.Set("original_model", "claude-test")
	service.BeginErrorSnapshotAttempt(c, 0)
	service.MarkErrorSnapshotChannelSelected(c)
	service.CaptureRelayDiagnosticUpstreamRequestIfNeeded(c, []byte(requestBody))
	return c, recorder
}

func claudeResponseDiagnosticInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-test",
		},
	}
}

func claudeResponseDiagnosticHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func requireSingleClaudeResponseDiagnosticSnapshot(t *testing.T, errorCode string) map[string]any {
	t.Helper()
	var snapshots []*model.ErrorSnapshot
	require.Eventually(t, func() bool {
		var err error
		snapshots, _, err = service.ListErrorSnapshots(model.ErrorSnapshotQuery{Limit: 20})
		return err == nil && len(snapshots) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, errorCode, snapshots[0].ErrorCode)
	require.Equal(t, model.ErrorSnapshotCaptureLevelDiagnostic, snapshots[0].CaptureLevel)
	payload, err := service.ReadErrorSnapshot(snapshots[0].ID)
	require.NoError(t, err)
	return payload
}

func TestClaudeStreamHandlerPersistsResponseAnomalyEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupClaudeResponseDiagnosticSnapshots(t)

	t.Run("empty end turn with one output token", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		c, _ := newClaudeResponseDiagnosticContext()
		streamBody := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"model\":\"claude-test\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
			"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n" +
			"data: {\"type\":\"message_stop\"}\n\n"

		usage, apiErr := ClaudeStreamHandler(c, claudeResponseDiagnosticHTTPResponse(streamBody), claudeResponseDiagnosticInfo())

		require.Nil(t, apiErr)
		require.Equal(t, 10, usage.PromptTokens)
		require.Equal(t, 1, usage.CompletionTokens)
		payload := requireSingleClaudeResponseDiagnosticSnapshot(t, service.RelayResponseAnomalyOutputTokensOne)
		response := payload["response"].(map[string]any)
		require.Equal(t, true, response["terminal_seen"])
		require.Equal(t, "upstream", response["usage_source"])
		require.NotNil(t, response["raw_usage"])
		require.NotNil(t, response["effective_usage"])
		require.NotNil(t, payload["stream"])
	})

	t.Run("missing message stop", func(t *testing.T) {
		require.NoError(t, service.DeleteAllErrorSnapshots())
		c, _ := newClaudeResponseDiagnosticContext()
		streamBody := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"model\":\"claude-test\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"partial answer\"}}\n\n" +
			"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"

		_, apiErr := ClaudeStreamHandler(c, claudeResponseDiagnosticHTTPResponse(streamBody), claudeResponseDiagnosticInfo())

		require.Nil(t, apiErr)
		payload := requireSingleClaudeResponseDiagnosticSnapshot(t, service.RelayResponseAnomalyTerminalMissing)
		response := payload["response"].(map[string]any)
		require.Equal(t, false, response["terminal_seen"])
	})
}
