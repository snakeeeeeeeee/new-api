package model

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteCodeStatusEnabled  = 1
	InviteCodeStatusDisabled = 2
	InviteAgentLevelNone     = 0
	InviteAgentLevelFirst    = 1
	InviteAgentLevelSecond   = 2
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
	ErrInviteCodeManualOnly       = errors.New("手动绑定邀请码不能用于注册")
	ErrInviteAgentNoPermission    = errors.New("当前用户没有开通下级邀请功能的权限")
	ErrInviteAgentTargetInvalid   = errors.New("只能给自己通过邀请码直接邀请的用户开启邀请功能")
	ErrInviteAgentTargetRole      = errors.New("不能给管理员用户开启代理邀请功能")
	ErrInviteAgentAlreadyEnabled  = errors.New("该用户已开启邀请功能")
)

type InviteCode struct {
	Id                        int            `json:"id"`
	Code                      string         `json:"code" gorm:"type:varchar(64);uniqueIndex"`
	Prefix                    string         `json:"prefix" gorm:"type:varchar(32);index"`
	OwnerUserId               int            `json:"owner_user_id" gorm:"type:int;index"`
	TargetGroup               string         `json:"target_group" gorm:"type:varchar(64);default:'default';index"`
	RewardQuotaPerUse         int            `json:"reward_quota_per_use" gorm:"type:int;default:0"`
	RewardTotalUses           int            `json:"reward_total_uses" gorm:"type:int;default:0"`
	RewardUsedUses            int            `json:"reward_used_uses" gorm:"type:int;default:0"`
	AgentLevel                int            `json:"agent_level" gorm:"type:int;default:1;index"`
	GrantedByUserId           int            `json:"granted_by_user_id" gorm:"type:int;default:0;index"`
	Status                    int            `json:"status" gorm:"type:int;default:1;index"`
	CreatedTime               int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime               int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt                 gorm.DeletedAt `gorm:"index"`
	Count                     int            `json:"count,omitempty" gorm:"-:all"`
	OwnerUsername             string         `json:"owner_username,omitempty" gorm:"-"`
	InvitedUserCount          int64          `json:"invited_user_count,omitempty" gorm:"-"`
	InviteTotalRecharge       float64        `json:"invite_total_recharge,omitempty" gorm:"-"`
	InviteTotalRechargeAmount int64          `json:"invite_total_recharge_amount,omitempty" gorm:"-"`
	InviteTotalRechargeMoney  float64        `json:"invite_total_recharge_money,omitempty" gorm:"-"`
	InviteTotalConsume        int            `json:"invite_total_consume,omitempty" gorm:"-"`
	RemainingRewardUses       int            `json:"remaining_reward_uses,omitempty" gorm:"-"`
	GrantsInvitePermission    bool           `json:"grants_invite_permission" gorm:"-"`
	IsDeleted                 bool           `json:"is_deleted,omitempty" gorm:"-"`
	IsManual                  bool           `json:"is_manual,omitempty" gorm:"-"`
}

type InviteeSummary struct {
	UserID                    int     `json:"user_id"`
	Username                  string  `json:"username"`
	Group                     string  `json:"group"`
	InviteCodeID              int     `json:"invite_code_id"`
	InviteCode                string  `json:"invite_code"`
	InviteCodeStatus          int     `json:"invite_code_status"`
	InviteCodeDeleted         bool    `json:"invite_code_deleted"`
	InviteTotalRecharge       float64 `json:"invite_total_recharge"`
	InviteTotalRechargeAmount int64   `json:"invite_total_recharge_amount"`
	InviteTotalRechargeMoney  float64 `json:"invite_total_recharge_money"`
	InviteTotalConsume        int     `json:"invite_total_consume"`
	InviteAgentLevel          int     `json:"invite_agent_level"`
	CanGrantInvitation        bool    `json:"can_grant_invitation"`
	InvitationEnabled         bool    `json:"invitation_enabled"`
	InvitationInviteCodeID    int     `json:"invitation_invite_code_id"`
	InvitationInviteCode      string  `json:"invitation_invite_code"`
}

type InviteStats struct {
	InviteUserCount           int64
	InviteTotalRecharge       float64
	InviteTotalRechargeAmount int64
	InviteTotalRechargeMoney  float64
	InviteTotalConsume        int
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

type InviteAgentTrendPoint struct {
	BucketStart    int64   `json:"bucket_start"`
	Label          string  `json:"label"`
	RechargeAmount int64   `json:"recharge_amount"`
	RechargeMoney  float64 `json:"recharge_money"`
	ConsumeQuota   int     `json:"consume_quota"`
}

type InviteAgentUserFlowStats struct {
	UserCount      int64   `json:"user_count"`
	RechargeAmount int64   `json:"recharge_amount"`
	RechargeMoney  float64 `json:"recharge_money"`
	ConsumeQuota   int     `json:"consume_quota"`
}

type InviteAgentSecondLevelStats struct {
	UserID       int                      `json:"user_id"`
	Username     string                   `json:"username"`
	Group        string                   `json:"group"`
	InviteCodeID int                      `json:"invite_code_id"`
	InviteCode   string                   `json:"invite_code"`
	SelfStats    InviteAgentUserFlowStats `json:"self_stats"`
	InviteeStats InviteAgentUserFlowStats `json:"invitee_stats"`
}

type InviteAgentStatsResponse struct {
	AgentLevel         int                           `json:"agent_level"`
	CanGrantInvitation bool                          `json:"can_grant_invitation"`
	Period             string                        `json:"period"`
	StartTime          int64                         `json:"start_time"`
	EndTime            int64                         `json:"end_time"`
	DirectStats        InviteAgentUserFlowStats      `json:"direct_stats"`
	DirectTrend        []InviteAgentTrendPoint       `json:"direct_trend"`
	SecondLevelStats   []InviteAgentSecondLevelStats `json:"second_level_stats"`
	SecondLevelTrend   []InviteAgentTrendPoint       `json:"second_level_trend"`
}

type InviteConsumptionInviter struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
}

type InviteConsumptionSummary struct {
	InviteUserCount                  int64   `json:"invite_user_count"`
	WalletAmount                     float64 `json:"wallet_amount"`
	WalletQuota                      int64   `json:"wallet_quota"`
	RequestCount                     int64   `json:"request_count"`
	ModelCount                       int     `json:"model_count"`
	ExcludedSubscriptionQuota        int64   `json:"excluded_subscription_quota"`
	ExcludedSubscriptionRequestCount int64   `json:"excluded_subscription_request_count"`
}

type InviteConsumptionModelStat struct {
	ModelName    string  `json:"model_name"`
	Quota        int64   `json:"quota"`
	Amount       float64 `json:"amount"`
	RequestCount int64   `json:"request_count"`
	Percent      float64 `json:"percent"`
}

type InviteConsumptionTrendPoint struct {
	BucketStart  int64   `json:"bucket_start"`
	Label        string  `json:"label"`
	Quota        int64   `json:"quota"`
	Amount       float64 `json:"amount"`
	RequestCount int64   `json:"request_count"`
}

type InviteSubscriptionUsageSummary struct {
	Quota        int64   `json:"quota"`
	Amount       float64 `json:"amount"`
	RequestCount int64   `json:"request_count"`
	ModelCount   int     `json:"model_count"`
}

type InviteSubscriptionUsageStats struct {
	Summary InviteSubscriptionUsageSummary `json:"summary"`
	Models  []InviteConsumptionModelStat   `json:"models"`
	Trend   []InviteConsumptionTrendPoint  `json:"trend"`
}

type InviteSubscriptionPurchaseSummary struct {
	Amount     float64 `json:"amount"`
	OrderCount int64   `json:"order_count"`
	BuyerCount int64   `json:"buyer_count"`
	PlanCount  int     `json:"plan_count"`
}

type InviteSubscriptionPlanStat struct {
	PlanId     int     `json:"plan_id"`
	PlanTitle  string  `json:"plan_title"`
	Amount     float64 `json:"amount"`
	OrderCount int64   `json:"order_count"`
	BuyerCount int64   `json:"buyer_count"`
	Percent    float64 `json:"percent"`
}

type InviteSubscriptionPurchaseTrendPoint struct {
	BucketStart int64   `json:"bucket_start"`
	Label       string  `json:"label"`
	Amount      float64 `json:"amount"`
	OrderCount  int64   `json:"order_count"`
	BuyerCount  int64   `json:"buyer_count"`
}

type InviteSubscriptionPurchaseStats struct {
	Summary InviteSubscriptionPurchaseSummary      `json:"summary"`
	Plans   []InviteSubscriptionPlanStat           `json:"plans"`
	Trend   []InviteSubscriptionPurchaseTrendPoint `json:"trend"`
}

type InviteConsumptionStatsResponse struct {
	Inviter                          InviteConsumptionInviter        `json:"inviter"`
	StartTime                        int64                           `json:"start_time"`
	EndTime                          int64                           `json:"end_time"`
	Summary                          InviteConsumptionSummary        `json:"summary"`
	Models                           []InviteConsumptionModelStat    `json:"models"`
	Trend                            []InviteConsumptionTrendPoint   `json:"trend"`
	SubscriptionUsage                InviteSubscriptionUsageStats    `json:"subscription_usage"`
	SubscriptionPurchase             InviteSubscriptionPurchaseStats `json:"subscription_purchase"`
	ExcludedSubscriptionQuota        int64                           `json:"excluded_subscription_quota"`
	ExcludedSubscriptionRequestCount int64                           `json:"excluded_subscription_request_count"`
}

type InviteConsumptionBreakdown struct {
	OwnerId                  int   `json:"owner_id"`
	InviteUserCount          int64 `json:"invite_user_count"`
	WalletQuota              int64 `json:"wallet_quota"`
	SubscriptionQuota        int64 `json:"subscription_quota"`
	LogTotalQuota            int64 `json:"log_total_quota"`
	TotalUsedQuota           int64 `json:"total_used_quota"`
	WalletRequestCount       int64 `json:"wallet_request_count"`
	SubscriptionRequestCount int64 `json:"subscription_request_count"`
	LogRequestCount          int64 `json:"log_request_count"`
}

const MaxInviteConsumptionBreakdownOwnerIDs = 100

func (code *InviteCode) normalizeDerivedFields() {
	remaining := code.RewardTotalUses - code.RewardUsedUses
	if remaining < 0 {
		remaining = 0
	}
	code.RemainingRewardUses = remaining
	code.IsDeleted = code.DeletedAt.Valid
	code.IsManual = isManualInviteCode(code)
	if code.IsManual {
		code.GrantsInvitePermission = false
		return
	}
	if code.AgentLevel <= InviteAgentLevelNone {
		code.AgentLevel = InviteAgentLevelFirst
	}
	code.GrantsInvitePermission = code.Status == InviteCodeStatusEnabled && !code.IsDeleted && code.AgentLevel > InviteAgentLevelNone
}

func isManualInviteCode(code *InviteCode) bool {
	if code == nil {
		return false
	}
	prefix := strings.TrimSpace(strings.ToUpper(code.Prefix))
	codeValue := strings.TrimSpace(strings.ToUpper(code.Code))
	return prefix == ManualInviteCodePrefix || strings.HasPrefix(codeValue, ManualInviteCodePrefix+"-")
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
		"agent_level",
		"granted_by_user_id",
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
	if isManualInviteCode(&inviteCode) {
		return nil, ErrInviteCodeManualOnly
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
				AgentLevel:        InviteAgentLevelFirst,
				GrantedByUserId:   0,
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
			code.InviteTotalRechargeAmount = stat.InviteTotalRechargeAmount
			code.InviteTotalRechargeMoney = stat.InviteTotalRechargeMoney
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
			user.InviteTotalRechargeAmount = stat.InviteTotalRechargeAmount
			user.InviteTotalRechargeMoney = stat.InviteTotalRechargeMoney
			user.InviteTotalConsume = stat.InviteTotalConsume
		}
	}
	if err := loadUserInviterUsernames(users); err != nil {
		return err
	}
	return nil
}

func loadUserInviterUsernames(users []*User) error {
	inviterIDSet := make(map[int]struct{})
	for _, user := range users {
		if user.InviterId > 0 {
			inviterIDSet[user.InviterId] = struct{}{}
		}
	}
	if len(inviterIDSet) == 0 {
		return nil
	}

	inviterIDs := make([]int, 0, len(inviterIDSet))
	for inviterID := range inviterIDSet {
		inviterIDs = append(inviterIDs, inviterID)
	}

	var inviters []User
	if err := DB.Select("id", "username").Where("id IN ?", inviterIDs).Find(&inviters).Error; err != nil {
		return err
	}

	usernameMap := make(map[int]string, len(inviters))
	for _, inviter := range inviters {
		usernameMap[inviter.Id] = inviter.Username
	}
	for _, user := range users {
		user.InviterUsername = usernameMap[user.InviterId]
	}
	return nil
}

type inviteUserSummaryRow struct {
	KeyID              int   `gorm:"column:key_id"`
	InviteUserCount    int64 `gorm:"column:invite_user_count"`
	InviteTotalConsume int64 `gorm:"column:invite_total_consume"`
}

type inviteRechargeSummaryRow struct {
	KeyID                     int     `gorm:"column:key_id"`
	InviteTotalRechargeAmount int64   `gorm:"column:invite_total_recharge_amount"`
	InviteTotalRechargeMoney  float64 `gorm:"column:invite_total_recharge_money"`
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
		Select(fmt.Sprintf("users.%s as key_id, COALESCE(sum(top_ups.amount), 0) as invite_total_recharge_amount, COALESCE(sum(top_ups.money), 0) as invite_total_recharge_money", userField)).
		Joins("JOIN users ON users.id = top_ups.user_id").
		Where(fmt.Sprintf("users.%s IN ? AND users.invite_code_id > 0 AND top_ups.status = ?", userField), ids, common.TopUpStatusSuccess).
		Group(fmt.Sprintf("users.%s", userField)).
		Scan(&rechargeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range rechargeRows {
		stat := stats[row.KeyID]
		stat.InviteTotalRecharge = row.InviteTotalRechargeMoney
		stat.InviteTotalRechargeAmount = row.InviteTotalRechargeAmount
		stat.InviteTotalRechargeMoney = row.InviteTotalRechargeMoney
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

type inviteConsumptionLogRow struct {
	UserId    int    `gorm:"column:user_id"`
	ModelName string `gorm:"column:model_name"`
	Quota     int    `gorm:"column:quota"`
	CreatedAt int64  `gorm:"column:created_at"`
	Other     string `gorm:"column:other"`
}

type inviteOwnerUserRow struct {
	OwnerID   int   `gorm:"column:owner_id"`
	UserID    int   `gorm:"column:user_id"`
	UsedQuota int64 `gorm:"column:used_quota"`
}

func GetInviteConsumptionStats(username string, startTime int64, endTime int64) (*InviteConsumptionStatsResponse, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("邀请人用户名不能为空")
	}
	if startTime <= 0 || endTime <= 0 {
		return nil, errors.New("时间范围不能为空")
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}

	inviter, err := GetUserByUsername(username, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("邀请人不存在")
		}
		return nil, err
	}

	var invitees []struct {
		Id int `gorm:"column:id"`
	}
	if err := DB.Model(&User{}).
		Select("id").
		Where("invite_code_owner_id = ? AND invite_code_id > 0", inviter.Id).
		Find(&invitees).Error; err != nil {
		return nil, err
	}

	resp := &InviteConsumptionStatsResponse{
		Inviter: InviteConsumptionInviter{
			Id:       inviter.Id,
			Username: inviter.Username,
		},
		StartTime: startTime,
		EndTime:   endTime,
		Summary: InviteConsumptionSummary{
			InviteUserCount: int64(len(invitees)),
		},
		Models: []InviteConsumptionModelStat{},
		Trend:  emptyInviteConsumptionTrend(startTime, endTime),
		SubscriptionUsage: InviteSubscriptionUsageStats{
			Models: []InviteConsumptionModelStat{},
			Trend:  emptyInviteConsumptionTrend(startTime, endTime),
		},
		SubscriptionPurchase: InviteSubscriptionPurchaseStats{
			Plans: []InviteSubscriptionPlanStat{},
			Trend: emptyInviteSubscriptionPurchaseTrend(startTime, endTime),
		},
	}
	if len(invitees) == 0 {
		return resp, nil
	}

	inviteeIDs := make([]int, 0, len(invitees))
	for _, invitee := range invitees {
		if invitee.Id > 0 {
			inviteeIDs = append(inviteeIDs, invitee.Id)
		}
	}
	if len(inviteeIDs) == 0 {
		return resp, nil
	}

	var rows []inviteConsumptionLogRow
	if err := LOG_DB.Model(&Log{}).
		Select("user_id, model_name, quota, created_at, other").
		Where("user_id IN ? AND type = ? AND created_at >= ? AND created_at <= ?", inviteeIDs, LogTypeConsume, startTime, endTime).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	modelStats := make(map[string]*InviteConsumptionModelStat)
	subscriptionModelStats := make(map[string]*InviteConsumptionModelStat)
	trendIndex := inviteConsumptionTrendIndex(resp.Trend)
	subscriptionTrendIndex := inviteConsumptionTrendIndex(resp.SubscriptionUsage.Trend)
	for _, row := range rows {
		quota := int64(row.Quota)
		if quota <= 0 {
			continue
		}
		if inviteConsumptionLogIsSubscription(row.Other) {
			resp.Summary.ExcludedSubscriptionQuota += quota
			resp.Summary.ExcludedSubscriptionRequestCount++
			modelName := normalizeInviteConsumptionModelName(row.ModelName)
			stat := subscriptionModelStats[modelName]
			if stat == nil {
				stat = &InviteConsumptionModelStat{ModelName: modelName}
				subscriptionModelStats[modelName] = stat
			}
			stat.Quota += quota
			stat.RequestCount++
			resp.SubscriptionUsage.Summary.Quota += quota
			resp.SubscriptionUsage.Summary.RequestCount++
			bucket, _ := bucketInviteTime(row.CreatedAt, "day")
			if idx, ok := subscriptionTrendIndex[bucket]; ok {
				resp.SubscriptionUsage.Trend[idx].Quota += quota
				resp.SubscriptionUsage.Trend[idx].RequestCount++
			}
			continue
		}

		modelName := normalizeInviteConsumptionModelName(row.ModelName)
		stat := modelStats[modelName]
		if stat == nil {
			stat = &InviteConsumptionModelStat{ModelName: modelName}
			modelStats[modelName] = stat
		}
		stat.Quota += quota
		stat.RequestCount++
		resp.Summary.WalletQuota += quota
		resp.Summary.RequestCount++

		bucket, _ := bucketInviteTime(row.CreatedAt, "day")
		if idx, ok := trendIndex[bucket]; ok {
			resp.Trend[idx].Quota += quota
			resp.Trend[idx].RequestCount++
		}
	}

	resp.Summary.WalletAmount = quotaToInviteConsumptionAmount(resp.Summary.WalletQuota)
	resp.Summary.ModelCount = len(modelStats)
	resp.ExcludedSubscriptionQuota = resp.Summary.ExcludedSubscriptionQuota
	resp.ExcludedSubscriptionRequestCount = resp.Summary.ExcludedSubscriptionRequestCount
	resp.Models = inviteConsumptionModelStatsToSlice(modelStats, resp.Summary.WalletQuota)
	resp.SubscriptionUsage.Summary.Amount = quotaToInviteConsumptionAmount(resp.SubscriptionUsage.Summary.Quota)
	resp.SubscriptionUsage.Summary.ModelCount = len(subscriptionModelStats)
	resp.SubscriptionUsage.Models = inviteConsumptionModelStatsToSlice(subscriptionModelStats, resp.SubscriptionUsage.Summary.Quota)
	for i := range resp.Trend {
		resp.Trend[i].Amount = quotaToInviteConsumptionAmount(resp.Trend[i].Quota)
	}
	for i := range resp.SubscriptionUsage.Trend {
		resp.SubscriptionUsage.Trend[i].Amount = quotaToInviteConsumptionAmount(resp.SubscriptionUsage.Trend[i].Quota)
	}
	if err := populateInviteSubscriptionPurchaseStats(resp, inviteeIDs, startTime, endTime); err != nil {
		return nil, err
	}
	return resp, nil
}

func GetInviteConsumptionBreakdownByOwnerIDs(ownerIDs []int) (map[int]InviteConsumptionBreakdown, error) {
	ownerIDs = normalizePositiveUniqueIDs(ownerIDs, MaxInviteConsumptionBreakdownOwnerIDs)
	result := make(map[int]InviteConsumptionBreakdown, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return result, nil
	}
	for _, ownerID := range ownerIDs {
		result[ownerID] = InviteConsumptionBreakdown{OwnerId: ownerID}
	}

	inviteesByOwner, inviteeOwnerMap, err := getInviteOwnerRows(ownerIDs)
	if err != nil {
		return nil, err
	}
	inviteeIDs := make([]int, 0, len(inviteeOwnerMap))
	for _, ownerID := range ownerIDs {
		rows := inviteesByOwner[ownerID]
		breakdown := result[ownerID]
		breakdown.InviteUserCount = int64(len(rows))
		for _, row := range rows {
			breakdown.TotalUsedQuota += row.UsedQuota
		}
		result[ownerID] = breakdown
	}
	for inviteeID := range inviteeOwnerMap {
		inviteeIDs = append(inviteeIDs, inviteeID)
	}
	if len(inviteeIDs) == 0 {
		return result, nil
	}

	var rows []inviteConsumptionLogRow
	if err := LOG_DB.Model(&Log{}).
		Select("user_id, quota, other").
		Where("user_id IN ? AND type = ?", inviteeIDs, LogTypeConsume).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		quota := int64(row.Quota)
		if quota <= 0 {
			continue
		}
		ownerID, ok := inviteeOwnerMap[row.UserId]
		if !ok {
			continue
		}
		breakdown := result[ownerID]
		breakdown.LogTotalQuota += quota
		breakdown.LogRequestCount++
		if inviteConsumptionLogIsSubscription(row.Other) {
			breakdown.SubscriptionQuota += quota
			breakdown.SubscriptionRequestCount++
		} else {
			breakdown.WalletQuota += quota
			breakdown.WalletRequestCount++
		}
		result[ownerID] = breakdown
	}
	return result, nil
}

func getInviteOwnerRows(ownerIDs []int) (map[int][]inviteOwnerUserRow, map[int]int, error) {
	inviteesByOwner := make(map[int][]inviteOwnerUserRow, len(ownerIDs))
	inviteeOwnerMap := make(map[int]int)
	if len(ownerIDs) == 0 {
		return inviteesByOwner, inviteeOwnerMap, nil
	}
	var rows []inviteOwnerUserRow
	if err := DB.Model(&User{}).
		Select("invite_code_owner_id as owner_id, id as user_id, used_quota").
		Where("invite_code_owner_id IN ? AND invite_code_id > 0", ownerIDs).
		Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		if row.OwnerID <= 0 || row.UserID <= 0 {
			continue
		}
		inviteesByOwner[row.OwnerID] = append(inviteesByOwner[row.OwnerID], row)
		inviteeOwnerMap[row.UserID] = row.OwnerID
	}
	return inviteesByOwner, inviteeOwnerMap, nil
}

func inviteConsumptionModelStatsToSlice(statsMap map[string]*InviteConsumptionModelStat, totalQuota int64) []InviteConsumptionModelStat {
	items := make([]InviteConsumptionModelStat, 0, len(statsMap))
	for _, stat := range statsMap {
		stat.Amount = quotaToInviteConsumptionAmount(stat.Quota)
		if totalQuota > 0 {
			stat.Percent = float64(stat.Quota) * 100 / float64(totalQuota)
		}
		items = append(items, *stat)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Quota == items[j].Quota {
			if items[i].RequestCount != items[j].RequestCount {
				return items[i].RequestCount > items[j].RequestCount
			}
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].Quota > items[j].Quota
	})
	return items
}

type inviteSubscriptionOrderRow struct {
	UserId       int     `gorm:"column:user_id"`
	PlanId       int     `gorm:"column:plan_id"`
	Money        float64 `gorm:"column:money"`
	CompleteTime int64   `gorm:"column:complete_time"`
}

func populateInviteSubscriptionPurchaseStats(resp *InviteConsumptionStatsResponse, inviteeIDs []int, startTime int64, endTime int64) error {
	if resp == nil || len(inviteeIDs) == 0 {
		return nil
	}
	var rows []inviteSubscriptionOrderRow
	if err := DB.Model(&SubscriptionOrder{}).
		Select("user_id, plan_id, money, complete_time").
		Where("user_id IN ? AND status = ? AND complete_time >= ? AND complete_time <= ?", inviteeIDs, common.TopUpStatusSuccess, startTime, endTime).
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	planStats := make(map[int]*InviteSubscriptionPlanStat)
	buyers := make(map[int]struct{})
	trendBuyers := make(map[int64]map[int]struct{})
	planBuyers := make(map[int]map[int]struct{})
	trendIndex := inviteSubscriptionPurchaseTrendIndex(resp.SubscriptionPurchase.Trend)
	planIDs := make([]int, 0)
	planIDSeen := make(map[int]struct{})
	for _, row := range rows {
		if row.Money < 0 {
			continue
		}
		resp.SubscriptionPurchase.Summary.Amount += row.Money
		resp.SubscriptionPurchase.Summary.OrderCount++
		buyers[row.UserId] = struct{}{}
		if _, ok := planIDSeen[row.PlanId]; !ok && row.PlanId > 0 {
			planIDSeen[row.PlanId] = struct{}{}
			planIDs = append(planIDs, row.PlanId)
		}

		stat := planStats[row.PlanId]
		if stat == nil {
			stat = &InviteSubscriptionPlanStat{PlanId: row.PlanId, PlanTitle: fallbackInviteSubscriptionPlanTitle(row.PlanId)}
			planStats[row.PlanId] = stat
		}
		stat.Amount += row.Money
		stat.OrderCount++
		if planBuyers[row.PlanId] == nil {
			planBuyers[row.PlanId] = make(map[int]struct{})
		}
		planBuyers[row.PlanId][row.UserId] = struct{}{}

		bucket, _ := bucketInviteTime(row.CompleteTime, "day")
		if idx, ok := trendIndex[bucket]; ok {
			resp.SubscriptionPurchase.Trend[idx].Amount += row.Money
			resp.SubscriptionPurchase.Trend[idx].OrderCount++
			if trendBuyers[bucket] == nil {
				trendBuyers[bucket] = make(map[int]struct{})
			}
			trendBuyers[bucket][row.UserId] = struct{}{}
		}
	}
	planTitles, err := getInviteSubscriptionPlanTitles(planIDs)
	if err != nil {
		return err
	}
	for planID, title := range planTitles {
		if stat := planStats[planID]; stat != nil {
			stat.PlanTitle = title
		}
	}
	resp.SubscriptionPurchase.Summary.BuyerCount = int64(len(buyers))
	resp.SubscriptionPurchase.Summary.PlanCount = len(planStats)
	resp.SubscriptionPurchase.Plans = make([]InviteSubscriptionPlanStat, 0, len(planStats))
	for planID, stat := range planStats {
		stat.BuyerCount = int64(len(planBuyers[planID]))
		if resp.SubscriptionPurchase.Summary.Amount > 0 {
			stat.Percent = stat.Amount * 100 / resp.SubscriptionPurchase.Summary.Amount
		}
		resp.SubscriptionPurchase.Plans = append(resp.SubscriptionPurchase.Plans, *stat)
	}
	sort.Slice(resp.SubscriptionPurchase.Plans, func(i, j int) bool {
		if resp.SubscriptionPurchase.Plans[i].Amount == resp.SubscriptionPurchase.Plans[j].Amount {
			if resp.SubscriptionPurchase.Plans[i].OrderCount != resp.SubscriptionPurchase.Plans[j].OrderCount {
				return resp.SubscriptionPurchase.Plans[i].OrderCount > resp.SubscriptionPurchase.Plans[j].OrderCount
			}
			return resp.SubscriptionPurchase.Plans[i].PlanTitle < resp.SubscriptionPurchase.Plans[j].PlanTitle
		}
		return resp.SubscriptionPurchase.Plans[i].Amount > resp.SubscriptionPurchase.Plans[j].Amount
	})
	for i := range resp.SubscriptionPurchase.Trend {
		bucket := resp.SubscriptionPurchase.Trend[i].BucketStart
		resp.SubscriptionPurchase.Trend[i].BuyerCount = int64(len(trendBuyers[bucket]))
	}
	return nil
}

func getInviteSubscriptionPlanTitles(planIDs []int) (map[int]string, error) {
	titles := make(map[int]string)
	planIDs = normalizePositiveUniqueIDs(planIDs, 0)
	if len(planIDs) == 0 {
		return titles, nil
	}
	var plans []SubscriptionPlan
	if err := DB.Model(&SubscriptionPlan{}).Select("id", "title").Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
		return nil, err
	}
	for _, plan := range plans {
		title := strings.TrimSpace(plan.Title)
		if title == "" {
			title = fallbackInviteSubscriptionPlanTitle(plan.Id)
		}
		titles[plan.Id] = title
	}
	return titles, nil
}

func fallbackInviteSubscriptionPlanTitle(planID int) string {
	if planID <= 0 {
		return "-"
	}
	return fmt.Sprintf("#%d", planID)
}

func normalizePositiveUniqueIDs(ids []int, limit int) []int {
	if len(ids) == 0 {
		return []int{}
	}
	result := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func inviteConsumptionLogIsSubscription(other string) bool {
	other = strings.TrimSpace(other)
	if other == "" {
		return false
	}
	otherMap, err := common.StrToMap(other)
	if err != nil || otherMap == nil {
		return false
	}
	raw, ok := otherMap["billing_source"]
	if !ok || raw == nil {
		return false
	}
	value, ok := raw.(string)
	return ok && strings.TrimSpace(value) == "subscription"
}

func normalizeInviteConsumptionModelName(modelName string) string {
	normalized := strings.TrimSpace(modelName)
	if normalized == "" {
		return "-"
	}
	return normalized
}

func quotaToInviteConsumptionAmount(quota int64) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func emptyInviteConsumptionTrend(startTime int64, endTime int64) []InviteConsumptionTrendPoint {
	startBucket, _ := bucketInviteTime(startTime, "day")
	endBucket, _ := bucketInviteTime(endTime, "day")
	points := make([]InviteConsumptionTrendPoint, 0)
	for cursor := time.Unix(startBucket, 0); !cursor.After(time.Unix(endBucket, 0)); cursor = cursor.AddDate(0, 0, 1) {
		bucketStart := cursor.Unix()
		_, label := bucketInviteTime(bucketStart, "day")
		points = append(points, InviteConsumptionTrendPoint{
			BucketStart: bucketStart,
			Label:       label,
		})
	}
	return points
}

func emptyInviteSubscriptionPurchaseTrend(startTime int64, endTime int64) []InviteSubscriptionPurchaseTrendPoint {
	startBucket, _ := bucketInviteTime(startTime, "day")
	endBucket, _ := bucketInviteTime(endTime, "day")
	points := make([]InviteSubscriptionPurchaseTrendPoint, 0)
	for cursor := time.Unix(startBucket, 0); !cursor.After(time.Unix(endBucket, 0)); cursor = cursor.AddDate(0, 0, 1) {
		bucketStart := cursor.Unix()
		_, label := bucketInviteTime(bucketStart, "day")
		points = append(points, InviteSubscriptionPurchaseTrendPoint{
			BucketStart: bucketStart,
			Label:       label,
		})
	}
	return points
}

func inviteConsumptionTrendIndex(points []InviteConsumptionTrendPoint) map[int64]int {
	index := make(map[int64]int, len(points))
	for i := range points {
		index[points[i].BucketStart] = i
	}
	return index
}

func inviteSubscriptionPurchaseTrendIndex(points []InviteSubscriptionPurchaseTrendPoint) map[int64]int {
	index := make(map[int64]int, len(points))
	for i := range points {
		index[points[i].BucketStart] = i
	}
	return index
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
		"agent_level":          InviteAgentLevelNone,
		"granted_by_user_id":   0,
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
		AgentLevel:        InviteAgentLevelNone,
		GrantedByUserId:   0,
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
	UserID                    int     `gorm:"column:user_id"`
	InviteTotalRechargeAmount int64   `gorm:"column:invite_total_recharge_amount"`
	InviteTotalRechargeMoney  float64 `gorm:"column:invite_total_recharge_money"`
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
	InviteAgentLevel    int          `gorm:"column:invite_agent_level"`
	InvitationCodeID    int          `gorm:"column:invitation_invite_code_id"`
	InvitationCode      string       `gorm:"column:invitation_invite_code"`
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
	if err := populateInviteeAgentFields(invitees); err != nil {
		return nil, 0, err
	}

	var rechargeRows []inviteeRechargeRow
	if err := DB.Table("top_ups").
		Select("user_id, COALESCE(sum(amount), 0) as invite_total_recharge_amount, COALESCE(sum(money), 0) as invite_total_recharge_money").
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&rechargeRows).Error; err != nil {
		return nil, 0, err
	}

	rechargeMap := make(map[int]inviteeRechargeRow, len(rechargeRows))
	for _, row := range rechargeRows {
		rechargeMap[row.UserID] = row
	}
	for _, invitee := range invitees {
		if row, ok := rechargeMap[invitee.UserID]; ok {
			invitee.InviteTotalRecharge = row.InviteTotalRechargeMoney
			invitee.InviteTotalRechargeAmount = row.InviteTotalRechargeAmount
			invitee.InviteTotalRechargeMoney = row.InviteTotalRechargeMoney
		}
	}
	return invitees, total, nil
}

func GetUserInviteAgentLevel(userID int) (int, error) {
	if userID <= 0 {
		return InviteAgentLevelNone, nil
	}
	var codes []InviteCode
	if err := DB.Where("owner_user_id = ? AND status = ?", userID, InviteCodeStatusEnabled).Find(&codes).Error; err != nil {
		return InviteAgentLevelNone, err
	}
	level := InviteAgentLevelNone
	for i := range codes {
		codes[i].normalizeDerivedFields()
		if codes[i].IsManual || !codes[i].GrantsInvitePermission {
			continue
		}
		if level == InviteAgentLevelNone || codes[i].AgentLevel < level {
			level = codes[i].AgentLevel
		}
	}
	return level, nil
}

func UserCanGrantInvitation(userID int) (bool, int, error) {
	level, err := GetUserInviteAgentLevel(userID)
	if err != nil {
		return false, InviteAgentLevelNone, err
	}
	return level == InviteAgentLevelFirst, level, nil
}

func populateInviteeAgentFields(invitees []*InviteeSummary) error {
	if len(invitees) == 0 {
		return nil
	}
	userIDs := make([]int, 0, len(invitees))
	for _, invitee := range invitees {
		if invitee != nil && invitee.UserID > 0 {
			userIDs = append(userIDs, invitee.UserID)
		}
	}
	if len(userIDs) == 0 {
		return nil
	}
	var codes []InviteCode
	if err := DB.Where("owner_user_id IN ? AND status = ?", userIDs, InviteCodeStatusEnabled).Find(&codes).Error; err != nil {
		return err
	}
	bestByOwner := make(map[int]*InviteCode)
	for i := range codes {
		codes[i].normalizeDerivedFields()
		if codes[i].IsManual || !codes[i].GrantsInvitePermission {
			continue
		}
		existing := bestByOwner[codes[i].OwnerUserId]
		if existing == nil || codes[i].AgentLevel < existing.AgentLevel || (codes[i].AgentLevel == existing.AgentLevel && codes[i].Id < existing.Id) {
			codeCopy := codes[i]
			bestByOwner[codes[i].OwnerUserId] = &codeCopy
		}
	}
	for _, invitee := range invitees {
		if invitee == nil {
			continue
		}
		code := bestByOwner[invitee.UserID]
		if code == nil {
			invitee.InviteAgentLevel = InviteAgentLevelNone
			invitee.CanGrantInvitation = false
			invitee.InvitationEnabled = false
			continue
		}
		invitee.InviteAgentLevel = code.AgentLevel
		invitee.CanGrantInvitation = code.AgentLevel == InviteAgentLevelFirst
		invitee.InvitationEnabled = true
		invitee.InvitationInviteCodeID = code.Id
		invitee.InvitationInviteCode = code.Code
	}
	return nil
}

func EnableInviteeInvitation(ownerUserID int, targetUserID int) (*InviteCode, error) {
	if ownerUserID <= 0 || targetUserID <= 0 {
		return nil, ErrInviteAgentTargetInvalid
	}
	var createdCode *InviteCode
	err := DB.Transaction(func(tx *gorm.DB) error {
		var ownerCodes []InviteCode
		if err := tx.Where("owner_user_id = ? AND status = ?", ownerUserID, InviteCodeStatusEnabled).Find(&ownerCodes).Error; err != nil {
			return err
		}
		ownerLevel := InviteAgentLevelNone
		for i := range ownerCodes {
			ownerCodes[i].normalizeDerivedFields()
			if ownerCodes[i].IsManual || !ownerCodes[i].GrantsInvitePermission {
				continue
			}
			if ownerLevel == InviteAgentLevelNone || ownerCodes[i].AgentLevel < ownerLevel {
				ownerLevel = ownerCodes[i].AgentLevel
			}
		}
		if ownerLevel != InviteAgentLevelFirst {
			return ErrInviteAgentNoPermission
		}

		var target User
		if err := tx.First(&target, "id = ?", targetUserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteBindingUserNotFound
			}
			return err
		}
		if target.Role >= common.RoleAdminUser {
			return ErrInviteAgentTargetRole
		}
		if target.InviteCodeOwnerId != ownerUserID || target.InviteCodeId <= 0 {
			return ErrInviteAgentTargetInvalid
		}

		var sourceCode InviteCode
		if err := tx.Unscoped().First(&sourceCode, "id = ?", target.InviteCodeId).Error; err != nil {
			return ErrInviteAgentTargetInvalid
		}
		sourceCode.normalizeDerivedFields()
		if sourceCode.IsManual {
			return ErrInviteAgentTargetInvalid
		}

		var existingCodes []InviteCode
		if err := tx.Unscoped().Where("owner_user_id = ?", target.Id).Find(&existingCodes).Error; err != nil {
			return err
		}
		for i := range existingCodes {
			existingCodes[i].normalizeDerivedFields()
			if existingCodes[i].IsManual || existingCodes[i].AgentLevel <= InviteAgentLevelNone {
				continue
			}
			return ErrInviteAgentAlreadyEnabled
		}

		prefix := fmt.Sprintf("AG%d", target.Id)
		targetGroup := strings.TrimSpace(target.Group)
		if targetGroup == "" {
			targetGroup = "default"
		}
		codes, err := createInviteCodesTx(tx, prefix, 1, target.Id, targetGroup, 0, 0, InviteCodeStatusEnabled, InviteAgentLevelSecond, ownerUserID)
		if err != nil {
			return err
		}
		var inviteCode InviteCode
		if err := tx.Where("code = ?", codes[0]).First(&inviteCode).Error; err != nil {
			return err
		}
		inviteCode.normalizeDerivedFields()
		createdCode = &inviteCode
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdCode, nil
}

func createInviteCodesTx(tx *gorm.DB, prefix string, count int, ownerUserID int, targetGroup string, rewardQuotaPerUse int, rewardTotalUses int, status int, agentLevel int, grantedByUserID int) ([]string, error) {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "-")
	targetGroup = strings.TrimSpace(targetGroup)
	if prefix == "" {
		return nil, errors.New("邀请码前缀不能为空")
	}
	if count <= 0 {
		return nil, errors.New("生成数量必须大于 0")
	}
	if agentLevel < InviteAgentLevelNone {
		agentLevel = InviteAgentLevelNone
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
				AgentLevel:        agentLevel,
				GrantedByUserId:   grantedByUserID,
				Status:            status,
			}
			if err := tx.Create(code).Error; err != nil {
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

type inviteFlowRechargeRow struct {
	UserID int     `gorm:"column:user_id"`
	Amount int64   `gorm:"column:amount"`
	Money  float64 `gorm:"column:money"`
	Time   int64   `gorm:"column:event_time"`
}

type inviteFlowConsumeRow struct {
	UserID int   `gorm:"column:user_id"`
	Quota  int   `gorm:"column:quota"`
	Time   int64 `gorm:"column:event_time"`
}

type inviteFlowUserRow struct {
	Id        int    `gorm:"column:id"`
	Username  string `gorm:"column:username"`
	Group     string `gorm:"column:user_group"`
	UsedQuota int    `gorm:"column:used_quota"`
}

func GetInviteAgentStats(ownerUserID int, period string, startTime int64, endTime int64) (*InviteAgentStatsResponse, error) {
	if ownerUserID <= 0 {
		return nil, ErrInviteBindingOwnerNotFound
	}
	period, startTime, endTime = normalizeInviteStatsRange(period, startTime, endTime)
	agentLevel, err := GetUserInviteAgentLevel(ownerUserID)
	if err != nil {
		return nil, err
	}
	resp := &InviteAgentStatsResponse{
		AgentLevel:         agentLevel,
		CanGrantInvitation: agentLevel == InviteAgentLevelFirst,
		Period:             period,
		StartTime:          startTime,
		EndTime:            endTime,
		DirectTrend:        emptyInviteTrend(period, startTime, endTime),
		SecondLevelTrend:   emptyInviteTrend(period, startTime, endTime),
	}

	directUsers, err := getDirectInviteAgentUsers(ownerUserID)
	if err != nil {
		return nil, err
	}
	directIDs := userRowsToIDs(directUsers)
	resp.DirectStats = summarizeInviteUserRows(directUsers)
	if err := fillRechargeStatsAndTrend(directIDs, startTime, endTime, period, &resp.DirectStats, resp.DirectTrend); err != nil {
		return nil, err
	}
	if err := fillConsumeTrend(directIDs, startTime, endTime, period, resp.DirectTrend); err != nil {
		return nil, err
	}

	if agentLevel != InviteAgentLevelFirst || len(directUsers) == 0 {
		return resp, nil
	}

	secondCodes, err := getSecondLevelCodesGrantedBy(ownerUserID)
	if err != nil {
		return nil, err
	}
	secondCodeByOwner := make(map[int]InviteCode, len(secondCodes))
	secondOwnerIDs := make([]int, 0, len(secondCodes))
	for _, code := range secondCodes {
		secondCodeByOwner[code.OwnerUserId] = code
		secondOwnerIDs = append(secondOwnerIDs, code.OwnerUserId)
	}
	if len(secondOwnerIDs) == 0 {
		return resp, nil
	}
	secondUsers, err := getUsersByIDs(secondOwnerIDs)
	if err != nil {
		return nil, err
	}
	secondInviteesByOwner, err := getDirectInviteAgentUsersByOwners(secondOwnerIDs)
	if err != nil {
		return nil, err
	}

	sort.Slice(secondUsers, func(i, j int) bool { return secondUsers[i].Id > secondUsers[j].Id })
	resp.SecondLevelStats = make([]InviteAgentSecondLevelStats, 0, len(secondUsers))
	allSecondInviteeIDs := make([]int, 0)
	for _, secondUser := range secondUsers {
		code := secondCodeByOwner[secondUser.Id]
		selfRows := []inviteFlowUserRow{secondUser}
		inviteeRows := secondInviteesByOwner[secondUser.Id]
		inviteeIDs := userRowsToIDs(inviteeRows)
		allSecondInviteeIDs = append(allSecondInviteeIDs, inviteeIDs...)
		item := InviteAgentSecondLevelStats{
			UserID:       secondUser.Id,
			Username:     secondUser.Username,
			Group:        secondUser.Group,
			InviteCodeID: code.Id,
			InviteCode:   code.Code,
			SelfStats:    summarizeInviteUserRows(selfRows),
			InviteeStats: summarizeInviteUserRows(inviteeRows),
		}
		if err := fillRechargeStatsAndTrend([]int{secondUser.Id}, startTime, endTime, period, &item.SelfStats, nil); err != nil {
			return nil, err
		}
		if err := fillRechargeStatsAndTrend(inviteeIDs, startTime, endTime, period, &item.InviteeStats, nil); err != nil {
			return nil, err
		}
		resp.SecondLevelStats = append(resp.SecondLevelStats, item)
	}
	if len(allSecondInviteeIDs) > 0 {
		resp.SecondLevelStats = resp.SecondLevelStats
		dummyStats := InviteAgentUserFlowStats{}
		if err := fillRechargeStatsAndTrend(allSecondInviteeIDs, startTime, endTime, period, &dummyStats, resp.SecondLevelTrend); err != nil {
			return nil, err
		}
		if err := fillConsumeTrend(allSecondInviteeIDs, startTime, endTime, period, resp.SecondLevelTrend); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func normalizeInviteStatsRange(period string, startTime int64, endTime int64) (string, int64, int64) {
	now := common.GetTimestamp()
	if endTime <= 0 {
		endTime = now
	}
	if period != "month" {
		period = "day"
	}
	if startTime <= 0 {
		if period == "month" {
			startTime = time.Unix(endTime, 0).AddDate(0, -11, 0).Unix()
		} else {
			startTime = time.Unix(endTime, 0).AddDate(0, 0, -29).Unix()
		}
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	if period == "month" {
		minStart := time.Unix(endTime, 0).AddDate(0, -35, 0).Unix()
		if startTime < minStart {
			startTime = minStart
		}
	} else {
		minStart := time.Unix(endTime, 0).AddDate(0, 0, -365).Unix()
		if startTime < minStart {
			startTime = minStart
		}
	}
	return period, startTime, endTime
}

func bucketInviteTime(ts int64, period string) (int64, string) {
	t := time.Unix(ts, 0).Local()
	if period == "month" {
		bucket := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		return bucket.Unix(), bucket.Format("2006-01")
	}
	bucket := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return bucket.Unix(), bucket.Format("01-02")
}

func emptyInviteTrend(period string, startTime int64, endTime int64) []InviteAgentTrendPoint {
	startBucket, _ := bucketInviteTime(startTime, period)
	endBucket, _ := bucketInviteTime(endTime, period)
	points := make([]InviteAgentTrendPoint, 0)
	for cursor := time.Unix(startBucket, 0); !cursor.After(time.Unix(endBucket, 0)); {
		bucketStart := cursor.Unix()
		_, label := bucketInviteTime(bucketStart, period)
		points = append(points, InviteAgentTrendPoint{BucketStart: bucketStart, Label: label})
		if period == "month" {
			cursor = cursor.AddDate(0, 1, 0)
		} else {
			cursor = cursor.AddDate(0, 0, 1)
		}
	}
	return points
}

func trendIndex(points []InviteAgentTrendPoint) map[int64]int {
	index := make(map[int64]int, len(points))
	for i := range points {
		index[points[i].BucketStart] = i
	}
	return index
}

func getDirectInviteAgentUsers(ownerUserID int) ([]inviteFlowUserRow, error) {
	return getDirectInviteAgentUsersByOwner(ownerUserID)
}

func getDirectInviteAgentUsersByOwner(ownerUserID int) ([]inviteFlowUserRow, error) {
	ensureCommonColumnsInitialized()
	var rows []inviteFlowUserRow
	selectClause := fmt.Sprintf("id, username, %s as user_group, used_quota", commonGroupCol)
	err := DB.Model(&User{}).
		Select(selectClause).
		Where("invite_code_owner_id = ? AND invite_code_id > 0", ownerUserID).
		Find(&rows).Error
	return rows, err
}

func getDirectInviteAgentUsersByOwners(ownerIDs []int) (map[int][]inviteFlowUserRow, error) {
	result := make(map[int][]inviteFlowUserRow, len(ownerIDs))
	if len(ownerIDs) == 0 {
		return result, nil
	}
	ensureCommonColumnsInitialized()
	var rows []struct {
		OwnerID   int    `gorm:"column:owner_id"`
		Id        int    `gorm:"column:id"`
		Username  string `gorm:"column:username"`
		Group     string `gorm:"column:user_group"`
		UsedQuota int    `gorm:"column:used_quota"`
	}
	selectClause := fmt.Sprintf("invite_code_owner_id as owner_id, id, username, %s as user_group, used_quota", commonGroupCol)
	if err := DB.Model(&User{}).Select(selectClause).Where("invite_code_owner_id IN ? AND invite_code_id > 0", ownerIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.OwnerID] = append(result[row.OwnerID], inviteFlowUserRow{
			Id:        row.Id,
			Username:  row.Username,
			Group:     row.Group,
			UsedQuota: row.UsedQuota,
		})
	}
	return result, nil
}

func getSecondLevelCodesGrantedBy(granterUserID int) ([]InviteCode, error) {
	var codes []InviteCode
	if err := DB.Where("granted_by_user_id = ? AND agent_level = ? AND status = ?", granterUserID, InviteAgentLevelSecond, InviteCodeStatusEnabled).Find(&codes).Error; err != nil {
		return nil, err
	}
	filtered := make([]InviteCode, 0, len(codes))
	for i := range codes {
		codes[i].normalizeDerivedFields()
		if codes[i].IsManual || !codes[i].GrantsInvitePermission {
			continue
		}
		filtered = append(filtered, codes[i])
	}
	return filtered, nil
}

func getUsersByIDs(userIDs []int) ([]inviteFlowUserRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	ensureCommonColumnsInitialized()
	var rows []inviteFlowUserRow
	selectClause := fmt.Sprintf("id, username, %s as user_group, used_quota", commonGroupCol)
	err := DB.Model(&User{}).Select(selectClause).Where("id IN ?", userIDs).Find(&rows).Error
	return rows, err
}

func userRowsToIDs(rows []inviteFlowUserRow) []int {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	return ids
}

func summarizeInviteUserRows(rows []inviteFlowUserRow) InviteAgentUserFlowStats {
	stats := InviteAgentUserFlowStats{UserCount: int64(len(rows))}
	for _, row := range rows {
		stats.ConsumeQuota += row.UsedQuota
	}
	return stats
}

func fillRechargeStatsAndTrend(userIDs []int, startTime int64, endTime int64, period string, stats *InviteAgentUserFlowStats, points []InviteAgentTrendPoint) error {
	if len(userIDs) == 0 {
		return nil
	}
	var rows []inviteFlowRechargeRow
	if err := DB.Table("top_ups").
		Select("user_id, amount, money, complete_time as event_time").
		Where("user_id IN ? AND status = ?", userIDs, common.TopUpStatusSuccess).
		Scan(&rows).Error; err != nil {
		return err
	}
	index := trendIndex(points)
	for _, row := range rows {
		if stats != nil {
			stats.RechargeAmount += row.Amount
			stats.RechargeMoney += row.Money
		}
		if len(points) == 0 || row.Time < startTime || row.Time > endTime {
			continue
		}
		bucket, _ := bucketInviteTime(row.Time, period)
		if idx, ok := index[bucket]; ok {
			points[idx].RechargeAmount += row.Amount
			points[idx].RechargeMoney += row.Money
		}
	}
	return nil
}

func fillConsumeTrend(userIDs []int, startTime int64, endTime int64, period string, points []InviteAgentTrendPoint) error {
	if len(userIDs) == 0 || len(points) == 0 {
		return nil
	}
	var rows []inviteFlowConsumeRow
	if err := LOG_DB.Model(&Log{}).
		Select("user_id, quota, created_at as event_time").
		Where("user_id IN ? AND type = ? AND created_at >= ? AND created_at <= ?", userIDs, LogTypeConsume, startTime, endTime).
		Find(&rows).Error; err != nil {
		return err
	}
	index := trendIndex(points)
	for _, row := range rows {
		bucket, _ := bucketInviteTime(row.Time, period)
		if idx, ok := index[bucket]; ok {
			points[idx].ConsumeQuota += row.Quota
		}
	}
	return nil
}
