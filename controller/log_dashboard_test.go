package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetLogsDashboardReturnsStructuredData(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	restoreNow := service.SetLogDashboardNowForTest(func() time.Time {
		return time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local)
	})
	defer restoreNow()

	adminToken := "admin-dashboard-token"
	adminUser := &model.User{
		Id:          9001,
		Username:    "admin_dashboard",
		Password:    "password123",
		DisplayName: "admin_dashboard",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-admin-dashboard",
		AccessToken: &adminToken,
	}
	require.NoError(t, db.Create(adminUser).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    adminUser.Id,
		CreatedAt: time.Date(2026, 4, 21, 9, 30, 0, 0, time.Local).Unix(),
		Type:      model.LogTypeConsume,
		Content:   "success",
		UseTime:   2,
		RequestId: "req-success-1",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/dashboard?window=1h", nil)
	GetLogsDashboard(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"window":"1h"`)
	require.Contains(t, string(resp.Data), `"total_requests":1`)
}

func TestGetLogsDashboardRejectsInvalidWindow(t *testing.T) {
	setupInviteCodeControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/dashboard?window=2h", nil)
	GetLogsDashboard(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "invalid window")
}

func TestGetLogsDashboardRequiresAdmin(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	userToken := "common-dashboard-token"
	user := &model.User{
		Id:          9101,
		Username:    "common_dashboard",
		Password:    "password123",
		DisplayName: "common_dashboard",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-common-dashboard",
		AccessToken: &userToken,
	}
	require.NoError(t, db.Create(user).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("log-dashboard-test"))
	router.Use(sessions.Sessions("test_session", store))
	router.GET("/api/log/dashboard", middleware.AdminAuth(), GetLogsDashboard)

	req := httptest.NewRequest(http.MethodGet, "/api/log/dashboard?window=1h", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("New-Api-User", "9101")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "权限不足")
}

func TestGetUsageStatsReturnsStructuredData(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	base := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.Log{
		UserId:           901,
		Username:         "usage_admin_target",
		CreatedAt:        base,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            123,
		PromptTokens:     11,
		CompletionTokens: 22,
		UseTime:          3,
		Group:            "default",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage_stats?start_timestamp="+strconv.FormatInt(base-60, 10)+"&end_timestamp="+strconv.FormatInt(base+3600, 10)+"&trend_granularity=hour", nil)
	GetUsageStats(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"quota":123`)
	require.Contains(t, string(resp.Data), `"input_tokens":11`)
	require.Contains(t, string(resp.Data), `"cache_tokens":0`)
	require.Contains(t, string(resp.Data), `"total_tokens":33`)
	require.Contains(t, string(resp.Data), `"ranking"`)
	require.Contains(t, string(resp.Data), `"trend"`)
	require.Contains(t, string(resp.Data), `"models"`)
	require.Contains(t, string(resp.Data), `"user_model_details"`)
	require.Contains(t, string(resp.Data), `"subscription_ranking"`)
	require.Contains(t, string(resp.Data), `"unknown_quota":123`)
}

func TestGetUsageStatsPassesSectionAndBillingSource(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.Log{UserId: 904, Username: "wallet_user", CreatedAt: base, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 100, Other: `{"billing_source":"wallet"}`}).Error)
	require.NoError(t, db.Create(&model.Log{UserId: 905, Username: "subscription_user", CreatedAt: base + 1, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 250, Other: `{"billing_source":"subscription"}`}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage_stats?section=usage&billing_source=subscription&start_timestamp="+strconv.FormatInt(base-1, 10)+"&end_timestamp="+strconv.FormatInt(base+60, 10), nil)
	GetUsageStats(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"quota":250`)
	require.Contains(t, string(resp.Data), `"user_id":905`)
	require.NotContains(t, string(resp.Data), `"user_id":904`)
}

func TestGetUsageStatsPassesUserIDFilter(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	base := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.Log{
		UserId:           902,
		Username:         "usage_user_filter_a",
		CreatedAt:        base,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            123,
		PromptTokens:     11,
		CompletionTokens: 22,
	}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:           903,
		Username:         "usage_user_filter_b",
		CreatedAt:        base,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            999,
		PromptTokens:     99,
		CompletionTokens: 99,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage_stats?user_id=902&start_timestamp="+strconv.FormatInt(base-60, 10)+"&end_timestamp="+strconv.FormatInt(base+3600, 10)+"&trend_granularity=hour", nil)
	GetUsageStats(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"quota":123`)
	require.Contains(t, string(resp.Data), `"user_id":902`)
	require.NotContains(t, string(resp.Data), `"user_id":903`)
}

func TestGetUsageStatsRejectsInvalidGranularity(t *testing.T) {
	setupInviteCodeControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/usage_stats?trend_granularity=minute", nil)
	GetUsageStats(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "trend_granularity")
}

func TestGetUsageStatsRequiresAdmin(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	userToken := "common-usage-stats-token"
	user := &model.User{
		Id:          9102,
		Username:    "common_usage_stats",
		Password:    "password123",
		DisplayName: "common_usage_stats",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-common-usage-stats",
		AccessToken: &userToken,
	}
	require.NoError(t, db.Create(user).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("usage-stats-test"))
	router.Use(sessions.Sessions("test_session", store))
	router.GET("/api/log/usage_stats", middleware.AdminAuth(), GetUsageStats)

	req := httptest.NewRequest(http.MethodGet, "/api/log/usage_stats", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("New-Api-User", "9102")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "权限不足")
}
