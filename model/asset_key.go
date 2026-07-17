package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func ParseAssetKeyScopes(value string) []string {
	return []string{AssetKeyScopeRead}
}

func AssetKeyHasScope(value, required string) bool {
	for _, scope := range ParseAssetKeyScopes(value) {
		if scope == required {
			return true
		}
	}
	return false
}

func NormalizeAssetKeyScopes(scopes []string) (string, error) {
	if len(scopes) == 0 {
		return AssetKeyScopeRead, nil
	}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != AssetKeyScopeRead {
			return "", fmt.Errorf("invalid asset key scope: %s", scope)
		}
	}
	return AssetKeyScopeRead, nil
}

func NormalizeExistingAssetKeyScopes() error {
	return DB.Model(&AssetKey{}).Where("scopes IS NULL OR scopes <> ?", AssetKeyScopeRead).
		Update("scopes", AssetKeyScopeRead).Error
}

// NormalizeExistingAssetKeys keeps the newest key as the account's current
// Resource Center credential and disables older records for compatibility.
func NormalizeExistingAssetKeys() error {
	var userIDs []int
	if err := DB.Model(&AssetKey{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	for _, userID := range userIDs {
		var current AssetKey
		if err := DB.Where("user_id = ?", userID).Order("id DESC").First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if err := DB.Model(&AssetKey{}).
			Where("user_id = ? AND id <> ? AND status <> ?", userID, current.ID, AssetKeyStatusDisabled).
			Update("status", AssetKeyStatusDisabled).Error; err != nil {
			return err
		}
	}
	return nil
}

func CreateAssetKey(userID int, name string, expiredAt int64, allowIPs string) (*AssetKey, error) {
	return CreateAssetKeyWithScopes(userID, name, expiredAt, allowIPs, nil)
}

func CreateAssetKeyWithScopes(userID int, name string, expiredAt int64, allowIPs string, scopes []string) (*AssetKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "resource-center"
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
	normalizedScopes, err := NormalizeAssetKeyScopes(scopes)
	if err != nil {
		return nil, err
	}
	var assetKey AssetKey
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&user, userID).Error; err != nil {
			return err
		}
		err := tx.Where("user_id = ?", userID).Order("id DESC").First(&assetKey).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			assetKey = AssetKey{
				UserID: userID, Name: name, Key: keyValue, Status: AssetKeyStatusEnabled,
				Scopes: normalizedScopes, AllowIPs: strings.TrimSpace(allowIPs), ExpiredAt: expiredAt,
				CreatedAt: now, UpdatedAt: now,
			}
			return tx.Create(&assetKey).Error
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"name": name, "key": keyValue, "status": AssetKeyStatusEnabled,
			"scopes": normalizedScopes, "allow_ips": strings.TrimSpace(allowIPs), "expired_at": expiredAt,
			"last_used_at": 0, "updated_at": now,
		}
		if err := tx.Model(&assetKey).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&AssetKey{}).
			Where("user_id = ? AND id <> ? AND status <> ?", userID, assetKey.ID, AssetKeyStatusDisabled).
			Update("status", AssetKeyStatusDisabled).Error; err != nil {
			return err
		}
		assetKey.Name = name
		assetKey.Key = keyValue
		assetKey.Status = AssetKeyStatusEnabled
		assetKey.Scopes = normalizedScopes
		assetKey.AllowIPs = strings.TrimSpace(allowIPs)
		assetKey.ExpiredAt = expiredAt
		assetKey.LastUsedAt = 0
		assetKey.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &assetKey, nil
}

func GetUserAssetKeys(userID int, startIdx int, num int) ([]*AssetKey, error) {
	if startIdx > 0 || num <= 0 {
		return []*AssetKey{}, nil
	}
	var keys []*AssetKey
	err := DB.Where("user_id = ?", userID).Order("id DESC").Limit(1).Find(&keys).Error
	return keys, err
}

func GetActiveUserAssetKey(userID int) (*AssetKey, bool, error) {
	if userID == 0 {
		return nil, false, errors.New("user_id is empty")
	}
	var key AssetKey
	now := time.Now().Unix()
	err := DB.Where(
		"user_id = ? AND status = ? AND (expired_at = ? OR expired_at = ? OR expired_at >= ?)",
		userID, AssetKeyStatusEnabled, -1, 0, now,
	).Order("id DESC").First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &key, true, nil
}

func CountUserAssetKeys(userID int) int64 {
	var total int64
	_ = DB.Model(&AssetKey{}).Where("user_id = ?", userID).Limit(1).Count(&total).Error
	if total > 1 {
		total = 1
	}
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
	var current AssetKey
	if err := DB.Where("user_id = ?", userID).Order("id DESC").First(&current).Error; err != nil {
		return nil, err
	}
	if current.ID != key.ID {
		return nil, gorm.ErrRecordNotFound
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
	var current AssetKey
	if err := DB.Where("user_id = ?", key.UserID).Order("id DESC").First(&current).Error; err != nil {
		return nil, err
	}
	if current.ID != key.ID {
		return nil, gorm.ErrRecordNotFound
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
	if _, err := GetUserAssetKeyByID(id, userID); err != nil {
		return err
	}
	return DB.Where("user_id = ?", userID).Delete(&AssetKey{}).Error
}

func TouchAssetKeyLastUsed(id int64) {
	if id == 0 {
		return
	}
	_ = DB.Model(&AssetKey{}).Where("id = ?", id).Update("last_used_at", time.Now().Unix()).Error
}
