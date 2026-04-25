package model

import (
	"fmt"
	"testing"

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
	topUp := &TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		CreateTime:    common.GetTimestamp(),
		Status:        status,
	}
	require.NoError(t, topUp.Insert())
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
