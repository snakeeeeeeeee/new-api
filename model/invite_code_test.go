package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func seedInviteOwner(t *testing.T, id int, username string) {
	t.Helper()
	user := &User{
		Id:       id,
		Username: username,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  fmt.Sprintf("aff-%d", id),
	}
	require.NoError(t, DB.Create(user).Error)
}

func seedInviteCode(t *testing.T, code string, ownerID int, targetGroup string, rewardQuota int, totalUses int, usedUses int, status int) *InviteCode {
	t.Helper()
	inviteCode := &InviteCode{
		Code:              code,
		Prefix:            "TEST-",
		OwnerUserId:       ownerID,
		TargetGroup:       targetGroup,
		RewardQuotaPerUse: rewardQuota,
		RewardTotalUses:   totalUses,
		RewardUsedUses:    usedUses,
		Status:            status,
	}
	require.NoError(t, inviteCode.Insert())
	return inviteCode
}

func seedInviteTopUp(t *testing.T, userID int, tradeNo string, amount int64, money float64, status string) {
	t.Helper()
	seedInviteTopUpAt(t, userID, tradeNo, amount, money, status, common.GetTimestamp())
}

func seedInviteTopUpAt(t *testing.T, userID int, tradeNo string, amount int64, money float64, status string, completeTime int64) {
	t.Helper()
	topUp := &TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		CreateTime:    completeTime,
		CompleteTime:  completeTime,
		Status:        status,
	}
	require.NoError(t, topUp.Insert())
}

func seedInviteConsumeLogAt(t *testing.T, userID int, username string, quota int, createdAt int64) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    userID,
		CreatedAt: createdAt,
		Type:      LogTypeConsume,
		Username:  username,
		Quota:     quota,
	}).Error)
}

func seedInviteModelConsumeLogAt(t *testing.T, userID int, username string, modelName string, quota int, createdAt int64, other map[string]interface{}) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    userID,
		CreatedAt: createdAt,
		Type:      LogTypeConsume,
		Username:  username,
		ModelName: modelName,
		Quota:     quota,
		Other:     common.MapToJsonStr(other),
	}).Error)
}

func seedInviteSubscriptionPlan(t *testing.T, id int, title string, price float64) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:            id,
		Title:         title,
		PriceAmount:   price,
		Currency:      "USD",
		DurationUnit:  "month",
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   10000,
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func seedInviteSubscriptionOrderAt(t *testing.T, userID int, planID int, tradeNo string, money float64, status string, completeTime int64) {
	t.Helper()
	order := &SubscriptionOrder{
		UserId:        userID,
		PlanId:        planID,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "stripe",
		CreateTime:    completeTime - 60,
		CompleteTime:  completeTime,
		Status:        status,
	}
	require.NoError(t, order.Insert())
}

func fixedInviteStatsTime(year int, month time.Month, day int) int64 {
	return time.Date(year, month, day, 12, 0, 0, 0, time.Local).Unix()
}

func sumInviteTrendRecharge(points []InviteAgentTrendPoint) int64 {
	var total int64
	for _, point := range points {
		total += point.RechargeAmount
	}
	return total
}

func sumInviteTrendConsume(points []InviteAgentTrendPoint) int {
	var total int
	for _, point := range points {
		total += point.ConsumeQuota
	}
	return total
}

func TestInsertWithManagedInviteCodeGrantsRewardAndBindsUser(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 10, "owner")
	seedInviteCode(t, "TEST-A1B2C3", 10, "vip", 2000, 3, 0, InviteCodeStatusEnabled)

	newUser := &User{
		Username:    "invitee_a",
		Password:    "password123",
		DisplayName: "invitee_a",
		Role:        common.RoleCommonUser,
	}

	inviteCode, rewardGranted, err := newUser.InsertWithManagedInviteCode("TEST-A1B2C3")
	require.NoError(t, err)
	require.True(t, rewardGranted)
	require.NotNil(t, inviteCode)
	require.Equal(t, 10, newUser.InviterId)
	require.Equal(t, inviteCode.Id, newUser.InviteCodeId)
	require.Equal(t, 10, newUser.InviteCodeOwnerId)
	require.Equal(t, "vip", newUser.Group)
	require.Equal(t, 1, inviteCode.RewardUsedUses)

	createdUser, err := GetUserById(newUser.Id, false)
	require.NoError(t, err)
	require.Equal(t, common.QuotaForNewUser+2000, createdUser.Quota)
	require.Equal(t, inviteCode.Id, createdUser.InviteCodeId)
	require.Equal(t, 10, createdUser.InviteCodeOwnerId)
}

func TestInsertWithManagedInviteCodeAllowsRegistrationAfterRewardExhausted(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 11, "owner_b")
	seedInviteCode(t, "TEST-EXHAUST", 11, "svip", 5000, 1, 1, InviteCodeStatusEnabled)

	newUser := &User{
		Username:    "invitee_b",
		Password:    "password123",
		DisplayName: "invitee_b",
		Role:        common.RoleCommonUser,
	}

	inviteCode, rewardGranted, err := newUser.InsertWithManagedInviteCode("TEST-EXHAUST")
	require.NoError(t, err)
	require.False(t, rewardGranted)
	require.NotNil(t, inviteCode)
	require.Equal(t, "svip", newUser.Group)

	createdUser, err := GetUserById(newUser.Id, false)
	require.NoError(t, err)
	require.Equal(t, common.QuotaForNewUser, createdUser.Quota)
	require.Equal(t, inviteCode.Id, createdUser.InviteCodeId)
}

func TestGetInviteStatsAggregatesUsersRechargeAndConsume(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 12, "owner_c")
	inviteCode := seedInviteCode(t, "TEST-STATS", 12, "vip", 0, 0, 0, InviteCodeStatusEnabled)

	user1 := &User{
		Id:                201,
		Username:          "invitee_c1",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 12,
		InviterId:         12,
		UsedQuota:         3000,
		AffCode:           "aff-c1",
	}
	user2 := &User{
		Id:                202,
		Username:          "invitee_c2",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 12,
		InviterId:         12,
		UsedQuota:         5000,
		AffCode:           "aff-c2",
	}
	require.NoError(t, DB.Create(user1).Error)
	require.NoError(t, DB.Create(user2).Error)

	seedInviteTopUp(t, 201, "topup_stats_1", 100, 12.5, common.TopUpStatusSuccess)
	seedInviteTopUp(t, 202, "topup_stats_2", 50, 7.5, common.TopUpStatusSuccess)
	seedInviteTopUp(t, 202, "topup_stats_3", 999, 99, common.TopUpStatusPending)

	ownerStats, err := GetInviteStatsByOwnerUserIDs([]int{12})
	require.NoError(t, err)
	require.Equal(t, int64(2), ownerStats[12].InviteUserCount)
	require.Equal(t, 20.0, ownerStats[12].InviteTotalRecharge)
	require.Equal(t, int64(150), ownerStats[12].InviteTotalRechargeAmount)
	require.Equal(t, 20.0, ownerStats[12].InviteTotalRechargeMoney)
	require.Equal(t, 8000, ownerStats[12].InviteTotalConsume)

	codeStats, err := GetInviteStatsByInviteCodeIDs([]int{inviteCode.Id})
	require.NoError(t, err)
	require.Equal(t, int64(2), codeStats[inviteCode.Id].InviteUserCount)
	require.Equal(t, 20.0, codeStats[inviteCode.Id].InviteTotalRecharge)
	require.Equal(t, int64(150), codeStats[inviteCode.Id].InviteTotalRechargeAmount)
	require.Equal(t, 20.0, codeStats[inviteCode.Id].InviteTotalRechargeMoney)
	require.Equal(t, 8000, codeStats[inviteCode.Id].InviteTotalConsume)
}

func TestGetInviteConsumptionStatsAggregatesWalletLogsByModel(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	seedInviteOwner(t, 120, "wallet_owner")
	inviteCode := seedInviteCode(t, "WALLET-STATS", 120, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	require.NoError(t, DB.Create(&User{
		Id:                1201,
		Username:          "wallet_invitee_a",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 120,
		InviterId:         120,
		AffCode:           "aff-wallet-a",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:                1202,
		Username:          "wallet_invitee_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 120,
		InviterId:         120,
		AffCode:           "aff-wallet-b",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:       1203,
		Username: "not_invited",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff-not-invited",
	}).Error)

	may10 := fixedInviteStatsTime(2026, time.May, 10)
	may11 := fixedInviteStatsTime(2026, time.May, 11)
	may12 := fixedInviteStatsTime(2026, time.May, 12)
	seedInviteModelConsumeLogAt(t, 1201, "wallet_invitee_a", "gpt-4o", 1000, may10, nil)
	seedInviteModelConsumeLogAt(t, 1201, "wallet_invitee_a", "gpt-4o", 2000, may10+60, map[string]interface{}{"billing_source": "wallet"})
	seedInviteModelConsumeLogAt(t, 1202, "wallet_invitee_b", "claude-sonnet", 3000, may11, nil)
	seedInviteModelConsumeLogAt(t, 1202, "wallet_invitee_b", "gpt-4o", 9000, may11+60, map[string]interface{}{"billing_source": "subscription"})
	seedInviteModelConsumeLogAt(t, 1202, "wallet_invitee_b", "gpt-4o", 5000, may12, nil)
	seedInviteModelConsumeLogAt(t, 1203, "not_invited", "gpt-4o", 7000, may10, nil)

	stats, err := GetInviteConsumptionStats("wallet_owner", may10-10, may11+3600)
	require.NoError(t, err)
	require.Equal(t, 120, stats.Inviter.Id)
	require.Equal(t, "wallet_owner", stats.Inviter.Username)
	require.Equal(t, int64(2), stats.Summary.InviteUserCount)
	require.Equal(t, int64(6000), stats.Summary.WalletQuota)
	require.Equal(t, 6.0, stats.Summary.WalletAmount)
	require.Equal(t, int64(3), stats.Summary.RequestCount)
	require.Equal(t, 2, stats.Summary.ModelCount)
	require.Equal(t, int64(9000), stats.Summary.ExcludedSubscriptionQuota)
	require.Equal(t, int64(1), stats.Summary.ExcludedSubscriptionRequestCount)
	require.Len(t, stats.Models, 2)
	require.Equal(t, "gpt-4o", stats.Models[0].ModelName)
	require.Equal(t, int64(3000), stats.Models[0].Quota)
	require.Equal(t, int64(2), stats.Models[0].RequestCount)
	require.Equal(t, 50.0, stats.Models[0].Percent)
	require.Equal(t, "claude-sonnet", stats.Models[1].ModelName)
	require.Equal(t, int64(3000), stats.Models[1].Quota)
	require.Len(t, stats.Trend, 2)
	require.Equal(t, int64(3000), stats.Trend[0].Quota)
	require.Equal(t, int64(3000), stats.Trend[1].Quota)
}

func TestGetInviteConsumptionStatsIncludesSubscriptionUsageAndPurchases(t *testing.T) {
	truncateTables(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	seedInviteOwner(t, 121, "sub_owner")
	inviteCode := seedInviteCode(t, "SUB-STATS", 121, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	require.NoError(t, DB.Create(&User{
		Id:                1211,
		Username:          "sub_invitee_a",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 121,
		InviterId:         121,
		AffCode:           "aff-sub-a",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:                1212,
		Username:          "sub_invitee_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      inviteCode.Id,
		InviteCodeOwnerId: 121,
		InviterId:         121,
		AffCode:           "aff-sub-b",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:       1213,
		Username: "sub_not_invited",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff-sub-not-invited",
	}).Error)

	may10 := fixedInviteStatsTime(2026, time.May, 10)
	may11 := fixedInviteStatsTime(2026, time.May, 11)
	may12 := fixedInviteStatsTime(2026, time.May, 12)
	seedInviteModelConsumeLogAt(t, 1211, "sub_invitee_a", "claude-opus", 4000, may10, map[string]interface{}{"billing_source": "subscription"})
	seedInviteModelConsumeLogAt(t, 1212, "sub_invitee_b", "claude-opus", 2000, may11, map[string]interface{}{"billing_source": "subscription"})
	seedInviteModelConsumeLogAt(t, 1212, "sub_invitee_b", "gpt-4o", 1000, may11+60, map[string]interface{}{"billing_source": "wallet"})
	seedInviteModelConsumeLogAt(t, 1213, "sub_not_invited", "claude-opus", 9000, may10, map[string]interface{}{"billing_source": "subscription"})

	seedInviteSubscriptionPlan(t, 701, "Pro Monthly", 29.9)
	seedInviteSubscriptionPlan(t, 702, "Team Monthly", 99)
	seedInviteSubscriptionOrderAt(t, 1211, 701, "sub_order_success_a", 29.9, common.TopUpStatusSuccess, may10)
	seedInviteSubscriptionOrderAt(t, 1212, 701, "sub_order_success_b", 29.9, common.TopUpStatusSuccess, may11)
	seedInviteSubscriptionOrderAt(t, 1212, 702, "sub_order_pending", 99, common.TopUpStatusPending, may11)
	seedInviteSubscriptionOrderAt(t, 1212, 702, "sub_order_out_of_range", 99, common.TopUpStatusSuccess, may12)
	seedInviteSubscriptionOrderAt(t, 1213, 702, "sub_order_not_invited", 99, common.TopUpStatusSuccess, may10)

	stats, err := GetInviteConsumptionStats("sub_owner", may10-10, may11+3600)
	require.NoError(t, err)
	require.Equal(t, int64(1000), stats.Summary.WalletQuota)
	require.Equal(t, int64(6000), stats.Summary.ExcludedSubscriptionQuota)
	require.Equal(t, int64(6000), stats.SubscriptionUsage.Summary.Quota)
	require.Equal(t, 6.0, stats.SubscriptionUsage.Summary.Amount)
	require.Equal(t, int64(2), stats.SubscriptionUsage.Summary.RequestCount)
	require.Equal(t, 1, stats.SubscriptionUsage.Summary.ModelCount)
	require.Len(t, stats.SubscriptionUsage.Models, 1)
	require.Equal(t, "claude-opus", stats.SubscriptionUsage.Models[0].ModelName)
	require.Equal(t, int64(6000), stats.SubscriptionUsage.Models[0].Quota)
	require.Len(t, stats.SubscriptionUsage.Trend, 2)
	require.Equal(t, int64(4000), stats.SubscriptionUsage.Trend[0].Quota)
	require.Equal(t, int64(2000), stats.SubscriptionUsage.Trend[1].Quota)

	require.InDelta(t, 59.8, stats.SubscriptionPurchase.Summary.Amount, 0.0001)
	require.Equal(t, int64(2), stats.SubscriptionPurchase.Summary.OrderCount)
	require.Equal(t, int64(2), stats.SubscriptionPurchase.Summary.BuyerCount)
	require.Equal(t, 1, stats.SubscriptionPurchase.Summary.PlanCount)
	require.Len(t, stats.SubscriptionPurchase.Plans, 1)
	require.Equal(t, 701, stats.SubscriptionPurchase.Plans[0].PlanId)
	require.Equal(t, "Pro Monthly", stats.SubscriptionPurchase.Plans[0].PlanTitle)
	require.Equal(t, int64(2), stats.SubscriptionPurchase.Plans[0].OrderCount)
	require.InDelta(t, 59.8, stats.SubscriptionPurchase.Plans[0].Amount, 0.0001)
	require.Len(t, stats.SubscriptionPurchase.Trend, 2)
	require.InDelta(t, 29.9, stats.SubscriptionPurchase.Trend[0].Amount, 0.0001)
	require.Equal(t, int64(1), stats.SubscriptionPurchase.Trend[0].BuyerCount)
	require.InDelta(t, 29.9, stats.SubscriptionPurchase.Trend[1].Amount, 0.0001)
}

func TestGetInviteConsumptionBreakdownByOwnerIDsSplitsWalletAndSubscription(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 122, "breakdown_owner_a")
	seedInviteOwner(t, 123, "breakdown_owner_b")
	codeA := seedInviteCode(t, "BREAKDOWN-A", 122, "default", 0, 0, 0, InviteCodeStatusEnabled)
	codeB := seedInviteCode(t, "BREAKDOWN-B", 123, "default", 0, 0, 0, InviteCodeStatusEnabled)
	require.NoError(t, DB.Create(&User{
		Id:                1221,
		Username:          "breakdown_a1",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		UsedQuota:         10000,
		InviteCodeId:      codeA.Id,
		InviteCodeOwnerId: 122,
		InviterId:         122,
		AffCode:           "aff-breakdown-a1",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:                1222,
		Username:          "breakdown_a2",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		UsedQuota:         5000,
		InviteCodeId:      codeA.Id,
		InviteCodeOwnerId: 122,
		InviterId:         122,
		AffCode:           "aff-breakdown-a2",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:                1231,
		Username:          "breakdown_b1",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		UsedQuota:         3000,
		InviteCodeId:      codeB.Id,
		InviteCodeOwnerId: 123,
		InviterId:         123,
		AffCode:           "aff-breakdown-b1",
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:        1241,
		Username:  "breakdown_not_invited",
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		UsedQuota: 9000,
		AffCode:   "aff-breakdown-not",
	}).Error)

	now := fixedInviteStatsTime(2026, time.May, 10)
	seedInviteModelConsumeLogAt(t, 1221, "breakdown_a1", "gpt-4o", 2000, now, map[string]interface{}{"billing_source": "wallet"})
	seedInviteModelConsumeLogAt(t, 1221, "breakdown_a1", "gpt-4o", 3000, now+1, map[string]interface{}{"billing_source": "subscription"})
	seedInviteModelConsumeLogAt(t, 1222, "breakdown_a2", "gpt-4o", 4000, now+2, nil)
	seedInviteModelConsumeLogAt(t, 1231, "breakdown_b1", "gpt-4o", 5000, now+3, map[string]interface{}{"billing_source": "subscription"})
	seedInviteModelConsumeLogAt(t, 1241, "breakdown_not_invited", "gpt-4o", 9000, now+4, nil)

	stats, err := GetInviteConsumptionBreakdownByOwnerIDs([]int{122, 123, 122, -1})
	require.NoError(t, err)
	require.Len(t, stats, 2)
	require.Equal(t, int64(2), stats[122].InviteUserCount)
	require.Equal(t, int64(15000), stats[122].TotalUsedQuota)
	require.Equal(t, int64(6000), stats[122].WalletQuota)
	require.Equal(t, int64(3000), stats[122].SubscriptionQuota)
	require.Equal(t, int64(9000), stats[122].LogTotalQuota)
	require.Equal(t, int64(3), stats[122].LogRequestCount)
	require.Equal(t, int64(2), stats[122].WalletRequestCount)
	require.Equal(t, int64(1), stats[122].SubscriptionRequestCount)
	require.Equal(t, int64(1), stats[123].InviteUserCount)
	require.Equal(t, int64(3000), stats[123].TotalUsedQuota)
	require.Equal(t, int64(0), stats[123].WalletQuota)
	require.Equal(t, int64(5000), stats[123].SubscriptionQuota)
}

func TestGetInviteConsumptionStatsReturnsErrorForMissingOwner(t *testing.T) {
	truncateTables(t)
	_, err := GetInviteConsumptionStats("missing_owner", fixedInviteStatsTime(2026, time.May, 10), fixedInviteStatsTime(2026, time.May, 11))
	require.Error(t, err)
	require.Contains(t, err.Error(), "邀请人不存在")
}

func TestGetInviteCodesByOwnerUserIDIncludesDeletedCodes(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 13, "owner_deleted")

	enabledCode := seedInviteCode(t, "TEST-KEEP01", 13, "vip", 1000, 3, 1, InviteCodeStatusEnabled)
	deletedCode := seedInviteCode(t, "TEST-DEL001", 13, "vip", 1000, 3, 0, InviteCodeStatusDisabled)
	require.NoError(t, deletedCode.Delete())

	inviteCodes, total, err := GetInviteCodesByOwnerUserID(13, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, inviteCodes, 2)

	codeMap := make(map[string]*InviteCode, len(inviteCodes))
	for _, inviteCode := range inviteCodes {
		codeMap[inviteCode.Code] = inviteCode
	}

	require.False(t, codeMap[enabledCode.Code].IsDeleted)
	require.True(t, codeMap[deletedCode.Code].IsDeleted)
	require.Equal(t, InviteCodeStatusDisabled, codeMap[deletedCode.Code].Status)
}

func TestGetInviteeSummariesByOwnerUserIDIncludesInviteCodeState(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 14, "owner_invitee_state")

	disabledCode := seedInviteCode(t, "TEST-DIS001", 14, "vip", 0, 0, 0, InviteCodeStatusDisabled)
	deletedCode := seedInviteCode(t, "TEST-DEL002", 14, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	require.NoError(t, deletedCode.Delete())

	userDisabled := &User{
		Id:                301,
		Username:          "invitee_disabled",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      disabledCode.Id,
		InviteCodeOwnerId: 14,
		InviterId:         14,
		UsedQuota:         1200,
		AffCode:           "aff-disabled",
	}
	userDeleted := &User{
		Id:                302,
		Username:          "invitee_deleted",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      deletedCode.Id,
		InviteCodeOwnerId: 14,
		InviterId:         14,
		UsedQuota:         2200,
		AffCode:           "aff-deleted",
	}
	require.NoError(t, DB.Create(userDisabled).Error)
	require.NoError(t, DB.Create(userDeleted).Error)
	seedInviteTopUp(t, 301, "topup_invitee_state_1", 42, 4.2, common.TopUpStatusSuccess)

	invitees, total, err := GetInviteeSummariesByOwnerUserID(14, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, invitees, 2)

	inviteeMap := make(map[string]*InviteeSummary, len(invitees))
	for _, invitee := range invitees {
		inviteeMap[invitee.Username] = invitee
	}

	require.Equal(t, InviteCodeStatusDisabled, inviteeMap["invitee_disabled"].InviteCodeStatus)
	require.False(t, inviteeMap["invitee_disabled"].InviteCodeDeleted)
	require.Equal(t, 4.2, inviteeMap["invitee_disabled"].InviteTotalRecharge)
	require.Equal(t, int64(42), inviteeMap["invitee_disabled"].InviteTotalRechargeAmount)
	require.Equal(t, 4.2, inviteeMap["invitee_disabled"].InviteTotalRechargeMoney)
	require.True(t, inviteeMap["invitee_deleted"].InviteCodeDeleted)
	require.Equal(t, InviteCodeStatusEnabled, inviteeMap["invitee_deleted"].InviteCodeStatus)
}

func TestBindUserInviteBindingManualCodeUpdatesStatsAndSummariesWithoutReward(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 80, "manual_owner")
	invitee := &User{
		Id:        801,
		Username:  "manual_invitee",
		Password:  "password123",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		Quota:     12345,
		UsedQuota: 678,
		AffCode:   "aff-manual-invitee",
	}
	require.NoError(t, DB.Create(invitee).Error)

	change, err := BindUserInviteBinding(801, 80, 0)
	require.NoError(t, err)
	require.Equal(t, 80, change.NewInviterID)
	require.Equal(t, 80, change.NewInviteCodeOwnerID)
	require.NotZero(t, change.NewInviteCodeID)
	require.NotNil(t, change.InviteCode)
	require.Equal(t, manualInviteCodeValue(80), change.InviteCode.Code)

	var updated User
	require.NoError(t, DB.First(&updated, "id = ?", 801).Error)
	require.Equal(t, 80, updated.InviterId)
	require.Equal(t, 80, updated.InviteCodeOwnerId)
	require.Equal(t, change.NewInviteCodeID, updated.InviteCodeId)
	require.Equal(t, 12345, updated.Quota)
	require.NoError(t, PopulateUsersInviteStats([]*User{&updated}))
	require.Equal(t, "manual_owner", updated.InviterUsername)

	manualCode, err := GetInviteCodeByID(change.NewInviteCodeID)
	require.NoError(t, err)
	require.Equal(t, ManualInviteCodePrefix, manualCode.Prefix)
	require.True(t, manualCode.IsManual)
	require.Equal(t, 0, manualCode.RewardQuotaPerUse)
	require.Equal(t, 0, manualCode.RewardTotalUses)
	require.Equal(t, 0, manualCode.RewardUsedUses)
	require.Equal(t, InviteCodeStatusEnabled, manualCode.Status)

	registrationUser := &User{
		Username: "manual_code_register",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	_, _, err = registrationUser.InsertWithManagedInviteCode(manualCode.Code)
	require.ErrorIs(t, err, ErrInviteCodeManualOnly)

	stats, err := GetInviteStatsByOwnerUserIDs([]int{80})
	require.NoError(t, err)
	require.Equal(t, int64(1), stats[80].InviteUserCount)
	require.Equal(t, 678, stats[80].InviteTotalConsume)

	invitees, total, err := GetInviteeSummariesByOwnerUserID(80, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, invitees, 1)
	require.Equal(t, "manual_invitee", invitees[0].Username)
	require.Equal(t, manualCode.Id, invitees[0].InviteCodeID)
}

func TestRebindUserInviteBindingMovesStatsBetweenOwners(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 81, "old_owner")
	seedInviteOwner(t, 82, "new_owner")
	oldCode := seedInviteCode(t, "REBIND-OLD", 81, "default", 0, 0, 0, InviteCodeStatusEnabled)
	newCode := seedInviteCode(t, "REBIND-NEW", 82, "default", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                811,
		Username:          "rebind_invitee",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		UsedQuota:         900,
		AffCode:           "aff-rebind-invitee",
		InviterId:         81,
		InviteCodeOwnerId: 81,
		InviteCodeId:      oldCode.Id,
	}
	require.NoError(t, DB.Create(invitee).Error)

	beforeStats, err := GetInviteStatsByOwnerUserIDs([]int{81, 82})
	require.NoError(t, err)
	require.Equal(t, int64(1), beforeStats[81].InviteUserCount)
	require.NotContains(t, beforeStats, 82)

	change, err := BindUserInviteBinding(811, 82, newCode.Id)
	require.NoError(t, err)
	require.Equal(t, 81, change.OldInviterID)
	require.Equal(t, oldCode.Id, change.OldInviteCodeID)
	require.Equal(t, 82, change.NewInviterID)
	require.Equal(t, newCode.Id, change.NewInviteCodeID)

	afterStats, err := GetInviteStatsByOwnerUserIDs([]int{81, 82})
	require.NoError(t, err)
	require.NotContains(t, afterStats, 81)
	require.Equal(t, int64(1), afterStats[82].InviteUserCount)
	require.Equal(t, 900, afterStats[82].InviteTotalConsume)
}

func TestUnbindUserInviteBindingRemovesStats(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 83, "unbind_owner")
	inviteCode := seedInviteCode(t, "UNBIND-CODE", 83, "default", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                831,
		Username:          "unbind_invitee",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		UsedQuota:         700,
		AffCode:           "aff-unbind-invitee",
		InviterId:         83,
		InviteCodeOwnerId: 83,
		InviteCodeId:      inviteCode.Id,
	}
	require.NoError(t, DB.Create(invitee).Error)

	change, err := UnbindUserInviteBinding(831)
	require.NoError(t, err)
	require.Equal(t, 83, change.OldInviterID)
	require.Equal(t, inviteCode.Id, change.OldInviteCodeID)
	require.Zero(t, change.NewInviterID)
	require.Zero(t, change.NewInviteCodeID)

	var updated User
	require.NoError(t, DB.First(&updated, "id = ?", 831).Error)
	require.Zero(t, updated.InviterId)
	require.Zero(t, updated.InviteCodeOwnerId)
	require.Zero(t, updated.InviteCodeId)

	stats, err := GetInviteStatsByOwnerUserIDs([]int{83})
	require.NoError(t, err)
	require.NotContains(t, stats, 83)

	invitees, total, err := GetInviteeSummariesByOwnerUserID(83, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.Empty(t, invitees)
}

func TestBindUserInviteBindingRejectsSelf(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 84, "self_owner")

	_, err := BindUserInviteBinding(84, 84, 0)
	require.ErrorIs(t, err, ErrInviteBindingSelf)
}

func TestBindUserInviteBindingRejectsForeignInviteCode(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 85, "expected_owner")
	seedInviteOwner(t, 86, "foreign_owner")
	foreignCode := seedInviteCode(t, "FOREIGN-CODE", 86, "default", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:       851,
		Username: "foreign_code_invitee",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff-foreign-code-invitee",
	}
	require.NoError(t, DB.Create(invitee).Error)

	_, err := BindUserInviteBinding(851, 85, foreignCode.Id)
	require.ErrorIs(t, err, ErrInviteCodeOwnerMismatch)
}

func TestEnableInviteeInvitationCreatesSecondLevelCode(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 90, "agent_a")
	firstCode := seedInviteCode(t, "AGENTA-CODE", 90, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                901,
		Username:          "agent_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 90,
		InviterId:         90,
	}
	require.NoError(t, DB.Create(invitee).Error)

	code, err := EnableInviteeInvitation(90, 901)
	require.NoError(t, err)
	require.NotNil(t, code)
	require.Equal(t, 901, code.OwnerUserId)
	require.Equal(t, "vip", code.TargetGroup)
	require.Equal(t, InviteAgentLevelSecond, code.AgentLevel)
	require.Equal(t, 90, code.GrantedByUserId)
	require.Equal(t, 0, code.RewardQuotaPerUse)
	require.Equal(t, 0, code.RewardTotalUses)

	level, err := GetUserInviteAgentLevel(901)
	require.NoError(t, err)
	require.Equal(t, InviteAgentLevelSecond, level)
}

func TestEnableInviteeInvitationRejectsSecondLevelGrant(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 91, "agent_a2")
	firstCode := seedInviteCode(t, "AGENTA2-CODE", 91, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	agentB := &User{
		Id:                911,
		Username:          "agent_b2",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b2",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 91,
		InviterId:         91,
	}
	require.NoError(t, DB.Create(agentB).Error)
	secondCode, err := EnableInviteeInvitation(91, 911)
	require.NoError(t, err)

	inviteeC := &User{
		Id:                912,
		Username:          "agent_c2",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-c2",
		InviteCodeId:      secondCode.Id,
		InviteCodeOwnerId: 911,
		InviterId:         911,
	}
	require.NoError(t, DB.Create(inviteeC).Error)

	_, err = EnableInviteeInvitation(911, 912)
	require.ErrorIs(t, err, ErrInviteAgentNoPermission)
}

func TestGetInviteAgentStatsSeparatesSecondLevelFlow(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 92, "agent_a3")
	firstCode := seedInviteCode(t, "AGENTA3-CODE", 92, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	agentB := &User{
		Id:                921,
		Username:          "agent_b3",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b3",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 92,
		InviterId:         92,
		UsedQuota:         3000,
	}
	require.NoError(t, DB.Create(agentB).Error)
	secondCode, err := EnableInviteeInvitation(92, 921)
	require.NoError(t, err)
	inviteeC := &User{
		Id:                922,
		Username:          "agent_c3",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-c3",
		InviteCodeId:      secondCode.Id,
		InviteCodeOwnerId: 921,
		InviterId:         921,
		UsedQuota:         7000,
	}
	require.NoError(t, DB.Create(inviteeC).Error)
	seedInviteTopUp(t, 921, "agent_b_topup", 100, 10, common.TopUpStatusSuccess)
	seedInviteTopUp(t, 922, "agent_c_topup", 200, 20, common.TopUpStatusSuccess)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    922,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeConsume,
		Username:  "agent_c3",
		Quota:     7000,
	}).Error)

	stats, err := GetInviteAgentStats(92, "day", common.GetTimestamp()-86400, common.GetTimestamp()+10)
	require.NoError(t, err)
	require.Equal(t, InviteAgentLevelFirst, stats.AgentLevel)
	require.Equal(t, int64(1), stats.DirectStats.UserCount)
	require.Equal(t, int64(100), stats.DirectStats.RechargeAmount)
	require.Equal(t, 3000, stats.DirectStats.ConsumeQuota)
	require.Len(t, stats.SecondLevelStats, 1)
	require.Equal(t, 921, stats.SecondLevelStats[0].UserID)
	require.Equal(t, int64(100), stats.SecondLevelStats[0].SelfStats.RechargeAmount)
	require.Equal(t, 3000, stats.SecondLevelStats[0].SelfStats.ConsumeQuota)
	require.Equal(t, int64(200), stats.SecondLevelStats[0].InviteeStats.RechargeAmount)
	require.Equal(t, 7000, stats.SecondLevelStats[0].InviteeStats.ConsumeQuota)
	require.NotEmpty(t, stats.SecondLevelTrend)
	var trendConsume int
	for _, point := range stats.SecondLevelTrend {
		trendConsume += point.ConsumeQuota
	}
	require.Equal(t, 7000, trendConsume)
}

func TestGetInviteAgentStatsFiltersRechargeStatusAndTrendRange(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 93, "agent_a4")
	firstCode := seedInviteCode(t, "AGENTA4-CODE", 93, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                931,
		Username:          "agent_b4",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b4",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 93,
		InviterId:         93,
		UsedQuota:         900,
	}
	require.NoError(t, DB.Create(invitee).Error)

	inRange := fixedInviteStatsTime(2026, time.May, 10)
	outOfRange := fixedInviteStatsTime(2026, time.May, 1)
	seedInviteTopUpAt(t, 931, "agent_b4_success_in_range", 100, 10, common.TopUpStatusSuccess, inRange)
	seedInviteTopUpAt(t, 931, "agent_b4_success_out_range", 70, 7, common.TopUpStatusSuccess, outOfRange)
	seedInviteTopUpAt(t, 931, "agent_b4_pending_in_range", 999, 99, common.TopUpStatusPending, inRange)
	seedInviteConsumeLogAt(t, 931, "agent_b4", 300, inRange)
	seedInviteConsumeLogAt(t, 931, "agent_b4", 600, outOfRange)

	stats, err := GetInviteAgentStats(93, "day", fixedInviteStatsTime(2026, time.May, 9), fixedInviteStatsTime(2026, time.May, 11))
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.DirectStats.UserCount)
	require.Equal(t, int64(170), stats.DirectStats.RechargeAmount)
	require.Equal(t, 17.0, stats.DirectStats.RechargeMoney)
	require.Equal(t, 900, stats.DirectStats.ConsumeQuota)
	require.Equal(t, int64(100), sumInviteTrendRecharge(stats.DirectTrend))
	require.Equal(t, 300, sumInviteTrendConsume(stats.DirectTrend))
}

func TestGetInviteAgentStatsAggregatesMonthTrend(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 94, "agent_a5")
	firstCode := seedInviteCode(t, "AGENTA5-CODE", 94, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                941,
		Username:          "agent_b5",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b5",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 94,
		InviterId:         94,
		UsedQuota:         1200,
	}
	require.NoError(t, DB.Create(invitee).Error)

	mayTime := fixedInviteStatsTime(2026, time.May, 10)
	juneTime := fixedInviteStatsTime(2026, time.June, 5)
	seedInviteTopUpAt(t, 941, "agent_b5_may_1", 100, 10, common.TopUpStatusSuccess, mayTime)
	seedInviteTopUpAt(t, 941, "agent_b5_may_2", 50, 5, common.TopUpStatusSuccess, mayTime)
	seedInviteTopUpAt(t, 941, "agent_b5_june_1", 200, 20, common.TopUpStatusSuccess, juneTime)
	seedInviteConsumeLogAt(t, 941, "agent_b5", 300, mayTime)
	seedInviteConsumeLogAt(t, 941, "agent_b5", 400, juneTime)

	stats, err := GetInviteAgentStats(94, "month", fixedInviteStatsTime(2026, time.May, 1), fixedInviteStatsTime(2026, time.June, 30))
	require.NoError(t, err)
	require.Equal(t, int64(350), stats.DirectStats.RechargeAmount)
	require.Equal(t, 35.0, stats.DirectStats.RechargeMoney)
	require.Len(t, stats.DirectTrend, 2)
	require.Equal(t, "2026-05", stats.DirectTrend[0].Label)
	require.Equal(t, int64(150), stats.DirectTrend[0].RechargeAmount)
	require.Equal(t, 300, stats.DirectTrend[0].ConsumeQuota)
	require.Equal(t, "2026-06", stats.DirectTrend[1].Label)
	require.Equal(t, int64(200), stats.DirectTrend[1].RechargeAmount)
	require.Equal(t, 400, stats.DirectTrend[1].ConsumeQuota)
}

func TestGetInviteAgentStatsKeepsConsumeTotalWithoutConsumeLogs(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 95, "agent_a6")
	firstCode := seedInviteCode(t, "AGENTA6-CODE", 95, "vip", 0, 0, 0, InviteCodeStatusEnabled)
	invitee := &User{
		Id:                951,
		Username:          "agent_b6",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-b6",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 95,
		InviterId:         95,
		UsedQuota:         4321,
	}
	require.NoError(t, DB.Create(invitee).Error)
	seedInviteTopUpAt(t, 951, "agent_b6_topup", 123, 12.3, common.TopUpStatusSuccess, fixedInviteStatsTime(2026, time.May, 10))

	stats, err := GetInviteAgentStats(95, "day", fixedInviteStatsTime(2026, time.May, 9), fixedInviteStatsTime(2026, time.May, 11))
	require.NoError(t, err)
	require.Equal(t, 4321, stats.DirectStats.ConsumeQuota)
	require.Equal(t, 0, sumInviteTrendConsume(stats.DirectTrend))
	require.Equal(t, int64(123), sumInviteTrendRecharge(stats.DirectTrend))
}
