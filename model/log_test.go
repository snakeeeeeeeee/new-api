package model

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func seedLogTestUser(t *testing.T, userID int, username string) {
	t.Helper()
	password := "password123"
	user := &User{
		Id:       userID,
		Username: username,
		Password: password,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  fmt.Sprintf("aff-%d", userID),
	}
	require.NoError(t, DB.Create(user).Error)
}

func resetLogTestTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
}

func countLogsByTypeAndUser(t *testing.T, userID int, logType int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ? AND type = ?", userID, logType).Count(&count).Error)
	return count
}

func TestRecordConsumeLogSkipsExcludedUsers(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)
	_, err := common.SetLogConsumeExcludedUserIDs("")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = common.SetLogConsumeExcludedUserIDs("")
		common.LogConsumeEnabled = true
	})
	common.LogConsumeEnabled = true

	seedLogTestUser(t, 17, "health-status")
	seedLogTestUser(t, 18, "normal-user")

	_, err = common.SetLogConsumeExcludedUserIDs("17,34")
	require.NoError(t, err)

	excludedRecorder := httptest.NewRecorder()
	excludedCtx, _ := gin.CreateTestContext(excludedRecorder)
	excludedCtx.Set("username", "health-status")
	excludedCtx.Set(common.RequestIdKey, "req-excluded")
	RecordConsumeLog(excludedCtx, 17, RecordConsumeLogParams{
		ModelName: "claude-opus-4-6",
		TokenName: "health-probe",
		Quota:     1,
		Group:     "health_probe",
	})
	require.Equal(t, int64(0), countLogsByTypeAndUser(t, 17, LogTypeConsume))

	normalRecorder := httptest.NewRecorder()
	normalCtx, _ := gin.CreateTestContext(normalRecorder)
	normalCtx.Set("username", "normal-user")
	normalCtx.Set(common.RequestIdKey, "req-normal")
	RecordConsumeLog(normalCtx, 18, RecordConsumeLogParams{
		ModelName: "claude-opus-4-6",
		TokenName: "normal-token",
		Quota:     1,
		Group:     "default",
	})
	require.Equal(t, int64(1), countLogsByTypeAndUser(t, 18, LogTypeConsume))
}

func TestRecordErrorLogStillWritesForExcludedUsers(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)
	_, err := common.SetLogConsumeExcludedUserIDs("")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = common.SetLogConsumeExcludedUserIDs("")
	})

	seedLogTestUser(t, 17, "health-status")
	_, err = common.SetLogConsumeExcludedUserIDs("17")
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("username", "health-status")
	ctx.Set(common.RequestIdKey, "req-error")

	RecordErrorLog(ctx, 17, 6, "claude-opus-4-6", "health-probe", "upstream 503", 0, 2, false, "health_probe", nil)

	require.Equal(t, int64(1), countLogsByTypeAndUser(t, 17, LogTypeError))
	require.Equal(t, int64(0), countLogsByTypeAndUser(t, 17, LogTypeConsume))
}

func TestFormatUserLogsHidesModelMapping(t *testing.T) {
	logs := []*Log{
		{
			Id:          42,
			ChannelName: "hidden-channel",
			ModelName:   "claude-haiku-4-5",
			Other: common.MapToJsonStr(map[string]interface{}{
				"admin_info":          map[string]interface{}{"use_channel": []string{"hidden"}},
				"reject_reason":       "hidden",
				"is_model_mapped":     true,
				"upstream_model_name": "claude-haiku-4-5-20251001",
				"model_ratio":         0.5,
			}),
		},
	}

	formatUserLogs(logs, 0)

	require.Empty(t, logs[0].ChannelName)
	require.Equal(t, 1, logs[0].Id)
	require.Equal(t, "claude-haiku-4-5", logs[0].ModelName)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "admin_info")
	require.NotContains(t, other, "reject_reason")
	require.NotContains(t, other, "is_model_mapped")
	require.NotContains(t, other, "upstream_model_name")
	require.Equal(t, float64(0.5), other["model_ratio"])
}

func TestFormatUserLogsSanitizesErrorContent(t *testing.T) {
	logs := []*Log{
		{
			Id:      42,
			Type:    LogTypeError,
			Content: `status_code=429, upstream capacity {"reason":"INSUFFICIENT_MODEL_CAPACITY"}`,
			Other: common.MapToJsonStr(map[string]interface{}{
				"error_type":     "openai_error",
				"error_code":     "rate_limit_error",
				"status_code":    429,
				"channel_id":     9,
				"channel_name":   "upstream-a",
				"channel_type":   1,
				"internal_retry": false,
				"user_safe":      true,
				"request_path":   "/v1/chat/completions",
			}),
		},
	}

	formatUserLogs(logs, 0)

	require.Equal(t, userFacingRelayErrorLog, logs[0].Content)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "error_type")
	require.NotContains(t, other, "error_code")
	require.NotContains(t, other, "status_code")
	require.NotContains(t, other, "channel_id")
	require.NotContains(t, other, "channel_name")
	require.NotContains(t, other, "channel_type")
	require.NotContains(t, other, "internal_retry")
	require.NotContains(t, other, "user_safe")
	require.Equal(t, "/v1/chat/completions", other["request_path"])
}

func TestGetUserLogsHidesInternalRetryErrors(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)
	seedLogTestUser(t, 19, "retry-hidden")

	finalLog := &Log{
		UserId:    19,
		Username:  "retry-hidden",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeError,
		Content:   "status_code=500, upstream final error",
		TokenName: "retry-token",
		ModelName: "claude-opus-4-6",
		Other:     common.MapToJsonStr(map[string]interface{}{"user_safe": true, "status_code": 500}),
	}
	internalRetryLog := &Log{
		UserId:    19,
		Username:  "retry-hidden",
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeError,
		Content:   "status_code=429, upstream retry error",
		TokenName: "retry-token",
		ModelName: "claude-opus-4-6",
		Other:     common.MapToJsonStr(map[string]interface{}{"internal_retry": true, "status_code": 429}),
	}
	require.NoError(t, LOG_DB.Create(internalRetryLog).Error)
	require.NoError(t, LOG_DB.Create(finalLog).Error)

	logs, total, err := GetUserLogs(19, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "")

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, userFacingRelayErrorLog, logs[0].Content)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "status_code")
	require.NotContains(t, other, "internal_retry")
	require.NotContains(t, logs[0].Content, "upstream")
}
