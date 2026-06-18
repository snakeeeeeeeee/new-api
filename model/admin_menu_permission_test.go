package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAdminMenuPermissionModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &AdminMenuPermission{}))

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestAdminMenuPermissionRootHasAllWithoutRows(t *testing.T) {
	setupAdminMenuPermissionModelTestDB(t)

	ok, err := UserHasAdminMenuPermission(1, common.RoleRootUser, AdminMenuChannel)
	require.NoError(t, err)
	require.True(t, ok)

	perms, err := GetAdminMenuPermissionMap(1, common.RoleRootUser)
	require.NoError(t, err)
	require.True(t, perms[AdminMenuSetting])
	require.True(t, perms[AdminMenuChannel])
}

func TestAdminMenuPermissionAdminRequiresExplicitRows(t *testing.T) {
	setupAdminMenuPermissionModelTestDB(t)

	ok, err := UserHasAdminMenuPermission(2, common.RoleAdminUser, AdminMenuChannel)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, SetAdminMenuPermissions(2, []string{AdminMenuChannel}))
	ok, err = UserHasAdminMenuPermission(2, common.RoleAdminUser, AdminMenuChannel)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = UserHasAdminMenuPermission(2, common.RoleAdminUser, AdminMenuModels)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestBackfillAdminMenuPermissionsOnlyExistingAdmins(t *testing.T) {
	db := setupAdminMenuPermissionModelTestDB(t)
	require.NoError(t, db.Create(&User{Id: 1, Username: "common", Password: "password123", Role: common.RoleCommonUser, AffCode: "common"}).Error)
	require.NoError(t, db.Create(&User{Id: 2, Username: "admin", Password: "password123", Role: common.RoleAdminUser, AffCode: "admin"}).Error)
	require.NoError(t, db.Create(&User{Id: 3, Username: "root", Password: "password123", Role: common.RoleRootUser, AffCode: "root"}).Error)

	require.NoError(t, BackfillAdminMenuPermissionsForExistingAdmins())

	adminKeys, err := GetAdminMenuPermissionKeys(2, common.RoleAdminUser)
	require.NoError(t, err)
	require.ElementsMatch(t, DefaultAdminMenuPermissionKeys(), adminKeys)

	var commonCount int64
	require.NoError(t, db.Model(&AdminMenuPermission{}).Where("user_id = ?", 1).Count(&commonCount).Error)
	require.Zero(t, commonCount)

	var rootCount int64
	require.NoError(t, db.Model(&AdminMenuPermission{}).Where("user_id = ?", 3).Count(&rootCount).Error)
	require.Zero(t, rootCount)
}
