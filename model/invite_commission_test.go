package model

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func seedInviteCommissionInvitee(t *testing.T, id int, username string, ownerID int, inviteCodeID int) {
	t.Helper()
	user := &User{
		Id:                id,
		Username:          username,
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		AffCode:           fmt.Sprintf("aff-commission-%d", id),
		InviterId:         ownerID,
		InviteCodeOwnerId: ownerID,
		InviteCodeId:      inviteCodeID,
	}
	require.NoError(t, DB.Create(user).Error)
}

func seedInviteCommissionTopUp(t *testing.T, userID int, tradeNo string, amount int64, money float64, completeTime int64, status string) {
	t.Helper()
	require.NoError(t, DB.Create(&TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		CreateTime:    completeTime - 10,
		CompleteTime:  completeTime,
		Status:        status,
	}).Error)
}

func seedInviteCommissionRedemption(t *testing.T, usedUserID int, quota int, redeemedTime int64) {
	t.Helper()
	require.NoError(t, DB.Create(&Redemption{
		Key:          fmt.Sprintf("redeem-%d-%d", usedUserID, quota),
		Status:       common.RedemptionCodeStatusUsed,
		Name:         "test",
		Quota:        quota,
		CreatedTime:  redeemedTime - 10,
		RedeemedTime: redeemedTime,
		UsedUserId:   usedUserID,
	}).Error)
}

func seedInviteCommissionLog(t *testing.T, userID int, createdAt int64, modelName string, quota int, other map[string]interface{}) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           userID,
		Username:         fmt.Sprintf("user-%d", userID),
		CreatedAt:        createdAt,
		Type:             LogTypeConsume,
		ModelName:        modelName,
		Quota:            quota,
		PromptTokens:     quota / 10,
		CompletionTokens: quota / 20,
		Other:            common.MapToJsonStr(other),
	}).Error)
}

func seedInviteCommissionLogWithSnapshot(t *testing.T, userID int, createdAt int64, modelName string, quota int, service string, maxCommissionRateBps int, other map[string]interface{}) {
	t.Helper()
	if other == nil {
		other = map[string]interface{}{}
	}
	service = normalizeInviteCommissionService(service)
	if service == "" {
		service = "other"
	}
	group := "commission-" + service
	other["admin_info"] = map[string]interface{}{
		"commission": InviteCommissionLogCommissionSnapshot{
			Group:                   group,
			Service:                 service,
			Configured:              true,
			ProfitRateBps:           10000,
			CostRateBps:             0,
			MaxCommissionRateBps:    maxCommissionRateBps,
			ProfitShareRateBps:      10000,
			ProfitProtectionEnabled: true,
			RevenueQuota:            int64(quota),
			UpstreamCostQuota:       0,
			GrossProfitQuota:        int64(quota),
			Source:                  "snapshot",
		},
	}
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           userID,
		Username:         fmt.Sprintf("user-%d", userID),
		CreatedAt:        createdAt,
		Type:             LogTypeConsume,
		ModelName:        modelName,
		Quota:            quota,
		PromptTokens:     quota / 10,
		CompletionTokens: quota / 20,
		Group:            group,
		Other:            common.MapToJsonStr(other),
	}).Error)
}

func seedInviteCommissionSubscriptionOrder(t *testing.T, userID int, planID int, tradeNo string, money float64, completeTime int64) {
	t.Helper()
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:        userID,
		PlanId:        planID,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: "complex-test",
		Status:        common.TopUpStatusSuccess,
		CreateTime:    completeTime - 10,
		CompleteTime:  completeTime,
	}).Error)
	seedInviteCommissionTopUp(t, userID, tradeNo, 999999, money, completeTime, common.TopUpStatusSuccess)
}

func seedInviteCommissionChannel(t *testing.T, id int, group string) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:     id,
		Type:   1,
		Key:    fmt.Sprintf("channel-key-%d", id),
		Name:   fmt.Sprintf("channel-%d", id),
		Group:  group,
		Models: "gpt-4",
	}).Error)
}

func seedInviteCommissionAggregateGroup(t *testing.T, id int, name string) {
	t.Helper()
	require.NoError(t, DB.Create(&AggregateGroup{
		Id:          id,
		Name:        name,
		DisplayName: name,
		Status:      AggregateGroupStatusEnabled,
	}).Error)
}

func TestInviteCommissionReportAggregatesTwoLevelsAndSources(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1000, "agent_a")
	codeA := seedInviteCode(t, "COM-A", 1000, "default", 100000, 100, 0, InviteCodeStatusEnabled)
	codeB := seedInviteCode(t, "COM-B", 1001, "default", 0, 0, 0, InviteCodeStatusEnabled)
	codeC := seedInviteCode(t, "COM-C", 1002, "default", 0, 0, 0, InviteCodeStatusEnabled)
	_, err := UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1000,
		Enabled:       true,
		Level1RateBps: 500,
		Level2RateBps: 150,
	})
	require.NoError(t, err)
	seedInviteCommissionInvitee(t, 1001, "agent_b", 1000, codeA.Id)
	seedInviteCommissionInvitee(t, 1002, "user_c", 1001, codeB.Id)
	seedInviteCommissionInvitee(t, 1003, "user_d_third_level", 1002, codeC.Id)

	seedInviteCommissionTopUp(t, 1001, "wallet-b", 100, 12, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1002, "wallet-c", 50, 6, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1002, "pending-c", 500, 60, now, common.TopUpStatusPending)
	seedInviteCommissionTopUp(t, 1003, "wallet-d", 999, 99, now, common.TopUpStatusSuccess)

	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:        1002,
		PlanId:        1,
		Money:         30,
		TradeNo:       "sub-c",
		PaymentMethod: "stripe",
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 10,
		CompleteTime:  now,
	}).Error)
	seedInviteCommissionTopUp(t, 1002, "sub-c", 0, 30, now, common.TopUpStatusSuccess)

	seedInviteCommissionRedemption(t, 1001, 2000, now)
	seedInviteCommissionRedemption(t, 1002, 3000, now)
	seedInviteCommissionRedemption(t, 1003, 9000, now)

	require.NoError(t, DB.Create(&UserSubscription{
		Id:          501,
		UserId:      1002,
		PlanId:      1,
		AmountTotal: 100000,
		AmountUsed:  20000,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "order",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          502,
		UserId:      1002,
		PlanId:      1,
		AmountTotal: 100000,
		AmountUsed:  50000,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "admin",
	}).Error)

	seedInviteCommissionLogWithSnapshot(t, 1001, now, "gpt-4", 10000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1002, now, "claude-3-5-sonnet", 20000, "claude", 1000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1002, now, "gpt-image-1", 30000, "gpt", 2000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       501,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 30000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1002, now, "gemini-2.5-pro", 40000, "gemini", 1000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       502,
		"subscription_total":    100000,
		"subscription_used":     50000,
		"subscription_consumed": 40000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1003, now, "gemini-ignored", 9000, "gemini", 1000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1001, now-10*24*60*60, "gpt-old", 8000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1000,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, report.Summary.InviteeCount)
	require.Equal(t, 1, report.Summary.Level1InviteeCount)
	require.Equal(t, 1, report.Summary.Level2InviteeCount)
	require.Equal(t, int64(150), report.Summary.WalletRechargeAmount)
	require.Equal(t, 18.0, report.Summary.WalletRechargeMoney)
	require.Equal(t, int64(5000), report.Summary.RedemptionQuota)
	require.Equal(t, 30.0, report.Summary.SubscriptionPurchaseMoney)
	require.Equal(t, int64(30000), report.Summary.WalletConsumptionQuota)
	require.Equal(t, int64(70000), report.Summary.SubscriptionConsumptionQuota)
	require.Equal(t, int64(100000), report.Summary.TotalConsumptionQuota)
	require.Equal(t, int64(40000), report.Summary.AdminSubscriptionConsumption)
	require.Equal(t, int64(30000), report.Summary.OrderSubscriptionConsumption)
	require.InDelta(t, 197.5, report.Summary.EstimatedCommissionQuota, 0.0001)
	require.InDelta(t, 130.0, report.Summary.WalletCommissionQuota, 0.0001)
	require.InDelta(t, 67.5, report.Summary.SubscriptionCommissionQuota, 0.0001)
	require.InDelta(t, 67.5, report.Summary.OrderSubscriptionCommission, 0.0001)

	require.Len(t, report.Levels, 2)
	require.Equal(t, int64(10000), report.Levels[0].WalletConsumptionQuota)
	require.InDelta(t, 100.0, report.Levels[0].EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(90000), report.Levels[1].TotalConsumptionQuota)
	require.InDelta(t, 97.5, report.Levels[1].EstimatedCommissionQuota, 0.0001)

	serviceStats := map[string]InviteCommissionServiceStat{}
	for _, service := range report.Services {
		serviceStats[service.Service] = service
	}
	require.InDelta(t, 167.5, serviceStats["gpt"].EstimatedCommissionQuota, 0.0001)
	require.InDelta(t, 30.0, serviceStats["claude"].EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(40000), serviceStats["gemini"].AdminSubscriptionConsumption)

	require.Len(t, report.Invitees, 2)
	require.Len(t, report.Models, 4)
}

func TestInviteCommissionUserConfigOverridesAndCanDisableCommission(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1100, "agent_custom")
	code := seedInviteCode(t, "COM-CUSTOM", 1100, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1101, "custom_invitee", 1100, code.Id)
	seedInviteCommissionLogWithSnapshot(t, 1101, now, "gpt-4", 10000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})

	_, err := UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1100,
		Enabled:       true,
		Level1RateBps: 1000,
		Level2RateBps: 0,
	})
	require.NoError(t, err)
	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1100,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
	})
	require.NoError(t, err)
	require.True(t, report.Effective.UseUserConfig)
	require.InDelta(t, 200.0, report.Summary.EstimatedCommissionQuota, 0.0001)

	_, err = UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1100,
		Enabled:       false,
		Level1RateBps: 1000,
		Level2RateBps: 0,
	})
	require.NoError(t, err)
	report, err = BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1100,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
	})
	require.NoError(t, err)
	require.Equal(t, int64(10000), report.Summary.WalletConsumptionQuota)
	require.Zero(t, report.Summary.EstimatedCommissionQuota)
}

func TestInviteCommissionGroupProfitRulesListAndCrud(t *testing.T) {
	truncateTables(t)
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"UserGroup-vip":1,"profit-ratio":1.2}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	seedInviteCommissionChannel(t, 1301, "default")
	seedInviteCommissionChannel(t, 1302, "UserGroup-vip")
	seedInviteCommissionChannel(t, 1303, "profit-gpt,profit-claude")
	seedInviteCommissionAggregateGroup(t, 1304, "profit-aggregate")

	rows, err := ListInviteCommissionGroupProfitRuleRows("")
	require.NoError(t, err)
	rowMap := map[string]InviteCommissionGroupProfitRuleRow{}
	for _, row := range rows {
		rowMap[row.Group] = row
	}
	require.NotContains(t, rowMap, "default")
	require.NotContains(t, rowMap, "UserGroup-vip")
	require.Equal(t, "ratio", rowMap["profit-ratio"].Type)
	require.Equal(t, "normal", rowMap["profit-gpt"].Type)
	require.Equal(t, "normal", rowMap["profit-claude"].Type)
	require.Equal(t, "aggregate", rowMap["profit-aggregate"].Type)

	rule, err := UpsertInviteCommissionGroupProfitRule(InviteCommissionGroupProfitRule{
		Group:                   "profit-gpt",
		Service:                 "gpt",
		ProfitRateBps:           3000,
		MaxCommissionRateBps:    1500,
		ProfitShareRateBps:      6000,
		ProfitProtectionEnabled: true,
		Remark:                  "first",
	})
	require.NoError(t, err)
	require.Equal(t, "profit-gpt", rule.Group)

	_, err = UpsertInviteCommissionGroupProfitRule(InviteCommissionGroupProfitRule{
		Group:                   "profit-gpt",
		Service:                 "claude",
		ProfitRateBps:           2000,
		MaxCommissionRateBps:    1000,
		ProfitShareRateBps:      5000,
		ProfitProtectionEnabled: true,
		Remark:                  "updated",
	})
	require.NoError(t, err)
	settings := GetInviteCommissionSettings()
	require.Len(t, settings.GroupProfitRules, 1)
	require.Equal(t, "claude", settings.GroupProfitRules[0].Service)
	require.Equal(t, "updated", settings.GroupProfitRules[0].Remark)

	filteredRows, err := ListInviteCommissionGroupProfitRuleRows("gpt")
	require.NoError(t, err)
	require.Len(t, filteredRows, 1)
	require.True(t, filteredRows[0].Configured)
	require.Equal(t, 2000, filteredRows[0].ProfitRateBps)

	require.NoError(t, DeleteInviteCommissionGroupProfitRule("profit-gpt"))
	settings = GetInviteCommissionSettings()
	require.Empty(t, settings.GroupProfitRules)
}

func TestInviteCommissionConsumeLogWritesSnapshotAndUserLogsStripAdminInfo(t *testing.T) {
	truncateTables(t)
	seedInviteOwner(t, 1310, "snapshot_user")
	seedInviteCommissionChannel(t, 1311, "profit-gpt")
	_, err := UpdateInviteCommissionSettings(InviteCommissionSettings{
		DefaultLevel1RateBps: 500,
		DefaultLevel2RateBps: 150,
		GroupProfitRules: []InviteCommissionGroupProfitRule{
			{
				Group:                   "profit-gpt",
				Service:                 "gpt",
				ProfitRateBps:           3000,
				MaxCommissionRateBps:    1500,
				ProfitShareRateBps:      6000,
				ProfitProtectionEnabled: true,
			},
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	ctx.Set("username", "snapshot_user")
	RecordConsumeLog(ctx, 1310, RecordConsumeLogParams{
		ModelName: "gpt-4",
		Quota:     10000,
		Group:     "profit-gpt",
		Other:     map[string]interface{}{"billing_source": "wallet"},
	})

	var logItem Log
	require.NoError(t, LOG_DB.First(&logItem, "user_id = ?", 1310).Error)
	other, err := common.StrToMap(logItem.Other)
	require.NoError(t, err)
	adminInfo := inviteCommissionMapMap(other, "admin_info")
	commission := inviteCommissionMapMap(adminInfo, "commission")
	require.Equal(t, "profit-gpt", inviteCommissionMapString(commission, "group"))
	require.Equal(t, "gpt", inviteCommissionMapString(commission, "service"))
	require.True(t, inviteCommissionMapBool(commission, "configured"))
	require.Equal(t, int64(3000), inviteCommissionMapInt64(commission, "profit_rate_bps"))
	require.Equal(t, int64(7000), inviteCommissionMapInt64(commission, "cost_rate_bps"))
	require.Equal(t, int64(7000), inviteCommissionMapInt64(commission, "upstream_cost_quota"))
	require.Equal(t, int64(3000), inviteCommissionMapInt64(commission, "gross_profit_quota"))

	logs := []*Log{&logItem}
	formatUserLogs(logs, 0)
	userOther, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, userOther, "admin_info")
}

func TestInviteCommissionReportUsesProfitProtectionSnapshotAndMissingSnapshot(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1320, "profit_agent")
	code := seedInviteCode(t, "COM-PROFIT", 1320, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1321, "profit_invitee", 1320, code.Id)
	_, err := UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1320,
		Enabled:       true,
		Level1RateBps: 500,
		Level2RateBps: 150,
	})
	require.NoError(t, err)

	seedInviteCommissionLog(t, 1321, now, "gpt-profit-protected", 100000, map[string]interface{}{
		"billing_source": "wallet",
		"admin_info": map[string]interface{}{
			"commission": InviteCommissionLogCommissionSnapshot{
				Group:                   "profit-gpt",
				Service:                 "gpt",
				Configured:              true,
				ProfitRateBps:           3000,
				CostRateBps:             7000,
				MaxCommissionRateBps:    5000,
				ProfitShareRateBps:      6000,
				ProfitProtectionEnabled: true,
				RevenueQuota:            100000,
				UpstreamCostQuota:       70000,
				GrossProfitQuota:        30000,
				Source:                  "snapshot",
			},
		},
	})
	seedInviteCommissionLog(t, 1321, now, "gpt-missing-snapshot", 10000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLog(t, 1321, now, "gpt-unconfigured", 20000, map[string]interface{}{
		"billing_source": "wallet",
		"admin_info": map[string]interface{}{
			"commission": InviteCommissionLogCommissionSnapshot{
				Group:        "profit-unconfigured",
				Service:      "gpt",
				Configured:   false,
				RevenueQuota: 20000,
				Source:       "snapshot",
			},
		},
	})

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1320,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(130000), report.Summary.WalletConsumptionQuota)
	require.Equal(t, int64(70000), report.Summary.UpstreamCostQuota)
	require.Equal(t, int64(30000), report.Summary.GrossProfitQuota)
	require.InDelta(t, 2500.0, report.Summary.TheoreticalCommissionQuota, 0.0001)
	require.InDelta(t, 900.0, report.Summary.ProfitCapCommissionQuota, 0.0001)
	require.InDelta(t, 1600.0, report.Summary.ProfitProtectionReducedQuota, 0.0001)
	require.InDelta(t, 900.0, report.Summary.EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(10000), report.Summary.MissingProfitSnapshotQuota)

	groupStats := map[string]InviteCommissionGroupStat{}
	for _, group := range report.Groups {
		groupStats[group.Group] = group
	}
	require.Equal(t, int64(100000), groupStats["profit-gpt"].TotalConsumptionQuota)
	require.Equal(t, int64(10000), groupStats["unknown"].MissingProfitSnapshotQuota)
	require.Equal(t, int64(20000), groupStats["profit-unconfigured"].TotalConsumptionQuota)
	require.Zero(t, groupStats["profit-unconfigured"].EstimatedCommissionQuota)
}

func TestInviteCommissionSubscriptionCommissionUsesTierAndProfitProtection(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1330, "subscription_profit_agent")
	code := seedInviteCode(t, "COM-SUB-PROFIT", 1330, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1331, "subscription_profit_invitee", 1330, code.Id)
	_, err := UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1330,
		Enabled:       true,
		Level1RateBps: 1000,
		Level2RateBps: 0,
	})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          13301,
		UserId:      1331,
		PlanId:      1,
		AmountTotal: 100000,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "order",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          13302,
		UserId:      1331,
		PlanId:      1,
		AmountTotal: 100000,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "admin",
	}).Error)

	seedInviteCommissionLog(t, 1331, now, "gpt-subscription-order", 100000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       13301,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 100000,
		"admin_info": map[string]interface{}{
			"commission": InviteCommissionLogCommissionSnapshot{
				Group:                   "profit-gpt",
				Service:                 "gpt",
				Configured:              true,
				ProfitRateBps:           2000,
				CostRateBps:             8000,
				MaxCommissionRateBps:    1000,
				ProfitShareRateBps:      5000,
				ProfitProtectionEnabled: true,
				RevenueQuota:            100000,
				UpstreamCostQuota:       80000,
				GrossProfitQuota:        20000,
				Source:                  "snapshot",
			},
		},
	})
	seedInviteCommissionLog(t, 1331, now, "gpt-subscription-admin", 50000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       13302,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 50000,
		"admin_info": map[string]interface{}{
			"commission": InviteCommissionLogCommissionSnapshot{
				Group:                   "profit-gpt",
				Service:                 "gpt",
				Configured:              true,
				ProfitRateBps:           2000,
				CostRateBps:             8000,
				MaxCommissionRateBps:    1000,
				ProfitShareRateBps:      5000,
				ProfitProtectionEnabled: true,
				RevenueQuota:            50000,
				UpstreamCostQuota:       40000,
				GrossProfitQuota:        10000,
				Source:                  "snapshot",
			},
		},
	})

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1330,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(150000), report.Summary.SubscriptionConsumptionQuota)
	require.Equal(t, int64(100000), report.Summary.OrderSubscriptionConsumption)
	require.Equal(t, int64(50000), report.Summary.AdminSubscriptionConsumption)
	require.InDelta(t, 1000.0, report.Summary.OrderSubscriptionCommission, 0.0001)
	require.InDelta(t, 1000.0, report.Summary.SubscriptionCommissionQuota, 0.0001)
	require.InDelta(t, 1500.0, report.Summary.TheoreticalCommissionQuota, 0.0001)
	require.InDelta(t, 1000.0, report.Summary.ProfitCapCommissionQuota, 0.0001)
	require.InDelta(t, 500.0, report.Summary.ProfitProtectionReducedQuota, 0.0001)
}

func TestInviteCommissionRequiresEnabledAgentConfigForCommission(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1110, "agent_without_config")
	code := seedInviteCode(t, "COM-NO-CONFIG", 1110, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1111, "no_config_invitee", 1110, code.Id)
	seedInviteCommissionLogWithSnapshot(t, 1111, now, "gpt-4", 10000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1110,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
	})
	require.NoError(t, err)
	require.False(t, report.Effective.Enabled)
	require.False(t, report.Effective.UseUserConfig)
	require.Equal(t, 1, report.Summary.InviteeCount)
	require.Equal(t, int64(10000), report.Summary.WalletConsumptionQuota)
	require.Zero(t, report.Levels[0].RateBps)
	require.Zero(t, report.Levels[1].RateBps)
	require.Zero(t, report.Summary.EstimatedCommissionQuota)
}

func TestInviteCommissionServiceClassificationUsesProfitSnapshotOnly(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	seedInviteOwner(t, 1120, "agent_classification")
	code := seedInviteCode(t, "COM-CLASSIFY", 1120, "default", 0, 0, 0, InviteCodeStatusEnabled)
	_, err := UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1120,
		Enabled:       true,
		Level1RateBps: 500,
		Level2RateBps: 150,
	})
	require.NoError(t, err)
	seedInviteCommissionInvitee(t, 1121, "classification_invitee", 1120, code.Id)

	seedInviteCommissionLogWithSnapshot(t, 1121, now, "gpt-5.4", 10000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1121, now, "claude-sonnet-4-6", 20000, "claude", 1000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1121, now, "gemini-3.1-flash-image", 30000, "gemini", 1000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1121, now, "alias-model", 50000, "gemini", 1000, map[string]interface{}{
		"billing_source":       "wallet",
		"upstream_model_name":  "gemini-3.1-pro",
		"subscription_ignored": "not subscription",
	})
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           1121,
		Username:         "classification_invitee",
		CreatedAt:        now,
		Type:             LogTypeConsume,
		ModelName:        "gpt-private-fast-model",
		Quota:            40000,
		PromptTokens:     4000,
		CompletionTokens: 2000,
		ChannelId:        9001,
		Group:            "claude-named-routing-group",
		Other:            common.MapToJsonStr(map[string]interface{}{"billing_source": "wallet"}),
	}).Error)

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1120,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)

	serviceStats := map[string]InviteCommissionServiceStat{}
	for _, service := range report.Services {
		serviceStats[service.Service] = service
	}
	require.Equal(t, int64(10000), serviceStats["gpt"].WalletConsumptionQuota)
	require.InDelta(t, 100.0, serviceStats["gpt"].EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(20000), serviceStats["claude"].WalletConsumptionQuota)
	require.InDelta(t, 100.0, serviceStats["claude"].EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(80000), serviceStats["gemini"].WalletConsumptionQuota)
	require.InDelta(t, 400.0, serviceStats["gemini"].EstimatedCommissionQuota, 0.0001)
	require.Equal(t, int64(40000), serviceStats["other"].WalletConsumptionQuota)
	require.Zero(t, serviceStats["other"].EstimatedCommissionQuota)
	require.InDelta(t, 600.0, report.Summary.EstimatedCommissionQuota, 0.0001)

	modelServices := map[string]string{}
	for _, modelStat := range report.Models {
		modelServices[modelStat.ModelName] = modelStat.Service
	}
	require.Equal(t, "gemini", modelServices["gemini-3.1-pro"])
	require.Equal(t, "other", modelServices["gpt-private-fast-model"])
}

func TestInviteCommissionServiceCategoriesDriveReportLabels(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	_, err := UpdateInviteCommissionSettings(InviteCommissionSettings{
		DefaultLevel1RateBps: 500,
		DefaultLevel2RateBps: 150,
		ServiceCategories: []InviteCommissionServiceCategory{
			{Service: "deepseek", Label: "DeepSeek"},
			{Service: "other", Label: "Other"},
		},
	})
	require.NoError(t, err)
	seedInviteOwner(t, 1130, "agent_service_category")
	code := seedInviteCode(t, "COM-SERVICE-CAT", 1130, "default", 0, 0, 0, InviteCodeStatusEnabled)
	_, err = UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1130,
		Enabled:       true,
		Level1RateBps: 500,
		Level2RateBps: 150,
	})
	require.NoError(t, err)
	seedInviteCommissionInvitee(t, 1131, "service_category_invitee", 1130, code.Id)
	seedInviteCommissionLogWithSnapshot(t, 1131, now, "deepseek-chat", 10000, "deepseek", 1000, map[string]interface{}{"billing_source": "wallet"})

	report, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1130,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)

	serviceStats := map[string]InviteCommissionServiceStat{}
	for _, service := range report.Services {
		serviceStats[service.Service] = service
	}
	require.Equal(t, "DeepSeek", serviceStats["deepseek"].Label)
	require.Equal(t, int64(10000), serviceStats["deepseek"].WalletConsumptionQuota)
	require.InDelta(t, 50.0, serviceStats["deepseek"].EstimatedCommissionQuota, 0.0001)
}

func TestInviteCommissionComplexChainWithCustomConfigAndSubscriptions(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	_, err := UpdateInviteCommissionSettings(InviteCommissionSettings{
		DefaultLevel1RateBps: 500,
		DefaultLevel2RateBps: 150,
		SubscriptionTiers: []InviteCommissionSubscriptionTier{
			{StartPercent: 0, EndPercent: 33, RateBps: 1500},
			{StartPercent: 33, EndPercent: 66, RateBps: 750},
			{StartPercent: 66, EndPercent: 100, RateBps: 0},
		},
	})
	require.NoError(t, err)

	seedInviteOwner(t, 1200, "complex_agent_a")
	codeA := seedInviteCode(t, "COMPLEX-A", 1200, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1201, "complex_agent_b", 1200, codeA.Id)
	codeB := seedInviteCode(t, "COMPLEX-B", 1201, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1202, "complex_c1", 1201, codeB.Id)
	seedInviteCommissionInvitee(t, 1203, "complex_c2", 1201, codeB.Id)
	codeC1 := seedInviteCode(t, "COMPLEX-C1", 1202, "default", 0, 0, 0, InviteCodeStatusEnabled)
	codeC2 := seedInviteCode(t, "COMPLEX-C2", 1203, "default", 0, 0, 0, InviteCodeStatusEnabled)
	seedInviteCommissionInvitee(t, 1204, "complex_d1", 1202, codeC1.Id)
	seedInviteCommissionInvitee(t, 1205, "complex_d2", 1203, codeC2.Id)

	_, err = UpsertInviteCommissionUserConfig(&InviteCommissionUserConfig{
		UserID:        1201,
		Enabled:       true,
		Level1RateBps: 800,
		Level2RateBps: 300,
		Remark:        "complex custom config",
	})
	require.NoError(t, err)

	require.NoError(t, DB.Create(&SubscriptionPlan{
		Id:            1300,
		Title:         "complex plan",
		PriceAmount:   12,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   100000,
	}).Error)

	seedInviteCommissionTopUp(t, 1201, "complex-wallet-b", 100, 10, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1202, "complex-wallet-c1", 200, 20, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1203, "complex-wallet-c2", 300, 30, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1204, "complex-wallet-d1", 400, 40, now, common.TopUpStatusSuccess)
	seedInviteCommissionTopUp(t, 1205, "complex-wallet-d2", 500, 50, now, common.TopUpStatusSuccess)

	seedInviteCommissionRedemption(t, 1201, 1000, now)
	seedInviteCommissionRedemption(t, 1202, 2000, now)
	seedInviteCommissionRedemption(t, 1203, 3000, now)
	seedInviteCommissionRedemption(t, 1204, 4000, now)
	seedInviteCommissionRedemption(t, 1205, 5000, now)

	seedInviteCommissionSubscriptionOrder(t, 1201, 1300, "complex-sub-b", 11, now)
	seedInviteCommissionSubscriptionOrder(t, 1202, 1300, "complex-sub-c1", 12, now)
	seedInviteCommissionSubscriptionOrder(t, 1203, 1300, "complex-sub-c2", 13, now)
	seedInviteCommissionSubscriptionOrder(t, 1204, 1300, "complex-sub-d1", 14, now)
	seedInviteCommissionSubscriptionOrder(t, 1205, 1300, "complex-sub-d2", 15, now)

	for _, sub := range []UserSubscription{
		{Id: 1401, UserId: 1201, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "order"},
		{Id: 1402, UserId: 1202, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "order"},
		{Id: 1403, UserId: 1203, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "order"},
		{Id: 1404, UserId: 1204, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "order"},
		{Id: 1405, UserId: 1205, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "order"},
		{Id: 1491, UserId: 1201, PlanId: 1300, AmountTotal: 100000, StartTime: now - 3600, EndTime: now + 3600, Status: "active", Source: "admin"},
	} {
		item := sub
		require.NoError(t, DB.Create(&item).Error)
	}

	seedInviteCommissionLogWithSnapshot(t, 1201, now, "gpt-4", 10000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1201, now, "claude-sonnet-4-6", 20000, "claude", 1000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       1401,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 20000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1201, now, "gemini-3.1-flash-image", 30000, "gemini", 1000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       1491,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 30000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1202, now, "claude-sonnet-4-6", 40000, "claude", 1000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1202, now, "gemini-3.1-flash-image", 50000, "gemini", 1000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       1402,
		"subscription_total":    100000,
		"subscription_used":     50000,
		"subscription_consumed": 50000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1203, now, "gpt-image-2", 60000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1203, now, "gpt-image-2", 70000, "gpt", 2000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       1403,
		"subscription_total":    100000,
		"subscription_used":     70000,
		"subscription_consumed": 70000,
	})
	seedInviteCommissionLogWithSnapshot(t, 1204, now, "gpt-4", 80000, "gpt", 2000, map[string]interface{}{"billing_source": "wallet"})
	seedInviteCommissionLogWithSnapshot(t, 1205, now, "claude-sonnet-4-6", 90000, "claude", 1000, map[string]interface{}{
		"billing_source":        "subscription",
		"subscription_id":       1405,
		"subscription_total":    100000,
		"subscription_used":     20000,
		"subscription_consumed": 90000,
	})

	reportA, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1200,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)
	require.False(t, reportA.Effective.UseUserConfig)
	require.False(t, reportA.Effective.Enabled)
	require.Zero(t, reportA.Levels[0].RateBps)
	require.Zero(t, reportA.Levels[1].RateBps)
	require.Equal(t, 3, reportA.Summary.InviteeCount)
	require.Equal(t, 1, reportA.Summary.Level1InviteeCount)
	require.Equal(t, 2, reportA.Summary.Level2InviteeCount)
	require.Equal(t, int64(600), reportA.Summary.WalletRechargeAmount)
	require.Equal(t, 60.0, reportA.Summary.WalletRechargeMoney)
	require.Equal(t, int64(6000), reportA.Summary.RedemptionQuota)
	require.Equal(t, 36.0, reportA.Summary.SubscriptionPurchaseMoney)
	require.Equal(t, int64(110000), reportA.Summary.WalletConsumptionQuota)
	require.Equal(t, int64(170000), reportA.Summary.SubscriptionConsumptionQuota)
	require.Equal(t, int64(30000), reportA.Summary.AdminSubscriptionConsumption)
	require.Equal(t, int64(140000), reportA.Summary.OrderSubscriptionConsumption)
	require.Zero(t, reportA.Summary.EstimatedCommissionQuota)
	require.Zero(t, reportA.Summary.WalletCommissionQuota)
	require.Zero(t, reportA.Summary.SubscriptionCommissionQuota)

	reportB, err := BuildInviteCommissionReport(InviteCommissionReportRequest{
		OwnerUserID:    1201,
		StartTimestamp: now - 3600,
		EndTimestamp:   now + 3600,
		IncludeDetails: true,
	})
	require.NoError(t, err)
	require.True(t, reportB.Effective.UseUserConfig)
	require.Equal(t, 4, reportB.Summary.InviteeCount)
	require.Equal(t, 2, reportB.Summary.Level1InviteeCount)
	require.Equal(t, 2, reportB.Summary.Level2InviteeCount)
	require.Equal(t, int64(1400), reportB.Summary.WalletRechargeAmount)
	require.Equal(t, 140.0, reportB.Summary.WalletRechargeMoney)
	require.Equal(t, int64(14000), reportB.Summary.RedemptionQuota)
	require.Equal(t, 54.0, reportB.Summary.SubscriptionPurchaseMoney)
	require.Equal(t, int64(180000), reportB.Summary.WalletConsumptionQuota)
	require.Equal(t, int64(210000), reportB.Summary.SubscriptionConsumptionQuota)
	require.InDelta(t, 2465.0, reportB.Summary.EstimatedCommissionQuota, 0.0001)
	require.InDelta(t, 1760.0, reportB.Summary.WalletCommissionQuota, 0.0001)
	require.InDelta(t, 705.0, reportB.Summary.SubscriptionCommissionQuota, 0.0001)
}
