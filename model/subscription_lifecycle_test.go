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

func seedSubscriptionLifecyclePlan(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, DB.Create(&SubscriptionPlan{
		Id:            id,
		Title:         "Lifecycle Plan",
		PriceAmount:   10,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   100000,
	}).Error)
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

func TestCompleteSubscriptionOrderLinksCreatedSubscription(t *testing.T) {
	truncateTables(t)
	const userId = 8106
	const planId = 9201
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	seedSubscriptionLifecyclePlan(t, planId)
	order := &SubscriptionOrder{
		UserId:        userId,
		PlanId:        planId,
		Money:         10,
		TradeNo:       "sub_lifecycle_order_link",
		PaymentMethod: "stripe",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	require.NoError(t, order.Insert())

	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, ""))

	var completed SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&completed).Error)
	require.Equal(t, common.TopUpStatusSuccess, completed.Status)
	require.NotZero(t, completed.UserSubscriptionId)

	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", completed.UserSubscriptionId).First(&sub).Error)
	require.Equal(t, userId, sub.UserId)
	require.Equal(t, planId, sub.PlanId)
}

func TestAdminInvalidateSubscriptionMarksLinkedOrderInvalidated(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8107
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	seedSubscriptionLifecycleSub(t, 9107, userId, "active", now+3600)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:             userId,
		PlanId:             9107,
		Money:              10,
		TradeNo:            "sub_lifecycle_order_invalidated",
		PaymentMethod:      "stripe",
		Status:             common.TopUpStatusSuccess,
		CreateTime:         now - 60,
		CompleteTime:       now,
		UserSubscriptionId: 9107,
	}).Error)

	_, err := AdminInvalidateUserSubscription(9107, 1001)
	require.NoError(t, err)

	var order SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", "sub_lifecycle_order_invalidated").First(&order).Error)
	require.NotZero(t, order.InvalidatedAt)
	require.Equal(t, 1001, order.InvalidatedByUserId)
	require.Equal(t, "admin_cancelled", order.InvalidationReason)
}

func TestAdminInvalidateSubscriptionMarksUniqueLegacyOrderInvalidated(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8108
	const subId = 9108
	const planId = 9208
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          subId,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: 100000,
		AmountUsed:  1000,
		StartTime:   now - 120,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:        userId,
		PlanId:        planId,
		Money:         10,
		TradeNo:       "sub_lifecycle_order_legacy_unique",
		PaymentMethod: "stripe",
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 180,
		CompleteTime:  now - 90,
	}).Error)

	_, err := AdminInvalidateUserSubscription(subId, 1002)
	require.NoError(t, err)

	var order SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", "sub_lifecycle_order_legacy_unique").First(&order).Error)
	require.Equal(t, subId, order.UserSubscriptionId)
	require.NotZero(t, order.InvalidatedAt)
	require.Equal(t, 1002, order.InvalidatedByUserId)
	require.Equal(t, "admin_cancelled", order.InvalidationReason)
}

func TestAdminInvalidateSubscriptionDoesNotMarkAmbiguousLegacyOrders(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8109
	const subId = 9109
	const planId = 9209
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          subId,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: 100000,
		AmountUsed:  1000,
		StartTime:   now - 120,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)
	for _, tradeNo := range []string{"sub_lifecycle_order_legacy_a", "sub_lifecycle_order_legacy_b"} {
		require.NoError(t, DB.Create(&SubscriptionOrder{
			UserId:        userId,
			PlanId:        planId,
			Money:         10,
			TradeNo:       tradeNo,
			PaymentMethod: "stripe",
			Status:        common.TopUpStatusSuccess,
			CreateTime:    now - 180,
			CompleteTime:  now - 90,
		}).Error)
	}

	_, err := AdminInvalidateUserSubscription(subId, 1003)
	require.NoError(t, err)

	var invalidatedCount int64
	require.NoError(t, DB.Model(&SubscriptionOrder{}).
		Where("trade_no IN ? AND invalidated_at > 0", []string{"sub_lifecycle_order_legacy_a", "sub_lifecycle_order_legacy_b"}).
		Count(&invalidatedCount).Error)
	require.Zero(t, invalidatedCount)
}

func TestBackfillInvalidatedSubscriptionOrdersMarksUniqueLegacyOrder(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8110
	const subId = 9110
	const planId = 9210
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          subId,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: 100000,
		AmountUsed:  1000,
		StartTime:   now - 120,
		EndTime:     now - 10,
		Status:      "cancelled",
		Source:      "order",
		UpdatedAt:   now - 5,
	}).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subId).UpdateColumn("updated_at", now-5).Error)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:        userId,
		PlanId:        planId,
		Money:         10,
		TradeNo:       "sub_lifecycle_backfill_unique",
		PaymentMethod: "stripe",
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 180,
		CompleteTime:  now - 90,
	}).Error)

	require.NoError(t, backfillInvalidatedSubscriptionOrders())

	var order SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", "sub_lifecycle_backfill_unique").First(&order).Error)
	require.Equal(t, subId, order.UserSubscriptionId)
	require.Equal(t, now-5, order.InvalidatedAt)
	require.Equal(t, "admin_cancelled_backfill", order.InvalidationReason)
}

func TestBackfillInvalidatedSubscriptionOrdersSkipsAmbiguousLegacyOrders(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	const userId = 8111
	const subId = 9111
	const planId = 9211
	seedSubscriptionLifecycleUser(t, userId, "wallet_first")
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          subId,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: 100000,
		AmountUsed:  1000,
		StartTime:   now - 120,
		EndTime:     now - 10,
		Status:      "cancelled",
		Source:      "order",
		UpdatedAt:   now - 5,
	}).Error)
	for _, tradeNo := range []string{"sub_lifecycle_backfill_a", "sub_lifecycle_backfill_b"} {
		require.NoError(t, DB.Create(&SubscriptionOrder{
			UserId:        userId,
			PlanId:        planId,
			Money:         10,
			TradeNo:       tradeNo,
			PaymentMethod: "stripe",
			Status:        common.TopUpStatusSuccess,
			CreateTime:    now - 180,
			CompleteTime:  now - 90,
		}).Error)
	}

	require.NoError(t, backfillInvalidatedSubscriptionOrders())

	var invalidatedCount int64
	require.NoError(t, DB.Model(&SubscriptionOrder{}).
		Where("trade_no IN ? AND invalidated_at > 0", []string{"sub_lifecycle_backfill_a", "sub_lifecycle_backfill_b"}).
		Count(&invalidatedCount).Error)
	require.Zero(t, invalidatedCount)
}
