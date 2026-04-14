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
