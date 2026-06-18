package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuthRoleRefreshTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func runRootAuthWithStaleSession(t *testing.T, dbRole int, sessionRole int) *httptest.ResponseRecorder {
	t.Helper()
	db := setupAuthRoleRefreshTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       2,
		Username: "role_refresh",
		Password: "password123",
		Role:     dbRole,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/root",
		func(c *gin.Context) {
			session := sessions.Default(c)
			session.Set("id", 2)
			session.Set("username", "role_refresh")
			session.Set("role", sessionRole)
			session.Set("status", common.UserStatusEnabled)
			session.Set("group", "default")
			require.NoError(t, session.Save())
			c.Next()
		},
		RootAuth(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		},
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/root", nil)
	req.Header.Set("New-Api-User", "2")
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestRootAuthRefreshesPromotedAdminSessionRole(t *testing.T) {
	recorder := runRootAuthWithStaleSession(t, common.RoleRootUser, common.RoleAdminUser)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestRootAuthRejectsDemotedRootSessionRole(t *testing.T) {
	recorder := runRootAuthWithStaleSession(t, common.RoleAdminUser, common.RoleRootUser)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "权限不足")
}
