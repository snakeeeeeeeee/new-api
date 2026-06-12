package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func seedSubscriptionLifecycleUser(t *testing.T, userId int, pref string) {
	t.Helper()
	user := &User{
		Id:       userId,
		Username: "sub_lifecycle_user",
		Quota:    500000,
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}
	user.SetSetting(dto.UserSetting{BillingPreference: pref, Language: "zh"})
	require.NoError(t, DB.Create(user).Error)
}

func seedSubscriptionLifecycleToken(t *testing.T, id int, userId int) Token {
	t.Helper()
	token := Token{
		Id:             id,
		UserId:         userId,
		Key:            "subbug-token",
		Status:         common.TokenStatusEnabled,
		Name:           "subbug-token-name",
		ExpiredTime:    -1,
		RemainQuota:    500000,
		UnlimitedQuota: false,
		UsedQuota:      123,
		Group:          "default",
	}
	require.NoError(t, DB.Create(&token).Error)
	return token
}

func seedSubscriptionLifecycleSub(t *testing.T, id int, userId int, status string, endTime int64) {
	t.Helper()
	sub := &UserSubscription{
		Id:          id,
		UserId:      userId,
		PlanId:      id,
		AmountTotal: 100000,
		AmountUsed:  1000,
		StartTime:   endTime - 3600,
		EndTime:     endTime,
		Status:      status,
	}
	require.NoError(t, DB.Create(sub).Error)
}

func requireBillingPreference(t *testing.T, userId int, expected string) {
	t.Helper()
	user, err := GetUserById(userId, true)
	require.NoError(t, err)
	require.Equal(t, expected, user.GetSetting().BillingPreference)
}

func requireTokenUnchanged(t *testing.T, before Token) {
	t.Helper()
	var after Token
	require.NoError(t, DB.Where("id = ?", before.Id).First(&after).Error)
	require.Equal(t, before.Key, after.Key)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.Name, after.Name)
	require.Equal(t, before.ExpiredTime, after.ExpiredTime)
	require.Equal(t, before.RemainQuota, after.RemainQuota)
	require.Equal(t, before.UnlimitedQuota, after.UnlimitedQuota)
	require.Equal(t, before.UsedQuota, after.UsedQuota)
	require.Equal(t, before.Group, after.Group)
}

func TestAdminInvalidateLastActiveSubscriptionFallsBackSubscriptionOnlyPreference(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8101
	seedSubscriptionLifecycleUser(t, userId, "subscription_only")
	token := seedSubscriptionLifecycleToken(t, 8101, userId)
	seedSubscriptionLifecycleSub(t, 9101, userId, "active", now+3600)

	msg, err := AdminInvalidateUserSubscription(9101)
	require.NoError(t, err)
	require.Empty(t, msg)

	requireBillingPreference(t, userId, "subscription_first")
	requireTokenUnchanged(t, token)
	hasActive, err := HasActiveUserSubscription(userId)
	require.NoError(t, err)
	require.False(t, hasActive)
}

func TestAdminInvalidateOneOfMultipleActiveSubscriptionsKeepsSubscriptionOnlyPreference(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8102
	seedSubscriptionLifecycleUser(t, userId, "subscription_only")
	token := seedSubscriptionLifecycleToken(t, 8102, userId)
	seedSubscriptionLifecycleSub(t, 9102, userId, "active", now+3600)
	seedSubscriptionLifecycleSub(t, 9103, userId, "active", now+7200)

	_, err := AdminInvalidateUserSubscription(9102)
	require.NoError(t, err)

	requireBillingPreference(t, userId, "subscription_only")
	requireTokenUnchanged(t, token)
	hasActive, err := HasActiveUserSubscription(userId)
	require.NoError(t, err)
	require.True(t, hasActive)
}

func TestAdminDeleteLastActiveSubscriptionFallsBackSubscriptionOnlyPreference(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8103
	seedSubscriptionLifecycleUser(t, userId, "subscription_only")
	token := seedSubscriptionLifecycleToken(t, 8103, userId)
	seedSubscriptionLifecycleSub(t, 9104, userId, "active", now+3600)

	msg, err := AdminDeleteUserSubscription(9104)
	require.NoError(t, err)
	require.Empty(t, msg)

	requireBillingPreference(t, userId, "subscription_first")
	requireTokenUnchanged(t, token)
	hasActive, err := HasActiveUserSubscription(userId)
	require.NoError(t, err)
	require.False(t, hasActive)
}

func TestAdminInvalidateLastActiveSubscriptionKeepsNonSubscriptionOnlyPreference(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8104
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	token := seedSubscriptionLifecycleToken(t, 8104, userId)
	seedSubscriptionLifecycleSub(t, 9105, userId, "active", now+3600)

	_, err := AdminInvalidateUserSubscription(9105)
	require.NoError(t, err)

	requireBillingPreference(t, userId, "wallet_first")
	requireTokenUnchanged(t, token)
}

func TestAdminInvalidateExpiredSubscriptionDoesNotFallbackPreference(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8105
	seedSubscriptionLifecycleUser(t, userId, "subscription_only")
	token := seedSubscriptionLifecycleToken(t, 8105, userId)
	seedSubscriptionLifecycleSub(t, 9106, userId, "active", now-60)

	_, err := AdminInvalidateUserSubscription(9106)
	require.NoError(t, err)

	requireBillingPreference(t, userId, "subscription_only")
	requireTokenUnchanged(t, token)
}
