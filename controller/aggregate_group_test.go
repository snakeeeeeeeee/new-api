package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func seedAggregateGroupControllerAbilityChannel(t *testing.T, id int, group string, modelName string, priority int64) {
	t.Helper()
	weight := uint(10)
	channel := &model.Channel{
		Id:          id,
		Name:        fmt.Sprintf("%s-channel-%d", group, id),
		Key:         "sk-test",
		Status:      common.ChannelStatusEnabled,
		Group:       group,
		Models:      modelName,
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
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

func TestGetAggregateGroupRuntimeDefaultsToSortedModelAndReturnsRouteState(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	setting.AggregateGroupSmartStrategyEnabled = true

	group := &model.AggregateGroup{
		Name:                    "runtime-ha",
		DisplayName:             "Runtime HA",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		SmartRoutingEnabled:     true,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0},
		{RealGroup: "vip", OrderIndex: 1},
	}))

	seedAggregateGroupControllerAbilityChannel(t, 1001, "default", "z-model", 0)
	seedAggregateGroupControllerAbilityChannel(t, 1002, "default", "a-model", 0)
	seedAggregateGroupControllerAbilityChannel(t, 1003, "vip", "a-model", 0)

	now := time.Now().Unix()
	require.NoError(t, service.SetAggregateGroupRuntimeState(group.Name, "a-model", &service.AggregateGroupRuntimeState{
		ActiveIndex:   1,
		ActiveGroup:   "vip",
		LastSuccessAt: now,
		LastSwitchAt:  now,
	}))
	require.NoError(t, service.SetAggregateGroupRouteStrategyState(group.Name, "a-model", "default", &service.AggregateGroupRouteStrategyState{
		DegradedUntil:     common.GetTimestamp() + 600,
		LastFailureAt:     now,
		LastTriggerReason: service.AggregateSmartTriggerReasonConsecutiveFailures,
		LastTriggerAt:     now,
	}))
	require.NoError(t, service.SetAggregateGroupRouteStrategyState(group.Name, "a-model", "vip", &service.AggregateGroupRouteStrategyState{
		ConsecutiveSlows: 1,
		LastSuccessAt:    now,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(group.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/aggregate_group/%d/runtime", group.Id), nil)
	GetAggregateGroupRuntime(ctx)

	var resp struct {
		Success bool                          `json:"success"`
		Message string                        `json:"message"`
		Data    aggregateGroupRuntimeResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, []string{"a-model", "z-model"}, resp.Data.Models)
	require.Equal(t, "a-model", resp.Data.SelectedModel)
	require.True(t, resp.Data.SmartStrategy.EffectiveEnabled)
	require.NotNil(t, resp.Data.Runtime)
	require.NotNil(t, resp.Data.Runtime.ActiveRoute)
	require.Equal(t, "vip", resp.Data.Runtime.ActiveRoute.ActiveGroup)
	require.Len(t, resp.Data.Runtime.Routes, 2)
	require.Equal(t, "default", resp.Data.Runtime.Routes[0].RouteGroup)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].PriorityCount)
	require.True(t, resp.Data.Runtime.Routes[0].IsDegraded)
	require.Equal(t, service.AggregateSmartTriggerReasonConsecutiveFailures, resp.Data.Runtime.Routes[0].LastTriggerReason)
	require.Equal(t, "vip", resp.Data.Runtime.Routes[1].RouteGroup)
	require.Equal(t, 1, resp.Data.Runtime.Routes[1].PriorityCount)
	require.True(t, resp.Data.Runtime.Routes[1].IsActive)
	require.Equal(t, 1, resp.Data.Runtime.Routes[1].ConsecutiveSlows)
}

func TestGetAggregateGroupRuntimeRejectsModelOutsideAggregateGroup(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	group := &model.AggregateGroup{
		Name:                    "runtime-invalid-model",
		DisplayName:             "Runtime Invalid Model",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0},
	}))
	seedAggregateGroupControllerAbilityChannel(t, 1001, "default", "a-model", 0)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(group.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/aggregate_group/%d/runtime?model=b-model", group.Id), nil)
	GetAggregateGroupRuntime(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "模型不属于当前聚合分组")
}

func TestGetAggregateGroupRuntimeHandlesAggregateGroupWithoutModels(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	group := &model.AggregateGroup{
		Name:                    "runtime-empty",
		DisplayName:             "Runtime Empty",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0},
		{RealGroup: "vip", OrderIndex: 1},
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(group.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/aggregate_group/%d/runtime", group.Id), nil)
	GetAggregateGroupRuntime(ctx)

	var resp struct {
		Success bool                          `json:"success"`
		Message string                        `json:"message"`
		Data    aggregateGroupRuntimeResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Empty(t, resp.Data.Models)
	require.Empty(t, resp.Data.SelectedModel)
	require.Nil(t, resp.Data.Runtime)
}

func TestGetAggregateGroupRuntimeMarksRouteUnavailableWhenSubGroupDoesNotSupportModel(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	group := &model.AggregateGroup{
		Name:                    "runtime-partial-model-support",
		DisplayName:             "Runtime Partial Model Support",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0},
		{RealGroup: "vip", OrderIndex: 1},
	}))

	seedAggregateGroupControllerAbilityChannel(t, 1001, "default", "claude-haiku-4-5", 0)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(group.Id)}}
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/aggregate_group/%d/runtime?model=claude-haiku-4-5", group.Id),
		nil,
	)
	GetAggregateGroupRuntime(ctx)

	var resp struct {
		Success bool                          `json:"success"`
		Message string                        `json:"message"`
		Data    aggregateGroupRuntimeResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, []string{"claude-haiku-4-5"}, resp.Data.Models)
	require.Equal(t, "claude-haiku-4-5", resp.Data.SelectedModel)
	require.NotNil(t, resp.Data.Runtime)
	require.Len(t, resp.Data.Runtime.Routes, 2)
	require.Equal(t, "default", resp.Data.Runtime.Routes[0].RouteGroup)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].PriorityCount)
	require.Equal(t, "vip", resp.Data.Runtime.Routes[1].RouteGroup)
	require.Equal(t, 0, resp.Data.Runtime.Routes[1].PriorityCount)
	require.False(t, resp.Data.Runtime.Routes[1].IsDegraded)
	require.False(t, resp.Data.Runtime.Routes[1].IsActive)
}
