package model

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

const (
	ErrorSnapshotCaptureLevelSummary  = "summary"
	ErrorSnapshotCaptureLevelPriority = "priority"
)

type ErrorSnapshot struct {
	ID               string `json:"id" gorm:"type:varchar(32);primaryKey"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_error_snapshots_created_at"`
	RequestID        string `json:"request_id" gorm:"type:varchar(64);index:idx_error_snapshots_request_id"`
	RequestPath      string `json:"request_path" gorm:"type:varchar(255)"`
	UserID           int    `json:"user_id" gorm:"index:idx_error_snapshots_user_id"`
	Username         string `json:"username" gorm:"type:varchar(128);index:idx_error_snapshots_username"`
	ChannelID        int    `json:"channel_id" gorm:"index:idx_error_snapshots_channel_id"`
	ChannelName      string `json:"channel_name" gorm:"type:varchar(255)"`
	ModelName        string `json:"model_name" gorm:"type:varchar(255)"`
	AggregateGroup   string `json:"aggregate_group" gorm:"type:varchar(255)"`
	RouteGroup       string `json:"route_group" gorm:"type:varchar(255)"`
	RetryIndex       int    `json:"retry_index"`
	StatusCode       int    `json:"status_code" gorm:"index:idx_error_snapshots_status_code"`
	ErrorType        string `json:"error_type" gorm:"type:varchar(64)"`
	ErrorCode        string `json:"error_code" gorm:"type:varchar(128);index:idx_error_snapshots_error_code"`
	ErrorMessage     string `json:"error_message" gorm:"type:text"`
	CaptureLevel     string `json:"capture_level" gorm:"type:varchar(16)"`
	IsStream         bool   `json:"is_stream"`
	InternalRetry    bool   `json:"internal_retry"`
	FinalOutcome     string `json:"final_outcome" gorm:"type:varchar(32);index:idx_error_snapshots_final_outcome"`
	RelativePath     string `json:"-" gorm:"type:varchar(512)"`
	CompressedSize   int64  `json:"compressed_size" gorm:"bigint"`
	OriginalSize     int64  `json:"original_size" gorm:"bigint"`
	PayloadTruncated bool   `json:"payload_truncated"`
}

type ErrorSnapshotQuery struct {
	StartTimestamp int64
	EndTimestamp   int64
	RequestID      string
	UserID         int
	Username       string
	ChannelID      int
	ErrorKeyword   string
	StartIndex     int
	Limit          int
}

type ErrorSnapshotStorageStats struct {
	FileCount  int64 `json:"file_count"`
	TotalBytes int64 `json:"total_bytes"`
	OldestAt   int64 `json:"oldest_at"`
}

type ErrorSnapshotUserOption struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type ErrorSnapshotChannelOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func CreateErrorSnapshot(snapshot *ErrorSnapshot) error {
	return LOG_DB.Create(snapshot).Error
}

func GetErrorSnapshot(id string) (*ErrorSnapshot, error) {
	var snapshot ErrorSnapshot
	err := LOG_DB.Where("id = ?", strings.TrimSpace(id)).First(&snapshot).Error
	return &snapshot, err
}

func ListErrorSnapshots(query ErrorSnapshotQuery) ([]*ErrorSnapshot, int64, error) {
	tx := applyErrorSnapshotFilters(LOG_DB.Model(&ErrorSnapshot{}), query)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var snapshots []*ErrorSnapshot
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	err := tx.Order("created_at desc").Offset(query.StartIndex).Limit(query.Limit).Find(&snapshots).Error
	return snapshots, total, err
}

func applyErrorSnapshotFilters(tx *gorm.DB, query ErrorSnapshotQuery) *gorm.DB {
	if query.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	if requestID := strings.TrimSpace(query.RequestID); requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}
	if query.UserID > 0 {
		tx = tx.Where("user_id = ?", query.UserID)
	}
	if username := strings.TrimSpace(query.Username); username != "" {
		tx = tx.Where("username = ?", username)
	}
	if query.ChannelID > 0 {
		tx = tx.Where("channel_id = ?", query.ChannelID)
	}
	if keyword := strings.TrimSpace(query.ErrorKeyword); keyword != "" {
		keyword = strings.ReplaceAll(strings.ReplaceAll(keyword, "%", ""), "_", "")
		if keyword != "" {
			tx = tx.Where("LOWER(error_message) LIKE ?", "%"+strings.ToLower(keyword)+"%")
		}
	}
	return tx
}

func UpdateErrorSnapshotOutcome(requestID, outcome string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil
	}
	return LOG_DB.Model(&ErrorSnapshot{}).
		Where("request_id = ? AND final_outcome = ?", requestID, "pending").
		Update("final_outcome", outcome).Error
}

func DeleteErrorSnapshotRecord(id string) error {
	return LOG_DB.Delete(&ErrorSnapshot{}, "id = ?", strings.TrimSpace(id)).Error
}

func DeleteAllErrorSnapshotRecords() error {
	return LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ErrorSnapshot{}).Error
}

func ListErrorSnapshotsForCleanup() ([]*ErrorSnapshot, error) {
	var snapshots []*ErrorSnapshot
	err := LOG_DB.Select("id", "created_at", "relative_path", "compressed_size").
		Order("created_at asc").Find(&snapshots).Error
	return snapshots, err
}

func GetErrorSnapshotStorageStats() (ErrorSnapshotStorageStats, error) {
	var result struct {
		FileCount  int64
		TotalBytes int64
		OldestAt   int64
	}
	err := LOG_DB.Model(&ErrorSnapshot{}).
		Select("COUNT(*) AS file_count, COALESCE(SUM(compressed_size), 0) AS total_bytes, COALESCE(MIN(created_at), 0) AS oldest_at").
		Scan(&result).Error
	return ErrorSnapshotStorageStats(result), err
}

func SearchErrorSnapshotUserOptions(keyword string, limit int) ([]ErrorSnapshotUserOption, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := DB.Model(&User{}).Select("id", "username", "display_name")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			query = query.Where("id = ? OR username LIKE ? OR display_name LIKE ?", id, likeKeyword, likeKeyword)
		} else {
			query = query.Where("username LIKE ? OR display_name LIKE ?", likeKeyword, likeKeyword)
		}
	}
	var options []ErrorSnapshotUserOption
	err := query.Order("id desc").Limit(limit).Scan(&options).Error
	return options, err
}

func SearchErrorSnapshotChannelOptions(keyword string, limit int) ([]ErrorSnapshotChannelOption, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := DB.Model(&Channel{}).Select("id", "name")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		if id, err := strconv.Atoi(keyword); err == nil && id > 0 {
			query = query.Where("id = ? OR name LIKE ?", id, likeKeyword)
		} else {
			query = query.Where("name LIKE ?", likeKeyword)
		}
	}
	var options []ErrorSnapshotChannelOption
	err := query.Order("id desc").Limit(limit).Scan(&options).Error
	return options, err
}
