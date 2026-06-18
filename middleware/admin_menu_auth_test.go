package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAdminMenuAuthTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AdminMenuPermission{}))

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

func runAdminMenuAuth(role int, userId int, menuKey string) (*httptest.ResponseRecorder, bool) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	c.Set("id", userId)
	c.Set("role", role)
	called := false
	AdminMenuAuth(menuKey)(c)
	if !c.IsAborted() {
		called = true
	}
	return recorder, called
}

func TestAdminMenuAuthAllowsRootWithoutRows(t *testing.T) {
	db := setupAdminMenuAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "root", Password: "password123", Role: common.RoleRootUser, Status: common.UserStatusEnabled}).Error)

	_, called := runAdminMenuAuth(common.RoleRootUser, 1, model.AdminMenuChannel)
	require.True(t, called)
}

func TestAdminMenuAuthAllowsExplicitAdminPermission(t *testing.T) {
	db := setupAdminMenuAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "admin", Password: "password123", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.SetAdminMenuPermissions(2, []string{model.AdminMenuChannel}))

	_, called := runAdminMenuAuth(common.RoleAdminUser, 2, model.AdminMenuChannel)
	require.True(t, called)
}

func TestAdminMenuAuthRejectsMissingAdminPermission(t *testing.T) {
	db := setupAdminMenuAuthTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "admin", Password: "password123", Role: common.RoleAdminUser, Status: common.UserStatusEnabled}).Error)

	recorder, called := runAdminMenuAuth(common.RoleAdminUser, 2, model.AdminMenuChannel)
	require.False(t, called)
	require.Contains(t, recorder.Body.String(), "菜单权限不足")
}
