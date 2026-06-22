package model

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AdminMenuChannel        = "channel"
	AdminMenuAggregateGroup = "aggregate_group"
	AdminMenuInviteCode     = "invite_code"
	AdminMenuInviteStats    = "invite_stats"
	AdminMenuLogDashboard   = "log_dashboard"
	AdminMenuUsageStats     = "usage_stats"
	AdminMenuAsyncTask      = "async_task"
	AdminMenuAssets         = "assets"
	AdminMenuRequestDump    = "request_dump"
	AdminMenuViolation      = "violation"
	AdminMenuCompatibility  = "compatibility"
	AdminMenuSubscription   = "subscription"
	AdminMenuModels         = "models"
	AdminMenuDeployment     = "deployment"
	AdminMenuRedemption     = "redemption"
	AdminMenuUser           = "user"
	AdminMenuSetting        = "setting"
)

var defaultAdminMenuPermissionKeys = []string{
	AdminMenuChannel,
	AdminMenuAggregateGroup,
	AdminMenuInviteCode,
	AdminMenuInviteStats,
	AdminMenuLogDashboard,
	AdminMenuUsageStats,
	AdminMenuAsyncTask,
	AdminMenuAssets,
	AdminMenuRequestDump,
	AdminMenuViolation,
	AdminMenuCompatibility,
	AdminMenuSubscription,
	AdminMenuModels,
	AdminMenuDeployment,
	AdminMenuRedemption,
	AdminMenuUser,
}

var grantableAdminMenuPermissionSet = func() map[string]bool {
	set := make(map[string]bool, len(defaultAdminMenuPermissionKeys))
	for _, key := range defaultAdminMenuPermissionKeys {
		set[key] = true
	}
	return set
}()

type AdminMenuPermission struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index;uniqueIndex:idx_admin_menu_user_key,priority:1"`
	MenuKey     string `json:"menu_key" gorm:"type:varchar(64);not null;uniqueIndex:idx_admin_menu_user_key,priority:2"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func DefaultAdminMenuPermissionKeys() []string {
	keys := make([]string, len(defaultAdminMenuPermissionKeys))
	copy(keys, defaultAdminMenuPermissionKeys)
	return keys
}

func RootAdminMenuPermissionKeys() []string {
	keys := DefaultAdminMenuPermissionKeys()
	keys = append(keys, AdminMenuSetting)
	return keys
}

func IsGrantableAdminMenuPermissionKey(menuKey string) bool {
	return grantableAdminMenuPermissionSet[strings.TrimSpace(menuKey)]
}

func NormalizeAdminMenuPermissionKeys(menuKeys []string) ([]string, []string) {
	seen := make(map[string]bool, len(menuKeys))
	for _, menuKey := range menuKeys {
		key := strings.TrimSpace(menuKey)
		if key == "" {
			continue
		}
		seen[key] = true
	}

	normalized := make([]string, 0, len(seen))
	invalid := make([]string, 0)
	for key := range seen {
		if IsGrantableAdminMenuPermissionKey(key) {
			normalized = append(normalized, key)
		} else {
			invalid = append(invalid, key)
		}
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return adminMenuPermissionOrder(normalized[i]) < adminMenuPermissionOrder(normalized[j])
	})
	sort.Strings(invalid)
	return normalized, invalid
}

func adminMenuPermissionOrder(menuKey string) int {
	for idx, key := range defaultAdminMenuPermissionKeys {
		if key == menuKey {
			return idx
		}
	}
	return len(defaultAdminMenuPermissionKeys)
}

func GetAdminMenuPermissionKeys(userId int, role int) ([]string, error) {
	if role >= common.RoleRootUser {
		return RootAdminMenuPermissionKeys(), nil
	}
	if role < common.RoleAdminUser || userId <= 0 {
		return []string{}, nil
	}

	var rows []AdminMenuPermission
	if err := DB.Where("user_id = ?", userId).Find(&rows).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if IsGrantableAdminMenuPermissionKey(row.MenuKey) {
			keys = append(keys, row.MenuKey)
		}
	}
	normalized, _ := NormalizeAdminMenuPermissionKeys(keys)
	return normalized, nil
}

func GetAdminMenuPermissionMap(userId int, role int) (map[string]bool, error) {
	result := make(map[string]bool)
	if role >= common.RoleRootUser {
		for _, key := range RootAdminMenuPermissionKeys() {
			result[key] = true
		}
		return result, nil
	}
	if role < common.RoleAdminUser || userId <= 0 {
		return result, nil
	}

	for _, key := range defaultAdminMenuPermissionKeys {
		result[key] = false
	}
	keys, err := GetAdminMenuPermissionKeys(userId, role)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		result[key] = true
	}
	result[AdminMenuSetting] = false
	return result, nil
}

func UserHasAdminMenuPermission(userId int, role int, menuKey string) (bool, error) {
	key := strings.TrimSpace(menuKey)
	if key == "" {
		return false, nil
	}
	if role >= common.RoleRootUser {
		return true, nil
	}
	if role < common.RoleAdminUser || userId <= 0 {
		return false, nil
	}
	if !IsGrantableAdminMenuPermissionKey(key) {
		return false, nil
	}

	var count int64
	err := DB.Model(&AdminMenuPermission{}).
		Where("user_id = ? AND menu_key = ?", userId, key).
		Count(&count).Error
	return count > 0, err
}

func SetAdminMenuPermissions(userId int, menuKeys []string) error {
	if userId <= 0 {
		return nil
	}
	normalized, _ := NormalizeAdminMenuPermissionKeys(menuKeys)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&AdminMenuPermission{}).Error; err != nil {
			return err
		}
		if len(normalized) == 0 {
			return nil
		}
		now := common.GetTimestamp()
		rows := make([]AdminMenuPermission, 0, len(normalized))
		for _, key := range normalized {
			rows = append(rows, AdminMenuPermission{
				UserId:      userId,
				MenuKey:     key,
				CreatedTime: now,
				UpdatedTime: now,
			})
		}
		return tx.Create(&rows).Error
	})
}

func AddMissingDefaultAdminMenuPermissions(userId int) error {
	if userId <= 0 {
		return nil
	}
	now := common.GetTimestamp()
	rows := make([]AdminMenuPermission, 0, len(defaultAdminMenuPermissionKeys))
	for _, key := range defaultAdminMenuPermissionKeys {
		rows = append(rows, AdminMenuPermission{
			UserId:      userId,
			MenuKey:     key,
			CreatedTime: now,
			UpdatedTime: now,
		})
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func DeleteAdminMenuPermissions(userId int) error {
	if userId <= 0 {
		return nil
	}
	return DB.Where("user_id = ?", userId).Delete(&AdminMenuPermission{}).Error
}

func BackfillAdminMenuPermissionsForExistingAdmins() error {
	var users []User
	if err := DB.Select("id").Where("role = ?", common.RoleAdminUser).Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if err := AddMissingDefaultAdminMenuPermissions(user.Id); err != nil {
			return err
		}
	}
	return nil
}
