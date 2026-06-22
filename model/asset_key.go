package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	AssetKeyPrefix         = "ak_"
	AssetKeyScopeRead      = "assets:read"
	AssetKeyStatusEnabled  = 1
	AssetKeyStatusDisabled = 2
)

type AssetKey struct {
	ID         int64          `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	UserID     int            `json:"user_id" gorm:"index"`
	Name       string         `json:"name" gorm:"type:varchar(100);index"`
	Key        string         `json:"key" gorm:"type:varchar(80);uniqueIndex"`
	Status     int            `json:"status" gorm:"default:1"`
	Scopes     string         `json:"scopes" gorm:"type:varchar(255);default:'assets:read'"`
	AllowIPs   string         `json:"allow_ips" gorm:"type:text"`
	ExpiredAt  int64          `json:"expired_at" gorm:"bigint;default:-1"`
	LastUsedAt int64          `json:"last_used_at" gorm:"bigint;default:0"`
	CreatedAt  int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt  int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func GenerateAssetKey() (string, error) {
	key, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return "", err
	}
	return AssetKeyPrefix + key, nil
}

func (key *AssetKey) GetIPLimits() []string {
	cleanIPs := strings.ReplaceAll(key.AllowIPs, " ", "")
	if cleanIPs == "" {
		return nil
	}
	lines := strings.Split(cleanIPs, "\n")
	limits := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.ReplaceAll(line, ",", ""))
		if line != "" {
			limits = append(limits, line)
		}
	}
	return limits
}

func (key *AssetKey) IsExpired(now int64) bool {
	return key.ExpiredAt != -1 && key.ExpiredAt != 0 && key.ExpiredAt < now
}

func CreateAssetKey(userID int, name string, expiredAt int64, allowIPs string) (*AssetKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if len(name) > 100 {
		return nil, errors.New("name is too long")
	}
	if expiredAt == 0 {
		expiredAt = -1
	}
	now := time.Now().Unix()
	keyValue, err := GenerateAssetKey()
	if err != nil {
		return nil, err
	}
	assetKey := &AssetKey{
		UserID:     userID,
		Name:       name,
		Key:        keyValue,
		Status:     AssetKeyStatusEnabled,
		Scopes:     AssetKeyScopeRead,
		AllowIPs:   strings.TrimSpace(allowIPs),
		ExpiredAt:  expiredAt,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastUsedAt: 0,
	}
	if err := DB.Create(assetKey).Error; err != nil {
		return nil, err
	}
	return assetKey, nil
}

func GetUserAssetKeys(userID int, startIdx int, num int) ([]*AssetKey, error) {
	var keys []*AssetKey
	err := DB.Where("user_id = ?", userID).Order("id desc").Limit(num).Offset(startIdx).Find(&keys).Error
	return keys, err
}

func CountUserAssetKeys(userID int) int64 {
	var total int64
	_ = DB.Model(&AssetKey{}).Where("user_id = ?", userID).Count(&total).Error
	return total
}

func GetUserAssetKeyByID(id int64, userID int) (*AssetKey, error) {
	if id == 0 || userID == 0 {
		return nil, errors.New("id or user_id is empty")
	}
	var key AssetKey
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func GetAssetKeyByKey(keyValue string) (*AssetKey, error) {
	keyValue = strings.TrimSpace(keyValue)
	if keyValue == "" {
		return nil, errors.New("key is empty")
	}
	var key AssetKey
	if err := DB.Where("key = ?", keyValue).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func UpdateUserAssetKeyStatus(id int64, userID int, status int) (*AssetKey, error) {
	if status != AssetKeyStatusEnabled && status != AssetKeyStatusDisabled {
		return nil, errors.New("invalid status")
	}
	key, err := GetUserAssetKeyByID(id, userID)
	if err != nil {
		return nil, err
	}
	if err := DB.Model(key).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}).Error; err != nil {
		return nil, err
	}
	key.Status = status
	key.UpdatedAt = time.Now().Unix()
	return key, nil
}

func DeleteUserAssetKey(id int64, userID int) error {
	key, err := GetUserAssetKeyByID(id, userID)
	if err != nil {
		return err
	}
	return DB.Delete(key).Error
}

func TouchAssetKeyLastUsed(id int64) {
	if id == 0 {
		return
	}
	_ = DB.Model(&AssetKey{}).Where("id = ?", id).Update("last_used_at", time.Now().Unix()).Error
}
