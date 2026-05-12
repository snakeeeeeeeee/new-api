package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type selfInvitePreviewResponse struct {
	InviteCodeCount    int                    `json:"invite_code_count"`
	InviteCodes        []model.InviteCode     `json:"invite_codes"`
	InviteUserCount    int                    `json:"invite_user_count"`
	Invitees           []model.InviteeSummary `json:"invitees"`
	BoundInviteCode    *model.InviteCode      `json:"bound_invite_code"`
	InviteAgentLevel   int                    `json:"invite_agent_level"`
	CanGrantInvitation bool                   `json:"can_grant_invitation"`
}

type selfInviteCodePageResponse struct {
	Total int                `json:"total"`
	Items []model.InviteCode `json:"items"`
}

type selfInviteePageResponse struct {
	Total int                    `json:"total"`
	Items []model.InviteeSummary `json:"items"`
}

type adminUserDetailResponse struct {
	Id                int                            `json:"id"`
	Username          string                         `json:"username"`
	Quota             int                            `json:"quota"`
	UsedQuota         int                            `json:"used_quota"`
	RequestCount      int                            `json:"request_count"`
	SubscriptionQuota model.SubscriptionQuotaSummary `json:"subscription_quota"`
}

func TestGetUserByUsernameIncludesSubscriptionQuota(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	user := &model.User{
		Id:           80,
		Username:     "dashboard_user",
		Password:     "password123",
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		Group:        "default",
		AffCode:      "aff-dashboard-user",
		Quota:        12345,
		UsedQuota:    678,
		RequestCount: 9,
	}
	require.NoError(t, db.Create(user).Error)

	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.UserSubscription{
		UserId:      80,
		PlanId:      1,
		AmountTotal: 1000,
		AmountUsed:  250,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/by_username?username=dashboard_user", nil)
	ctx.Set("role", common.RoleAdminUser)
	GetUserByUsername(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var data adminUserDetailResponse
	require.NoError(t, common.Unmarshal(resp.Data, &data))
	require.Equal(t, 80, data.Id)
	require.Equal(t, "dashboard_user", data.Username)
	require.Equal(t, 12345, data.Quota)
	require.Equal(t, 678, data.UsedQuota)
	require.Equal(t, 9, data.RequestCount)
	require.Equal(t, 1, data.SubscriptionQuota.ActiveCount)
	require.Equal(t, int64(1000), data.SubscriptionQuota.AmountTotal)
	require.Equal(t, int64(750), data.SubscriptionQuota.AmountRemain)
}

func TestGetSelfIncludesBoundInviteCodeAndPreviewData(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 49, "upstream_owner", "aff-upstream")

	boundCode := &model.InviteCode{
		Code:              "UP-BOUND01",
		Prefix:            "UP",
		OwnerUserId:       49,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 1000,
		RewardTotalUses:   5,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, boundCode.Insert())
	require.NoError(t, boundCode.Delete())

	currentUser := &model.User{
		Id:                50,
		Username:          "self_owner",
		Password:          "password123",
		DisplayName:       "self_owner",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-self",
		InviteCodeId:      boundCode.Id,
		InviteCodeOwnerId: 49,
		InviterId:         49,
	}
	require.NoError(t, db.Create(currentUser).Error)

	var ownerCodes []*model.InviteCode
	for i := 0; i < 6; i++ {
		code := &model.InviteCode{
			Code:              fmt.Sprintf("OWN-CODE%02d", i+1),
			Prefix:            "OWN",
			OwnerUserId:       50,
			TargetGroup:       "vip",
			RewardQuotaPerUse: 2000,
			RewardTotalUses:   3,
			Status:            model.InviteCodeStatusEnabled,
		}
		require.NoError(t, code.Insert())
		ownerCodes = append(ownerCodes, code)
	}

	inviteeA := &model.User{
		Id:                501,
		Username:          "invitee_preview_a",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      ownerCodes[0].Id,
		InviteCodeOwnerId: 50,
		InviterId:         50,
		AffCode:           "aff-preview-a",
	}
	inviteeB := &model.User{
		Id:                502,
		Username:          "invitee_preview_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      ownerCodes[1].Id,
		InviteCodeOwnerId: 50,
		InviterId:         50,
		AffCode:           "aff-preview-b",
	}
	require.NoError(t, db.Create(inviteeA).Error)
	require.NoError(t, db.Create(inviteeB).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", 50)
	ctx.Set("role", common.RoleCommonUser)
	GetSelf(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var data selfInvitePreviewResponse
	require.NoError(t, common.Unmarshal(resp.Data, &data))
	require.Equal(t, 6, data.InviteCodeCount)
	require.Len(t, data.InviteCodes, 5)
	require.Equal(t, 2, data.InviteUserCount)
	require.Len(t, data.Invitees, 2)
	require.NotNil(t, data.BoundInviteCode)
	require.Equal(t, "UP-BOUND01", data.BoundInviteCode.Code)
	require.True(t, data.BoundInviteCode.IsDeleted)
}

func TestGetSelfInviteCodesIncludesDeletedEntries(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 60, "owner_codes", "aff-owner-codes")

	activeCode := &model.InviteCode{
		Code:              "OWN-ACT001",
		Prefix:            "OWN",
		OwnerUserId:       60,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 1000,
		RewardTotalUses:   2,
		Status:            model.InviteCodeStatusEnabled,
	}
	deletedCode := &model.InviteCode{
		Code:              "OWN-DEL001",
		Prefix:            "OWN",
		OwnerUserId:       60,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 1000,
		RewardTotalUses:   2,
		Status:            model.InviteCodeStatusDisabled,
	}
	require.NoError(t, activeCode.Insert())
	require.NoError(t, deletedCode.Insert())
	require.NoError(t, deletedCode.Delete())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/invite_codes?p=1&page_size=10", nil)
	ctx.Set("id", 60)
	GetSelfInviteCodes(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var data selfInviteCodePageResponse
	require.NoError(t, common.Unmarshal(resp.Data, &data))
	require.Equal(t, 2, data.Total)
	require.Len(t, data.Items, 2)

	codeMap := make(map[string]model.InviteCode, len(data.Items))
	for _, item := range data.Items {
		codeMap[item.Code] = item
	}
	require.True(t, codeMap["OWN-DEL001"].IsDeleted)
	require.False(t, codeMap["OWN-ACT001"].IsDeleted)
}

func TestGetSelfInviteesIncludesInviteCodeState(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 70, "owner_invitees", "aff-owner-invitees")

	disabledCode := &model.InviteCode{
		Code:              "INV-DIS001",
		Prefix:            "INV",
		OwnerUserId:       70,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusDisabled,
	}
	deletedCode := &model.InviteCode{
		Code:              "INV-DEL001",
		Prefix:            "INV",
		OwnerUserId:       70,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, disabledCode.Insert())
	require.NoError(t, deletedCode.Insert())
	require.NoError(t, deletedCode.Delete())

	inviteeDisabled := &model.User{
		Id:                701,
		Username:          "invitee_disabled_ui",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      disabledCode.Id,
		InviteCodeOwnerId: 70,
		InviterId:         70,
		AffCode:           "aff-invitee-disabled-ui",
	}
	inviteeDeleted := &model.User{
		Id:                702,
		Username:          "invitee_deleted_ui",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		InviteCodeId:      deletedCode.Id,
		InviteCodeOwnerId: 70,
		InviterId:         70,
		AffCode:           "aff-invitee-deleted-ui",
	}
	require.NoError(t, db.Create(inviteeDisabled).Error)
	require.NoError(t, db.Create(inviteeDeleted).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/invitees?p=1&page_size=10", nil)
	ctx.Set("id", 70)
	GetSelfInvitees(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var data selfInviteePageResponse
	require.NoError(t, common.Unmarshal(resp.Data, &data))
	require.Equal(t, 2, data.Total)
	require.Len(t, data.Items, 2)

	inviteeMap := make(map[string]model.InviteeSummary, len(data.Items))
	for _, item := range data.Items {
		inviteeMap[item.Username] = item
	}
	require.Equal(t, model.InviteCodeStatusDisabled, inviteeMap["invitee_disabled_ui"].InviteCodeStatus)
	require.False(t, inviteeMap["invitee_disabled_ui"].InviteCodeDeleted)
	require.True(t, inviteeMap["invitee_deleted_ui"].InviteCodeDeleted)
}

func TestEnableSelfInviteeInvitationCreatesSecondLevelCode(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 100, "agent_controller_a", "aff-agent-controller-a")

	firstCode := &model.InviteCode{
		Code:              "CTRL-A-001",
		Prefix:            "CTRL",
		OwnerUserId:       100,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, firstCode.Insert())
	require.NoError(t, db.Create(&model.User{
		Id:                101,
		Username:          "agent_controller_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-controller-b",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 100,
		InviterId:         100,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/self/invitees/101/enable_invitation", nil)
	ctx.Set("id", 100)
	ctx.Params = gin.Params{{Key: "id", Value: "101"}}
	EnableSelfInviteeInvitation(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var code model.InviteCode
	require.NoError(t, common.Unmarshal(resp.Data, &code))
	require.Equal(t, 101, code.OwnerUserId)
	require.Equal(t, model.InviteAgentLevelSecond, code.AgentLevel)
	require.Equal(t, 100, code.GrantedByUserId)
}

func TestGetSelfInviteAgentStatsReturnsSecondLevelRows(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 110, "agent_stats_a", "aff-agent-stats-a")
	firstCode := &model.InviteCode{
		Code:              "CTRL-STATS-A",
		Prefix:            "CTRL",
		OwnerUserId:       110,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, firstCode.Insert())
	require.NoError(t, db.Create(&model.User{
		Id:                111,
		Username:          "agent_stats_b",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-stats-b",
		InviteCodeId:      firstCode.Id,
		InviteCodeOwnerId: 110,
		InviterId:         110,
		UsedQuota:         500,
	}).Error)
	code, err := model.EnableInviteeInvitation(110, 111)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Id:                112,
		Username:          "agent_stats_c",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "vip",
		AffCode:           "aff-agent-stats-c",
		InviteCodeId:      code.Id,
		InviteCodeOwnerId: 111,
		InviterId:         111,
		UsedQuota:         900,
	}).Error)
	require.NoError(t, db.Create(&model.TopUp{
		UserId:       112,
		Amount:       120,
		Money:        12,
		TradeNo:      "ctrl-agent-stats-c",
		CreateTime:   common.GetTimestamp(),
		CompleteTime: common.GetTimestamp(),
		Status:       common.TopUpStatusSuccess,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/invite_agent_stats?period=day", nil)
	ctx.Set("id", 110)
	GetSelfInviteAgentStats(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var stats model.InviteAgentStatsResponse
	require.NoError(t, common.Unmarshal(resp.Data, &stats))
	require.Equal(t, model.InviteAgentLevelFirst, stats.AgentLevel)
	require.Len(t, stats.SecondLevelStats, 1)
	require.Equal(t, 111, stats.SecondLevelStats[0].UserID)
	require.Equal(t, int64(120), stats.SecondLevelStats[0].InviteeStats.RechargeAmount)
	require.Equal(t, 12.0, stats.SecondLevelStats[0].InviteeStats.RechargeUSD)
	require.Equal(t, 900, stats.SecondLevelStats[0].InviteeStats.ConsumeQuota)
	require.Equal(t, float64(900)/common.QuotaPerUnit, stats.SecondLevelStats[0].InviteeStats.ConsumeUSD)
}
