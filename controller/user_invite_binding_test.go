package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userInviteCodePageResponse struct {
	Total int                `json:"total"`
	Items []model.InviteCode `json:"items"`
}

func performAdminInviteBindingRequest(t *testing.T, method string, target string, body []byte, handler gin.HandlerFunc) tokenAPIResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "92"}}
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)

	handler(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestUpdateAndDeleteUserInviteBindingWriteManageLogs(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 90, "manual_log_owner", "aff-manual-log-owner")
	seedInviteCodeControllerUser(t, db, 91, "selected_log_owner", "aff-selected-log-owner")
	seedInviteCodeControllerUser(t, db, 92, "binding_log_invitee", "aff-binding-log-invitee")

	selectedCode := &model.InviteCode{
		Code:              "LOG-SELECTED",
		Prefix:            "LOG",
		OwnerUserId:       91,
		TargetGroup:       "default",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, selectedCode.Insert())

	bindResp := performAdminInviteBindingRequest(
		t,
		http.MethodPut,
		"/api/user/92/invite_binding",
		[]byte(`{"owner_user_id":90}`),
		UpdateUserInviteBinding,
	)
	require.True(t, bindResp.Success, bindResp.Message)

	rebindResp := performAdminInviteBindingRequest(
		t,
		http.MethodPut,
		"/api/user/92/invite_binding",
		[]byte(`{"owner_user_id":91,"invite_code_id":`+strconv.Itoa(selectedCode.Id)+`}`),
		UpdateUserInviteBinding,
	)
	require.True(t, rebindResp.Success, rebindResp.Message)

	unbindResp := performAdminInviteBindingRequest(
		t,
		http.MethodDelete,
		"/api/user/92/invite_binding",
		nil,
		DeleteUserInviteBinding,
	)
	require.True(t, unbindResp.Success, unbindResp.Message)

	var logs []model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", 92, model.LogTypeManage).Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 3)
	require.Contains(t, logs[0].Content, "旧邀请人=0")
	require.Contains(t, logs[0].Content, "新邀请人=90")
	require.Contains(t, logs[1].Content, "旧邀请人=90")
	require.Contains(t, logs[1].Content, "新邀请人=91")
	require.Contains(t, logs[2].Content, "旧邀请人=91")
	require.Contains(t, logs[2].Content, "新邀请人=0")
}

func TestGetUserInviteCodesByAdminExcludesDeletedCodes(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 93, "bindable_owner", "aff-bindable-owner")
	activeCode := &model.InviteCode{
		Code:              "BIND-ACTIVE",
		Prefix:            "BIND",
		OwnerUserId:       93,
		TargetGroup:       "default",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	deletedCode := &model.InviteCode{
		Code:              "BIND-DELETED",
		Prefix:            "BIND",
		OwnerUserId:       93,
		TargetGroup:       "default",
		RewardQuotaPerUse: 0,
		RewardTotalUses:   0,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, activeCode.Insert())
	require.NoError(t, deletedCode.Insert())
	require.NoError(t, deletedCode.Delete())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/93/invite_codes?p=1&page_size=10", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "93"}}
	GetUserInviteCodesByAdmin(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var data userInviteCodePageResponse
	require.NoError(t, common.Unmarshal(resp.Data, &data))
	require.Equal(t, 1, data.Total)
	require.Len(t, data.Items, 1)
	require.Equal(t, "BIND-ACTIVE", data.Items[0].Code)
}
