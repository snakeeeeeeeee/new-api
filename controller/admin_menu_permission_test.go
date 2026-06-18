package controller

import (
	"bytes"
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

func setupAdminMenuPermissionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.AdminMenuPermission{}))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func adminMenuPermissionControllerCtx(method string, path string, body string, idParam string, role int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Params = gin.Params{{Key: "id", Value: idParam}}
	ctx.Set("id", 100)
	ctx.Set("role", role)
	return ctx, recorder
}

func TestUpdateAdminMenuPermissionsRootOnlyForAdminTarget(t *testing.T) {
	db := setupAdminMenuPermissionControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "admin", Password: "password123", Role: common.RoleAdminUser}).Error)

	ctx, recorder := adminMenuPermissionControllerCtx(
		http.MethodPut,
		"/api/user/2/admin_menu_permissions",
		`{"menu_keys":["channel","models"]}`,
		"2",
		common.RoleRootUser,
	)
	UpdateAdminMenuPermissions(ctx)
	require.Contains(t, recorder.Body.String(), `"success":true`)

	keys, err := model.GetAdminMenuPermissionKeys(2, common.RoleAdminUser)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{model.AdminMenuChannel, model.AdminMenuModels}, keys)

	commonCtx, commonRecorder := adminMenuPermissionControllerCtx(
		http.MethodPut,
		"/api/user/2/admin_menu_permissions",
		`{"menu_keys":["setting"]}`,
		"2",
		common.RoleRootUser,
	)
	UpdateAdminMenuPermissions(commonCtx)
	require.Contains(t, commonRecorder.Body.String(), `"success":false`)
}

func TestManageUserPromoteAndPromoteRootPermissionLifecycle(t *testing.T) {
	db := setupAdminMenuPermissionControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "common", Password: "password123", Role: common.RoleCommonUser}).Error)

	promoteCtx, promoteRecorder := adminMenuPermissionControllerCtx(
		http.MethodPost,
		"/api/user/manage",
		`{"id":2,"action":"promote"}`,
		"2",
		common.RoleRootUser,
	)
	ManageUser(promoteCtx)
	require.Contains(t, promoteRecorder.Body.String(), `"success":true`)

	var promoted model.User
	require.NoError(t, db.First(&promoted, 2).Error)
	require.Equal(t, common.RoleAdminUser, promoted.Role)
	keys, err := model.GetAdminMenuPermissionKeys(2, common.RoleAdminUser)
	require.NoError(t, err)
	require.ElementsMatch(t, model.DefaultAdminMenuPermissionKeys(), keys)

	rootCtx, rootRecorder := adminMenuPermissionControllerCtx(
		http.MethodPost,
		"/api/user/manage",
		`{"id":2,"action":"promote_root"}`,
		"2",
		common.RoleRootUser,
	)
	ManageUser(rootCtx)
	require.Contains(t, rootRecorder.Body.String(), `"success":true`)

	var rootUser model.User
	require.NoError(t, db.First(&rootUser, 2).Error)
	require.Equal(t, common.RoleRootUser, rootUser.Role)
	var count int64
	require.NoError(t, db.Model(&model.AdminMenuPermission{}).Where("user_id = ?", 2).Count(&count).Error)
	require.Zero(t, count)
}
