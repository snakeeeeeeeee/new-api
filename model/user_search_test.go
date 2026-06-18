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

func setupUserSearchModelTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&User{}))

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

func seedUserSearchUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []User{
		{Id: 1, Username: "common", Password: "password123", DisplayName: "Common User", Email: "common@example.com", Role: common.RoleCommonUser, Group: "default", AffCode: "common"},
		{Id: 2, Username: "admin-default", Password: "password123", DisplayName: "Default Admin", Email: "admin-default@example.com", Role: common.RoleAdminUser, Group: "default", AffCode: "admin-default"},
		{Id: 3, Username: "admin-vip", Password: "password123", DisplayName: "Vip Admin", Email: "admin-vip@example.com", Role: common.RoleAdminUser, Group: "vip", AffCode: "admin-vip"},
		{Id: 4, Username: "root", Password: "password123", DisplayName: "Root User", Email: "root@example.com", Role: common.RoleRootUser, Group: "default", AffCode: "root"},
	}
	for i := range users {
		require.NoError(t, db.Create(&users[i]).Error)
	}
}

func TestSearchUsersFiltersByRoleOnly(t *testing.T) {
	db := setupUserSearchModelTestDB(t)
	seedUserSearchUsers(t, db)

	role := common.RoleAdminUser
	users, total, err := SearchUsers("", "", &role, 0, 20)

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, users, 2)
	for _, user := range users {
		require.Equal(t, common.RoleAdminUser, user.Role)
		require.Empty(t, user.Password)
	}
}

func TestSearchUsersCombinesKeywordGroupAndRole(t *testing.T) {
	db := setupUserSearchModelTestDB(t)
	seedUserSearchUsers(t, db)

	role := common.RoleAdminUser
	users, total, err := SearchUsers("admin", "vip", &role, 0, 20)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	require.Equal(t, "admin-vip", users[0].Username)
	require.Equal(t, "vip", users[0].Group)
	require.Equal(t, common.RoleAdminUser, users[0].Role)
}
