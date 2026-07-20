package service

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupErrorSnapshotTest(t *testing.T) string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ErrorSnapshot{}))
	oldLogDB := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = oldLogDB })

	root := filepath.Join(t.TempDir(), "snapshots")
	t.Setenv("ERROR_SNAPSHOT_DIR", root)
	cfg := config.GlobalConfig.Get("error_snapshot")
	original, err := config.ConfigToMap(cfg)
	require.NoError(t, err)
	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"enabled":              "true",
		"ttl_minutes":          "5",
		"max_storage_mib":      "16",
		"max_files":            "10",
		"priority_user_ids":    "7",
		"priority_channel_ids": "11",
	}))
	error_snapshot_setting.RefreshSnapshot()
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(cfg, original))
		error_snapshot_setting.RefreshSnapshot()
	})
	return root
}

func TestErrorSnapshotWriteReadPermissionsAndCleanup(t *testing.T) {
	root := setupErrorSnapshotTest(t)
	payload := []byte(`{"schema_version":1,"snapshot_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","secret":"masked"}`)
	work := errorSnapshotWork{
		kind: errorSnapshotWorkCapture,
		index: model.ErrorSnapshot{
			ID:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			CreatedAt:    time.Now().Unix(),
			RequestID:    "req-write",
			FinalOutcome: ErrorSnapshotOutcomePending,
			OriginalSize: int64(len(payload)),
		},
		payload: payload,
	}
	require.NoError(t, writeErrorSnapshot(work))

	row, err := model.GetErrorSnapshot(work.index.ID)
	require.NoError(t, err)
	path := filepath.Join(root, row.RelativePath)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	compressed, err := ReadCompressedErrorSnapshot(work.index.ID)
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, payload, decoded)

	require.NoError(t, model.LOG_DB.Model(&model.ErrorSnapshot{}).Where("id = ?", work.index.ID).Update("created_at", time.Now().Add(-10*time.Minute).Unix()).Error)
	require.NoError(t, CleanupErrorSnapshots())
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
	_, err = model.GetErrorSnapshot(work.index.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestBuildErrorSnapshotWorkPriorityAndSummary(t *testing.T) {
	setupErrorSnapshotTest(t)
	body := `{"model":"claude-test","messages":[{"role":"user","content":"hello"}],"api_key":"secret-value","channel_key":"channel-secret","apikey":"compact-secret","max_tokens":32}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer very-secret")
	c.Set(common.RequestIdKey, "req-priority")
	c.Set("id", 7)
	c.Set("username", "alice")
	c.Set("channel_id", 11)
	c.Set("channel_name", "provider")
	c.Set("channel_type", 14)
	c.Set("original_model", "claude-test")

	BeginErrorSnapshotAttempt(c, 2)
	MarkErrorSnapshotChannelSelected(c)
	CaptureErrorSnapshotUpstreamRequestIfNeeded(c, []byte(`{"model":"upstream","access_token":"secret"}`))
	apiErr := types.NewErrorWithStatusCode(io.ErrUnexpectedEOF, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	apiErr.Diagnostic = &types.RelayErrorDiagnostic{UpstreamResponseBody: []byte(`{"error":{"message":"bad"},"api_key":"secret"}`)}
	work, err := buildErrorSnapshotWork(c, apiErr, true)
	require.NoError(t, err)
	require.Equal(t, model.ErrorSnapshotCaptureLevelPriority, work.index.CaptureLevel)
	require.Equal(t, 2, work.index.RetryIndex)
	require.NotContains(t, string(work.payload), "very-secret")
	require.NotContains(t, string(work.payload), "secret-value")
	require.NotContains(t, string(work.payload), "channel-secret")
	require.NotContains(t, string(work.payload), "compact-secret")
	var priorityEnvelope errorSnapshotEnvelope
	require.NoError(t, common.Unmarshal(work.payload, &priorityEnvelope))
	require.NotNil(t, priorityEnvelope.ClientRequest)
	require.Contains(t, priorityEnvelope.ClientRequest.Body, `"max_tokens":32`)
	require.NotContains(t, string(work.payload), `"access_token":"secret"`)
	require.Contains(t, string(work.payload), "client_request")
	require.Contains(t, string(work.payload), "upstream_request")
	require.Contains(t, string(work.payload), "upstream_response")

	c.Set("id", 8)
	c.Set("channel_id", 12)
	BeginErrorSnapshotAttempt(c, 0)
	MarkErrorSnapshotChannelSelected(c)
	work, err = buildErrorSnapshotWork(c, apiErr, false)
	require.NoError(t, err)
	require.Equal(t, model.ErrorSnapshotCaptureLevelSummary, work.index.CaptureLevel)
	require.NotContains(t, string(work.payload), "client_request")
	require.NotContains(t, string(work.payload), "upstream_request")
}

func TestErrorSnapshotUnsupportedClientBodyStoresMetadataOnly(t *testing.T) {
	setupErrorSnapshotTest(t)
	body := []byte("binary multipart payload")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(body))
	contentType := "multipart/form-data; boundary=snapshot-test"
	c.Request.Header.Set("Content-Type", contentType)

	fragment := captureClientRequestBody(c, contentType)
	require.Equal(t, contentType, fragment.ContentType)
	require.EqualValues(t, len(body), fragment.OriginalSize)
	require.Equal(t, hashErrorSnapshotBytes(body), fragment.SHA256)
	require.Equal(t, "unsupported_content_type", fragment.SkipReason)
	require.Empty(t, fragment.Body)

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	position, err := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.EqualValues(t, 0, position)
	common.CleanupBodyStorage(c)
}

func TestErrorSnapshotTextMasksSecretAssignments(t *testing.T) {
	input := `upstream rejected {"channel_key":"channel-secret","access_token":"token-secret"}`
	masked := sanitizeErrorSnapshotText(input)
	require.NotContains(t, masked, "channel-secret")
	require.NotContains(t, masked, "token-secret")
	require.Contains(t, masked, `"channel_key":"***"`)
	require.Contains(t, masked, `"access_token":"***"`)
}

func TestErrorSnapshotPayloadBoundAndSafePath(t *testing.T) {
	setupErrorSnapshotTest(t)
	payload := bytes.Repeat([]byte("a"), errorSnapshotMaxPayloadBytes*2)
	bounded, originalSize, truncated, err := boundErrorSnapshotPayload("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", payload)
	require.NoError(t, err)
	require.True(t, truncated)
	require.EqualValues(t, len(payload), originalSize)
	require.LessOrEqual(t, len(bounded), errorSnapshotMaxPayloadBytes)

	_, err = safeErrorSnapshotPath("../escape.json.gz")
	require.Error(t, err)
	_, err = getValidatedErrorSnapshot("../../etc/passwd")
	require.Error(t, err)
}

func TestErrorSnapshotBase64BodiesAreReplacedWithMetadata(t *testing.T) {
	setupErrorSnapshotTest(t)
	encoded := strings.Repeat("QUJD", 100)
	body, err := common.Marshal(map[string]any{
		"model":        "image-test",
		"prompt":       "keep this text",
		"image":        "data:image/png;base64," + encoded,
		"audio_base64": encoded,
	})
	require.NoError(t, err)

	fragment := buildErrorSnapshotBodyFragment(body, int64(len(body)), "application/json", errorSnapshotMaxBodyFragment)
	require.Empty(t, fragment.SkipReason)
	require.NotContains(t, fragment.Body, encoded)
	require.Contains(t, fragment.Body, "base64_content")
	require.Contains(t, fragment.Body, "image/png")
	require.Contains(t, fragment.Body, "estimated_decoded_size")
	require.Contains(t, fragment.Body, "keep this text")
}

func TestErrorSnapshotLargeClientBodyStoresOnlyBoundedMetadata(t *testing.T) {
	setupErrorSnapshotTest(t)
	body := bytes.Repeat([]byte("x"), errorSnapshotMaxBodyFragment+1)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	fragment := captureClientRequestBody(c, "application/json")
	require.EqualValues(t, len(body), fragment.OriginalSize)
	require.Equal(t, hashErrorSnapshotBytes(body), fragment.SHA256)
	require.True(t, fragment.Truncated)
	require.Equal(t, "body_too_large", fragment.SkipReason)
	require.Empty(t, fragment.Body)

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	position, err := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.EqualValues(t, 0, position)
	common.CleanupBodyStorage(c)
}

func createIndexedErrorSnapshotFile(t *testing.T, root string, createdAt, compressedSize int64) *model.ErrorSnapshot {
	t.Helper()
	id := common.GetUUID()
	relativePath := filepath.Join("20260720", id+".json.gz")
	path := filepath.Join(root, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("snapshot"), 0o600))
	snapshot := &model.ErrorSnapshot{
		ID:             id,
		CreatedAt:      createdAt,
		RequestID:      "req-" + id,
		RelativePath:   relativePath,
		CompressedSize: compressedSize,
		FinalOutcome:   ErrorSnapshotOutcomeFinalFailure,
	}
	require.NoError(t, model.CreateErrorSnapshot(snapshot))
	return snapshot
}

func TestErrorSnapshotCleanupEnforcesFileAndStorageLimitsOldestFirst(t *testing.T) {
	root := setupErrorSnapshotTest(t)
	now := time.Now().Unix()
	var snapshots []*model.ErrorSnapshot
	for i := 0; i < 11; i++ {
		snapshots = append(snapshots, createIndexedErrorSnapshotFile(t, root, now-int64(11-i), 1))
	}

	require.NoError(t, CleanupErrorSnapshots())
	remaining, err := model.ListErrorSnapshotsForCleanup()
	require.NoError(t, err)
	require.Len(t, remaining, 10)
	_, err = model.GetErrorSnapshot(snapshots[0].ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = os.Stat(filepath.Join(root, snapshots[0].RelativePath))
	require.True(t, os.IsNotExist(err))

	require.NoError(t, DeleteAllErrorSnapshots())
	older := createIndexedErrorSnapshotFile(t, root, now-2, 10<<20)
	newer := createIndexedErrorSnapshotFile(t, root, now-1, 10<<20)
	require.NoError(t, CleanupErrorSnapshots())
	remaining, err = model.ListErrorSnapshotsForCleanup()
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, newer.ID, remaining[0].ID)
	_, err = model.GetErrorSnapshot(older.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestErrorSnapshotQueueFullReturnsWithoutBlocking(t *testing.T) {
	setupErrorSnapshotTest(t)
	oldQueue := errorSnapshotManager.queue
	oldDropped := errorSnapshotManager.dropped.Load()
	errorSnapshotManager.queue = make(chan errorSnapshotWork, 1)
	t.Cleanup(func() {
		errorSnapshotManager.queue = oldQueue
		errorSnapshotManager.dropped.Store(oldDropped)
	})

	require.True(t, enqueueErrorSnapshotWork(errorSnapshotWork{kind: errorSnapshotWorkCapture}))
	startedAt := time.Now()
	require.False(t, enqueueErrorSnapshotWork(errorSnapshotWork{kind: errorSnapshotWorkCapture}))
	require.Less(t, time.Since(startedAt), 50*time.Millisecond)
	require.Equal(t, oldDropped+1, errorSnapshotManager.dropped.Load())
}

func TestErrorSnapshotFallbackOutcomeIsQueuedAfterFailedAttempt(t *testing.T) {
	setupErrorSnapshotTest(t)
	oldQueue := errorSnapshotManager.queue
	errorSnapshotManager.queue = make(chan errorSnapshotWork, 4)
	t.Cleanup(func() { errorSnapshotManager.queue = oldQueue })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	c.Set(common.RequestIdKey, "req-fallback-order")
	c.Set("id", 8)
	c.Set("username", "fallback-user")
	c.Set("original_model", "claude-test")
	BeginErrorSnapshotAttempt(c, 1)
	apiErr := types.NewErrorWithStatusCode(io.ErrUnexpectedEOF, types.ErrorCodeBadResponseBody, http.StatusBadGateway)

	CaptureRelayErrorSnapshot(c, apiErr, false)
	FinalizeErrorSnapshotRequest(c, nil)

	failedAttempt := <-errorSnapshotManager.queue
	finalOutcome := <-errorSnapshotManager.queue
	require.Equal(t, errorSnapshotWorkCapture, failedAttempt.kind)
	require.Equal(t, "req-fallback-order", failedAttempt.index.RequestID)
	require.Equal(t, errorSnapshotWorkOutcome, finalOutcome.kind)
	require.Equal(t, "req-fallback-order", finalOutcome.requestID)
	require.Equal(t, ErrorSnapshotOutcomeFallbackSucceeded, finalOutcome.outcome)
}
