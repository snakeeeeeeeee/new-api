package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performInviteCommissionRequest(t *testing.T, method string, target string, body []byte, handler gin.HandlerFunc, userID int) tokenAPIResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	handler(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestInviteCommissionSettingsAndSelfReportControllers(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	originalOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	seedInviteCodeControllerUser(t, db, 1200, "controller_agent", "aff-controller-agent")
	code := &model.InviteCode{
		Code:        "CTRL-COM",
		Prefix:      "CTRL",
		OwnerUserId: 1200,
		TargetGroup: "default",
		Status:      model.InviteCodeStatusEnabled,
	}
	require.NoError(t, code.Insert())
	require.NoError(t, db.Create(&model.User{
		Id:                1201,
		Username:          "controller_invitee",
		Password:          "password123",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusEnabled,
		Group:             "default",
		AffCode:           "aff-controller-invitee",
		InviterId:         1200,
		InviteCodeOwnerId: 1200,
		InviteCodeId:      code.Id,
	}).Error)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.Log{
		UserId:    1201,
		Username:  "controller_invitee",
		CreatedAt: now,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-4",
		Quota:     10000,
		Other:     common.MapToJsonStr(map[string]interface{}{"billing_source": "wallet"}),
	}).Error)

	settingsResp := performInviteCommissionRequest(
		t,
		http.MethodPut,
		"/api/invite_commission/settings",
		[]byte(`{
			"default_level1_rate_bps":1000,
			"default_level2_rate_bps":200,
			"service_categories":[{"service":"deepseek","label":"DeepSeek","remark":"custom service"},{"service":"other","label":"Other"}],
			"subscription_tiers":[{"start_percent":0,"end_percent":100,"rate_bps":1500}],
			"group_profit_rules":[{"group":"profit-gpt","service":"deepseek","profit_rate_bps":3000,"max_commission_rate_bps":1500,"profit_share_rate_bps":6000,"profit_protection_enabled":true}]
		}`),
		UpdateInviteCommissionSettings,
		1,
	)
	require.True(t, settingsResp.Success, settingsResp.Message)
	var settings model.InviteCommissionSettings
	require.NoError(t, common.Unmarshal(settingsResp.Data, &settings))
	require.Equal(t, "deepseek", settings.ServiceCategories[0].Service)
	require.Equal(t, "DeepSeek", settings.ServiceCategories[0].Label)

	selfResp := performInviteCommissionRequest(
		t,
		http.MethodGet,
		"/api/invite_commission/self/report",
		nil,
		GetInviteCommissionSelfReport,
		1200,
	)
	require.True(t, selfResp.Success, selfResp.Message)

	var report model.InviteCommissionReport
	require.NoError(t, common.Unmarshal(selfResp.Data, &report))
	require.Equal(t, 1200, report.OwnerUserID)
	require.Equal(t, 1, report.Summary.InviteeCount)
	require.Equal(t, int64(10000), report.Summary.WalletConsumptionQuota)
	require.False(t, report.Effective.Enabled)
	require.False(t, report.Effective.UseUserConfig)
	require.Zero(t, report.Summary.EstimatedCommissionQuota)
	require.Empty(t, report.Invitees)
	require.Empty(t, report.Models)
	require.Empty(t, report.Settings.GroupProfitRules)
	require.NotContains(t, string(selfResp.Data), "profit_rate_bps")
	require.NotContains(t, string(selfResp.Data), "upstream_cost_quota")
	require.NotContains(t, string(selfResp.Data), "gross_profit_quota")
}

func TestInviteCommissionUserConfigController(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 1210, "config_agent", "aff-config-agent")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/invite_commission/user_configs/1210", bytes.NewReader([]byte(`{
		"enabled": false,
		"level1_rate_bps": 0,
		"level2_rate_bps": 0,
		"remark": "disabled test"
	}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "user_id", Value: "1210"}}
	UpdateInviteCommissionUserConfig(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var config model.InviteCommissionUserConfig
	require.NoError(t, common.Unmarshal(resp.Data, &config))
	require.Equal(t, 1210, config.UserID)
	require.False(t, config.Enabled)
	require.Zero(t, config.Level1RateBps)
	require.Zero(t, config.Level2RateBps)
	require.Equal(t, "disabled test", config.Remark)
}

func TestInviteCommissionGroupProfitRuleController(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1401,
		Type:   1,
		Key:    "controller-channel-key",
		Name:   "controller-channel",
		Group:  "controller-profit-gpt",
		Models: "gpt-4",
	}).Error)
	require.NoError(t, db.Create(&model.AggregateGroup{
		Id:          1402,
		Name:        "controller-profit-aggregate",
		DisplayName: "controller aggregate",
		Status:      model.AggregateGroupStatusEnabled,
	}).Error)

	listResp := performInviteCommissionRequest(
		t,
		http.MethodGet,
		"/api/invite_commission/group_profit_rules",
		nil,
		GetInviteCommissionGroupProfitRules,
		1,
	)
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), "controller-profit-gpt")
	require.Contains(t, string(listResp.Data), "controller-profit-aggregate")

	updateResp := performInviteCommissionRequest(
		t,
		http.MethodPut,
		"/api/invite_commission/group_profit_rules",
		[]byte(`{
			"group":"controller-profit-gpt",
			"service":"gpt",
			"profit_rate_bps":3000,
			"max_commission_rate_bps":1500,
			"profit_share_rate_bps":6000,
			"profit_protection_enabled":true,
			"remark":"controller test"
		}`),
		UpdateInviteCommissionGroupProfitRule,
		1,
	)
	require.True(t, updateResp.Success, updateResp.Message)
	var rule model.InviteCommissionGroupProfitRule
	require.NoError(t, common.Unmarshal(updateResp.Data, &rule))
	require.Equal(t, "controller-profit-gpt", rule.Group)
	require.Equal(t, 3000, rule.ProfitRateBps)

	filteredResp := performInviteCommissionRequest(
		t,
		http.MethodGet,
		"/api/invite_commission/group_profit_rules?keyword=gpt",
		nil,
		GetInviteCommissionGroupProfitRules,
		1,
	)
	require.True(t, filteredResp.Success, filteredResp.Message)
	require.Contains(t, string(filteredResp.Data), `"configured":true`)

	deleteResp := performInviteCommissionRequest(
		t,
		http.MethodDelete,
		"/api/invite_commission/group_profit_rules?group=controller-profit-gpt",
		nil,
		DeleteInviteCommissionGroupProfitRule,
		1,
	)
	require.True(t, deleteResp.Success, deleteResp.Message)
	require.Empty(t, model.GetInviteCommissionSettings().GroupProfitRules)
}

func TestInviteCommissionGroupProfitRuleRoutesRequireAdmin(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	userToken := "commission-common-user-token"
	require.NoError(t, db.Create(&model.User{
		Id:          1410,
		Username:    "commission_common_user",
		Password:    "password123",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-commission-common",
		AccessToken: &userToken,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("invite-commission-test"))
	router.Use(sessions.Sessions("test_session", store))
	router.GET("/api/invite_commission/group_profit_rules", middleware.AdminAuth(), GetInviteCommissionGroupProfitRules)

	req := httptest.NewRequest(http.MethodGet, "/api/invite_commission/group_profit_rules", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("New-Api-User", "1410")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "权限不足")
}
