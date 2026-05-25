package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ViolationLog struct {
	Id             int    `json:"id" gorm:"index:idx_violation_created_id,priority:2"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index:idx_violation_created_id,priority:1"`
	UserId         int    `json:"user_id" gorm:"index"`
	Username       string `json:"username" gorm:"index;default:''"`
	TokenId        int    `json:"token_id" gorm:"index;default:0"`
	TokenName      string `json:"token_name" gorm:"index;default:''"`
	ModelName      string `json:"model_name" gorm:"index;default:''"`
	UserGroup      string `json:"user_group" gorm:"type:varchar(64);default:''"`
	UsingGroup     string `json:"using_group" gorm:"type:varchar(64);index;default:''"`
	AggregateGroup string `json:"aggregate_group" gorm:"type:varchar(128);index;default:''"`
	RouteGroup     string `json:"route_group" gorm:"type:varchar(128);index;default:''"`
	RequestId      string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	RequestPath    string `json:"request_path" gorm:"type:varchar(255);default:''"`
	MatchedWords   string `json:"matched_words" gorm:"type:text"`
	TextExcerpt    string `json:"text_excerpt" gorm:"type:text"`
	Action         string `json:"action" gorm:"type:varchar(32);index;default:''"`
	HTTPStatusCode int    `json:"http_status_code" gorm:"default:0"`
	ErrorCode      string `json:"error_code" gorm:"type:varchar(64);default:''"`
	Banned         bool   `json:"banned" gorm:"index;default:false"`
	IsStream       bool   `json:"is_stream" gorm:"default:false"`
}

type ViolationLogQuery struct {
	UserId         int
	Username       string
	TokenId        int
	TokenName      string
	ModelName      string
	UserGroup      string
	UsingGroup     string
	AggregateGroup string
	RouteGroup     string
	RequestId      string
	MatchedWord    string
	Action         string
	Banned         *bool
	StartTimestamp int64
	EndTimestamp   int64
}

func InsertViolationLog(log *ViolationLog) error {
	if log.CreatedAt == 0 {
		log.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(log).Error
}

func CountViolationLogsByUserID(userID int) (int64, error) {
	if userID <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&ViolationLog{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func GetViolationLogs(query ViolationLogQuery, startIdx int, num int) ([]*ViolationLog, int64, error) {
	tx := applyViolationLogQuery(DB.Model(&ViolationLog{}), query)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []*ViolationLog
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func DeleteViolationLogsBefore(targetTimestamp int64, limit int) (int64, error) {
	if targetTimestamp <= 0 {
		return 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	var total int64
	for {
		var logs []ViolationLog
		if err := DB.Select("id").
			Where("created_at < ?", targetTimestamp).
			Order("id asc").
			Limit(limit).
			Find(&logs).Error; err != nil {
			return total, err
		}
		if len(logs) == 0 {
			return total, nil
		}
		ids := make([]int, 0, len(logs))
		for _, log := range logs {
			ids = append(ids, log.Id)
		}
		result := DB.Delete(&ViolationLog{}, ids)
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < int64(limit) {
			return total, nil
		}
	}
}

func DisableUserByViolation(userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	result := DB.Model(&User{}).
		Where("id = ? AND status <> ?", userID, common.UserStatusDisabled).
		Update("status", common.UserStatusDisabled)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		_ = updateUserStatusCache(userID, false)
		return false, nil
	}
	if err := updateUserStatusCache(userID, false); err != nil {
		return true, err
	}
	return true, nil
}

func MarkViolationLogBanned(id int) error {
	if id <= 0 {
		return nil
	}
	return DB.Model(&ViolationLog{}).Where("id = ?", id).Update("banned", true).Error
}

func UpdateViolationLogRouteContext(id int, usingGroup string, routeGroup string) error {
	if id <= 0 {
		return nil
	}
	updates := map[string]interface{}{}
	if strings.TrimSpace(usingGroup) != "" {
		updates["using_group"] = usingGroup
	}
	if strings.TrimSpace(routeGroup) != "" {
		updates["route_group"] = routeGroup
	}
	if len(updates) == 0 {
		return nil
	}
	return DB.Model(&ViolationLog{}).Where("id = ?", id).Updates(updates).Error
}

func applyViolationLogQuery(tx *gorm.DB, query ViolationLogQuery) *gorm.DB {
	if query.UserId > 0 {
		tx = tx.Where("user_id = ?", query.UserId)
	}
	if strings.TrimSpace(query.Username) != "" {
		tx = tx.Where("username = ?", strings.TrimSpace(query.Username))
	}
	if query.TokenId > 0 {
		tx = tx.Where("token_id = ?", query.TokenId)
	}
	if strings.TrimSpace(query.TokenName) != "" {
		tx = tx.Where("token_name = ?", strings.TrimSpace(query.TokenName))
	}
	if strings.TrimSpace(query.ModelName) != "" {
		tx = tx.Where("model_name = ?", strings.TrimSpace(query.ModelName))
	}
	if strings.TrimSpace(query.UserGroup) != "" {
		tx = tx.Where("user_group = ?", strings.TrimSpace(query.UserGroup))
	}
	if strings.TrimSpace(query.UsingGroup) != "" {
		tx = tx.Where("using_group = ?", strings.TrimSpace(query.UsingGroup))
	}
	if strings.TrimSpace(query.AggregateGroup) != "" {
		tx = tx.Where("aggregate_group = ?", strings.TrimSpace(query.AggregateGroup))
	}
	if strings.TrimSpace(query.RouteGroup) != "" {
		tx = tx.Where("route_group = ?", strings.TrimSpace(query.RouteGroup))
	}
	if strings.TrimSpace(query.RequestId) != "" {
		tx = tx.Where("request_id = ?", strings.TrimSpace(query.RequestId))
	}
	if strings.TrimSpace(query.MatchedWord) != "" {
		tx = tx.Where("matched_words LIKE ?", "%"+strings.TrimSpace(query.MatchedWord)+"%")
	}
	if strings.TrimSpace(query.Action) != "" {
		tx = tx.Where("action = ?", strings.TrimSpace(query.Action))
	}
	if query.Banned != nil {
		tx = tx.Where("banned = ?", *query.Banned)
	}
	if query.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	return tx
}
