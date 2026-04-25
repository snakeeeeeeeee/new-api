package model

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteCodeStatusEnabled  = 1
	InviteCodeStatusDisabled = 2
	ManualInviteCodePrefix   = "MANUAL"
)

var (
	ErrInviteCodeNotFound         = errors.New("invite code not found")
	ErrInviteCodeDisabled         = errors.New("invite code disabled")
	ErrInviteBindingUserNotFound  = errors.New("被绑定用户不存在")
	ErrInviteBindingOwnerNotFound = errors.New("目标邀请人不存在")
	ErrInviteBindingSelf          = errors.New("不能绑定给自己")
	ErrInviteCodeOwnerMismatch    = errors.New("邀请码不属于目标邀请人")
	ErrInviteCodeUnavailable      = errors.New("邀请码不存在或已删除")
)

type InviteCode struct {
	Id                  int            `json:"id"`
	Code                string         `json:"code" gorm:"type:varchar(64);uniqueIndex"`
	Prefix              string         `json:"prefix" gorm:"type:varchar(32);index"`
	OwnerUserId         int            `json:"owner_user_id" gorm:"type:int;index"`
	TargetGroup         string         `json:"target_group" gorm:"type:varchar(64);default:'default';index"`
	RewardQuotaPerUse   int            `json:"reward_quota_per_use" gorm:"type:int;default:0"`
	RewardTotalUses     int            `json:"reward_total_uses" gorm:"type:int;default:0"`
	RewardUsedUses      int            `json:"reward_used_uses" gorm:"type:int;default:0"`
	Status              int            `json:"status" gorm:"type:int;default:1;index"`
	CreatedTime         int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
	Count               int            `json:"count,omitempty" gorm:"-:all"`
	OwnerUsername       string         `json:"owner_username,omitempty" gorm:"-"`
	InvitedUserCount    int64          `json:"invited_user_count,omitempty" gorm:"-"`
	InviteTotalRecharge float64        `json:"invite_total_recharge,omitempty" gorm:"-"`
	InviteTotalConsume  int            `json:"invite_total_consume,omitempty" gorm:"-"`
	RemainingRewardUses int            `json:"remaining_reward_uses,omitempty" gorm:"-"`
	IsDeleted           bool           `json:"is_deleted,omitempty" gorm:"-"`
}

type InviteeSummary struct {
	UserID              int     `json:"user_id"`
	Username            string  `json:"username"`
	Group               string  `json:"group"`
	InviteCodeID        int     `json:"invite_code_id"`
	InviteCode          string  `json:"invite_code"`
	InviteCodeStatus    int     `json:"invite_code_status"`
	InviteCodeDeleted   bool    `json:"invite_code_deleted"`
	InviteTotalRecharge float64 `json:"invite_total_recharge"`
	InviteTotalConsume  int     `json:"invite_total_consume"`
}

type InviteStats struct {
	InviteUserCount     int64
	InviteTotalRecharge float64
	InviteTotalConsume  int
}

type InviteBindingChange struct {
	UserID               int         `json:"user_id"`
	OldInviterID         int         `json:"old_inviter_id"`
	OldInviteCodeOwnerID int         `json:"old_invite_code_owner_id"`
	OldInviteCodeID      int         `json:"old_invite_code_id"`
	NewInviterID         int         `json:"new_inviter_id"`
	NewInviteCodeOwnerID int         `json:"new_invite_code_owner_id"`
	NewInviteCodeID      int         `json:"new_invite_code_id"`
	InviteCode           *InviteCode `json:"invite_code,omitempty"`
}

func (code *InviteCode) normalizeDerivedFields() {
	remaining := code.RewardTotalUses - code.RewardUsedUses
	if remaining < 0 {
		remaining = 0
	}
	code.RemainingRewardUses = remaining
	code.IsDeleted = code.DeletedAt.Valid
}

func (code *InviteCode) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if code.CreatedTime == 0 {
		code.CreatedTime = now
	}
	code.UpdatedTime = now
	return nil
}

func (code *InviteCode) BeforeUpdate(tx *gorm.DB) error {
	code.UpdatedTime = common.GetTimestamp()
	return nil
}

func (code *InviteCode) Insert() error {
	return DB.Create(code).Error
}

func (code *InviteCode) Update() error {
	code.normalizeDerivedFields()
	return DB.Model(code).Select(
		"owner_user_id",
		"target_group",
		"reward_quota_per_use",
		"reward_total_uses",
		"reward_used_uses",
		"status",
		"updated_time",
	).Updates(code).Error
}

func (code *InviteCode) Delete() error {
	return DB.Delete(code).Error
}

func GetInviteCodeByID(id int) (*InviteCode, error) {
	return getInviteCodeByID(id, false)
}

func GetInviteCodeByIDUnscoped(id int) (*InviteCode, error) {
	return getInviteCodeByID(id, true)
}

func getInviteCodeByID(id int, unscoped bool) (*InviteCode, error) {
	if id == 0 {
		return nil, errors.New("invite code id is empty")
	}
	var inviteCode InviteCode
	query := DB
	if unscoped {
		query = query.Unscoped()
	}
	if err := query.First(&inviteCode, "id = ?", id).Error; err != nil {
		return nil, err
	}
	inviteCode.normalizeDerivedFields()
	return &inviteCode, nil
}

func GetInviteCodeByCode(code string) (*InviteCode, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrInviteCodeNotFound
	}
	var inviteCode InviteCode
	if err := DB.First(&inviteCode, "code = ?", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInviteCodeNotFound
		}
		return nil, err
	}
	inviteCode.normalizeDerivedFields()
	return &inviteCode, nil
}

func LockInviteCodeByCodeTx(tx *gorm.DB, code string) (*InviteCode, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrInviteCodeNotFound
	}
	var inviteCode InviteCode
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("code = ?", code).First(&inviteCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInviteCodeNotFound
		}
		return nil, err
	}
	if inviteCode.Status != InviteCodeStatusEnabled {
		return nil, ErrInviteCodeDisabled
	}
	inviteCode.normalizeDerivedFields()
	return &inviteCode, nil
}

func ApplyInviteCodeRewardTx(tx *gorm.DB, userID int, inviteCode *InviteCode) (bool, error) {
	if tx == nil || inviteCode == nil || userID == 0 {
		return false, errors.New("invalid invite code reward params")
	}
	if inviteCode.RewardQuotaPerUse <= 0 || inviteCode.RewardUsedUses >= inviteCode.RewardTotalUses {
		return false, nil
	}

	if err := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", inviteCode.RewardQuotaPerUse)).Error; err != nil {
		return false, err
	}

	inviteCode.RewardUsedUses++
	if err := tx.Model(inviteCode).Select("reward_used_uses", "updated_time").Updates(inviteCode).Error; err != nil {
		return false, err
	}
	inviteCode.normalizeDerivedFields()
	return true, nil
}

func generateInviteCode(prefix string) string {
	normalizedPrefix := strings.TrimSpace(strings.ToUpper(prefix))
	normalizedPrefix = strings.TrimSuffix(normalizedPrefix, "-")
	if normalizedPrefix == "" {
		return strings.ToUpper(common.GetRandomString(6))
	}
	return normalizedPrefix + "-" + strings.ToUpper(common.GetRandomString(6))
}

func CreateInviteCodes(prefix string, count int, ownerUserID int, targetGroup string, rewardQuotaPerUse int, rewardTotalUses int, status int) ([]string, error) {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "-")
	targetGroup = strings.TrimSpace(targetGroup)
	if prefix == "" {
		return nil, errors.New("邀请码前缀不能为空")
	}
	if count <= 0 {
		return nil, errors.New("生成数量必须大于 0")
	}

	createdCodes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		var inserted bool
		for retry := 0; retry < 10; retry++ {
			code := &InviteCode{
				Code:              generateInviteCode(prefix),
				Prefix:            prefix,
				OwnerUserId:       ownerUserID,
				TargetGroup:       targetGroup,
				RewardQuotaPerUse: rewardQuotaPerUse,
				RewardTotalUses:   rewardTotalUses,
				Status:            status,
			}
			if err := code.Insert(); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
					continue
				}
				return nil, err
			}
			createdCodes = append(createdCodes, code.Code)
			inserted = true
			break
		}
		if !inserted {
			return nil, fmt.Errorf("生成邀请码失败，前缀 %s 出现重复冲突", prefix)
		}
	}
	return createdCodes, nil
}

func GetAllInviteCodes(startIdx int, num int) (inviteCodes []*InviteCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&InviteCode{})
	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&inviteCodes).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return inviteCodes, total, nil
}

func SearchInviteCodes(keyword string, startIdx int, num int) (inviteCodes []*InviteCode, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	like := "%" + strings.TrimSpace(keyword) + "%"
	query := tx.Model(&InviteCode{}).Joins("LEFT JOIN users ON users.id = invite_codes.owner_user_id")
	if id, convErr := strconv.Atoi(strings.TrimSpace(keyword)); convErr == nil {
		query = query.Where("invite_codes.id = ? OR invite_codes.code LIKE ? OR invite_codes.prefix LIKE ? OR users.username LIKE ?", id, like, like, like)
	} else {
		query = query.Where("invite_codes.code LIKE ? OR invite_codes.prefix LIKE ? OR users.username LIKE ?", like, like, like)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err = query.Order("invite_codes.id desc").Limit(num).Offset(startIdx).Find(&inviteCodes).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return inviteCodes, total, nil
}

func DeleteInviteCodeByID(id int) error {
	inviteCode, err := GetInviteCodeByID(id)
	if err != nil {
		return err
	}
	return inviteCode.Delete()
}

func loadInviteCodeOwnerUsernames(inviteCodes []*InviteCode) error {
	if len(inviteCodes) == 0 {
		return nil
	}

	ownerIDSet := make(map[int]struct{})
	for _, code := range inviteCodes {
		if code.OwnerUserId > 0 {
			ownerIDSet[code.OwnerUserId] = struct{}{}
		}
	}
	if len(ownerIDSet) == 0 {
		return nil
	}

	ownerIDs := make([]int, 0, len(ownerIDSet))
	for ownerID := range ownerIDSet {
		ownerIDs = append(ownerIDs, ownerID)
	}

	var users []User
	if err := DB.Select("id", "username").Where("id IN ?", ownerIDs).Find(&users).Error; err != nil {
		return err
	}

	usernameMap := make(map[int]string, len(users))
	for _, user := range users {
		usernameMap[user.Id] = user.Username
	}

	for _, code := range inviteCodes {
		code.OwnerUsername = usernameMap[code.OwnerUserId]
		code.normalizeDerivedFields()
	}
	return nil
}

func PopulateInviteCodeStats(inviteCodes []*InviteCode) error {
	if len(inviteCodes) == 0 {
		return nil
	}

	codeIDs := make([]int, 0, len(inviteCodes))
	for _, code := range inviteCodes {
		codeIDs = append(codeIDs, code.Id)
	}

	statsByCodeID, err := GetInviteStatsByInviteCodeIDs(codeIDs)
	if err != nil {
		return err
	}
	if err := loadInviteCodeOwnerUsernames(inviteCodes); err != nil {
		return err
	}

	for _, code := range inviteCodes {
		if stat, ok := statsByCodeID[code.Id]; ok {
			code.InvitedUserCount = stat.InviteUserCount
			code.InviteTotalRecharge = stat.InviteTotalRecharge
			code.InviteTotalConsume = stat.InviteTotalConsume
		}
		code.normalizeDerivedFields()
	}
	return nil
}

func PopulateUsersInviteStats(users []*User) error {
	if len(users) == 0 {
		return nil
	}
	userIDs := make([]int, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.Id)
	}

	statsByOwnerID, err := GetInviteStatsByOwnerUserIDs(userIDs)
	if err != nil {
		return err
	}

	for _, user := range users {
		if stat, ok := statsByOwnerID[user.Id]; ok {
			user.InviteUserCount = stat.InviteUserCount
			user.InviteTotalRecharge = stat.InviteTotalRecharge
			user.InviteTotalConsume = stat.InviteTotalConsume
		}
	}
	return nil
}

type inviteUserSummaryRow struct {
	KeyID              int   `gorm:"column:key_id"`
	InviteUserCount    int64 `gorm:"column:invite_user_count"`
	InviteTotalConsume int64 `gorm:"column:invite_total_consume"`
}

type inviteRechargeSummaryRow struct {
	KeyID               int     `gorm:"column:key_id"`
	InviteTotalRecharge float64 `gorm:"column:invite_total_recharge"`
}

func getInviteStatsByUserField(userField string, ids []int) (map[int]InviteStats, error) {
	stats := make(map[int]InviteStats, len(ids))
	if len(ids) == 0 {
		return stats, nil
	}

	var userRows []inviteUserSummaryRow
	if err := DB.Model(&User{}).
		Select(fmt.Sprintf("%s as key_id, count(*) as invite_user_count, COALESCE(sum(used_quota), 0) as invite_total_consume", userField)).
		Where(fmt.Sprintf("%s IN ? AND invite_code_id > 0", userField), ids).
		Group(userField).
		Scan(&userRows).Error; err != nil {
		return nil, err
	}
	for _, row := range userRows {
		stats[row.KeyID] = InviteStats{
			InviteUserCount:    row.InviteUserCount,
			InviteTotalConsume: int(row.InviteTotalConsume),
		}
	}

	var rechargeRows []inviteRechargeSummaryRow
	if err := DB.Table("top_ups").
		Select(fmt.Sprintf("users.%s as key_id, COALESCE(sum(top_ups.money), 0) as invite_total_recharge", userField)).
		Joins("JOIN users ON users.id = top_ups.user_id").
		Where(fmt.Sprintf("users.%s IN ? AND users.invite_code_id > 0 AND top_ups.status = ?", userField), ids, common.TopUpStatusSuccess).
		Group(fmt.Sprintf("users.%s", userField)).
		Scan(&rechargeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range rechargeRows {
		stat := stats[row.KeyID]
		stat.InviteTotalRecharge = row.InviteTotalRecharge
		stats[row.KeyID] = stat
	}
	return stats, nil
}

func GetInviteStatsByOwnerUserIDs(ownerIDs []int) (map[int]InviteStats, error) {
	return getInviteStatsByUserField("invite_code_owner_id", ownerIDs)
}

func GetInviteStatsByInviteCodeIDs(inviteCodeIDs []int) (map[int]InviteStats, error) {
	return getInviteStatsByUserField("invite_code_id", inviteCodeIDs)
}

func GetInviteCodesByOwnerUserID(ownerUserID int, startIdx int, num int) ([]*InviteCode, int64, error) {
	if ownerUserID <= 0 {
		return []*InviteCode{}, 0, nil
	}

	var inviteCodes []*InviteCode
	var total int64
	baseQuery := DB.Unscoped().Model(&InviteCode{}).Where("owner_user_id = ?", ownerUserID)
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := DB.Unscoped().Where("owner_user_id = ?", ownerUserID).Order("id desc")
	if num > 0 {
		query = query.Limit(num).Offset(startIdx)
	}
	if err := query.Find(&inviteCodes).Error; err != nil {
		return nil, 0, err
	}
	if err := PopulateInviteCodeStats(inviteCodes); err != nil {
		return nil, 0, err
	}
	return inviteCodes, total, nil
}

func GetBindableInviteCodesByOwnerUserID(ownerUserID int, startIdx int, num int) ([]*InviteCode, int64, error) {
	if ownerUserID <= 0 {
		return []*InviteCode{}, 0, nil
	}

	var inviteCodes []*InviteCode
	var total int64
	baseQuery := DB.Model(&InviteCode{}).Where("owner_user_id = ?", ownerUserID)
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := DB.Where("owner_user_id = ?", ownerUserID).Order("id desc")
	if num > 0 {
		query = query.Limit(num).Offset(startIdx)
	}
	if err := query.Find(&inviteCodes).Error; err != nil {
		return nil, 0, err
	}
	if err := PopulateInviteCodeStats(inviteCodes); err != nil {
		return nil, 0, err
	}
	return inviteCodes, total, nil
}

func manualInviteCodeValue(ownerUserID int) string {
	return fmt.Sprintf("%s-%d", ManualInviteCodePrefix, ownerUserID)
}

func manualInviteCodeTargetGroup(owner *User) string {
	targetGroup := strings.TrimSpace(owner.Group)
	if targetGroup == "" {
		return "default"
	}
	return targetGroup
}

func getOrCreateManualInviteCodeTx(tx *gorm.DB, owner *User) (*InviteCode, error) {
	codeValue := manualInviteCodeValue(owner.Id)
	targetGroup := manualInviteCodeTargetGroup(owner)
	updates := map[string]interface{}{
		"prefix":               ManualInviteCodePrefix,
		"owner_user_id":        owner.Id,
		"target_group":         targetGroup,
		"reward_quota_per_use": 0,
		"reward_total_uses":    0,
		"reward_used_uses":     0,
		"status":               InviteCodeStatusEnabled,
		"deleted_at":           nil,
		"updated_time":         common.GetTimestamp(),
	}

	var inviteCode InviteCode
	err := tx.Unscoped().Where("code = ?", codeValue).First(&inviteCode).Error
	if err == nil {
		if err := tx.Unscoped().Model(&InviteCode{}).Where("id = ?", inviteCode.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("id = ?", inviteCode.Id).First(&inviteCode).Error; err != nil {
			return nil, err
		}
		inviteCode.normalizeDerivedFields()
		return &inviteCode, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	inviteCode = InviteCode{
		Code:              codeValue,
		Prefix:            ManualInviteCodePrefix,
		OwnerUserId:       owner.Id,
		TargetGroup:       targetGroup,
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		RewardUsedUses:    0,
		Status:            InviteCodeStatusEnabled,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&inviteCode).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("code = ?", codeValue).First(&inviteCode).Error; err != nil {
		return nil, err
	}
	inviteCode.normalizeDerivedFields()
	return &inviteCode, nil
}

func BindUserInviteBinding(userID int, ownerUserID int, inviteCodeID int) (*InviteBindingChange, error) {
	var change *InviteBindingChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteBindingUserNotFound
			}
			return err
		}

		var owner User
		if err := tx.First(&owner, "id = ?", ownerUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteBindingOwnerNotFound
			}
			return err
		}

		if user.Id == owner.Id {
			return ErrInviteBindingSelf
		}

		var inviteCode *InviteCode
		if inviteCodeID > 0 {
			var selectedCode InviteCode
			if err := tx.First(&selectedCode, "id = ?", inviteCodeID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrInviteCodeUnavailable
				}
				return err
			}
			if selectedCode.OwnerUserId != owner.Id {
				return ErrInviteCodeOwnerMismatch
			}
			selectedCode.normalizeDerivedFields()
			inviteCode = &selectedCode
		} else {
			var err error
			inviteCode, err = getOrCreateManualInviteCodeTx(tx, &owner)
			if err != nil {
				return err
			}
		}

		change = &InviteBindingChange{
			UserID:               user.Id,
			OldInviterID:         user.InviterId,
			OldInviteCodeOwnerID: user.InviteCodeOwnerId,
			OldInviteCodeID:      user.InviteCodeId,
			NewInviterID:         owner.Id,
			NewInviteCodeOwnerID: owner.Id,
			NewInviteCodeID:      inviteCode.Id,
			InviteCode:           inviteCode,
		}

		return tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
			"inviter_id":           owner.Id,
			"invite_code_owner_id": owner.Id,
			"invite_code_id":       inviteCode.Id,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, invalidateUserCache(userID)
}

func UnbindUserInviteBinding(userID int) (*InviteBindingChange, error) {
	var change *InviteBindingChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteBindingUserNotFound
			}
			return err
		}

		change = &InviteBindingChange{
			UserID:               user.Id,
			OldInviterID:         user.InviterId,
			OldInviteCodeOwnerID: user.InviteCodeOwnerId,
			OldInviteCodeID:      user.InviteCodeId,
			NewInviterID:         0,
			NewInviteCodeOwnerID: 0,
			NewInviteCodeID:      0,
		}

		return tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
			"inviter_id":           0,
			"invite_code_owner_id": 0,
			"invite_code_id":       0,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return change, invalidateUserCache(userID)
}

type inviteeRechargeRow struct {
	UserID              int     `gorm:"column:user_id"`
	InviteTotalRecharge float64 `gorm:"column:invite_total_recharge"`
}

type inviteeSummaryQueryRow struct {
	UserID              int          `gorm:"column:user_id"`
	Username            string       `gorm:"column:username"`
	Group               string       `gorm:"column:user_group"`
	InviteCodeID        int          `gorm:"column:invite_code_id"`
	InviteCode          string       `gorm:"column:invite_code"`
	InviteCodeStatus    int          `gorm:"column:invite_code_status"`
	InviteCodeDeletedAt sql.NullTime `gorm:"column:invite_code_deleted_at"`
	InviteTotalConsume  int          `gorm:"column:invite_total_consume"`
}

func GetInviteeSummariesByOwnerUserID(ownerUserID int, startIdx int, limit int) ([]*InviteeSummary, int64, error) {
	if ownerUserID <= 0 {
		return []*InviteeSummary{}, 0, nil
	}
	if limit <= 0 {
		limit = 5
	}
	ensureCommonColumnsInitialized()

	var total int64
	if err := DB.Model(&User{}).
		Where("invite_code_owner_id = ? AND invite_code_id > 0", ownerUserID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []*inviteeSummaryQueryRow
	selectClause := fmt.Sprintf(
		"users.id as user_id, users.username, users.%s as user_group, users.invite_code_id, invite_codes.code as invite_code, invite_codes.status as invite_code_status, invite_codes.deleted_at as invite_code_deleted_at, users.used_quota as invite_total_consume",
		commonGroupCol,
	)
	if err := DB.Table("users").
		Select(selectClause).
		Joins("LEFT JOIN invite_codes ON invite_codes.id = users.invite_code_id").
		Where("users.invite_code_owner_id = ? AND users.invite_code_id > 0", ownerUserID).
		Order("users.id desc").
		Offset(startIdx).
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []*InviteeSummary{}, total, nil
	}

	userIDs := make([]int, 0, len(rows))
	invitees := make([]*InviteeSummary, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		invitees = append(invitees, &InviteeSummary{
			UserID:             row.UserID,
			Username:           row.Username,
			Group:              row.Group,
			InviteCodeID:       row.InviteCodeID,
			InviteCode:         row.InviteCode,
			InviteCodeStatus:   row.InviteCodeStatus,
			InviteCodeDeleted:  row.InviteCodeDeletedAt.Valid,
			InviteTotalConsume: row.InviteTotalConsume,
		})
	}

	var rechargeRows []inviteeRechargeRow
	if err := DB.Table("top_ups").
		Select("user_id, COALESCE(sum(money), 0) as invite_total_recharge").
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&rechargeRows).Error; err != nil {
		return nil, 0, err
	}

	rechargeMap := make(map[int]float64, len(rechargeRows))
	for _, row := range rechargeRows {
		rechargeMap[row.UserID] = row.InviteTotalRecharge
	}
	for _, invitee := range invitees {
		invitee.InviteTotalRecharge = rechargeMap[invitee.UserID]
	}
	return invitees, total, nil
}
