package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
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
	originalSmartStrategyEnabled := setting.AggregateGroupSmartStrategyEnabled
	originalFailureThreshold := setting.AggregateGroupFailureThreshold
	originalDegradeDurationSeconds := setting.AggregateGroupDegradeDurationSeconds
	originalSlowRequestThreshold := setting.AggregateGroupSlowRequestThreshold
	originalConsecutiveSlowLimit := setting.AggregateGroupConsecutiveSlowLimit

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.AggregateGroup{}, &model.AggregateGroupTarget{}, &model.Channel{}, &model.Ability{}, &model.Option{}))

	t.Cleanup(func() {
		setting.AggregateGroupSmartStrategyEnabled = originalSmartStrategyEnabled
		setting.AggregateGroupFailureThreshold = originalFailureThreshold
		setting.AggregateGroupDegradeDurationSeconds = originalDegradeDurationSeconds
		setting.AggregateGroupSlowRequestThreshold = originalSlowRequestThreshold
		setting.AggregateGroupConsecutiveSlowLimit = originalConsecutiveSlowLimit
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
		"smart_routing_enabled":true,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"retry_status_codes":"401,429,500-599",
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
	require.Contains(t, string(listResp.Data), `"retry_status_codes":"401,429,500-599"`)
	require.Contains(t, string(listResp.Data), `"smart_routing_enabled":true`)
}

func TestCreateAggregateGroupRejectsInvalidRetryStatusCodes(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"description":"for enterprise",
		"status":1,
		"group_ratio":1.5,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"retry_status_codes":"401,abc,500-599",
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default"},{"real_group":"vip"}]
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "重试状态码规则无效")
}

func TestGetChannelRetryDoesNotReuseInitialSelectedChannelForAggregateGroup(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	group := &model.AggregateGroup{
		Name:                    "ha-route",
		DisplayName:             "HA Route",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 10,
		RetryStatusCodes:        "400-599",
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "kiro2", OrderIndex: 0},
		{RealGroup: "kiro1", OrderIndex: 1},
	}))

	weight := uint(10)
	priority := int64(0)
	channel1 := &model.Channel{
		Id:          5,
		Name:        "pp_kiro1",
		Key:         "sk-test-1",
		Status:      common.ChannelStatusEnabled,
		Group:       "kiro1",
		Models:      "claude-haiku-4-5",
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: time.Now().Unix(),
	}
	channel2 := &model.Channel{
		Id:          6,
		Name:        "doge_kiro2",
		Key:         "sk-test-2",
		Status:      common.ChannelStatusEnabled,
		Group:       "kiro2",
		Models:      "claude-haiku-4-5",
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(channel1).Error)
	require.NoError(t, model.DB.Create(channel2).Error)
	require.NoError(t, channel1.AddAbilities(nil))
	require.NoError(t, channel2.AddAbilities(nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "ha-route")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "kiro2")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 1)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 6)
	common.SetContextKey(ctx, constant.ContextKeyChannelName, "doge_kiro2")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, 14)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "claude-haiku-4-5")

	retryParam := &service.RetryParam{
		Ctx:        ctx,
		TokenGroup: "ha-route",
		ModelName:  "claude-haiku-4-5",
		Retry:      common.GetPointer(1),
	}
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "claude-haiku-4-5",
	}

	channel, apiErr := getChannel(ctx, relayInfo, retryParam)
	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, 5, channel.Id)
	require.Equal(t, "kiro1", common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup))
}
