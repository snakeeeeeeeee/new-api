package model

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// QuotaData 柱状图数据
type QuotaData struct {
	Id        int    `json:"id"`
	UserID    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	TokenUsed int    `json:"token_used" gorm:"default:0"`
	Count     int    `json:"count" gorm:"default:0"`
	Quota     int    `json:"quota" gorm:"default:0"`
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

func logQuotaDataCache(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	key := fmt.Sprintf("%d-%s-%s-%d", userId, username, modelName, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.Count += 1
		quotaData.Quota += quota
		quotaData.TokenUsed += tokenUsed
	} else {
		quotaData = &QuotaData{
			UserID:    userId,
			Username:  username,
			ModelName: modelName,
			CreatedAt: createdAt,
			Count:     1,
			Quota:     quota,
			TokenUsed: tokenUsed,
		}
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	// 只精确到小时
	createdAt = createdAt - (createdAt % 3600)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(userId, username, modelName, quota, createdAt, tokenUsed)
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.CreatedAt).First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			//quotaDataDB.Count += quotaData.Count
			//quotaDataDB.Quota += quotaData.Quota
			//DB.Table("quota_data").Save(quotaDataDB)
			increaseQuotaData(quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.Count, quotaData.Quota, quotaData.CreatedAt, quotaData.TokenUsed)
		} else {
			DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(userId int, username string, modelName string, count int, quota int, createdAt int64, tokenUsed int) {
	err := DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
		userId, username, modelName, createdAt).Updates(map[string]interface{}{
		"count":      gorm.Expr("count + ?", count),
		"quota":      gorm.Expr("quota + ?", quota),
		"token_used": gorm.Expr("token_used + ?", tokenUsed),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).Find(&quotaDatas).Error
	if err == nil {
		quotaDatas = appendQuotaDataCacheSnapshot(quotaDatas, startTime, endTime, func(data *QuotaData) bool {
			return data.Username == username
		})
		quotaDatas, err = appendQuotaDataSettlementAdjustments(quotaDatas, startTime, endTime, 0, username)
	}
	return quotaDatas, err
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).Find(&quotaDatas).Error
	if err == nil {
		quotaDatas = appendQuotaDataCacheSnapshot(quotaDatas, startTime, endTime, func(data *QuotaData) bool {
			return data.UserID == userId
		})
		quotaDatas, err = appendQuotaDataSettlementAdjustments(quotaDatas, startTime, endTime, userId, "")
	}
	return quotaDatas, err
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = DB.Table("quota_data").Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, created_at").Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	if err == nil {
		quotaDatas = appendQuotaDataCacheSnapshot(quotaDatas, startTime, endTime, func(data *QuotaData) bool {
			return true
		})
		quotaDatas, err = appendQuotaDataSettlementAdjustments(quotaDatas, startTime, endTime, 0, "")
	}
	return quotaDatas, err
}

type quotaDataSettlementLog struct {
	UserId    int
	Username  string
	ModelName string
	CreatedAt int64
	Type      int
	Quota     int
	Other     string
}

func quotaDataSettlementQuery(startTime int64, endTime int64, userId int, username string) *gorm.DB {
	query := LOG_DB.Model(&Log{}).
		Select("user_id, username, model_name, created_at, type, quota, other").
		Where("created_at >= ? AND created_at <= ?", startTime, endTime)
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if username != "" {
		query = query.Where("username = ?", username)
	}
	return excludeNonBillingAuditLogs(query, "logs.other")
}

func quotaDataOtherInt(other map[string]interface{}, key string) (int, bool) {
	value, ok := other[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		var parsed int
		if _, err := fmt.Sscan(v, &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func appendQuotaDataSettlementAdjustments(quotaDatas []*QuotaData, startTime int64, endTime int64, userId int, username string) ([]*QuotaData, error) {
	// quota_data stores hourly buckets. Settlement rows must use the same bucket filter
	// after their second-level log timestamps are selected.
	appendAdjustment := func(row quotaDataSettlementLog, quotaDelta int) {
		if quotaDelta == 0 {
			return
		}
		adjustment := quotaDataAdjustmentFromLog(row, quotaDelta)
		if adjustment.CreatedAt < startTime || adjustment.CreatedAt > endTime {
			return
		}
		quotaDatas = append(quotaDatas, adjustment)
	}

	var rows []quotaDataSettlementLog
	if err := quotaDataSettlementQuery(startTime, endTime, userId, username).
		Where("type = ?", LogTypeRefund).
		Scan(&rows).Error; err != nil {
		return quotaDatas, err
	}
	for _, row := range rows {
		appendAdjustment(row, -row.Quota)
	}

	rows = nil
	if err := quotaDataSettlementQuery(startTime, endTime, userId, username).
		Where("type IN ?", []int{LogTypeConsume, LogTypeError}).
		Where("other LIKE ?", "%pre_consumed_quota%").
		Scan(&rows).Error; err != nil {
		return quotaDatas, err
	}
	for _, row := range rows {
		other := make(map[string]interface{})
		if err := common.UnmarshalJsonStr(row.Other, &other); err != nil {
			continue
		}
		billingStage, _ := other["billing_stage"].(string)
		quotaDelta := 0
		if strings.HasPrefix(billingStage, "async_image_") {
			var ok bool
			quotaDelta, ok = quotaDataOtherInt(other, "quota_delta")
			if !ok {
				actualQuota, actualOK := quotaDataOtherInt(other, "actual_quota")
				preConsumedQuota, preConsumedOK := quotaDataOtherInt(other, "pre_consumed_quota")
				if actualOK && preConsumedOK {
					quotaDelta = actualQuota - preConsumedQuota
				}
			}
		} else if row.Type == LogTypeConsume {
			_, preConsumedOK := quotaDataOtherInt(other, "pre_consumed_quota")
			_, actualOK := quotaDataOtherInt(other, "actual_quota")
			if preConsumedOK && actualOK {
				quotaDelta = row.Quota
			}
		}
		appendAdjustment(row, quotaDelta)
	}
	return quotaDatas, nil
}

func quotaDataAdjustmentFromLog(row quotaDataSettlementLog, quotaDelta int) *QuotaData {
	createdAt := row.CreatedAt - (row.CreatedAt % 3600)
	return &QuotaData{
		UserID:    row.UserId,
		Username:  row.Username,
		ModelName: row.ModelName,
		CreatedAt: createdAt,
		Quota:     quotaDelta,
	}
}

func appendQuotaDataCacheSnapshot(quotaDatas []*QuotaData, startTime int64, endTime int64, match func(*QuotaData) bool) []*QuotaData {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	for _, data := range CacheQuotaData {
		if data == nil || data.CreatedAt < startTime || data.CreatedAt > endTime {
			continue
		}
		if match != nil && !match(data) {
			continue
		}
		copied := *data
		quotaDatas = append(quotaDatas, &copied)
	}
	return quotaDatas
}
