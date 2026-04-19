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
	InviteCodeCount int                    `json:"invite_code_count"`
	InviteCodes     []model.InviteCode     `json:"invite_codes"`
	InviteUserCount int                    `json:"invite_user_count"`
	Invitees        []model.InviteeSummary `json:"invitees"`
	BoundInviteCode *model.InviteCode      `json:"bound_invite_code"`
}

type selfInviteCodePageResponse struct {
	Total int                `json:"total"`
	Items []model.InviteCode `json:"items"`
}

type selfInviteePageResponse struct {
	Total int                    `json:"total"`
	Items []model.InviteeSummary `json:"items"`
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
