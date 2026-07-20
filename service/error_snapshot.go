package service

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	errorSnapshotMaxPayloadBytes     = 128 << 10
	errorSnapshotMaxBodyFragment     = 32 << 10
	errorSnapshotMaxResponseFragment = 64 << 10
	errorSnapshotQueueSize           = 32
	errorSnapshotCleanupInterval     = 5 * time.Minute
)

var errorSnapshotSecretAssignmentPattern = regexp.MustCompile(`(?i)((?:authorization|proxy[-_ ]?authorization|cookie|set[-_ ]?cookie|api[-_ ]?key|apikey|x[-_ ]?api[-_ ]?key|access[-_ ]?token|refresh[-_ ]?token|client[-_ ]?secret|channel[-_ ]?key|password|credential|secret)["']?\s*[:=]\s*["']?)[^"',}\]\r\n]+`)

const (
	errorSnapshotRetryIndexKey      = "_error_snapshot_retry_index"
	errorSnapshotUpstreamRequestKey = "_error_snapshot_upstream_request"
	errorSnapshotChannelSelectedKey = "_error_snapshot_channel_selected"
	errorSnapshotCurrentCapturedKey = "_error_snapshot_current_captured"
	errorSnapshotAnyCapturedKey     = "_error_snapshot_any_captured"
	errorSnapshotTerminalOutcomeKey = "_error_snapshot_terminal_outcome"
)

const (
	ErrorSnapshotOutcomePending            = "pending"
	ErrorSnapshotOutcomeFallbackSucceeded  = "fallback_succeeded"
	ErrorSnapshotOutcomeFinalFailure       = "final_failure"
	ErrorSnapshotOutcomeStreamIncomplete   = "stream_incomplete"
	ErrorSnapshotOutcomeClientDisconnected = "client_disconnected"
)

type ErrorSnapshotSettingsView struct {
	Enabled            bool  `json:"enabled"`
	TTLMinutes         int   `json:"ttl_minutes"`
	MaxStorageMiB      int   `json:"max_storage_mib"`
	MaxFiles           int   `json:"max_files"`
	PriorityUserIDs    []int `json:"priority_user_ids"`
	PriorityChannelIDs []int `json:"priority_channel_ids"`
}

type ErrorSnapshotSettingsUpdate struct {
	Enabled            bool  `json:"enabled"`
	TTLMinutes         int   `json:"ttl_minutes"`
	MaxStorageMiB      int   `json:"max_storage_mib"`
	MaxFiles           int   `json:"max_files"`
	PriorityUserIDs    []int `json:"priority_user_ids"`
	PriorityChannelIDs []int `json:"priority_channel_ids"`
}

type ErrorSnapshotStatus struct {
	Settings      ErrorSnapshotSettingsView       `json:"settings"`
	StoragePath   string                          `json:"storage_path"`
	Storage       model.ErrorSnapshotStorageStats `json:"storage"`
	DroppedCount  int64                           `json:"dropped_count"`
	WriteErrors   int64                           `json:"write_errors"`
	LastCleanupAt int64                           `json:"last_cleanup_at"`
	LastError     string                          `json:"last_error,omitempty"`
}

type errorSnapshotWorkKind int

const (
	errorSnapshotWorkCapture errorSnapshotWorkKind = iota
	errorSnapshotWorkOutcome
)

type errorSnapshotWork struct {
	kind      errorSnapshotWorkKind
	index     model.ErrorSnapshot
	payload   []byte
	requestID string
	outcome   string
}

type errorSnapshotBodyCapture struct {
	Body         []byte
	OriginalSize int64
	ContentType  string
	SHA256       string
	Truncated    bool
	SkipReason   string
}

type errorSnapshotEnvelope struct {
	SchemaVersion    int                        `json:"schema_version"`
	SnapshotID       string                     `json:"snapshot_id"`
	CreatedAt        int64                      `json:"created_at"`
	Request          map[string]any             `json:"request"`
	Route            map[string]any             `json:"route"`
	Error            map[string]any             `json:"error"`
	Timing           map[string]any             `json:"timing,omitempty"`
	ClientRequest    *errorSnapshotBodyFragment `json:"client_request,omitempty"`
	UpstreamRequest  *errorSnapshotBodyFragment `json:"upstream_request,omitempty"`
	UpstreamResponse *errorSnapshotBodyFragment `json:"upstream_response,omitempty"`
	Stream           map[string]any             `json:"stream,omitempty"`
}

type errorSnapshotBodyFragment struct {
	ContentType  string `json:"content_type,omitempty"`
	Body         string `json:"body,omitempty"`
	OriginalSize int64  `json:"original_size"`
	SHA256       string `json:"sha256,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	SkipReason   string `json:"skip_reason,omitempty"`
}

type errorSnapshotTruncatedEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	SnapshotID    string `json:"snapshot_id"`
	Truncated     bool   `json:"truncated"`
	OriginalSize  int64  `json:"original_size"`
	SHA256        string `json:"sha256"`
	PayloadHead   string `json:"payload_head"`
	PayloadTail   string `json:"payload_tail"`
}

var errorSnapshotManager = struct {
	startOnce     sync.Once
	queue         chan errorSnapshotWork
	storageMu     sync.Mutex
	dropped       atomic.Int64
	writeErrors   atomic.Int64
	lastCleanupAt atomic.Int64
	lastError     atomic.Value
}{}

func StartErrorSnapshotManager() {
	errorSnapshotManager.startOnce.Do(func() {
		errorSnapshotManager.queue = make(chan errorSnapshotWork, errorSnapshotQueueSize)
		if err := ensureErrorSnapshotRoot(); err != nil {
			recordErrorSnapshotManagerError(err)
		}
		if err := reconcileErrorSnapshots(); err != nil {
			recordErrorSnapshotManagerError(err)
		}
		go errorSnapshotWorker()
		go errorSnapshotCleanupLoop()
	})
}

func BeginErrorSnapshotAttempt(c *gin.Context, retryIndex int) {
	if c == nil || !error_snapshot_setting.GetSnapshot().Enabled {
		return
	}
	c.Set(errorSnapshotRetryIndexKey, retryIndex)
	c.Set(errorSnapshotUpstreamRequestKey, nil)
	c.Set(errorSnapshotChannelSelectedKey, false)
	c.Set(errorSnapshotCurrentCapturedKey, false)
}

func MarkErrorSnapshotChannelSelected(c *gin.Context) {
	if c == nil || !error_snapshot_setting.GetSnapshot().Enabled {
		return
	}
	c.Set(errorSnapshotChannelSelectedKey, true)
}

func CaptureErrorSnapshotUpstreamRequestIfNeeded(c *gin.Context, body []byte) {
	if c == nil || len(body) == 0 {
		return
	}
	settings := error_snapshot_setting.GetSnapshot()
	if !settings.Enabled || !error_snapshot_setting.IsPriority(c.GetInt("id"), c.GetInt("channel_id")) {
		return
	}
	capture := errorSnapshotBodyCapture{
		OriginalSize: int64(len(body)),
		ContentType:  "application/json",
		SHA256:       hashErrorSnapshotBytes(body),
		Truncated:    len(body) > errorSnapshotMaxResponseFragment,
	}
	if capture.Truncated {
		capture.SkipReason = "body_too_large"
	} else {
		capture.Body = append([]byte(nil), body...)
	}
	c.Set(errorSnapshotUpstreamRequestKey, capture)
}

func CaptureRelayErrorSnapshot(c *gin.Context, err *types.NewAPIError, internalRetry bool) {
	if c == nil || err == nil || isErrorSnapshotClientDisconnect(err) {
		return
	}
	if !error_snapshot_setting.GetSnapshot().Enabled {
		return
	}
	work, buildErr := buildErrorSnapshotWork(c, err, internalRetry)
	if buildErr != nil {
		errorSnapshotManager.dropped.Add(1)
		recordErrorSnapshotManagerError(buildErr)
		return
	}
	if enqueueErrorSnapshotWork(work) {
		c.Set(errorSnapshotCurrentCapturedKey, true)
		c.Set(errorSnapshotAnyCapturedKey, true)
	}
}

func CaptureFinalRelayErrorSnapshotIfNeeded(c *gin.Context, err *types.NewAPIError) {
	if c == nil || err == nil || c.GetBool(errorSnapshotCurrentCapturedKey) || isErrorSnapshotClientDisconnect(err) {
		return
	}
	CaptureRelayErrorSnapshot(c, err, false)
}

func CaptureStreamErrorSnapshot(c *gin.Context, reason string, summary map[string]any) {
	if c == nil || !error_snapshot_setting.GetSnapshot().Enabled {
		return
	}
	err := types.NewErrorWithStatusCode(
		errors.New(strings.TrimSpace(reason)),
		types.ErrorCodeClaudeStreamIncomplete,
		http.StatusBadGateway,
		types.ErrOptionWithSkipRetry(),
	)
	err.Diagnostic = &types.RelayErrorDiagnostic{StreamSummary: summary}
	c.Set(errorSnapshotTerminalOutcomeKey, ErrorSnapshotOutcomeStreamIncomplete)
	CaptureRelayErrorSnapshot(c, err, false)
}

func FinalizeErrorSnapshotRequest(c *gin.Context, err *types.NewAPIError) {
	if c == nil || !c.GetBool(errorSnapshotAnyCapturedKey) {
		return
	}
	outcome := c.GetString(errorSnapshotTerminalOutcomeKey)
	if outcome == "" {
		switch {
		case isErrorSnapshotClientDisconnect(err):
			outcome = ErrorSnapshotOutcomeClientDisconnected
		case err != nil:
			outcome = ErrorSnapshotOutcomeFinalFailure
		default:
			outcome = ErrorSnapshotOutcomeFallbackSucceeded
		}
	}
	enqueueErrorSnapshotWork(errorSnapshotWork{
		kind:      errorSnapshotWorkOutcome,
		requestID: c.GetString(common.RequestIdKey),
		outcome:   outcome,
	})
}

func GetErrorSnapshotStatus() (ErrorSnapshotStatus, error) {
	stats, err := model.GetErrorSnapshotStorageStats()
	status := ErrorSnapshotStatus{
		Settings:      currentErrorSnapshotSettingsView(),
		StoragePath:   errorSnapshotRoot(),
		Storage:       stats,
		DroppedCount:  errorSnapshotManager.dropped.Load(),
		WriteErrors:   errorSnapshotManager.writeErrors.Load(),
		LastCleanupAt: errorSnapshotManager.lastCleanupAt.Load(),
	}
	if value := errorSnapshotManager.lastError.Load(); value != nil {
		status.LastError, _ = value.(string)
	}
	return status, err
}

func UpdateErrorSnapshotSettings(update ErrorSnapshotSettingsUpdate) (ErrorSnapshotStatus, error) {
	values := map[string]string{
		"error_snapshot.enabled":              strconv.FormatBool(update.Enabled),
		"error_snapshot.ttl_minutes":          strconv.Itoa(update.TTLMinutes),
		"error_snapshot.max_storage_mib":      strconv.Itoa(update.MaxStorageMiB),
		"error_snapshot.max_files":            strconv.Itoa(update.MaxFiles),
		"error_snapshot.priority_user_ids":    formatErrorSnapshotIDs(update.PriorityUserIDs),
		"error_snapshot.priority_channel_ids": formatErrorSnapshotIDs(update.PriorityChannelIDs),
	}
	for key, value := range values {
		configKey := strings.TrimPrefix(key, "error_snapshot.")
		normalized, err := error_snapshot_setting.NormalizeOptionValue(configKey, value)
		if err != nil {
			return ErrorSnapshotStatus{}, err
		}
		values[key] = normalized
	}
	if err := model.UpdateOptions(values); err != nil {
		return ErrorSnapshotStatus{}, err
	}
	if err := CleanupErrorSnapshots(); err != nil {
		recordErrorSnapshotManagerError(err)
	}
	return GetErrorSnapshotStatus()
}

func ListErrorSnapshots(query model.ErrorSnapshotQuery) ([]*model.ErrorSnapshot, int64, error) {
	return model.ListErrorSnapshots(query)
}

func ReadErrorSnapshot(id string) (map[string]any, error) {
	compressed, err := ReadCompressedErrorSnapshot(id)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, errorSnapshotMaxPayloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > errorSnapshotMaxPayloadBytes {
		return nil, errors.New("error snapshot payload exceeds limit")
	}
	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func ReadCompressedErrorSnapshot(id string) ([]byte, error) {
	snapshot, err := getValidatedErrorSnapshot(id)
	if err != nil {
		return nil, err
	}
	path, err := safeErrorSnapshotPath(snapshot.RelativePath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func DeleteErrorSnapshot(id string) error {
	errorSnapshotManager.storageMu.Lock()
	defer errorSnapshotManager.storageMu.Unlock()
	snapshot, err := getValidatedErrorSnapshot(id)
	if err != nil {
		return err
	}
	return deleteErrorSnapshotLocked(snapshot)
}

func DeleteAllErrorSnapshots() error {
	errorSnapshotManager.storageMu.Lock()
	defer errorSnapshotManager.storageMu.Unlock()
	snapshots, err := model.ListErrorSnapshotsForCleanup()
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if err := deleteErrorSnapshotLocked(snapshot); err != nil {
			return err
		}
	}
	removeEmptyErrorSnapshotDirectories()
	return nil
}

func CleanupErrorSnapshots() error {
	errorSnapshotManager.storageMu.Lock()
	defer errorSnapshotManager.storageMu.Unlock()
	return cleanupErrorSnapshotsLocked(0, 0)
}

func buildErrorSnapshotWork(c *gin.Context, apiErr *types.NewAPIError, internalRetry bool) (errorSnapshotWork, error) {
	now := time.Now()
	snapshotID := common.GetUUID()
	channelID := 0
	channelName := ""
	channelType := 0
	if c.GetBool(errorSnapshotChannelSelectedKey) {
		channelID = c.GetInt("channel_id")
		channelName = c.GetString("channel_name")
		channelType = c.GetInt("channel_type")
	}
	priority := error_snapshot_setting.IsPriority(c.GetInt("id"), channelID)
	captureLevel := model.ErrorSnapshotCaptureLevelSummary
	if priority {
		captureLevel = model.ErrorSnapshotCaptureLevelPriority
	}
	requestPath := ""
	method := ""
	contentType := ""
	if c.Request != nil {
		method = c.Request.Method
		contentType = c.Request.Header.Get("Content-Type")
		if c.Request.URL != nil {
			requestPath = c.Request.URL.Path
		}
	}
	index := model.ErrorSnapshot{
		ID:             snapshotID,
		CreatedAt:      now.Unix(),
		RequestID:      c.GetString(common.RequestIdKey),
		RequestPath:    requestPath,
		UserID:         c.GetInt("id"),
		Username:       c.GetString("username"),
		ChannelID:      channelID,
		ChannelName:    channelName,
		ModelName:      c.GetString("original_model"),
		AggregateGroup: common.GetContextKeyString(c, constant.ContextKeyAggregateGroup),
		RouteGroup:     common.GetContextKeyString(c, constant.ContextKeyRouteGroup),
		RetryIndex:     c.GetInt(errorSnapshotRetryIndexKey),
		StatusCode:     apiErr.StatusCode,
		ErrorType:      string(apiErr.GetErrorType()),
		ErrorCode:      string(apiErr.GetErrorCode()),
		ErrorMessage:   limitRequestDumpString(sanitizeErrorSnapshotText(apiErr.MaskSensitiveError()), 4000),
		CaptureLevel:   captureLevel,
		IsStream:       common.GetContextKeyBool(c, constant.ContextKeyRelayIsStream),
		InternalRetry:  internalRetry,
		FinalOutcome:   ErrorSnapshotOutcomePending,
	}
	envelope := errorSnapshotEnvelope{
		SchemaVersion: 1,
		SnapshotID:    snapshotID,
		CreatedAt:     index.CreatedAt,
		Request: map[string]any{
			"request_id":   index.RequestID,
			"method":       method,
			"path":         requestPath,
			"content_type": contentType,
			"user_id":      index.UserID,
			"username":     index.Username,
			"token_id":     c.GetInt("token_id"),
			"token_name":   c.GetString("token_name"),
			"model":        index.ModelName,
			"is_stream":    index.IsStream,
			"headers":      filterRequestDumpHeaders(requestHeaders(c)),
		},
		Route: map[string]any{
			"channel_id":        index.ChannelID,
			"channel_name":      index.ChannelName,
			"channel_type":      channelType,
			"aggregate_group":   index.AggregateGroup,
			"route_group":       index.RouteGroup,
			"route_group_index": common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex),
			"retry_index":       index.RetryIndex,
		},
		Error: map[string]any{
			"status_code":    index.StatusCode,
			"type":           index.ErrorType,
			"code":           index.ErrorCode,
			"message":        index.ErrorMessage,
			"internal_retry": internalRetry,
		},
		Timing: map[string]any{
			"elapsed_ms":              elapsedErrorSnapshotMilliseconds(c, now),
			"first_response_ms":       common.GetContextKeyInt(c, constant.ContextKeyFirstResponseMs),
			"upstream_first_event_ms": common.GetContextKeyInt(c, constant.ContextKeyUpstreamFirstEventMs),
		},
	}
	if priority {
		envelope.ClientRequest = captureClientRequestBody(c, contentType)
		if value, ok := c.Get(errorSnapshotUpstreamRequestKey); ok {
			if capture, captureOK := value.(errorSnapshotBodyCapture); captureOK {
				envelope.UpstreamRequest = buildErrorSnapshotBodyFragment(capture.Body, capture.OriginalSize, capture.ContentType, errorSnapshotMaxBodyFragment)
				envelope.UpstreamRequest.SHA256 = capture.SHA256
				envelope.UpstreamRequest.Truncated = envelope.UpstreamRequest.Truncated || capture.Truncated
				if capture.SkipReason != "" {
					envelope.UpstreamRequest.SkipReason = capture.SkipReason
				}
			}
		}
	}
	if diagnostic := apiErr.Diagnostic; diagnostic != nil {
		if len(diagnostic.UpstreamResponseBody) > 0 || diagnostic.UpstreamBodySize > 0 {
			size := diagnostic.UpstreamBodySize
			if size <= 0 {
				size = int64(len(diagnostic.UpstreamResponseBody))
			}
			envelope.UpstreamResponse = buildErrorSnapshotBodyFragment(diagnostic.UpstreamResponseBody, size, "application/json", errorSnapshotMaxResponseFragment)
		}
		if len(diagnostic.StreamSummary) > 0 {
			envelope.Stream = common.MaskSensitiveValue(diagnostic.StreamSummary).(map[string]any)
		}
	}
	payload, err := common.Marshal(envelope)
	if err != nil {
		return errorSnapshotWork{}, err
	}
	boundedPayload, originalSize, truncated, err := boundErrorSnapshotPayload(snapshotID, payload)
	if err != nil {
		return errorSnapshotWork{}, err
	}
	index.OriginalSize = originalSize
	index.PayloadTruncated = truncated
	return errorSnapshotWork{kind: errorSnapshotWorkCapture, index: index, payload: boundedPayload}, nil
}

func errorSnapshotWorker() {
	for work := range errorSnapshotManager.queue {
		switch work.kind {
		case errorSnapshotWorkCapture:
			if err := writeErrorSnapshot(work); err != nil {
				errorSnapshotManager.writeErrors.Add(1)
				recordErrorSnapshotManagerError(err)
			}
		case errorSnapshotWorkOutcome:
			if err := model.UpdateErrorSnapshotOutcome(work.requestID, work.outcome); err != nil {
				errorSnapshotManager.writeErrors.Add(1)
				recordErrorSnapshotManagerError(err)
			}
		}
	}
}

func errorSnapshotCleanupLoop() {
	ticker := time.NewTicker(errorSnapshotCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := CleanupErrorSnapshots(); err != nil {
			recordErrorSnapshotManagerError(err)
		}
	}
}

func writeErrorSnapshot(work errorSnapshotWork) error {
	errorSnapshotManager.storageMu.Lock()
	defer errorSnapshotManager.storageMu.Unlock()
	if err := ensureErrorSnapshotRoot(); err != nil {
		return err
	}
	if err := cleanupErrorSnapshotsLocked(1, errorSnapshotMaxPayloadBytes+1024); err != nil {
		return err
	}
	dateDir := time.Unix(work.index.CreatedAt, 0).Format("20060102")
	relativePath := filepath.Join(dateDir, work.index.ID+".json.gz")
	finalPath, err := safeErrorSnapshotPath(relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(finalPath), 0o700); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(filepath.Dir(finalPath), ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		_ = tempFile.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := tempFile.Chmod(0o600); err != nil {
		return err
	}
	gzipWriter, err := gzip.NewWriterLevel(tempFile, gzip.BestSpeed)
	if err != nil {
		return err
	}
	if _, err := gzipWriter.Write(work.payload); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return err
	}
	removeTemp = false
	info, err := os.Stat(finalPath)
	if err != nil {
		_ = os.Remove(finalPath)
		return err
	}
	work.index.RelativePath = relativePath
	work.index.CompressedSize = info.Size()
	if err := model.CreateErrorSnapshot(&work.index); err != nil {
		_ = os.Remove(finalPath)
		return err
	}
	return cleanupErrorSnapshotsLocked(0, 0)
}

func cleanupErrorSnapshotsLocked(reserveFiles int, reserveBytes int64) error {
	snapshots, err := model.ListErrorSnapshotsForCleanup()
	if err != nil {
		return err
	}
	settings := error_snapshot_setting.GetSnapshot()
	cutoff := time.Now().Add(-time.Duration(settings.TTLMinutes) * time.Minute).Unix()
	remaining := make([]*model.ErrorSnapshot, 0, len(snapshots))
	var totalBytes int64
	for _, snapshot := range snapshots {
		if snapshot.CreatedAt < cutoff {
			if err := deleteErrorSnapshotLocked(snapshot); err != nil {
				remaining = append(remaining, snapshot)
				totalBytes += snapshot.CompressedSize
			}
			continue
		}
		remaining = append(remaining, snapshot)
		totalBytes += snapshot.CompressedSize
	}
	maxBytes := int64(settings.MaxStorageMiB) << 20
	for len(remaining)+reserveFiles > settings.MaxFiles || totalBytes+reserveBytes > maxBytes {
		if len(remaining) == 0 {
			break
		}
		oldest := remaining[0]
		remaining = remaining[1:]
		if err := deleteErrorSnapshotLocked(oldest); err != nil {
			return err
		}
		totalBytes -= oldest.CompressedSize
	}
	errorSnapshotManager.lastCleanupAt.Store(time.Now().Unix())
	removeEmptyErrorSnapshotDirectories()
	return nil
}

func reconcileErrorSnapshots() error {
	errorSnapshotManager.storageMu.Lock()
	defer errorSnapshotManager.storageMu.Unlock()
	if err := ensureErrorSnapshotRoot(); err != nil {
		return err
	}
	snapshots, err := model.ListErrorSnapshotsForCleanup()
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		path, pathErr := safeErrorSnapshotPath(snapshot.RelativePath)
		if pathErr != nil {
			_ = model.DeleteErrorSnapshotRecord(snapshot.ID)
			continue
		}
		if _, statErr := os.Stat(path); statErr != nil {
			if os.IsNotExist(statErr) {
				_ = model.DeleteErrorSnapshotRecord(snapshot.ID)
				continue
			}
			return statErr
		}
		known[filepath.Clean(path)] = struct{}{}
	}
	err = filepath.WalkDir(errorSnapshotRoot(), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".tmp") {
			return os.Remove(path)
		}
		if strings.HasSuffix(entry.Name(), ".json.gz") {
			if _, ok := known[filepath.Clean(path)]; !ok {
				return os.Remove(path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return cleanupErrorSnapshotsLocked(0, 0)
}

func deleteErrorSnapshotLocked(snapshot *model.ErrorSnapshot) error {
	path, err := safeErrorSnapshotPath(snapshot.RelativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return model.DeleteErrorSnapshotRecord(snapshot.ID)
}

func ensureErrorSnapshotRoot() error {
	root := errorSnapshotRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	return os.Chmod(root, 0o700)
}

func errorSnapshotRoot() string {
	configured := strings.TrimSpace(os.Getenv("ERROR_SNAPSHOT_DIR"))
	if configured == "" {
		configured = filepath.Join(*common.LogDir, "error-snapshots")
	}
	absolute, err := filepath.Abs(configured)
	if err != nil {
		return filepath.Clean(configured)
	}
	return filepath.Clean(absolute)
}

func safeErrorSnapshotPath(relativePath string) (string, error) {
	relativePath = filepath.Clean(strings.TrimSpace(relativePath))
	if relativePath == "." || filepath.IsAbs(relativePath) || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid error snapshot path")
	}
	if !strings.HasSuffix(relativePath, ".json.gz") {
		return "", errors.New("invalid error snapshot extension")
	}
	root := errorSnapshotRoot()
	path := filepath.Join(root, relativePath)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("error snapshot path escapes storage root")
	}
	return path, nil
}

func getValidatedErrorSnapshot(id string) (*model.ErrorSnapshot, error) {
	id = strings.TrimSpace(id)
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 {
		return nil, errors.New("invalid error snapshot id")
	}
	return model.GetErrorSnapshot(id)
}

func enqueueErrorSnapshotWork(work errorSnapshotWork) bool {
	if errorSnapshotManager.queue == nil {
		errorSnapshotManager.dropped.Add(1)
		return false
	}
	select {
	case errorSnapshotManager.queue <- work:
		return true
	default:
		errorSnapshotManager.dropped.Add(1)
		return false
	}
}

func currentErrorSnapshotSettingsView() ErrorSnapshotSettingsView {
	settings := error_snapshot_setting.GetSettings()
	return ErrorSnapshotSettingsView{
		Enabled:            settings.Enabled,
		TTLMinutes:         settings.TTLMinutes,
		MaxStorageMiB:      settings.MaxStorageMiB,
		MaxFiles:           settings.MaxFiles,
		PriorityUserIDs:    parseErrorSnapshotIDs(settings.PriorityUserIDs),
		PriorityChannelIDs: parseErrorSnapshotIDs(settings.PriorityChannelIDs),
	}
}

func captureClientRequestBody(c *gin.Context, contentType string) *errorSnapshotBodyFragment {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return &errorSnapshotBodyFragment{ContentType: contentType, SkipReason: "read_body_failed"}
	}
	if isUnsupportedRequestDumpContentType(contentType) {
		fragment := &errorSnapshotBodyFragment{
			ContentType:  contentType,
			OriginalSize: storage.Size(),
			SkipReason:   "unsupported_content_type",
		}
		fragment.SHA256, err = hashErrorSnapshotBodyStorage(storage)
		if err != nil {
			fragment.SkipReason = "read_body_failed"
		}
		return fragment
	}
	if storage.Size() > errorSnapshotMaxBodyFragment {
		hash, hashErr := hashErrorSnapshotBodyStorage(storage)
		if hashErr != nil {
			return &errorSnapshotBodyFragment{ContentType: contentType, OriginalSize: storage.Size(), SkipReason: "read_body_failed"}
		}
		return &errorSnapshotBodyFragment{
			ContentType:  contentType,
			OriginalSize: storage.Size(),
			SHA256:       hash,
			Truncated:    true,
			SkipReason:   "body_too_large",
		}
	}
	body, err := storage.Bytes()
	if err != nil {
		return &errorSnapshotBodyFragment{ContentType: contentType, OriginalSize: storage.Size(), SkipReason: "read_body_failed"}
	}
	return buildErrorSnapshotBodyFragment(body, storage.Size(), contentType, errorSnapshotMaxBodyFragment)
}

func buildErrorSnapshotBodyFragment(body []byte, originalSize int64, contentType string, maxBytes int) *errorSnapshotBodyFragment {
	fragment := &errorSnapshotBodyFragment{ContentType: contentType, OriginalSize: originalSize}
	if len(body) == 0 {
		return fragment
	}
	fragment.SHA256 = hashErrorSnapshotBytes(body)
	if len(body) > maxBytes {
		fragment.Truncated = true
		fragment.SkipReason = "body_too_large"
		return fragment
	}
	if !utf8.Valid(body) {
		fragment.SkipReason = "binary_content"
		return fragment
	}
	sanitized := sanitizeErrorSnapshotBody(body)
	if len(sanitized) > maxBytes {
		sanitized = preserveBodyHeadTail(sanitized, maxBytes)
		fragment.Truncated = true
	}
	fragment.Body = string(sanitized)
	return fragment
}

func hashErrorSnapshotBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hashErrorSnapshotString(data string) string {
	hash := sha256.New()
	_, _ = io.WriteString(hash, data)
	return hex.EncodeToString(hash.Sum(nil))
}

func hashErrorSnapshotBodyStorage(storage common.BodyStorage) (hashValue string, err error) {
	currentPosition, err := storage.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}
	if _, err = storage.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	defer func() {
		if _, restoreErr := storage.Seek(currentPosition, io.SeekStart); err == nil && restoreErr != nil {
			err = restoreErr
			hashValue = ""
		}
	}()
	hash := sha256.New()
	buffer := make([]byte, 32<<10)
	if _, err = io.CopyBuffer(hash, storage, buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sanitizeErrorSnapshotBody(body []byte) []byte {
	var value any
	if err := common.Unmarshal(body, &value); err == nil {
		value = maskErrorSnapshotValue(value, "")
		if sanitized, marshalErr := common.Marshal(value); marshalErr == nil {
			return sanitized
		}
	}
	return []byte(sanitizeErrorSnapshotText(string(body)))
}

func sanitizeErrorSnapshotText(value string) string {
	masked := common.MaskSensitiveInfo(value)
	return errorSnapshotSecretAssignmentPattern.ReplaceAllString(masked, "${1}***")
}

func maskErrorSnapshotValue(value any, key string) any {
	if isErrorSnapshotSecretKey(key) {
		return "***"
	}
	switch typed := value.(type) {
	case string:
		if summary, ok := summarizeErrorSnapshotBase64(typed, key); ok {
			return summary
		}
		return common.MaskSensitiveInfo(typed)
	case map[string]any:
		masked := make(map[string]any, len(typed))
		for childKey, child := range typed {
			masked[childKey] = maskErrorSnapshotValue(child, childKey)
		}
		return masked
	case []any:
		masked := make([]any, len(typed))
		for index, child := range typed {
			masked[index] = maskErrorSnapshotValue(child, key)
		}
		return masked
	default:
		return value
	}
}

func summarizeErrorSnapshotBase64(value, key string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil, false
	}
	mimeType := "application/octet-stream"
	payload := trimmed
	lowerPrefix := strings.ToLower(trimmed[:min(len(trimmed), 256)])
	if strings.HasPrefix(lowerPrefix, "data:") {
		separator := strings.Index(trimmed, ",")
		if separator <= len("data:") {
			return nil, false
		}
		metadata := trimmed[len("data:"):separator]
		if !strings.Contains(strings.ToLower(metadata), ";base64") {
			return nil, false
		}
		if mimeEnd := strings.Index(metadata, ";"); mimeEnd > 0 {
			mimeType = metadata[:mimeEnd]
		}
		payload = trimmed[separator+1:]
	} else if !isErrorSnapshotBase64Field(key) || !looksLikeErrorSnapshotBase64(payload) {
		return nil, false
	} else {
		normalizedKey := strings.ToLower(key)
		switch {
		case strings.Contains(normalizedKey, "image"):
			mimeType = "image/*"
		case strings.Contains(normalizedKey, "audio"):
			mimeType = "audio/*"
		case strings.Contains(normalizedKey, "video"):
			mimeType = "video/*"
		}
	}
	return map[string]any{
		"skip_reason":            "base64_content",
		"mime_type":              mimeType,
		"encoded_size":           len(payload),
		"estimated_decoded_size": estimatedErrorSnapshotBase64Size(payload),
		"sha256":                 hashErrorSnapshotString(payload),
	}, true
}

func isErrorSnapshotBase64Field(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"), " ", "_"))
	return strings.Contains(normalized, "base64") ||
		strings.Contains(normalized, "image") ||
		strings.Contains(normalized, "audio") ||
		strings.Contains(normalized, "video") ||
		strings.Contains(normalized, "file")
}

func looksLikeErrorSnapshotBase64(value string) bool {
	if len(value) < 256 || len(value)%4 != 0 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'A' && character <= 'Z') ||
			(character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '+' || character == '/' ||
			(character == '=' && index >= len(value)-2) {
			continue
		}
		return false
	}
	return true
}

func estimatedErrorSnapshotBase64Size(value string) int {
	padding := 0
	if strings.HasSuffix(value, "==") {
		padding = 2
	} else if strings.HasSuffix(value, "=") {
		padding = 1
	}
	return len(value)*3/4 - padding
}

func isErrorSnapshotSecretKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"), " ", "_"))
	switch normalized {
	case "authorization", "proxy_authorization", "cookie", "set_cookie", "key", "token", "password", "secret":
		return true
	}
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.HasSuffix(normalized, "_key") ||
		strings.HasSuffix(normalized, "_token") ||
		strings.HasPrefix(normalized, "token_")
}

func boundErrorSnapshotPayload(snapshotID string, payload []byte) ([]byte, int64, bool, error) {
	originalSize := int64(len(payload))
	if len(payload) <= errorSnapshotMaxPayloadBytes {
		return payload, originalSize, false, nil
	}
	hash := sha256.Sum256(payload)
	headTailLimit := (errorSnapshotMaxPayloadBytes - 4096) / 2
	truncated := errorSnapshotTruncatedEnvelope{
		SchemaVersion: 1,
		SnapshotID:    snapshotID,
		Truncated:     true,
		OriginalSize:  originalSize,
		SHA256:        hex.EncodeToString(hash[:]),
		PayloadHead:   string(payload[:headTailLimit]),
		PayloadTail:   string(payload[len(payload)-headTailLimit:]),
	}
	bounded, err := common.Marshal(truncated)
	if err != nil {
		return nil, 0, false, err
	}
	if len(bounded) > errorSnapshotMaxPayloadBytes {
		return nil, 0, false, errors.New("failed to bound error snapshot payload")
	}
	return bounded, originalSize, true, nil
}

func preserveBodyHeadTail(body []byte, maxBytes int) []byte {
	if len(body) <= maxBytes {
		return append([]byte(nil), body...)
	}
	separator := []byte("\n...[truncated]...\n")
	available := maxBytes - len(separator)
	if available <= 0 {
		return append([]byte(nil), body[:maxBytes]...)
	}
	head := available / 2
	tail := available - head
	result := make([]byte, 0, maxBytes)
	result = append(result, body[:head]...)
	result = append(result, separator...)
	result = append(result, body[len(body)-tail:]...)
	return result
}

func elapsedErrorSnapshotMilliseconds(c *gin.Context, now time.Time) int64 {
	start := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if start.IsZero() {
		return 0
	}
	return now.Sub(start).Milliseconds()
}

func requestHeaders(c *gin.Context) http.Header {
	if c == nil || c.Request == nil {
		return nil
	}
	return c.Request.Header
}

func isErrorSnapshotClientDisconnect(err *types.NewAPIError) bool {
	return err != nil && err.StatusCode == 499 && err.GetErrorCode() == types.ErrorCodeDoRequestFailed
}

func parseErrorSnapshotIDs(value string) []int {
	ids := make([]int, 0)
	seen := make(map[int]struct{})
	for _, part := range strings.Split(value, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func formatErrorSnapshotIDs(ids []int) string {
	values := make(map[int]struct{})
	for _, id := range ids {
		if id > 0 {
			values[id] = struct{}{}
		}
	}
	ordered := make([]int, 0, len(values))
	for id := range values {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)
	parts := make([]string, 0, len(ordered))
	for _, id := range ordered {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

func removeEmptyErrorSnapshotDirectories() {
	entries, err := os.ReadDir(errorSnapshotRoot())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_ = os.Remove(filepath.Join(errorSnapshotRoot(), entry.Name()))
		}
	}
}

func recordErrorSnapshotManagerError(err error) {
	if err == nil {
		return
	}
	errorSnapshotManager.lastError.Store(err.Error())
	logger.LogError(contextForRequestDumpLog(""), "error snapshot: "+err.Error())
}
