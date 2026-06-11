package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1;index:idx_logs_user_type_time,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type;index:idx_logs_user_type_time,priority:3"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type;index:idx_logs_user_type_time,priority:2"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	RequestId        string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	Other            string `json:"other"`
}

type DashboardLogEntry struct {
	UserId      int    `json:"user_id"`
	CreatedAt   int64  `json:"created_at"`
	Type        int    `json:"type"`
	Content     string `json:"content"`
	UseTime     int    `json:"use_time"`
	IsStream    bool   `json:"is_stream"`
	ChannelId   int    `json:"channel_id"`
	ModelName   string `json:"model_name"`
	RequestId   string `json:"request_id"`
	Other       string `json:"other"`
	Group       string `json:"group" gorm:"column:log_group"`
	ChannelName string `json:"channel_name" gorm:"-"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

const userFacingRelayErrorLog = "status_code=500, 系统异常，请稍后重试"

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			delete(otherMap, "reject_reason")
			delete(otherMap, "is_model_mapped")
			delete(otherMap, "upstream_model_name")
			if logs[i].Type == LogTypeError {
				delete(otherMap, "error_type")
				delete(otherMap, "error_code")
				delete(otherMap, "status_code")
				delete(otherMap, "channel_id")
				delete(otherMap, "channel_name")
				delete(otherMap, "channel_type")
				delete(otherMap, "internal_retry")
				delete(otherMap, "user_safe")
			}
		}
		if logs[i].Type == LogTypeError {
			logs[i].Content = userFacingRelayErrorLog
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).
		Where("token_id = ?", tokenId).
		Where("(logs.other IS NULL OR logs.other = '' OR logs.other NOT LIKE ?)", `%"internal_retry"%`).
		Order("id desc").
		Limit(common.MaxRecentItems).
		Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func GetChannelNameMap(channelIDs []int) (map[int]string, error) {
	channelMap := make(map[int]string)
	if len(channelIDs) == 0 {
		return channelMap, nil
	}

	channelIDSet := types.NewSet[int]()
	for _, channelID := range channelIDs {
		if channelID > 0 {
			channelIDSet.Add(channelID)
		}
	}
	if channelIDSet.Len() == 0 {
		return channelMap, nil
	}

	if common.MemoryCacheEnabled {
		for _, channelID := range channelIDSet.Items() {
			cacheChannel, err := CacheGetChannel(channelID)
			if err == nil && cacheChannel != nil && cacheChannel.Name != "" {
				channelMap[channelID] = cacheChannel.Name
			}
		}
	}

	missingIDs := make([]int, 0, channelIDSet.Len())
	for _, channelID := range channelIDSet.Items() {
		if _, ok := channelMap[channelID]; !ok {
			missingIDs = append(missingIDs, channelID)
		}
	}

	if len(missingIDs) == 0 {
		return channelMap, nil
	}

	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := DB.Table("channels").Select("id, name").Where("id IN ?", missingIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		channelMap[channel.Id] = channel.Name
	}
	return channelMap, nil
}

func GetLogsForDashboard(ctx context.Context, startTimestamp int64, endTimestamp int64) ([]*DashboardLogEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ensureCommonColumnsInitialized()
	var logs []*DashboardLogEntry
	query := LOG_DB.WithContext(ctx).
		Model(&Log{}).
		Select("user_id, created_at, type, content, use_time, is_stream, channel_id, model_name, request_id, other, "+logGroupCol+" as log_group").
		Where("type IN ?", []int{LogTypeConsume, LogTypeError})
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	if err := query.Order("created_at asc, id asc").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	if common.IsLogConsumeExcludedUserID(userId) {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	tx = tx.Where("(logs.other IS NULL OR logs.other = '' OR logs.other NOT LIKE ?)", `%"internal_retry"%`)
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

const (
	UsageStatsGranularityHour = "hour"
	UsageStatsGranularityDay  = "day"
	usageStatsDefaultLimit    = 50
	usageStatsMaxLimit        = 200
	usageStatsMaxRangeSeconds = 90 * 24 * 60 * 60
)

type UsageStatsQuery struct {
	StartTimestamp   int64
	EndTimestamp     int64
	ModelName        string
	Username         string
	Group            string
	Channel          int
	Limit            int
	TrendGranularity string
}

type UsageStatsSummary struct {
	Quota            int64 `json:"quota"`
	RequestCount     int64 `json:"request_count"`
	ActiveUserCount  int64 `json:"active_user_count"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type UsageStatsRankItem struct {
	UserId           int     `json:"user_id"`
	Username         string  `json:"username"`
	Quota            int64   `json:"quota"`
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AverageUseTime   float64 `json:"average_use_time"`
	LastRequestAt    int64   `json:"last_request_at"`
}

type UsageStatsTrendPoint struct {
	BucketStart      int64  `json:"bucket_start"`
	Label            string `json:"label"`
	Quota            int64  `json:"quota"`
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

type UsageStatsModelItem struct {
	ModelName        string  `json:"model_name"`
	Quota            int64   `json:"quota"`
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AverageUseTime   float64 `json:"average_use_time"`
}

type UsageStatsUserModelDetail struct {
	UserId           int     `json:"user_id"`
	Username         string  `json:"username"`
	ModelName        string  `json:"model_name"`
	Quota            int64   `json:"quota"`
	RequestCount     int64   `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AverageUseTime   float64 `json:"average_use_time"`
}

type UsageStatsData struct {
	StartTimestamp   int64                       `json:"start_timestamp"`
	EndTimestamp     int64                       `json:"end_timestamp"`
	TrendGranularity string                      `json:"trend_granularity"`
	GeneratedAt      int64                       `json:"generated_at"`
	Summary          UsageStatsSummary           `json:"summary"`
	Ranking          []UsageStatsRankItem        `json:"ranking"`
	Trend            []UsageStatsTrendPoint      `json:"trend"`
	Models           []UsageStatsModelItem       `json:"models"`
	UserModelDetails []UsageStatsUserModelDetail `json:"user_model_details"`
}

type usageStatsBaseRow struct {
	UserId           int
	Username         string
	ModelName        string
	Quota            int64
	PromptTokens     int64
	CompletionTokens int64
	RequestCount     int64
	AverageUseTime   float64
}

type usageStatsTrendRow struct {
	BucketStart      int64
	Quota            int64
	RequestCount     int64
	PromptTokens     int64
	CompletionTokens int64
}

func normalizeUsageStatsQuery(query UsageStatsQuery) (UsageStatsQuery, error) {
	now := time.Now().Unix()
	if query.EndTimestamp == 0 {
		query.EndTimestamp = now
	}
	if query.StartTimestamp == 0 {
		query.StartTimestamp = query.EndTimestamp - 7*24*60*60 + 1
	}
	if query.StartTimestamp > query.EndTimestamp {
		return query, errors.New("开始时间不能晚于结束时间")
	}
	if query.EndTimestamp-query.StartTimestamp > usageStatsMaxRangeSeconds {
		return query, errors.New("时间跨度不能超过 90 天")
	}
	if query.Limit <= 0 {
		query.Limit = usageStatsDefaultLimit
	}
	if query.Limit > usageStatsMaxLimit {
		query.Limit = usageStatsMaxLimit
	}
	if query.TrendGranularity == "" {
		if query.EndTimestamp-query.StartTimestamp > 3*24*60*60 {
			query.TrendGranularity = UsageStatsGranularityDay
		} else {
			query.TrendGranularity = UsageStatsGranularityHour
		}
	}
	if query.TrendGranularity != UsageStatsGranularityHour && query.TrendGranularity != UsageStatsGranularityDay {
		return query, errors.New("trend_granularity 仅支持 hour 或 day")
	}
	return query, nil
}

func applyUsageStatsFilters(tx *gorm.DB, query UsageStatsQuery) (*gorm.DB, error) {
	tx = tx.Where("logs.type = ?", LogTypeConsume)
	tx = tx.Where("logs.created_at >= ? AND logs.created_at <= ?", query.StartTimestamp, query.EndTimestamp)
	if query.ModelName != "" {
		modelNamePattern, err := sanitizeLikePattern(query.ModelName)
		if err != nil {
			return tx, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if query.Username != "" {
		tx = tx.Where("logs.username = ?", query.Username)
	}
	if query.Group != "" {
		ensureCommonColumnsInitialized()
		tx = tx.Where("logs."+logGroupCol+" = ?", query.Group)
	}
	if query.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", query.Channel)
	}
	return tx, nil
}

func usageStatsBucketStart(ts int64, granularity string) int64 {
	if granularity == UsageStatsGranularityDay {
		t := time.Unix(ts, 0).Local()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	}
	t := time.Unix(ts, 0).Local()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Unix()
}

func usageStatsBucketStep(granularity string) int64 {
	if granularity == UsageStatsGranularityDay {
		return 24 * 60 * 60
	}
	return 3600
}

func usageStatsBucketLabel(bucketStart int64, granularity string) string {
	layout := "01-02 15:04"
	if granularity == UsageStatsGranularityDay {
		layout = "01-02"
	}
	return time.Unix(bucketStart, 0).Local().Format(layout)
}

func buildUsageStatsTrend(rows []usageStatsTrendRow, query UsageStatsQuery) []UsageStatsTrendPoint {
	step := usageStatsBucketStep(query.TrendGranularity)
	startBucket := usageStatsBucketStart(query.StartTimestamp, query.TrendGranularity)
	endBucket := usageStatsBucketStart(query.EndTimestamp, query.TrendGranularity)
	trendMap := make(map[int64]*UsageStatsTrendPoint)
	for bucket := startBucket; bucket <= endBucket; bucket += step {
		trendMap[bucket] = &UsageStatsTrendPoint{
			BucketStart: bucket,
			Label:       usageStatsBucketLabel(bucket, query.TrendGranularity),
		}
	}
	for _, row := range rows {
		point, ok := trendMap[row.BucketStart]
		if !ok {
			point = &UsageStatsTrendPoint{
				BucketStart: row.BucketStart,
				Label:       usageStatsBucketLabel(row.BucketStart, query.TrendGranularity),
			}
			trendMap[row.BucketStart] = point
		}
		point.Quota += row.Quota
		point.RequestCount += row.RequestCount
		point.PromptTokens += row.PromptTokens
		point.CompletionTokens += row.CompletionTokens
		point.TotalTokens += row.PromptTokens + row.CompletionTokens
	}
	buckets := make([]int64, 0, len(trendMap))
	for bucket := range trendMap {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i] < buckets[j]
	})
	trend := make([]UsageStatsTrendPoint, 0, len(buckets))
	for _, bucket := range buckets {
		trend = append(trend, *trendMap[bucket])
	}
	return trend
}

func buildUsageStatsUserModelDetails(rows []usageStatsBaseRow, rankedUsers []UsageStatsRankItem) []UsageStatsUserModelDetail {
	if len(rankedUsers) == 0 {
		return []UsageStatsUserModelDetail{}
	}
	details := make([]UsageStatsUserModelDetail, 0, len(rows))
	for _, row := range rows {
		details = append(details, UsageStatsUserModelDetail{
			UserId:           row.UserId,
			Username:         row.Username,
			ModelName:        row.ModelName,
			Quota:            row.Quota,
			RequestCount:     row.RequestCount,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.PromptTokens + row.CompletionTokens,
			AverageUseTime:   row.AverageUseTime,
		})
	}
	sort.Slice(details, func(i, j int) bool {
		if details[i].UserId != details[j].UserId {
			return details[i].UserId < details[j].UserId
		}
		if details[i].Quota != details[j].Quota {
			return details[i].Quota > details[j].Quota
		}
		return details[i].ModelName < details[j].ModelName
	})
	return details
}

func usageStatsBucketExpression(step int64, offset int) string {
	return fmt.Sprintf("created_at - ((created_at + %d) %% %d)", offset, step)
}

func GetUsageStats(query UsageStatsQuery) (UsageStatsData, error) {
	var err error
	query, err = normalizeUsageStatsQuery(query)
	if err != nil {
		return UsageStatsData{}, err
	}
	ensureCommonColumnsInitialized()

	data := UsageStatsData{
		StartTimestamp:   query.StartTimestamp,
		EndTimestamp:     query.EndTimestamp,
		TrendGranularity: query.TrendGranularity,
		GeneratedAt:      time.Now().Unix(),
		Ranking:          []UsageStatsRankItem{},
		Trend:            []UsageStatsTrendPoint{},
		Models:           []UsageStatsModelItem{},
		UserModelDetails: []UsageStatsUserModelDetail{},
	}

	baseQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
	if err != nil {
		return data, err
	}

	var summaryRow struct {
		Quota            int64
		RequestCount     int64
		ActiveUserCount  int64
		PromptTokens     int64
		CompletionTokens int64
	}
	if err = baseQuery.Select("COALESCE(SUM(quota), 0) as quota, COUNT(*) as request_count, COUNT(DISTINCT user_id) as active_user_count, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens").Scan(&summaryRow).Error; err != nil {
		return data, err
	}
	data.Summary = UsageStatsSummary{
		Quota:            summaryRow.Quota,
		RequestCount:     summaryRow.RequestCount,
		ActiveUserCount:  summaryRow.ActiveUserCount,
		PromptTokens:     summaryRow.PromptTokens,
		CompletionTokens: summaryRow.CompletionTokens,
		TotalTokens:      summaryRow.PromptTokens + summaryRow.CompletionTokens,
	}

	rankingQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
	if err != nil {
		return data, err
	}
	if err = rankingQuery.Select("user_id, username, COALESCE(SUM(quota), 0) as quota, COUNT(*) as request_count, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(AVG(use_time), 0) as average_use_time, COALESCE(MAX(created_at), 0) as last_request_at").
		Group("user_id, username").
		Order("quota desc, request_count desc, user_id asc").
		Limit(query.Limit).
		Scan(&data.Ranking).Error; err != nil {
		return data, err
	}
	for i := range data.Ranking {
		data.Ranking[i].TotalTokens = data.Ranking[i].PromptTokens + data.Ranking[i].CompletionTokens
	}

	modelQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
	if err != nil {
		return data, err
	}
	if err = modelQuery.Select("model_name, COALESCE(SUM(quota), 0) as quota, COUNT(*) as request_count, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(AVG(use_time), 0) as average_use_time").
		Group("model_name").
		Order("quota desc, request_count desc, model_name asc").
		Limit(query.Limit).
		Scan(&data.Models).Error; err != nil {
		return data, err
	}
	for i := range data.Models {
		data.Models[i].TotalTokens = data.Models[i].PromptTokens + data.Models[i].CompletionTokens
	}

	step := usageStatsBucketStep(query.TrendGranularity)
	_, offset := time.Unix(query.StartTimestamp, 0).Local().Zone()
	bucketExpr := usageStatsBucketExpression(step, offset)
	trendQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
	if err != nil {
		return data, err
	}
	var trendRows []usageStatsTrendRow
	if err = trendQuery.Select(bucketExpr + " as bucket_start, COALESCE(SUM(quota), 0) as quota, COUNT(*) as request_count, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens").
		Group(bucketExpr).
		Order("bucket_start asc").
		Scan(&trendRows).Error; err != nil {
		return data, err
	}
	data.Trend = buildUsageStatsTrend(trendRows, query)

	rankedUserIDs := make([]int, 0, len(data.Ranking))
	for _, item := range data.Ranking {
		rankedUserIDs = append(rankedUserIDs, item.UserId)
	}
	if len(rankedUserIDs) > 0 {
		detailQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
		if err != nil {
			return data, err
		}
		var detailRows []usageStatsBaseRow
		if err = detailQuery.Where("logs.user_id IN ?", rankedUserIDs).
			Select("user_id, username, model_name, COALESCE(SUM(quota), 0) as quota, COUNT(*) as request_count, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens, COALESCE(AVG(use_time), 0) as average_use_time").
			Group("user_id, username, model_name").
			Order("user_id asc, quota desc, model_name asc").
			Scan(&detailRows).Error; err != nil {
			return data, err
		}
		data.UserModelDetails = buildUsageStatsUserModelDetails(detailRows, data.Ranking)
	}

	return data, nil
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
