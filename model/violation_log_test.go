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

func setupViolationModelTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&User{}, &ViolationLog{}))

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestViolationLogQueryCountAndDelete(t *testing.T) {
	setupViolationModelTestDB(t)

	require.NoError(t, InsertViolationLog(&ViolationLog{
		CreatedAt:      100,
		UserId:         10,
		Username:       "risk-user",
		TokenId:        20,
		TokenName:      "risk-token",
		ModelName:      "gpt-risk",
		UsingGroup:     "default",
		AggregateGroup: "ag-risk",
		RouteGroup:     "route-a",
		RequestId:      "req-1",
		MatchedWords:   `["reverse"]`,
		Action:         "block",
	}))
	require.NoError(t, InsertViolationLog(&ViolationLog{
		CreatedAt:      200,
		UserId:         10,
		Username:       "risk-user",
		TokenId:        21,
		TokenName:      "safe-token",
		ModelName:      "gpt-risk",
		UsingGroup:     "vip",
		AggregateGroup: "ag-risk",
		RouteGroup:     "route-b",
		RequestId:      "req-2",
		MatchedWords:   `["jailbreak"]`,
		Action:         "log_only",
	}))

	count, err := CountViolationLogsByUserID(10)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	logs, total, err := GetViolationLogs(ViolationLogQuery{
		Username:       "risk-user",
		AggregateGroup: "ag-risk",
		MatchedWord:    "reverse",
	}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, "req-1", logs[0].RequestId)

	deleted, err := DeleteViolationLogsBefore(150, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	count, err = CountViolationLogsByUserID(10)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestDisableUserByViolationIsIdempotentAndUpdatesStatus(t *testing.T) {
	db := setupViolationModelTestDB(t)
	require.NoError(t, db.Create(&User{
		Id:       88,
		Username: "ban-risk-user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	banned, err := DisableUserByViolation(88)
	require.NoError(t, err)
	require.True(t, banned)

	user, err := GetUserById(88, false)
	require.NoError(t, err)
	require.Equal(t, common.UserStatusDisabled, user.Status)

	banned, err = DisableUserByViolation(88)
	require.NoError(t, err)
	require.False(t, banned)
}
