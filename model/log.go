package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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
const nonBillingAuditLogPattern = `%"non_billing_audit":true%`

func excludeNonBillingAuditLogs(tx *gorm.DB, column string) *gorm.DB {
	return tx.Where("COALESCE("+column+", '') NOT LIKE ?", nonBillingAuditLogPattern)
}

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
			delete(otherMap, "original_group_ratio")
			delete(otherMap, "original_ratio")
			delete(otherMap, "ratio_override")
			delete(otherMap, "has_ratio_override")
			delete(otherMap, "ratio_override_applied")
			delete(otherMap, "user_group_ratio")
			delete(otherMap, "route_model_group_ratio_applied")
			delete(otherMap, "route_model_group_ratio")
			delete(otherMap, "route_model_group_ratio_source")
			delete(otherMap, "route_model_ratio_aggregate_group")
			delete(otherMap, "route_model_ratio_real_group")
			delete(otherMap, "route_model_ratio_model_name")
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
	query = excludeNonBillingAuditLogs(query, "logs.other")
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

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) *Log {
	if !common.LogConsumeEnabled {
		return nil
	}
	if common.IsLogConsumeExcludedUserID(userId) {
		return nil
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
		return nil
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
	return log
}

type RecordTaskBillingLogParams struct {
	UserId           int
	LogType          int
	Content          string
	ChannelId        int
	ModelName        string
	Quota            int
	PromptTokens     int
	CompletionTokens int
	TokenId          int
	Group            string
	UseTimeSeconds   int
	Other            map[string]interface{}
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
		UserId:           params.UserId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             params.LogType,
		Content:          params.Content,
		TokenName:        tokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		Group:            params.Group,
		UseTime:          params.UseTimeSeconds,
		Other:            common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

// MergeConsumeLogOther atomically appends metadata to a task's original
// consume log. The task ID check prevents an incorrect log association even
// if a stale or corrupted consume_log_id is present in task private data.
func MergeConsumeLogOther(logId int, userId int, taskId string, updates map[string]interface{}) (bool, error) {
	if logId <= 0 || userId <= 0 || strings.TrimSpace(taskId) == "" || len(updates) == 0 {
		return false, nil
	}

	var logItem Log
	if err := LOG_DB.Where("id = ? AND user_id = ? AND type = ?", logId, userId, LogTypeConsume).First(&logItem).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	other, err := common.StrToMap(logItem.Other)
	if err != nil {
		return false, err
	}
	storedTaskId, _ := other["task_id"].(string)
	if strings.TrimSpace(storedTaskId) != strings.TrimSpace(taskId) {
		return false, nil
	}
	for key, value := range updates {
		other[key] = value
	}
	encoded, err := common.Marshal(other)
	if err != nil {
		return false, err
	}
	result := LOG_DB.Model(&Log{}).
		Where("id = ? AND other = ?", logItem.Id, logItem.Other).
		Update("other", string(encoded))
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
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
	UsageStatsGranularityHour             = "hour"
	UsageStatsGranularityDay              = "day"
	UsageStatsSectionAll                  = "all"
	UsageStatsSectionUsage                = "usage"
	UsageStatsSectionRecharge             = "recharge"
	UsageStatsSectionSubscriptionPurchase = "subscription_purchase"
	UsageStatsBillingSourceAll            = "all"
	UsageStatsBillingSourceWallet         = "wallet"
	UsageStatsBillingSourceSubscription   = "subscription"
	UsageStatsBillingSourceUnknown        = "unknown"
	usageStatsDefaultLimit                = 50
	usageStatsMaxLimit                    = 200
	usageStatsMaxRangeSeconds             = 90 * 24 * 60 * 60
)

type UsageStatsQuery struct {
	Section                        string
	BillingSource                  string
	StartTimestamp                 int64
	EndTimestamp                   int64
	UserId                         int
	ModelName                      string
	Username                       string
	Group                          string
	Channel                        int
	Limit                          int
	TrendGranularity               string
	RechargePage                   int
	RechargePageSize               int
	RechargeUserId                 int
	RechargeDetailPage             int
	RechargeDetailSize             int
	SubscriptionPurchasePage       int
	SubscriptionPurchasePageSize   int
	SubscriptionPurchaseUserId     int
	SubscriptionPurchaseDetailPage int
	SubscriptionPurchaseDetailSize int
}

type UsageStatsSummary struct {
	Quota                             int64 `json:"quota"`
	RequestCount                      int64 `json:"request_count"`
	ActiveUserCount                   int64 `json:"active_user_count"`
	WalletQuota                       int64 `json:"wallet_quota"`
	WalletRequestCount                int64 `json:"wallet_request_count"`
	SubscriptionQuota                 int64 `json:"subscription_quota"`
	SubscriptionRequestCount          int64 `json:"subscription_request_count"`
	SubscriptionActiveUserCount       int64 `json:"subscription_active_user_count"`
	UnknownQuota                      int64 `json:"unknown_quota"`
	UnknownRequestCount               int64 `json:"unknown_request_count"`
	InputTokens                       int64 `json:"input_tokens"`
	CacheTokens                       int64 `json:"cache_tokens"`
	PromptTokens                      int64 `json:"prompt_tokens"`
	CompletionTokens                  int64 `json:"completion_tokens"`
	TotalTokens                       int64 `json:"total_tokens"`
	ClaudeCacheTTLSubsidyQuota        int64 `json:"claude_cache_ttl_subsidy_quota"`
	ClaudeCacheTTLSubsidyRequestCount int64 `json:"claude_cache_ttl_subsidy_request_count"`
	ClaudeCacheTTLRepricedTokens      int64 `json:"claude_cache_ttl_repriced_tokens"`
	ClaudeCacheTTLUpstream1hTokens    int64 `json:"claude_cache_ttl_upstream_1h_tokens"`
	ClaudeCacheTTLBilled5mTokens      int64 `json:"claude_cache_ttl_billed_5m_tokens"`
}

type UsageStatsRankItem struct {
	UserId                   int     `json:"user_id"`
	Username                 string  `json:"username"`
	Quota                    int64   `json:"quota"`
	RequestCount             int64   `json:"request_count"`
	InputTokens              int64   `json:"input_tokens"`
	CacheTokens              int64   `json:"cache_tokens"`
	PromptTokens             int64   `json:"prompt_tokens"`
	CompletionTokens         int64   `json:"completion_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	AverageUseTime           float64 `json:"average_use_time"`
	LastRequestAt            int64   `json:"last_request_at"`
	WalletQuota              int64   `json:"wallet_quota"`
	WalletRequestCount       int64   `json:"wallet_request_count"`
	SubscriptionQuota        int64   `json:"subscription_quota"`
	SubscriptionRequestCount int64   `json:"subscription_request_count"`
	UnknownQuota             int64   `json:"unknown_quota"`
	UnknownRequestCount      int64   `json:"unknown_request_count"`
}

type UsageStatsTrendPoint struct {
	BucketStart                       int64  `json:"bucket_start"`
	Label                             string `json:"label"`
	Quota                             int64  `json:"quota"`
	RequestCount                      int64  `json:"request_count"`
	InputTokens                       int64  `json:"input_tokens"`
	CacheTokens                       int64  `json:"cache_tokens"`
	PromptTokens                      int64  `json:"prompt_tokens"`
	CompletionTokens                  int64  `json:"completion_tokens"`
	TotalTokens                       int64  `json:"total_tokens"`
	WalletQuota                       int64  `json:"wallet_quota"`
	WalletRequestCount                int64  `json:"wallet_request_count"`
	SubscriptionQuota                 int64  `json:"subscription_quota"`
	SubscriptionRequestCount          int64  `json:"subscription_request_count"`
	UnknownQuota                      int64  `json:"unknown_quota"`
	UnknownRequestCount               int64  `json:"unknown_request_count"`
	ClaudeCacheTTLSubsidyQuota        int64  `json:"claude_cache_ttl_subsidy_quota"`
	ClaudeCacheTTLSubsidyRequestCount int64  `json:"claude_cache_ttl_subsidy_request_count"`
	ClaudeCacheTTLRepricedTokens      int64  `json:"claude_cache_ttl_repriced_tokens"`
}

type UsageStatsModelItem struct {
	ModelName                string  `json:"model_name"`
	Quota                    int64   `json:"quota"`
	RequestCount             int64   `json:"request_count"`
	InputTokens              int64   `json:"input_tokens"`
	CacheTokens              int64   `json:"cache_tokens"`
	PromptTokens             int64   `json:"prompt_tokens"`
	CompletionTokens         int64   `json:"completion_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	AverageUseTime           float64 `json:"average_use_time"`
	WalletQuota              int64   `json:"wallet_quota"`
	WalletRequestCount       int64   `json:"wallet_request_count"`
	SubscriptionQuota        int64   `json:"subscription_quota"`
	SubscriptionRequestCount int64   `json:"subscription_request_count"`
	UnknownQuota             int64   `json:"unknown_quota"`
	UnknownRequestCount      int64   `json:"unknown_request_count"`
}

type UsageStatsUserModelDetail struct {
	UserId                   int     `json:"user_id"`
	Username                 string  `json:"username"`
	ModelName                string  `json:"model_name"`
	Quota                    int64   `json:"quota"`
	RequestCount             int64   `json:"request_count"`
	InputTokens              int64   `json:"input_tokens"`
	CacheTokens              int64   `json:"cache_tokens"`
	PromptTokens             int64   `json:"prompt_tokens"`
	CompletionTokens         int64   `json:"completion_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	AverageUseTime           float64 `json:"average_use_time"`
	WalletQuota              int64   `json:"wallet_quota"`
	WalletRequestCount       int64   `json:"wallet_request_count"`
	SubscriptionQuota        int64   `json:"subscription_quota"`
	SubscriptionRequestCount int64   `json:"subscription_request_count"`
	UnknownQuota             int64   `json:"unknown_quota"`
	UnknownRequestCount      int64   `json:"unknown_request_count"`
}

type UsageStatsRechargeSummary struct {
	Amount      int64   `json:"amount"`
	Money       float64 `json:"money"`
	OrderCount  int64   `json:"order_count"`
	UserCount   int64   `json:"user_count"`
	LastTopUpAt int64   `json:"last_topup_at"`
}

type UsageStatsRechargeRankItem struct {
	UserId      int     `json:"user_id"`
	Username    string  `json:"username"`
	Amount      int64   `json:"amount"`
	Money       float64 `json:"money"`
	OrderCount  int64   `json:"order_count"`
	LastTopUpAt int64   `json:"last_topup_at"`
}

type UsageStatsRechargeRankPage struct {
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
	Total    int64                        `json:"total"`
	Items    []UsageStatsRechargeRankItem `json:"items"`
}

type UsageStatsRechargeDetailItem struct {
	Id            int     `json:"id"`
	UserId        int     `json:"user_id"`
	Username      string  `json:"username"`
	Amount        int64   `json:"amount"`
	Money         float64 `json:"money"`
	TradeNo       string  `json:"trade_no"`
	PaymentMethod string  `json:"payment_method"`
	CreateTime    int64   `json:"create_time"`
	CompleteTime  int64   `json:"complete_time"`
	Status        string  `json:"status"`
}

type UsageStatsRechargeDetailPage struct {
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
	Total    int64                          `json:"total"`
	UserId   int                            `json:"user_id"`
	Items    []UsageStatsRechargeDetailItem `json:"items"`
}

type UsageStatsSubscriptionPurchaseSummary struct {
	Money          float64 `json:"money"`
	OrderCount     int64   `json:"order_count"`
	UserCount      int64   `json:"user_count"`
	PlanCount      int64   `json:"plan_count"`
	LastPurchaseAt int64   `json:"last_purchase_at"`
}

type UsageStatsSubscriptionPurchaseRankItem struct {
	UserId         int     `json:"user_id"`
	Username       string  `json:"username"`
	Money          float64 `json:"money"`
	OrderCount     int64   `json:"order_count"`
	PlanCount      int64   `json:"plan_count"`
	LastPurchaseAt int64   `json:"last_purchase_at"`
}

type UsageStatsSubscriptionPurchaseRankPage struct {
	Page     int                                      `json:"page"`
	PageSize int                                      `json:"page_size"`
	Total    int64                                    `json:"total"`
	Items    []UsageStatsSubscriptionPurchaseRankItem `json:"items"`
}

type UsageStatsSubscriptionPurchaseDetailItem struct {
	Id                 int     `json:"id"`
	UserId             int     `json:"user_id"`
	Username           string  `json:"username"`
	PlanId             int     `json:"plan_id"`
	PlanTitle          string  `json:"plan_title"`
	Money              float64 `json:"money"`
	TradeNo            string  `json:"trade_no"`
	PaymentMethod      string  `json:"payment_method"`
	CreateTime         int64   `json:"create_time"`
	CompleteTime       int64   `json:"complete_time"`
	Status             string  `json:"status"`
	UserSubscriptionId int     `json:"user_subscription_id"`
}

type UsageStatsSubscriptionPurchaseDetailPage struct {
	Page     int                                        `json:"page"`
	PageSize int                                        `json:"page_size"`
	Total    int64                                      `json:"total"`
	UserId   int                                        `json:"user_id"`
	Items    []UsageStatsSubscriptionPurchaseDetailItem `json:"items"`
}

type UsageStatsData struct {
	StartTimestamp              int64                                    `json:"start_timestamp"`
	EndTimestamp                int64                                    `json:"end_timestamp"`
	TrendGranularity            string                                   `json:"trend_granularity"`
	GeneratedAt                 int64                                    `json:"generated_at"`
	Summary                     UsageStatsSummary                        `json:"summary"`
	Ranking                     []UsageStatsRankItem                     `json:"ranking"`
	WalletRanking               []UsageStatsRankItem                     `json:"wallet_ranking"`
	SubscriptionRanking         []UsageStatsRankItem                     `json:"subscription_ranking"`
	Trend                       []UsageStatsTrendPoint                   `json:"trend"`
	Models                      []UsageStatsModelItem                    `json:"models"`
	UserModelDetails            []UsageStatsUserModelDetail              `json:"user_model_details"`
	RechargeSummary             UsageStatsRechargeSummary                `json:"recharge_summary"`
	RechargeRanking             UsageStatsRechargeRankPage               `json:"recharge_ranking"`
	RechargeDetails             UsageStatsRechargeDetailPage             `json:"recharge_details"`
	SubscriptionPurchaseSummary UsageStatsSubscriptionPurchaseSummary    `json:"subscription_purchase_summary"`
	SubscriptionPurchaseRanking UsageStatsSubscriptionPurchaseRankPage   `json:"subscription_purchase_ranking"`
	SubscriptionPurchaseDetails UsageStatsSubscriptionPurchaseDetailPage `json:"subscription_purchase_details"`
}

type usageStatsBaseRow struct {
	UserId                   int
	Username                 string
	ModelName                string
	Quota                    int64
	InputTokens              int64
	CacheTokens              int64
	PromptTokens             int64
	CompletionTokens         int64
	TotalTokens              int64
	RequestCount             int64
	AverageUseTime           float64
	LastRequestAt            int64
	WalletQuota              int64
	WalletRequestCount       int64
	SubscriptionQuota        int64
	SubscriptionRequestCount int64
	UnknownQuota             int64
	UnknownRequestCount      int64
	useTimeTotal             int64
}

type usageStatsTrendRow struct {
	BucketStart                       int64
	Quota                             int64
	RequestCount                      int64
	InputTokens                       int64
	CacheTokens                       int64
	PromptTokens                      int64
	CompletionTokens                  int64
	TotalTokens                       int64
	WalletQuota                       int64
	WalletRequestCount                int64
	SubscriptionQuota                 int64
	SubscriptionRequestCount          int64
	UnknownQuota                      int64
	UnknownRequestCount               int64
	ClaudeCacheTTLSubsidyQuota        int64
	ClaudeCacheTTLSubsidyRequestCount int64
	ClaudeCacheTTLRepricedTokens      int64
}

type usageStatsLogRow struct {
	UserId           int
	Username         string
	ModelName        string
	Quota            int64
	PromptTokens     int64
	CompletionTokens int64
	UseTime          int64
	CreatedAt        int64
	Other            string
}

type usageStatsRechargeSummaryRow struct {
	Amount      int64   `gorm:"column:amount"`
	Money       float64 `gorm:"column:money"`
	OrderCount  int64   `gorm:"column:order_count"`
	UserCount   int64   `gorm:"column:user_count"`
	LastTopUpAt int64   `gorm:"column:last_topup_at"`
}

type usageStatsRechargeRankRow struct {
	UserId      int     `gorm:"column:user_id"`
	Username    string  `gorm:"column:username"`
	Amount      int64   `gorm:"column:amount"`
	Money       float64 `gorm:"column:money"`
	OrderCount  int64   `gorm:"column:order_count"`
	LastTopUpAt int64   `gorm:"column:last_topup_at"`
}

type usageStatsRechargeDetailRow struct {
	Id            int     `gorm:"column:id"`
	UserId        int     `gorm:"column:user_id"`
	Username      string  `gorm:"column:username"`
	Amount        int64   `gorm:"column:amount"`
	Money         float64 `gorm:"column:money"`
	TradeNo       string  `gorm:"column:trade_no"`
	PaymentMethod string  `gorm:"column:payment_method"`
	CreateTime    int64   `gorm:"column:create_time"`
	CompleteTime  int64   `gorm:"column:complete_time"`
	Status        string  `gorm:"column:status"`
}

type usageStatsSubscriptionPurchaseSummaryRow struct {
	Money          float64 `gorm:"column:money"`
	OrderCount     int64   `gorm:"column:order_count"`
	UserCount      int64   `gorm:"column:user_count"`
	PlanCount      int64   `gorm:"column:plan_count"`
	LastPurchaseAt int64   `gorm:"column:last_purchase_at"`
}

type usageStatsSubscriptionPurchaseRankRow struct {
	UserId         int     `gorm:"column:user_id"`
	Username       string  `gorm:"column:username"`
	Money          float64 `gorm:"column:money"`
	OrderCount     int64   `gorm:"column:order_count"`
	PlanCount      int64   `gorm:"column:plan_count"`
	LastPurchaseAt int64   `gorm:"column:last_purchase_at"`
}

type usageStatsSubscriptionPurchaseDetailRow struct {
	Id                 int     `gorm:"column:id"`
	UserId             int     `gorm:"column:user_id"`
	Username           string  `gorm:"column:username"`
	PlanId             int     `gorm:"column:plan_id"`
	PlanTitle          string  `gorm:"column:plan_title"`
	Money              float64 `gorm:"column:money"`
	TradeNo            string  `gorm:"column:trade_no"`
	PaymentMethod      string  `gorm:"column:payment_method"`
	CreateTime         int64   `gorm:"column:create_time"`
	CompleteTime       int64   `gorm:"column:complete_time"`
	Status             string  `gorm:"column:status"`
	UserSubscriptionId int     `gorm:"column:user_subscription_id"`
}

type usageStatsTokenBreakdown struct {
	InputTokens      int64
	CacheTokens      int64
	CompletionTokens int64
	TotalTokens      int64
}

type usageStatsClaudeCacheTTLSubsidy struct {
	Quota            int64
	RequestCount     int64
	RepricedTokens   int64
	Upstream1hTokens int64
	Billed5mTokens   int64
}

func normalizeUsageStatsQuery(query UsageStatsQuery) (UsageStatsQuery, error) {
	if query.Section == "" {
		query.Section = UsageStatsSectionAll
	}
	switch query.Section {
	case UsageStatsSectionAll, UsageStatsSectionUsage, UsageStatsSectionRecharge, UsageStatsSectionSubscriptionPurchase:
	default:
		return query, errors.New("section 仅支持 all、usage、recharge 或 subscription_purchase")
	}
	if query.BillingSource == "" {
		query.BillingSource = UsageStatsBillingSourceAll
	}
	switch query.BillingSource {
	case UsageStatsBillingSourceAll, UsageStatsBillingSourceWallet, UsageStatsBillingSourceSubscription, UsageStatsBillingSourceUnknown:
	default:
		return query, errors.New("billing_source 仅支持 all、wallet、subscription 或 unknown")
	}
	nowTime := time.Now()
	if query.EndTimestamp == 0 {
		query.EndTimestamp = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 23, 59, 59, 0, nowTime.Location()).Unix()
	}
	if query.StartTimestamp == 0 {
		startTime := time.Unix(query.EndTimestamp, 0).Local()
		query.StartTimestamp = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location()).Unix()
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
	if query.RechargePage <= 0 {
		query.RechargePage = 1
	}
	if query.RechargePageSize <= 0 {
		query.RechargePageSize = 20
	}
	if query.RechargePageSize > usageStatsMaxLimit {
		query.RechargePageSize = usageStatsMaxLimit
	}
	if query.RechargeDetailPage <= 0 {
		query.RechargeDetailPage = 1
	}
	if query.RechargeDetailSize <= 0 {
		query.RechargeDetailSize = 20
	}
	if query.RechargeDetailSize > usageStatsMaxLimit {
		query.RechargeDetailSize = usageStatsMaxLimit
	}
	if query.SubscriptionPurchasePage <= 0 {
		query.SubscriptionPurchasePage = 1
	}
	if query.SubscriptionPurchasePageSize <= 0 {
		query.SubscriptionPurchasePageSize = 20
	}
	if query.SubscriptionPurchasePageSize > usageStatsMaxLimit {
		query.SubscriptionPurchasePageSize = usageStatsMaxLimit
	}
	if query.SubscriptionPurchaseDetailPage <= 0 {
		query.SubscriptionPurchaseDetailPage = 1
	}
	if query.SubscriptionPurchaseDetailSize <= 0 {
		query.SubscriptionPurchaseDetailSize = 20
	}
	if query.SubscriptionPurchaseDetailSize > usageStatsMaxLimit {
		query.SubscriptionPurchaseDetailSize = usageStatsMaxLimit
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

func usageStatsIncludesSection(query UsageStatsQuery, section string) bool {
	return query.Section == UsageStatsSectionAll || query.Section == section
}

func applyUsageStatsRechargeFilters(tx *gorm.DB, query UsageStatsQuery) (*gorm.DB, error) {
	tx = tx.Where("top_ups.status = ?", common.TopUpStatusSuccess)
	tx = tx.Where("top_ups.complete_time >= ? AND top_ups.complete_time <= ?", query.StartTimestamp, query.EndTimestamp)
	tx = tx.Where("subscription_orders.id IS NULL")
	if query.Username != "" {
		tx = tx.Where("users.username = ?", query.Username)
	}
	if query.Group != "" {
		ensureCommonColumnsInitialized()
		tx = tx.Where("users."+commonGroupCol+" = ?", query.Group)
	}
	return tx, nil
}

func applyUsageStatsSubscriptionPurchaseFilters(tx *gorm.DB, query UsageStatsQuery) (*gorm.DB, error) {
	tx = tx.Where("subscription_orders.status = ?", common.TopUpStatusSuccess)
	tx = tx.Where("(subscription_orders.invalidated_at = ? OR subscription_orders.invalidated_at IS NULL)", 0)
	tx = tx.Where("subscription_orders.complete_time >= ? AND subscription_orders.complete_time <= ?", query.StartTimestamp, query.EndTimestamp)
	if query.Username != "" {
		tx = tx.Where("users.username = ?", query.Username)
	}
	if query.Group != "" {
		ensureCommonColumnsInitialized()
		tx = tx.Where("users."+commonGroupCol+" = ?", query.Group)
	}
	return tx, nil
}

func applyUsageStatsFilters(tx *gorm.DB, query UsageStatsQuery) (*gorm.DB, error) {
	tx = tx.Where("logs.type = ?", LogTypeConsume)
	tx = excludeNonBillingAuditLogs(tx, "logs.other")
	tx = tx.Where("logs.created_at >= ? AND logs.created_at <= ?", query.StartTimestamp, query.EndTimestamp)
	if query.UserId > 0 {
		tx = tx.Where("logs.user_id = ?", query.UserId)
	}
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

func usageStatsOtherInt64(other map[string]interface{}, key string) int64 {
	value, ok := other[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int:
		if v > 0 {
			return int64(v)
		}
	case int64:
		if v > 0 {
			return v
		}
	case float64:
		if v > 0 {
			return int64(v)
		}
	case string:
		var parsed int64
		if _, err := fmt.Sscan(v, &parsed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func usageStatsOtherBool(other map[string]interface{}, key string) bool {
	value, ok := other[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case float64:
		return v != 0
	}
	return false
}

func usageStatsCacheWriteTokens(other map[string]interface{}) int64 {
	cacheWriteTokens := usageStatsOtherInt64(other, "cache_write_tokens")
	if cacheWriteTokens > 0 {
		return cacheWriteTokens
	}
	cacheCreationTokens := usageStatsOtherInt64(other, "cache_creation_tokens")
	cacheCreationTokens5m := usageStatsOtherInt64(other, "cache_creation_tokens_5m")
	cacheCreationTokens1h := usageStatsOtherInt64(other, "cache_creation_tokens_1h")
	if cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0 {
		splitCacheWriteTokens := cacheCreationTokens5m + cacheCreationTokens1h
		if cacheCreationTokens > splitCacheWriteTokens {
			return cacheCreationTokens
		}
		return splitCacheWriteTokens
	}
	return cacheCreationTokens
}

func usageStatsOtherFromLog(row usageStatsLogRow) map[string]interface{} {
	var other map[string]interface{}
	if row.Other != "" {
		_ = common.UnmarshalJsonStr(row.Other, &other)
	}
	return other
}

func usageStatsBillingSourceFromOther(other map[string]interface{}) string {
	value, ok := other["billing_source"].(string)
	if !ok {
		return UsageStatsBillingSourceUnknown
	}
	switch strings.TrimSpace(value) {
	case UsageStatsBillingSourceWallet:
		return UsageStatsBillingSourceWallet
	case UsageStatsBillingSourceSubscription:
		return UsageStatsBillingSourceSubscription
	default:
		return UsageStatsBillingSourceUnknown
	}
}

func usageStatsMatchesBillingSource(query UsageStatsQuery, source string) bool {
	return query.BillingSource == UsageStatsBillingSourceAll || query.BillingSource == source
}

func usageStatsTokenBreakdownFromLog(row usageStatsLogRow) usageStatsTokenBreakdown {
	return usageStatsTokenBreakdownFromOther(row, usageStatsOtherFromLog(row))
}

func usageStatsTokenBreakdownFromOther(row usageStatsLogRow, other map[string]interface{}) usageStatsTokenBreakdown {
	promptTokens := row.PromptTokens
	completionTokens := row.CompletionTokens

	cacheReadTokens := usageStatsOtherInt64(other, "cache_tokens")
	cacheWriteTokens := usageStatsCacheWriteTokens(other)
	isAnthropic := usageStatsOtherBool(other, "claude")
	if semantic, ok := other["usage_semantic"].(string); ok && semantic == "anthropic" {
		isAnthropic = true
	}

	inputTokens := promptTokens
	if isAnthropic {
		inputTokens = promptTokens + cacheWriteTokens
	} else {
		rawInputTokens := usageStatsOtherInt64(other, "input_tokens_total")
		if rawInputTokens <= 0 {
			rawInputTokens = promptTokens
		}
		inputTokens = rawInputTokens - cacheReadTokens
		if inputTokens < 0 {
			inputTokens = 0
		}
		inputTokens += cacheWriteTokens
	}

	totalTokens := inputTokens + cacheReadTokens + completionTokens
	return usageStatsTokenBreakdown{
		InputTokens:      inputTokens,
		CacheTokens:      cacheReadTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}
}

func usageStatsClaudeCacheTTLSubsidyFromOther(other map[string]interface{}) usageStatsClaudeCacheTTLSubsidy {
	if !usageStatsOtherBool(other, "claude_cache_ttl_billing_compat") {
		return usageStatsClaudeCacheTTLSubsidy{}
	}
	subsidy := usageStatsClaudeCacheTTLSubsidy{
		Quota:            usageStatsOtherInt64(other, "claude_cache_ttl_subsidy_quota"),
		RepricedTokens:   usageStatsOtherInt64(other, "claude_cache_ttl_repriced_tokens"),
		Upstream1hTokens: usageStatsOtherInt64(other, "claude_cache_ttl_upstream_cache_creation_tokens_1h"),
		Billed5mTokens:   usageStatsOtherInt64(other, "claude_cache_ttl_billed_cache_creation_tokens_5m"),
	}
	if subsidy.Quota > 0 || subsidy.RepricedTokens > 0 || subsidy.Upstream1hTokens > 0 || subsidy.Billed5mTokens > 0 {
		subsidy.RequestCount = 1
	}
	return subsidy
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
		point.InputTokens += row.InputTokens
		point.CacheTokens += row.CacheTokens
		point.PromptTokens += row.InputTokens
		point.CompletionTokens += row.CompletionTokens
		point.TotalTokens += row.TotalTokens
		point.WalletQuota += row.WalletQuota
		point.WalletRequestCount += row.WalletRequestCount
		point.SubscriptionQuota += row.SubscriptionQuota
		point.SubscriptionRequestCount += row.SubscriptionRequestCount
		point.UnknownQuota += row.UnknownQuota
		point.UnknownRequestCount += row.UnknownRequestCount
		point.ClaudeCacheTTLSubsidyQuota += row.ClaudeCacheTTLSubsidyQuota
		point.ClaudeCacheTTLSubsidyRequestCount += row.ClaudeCacheTTLSubsidyRequestCount
		point.ClaudeCacheTTLRepricedTokens += row.ClaudeCacheTTLRepricedTokens
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
			UserId:                   row.UserId,
			Username:                 row.Username,
			ModelName:                row.ModelName,
			Quota:                    row.Quota,
			RequestCount:             row.RequestCount,
			InputTokens:              row.InputTokens,
			CacheTokens:              row.CacheTokens,
			PromptTokens:             row.InputTokens,
			CompletionTokens:         row.CompletionTokens,
			TotalTokens:              row.TotalTokens,
			AverageUseTime:           row.AverageUseTime,
			WalletQuota:              row.WalletQuota,
			WalletRequestCount:       row.WalletRequestCount,
			SubscriptionQuota:        row.SubscriptionQuota,
			SubscriptionRequestCount: row.SubscriptionRequestCount,
			UnknownQuota:             row.UnknownQuota,
			UnknownRequestCount:      row.UnknownRequestCount,
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

func newUsageStatsRechargeBaseQuery(query UsageStatsQuery) (*gorm.DB, error) {
	tx := DB.Table("top_ups").
		Joins("LEFT JOIN users ON users.id = top_ups.user_id").
		Joins("LEFT JOIN subscription_orders ON subscription_orders.trade_no = top_ups.trade_no AND subscription_orders.status = ?", common.TopUpStatusSuccess)
	return applyUsageStatsRechargeFilters(tx, query)
}

func newUsageStatsSubscriptionPurchaseBaseQuery(query UsageStatsQuery) (*gorm.DB, error) {
	tx := DB.Table("subscription_orders").
		Joins("LEFT JOIN users ON users.id = subscription_orders.user_id").
		Joins("LEFT JOIN subscription_plans ON subscription_plans.id = subscription_orders.plan_id")
	return applyUsageStatsSubscriptionPurchaseFilters(tx, query)
}

func populateUsageStatsRechargeSummary(data *UsageStatsData, query UsageStatsQuery) error {
	baseQuery, err := newUsageStatsRechargeBaseQuery(query)
	if err != nil {
		return err
	}
	var row usageStatsRechargeSummaryRow
	if err := baseQuery.
		Select("COALESCE(sum(top_ups.amount), 0) as amount, COALESCE(sum(top_ups.money), 0) as money, count(*) as order_count, count(distinct top_ups.user_id) as user_count, COALESCE(max(top_ups.complete_time), 0) as last_topup_at").
		Scan(&row).Error; err != nil {
		return err
	}
	data.RechargeSummary = UsageStatsRechargeSummary{
		Amount:      row.Amount,
		Money:       row.Money,
		OrderCount:  row.OrderCount,
		UserCount:   row.UserCount,
		LastTopUpAt: row.LastTopUpAt,
	}
	return nil
}

func populateUsageStatsRechargeRanking(data *UsageStatsData, query UsageStatsQuery) error {
	data.RechargeRanking = UsageStatsRechargeRankPage{
		Page:     query.RechargePage,
		PageSize: query.RechargePageSize,
		Items:    []UsageStatsRechargeRankItem{},
	}

	groupedQuery, err := newUsageStatsRechargeBaseQuery(query)
	if err != nil {
		return err
	}
	groupedQuery = groupedQuery.Select("top_ups.user_id").Group("top_ups.user_id")
	if err := DB.Table("(?) as recharge_users", groupedQuery).Count(&data.RechargeRanking.Total).Error; err != nil {
		return err
	}
	if data.RechargeRanking.Total == 0 {
		return nil
	}

	rankQuery, err := newUsageStatsRechargeBaseQuery(query)
	if err != nil {
		return err
	}
	var rows []usageStatsRechargeRankRow
	offset := (query.RechargePage - 1) * query.RechargePageSize
	if err := rankQuery.
		Select("top_ups.user_id as user_id, COALESCE(users.username, '') as username, COALESCE(sum(top_ups.amount), 0) as amount, COALESCE(sum(top_ups.money), 0) as money, count(*) as order_count, COALESCE(max(top_ups.complete_time), 0) as last_topup_at").
		Group("top_ups.user_id, users.username").
		Order("money desc, amount desc, order_count desc, user_id asc").
		Limit(query.RechargePageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		data.RechargeRanking.Items = append(data.RechargeRanking.Items, UsageStatsRechargeRankItem{
			UserId:      row.UserId,
			Username:    row.Username,
			Amount:      row.Amount,
			Money:       row.Money,
			OrderCount:  row.OrderCount,
			LastTopUpAt: row.LastTopUpAt,
		})
	}
	return nil
}

func populateUsageStatsRechargeDetails(data *UsageStatsData, query UsageStatsQuery) error {
	data.RechargeDetails = UsageStatsRechargeDetailPage{
		Page:     query.RechargeDetailPage,
		PageSize: query.RechargeDetailSize,
		UserId:   query.RechargeUserId,
		Items:    []UsageStatsRechargeDetailItem{},
	}
	if query.RechargeUserId <= 0 {
		return nil
	}

	countQuery, err := newUsageStatsRechargeBaseQuery(query)
	if err != nil {
		return err
	}
	countQuery = countQuery.Where("top_ups.user_id = ?", query.RechargeUserId)
	if err := countQuery.Count(&data.RechargeDetails.Total).Error; err != nil {
		return err
	}
	if data.RechargeDetails.Total == 0 {
		return nil
	}

	detailQuery, err := newUsageStatsRechargeBaseQuery(query)
	if err != nil {
		return err
	}
	var rows []usageStatsRechargeDetailRow
	offset := (query.RechargeDetailPage - 1) * query.RechargeDetailSize
	if err := detailQuery.
		Where("top_ups.user_id = ?", query.RechargeUserId).
		Select("top_ups.id as id, top_ups.user_id as user_id, COALESCE(users.username, '') as username, top_ups.amount as amount, top_ups.money as money, top_ups.trade_no as trade_no, top_ups.payment_method as payment_method, top_ups.create_time as create_time, top_ups.complete_time as complete_time, top_ups.status as status").
		Order("top_ups.complete_time desc, top_ups.id desc").
		Limit(query.RechargeDetailSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		data.RechargeDetails.Items = append(data.RechargeDetails.Items, UsageStatsRechargeDetailItem{
			Id:            row.Id,
			UserId:        row.UserId,
			Username:      row.Username,
			Amount:        row.Amount,
			Money:         row.Money,
			TradeNo:       row.TradeNo,
			PaymentMethod: row.PaymentMethod,
			CreateTime:    row.CreateTime,
			CompleteTime:  row.CompleteTime,
			Status:        row.Status,
		})
	}
	return nil
}

func populateUsageStatsRecharge(data *UsageStatsData, query UsageStatsQuery) error {
	if err := populateUsageStatsRechargeSummary(data, query); err != nil {
		return err
	}
	if err := populateUsageStatsRechargeRanking(data, query); err != nil {
		return err
	}
	return populateUsageStatsRechargeDetails(data, query)
}

func populateUsageStatsSubscriptionPurchaseSummary(data *UsageStatsData, query UsageStatsQuery) error {
	baseQuery, err := newUsageStatsSubscriptionPurchaseBaseQuery(query)
	if err != nil {
		return err
	}
	var row usageStatsSubscriptionPurchaseSummaryRow
	if err := baseQuery.
		Select("COALESCE(sum(subscription_orders.money), 0) as money, count(*) as order_count, count(distinct subscription_orders.user_id) as user_count, count(distinct subscription_orders.plan_id) as plan_count, COALESCE(max(subscription_orders.complete_time), 0) as last_purchase_at").
		Scan(&row).Error; err != nil {
		return err
	}
	data.SubscriptionPurchaseSummary = UsageStatsSubscriptionPurchaseSummary{
		Money:          row.Money,
		OrderCount:     row.OrderCount,
		UserCount:      row.UserCount,
		PlanCount:      row.PlanCount,
		LastPurchaseAt: row.LastPurchaseAt,
	}
	return nil
}

func populateUsageStatsSubscriptionPurchaseRanking(data *UsageStatsData, query UsageStatsQuery) error {
	data.SubscriptionPurchaseRanking = UsageStatsSubscriptionPurchaseRankPage{
		Page:     query.SubscriptionPurchasePage,
		PageSize: query.SubscriptionPurchasePageSize,
		Items:    []UsageStatsSubscriptionPurchaseRankItem{},
	}

	groupedQuery, err := newUsageStatsSubscriptionPurchaseBaseQuery(query)
	if err != nil {
		return err
	}
	groupedQuery = groupedQuery.Select("subscription_orders.user_id").Group("subscription_orders.user_id")
	if err := DB.Table("(?) as subscription_purchase_users", groupedQuery).Count(&data.SubscriptionPurchaseRanking.Total).Error; err != nil {
		return err
	}
	if data.SubscriptionPurchaseRanking.Total == 0 {
		return nil
	}

	rankQuery, err := newUsageStatsSubscriptionPurchaseBaseQuery(query)
	if err != nil {
		return err
	}
	var rows []usageStatsSubscriptionPurchaseRankRow
	offset := (query.SubscriptionPurchasePage - 1) * query.SubscriptionPurchasePageSize
	if err := rankQuery.
		Select("subscription_orders.user_id as user_id, COALESCE(users.username, '') as username, COALESCE(sum(subscription_orders.money), 0) as money, count(*) as order_count, count(distinct subscription_orders.plan_id) as plan_count, COALESCE(max(subscription_orders.complete_time), 0) as last_purchase_at").
		Group("subscription_orders.user_id, users.username").
		Order("money desc, order_count desc, plan_count desc, user_id asc").
		Limit(query.SubscriptionPurchasePageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		data.SubscriptionPurchaseRanking.Items = append(data.SubscriptionPurchaseRanking.Items, UsageStatsSubscriptionPurchaseRankItem{
			UserId:         row.UserId,
			Username:       row.Username,
			Money:          row.Money,
			OrderCount:     row.OrderCount,
			PlanCount:      row.PlanCount,
			LastPurchaseAt: row.LastPurchaseAt,
		})
	}
	return nil
}

func populateUsageStatsSubscriptionPurchaseDetails(data *UsageStatsData, query UsageStatsQuery) error {
	data.SubscriptionPurchaseDetails = UsageStatsSubscriptionPurchaseDetailPage{
		Page:     query.SubscriptionPurchaseDetailPage,
		PageSize: query.SubscriptionPurchaseDetailSize,
		UserId:   query.SubscriptionPurchaseUserId,
		Items:    []UsageStatsSubscriptionPurchaseDetailItem{},
	}
	if query.SubscriptionPurchaseUserId <= 0 {
		return nil
	}

	countQuery, err := newUsageStatsSubscriptionPurchaseBaseQuery(query)
	if err != nil {
		return err
	}
	countQuery = countQuery.Where("subscription_orders.user_id = ?", query.SubscriptionPurchaseUserId)
	if err := countQuery.Count(&data.SubscriptionPurchaseDetails.Total).Error; err != nil {
		return err
	}
	if data.SubscriptionPurchaseDetails.Total == 0 {
		return nil
	}

	detailQuery, err := newUsageStatsSubscriptionPurchaseBaseQuery(query)
	if err != nil {
		return err
	}
	var rows []usageStatsSubscriptionPurchaseDetailRow
	offset := (query.SubscriptionPurchaseDetailPage - 1) * query.SubscriptionPurchaseDetailSize
	if err := detailQuery.
		Where("subscription_orders.user_id = ?", query.SubscriptionPurchaseUserId).
		Select("subscription_orders.id as id, subscription_orders.user_id as user_id, COALESCE(users.username, '') as username, subscription_orders.plan_id as plan_id, COALESCE(subscription_plans.title, '') as plan_title, subscription_orders.money as money, subscription_orders.trade_no as trade_no, subscription_orders.payment_method as payment_method, subscription_orders.create_time as create_time, subscription_orders.complete_time as complete_time, subscription_orders.status as status, subscription_orders.user_subscription_id as user_subscription_id").
		Order("subscription_orders.complete_time desc, subscription_orders.id desc").
		Limit(query.SubscriptionPurchaseDetailSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		data.SubscriptionPurchaseDetails.Items = append(data.SubscriptionPurchaseDetails.Items, UsageStatsSubscriptionPurchaseDetailItem{
			Id:                 row.Id,
			UserId:             row.UserId,
			Username:           row.Username,
			PlanId:             row.PlanId,
			PlanTitle:          row.PlanTitle,
			Money:              row.Money,
			TradeNo:            row.TradeNo,
			PaymentMethod:      row.PaymentMethod,
			CreateTime:         row.CreateTime,
			CompleteTime:       row.CompleteTime,
			Status:             row.Status,
			UserSubscriptionId: row.UserSubscriptionId,
		})
	}
	return nil
}

func populateUsageStatsSubscriptionPurchase(data *UsageStatsData, query UsageStatsQuery) error {
	if err := populateUsageStatsSubscriptionPurchaseSummary(data, query); err != nil {
		return err
	}
	if err := populateUsageStatsSubscriptionPurchaseRanking(data, query); err != nil {
		return err
	}
	return populateUsageStatsSubscriptionPurchaseDetails(data, query)
}

func addUsageStatsBaseRow(target *usageStatsBaseRow, row usageStatsLogRow, tokens usageStatsTokenBreakdown, source string) {
	target.Quota += row.Quota
	target.RequestCount++
	target.InputTokens += tokens.InputTokens
	target.CacheTokens += tokens.CacheTokens
	target.PromptTokens = target.InputTokens
	target.CompletionTokens += tokens.CompletionTokens
	target.TotalTokens += tokens.TotalTokens
	target.useTimeTotal += row.UseTime
	switch source {
	case UsageStatsBillingSourceWallet:
		target.WalletQuota += row.Quota
		target.WalletRequestCount++
	case UsageStatsBillingSourceSubscription:
		target.SubscriptionQuota += row.Quota
		target.SubscriptionRequestCount++
	default:
		target.UnknownQuota += row.Quota
		target.UnknownRequestCount++
	}
	if row.CreatedAt > target.LastRequestAt {
		target.LastRequestAt = row.CreatedAt
		target.Username = row.Username
	}
}

func finalizeUsageStatsBaseRow(row *usageStatsBaseRow) {
	row.PromptTokens = row.InputTokens
	if row.RequestCount > 0 {
		row.AverageUseTime = float64(row.useTimeTotal) / float64(row.RequestCount)
	}
}

func usageStatsRankItemFromRow(row usageStatsBaseRow) UsageStatsRankItem {
	return UsageStatsRankItem{
		UserId:                   row.UserId,
		Username:                 row.Username,
		Quota:                    row.Quota,
		RequestCount:             row.RequestCount,
		InputTokens:              row.InputTokens,
		CacheTokens:              row.CacheTokens,
		PromptTokens:             row.InputTokens,
		CompletionTokens:         row.CompletionTokens,
		TotalTokens:              row.TotalTokens,
		AverageUseTime:           row.AverageUseTime,
		LastRequestAt:            row.LastRequestAt,
		WalletQuota:              row.WalletQuota,
		WalletRequestCount:       row.WalletRequestCount,
		SubscriptionQuota:        row.SubscriptionQuota,
		SubscriptionRequestCount: row.SubscriptionRequestCount,
		UnknownQuota:             row.UnknownQuota,
		UnknownRequestCount:      row.UnknownRequestCount,
	}
}

func usageStatsModelItemFromRow(row usageStatsBaseRow) UsageStatsModelItem {
	return UsageStatsModelItem{
		ModelName:                row.ModelName,
		Quota:                    row.Quota,
		RequestCount:             row.RequestCount,
		InputTokens:              row.InputTokens,
		CacheTokens:              row.CacheTokens,
		PromptTokens:             row.InputTokens,
		CompletionTokens:         row.CompletionTokens,
		TotalTokens:              row.TotalTokens,
		AverageUseTime:           row.AverageUseTime,
		WalletQuota:              row.WalletQuota,
		WalletRequestCount:       row.WalletRequestCount,
		SubscriptionQuota:        row.SubscriptionQuota,
		SubscriptionRequestCount: row.SubscriptionRequestCount,
		UnknownQuota:             row.UnknownQuota,
		UnknownRequestCount:      row.UnknownRequestCount,
	}
}

func usageStatsSortedUserRows(rows map[int]*usageStatsBaseRow, limit int) []usageStatsBaseRow {
	result := make([]usageStatsBaseRow, 0, len(rows))
	for _, row := range rows {
		finalizeUsageStatsBaseRow(row)
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Quota != result[j].Quota {
			return result[i].Quota > result[j].Quota
		}
		if result[i].RequestCount != result[j].RequestCount {
			return result[i].RequestCount > result[j].RequestCount
		}
		return result[i].UserId < result[j].UserId
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func usageStatsAddSummarySource(summary *UsageStatsSummary, source string, quota int64) {
	switch source {
	case UsageStatsBillingSourceWallet:
		summary.WalletQuota += quota
		summary.WalletRequestCount++
	case UsageStatsBillingSourceSubscription:
		summary.SubscriptionQuota += quota
		summary.SubscriptionRequestCount++
	default:
		summary.UnknownQuota += quota
		summary.UnknownRequestCount++
	}
}

func usageStatsAddTrendSource(row *usageStatsTrendRow, source string, quota int64) {
	switch source {
	case UsageStatsBillingSourceWallet:
		row.WalletQuota += quota
		row.WalletRequestCount++
	case UsageStatsBillingSourceSubscription:
		row.SubscriptionQuota += quota
		row.SubscriptionRequestCount++
	default:
		row.UnknownQuota += quota
		row.UnknownRequestCount++
	}
}

func populateUsageStatsUsage(data *UsageStatsData, query UsageStatsQuery) error {
	baseQuery, err := applyUsageStatsFilters(LOG_DB.Table("logs"), query)
	if err != nil {
		return err
	}
	var logRows []usageStatsLogRow
	if err = baseQuery.Select("user_id, username, model_name, quota, prompt_tokens, completion_tokens, use_time, created_at, other").Scan(&logRows).Error; err != nil {
		return err
	}

	activeUsers := make(map[int]struct{})
	subscriptionActiveUsers := make(map[int]struct{})
	rankingMap := make(map[int]*usageStatsBaseRow)
	walletRankingMap := make(map[int]*usageStatsBaseRow)
	subscriptionRankingMap := make(map[int]*usageStatsBaseRow)
	modelMap := make(map[string]*usageStatsBaseRow)
	detailMap := make(map[string]*usageStatsBaseRow)
	trendMap := make(map[int64]*usageStatsTrendRow)

	for _, row := range logRows {
		other := usageStatsOtherFromLog(row)
		source := usageStatsBillingSourceFromOther(other)
		if !usageStatsMatchesBillingSource(query, source) {
			continue
		}
		tokens := usageStatsTokenBreakdownFromOther(row, other)
		claudeCacheTTLSubsidy := usageStatsClaudeCacheTTLSubsidyFromOther(other)
		data.Summary.Quota += row.Quota
		data.Summary.RequestCount++
		data.Summary.InputTokens += tokens.InputTokens
		data.Summary.CacheTokens += tokens.CacheTokens
		data.Summary.PromptTokens += tokens.InputTokens
		data.Summary.CompletionTokens += tokens.CompletionTokens
		data.Summary.TotalTokens += tokens.TotalTokens
		data.Summary.ClaudeCacheTTLSubsidyQuota += claudeCacheTTLSubsidy.Quota
		data.Summary.ClaudeCacheTTLSubsidyRequestCount += claudeCacheTTLSubsidy.RequestCount
		data.Summary.ClaudeCacheTTLRepricedTokens += claudeCacheTTLSubsidy.RepricedTokens
		data.Summary.ClaudeCacheTTLUpstream1hTokens += claudeCacheTTLSubsidy.Upstream1hTokens
		data.Summary.ClaudeCacheTTLBilled5mTokens += claudeCacheTTLSubsidy.Billed5mTokens
		usageStatsAddSummarySource(&data.Summary, source, row.Quota)
		activeUsers[row.UserId] = struct{}{}
		if source == UsageStatsBillingSourceSubscription && row.Quota > 0 {
			subscriptionActiveUsers[row.UserId] = struct{}{}
		}

		rankingRow := rankingMap[row.UserId]
		if rankingRow == nil {
			rankingRow = &usageStatsBaseRow{UserId: row.UserId, Username: row.Username}
			rankingMap[row.UserId] = rankingRow
		}
		addUsageStatsBaseRow(rankingRow, row, tokens, source)
		if source == UsageStatsBillingSourceWallet && row.Quota > 0 {
			walletRow := walletRankingMap[row.UserId]
			if walletRow == nil {
				walletRow = &usageStatsBaseRow{UserId: row.UserId, Username: row.Username}
				walletRankingMap[row.UserId] = walletRow
			}
			addUsageStatsBaseRow(walletRow, row, tokens, source)
		}
		if source == UsageStatsBillingSourceSubscription && row.Quota > 0 {
			subscriptionRow := subscriptionRankingMap[row.UserId]
			if subscriptionRow == nil {
				subscriptionRow = &usageStatsBaseRow{UserId: row.UserId, Username: row.Username}
				subscriptionRankingMap[row.UserId] = subscriptionRow
			}
			addUsageStatsBaseRow(subscriptionRow, row, tokens, source)
		}

		modelRow := modelMap[row.ModelName]
		if modelRow == nil {
			modelRow = &usageStatsBaseRow{ModelName: row.ModelName}
			modelMap[row.ModelName] = modelRow
		}
		addUsageStatsBaseRow(modelRow, row, tokens, source)

		detailKey := fmt.Sprintf("%d\x00%s", row.UserId, row.ModelName)
		detailRow := detailMap[detailKey]
		if detailRow == nil {
			detailRow = &usageStatsBaseRow{UserId: row.UserId, Username: row.Username, ModelName: row.ModelName}
			detailMap[detailKey] = detailRow
		}
		addUsageStatsBaseRow(detailRow, row, tokens, source)

		bucketStart := usageStatsBucketStart(row.CreatedAt, query.TrendGranularity)
		trendRow := trendMap[bucketStart]
		if trendRow == nil {
			trendRow = &usageStatsTrendRow{BucketStart: bucketStart}
			trendMap[bucketStart] = trendRow
		}
		trendRow.Quota += row.Quota
		trendRow.RequestCount++
		trendRow.InputTokens += tokens.InputTokens
		trendRow.CacheTokens += tokens.CacheTokens
		trendRow.PromptTokens = trendRow.InputTokens
		trendRow.CompletionTokens += tokens.CompletionTokens
		trendRow.TotalTokens += tokens.TotalTokens
		trendRow.ClaudeCacheTTLSubsidyQuota += claudeCacheTTLSubsidy.Quota
		trendRow.ClaudeCacheTTLSubsidyRequestCount += claudeCacheTTLSubsidy.RequestCount
		trendRow.ClaudeCacheTTLRepricedTokens += claudeCacheTTLSubsidy.RepricedTokens
		usageStatsAddTrendSource(trendRow, source, row.Quota)
	}
	data.Summary.ActiveUserCount = int64(len(activeUsers))
	data.Summary.SubscriptionActiveUserCount = int64(len(subscriptionActiveUsers))

	for _, row := range usageStatsSortedUserRows(rankingMap, query.Limit) {
		data.Ranking = append(data.Ranking, usageStatsRankItemFromRow(row))
	}
	for _, row := range usageStatsSortedUserRows(walletRankingMap, query.Limit) {
		data.WalletRanking = append(data.WalletRanking, usageStatsRankItemFromRow(row))
	}
	for _, row := range usageStatsSortedUserRows(subscriptionRankingMap, query.Limit) {
		data.SubscriptionRanking = append(data.SubscriptionRanking, usageStatsRankItemFromRow(row))
	}

	modelRows := make([]usageStatsBaseRow, 0, len(modelMap))
	for _, row := range modelMap {
		finalizeUsageStatsBaseRow(row)
		modelRows = append(modelRows, *row)
	}
	sort.Slice(modelRows, func(i, j int) bool {
		if modelRows[i].Quota != modelRows[j].Quota {
			return modelRows[i].Quota > modelRows[j].Quota
		}
		if modelRows[i].RequestCount != modelRows[j].RequestCount {
			return modelRows[i].RequestCount > modelRows[j].RequestCount
		}
		return modelRows[i].ModelName < modelRows[j].ModelName
	})
	if len(modelRows) > query.Limit {
		modelRows = modelRows[:query.Limit]
	}
	for _, row := range modelRows {
		data.Models = append(data.Models, usageStatsModelItemFromRow(row))
	}

	trendRows := make([]usageStatsTrendRow, 0, len(trendMap))
	for _, row := range trendMap {
		trendRows = append(trendRows, *row)
	}
	data.Trend = buildUsageStatsTrend(trendRows, query)

	rankedUserIDs := make(map[int]struct{}, len(data.Ranking))
	for _, item := range data.Ranking {
		rankedUserIDs[item.UserId] = struct{}{}
	}
	detailRows := make([]usageStatsBaseRow, 0, len(detailMap))
	for _, row := range detailMap {
		if _, ok := rankedUserIDs[row.UserId]; !ok {
			continue
		}
		finalizeUsageStatsBaseRow(row)
		detailRows = append(detailRows, *row)
	}
	data.UserModelDetails = buildUsageStatsUserModelDetails(detailRows, data.Ranking)
	return nil
}

func GetUsageStats(query UsageStatsQuery) (UsageStatsData, error) {
	var err error
	query, err = normalizeUsageStatsQuery(query)
	if err != nil {
		return UsageStatsData{}, err
	}
	ensureCommonColumnsInitialized()
	data := UsageStatsData{
		StartTimestamp:              query.StartTimestamp,
		EndTimestamp:                query.EndTimestamp,
		TrendGranularity:            query.TrendGranularity,
		GeneratedAt:                 time.Now().Unix(),
		Ranking:                     []UsageStatsRankItem{},
		WalletRanking:               []UsageStatsRankItem{},
		SubscriptionRanking:         []UsageStatsRankItem{},
		Trend:                       []UsageStatsTrendPoint{},
		Models:                      []UsageStatsModelItem{},
		UserModelDetails:            []UsageStatsUserModelDetail{},
		RechargeRanking:             UsageStatsRechargeRankPage{Page: query.RechargePage, PageSize: query.RechargePageSize, Items: []UsageStatsRechargeRankItem{}},
		RechargeDetails:             UsageStatsRechargeDetailPage{Page: query.RechargeDetailPage, PageSize: query.RechargeDetailSize, UserId: query.RechargeUserId, Items: []UsageStatsRechargeDetailItem{}},
		SubscriptionPurchaseRanking: UsageStatsSubscriptionPurchaseRankPage{Page: query.SubscriptionPurchasePage, PageSize: query.SubscriptionPurchasePageSize, Items: []UsageStatsSubscriptionPurchaseRankItem{}},
		SubscriptionPurchaseDetails: UsageStatsSubscriptionPurchaseDetailPage{Page: query.SubscriptionPurchaseDetailPage, PageSize: query.SubscriptionPurchaseDetailSize, UserId: query.SubscriptionPurchaseUserId, Items: []UsageStatsSubscriptionPurchaseDetailItem{}},
	}
	if usageStatsIncludesSection(query, UsageStatsSectionUsage) {
		if err = populateUsageStatsUsage(&data, query); err != nil {
			return data, err
		}
	}
	if usageStatsIncludesSection(query, UsageStatsSectionRecharge) {
		if err = populateUsageStatsRecharge(&data, query); err != nil {
			return data, err
		}
	}
	if usageStatsIncludesSection(query, UsageStatsSectionSubscriptionPurchase) {
		if err = populateUsageStatsSubscriptionPurchase(&data, query); err != nil {
			return data, err
		}
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
	tx = excludeNonBillingAuditLogs(tx, "logs.other")
	rpmTpmQuery = excludeNonBillingAuditLogs(rpmTpmQuery, "logs.other")

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
	tx = tx.Where("type = ?", LogTypeConsume)
	tx = excludeNonBillingAuditLogs(tx, "logs.other")
	tx.Scan(&token)
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
