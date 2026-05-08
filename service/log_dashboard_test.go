package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogDashboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalExcludedIDs := common.LogConsumeExcludedUserIDs

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	_, err := common.SetLogConsumeExcludedUserIDs("")
	require.NoError(t, err)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.Channel{}, &model.AggregateGroup{}, &model.AggregateGroupTarget{}))

	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		_, _ = common.SetLogConsumeExcludedUserIDs(originalExcludedIDs)
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedLogDashboardChannel(t *testing.T, db *gorm.DB, id int, name string) {
	t.Helper()
	channel := &model.Channel{
		Id:     id,
		Name:   name,
		Key:    fmt.Sprintf("key-%d", id),
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "claude-haiku-4-5",
	}
	require.NoError(t, db.Create(channel).Error)
}

func seedLogDashboardLog(t *testing.T, db *gorm.DB, logItem *model.Log) {
	t.Helper()
	require.NoError(t, db.Create(logItem).Error)
}

func TestGetLogDashboardAggregatesRequestsAndChannels(t *testing.T) {
	db := setupLogDashboardTestDB(t)
	restoreNow := SetLogDashboardNowForTest(func() time.Time {
		return time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local)
	})
	defer restoreNow()

	_, err := common.SetLogConsumeExcludedUserIDs("200")
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.AggregateGroup{
		Name:        "vip",
		DisplayName: "vip",
		Status:      model.AggregateGroupStatusEnabled,
		GroupRatio:  1,
	}).Error)

	seedLogDashboardChannel(t, db, 1, "alpha")
	seedLogDashboardChannel(t, db, 2, "beta")
	seedLogDashboardChannel(t, db, 3, "probe")

	base := time.Date(2026, 4, 21, 9, 30, 0, 0, time.Local).Unix()

	seedLogDashboardLog(t, db, &model.Log{
		UserId:    101,
		CreatedAt: base,
		Type:      model.LogTypeError,
		Content:   "上游繁忙，请稍后重试 (request id: req-a-1)",
		UseTime:   2,
		ChannelId: 1,
		RequestId: "req-success",
		Group:     "default",
		Other:     `{"status_code":503}`,
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    101,
		CreatedAt: base + 20,
		Type:      model.LogTypeConsume,
		Content:   "success",
		UseTime:   4,
		ChannelId: 2,
		ModelName: "claude-haiku-4-5",
		RequestId: "req-success",
		Group:     "vip",
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    102,
		CreatedAt: base + 60,
		Type:      model.LogTypeError,
		Content:   "系统服务繁忙，请您稍后重试 (request id: req-fail-1)",
		UseTime:   3,
		ChannelId: 1,
		RequestId: "req-failed-1",
		Group:     "default",
		Other:     `{"status_code":503}`,
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    103,
		CreatedAt: base + 120,
		Type:      model.LogTypeError,
		Content:   "系统服务繁忙，请您稍后重试 (request id: req-fail-2)",
		UseTime:   5,
		ChannelId: 1,
		RequestId: "req-failed-2",
		Group:     "default",
		Other:     `{"status_code":503}`,
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    104,
		CreatedAt: base + 180,
		Type:      model.LogTypeError,
		Content:   "无权访问上游分组 (request id: req-empty)",
		UseTime:   1,
		ChannelId: 2,
		RequestId: "",
		Group:     "vip",
		Other:     `{"status_code":403}`,
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    200,
		CreatedAt: base + 240,
		Type:      model.LogTypeError,
		Content:   "探针失败 (request id: probe-1)",
		UseTime:   2,
		ChannelId: 3,
		RequestId: "req-probe",
		Group:     "probe",
		Other:     `{"status_code":503}`,
	})

	dashboard, err := GetLogDashboard(nil, LogDashboardWindow1h)
	require.NoError(t, err)
	require.Equal(t, LogDashboardWindow1h, dashboard.Window)
	require.Equal(t, time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local).Unix(), dashboard.GeneratedAt)
	require.Equal(t, 3, dashboard.Summary.TotalRequests)
	require.Equal(t, 1, dashboard.Summary.SuccessfulRequests)
	require.Equal(t, 2, dashboard.Summary.FailedRequests)
	require.InDelta(t, 33.333, dashboard.Summary.SuccessRate, 0.01)
	require.InDelta(t, 66.666, dashboard.Summary.ErrorRate, 0.01)
	require.InDelta(t, 4.0, dashboard.Summary.AverageSuccessUseTimeSeconds, 0.001)

	require.Len(t, dashboard.TopErrorMessages, 1)
	require.Equal(t, "系统服务繁忙，请您稍后重试", dashboard.TopErrorMessages[0].Message)
	require.Equal(t, 2, dashboard.TopErrorMessages[0].Count)

	require.Len(t, dashboard.TopStatusCodes, 1)
	require.Equal(t, 503, dashboard.TopStatusCodes[0].StatusCode)
	require.Equal(t, 2, dashboard.TopStatusCodes[0].Count)

	require.Len(t, dashboard.Channels, 2)
	require.Equal(t, 1, dashboard.Channels[0].ChannelId)
	require.Equal(t, "alpha", dashboard.Channels[0].ChannelName)
	require.Equal(t, 3, dashboard.Channels[0].AttemptCount)
	require.Equal(t, 0, dashboard.Channels[0].SuccessCount)
	require.Equal(t, 3, dashboard.Channels[0].FailureCount)
	require.Equal(t, 503, dashboard.Channels[0].TopStatusCode)
	require.Equal(t, "系统服务繁忙，请您稍后重试", dashboard.Channels[0].TopErrorMessage)

	require.Equal(t, 2, dashboard.Channels[1].ChannelId)
	require.Equal(t, "beta", dashboard.Channels[1].ChannelName)
	require.Equal(t, 2, dashboard.Channels[1].AttemptCount)
	require.Equal(t, 1, dashboard.Channels[1].SuccessCount)
	require.Equal(t, 1, dashboard.Channels[1].FailureCount)
	require.Equal(t, 403, dashboard.Channels[1].TopStatusCode)

	require.Len(t, dashboard.Groups, 2)
	require.Equal(t, "default", dashboard.Groups[0].GroupName)
	require.Equal(t, 2, dashboard.Groups[0].TotalRequests)
	require.Equal(t, 0, dashboard.Groups[0].SuccessCount)
	require.Equal(t, 2, dashboard.Groups[0].FailureCount)
	require.Equal(t, 503, dashboard.Groups[0].TopStatusCode)
	require.Equal(t, "系统服务繁忙，请您稍后重试", dashboard.Groups[0].TopErrorMessage)

	require.Equal(t, "vip", dashboard.Groups[1].GroupName)
	require.True(t, dashboard.Groups[1].IsAggregateGroup)
	require.Equal(t, 1, dashboard.Groups[1].TotalRequests)
	require.Equal(t, 1, dashboard.Groups[1].SuccessCount)
	require.Equal(t, 0, dashboard.Groups[1].FailureCount)
	require.InDelta(t, 4.0, dashboard.Groups[1].AverageSuccessUseTimeSeconds, 0.001)

	require.Len(t, dashboard.Latency.Channels, 1)
	require.Equal(t, 2, dashboard.Latency.Channels[0].ChannelId)
	require.Equal(t, "beta", dashboard.Latency.Channels[0].ChannelName)
	require.Equal(t, 1, dashboard.Latency.Channels[0].RequestCount)
	require.InDelta(t, 4.0, dashboard.Latency.Channels[0].P95UseTimeSeconds, 0.001)

	require.Len(t, dashboard.Latency.Groups, 1)
	require.Equal(t, "vip", dashboard.Latency.Groups[0].GroupName)
	require.True(t, dashboard.Latency.Groups[0].IsAggregateGroup)
	require.Equal(t, 1, dashboard.Latency.Groups[0].RequestCount)

	require.Len(t, dashboard.Latency.ChannelModels, 1)
	require.Equal(t, 2, dashboard.Latency.ChannelModels[0].ChannelId)
	require.Equal(t, "claude-haiku-4-5", dashboard.Latency.ChannelModels[0].ModelName)

	require.NotEmpty(t, dashboard.GroupTrend)
	require.NotEmpty(t, dashboard.ChannelTrend)
	hasAggregateTrendPoint := false
	for _, point := range dashboard.GroupTrend {
		if point.Series == "vip" && point.IsAggregateGroup {
			hasAggregateTrendPoint = true
			break
		}
	}
	require.True(t, hasAggregateTrendPoint)

	require.Len(t, dashboard.Trend, 13)
	totalTrendRequests := 0
	for _, point := range dashboard.Trend {
		totalTrendRequests += point.TotalRequests
	}
	require.Equal(t, 3, totalTrendRequests)
}

func TestGetLogDashboardSupportsAllWindows(t *testing.T) {
	db := setupLogDashboardTestDB(t)
	restoreNow := SetLogDashboardNowForTest(func() time.Time {
		return time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local)
	})
	defer restoreNow()

	seedLogDashboardLog(t, db, &model.Log{
		UserId:    101,
		CreatedAt: time.Date(2026, 4, 21, 9, 0, 0, 0, time.Local).Unix(),
		Type:      model.LogTypeConsume,
		Content:   "success",
		UseTime:   1,
		ChannelId: 1,
		RequestId: "req-window",
		Group:     "default",
	})

	dashboard1h, err := GetLogDashboard(nil, LogDashboardWindow1h)
	require.NoError(t, err)
	require.Len(t, dashboard1h.Trend, 13)

	dashboard6h, err := GetLogDashboard(nil, LogDashboardWindow6h)
	require.NoError(t, err)
	require.Len(t, dashboard6h.Trend, 25)

	dashboard24h, err := GetLogDashboard(nil, LogDashboardWindow24h)
	require.NoError(t, err)
	require.Len(t, dashboard24h.Trend, 25)
}

func TestGetLogDashboardLatencyAggregatesSuccessfulFinalRequests(t *testing.T) {
	db := setupLogDashboardTestDB(t)
	restoreNow := SetLogDashboardNowForTest(func() time.Time {
		return time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local)
	})
	defer restoreNow()

	require.NoError(t, db.Create(&model.AggregateGroup{
		Name:        "vip",
		DisplayName: "vip",
		Status:      model.AggregateGroupStatusEnabled,
		GroupRatio:  1,
	}).Error)

	seedLogDashboardChannel(t, db, 1, "alpha")
	seedLogDashboardChannel(t, db, 2, "beta")
	base := time.Date(2026, 4, 21, 9, 40, 0, 0, time.Local).Unix()

	seedLogDashboardLog(t, db, &model.Log{
		UserId:    101,
		CreatedAt: base,
		Type:      model.LogTypeError,
		Content:   "first attempt failed",
		UseTime:   99,
		ChannelId: 1,
		ModelName: "claude-haiku-4-5",
		RequestId: "req-retry-success",
		Group:     "default",
		Other:     `{"status_code":503}`,
	})
	for i, useTime := range []int{0, 2, 4, 6, 8, 10} {
		requestID := fmt.Sprintf("req-alpha-%d", i)
		if i == 0 {
			requestID = "req-retry-success"
		}
		seedLogDashboardLog(t, db, &model.Log{
			UserId:    101 + i,
			CreatedAt: base + int64(i+1)*10,
			Type:      model.LogTypeConsume,
			Content:   "success",
			UseTime:   useTime,
			IsStream:  true,
			ChannelId: 2,
			ModelName: "claude-haiku-4-5",
			RequestId: requestID,
			Group:     "vip",
			Other:     fmt.Sprintf(`{"frt":%d}`, i*1000),
		})
	}
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    201,
		CreatedAt: base + 100,
		Type:      model.LogTypeConsume,
		Content:   "success",
		UseTime:   11,
		IsStream:  true,
		ChannelId: 2,
		ModelName: "gpt-4o",
		RequestId: "req-beta-model",
		Group:     "vip",
		Other:     `{"frt":"11000"}`,
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    202,
		CreatedAt: base + 110,
		Type:      model.LogTypeConsume,
		Content:   "success",
		UseTime:   3,
		ChannelId: 1,
		ModelName: "claude-sonnet-4-6",
		RequestId: "req-default-success",
		Group:     "default",
	})
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    203,
		CreatedAt: base + 120,
		Type:      model.LogTypeConsume,
		Content:   "success without request id",
		UseTime:   120,
		ChannelId: 1,
		ModelName: "ignored",
		RequestId: "",
		Group:     "default",
	})

	dashboard, err := GetLogDashboard(nil, LogDashboardWindow1h)
	require.NoError(t, err)
	require.Equal(t, 8, dashboard.Summary.SuccessfulRequests)
	require.Equal(t, 8, dashboard.Summary.TotalRequests)

	require.Len(t, dashboard.Latency.Channels, 2)
	betaChannel := dashboard.Latency.Channels[0]
	require.Equal(t, 2, betaChannel.ChannelId)
	require.Equal(t, "beta", betaChannel.ChannelName)
	require.Equal(t, 7, betaChannel.RequestCount)
	require.InDelta(t, 41.0/7.0, betaChannel.AverageUseTimeSeconds, 0.001)
	require.InDelta(t, 6.0, betaChannel.P50UseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.P90UseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.P95UseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.MaxUseTimeSeconds, 0.001)
	require.Equal(t, 7, betaChannel.FirstResponseTimeCount)
	require.InDelta(t, 26.0/7.0, betaChannel.AverageFirstResponseTimeSeconds, 0.001)
	require.InDelta(t, 3.0, betaChannel.P50FirstResponseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.P90FirstResponseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.P95FirstResponseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, betaChannel.MaxFirstResponseTimeSeconds, 0.001)

	alphaChannel := dashboard.Latency.Channels[1]
	require.Equal(t, 1, alphaChannel.ChannelId)
	require.Equal(t, 1, alphaChannel.RequestCount)
	require.InDelta(t, 3.0, alphaChannel.P95UseTimeSeconds, 0.001)
	require.Equal(t, 0, alphaChannel.FirstResponseTimeCount)

	require.Len(t, dashboard.Latency.Groups, 2)
	vipGroup := dashboard.Latency.Groups[0]
	require.Equal(t, "vip", vipGroup.GroupName)
	require.True(t, vipGroup.IsAggregateGroup)
	require.Equal(t, 7, vipGroup.RequestCount)
	require.InDelta(t, 6.0, vipGroup.P50UseTimeSeconds, 0.001)
	require.InDelta(t, 11.0, vipGroup.P95UseTimeSeconds, 0.001)
	require.Equal(t, 7, vipGroup.FirstResponseTimeCount)
	require.InDelta(t, 11.0, vipGroup.P95FirstResponseTimeSeconds, 0.001)

	defaultGroup := dashboard.Latency.Groups[1]
	require.Equal(t, "default", defaultGroup.GroupName)
	require.False(t, defaultGroup.IsAggregateGroup)
	require.Equal(t, 1, defaultGroup.RequestCount)
	require.InDelta(t, 3.0, defaultGroup.P95UseTimeSeconds, 0.001)
	require.Equal(t, 0, defaultGroup.FirstResponseTimeCount)

	require.Len(t, dashboard.Latency.ChannelModels, 3)
	require.Equal(t, 2, dashboard.Latency.ChannelModels[0].ChannelId)
	require.Equal(t, "gpt-4o", dashboard.Latency.ChannelModels[0].ModelName)
	require.Equal(t, 1, dashboard.Latency.ChannelModels[0].RequestCount)
	require.InDelta(t, 11.0, dashboard.Latency.ChannelModels[0].P95UseTimeSeconds, 0.001)
	require.Equal(t, 1, dashboard.Latency.ChannelModels[0].FirstResponseTimeCount)
	require.InDelta(t, 11.0, dashboard.Latency.ChannelModels[0].P95FirstResponseTimeSeconds, 0.001)

	require.Equal(t, 2, dashboard.Latency.ChannelModels[1].ChannelId)
	require.Equal(t, "claude-haiku-4-5", dashboard.Latency.ChannelModels[1].ModelName)
	require.Equal(t, 6, dashboard.Latency.ChannelModels[1].RequestCount)
	require.InDelta(t, 4.0, dashboard.Latency.ChannelModels[1].P50UseTimeSeconds, 0.001)
	require.InDelta(t, 10.0, dashboard.Latency.ChannelModels[1].P95UseTimeSeconds, 0.001)
	require.Equal(t, 6, dashboard.Latency.ChannelModels[1].FirstResponseTimeCount)
	require.InDelta(t, 2.0, dashboard.Latency.ChannelModels[1].P50FirstResponseTimeSeconds, 0.001)
	require.InDelta(t, 5.0, dashboard.Latency.ChannelModels[1].P95FirstResponseTimeSeconds, 0.001)

	require.Equal(t, 1, dashboard.Latency.ChannelModels[2].ChannelId)
	require.Equal(t, "claude-sonnet-4-6", dashboard.Latency.ChannelModels[2].ModelName)
	require.Equal(t, 1, dashboard.Latency.ChannelModels[2].RequestCount)
	require.InDelta(t, 3.0, dashboard.Latency.ChannelModels[2].P95UseTimeSeconds, 0.001)
	require.Equal(t, 0, dashboard.Latency.ChannelModels[2].FirstResponseTimeCount)
}

func TestGetLogDashboardLatencyEmptyWithoutSuccessfulRequests(t *testing.T) {
	db := setupLogDashboardTestDB(t)
	restoreNow := SetLogDashboardNowForTest(func() time.Time {
		return time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local)
	})
	defer restoreNow()

	seedLogDashboardChannel(t, db, 1, "alpha")
	seedLogDashboardLog(t, db, &model.Log{
		UserId:    101,
		CreatedAt: time.Date(2026, 4, 21, 9, 45, 0, 0, time.Local).Unix(),
		Type:      model.LogTypeError,
		Content:   "failed",
		UseTime:   7,
		ChannelId: 1,
		ModelName: "claude-haiku-4-5",
		RequestId: "req-failed",
		Group:     "default",
		Other:     `{"status_code":500}`,
	})

	dashboard, err := GetLogDashboard(nil, LogDashboardWindow1h)
	require.NoError(t, err)
	require.Equal(t, 1, dashboard.Summary.TotalRequests)
	require.Equal(t, 0, dashboard.Summary.SuccessfulRequests)
	require.Empty(t, dashboard.Latency.Channels)
	require.Empty(t, dashboard.Latency.Groups)
	require.Empty(t, dashboard.Latency.ChannelModels)
	require.NotEmpty(t, dashboard.Trend)
}

func TestGetLogDashboardRejectsInvalidWindow(t *testing.T) {
	setupLogDashboardTestDB(t)
	_, err := GetLogDashboard(nil, "2h")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid window")
}
