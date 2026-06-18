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

func seedUsageStatsTopUp(t *testing.T, topUp *TopUp) {
	t.Helper()
	require.NoError(t, DB.Create(topUp).Error)
}

func sumUsageStatsTrend(trend []UsageStatsTrendPoint) UsageStatsTrendPoint {
	var total UsageStatsTrendPoint
	for _, point := range trend {
		total.Quota += point.Quota
		total.RequestCount += point.RequestCount
		total.InputTokens += point.InputTokens
		total.CacheTokens += point.CacheTokens
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
	require.Equal(t, int64(15), stats.Summary.InputTokens)
	require.Equal(t, int64(0), stats.Summary.CacheTokens)
	require.Equal(t, int64(15), stats.Summary.PromptTokens)
	require.Equal(t, int64(26), stats.Summary.CompletionTokens)
	require.Equal(t, int64(41), stats.Summary.TotalTokens)
	require.Equal(t, UsageStatsGranularityHour, stats.TrendGranularity)

	require.Len(t, stats.Ranking, 1)
	require.Equal(t, 1, stats.Ranking[0].UserId)
	require.Equal(t, "alice", stats.Ranking[0].Username)
	require.Equal(t, int64(100), stats.Ranking[0].Quota)
	require.Equal(t, int64(2), stats.Ranking[0].RequestCount)
	require.Equal(t, int64(15), stats.Ranking[0].InputTokens)
	require.Equal(t, int64(0), stats.Ranking[0].CacheTokens)
	require.Equal(t, int64(41), stats.Ranking[0].TotalTokens)
	require.Equal(t, float64(3), stats.Ranking[0].AverageUseTime)

	require.Len(t, stats.Models, 1)
	require.Equal(t, "gpt-4o", stats.Models[0].ModelName)
	require.Equal(t, int64(100), stats.Models[0].Quota)
	require.Equal(t, int64(2), stats.Models[0].RequestCount)
	require.Equal(t, int64(15), stats.Models[0].InputTokens)
	require.Equal(t, int64(0), stats.Models[0].CacheTokens)
	require.Equal(t, int64(41), stats.Models[0].TotalTokens)

	require.NotEmpty(t, stats.Trend)
	trendTotal := sumUsageStatsTrend(stats.Trend)
	require.Equal(t, int64(2), trendTotal.RequestCount)
	require.Equal(t, int64(100), trendTotal.Quota)

	require.Len(t, stats.UserModelDetails, 1)
	require.Equal(t, "gpt-4o", stats.UserModelDetails[0].ModelName)
	require.Equal(t, int64(100), stats.UserModelDetails[0].Quota)
	require.Equal(t, int64(2), stats.UserModelDetails[0].RequestCount)
	require.Equal(t, int64(15), stats.UserModelDetails[0].InputTokens)
	require.Equal(t, int64(0), stats.UserModelDetails[0].CacheTokens)
	require.Equal(t, int64(41), stats.UserModelDetails[0].TotalTokens)
}

func TestGetUsageStatsNormalizesCacheTokenBreakdown(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)

	base := time.Date(2026, 6, 12, 19, 48, 0, 0, time.Local).Unix()
	seedUsageStatsLog(t, &Log{
		UserId:           1,
		Username:         "gpt_user",
		CreatedAt:        base,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4.1",
		Quota:            93830,
		PromptTokens:     29820,
		CompletionTokens: 1246,
		Other:            `{"cache_tokens":24064}`,
	})
	seedUsageStatsLog(t, &Log{
		UserId:           2,
		Username:         "claude_write_user",
		CreatedAt:        base + 60,
		Type:             LogTypeConsume,
		ModelName:        "claude-sonnet-4",
		Quota:            2582,
		PromptTokens:     39,
		CompletionTokens: 32,
		Other:            `{"claude":true,"usage_semantic":"anthropic","cache_write_tokens":81}`,
	})
	seedUsageStatsLog(t, &Log{
		UserId:           3,
		Username:         "claude_read_user",
		CreatedAt:        base + 120,
		Type:             LogTypeConsume,
		ModelName:        "claude-opus-4",
		Quota:            3000,
		PromptTokens:     39,
		CompletionTokens: 32,
		Other:            `{"claude":true,"cache_tokens":100,"cache_creation_tokens_5m":50,"cache_creation_tokens_1h":31}`,
	})

	stats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:   base - 1,
		EndTimestamp:     base + 3600,
		Limit:            10,
		TrendGranularity: UsageStatsGranularityHour,
	})

	require.NoError(t, err)
	require.Equal(t, int64(5996), stats.Summary.InputTokens)
	require.Equal(t, int64(24164), stats.Summary.CacheTokens)
	require.Equal(t, int64(1310), stats.Summary.CompletionTokens)
	require.Equal(t, int64(31470), stats.Summary.TotalTokens)

	byUser := make(map[string]UsageStatsRankItem)
	for _, item := range stats.Ranking {
		byUser[item.Username] = item
	}
	require.Equal(t, int64(5756), byUser["gpt_user"].InputTokens)
	require.Equal(t, int64(24064), byUser["gpt_user"].CacheTokens)
	require.Equal(t, int64(1246), byUser["gpt_user"].CompletionTokens)
	require.Equal(t, int64(31066), byUser["gpt_user"].TotalTokens)

	require.Equal(t, int64(120), byUser["claude_write_user"].InputTokens)
	require.Equal(t, int64(0), byUser["claude_write_user"].CacheTokens)
	require.Equal(t, int64(32), byUser["claude_write_user"].CompletionTokens)
	require.Equal(t, int64(152), byUser["claude_write_user"].TotalTokens)

	require.Equal(t, int64(120), byUser["claude_read_user"].InputTokens)
	require.Equal(t, int64(100), byUser["claude_read_user"].CacheTokens)
	require.Equal(t, int64(32), byUser["claude_read_user"].CompletionTokens)
	require.Equal(t, int64(252), byUser["claude_read_user"].TotalTokens)

	trendTotal := sumUsageStatsTrend(stats.Trend)
	require.Equal(t, stats.Summary.InputTokens, trendTotal.InputTokens)
	require.Equal(t, stats.Summary.CacheTokens, trendTotal.CacheTokens)
	require.Equal(t, stats.Summary.CompletionTokens, trendTotal.CompletionTokens)
	require.Equal(t, stats.Summary.TotalTokens, trendTotal.TotalTokens)
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

func TestGetUsageStatsFiltersByUserID(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)

	base := time.Date(2026, 6, 3, 12, 0, 0, 0, time.Local).Unix()
	seedUsageStatsLog(t, &Log{
		UserId:           11,
		Username:         "usage_user_a",
		CreatedAt:        base,
		Type:             LogTypeConsume,
		ModelName:        "model-a",
		Quota:            100,
		PromptTokens:     10,
		CompletionTokens: 20,
	})
	seedUsageStatsLog(t, &Log{
		UserId:           12,
		Username:         "usage_user_b",
		CreatedAt:        base + 3600,
		Type:             LogTypeConsume,
		ModelName:        "model-b",
		Quota:            900,
		PromptTokens:     90,
		CompletionTokens: 100,
	})

	stats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:   base - 10,
		EndTimestamp:     base + 7200,
		UserId:           11,
		Limit:            10,
		TrendGranularity: UsageStatsGranularityHour,
	})

	require.NoError(t, err)
	require.Equal(t, int64(100), stats.Summary.Quota)
	require.Equal(t, int64(1), stats.Summary.RequestCount)
	require.Len(t, stats.Ranking, 1)
	require.Equal(t, 11, stats.Ranking[0].UserId)
	require.Len(t, stats.Models, 1)
	require.Equal(t, "model-a", stats.Models[0].ModelName)
	trendTotal := sumUsageStatsTrend(stats.Trend)
	require.Equal(t, int64(100), trendTotal.Quota)
	require.Len(t, stats.UserModelDetails, 1)
	require.Equal(t, 11, stats.UserModelDetails[0].UserId)
}

func TestGetUsageStatsRechargeRankingAndDetails(t *testing.T) {
	truncateTables(t)
	resetLogTestTables(t)
	require.NoError(t, DB.Exec("DELETE FROM top_ups").Error)

	base := time.Date(2026, 6, 13, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, DB.Create(&User{Id: 101, Username: "recharge_alice", Group: "vip", Password: "password123", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "usage-recharge-101"}).Error)
	require.NoError(t, DB.Create(&User{Id: 102, Username: "recharge_bob", Group: "vip", Password: "password123", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "usage-recharge-102"}).Error)
	require.NoError(t, DB.Create(&User{Id: 103, Username: "recharge_carol", Group: "default", Password: "password123", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "usage-recharge-103"}).Error)

	seedUsageStatsTopUp(t, &TopUp{
		UserId:        101,
		Amount:        100,
		Money:         10.5,
		TradeNo:       "usage_recharge_alice_1",
		PaymentMethod: "stripe",
		CreateTime:    base - 60,
		CompleteTime:  base,
		Status:        common.TopUpStatusSuccess,
	})
	seedUsageStatsTopUp(t, &TopUp{
		UserId:        101,
		Amount:        200,
		Money:         20,
		TradeNo:       "usage_recharge_alice_2",
		PaymentMethod: "epay",
		CreateTime:    base + 30,
		CompleteTime:  base + 60,
		Status:        common.TopUpStatusSuccess,
	})
	seedUsageStatsTopUp(t, &TopUp{
		UserId:        102,
		Amount:        300,
		Money:         15,
		TradeNo:       "usage_recharge_bob_1",
		PaymentMethod: "waffo",
		CreateTime:    base + 90,
		CompleteTime:  base + 120,
		Status:        common.TopUpStatusSuccess,
	})
	seedUsageStatsTopUp(t, &TopUp{
		UserId:        102,
		Amount:        999,
		Money:         99,
		TradeNo:       "usage_recharge_bob_pending",
		PaymentMethod: "stripe",
		CreateTime:    base + 120,
		CompleteTime:  base + 180,
		Status:        common.TopUpStatusPending,
	})
	seedUsageStatsTopUp(t, &TopUp{
		UserId:        103,
		Amount:        400,
		Money:         40,
		TradeNo:       "usage_recharge_carol_default_group",
		PaymentMethod: "stripe",
		CreateTime:    base + 120,
		CompleteTime:  base + 180,
		Status:        common.TopUpStatusSuccess,
	})
	seedUsageStatsTopUp(t, &TopUp{
		UserId:        101,
		Amount:        500,
		Money:         50,
		TradeNo:       "usage_recharge_alice_out_of_range",
		PaymentMethod: "stripe",
		CreateTime:    base - 7200,
		CompleteTime:  base - 3600,
		Status:        common.TopUpStatusSuccess,
	})

	stats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:     base - 1,
		EndTimestamp:       base + 3600,
		Group:              "vip",
		TrendGranularity:   UsageStatsGranularityHour,
		RechargePage:       1,
		RechargePageSize:   1,
		RechargeUserId:     101,
		RechargeDetailPage: 1,
		RechargeDetailSize: 1,
	})

	require.NoError(t, err)
	require.Equal(t, int64(600), stats.RechargeSummary.Amount)
	require.InDelta(t, 45.5, stats.RechargeSummary.Money, 0.000001)
	require.Equal(t, int64(3), stats.RechargeSummary.OrderCount)
	require.Equal(t, int64(2), stats.RechargeSummary.UserCount)
	require.Equal(t, base+120, stats.RechargeSummary.LastTopUpAt)

	require.Equal(t, 1, stats.RechargeRanking.Page)
	require.Equal(t, 1, stats.RechargeRanking.PageSize)
	require.Equal(t, int64(2), stats.RechargeRanking.Total)
	require.Len(t, stats.RechargeRanking.Items, 1)
	require.Equal(t, 101, stats.RechargeRanking.Items[0].UserId)
	require.Equal(t, "recharge_alice", stats.RechargeRanking.Items[0].Username)
	require.Equal(t, int64(300), stats.RechargeRanking.Items[0].Amount)
	require.InDelta(t, 30.5, stats.RechargeRanking.Items[0].Money, 0.000001)
	require.Equal(t, int64(2), stats.RechargeRanking.Items[0].OrderCount)
	require.Equal(t, base+60, stats.RechargeRanking.Items[0].LastTopUpAt)

	require.Equal(t, 101, stats.RechargeDetails.UserId)
	require.Equal(t, int64(2), stats.RechargeDetails.Total)
	require.Len(t, stats.RechargeDetails.Items, 1)
	require.Equal(t, "usage_recharge_alice_2", stats.RechargeDetails.Items[0].TradeNo)
	require.Equal(t, int64(200), stats.RechargeDetails.Items[0].Amount)
	require.InDelta(t, 20, stats.RechargeDetails.Items[0].Money, 0.000001)
	require.Equal(t, common.TopUpStatusSuccess, stats.RechargeDetails.Items[0].Status)

	pageTwoStats, err := GetUsageStats(UsageStatsQuery{
		StartTimestamp:   base - 1,
		EndTimestamp:     base + 3600,
		Group:            "vip",
		TrendGranularity: UsageStatsGranularityHour,
		RechargePage:     2,
		RechargePageSize: 1,
	})
	require.NoError(t, err)
	require.Len(t, pageTwoStats.RechargeRanking.Items, 1)
	require.Equal(t, 102, pageTwoStats.RechargeRanking.Items[0].UserId)
	require.Equal(t, int64(300), pageTwoStats.RechargeRanking.Items[0].Amount)
	require.InDelta(t, 15, pageTwoStats.RechargeRanking.Items[0].Money, 0.000001)
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

func TestNormalizeUsageStatsQueryDefaultsToToday(t *testing.T) {
	query, err := normalizeUsageStatsQuery(UsageStatsQuery{})

	require.NoError(t, err)
	start := time.Unix(query.StartTimestamp, 0).Local()
	end := time.Unix(query.EndTimestamp, 0).Local()
	require.Equal(t, start.Year(), end.Year())
	require.Equal(t, start.Month(), end.Month())
	require.Equal(t, start.Day(), end.Day())
	require.Equal(t, 0, start.Hour())
	require.Equal(t, 0, start.Minute())
	require.Equal(t, 0, start.Second())
	require.Equal(t, 23, end.Hour())
	require.Equal(t, 59, end.Minute())
	require.Equal(t, 59, end.Second())
	require.Equal(t, UsageStatsGranularityHour, query.TrendGranularity)
}
