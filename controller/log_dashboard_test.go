package controller

import (
	"net/http"
	"net/http/httptest"
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
