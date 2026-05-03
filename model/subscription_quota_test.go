package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetActiveSubscriptionQuotaSummaryByUserIDs(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	subs := []UserSubscription{
		{
			Id:            1,
			UserId:        10,
			PlanId:        1,
			AmountTotal:   1000,
			AmountUsed:    250,
			StartTime:     now - 100,
			EndTime:       now + 3600,
			Status:        "active",
			NextResetTime: now + 600,
		},
		{
			Id:          2,
			UserId:      10,
			PlanId:      2,
			AmountTotal: 0,
			AmountUsed:  0,
			StartTime:   now - 100,
			EndTime:     now + 1800,
			Status:      "active",
		},
		{
			Id:            3,
			UserId:        11,
			PlanId:        3,
			AmountTotal:   500,
			AmountUsed:    700,
			StartTime:     now - 100,
			EndTime:       now + 7200,
			Status:        "active",
			NextResetTime: now + 1200,
		},
		{
			Id:          4,
			UserId:      10,
			PlanId:      4,
			AmountTotal: 900,
			AmountUsed:  100,
			StartTime:   now - 7200,
			EndTime:     now - 3600,
			Status:      "active",
		},
		{
			Id:          5,
			UserId:      10,
			PlanId:      5,
			AmountTotal: 900,
			AmountUsed:  100,
			StartTime:   now - 100,
			EndTime:     now + 3600,
			Status:      "cancelled",
		},
	}
	require.NoError(t, DB.Create(&subs).Error)

	summaries, err := GetActiveSubscriptionQuotaSummaryByUserIDs([]int{10, 11, 10, 0})
	require.NoError(t, err)

	user10 := summaries[10]
	require.Equal(t, 2, user10.ActiveCount)
	require.Equal(t, 1, user10.UnlimitedCount)
	require.Equal(t, int64(1000), user10.AmountTotal)
	require.Equal(t, int64(250), user10.AmountUsed)
	require.Equal(t, int64(750), user10.AmountRemain)
	require.Equal(t, now+600, user10.NextResetTime)
	require.Equal(t, now+1800, user10.EarliestEndTime)

	user11 := summaries[11]
	require.Equal(t, 1, user11.ActiveCount)
	require.Equal(t, 0, user11.UnlimitedCount)
	require.Equal(t, int64(500), user11.AmountTotal)
	require.Equal(t, int64(700), user11.AmountUsed)
	require.Equal(t, int64(0), user11.AmountRemain)
	require.Equal(t, now+1200, user11.NextResetTime)
	require.Equal(t, now+7200, user11.EarliestEndTime)
}
