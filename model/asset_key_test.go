package model

import (
	"bytes"
	"log"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestGetAssetKeyByKeyQuotesKeyColumnForMySQL(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalCommonGroupCol := commonGroupCol
	originalCommonKeyCol := commonKeyCol
	originalCommonTrueVal := commonTrueVal
	originalCommonFalseVal := commonFalseVal
	originalLogKeyCol := logKeyCol
	originalLogGroupCol := logGroupCol

	common.UsingSQLite = false
	common.UsingMySQL = true
	common.UsingPostgreSQL = false
	initCol()

	var queryLog bytes.Buffer
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(127.0.0.1:3306)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger: gormlogger.New(log.New(&queryLog, "", 0), gormlogger.Config{
			LogLevel: gormlogger.Info,
		}),
	})
	require.NoError(t, err)
	DB = db

	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		commonGroupCol = originalCommonGroupCol
		commonKeyCol = originalCommonKeyCol
		commonTrueVal = originalCommonTrueVal
		commonFalseVal = originalCommonFalseVal
		logKeyCol = originalLogKeyCol
		logGroupCol = originalLogGroupCol
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	_, err = GetAssetKeyByKey("ak_mysql")

	require.NoError(t, err)
	require.Contains(t, queryLog.String(), "WHERE `key` = 'ak_mysql'")
}
