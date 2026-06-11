package model

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

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

func seedUsageStatsLog(t *testing.T, logItem *Log) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(logItem).Error)
}

func sumUsageStatsTrend(trend []UsageStatsTrendPoint) UsageStatsTrendPoint {
	var total UsageStatsTrendPoint
	for _, point := range trend {
		total.Quota += point.Quota
		total.RequestCount += point.RequestCount
		total.PromptTokens += point.PromptTokens
		total.CompletionTokens += point.CompletionTokens
		total.TotalTokens += point.TotalTokens
	}
	return total
}

func TestGetUsageStatsAggregatesAndFiltersConsumeLogs(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)

	base := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local).Unix()
	seedUsageStatsLog(t, &Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        base,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            100,
		PromptTokens:     10,
		CompletionTokens: 20,
		UseTime:          2,
		ChannelId:        7,
		Group:            "vip",
	})
	seedUsageStatsLog(t, &Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        base + 1800,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            0,
		PromptTokens:     5,
		CompletionTokens: 6,
		UseTime:          4,
		ChannelId:        7,
		Group:            "vip",
	})
	seedUsageStatsLog(t, &Log{
		UserId:           2,
		Username:         "bob",
		CreatedAt:        base + 3600,
		Type:             LogTypeConsume,
		ModelName:        "claude-sonnet",
		Quota:            250,
		PromptTokens:     30,
		CompletionTokens: 40,
		UseTime:          6,
		ChannelId:        8,
		Group:            "default",
	})
	seedUsageStatsLog(t, &Log{
		UserId:    3,
		Username:  "carol",
		CreatedAt: base + 60,
		Type:      LogTypeError,
		ModelName: "gpt-4o",
		Quota:     999,
		ChannelId: 7,
		Group:     "vip",
	})
	seedUsageStatsLog(t, &Log{
		UserId:    4,
		Username:  "dave",
		CreatedAt: base - 60,
		Type:      LogTypeConsume,
		ModelName: "gpt-4o",
		Quota:     500,
		ChannelId: 7,
		Group:     "vip",
	})

	stats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:   base - 1,
		EndTimestamp:     base + 7200,
		ModelName:        "gpt-4o",
		Group:            "vip",
		Channel:          7,
		Limit:            10,
		TrendGranularity: UsageStatsGranularityHour,
	})

	require.NoError(t, err)
	require.Equal(t, int64(100), stats.Summary.Quota)
	require.Equal(t, int64(2), stats.Summary.RequestCount)
	require.Equal(t, int64(1), stats.Summary.ActiveUserCount)
	require.Equal(t, int64(15), stats.Summary.PromptTokens)
	require.Equal(t, int64(26), stats.Summary.CompletionTokens)
	require.Equal(t, int64(41), stats.Summary.TotalTokens)
	require.Equal(t, UsageStatsGranularityHour, stats.TrendGranularity)

	require.Len(t, stats.Ranking, 1)
	require.Equal(t, 1, stats.Ranking[0].UserId)
	require.Equal(t, "alice", stats.Ranking[0].Username)
	require.Equal(t, int64(100), stats.Ranking[0].Quota)
	require.Equal(t, int64(2), stats.Ranking[0].RequestCount)
	require.Equal(t, float64(3), stats.Ranking[0].AverageUseTime)

	require.Len(t, stats.Models, 1)
	require.Equal(t, "gpt-4o", stats.Models[0].ModelName)
	require.Equal(t, int64(100), stats.Models[0].Quota)
	require.Equal(t, int64(2), stats.Models[0].RequestCount)

	require.NotEmpty(t, stats.Trend)
	trendTotal := sumUsageStatsTrend(stats.Trend)
	require.Equal(t, int64(2), trendTotal.RequestCount)
	require.Equal(t, int64(100), trendTotal.Quota)

	require.Len(t, stats.UserModelDetails, 1)
	require.Equal(t, "gpt-4o", stats.UserModelDetails[0].ModelName)
	require.Equal(t, int64(100), stats.UserModelDetails[0].Quota)
	require.Equal(t, int64(2), stats.UserModelDetails[0].RequestCount)
}

func TestGetUsageStatsRankingLimitAndDailyTrend(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)

	dayOne := time.Date(2026, 6, 1, 12, 0, 0, 0, time.Local).Unix()
	dayTwo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.Local).Unix()
	seedUsageStatsLog(t, &Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        dayOne,
		Type:             LogTypeConsume,
		ModelName:        "model-a",
		Quota:            10,
		PromptTokens:     1,
		CompletionTokens: 2,
	})
	seedUsageStatsLog(t, &Log{
		UserId:           2,
		Username:         "bob",
		CreatedAt:        dayTwo,
		Type:             LogTypeConsume,
		ModelName:        "model-b",
		Quota:            30,
		PromptTokens:     3,
		CompletionTokens: 4,
	})

	stats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local).Unix(),
		EndTimestamp:     time.Date(2026, 6, 2, 23, 59, 59, 0, time.Local).Unix(),
		Limit:            1,
		TrendGranularity: UsageStatsGranularityDay,
	})

	require.NoError(t, err)
	require.Len(t, stats.Ranking, 1)
	require.Equal(t, 2, stats.Ranking[0].UserId)
	require.Equal(t, int64(30), stats.Ranking[0].Quota)
	require.Len(t, stats.Trend, 2)
	require.Equal(t, int64(10), stats.Trend[0].Quota)
	require.Equal(t, int64(30), stats.Trend[1].Quota)
	require.Len(t, stats.UserModelDetails, 1)
	require.Equal(t, 2, stats.UserModelDetails[0].UserId)
}

func TestGetUsageStatsRejectsInvalidRangeAndGranularity(t *testing.T) {
	_, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp: time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local).Unix(),
		EndTimestamp:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local).Unix(),
	})
	require.ErrorContains(t, err, "开始时间不能晚于结束时间")

	_, err = GetUsageStats(UsageStatsQuery{
		StartTimestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).Unix(),
		EndTimestamp:     time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local).Unix(),
		TrendGranularity: "minute",
	})
	require.ErrorContains(t, err, "trend_granularity")

	_, err = GetUsageStats(UsageStatsQuery{
		StartTimestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).Unix(),
		EndTimestamp:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local).Unix(),
	})
	require.ErrorContains(t, err, "90 天")
}
