package model

import (
	"database/sql/driver"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssetType string

const (
	AssetTypeImage AssetType = "image"
	AssetTypeVideo AssetType = "video"
	AssetTypeAudio AssetType = "audio"
	AssetTypeFile  AssetType = "file"
)

type AssetStatus string

const (
	AssetStatusAvailable   AssetStatus = "available"
	AssetStatusBlocked     AssetStatus = "blocked"
	AssetStatusDeleted     AssetStatus = "deleted"
	AssetStatusUnavailable AssetStatus = "unavailable"
)

type AssetMetadata map[string]any

func (m *AssetMetadata) Scan(val interface{}) error {
	bytesValue, _ := val.([]byte)
	if len(bytesValue) == 0 {
		*m = AssetMetadata{}
		return nil
	}
	return common.Unmarshal(bytesValue, m)
}

func (m AssetMetadata) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return common.Marshal(m)
}

type Asset struct {
	ID           int64                 `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	AssetID      string                `json:"asset_id" gorm:"type:varchar(191);uniqueIndex"`
	TaskID       string                `json:"task_id" gorm:"type:varchar(191);uniqueIndex:idx_asset_task_index,priority:1;index"`
	TaskRecordID int64                 `json:"task_record_id" gorm:"index"`
	AssetIndex   int                   `json:"asset_index" gorm:"uniqueIndex:idx_asset_task_index,priority:2"`
	UserID       int                   `json:"user_id" gorm:"index:idx_asset_user_created,priority:1"`
	Group        string                `json:"group" gorm:"type:varchar(50)"`
	ChannelID    int                   `json:"channel_id" gorm:"index"`
	Platform     constant.TaskPlatform `json:"platform" gorm:"type:varchar(30);index:idx_asset_platform_action,priority:1"`
	Action       string                `json:"action" gorm:"type:varchar(40);index:idx_asset_platform_action,priority:2"`
	Model        string                `json:"model" gorm:"type:varchar(191);index"`
	AssetType    AssetType             `json:"asset_type" gorm:"type:varchar(20);index:idx_asset_type_created,priority:1"`
	URL          string                `json:"url" gorm:"type:text"`
	ThumbnailURL string                `json:"thumbnail_url" gorm:"type:text"`
	MimeType     string                `json:"mime_type" gorm:"type:varchar(100)"`
	Filename     string                `json:"filename" gorm:"type:varchar(255)"`
	SizeBytes    int64                 `json:"size_bytes"`
	Width        int                   `json:"width"`
	Height       int                   `json:"height"`
	DurationMS   int64                 `json:"duration_ms"`
	Status       AssetStatus           `json:"status" gorm:"type:varchar(20);index"`
	Metadata     AssetMetadata         `json:"metadata" gorm:"type:json"`
	CreatedAt    int64                 `json:"created_at" gorm:"index:idx_asset_user_created,priority:2;index:idx_asset_type_created,priority:2"`
	UpdatedAt    int64                 `json:"updated_at"`
	DeletedAt    int64                 `json:"deleted_at" gorm:"index"`
	Username     string                `json:"username,omitempty" gorm:"-"`
}

func GenerateAssetID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "asset_" + key
}

type AssetQueryParams struct {
	AssetType      AssetType
	Status         AssetStatus
	TaskID         string
	Platform       constant.TaskPlatform
	Action         string
	Model          string
	ChannelID      string
	UserID         string
	StartTimestamp int64
	EndTimestamp   int64
	Keyword        string
	IncludeHidden  bool
}

type AssetCreateInput struct {
	Task         *Task
	AssetIndex   int
	AssetType    AssetType
	URL          string
	ThumbnailURL string
	MimeType     string
	Filename     string
	SizeBytes    int64
	Width        int
	Height       int
	DurationMS   int64
	Metadata     AssetMetadata
}

func normalizeAssetStatus(status AssetStatus) AssetStatus {
	switch status {
	case AssetStatusAvailable, AssetStatusBlocked, AssetStatusDeleted, AssetStatusUnavailable:
		return status
	default:
		return ""
	}
}

func normalizeAssetType(assetType AssetType) AssetType {
	switch assetType {
	case AssetTypeImage, AssetTypeVideo, AssetTypeAudio, AssetTypeFile:
		return assetType
	default:
		return ""
	}
}

func visibleAssetQuery(query *gorm.DB) *gorm.DB {
	return query.Where("status = ?", AssetStatusAvailable).Where("deleted_at = ?", 0)
}

func applyAssetQuery(query *gorm.DB, queryParams AssetQueryParams) *gorm.DB {
	if !queryParams.IncludeHidden {
		query = visibleAssetQuery(query)
	}
	if queryParams.AssetType != "" {
		query = query.Where("asset_type = ?", queryParams.AssetType)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Model != "" {
		query = query.Where("model = ?", queryParams.Model)
	}
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("created_at >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("created_at <= ?", queryParams.EndTimestamp)
	}
	if strings.TrimSpace(queryParams.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(queryParams.Keyword) + "%"
		query = query.Where("(asset_id LIKE ? OR task_id LIKE ? OR filename LIKE ? OR url LIKE ?)", keyword, keyword, keyword, keyword)
	}
	return query
}

func CreateAssetsForTask(inputs []AssetCreateInput) error {
	return CreateAssetsForTaskTx(DB, inputs)
}

func CreateAssetsForTaskTx(tx *gorm.DB, inputs []AssetCreateInput) error {
	if len(inputs) == 0 {
		return nil
	}
	now := time.Now().Unix()
	assets := make([]Asset, 0, len(inputs))
	for _, input := range inputs {
		task := input.Task
		if task == nil || strings.TrimSpace(input.URL) == "" {
			continue
		}
		assetType := normalizeAssetType(input.AssetType)
		if assetType == "" {
			assetType = AssetTypeFile
		}
		modelName := task.Properties.OriginModelName
		if modelName == "" {
			modelName = task.Properties.UpstreamModelName
		}
		assets = append(assets, Asset{
			AssetID:      GenerateAssetID(),
			TaskID:       task.TaskID,
			TaskRecordID: task.ID,
			AssetIndex:   input.AssetIndex,
			UserID:       task.UserId,
			Group:        task.Group,
			ChannelID:    task.ChannelId,
			Platform:     task.Platform,
			Action:       task.Action,
			Model:        modelName,
			AssetType:    assetType,
			URL:          strings.TrimSpace(input.URL),
			ThumbnailURL: strings.TrimSpace(input.ThumbnailURL),
			MimeType:     strings.TrimSpace(input.MimeType),
			Filename:     strings.TrimSpace(input.Filename),
			SizeBytes:    input.SizeBytes,
			Width:        input.Width,
			Height:       input.Height,
			DurationMS:   input.DurationMS,
			Status:       AssetStatusAvailable,
			Metadata:     input.Metadata,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	if len(assets) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "asset_index"}},
		DoNothing: true,
	}).Create(&assets).Error
}

func AssetGetAll(startIdx int, num int, queryParams AssetQueryParams) ([]*Asset, error) {
	var assets []*Asset
	query := applyAssetQuery(DB.Model(&Asset{}), queryParams)
	err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&assets).Error
	return assets, err
}

func AssetCountAll(queryParams AssetQueryParams) int64 {
	var total int64
	_ = applyAssetQuery(DB.Model(&Asset{}), queryParams).Count(&total).Error
	return total
}

func AssetGetAllByUser(userID int, startIdx int, num int, queryParams AssetQueryParams) ([]*Asset, error) {
	queryParams.UserID = ""
	var assets []*Asset
	query := applyAssetQuery(DB.Model(&Asset{}).Where("user_id = ?", userID), queryParams)
	err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&assets).Error
	return assets, err
}

func AssetCountAllByUser(userID int, queryParams AssetQueryParams) int64 {
	queryParams.UserID = ""
	var total int64
	_ = applyAssetQuery(DB.Model(&Asset{}).Where("user_id = ?", userID), queryParams).Count(&total).Error
	return total
}

func GetAssetByAssetID(assetID string, includeHidden bool) (*Asset, bool, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, false, nil
	}
	var asset Asset
	query := DB.Where("asset_id = ?", assetID)
	if !includeHidden {
		query = visibleAssetQuery(query)
	}
	err := query.First(&asset).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return &asset, exist, nil
}

func GetUserAssetByAssetID(userID int, assetID string) (*Asset, bool, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, false, nil
	}
	var asset Asset
	err := visibleAssetQuery(DB.Where("user_id = ? AND asset_id = ?", userID, assetID)).First(&asset).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return &asset, exist, nil
}

func GetAssetsByAssetIDs(assetIDs []string, includeHidden bool) ([]*Asset, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	var assets []*Asset
	query := DB.Where("asset_id in (?)", assetIDs)
	if !includeHidden {
		query = visibleAssetQuery(query)
	}
	err := query.Order("id desc").Find(&assets).Error
	return assets, err
}

func GetUserAssetsByAssetIDs(userID int, assetIDs []string) ([]*Asset, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	var assets []*Asset
	err := visibleAssetQuery(DB.Where("user_id = ? AND asset_id in (?)", userID, assetIDs)).
		Order("id desc").Find(&assets).Error
	return assets, err
}

func GetUserAssetsByTaskIDs(userID int, taskIDs []string) ([]*Asset, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	var assets []*Asset
	err := visibleAssetQuery(DB.Where("user_id = ? AND task_id IN ?", userID, taskIDs)).
		Order("task_id ASC, asset_index ASC").Find(&assets).Error
	return assets, err
}

func UpdateAssetBlocked(assetID string, blocked bool) (*Asset, bool, error) {
	asset, exists, err := GetAssetByAssetID(assetID, true)
	if err != nil || !exists {
		return asset, exists, err
	}
	status := AssetStatusAvailable
	if blocked {
		status = AssetStatusBlocked
	}
	if asset.Status == AssetStatusDeleted {
		return asset, true, nil
	}
	if err := DB.Model(asset).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}).Error; err != nil {
		return nil, true, err
	}
	asset.Status = status
	return asset, true, nil
}
