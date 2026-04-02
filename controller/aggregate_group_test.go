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

func setupAggregateGroupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.AggregateGroup{}, &model.AggregateGroupTarget{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestCreateAggregateGroupAndList(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"description":"for enterprise",
		"status":1,
		"group_ratio":1.5,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default"},{"real_group":"vip"}]
	}`)

	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	createCtx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(createCtx)

	var createResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(createRecorder.Body.Bytes(), &createResp))
	require.True(t, createResp.Success, createResp.Message)

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/aggregate_group", nil)
	GetAggregateGroups(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), "enterprise-stable")
	require.Contains(t, string(listResp.Data), `"real_group":"default"`)
}
