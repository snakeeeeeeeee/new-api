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

func seedInviteTopUp(t *testing.T, userID int, tradeNo string, money float64, status string) {
	t.Helper()
	topUp := &TopUp{
		UserId:        userID,
		Amount:        0,
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

	seedInviteTopUp(t, 201, "topup_stats_1", 12.5, common.TopUpStatusSuccess)
	seedInviteTopUp(t, 202, "topup_stats_2", 7.5, common.TopUpStatusSuccess)
	seedInviteTopUp(t, 202, "topup_stats_3", 99, common.TopUpStatusPending)

	ownerStats, err := GetInviteStatsByOwnerUserIDs([]int{12})
	require.NoError(t, err)
	require.Equal(t, int64(2), ownerStats[12].InviteUserCount)
	require.Equal(t, 20.0, ownerStats[12].InviteTotalRecharge)
	require.Equal(t, 8000, ownerStats[12].InviteTotalConsume)

	codeStats, err := GetInviteStatsByInviteCodeIDs([]int{inviteCode.Id})
	require.NoError(t, err)
	require.Equal(t, int64(2), codeStats[inviteCode.Id].InviteUserCount)
	require.Equal(t, 20.0, codeStats[inviteCode.Id].InviteTotalRecharge)
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
	require.True(t, inviteeMap["invitee_deleted"].InviteCodeDeleted)
	require.Equal(t, InviteCodeStatusEnabled, inviteeMap["invitee_deleted"].InviteCodeStatus)
}
