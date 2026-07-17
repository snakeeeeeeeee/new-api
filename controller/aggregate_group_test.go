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
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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
	originalClusterDegradedWeightPercent := setting.AggregateGroupClusterDegradedWeightPct
	originalSlowRequestThreshold := setting.AggregateGroupSlowRequestThreshold
	originalSlowFirstResponseThreshold := setting.AggregateGroupSlowFirstResponseThreshold
	originalConsecutiveSlowLimit := setting.AggregateGroupConsecutiveSlowLimit
	originalFailureRateWindowSeconds := setting.AggregateGroupFailureRateWindowSeconds
	originalFailureRateMinRequests := setting.AggregateGroupFailureRateMinRequests
	originalFailureRateThresholdPct := setting.AggregateGroupFailureRateThresholdPct
	originalSlowRateWindowSeconds := setting.AggregateGroupSlowRateWindowSeconds
	originalSlowRateMinRequests := setting.AggregateGroupSlowRateMinRequests
	originalSlowRateThresholdPct := setting.AggregateGroupSlowRateThresholdPct

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AggregateGroupCategory{}, &model.AggregateGroup{}, &model.AggregateGroupTarget{}, &model.AggregateGroupRouteModelRatio{}, &model.Channel{}, &model.Ability{}, &model.Option{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		setting.AggregateGroupSmartStrategyEnabled = originalSmartStrategyEnabled
		setting.AggregateGroupFailureThreshold = originalFailureThreshold
		setting.AggregateGroupDegradeDurationSeconds = originalDegradeDurationSeconds
		setting.AggregateGroupClusterDegradedWeightPct = originalClusterDegradedWeightPercent
		setting.AggregateGroupSlowRequestThreshold = originalSlowRequestThreshold
		setting.AggregateGroupSlowFirstResponseThreshold = originalSlowFirstResponseThreshold
		setting.AggregateGroupConsecutiveSlowLimit = originalConsecutiveSlowLimit
		setting.AggregateGroupFailureRateWindowSeconds = originalFailureRateWindowSeconds
		setting.AggregateGroupFailureRateMinRequests = originalFailureRateMinRequests
		setting.AggregateGroupFailureRateThresholdPct = originalFailureRateThresholdPct
		setting.AggregateGroupSlowRateWindowSeconds = originalSlowRateWindowSeconds
		setting.AggregateGroupSlowRateMinRequests = originalSlowRateMinRequests
		setting.AggregateGroupSlowRateThresholdPct = originalSlowRateThresholdPct
		service.ClearAggregateRouteAffinityCacheAll()
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

func seedAggregateGroupControllerUser(t *testing.T, db *gorm.DB, id int, username string, group string, role int, setting dto.UserSetting) *model.User {
	t.Helper()
	user := &model.User{
		Id:          id,
		Username:    username,
		Password:    "password123",
		DisplayName: username,
		Status:      common.UserStatusEnabled,
		Role:        role,
		Group:       group,
	}
	user.SetSetting(setting)
	require.NoError(t, db.Create(user).Error)
	return user
}

func decodeAggregateGroupAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestCreateAggregateGroupAndList(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"description":"for enterprise",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"smart_routing_enabled":true,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"cluster_affinity_ttl_seconds":120,
		"route_affinity_strategy":"request_only",
		"route_affinity_scope":"model",
		"route_affinity_key_sources":[
			{"type":"header","key":"X-Aggregate-Affinity-Key"},
			{"type":"gjson","path":"metadata.user_id"}
		],
		"retry_status_codes":"401,429,500-599",
		"smart_strategy_config":{
			"failure_rate_window_seconds":120,
			"failure_rate_min_requests":50,
			"failure_rate_threshold_percent":8,
			"slow_rate_window_seconds":180,
			"slow_rate_min_requests":40,
			"slow_rate_threshold_percent":25,
			"degrade_duration_seconds":300,
			"cluster_degraded_weight_percent":35,
			"slow_request_threshold_seconds":20,
			"slow_first_response_threshold_seconds":1
		},
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":50,"rpm_limit":0},{"real_group":"vip","weight":150,"rpm_limit":80}],
		"client_route_pools":{
			"enabled":true,
			"claude_code_cli":{
				"enabled":true,
				"fallback_to_default":false,
				"targets":[{"real_group":"vip","weight":250,"rpm_limit":80}]
			}
		}
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
	require.Contains(t, string(listResp.Data), `"routing_mode":"cluster"`)
	require.Contains(t, string(listResp.Data), `"cluster_affinity_ttl_seconds":120`)
	require.Contains(t, string(listResp.Data), `"route_affinity_strategy":"request_only"`)
	require.Contains(t, string(listResp.Data), `"route_affinity_scope":"model"`)
	require.Contains(t, string(listResp.Data), `"route_affinity_key_sources"`)
	require.Contains(t, string(listResp.Data), `"X-Aggregate-Affinity-Key"`)
	require.Contains(t, string(listResp.Data), `"weight":150`)
	require.Contains(t, string(listResp.Data), `"rpm_limit":80`)
	require.Contains(t, string(listResp.Data), `"client_route_pools"`)
	require.Contains(t, string(listResp.Data), `"fallback_to_default":false`)
	require.Contains(t, string(listResp.Data), `"weight":250`)
	require.Contains(t, string(listResp.Data), `"smart_strategy_config"`)
	require.Contains(t, string(listResp.Data), `"failure_rate_threshold_percent":8`)
	require.Contains(t, string(listResp.Data), `"cluster_degraded_weight_percent":35`)
}

func TestAggregateGroupCategoryAPIsAndUserGroupMetadata(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	seedAggregateGroupControllerUser(t, db, 501, "category-user", "vip", common.RoleCommonUser, dto.UserSetting{})

	createCategoryRecorder := httptest.NewRecorder()
	createCategoryCtx, _ := gin.CreateTestContext(createCategoryRecorder)
	createCategoryCtx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group/categories", bytes.NewBufferString(`{"name":"生图"}`))
	createCategoryCtx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroupCategory(createCategoryCtx)
	createCategoryResp := decodeAggregateGroupAPIResponse(t, createCategoryRecorder)
	require.True(t, createCategoryResp.Success, createCategoryResp.Message)
	var category aggregateGroupCategoryResponse
	require.NoError(t, common.Unmarshal(createCategoryResp.Data, &category))
	require.NotZero(t, category.Id)

	group := &model.AggregateGroup{
		Name:        "category-image-ha",
		DisplayName: "图片高可用",
		Status:      model.AggregateGroupStatusEnabled,
		GroupRatio:  1,
		CategoryId:  category.Id,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{{RealGroup: "default"}}))

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/aggregate_group", nil)
	GetAggregateGroups(listCtx)
	listResp := decodeAggregateGroupAPIResponse(t, listRecorder)
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"category_name":"生图"`)

	userGroupsRecorder := httptest.NewRecorder()
	userGroupsCtx, _ := gin.CreateTestContext(userGroupsRecorder)
	userGroupsCtx.Set("id", 501)
	userGroupsCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	GetUserGroups(userGroupsCtx)
	userGroupsResp := decodeAggregateGroupAPIResponse(t, userGroupsRecorder)
	require.True(t, userGroupsResp.Success, userGroupsResp.Message)
	require.Contains(t, string(userGroupsResp.Data), `"category_name":"生图"`)
	require.Contains(t, string(userGroupsResp.Data), `"type":"aggregate"`)

	updatePayload := fmt.Sprintf(`{
		"id":%d,
		"name":"category-image-ha",
		"display_name":"图片高可用更新",
		"status":1,
		"group_ratio":1,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default"}]
	}`, group.Id)
	updateRecorder := httptest.NewRecorder()
	updateCtx, _ := gin.CreateTestContext(updateRecorder)
	updateCtx.Request = httptest.NewRequest(http.MethodPut, "/api/aggregate_group", bytes.NewBufferString(updatePayload))
	updateCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateAggregateGroup(updateCtx)
	updateResp := decodeAggregateGroupAPIResponse(t, updateRecorder)
	require.True(t, updateResp.Success, updateResp.Message)
	var updated aggregateGroupResponse
	require.NoError(t, common.Unmarshal(updateResp.Data, &updated))
	require.Equal(t, category.Id, updated.CategoryId)
}

func TestAggregateGroupRouteModelRatioAPIPreservePruneAndClear(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	createPayload := []byte(`{
		"name":"route-model-ratio-api",
		"display_name":"Route Model Ratio API",
		"status":1,
		"group_ratio":1.25,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100},{"real_group":"vip","weight":100}],
		"route_model_group_ratio_overrides":[
			{"real_group":"default","model_name":"gpt-route-a","group_ratio":2},
			{"real_group":"vip","model_name":"gpt-route-b","group_ratio":0,"enabled":false}
		]
	}`)
	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(createPayload))
	createCtx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(createCtx)

	createResp := decodeAggregateGroupAPIResponse(t, createRecorder)
	require.True(t, createResp.Success, createResp.Message)
	var created aggregateGroupResponse
	require.NoError(t, common.Unmarshal(createResp.Data, &created))
	require.NotZero(t, created.Id)
	require.Len(t, created.RouteModelGroupRatioOverrides, 2)
	require.Equal(t, 1, created.EnabledRouteModelGroupRatioOverrideCount)
	require.True(t, created.RouteModelGroupRatioOverrides[0].Enabled)
	require.Zero(t, created.RouteModelGroupRatioOverrides[1].GroupRatio)

	omitRulesPayload := []byte(fmt.Sprintf(`{
		"id":%d,
		"name":"route-model-ratio-api",
		"display_name":"Route Model Ratio API Updated",
		"status":1,
		"group_ratio":1.25,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}]
	}`, created.Id))
	omitRecorder := httptest.NewRecorder()
	omitCtx, _ := gin.CreateTestContext(omitRecorder)
	omitCtx.Request = httptest.NewRequest(http.MethodPut, "/api/aggregate_group", bytes.NewReader(omitRulesPayload))
	omitCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateAggregateGroup(omitCtx)

	omitResp := decodeAggregateGroupAPIResponse(t, omitRecorder)
	require.True(t, omitResp.Success, omitResp.Message)
	var afterOmit aggregateGroupResponse
	require.NoError(t, common.Unmarshal(omitResp.Data, &afterOmit))
	require.Len(t, afterOmit.RouteModelGroupRatioOverrides, 1)
	require.Equal(t, "default", afterOmit.RouteModelGroupRatioOverrides[0].RealGroup)
	require.Equal(t, "gpt-route-a", afterOmit.RouteModelGroupRatioOverrides[0].ModelName)

	clearRulesPayload := []byte(fmt.Sprintf(`{
		"id":%d,
		"name":"route-model-ratio-api",
		"display_name":"Route Model Ratio API Updated",
		"status":1,
		"group_ratio":1.25,
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}],
		"route_model_group_ratio_overrides":[]
	}`, created.Id))
	clearRecorder := httptest.NewRecorder()
	clearCtx, _ := gin.CreateTestContext(clearRecorder)
	clearCtx.Request = httptest.NewRequest(http.MethodPut, "/api/aggregate_group", bytes.NewReader(clearRulesPayload))
	clearCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateAggregateGroup(clearCtx)

	clearResp := decodeAggregateGroupAPIResponse(t, clearRecorder)
	require.True(t, clearResp.Success, clearResp.Message)
	var afterClear aggregateGroupResponse
	require.NoError(t, common.Unmarshal(clearResp.Data, &afterClear))
	require.Empty(t, afterClear.RouteModelGroupRatioOverrides)
	require.Zero(t, afterClear.EnabledRouteModelGroupRatioOverrideCount)
}

func TestCreateAggregateGroupRejectsInvalidRouteModelRatios(t *testing.T) {
	tests := []struct {
		name    string
		rules   string
		message string
	}{
		{
			name: "duplicate",
			rules: `[
				{"real_group":"default","model_name":"same-model","group_ratio":2},
				{"real_group":"default","model_name":"same-model","group_ratio":3}
			]`,
			message: "重复配置",
		},
		{
			name:    "negative ratio",
			rules:   `[{"real_group":"default","model_name":"bad-model","group_ratio":-1}]`,
			message: "有限数值",
		},
		{
			name:    "unknown child group",
			rules:   `[{"real_group":"missing","model_name":"bad-model","group_ratio":2}]`,
			message: "未配置的真实分组",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAggregateGroupControllerTestDB(t)
			payload := []byte(fmt.Sprintf(`{
				"name":"invalid-route-ratio-%s",
				"display_name":"Invalid Route Ratio",
				"status":1,
				"group_ratio":1,
				"recovery_enabled":true,
				"recovery_interval_seconds":300,
				"visible_user_groups":["vip"],
				"targets":[{"real_group":"default","weight":100}],
				"route_model_group_ratio_overrides":%s
			}`, strings.ReplaceAll(tt.name, " ", "-"), tt.rules))
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
			ctx.Request.Header.Set("Content-Type", "application/json")
			CreateAggregateGroup(ctx)

			resp := decodeAggregateGroupAPIResponse(t, recorder)
			require.False(t, resp.Success)
			require.Contains(t, resp.Message, tt.message)
			var count int64
			require.NoError(t, model.DB.Model(&model.AggregateGroup{}).Where("name = ?", "invalid-route-ratio-"+strings.ReplaceAll(tt.name, " ", "-")).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestGetAggregateGroupTargetModelsUsesEnabledChannelConfiguration(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	priority := int64(0)
	weight := uint(10)
	channels := []model.Channel{
		{
			Id: 2301, Name: "enabled-multi-group", Key: "sk-test",
			Status: common.ChannelStatusEnabled, Group: "default, vip",
			Models: "z-model, a-model", Priority: &priority, Weight: &weight,
			CreatedTime: time.Now().Unix(),
		},
		{
			Id: 2302, Name: "enabled-duplicate-model", Key: "sk-test",
			Status: common.ChannelStatusEnabled, Group: "default",
			Models: "a-model", Priority: &priority, Weight: &weight,
			CreatedTime: time.Now().Unix(),
		},
		{
			Id: 2303, Name: "disabled-channel", Key: "sk-test",
			Status: common.ChannelStatusManuallyDisabled, Group: "default",
			Models: "disabled-model", Priority: &priority, Weight: &weight,
			CreatedTime: time.Now().Unix(),
		},
		{
			Id: 2304, Name: "similar-group", Key: "sk-test",
			Status: common.ChannelStatusEnabled, Group: "default-extra",
			Models: "wrong-group-model", Priority: &priority, Weight: &weight,
			CreatedTime: time.Now().Unix(),
		},
	}
	require.NoError(t, model.DB.Create(&channels).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/aggregate_group/models?group=default", nil)
	GetAggregateGroupTargetModels(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)
	var models []string
	require.NoError(t, common.Unmarshal(resp.Data, &models))
	require.Equal(t, []string{"a-model", "z-model"}, models)
}

func TestCreateAggregateGroupRejectsInvalidRouteAffinityScope(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"route_affinity_scope":"invalid",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}]
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "亲和范围")
}

func TestUpdateUserAggregateGroupRatioOverridesPreservesUserSetting(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)

	stable := &model.AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "Enterprise Stable",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.5,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, stable.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, stable.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	fast := &model.AggregateGroup{
		Name:                    "enterprise-fast",
		DisplayName:             "Enterprise Fast",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, fast.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, fast.InsertWithTargets(service.NormalizeAggregateTargets([]string{"vip"})))

	seedAggregateGroupControllerUser(t, db, 41, "override_user", "vip", common.RoleCommonUser, dto.UserSetting{
		NotifyType:            dto.NotifyTypeWebhook,
		QuotaWarningThreshold: 0.25,
		WebhookUrl:            "https://example.com/hook",
		WebhookSecret:         "secret",
		SidebarModules:        `{"dashboard":true}`,
		BillingPreference:     "wallet_first",
		Language:              "en",
		ExtraUsableGroups:     []string{"enterprise-extra"},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/user/41/aggregate_group_ratio_overrides", bytes.NewReader([]byte(`{
		"overrides":{"enterprise-stable":0.1,"enterprise-fast":0}
	}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "41"}}
	ctx.Set("role", common.RoleRootUser)

	UpdateUserAggregateGroupRatioOverrides(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)

	updated, err := model.GetUserById(41, true)
	require.NoError(t, err)
	setting := updated.GetSetting()
	require.Equal(t, dto.NotifyTypeWebhook, setting.NotifyType)
	require.Equal(t, 0.25, setting.QuotaWarningThreshold)
	require.Equal(t, "https://example.com/hook", setting.WebhookUrl)
	require.Equal(t, "secret", setting.WebhookSecret)
	require.Equal(t, `{"dashboard":true}`, setting.SidebarModules)
	require.Equal(t, "wallet_first", setting.BillingPreference)
	require.Equal(t, "en", setting.Language)
	require.Equal(t, []string{"enterprise-extra"}, setting.ExtraUsableGroups)
	require.Equal(t, map[string]float64{
		"enterprise-stable": 0.1,
		"enterprise-fast":   0,
	}, setting.AggregateGroupRatioOverrides)

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/41/aggregate_group_ratio_overrides", nil)
	getCtx.Params = gin.Params{{Key: "id", Value: "41"}}
	getCtx.Set("role", common.RoleRootUser)
	GetUserAggregateGroupRatioOverrides(getCtx)

	getResp := decodeAggregateGroupAPIResponse(t, getRecorder)
	require.True(t, getResp.Success, getResp.Message)
	var data struct {
		Overrides       map[string]float64       `json:"overrides"`
		AggregateGroups []aggregateGroupResponse `json:"aggregate_groups"`
	}
	require.NoError(t, common.Unmarshal(getResp.Data, &data))
	require.Equal(t, 0.1, data.Overrides["enterprise-stable"])
	require.Equal(t, 0.0, data.Overrides["enterprise-fast"])
	groupNames := make([]string, 0, len(data.AggregateGroups))
	for _, group := range data.AggregateGroups {
		groupNames = append(groupNames, group.Name)
	}
	require.ElementsMatch(t, []string{"enterprise-stable", "enterprise-fast"}, groupNames)
}

func TestUserAggregateGroupRatioOverridesIncludesExtraAuthorizedAggregateGroups(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"business-real":1}`))

	visible := &model.AggregateGroup{
		Name:                    "visible-aggregate",
		DisplayName:             "Visible Aggregate",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.25,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, visible.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, visible.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	extra := &model.AggregateGroup{
		Name:                    "extra-aggregate",
		DisplayName:             "Extra Aggregate",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, extra.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, extra.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	disabled := &model.AggregateGroup{
		Name:                    "disabled-extra-aggregate",
		DisplayName:             "Disabled Extra Aggregate",
		Status:                  model.AggregateGroupStatusDisabled,
		GroupRatio:              3,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, disabled.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, disabled.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	seedAggregateGroupControllerUser(t, db, 44, "extra_ratio_user", "vip", common.RoleCommonUser, dto.UserSetting{
		NotifyType:            dto.NotifyTypeWebhook,
		QuotaWarningThreshold: 0.5,
		WebhookUrl:            "https://example.com/extra",
		SidebarModules:        `{"setting":true}`,
		BillingPreference:     "subscription_first",
		ExtraUsableGroups: []string{
			"extra-aggregate",
			"business-real",
			"disabled-extra-aggregate",
			"missing-group",
			"default",
			"UserGroup-demo",
			"auto",
		},
	})

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/44/extra_usable_groups", nil)
	getCtx.Params = gin.Params{{Key: "id", Value: "44"}}
	getCtx.Set("role", common.RoleRootUser)
	GetUserExtraUsableGroups(getCtx)

	getResp := decodeAggregateGroupAPIResponse(t, getRecorder)
	require.True(t, getResp.Success, getResp.Message)
	var data struct {
		ExtraUsableGroups    []string                  `json:"extra_usable_groups"`
		BusinessGroupOptions []userBusinessGroupOption `json:"business_group_options"`
	}
	require.NoError(t, common.Unmarshal(getResp.Data, &data))
	require.Contains(t, data.ExtraUsableGroups, "extra-aggregate")
	require.Contains(t, data.ExtraUsableGroups, "business-real")
	optionTypes := make(map[string]string)
	for _, option := range data.BusinessGroupOptions {
		optionTypes[option.Value] = option.GroupType
	}
	require.Equal(t, "real", optionTypes["business-real"])
	require.Equal(t, "aggregate", optionTypes["extra-aggregate"])
	require.NotContains(t, optionTypes, "default")
	require.NotContains(t, optionTypes, "UserGroup-demo")

	putRecorder := httptest.NewRecorder()
	putCtx, _ := gin.CreateTestContext(putRecorder)
	putCtx.Request = httptest.NewRequest(http.MethodPut, "/api/user/44/extra_usable_groups", bytes.NewReader([]byte(`{
		"extra_usable_groups":["business-real","extra-aggregate","extra-aggregate","default","UserGroup-demo","disabled-extra-aggregate","missing-group"]
	}`)))
	putCtx.Request.Header.Set("Content-Type", "application/json")
	putCtx.Params = gin.Params{{Key: "id", Value: "44"}}
	putCtx.Set("role", common.RoleRootUser)
	UpdateUserExtraUsableGroups(putCtx)

	putResp := decodeAggregateGroupAPIResponse(t, putRecorder)
	require.True(t, putResp.Success, putResp.Message)
	updated, err := model.GetUserById(44, true)
	require.NoError(t, err)
	setting := updated.GetSetting()
	require.Equal(t, dto.NotifyTypeWebhook, setting.NotifyType)
	require.Equal(t, 0.5, setting.QuotaWarningThreshold)
	require.Equal(t, "https://example.com/extra", setting.WebhookUrl)
	require.Equal(t, `{"setting":true}`, setting.SidebarModules)
	require.Equal(t, "subscription_first", setting.BillingPreference)
	require.ElementsMatch(t, []string{"business-real", "extra-aggregate"}, setting.ExtraUsableGroups)

	ratioGetRecorder := httptest.NewRecorder()
	ratioGetCtx, _ := gin.CreateTestContext(ratioGetRecorder)
	ratioGetCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/44/aggregate_group_ratio_overrides", nil)
	ratioGetCtx.Params = gin.Params{{Key: "id", Value: "44"}}
	ratioGetCtx.Set("role", common.RoleRootUser)
	GetUserAggregateGroupRatioOverrides(ratioGetCtx)

	ratioGetResp := decodeAggregateGroupAPIResponse(t, ratioGetRecorder)
	require.True(t, ratioGetResp.Success, ratioGetResp.Message)
	var ratioData struct {
		AggregateGroups []aggregateGroupResponse `json:"aggregate_groups"`
	}
	require.NoError(t, common.Unmarshal(ratioGetResp.Data, &ratioData))
	groupNames := make([]string, 0, len(ratioData.AggregateGroups))
	for _, group := range ratioData.AggregateGroups {
		groupNames = append(groupNames, group.Name)
	}
	require.ElementsMatch(t, []string{"visible-aggregate", "extra-aggregate"}, groupNames)

	ratioPutRecorder := httptest.NewRecorder()
	ratioPutCtx, _ := gin.CreateTestContext(ratioPutRecorder)
	ratioPutCtx.Request = httptest.NewRequest(http.MethodPut, "/api/user/44/aggregate_group_ratio_overrides", bytes.NewReader([]byte(`{
		"overrides":{"extra-aggregate":0.75}
	}`)))
	ratioPutCtx.Request.Header.Set("Content-Type", "application/json")
	ratioPutCtx.Params = gin.Params{{Key: "id", Value: "44"}}
	ratioPutCtx.Set("role", common.RoleRootUser)
	UpdateUserAggregateGroupRatioOverrides(ratioPutCtx)

	ratioPutResp := decodeAggregateGroupAPIResponse(t, ratioPutRecorder)
	require.True(t, ratioPutResp.Success, ratioPutResp.Message)
	updated, err = model.GetUserById(44, true)
	require.NoError(t, err)
	require.Equal(t, 0.75, updated.GetSetting().AggregateGroupRatioOverrides["extra-aggregate"])
}

func TestUserAggregateGroupRatioOverridesOnlyExposeVisibleGroups(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)

	visible := &model.AggregateGroup{
		Name:                    "visible-aggregate",
		DisplayName:             "Visible Aggregate",
		Description:             "visible to vip",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.25,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, visible.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, visible.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	hidden := &model.AggregateGroup{
		Name:                    "hidden-aggregate",
		DisplayName:             "Hidden Aggregate",
		Description:             "visible to svip",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, hidden.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, hidden.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	disabled := &model.AggregateGroup{
		Name:                    "disabled-aggregate",
		DisplayName:             "Disabled Aggregate",
		Status:                  model.AggregateGroupStatusDisabled,
		GroupRatio:              3,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, disabled.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, disabled.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	seedAggregateGroupControllerUser(t, db, 43, "visible_list_user", "vip", common.RoleCommonUser, dto.UserSetting{})

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/user/43/aggregate_group_ratio_overrides", nil)
	getCtx.Params = gin.Params{{Key: "id", Value: "43"}}
	getCtx.Set("role", common.RoleRootUser)
	GetUserAggregateGroupRatioOverrides(getCtx)

	getResp := decodeAggregateGroupAPIResponse(t, getRecorder)
	require.True(t, getResp.Success, getResp.Message)
	var data struct {
		AggregateGroups []aggregateGroupResponse `json:"aggregate_groups"`
	}
	require.NoError(t, common.Unmarshal(getResp.Data, &data))
	require.Len(t, data.AggregateGroups, 1)
	require.Equal(t, "visible-aggregate", data.AggregateGroups[0].Name)
	require.Equal(t, "visible to vip", data.AggregateGroups[0].Description)
	require.Equal(t, 1.25, data.AggregateGroups[0].GroupRatio)

	putRecorder := httptest.NewRecorder()
	putCtx, _ := gin.CreateTestContext(putRecorder)
	putCtx.Request = httptest.NewRequest(http.MethodPut, "/api/user/43/aggregate_group_ratio_overrides", bytes.NewReader([]byte(`{
		"overrides":{"hidden-aggregate":0.5}
	}`)))
	putCtx.Request.Header.Set("Content-Type", "application/json")
	putCtx.Params = gin.Params{{Key: "id", Value: "43"}}
	putCtx.Set("role", common.RoleRootUser)
	UpdateUserAggregateGroupRatioOverrides(putCtx)

	putResp := decodeAggregateGroupAPIResponse(t, putRecorder)
	require.False(t, putResp.Success)
	require.Contains(t, putResp.Message, "当前用户不可见")

	okRecorder := httptest.NewRecorder()
	okCtx, _ := gin.CreateTestContext(okRecorder)
	okCtx.Request = httptest.NewRequest(http.MethodPut, "/api/user/43/aggregate_group_ratio_overrides", bytes.NewReader([]byte(`{
		"overrides":{"visible-aggregate":0.5}
	}`)))
	okCtx.Request.Header.Set("Content-Type", "application/json")
	okCtx.Params = gin.Params{{Key: "id", Value: "43"}}
	okCtx.Set("role", common.RoleRootUser)
	UpdateUserAggregateGroupRatioOverrides(okCtx)

	okResp := decodeAggregateGroupAPIResponse(t, okRecorder)
	require.True(t, okResp.Success, okResp.Message)
	var okData struct {
		Overrides       map[string]float64       `json:"overrides"`
		AggregateGroups []aggregateGroupResponse `json:"aggregate_groups"`
	}
	require.NoError(t, common.Unmarshal(okResp.Data, &okData))
	require.Equal(t, 0.5, okData.Overrides["visible-aggregate"])
	require.Len(t, okData.AggregateGroups, 1)
	require.Equal(t, "visible-aggregate", okData.AggregateGroups[0].Name)
}

func TestGetUserGroupsReturnsAggregateRatioOverrideDetails(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
	})

	group := &model.AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "Enterprise Stable",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.5,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	seedAggregateGroupControllerUser(t, db, 42, "visible_override_user", "vip", common.RoleCommonUser, dto.UserSetting{
		AggregateGroupRatioOverrides: map[string]float64{
			"enterprise-stable": 0.1,
			"default":           0.2,
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	ctx.Set("id", 42)
	GetUserGroups(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)
	var groups map[string]struct {
		Ratio            float64  `json:"ratio"`
		OriginalRatio    float64  `json:"original_ratio"`
		RatioOverride    *float64 `json:"ratio_override"`
		HasRatioOverride bool     `json:"has_ratio_override"`
		Type             string   `json:"type"`
	}
	require.NoError(t, common.Unmarshal(resp.Data, &groups))

	require.Equal(t, "aggregate", groups["enterprise-stable"].Type)
	require.Equal(t, 0.1, groups["enterprise-stable"].Ratio)
	require.Equal(t, 1.5, groups["enterprise-stable"].OriginalRatio)
	require.True(t, groups["enterprise-stable"].HasRatioOverride)
	require.NotNil(t, groups["enterprise-stable"].RatioOverride)
	require.Equal(t, 0.1, *groups["enterprise-stable"].RatioOverride)

	require.Equal(t, "real", groups["default"].Type)
	require.Equal(t, 1.0, groups["default"].Ratio)
	require.False(t, groups["default"].HasRatioOverride)
	require.Nil(t, groups["default"].RatioOverride)
}

func TestGetUserGroupsReturnsExtraUsableGroups(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组","svip":"SVIP分组"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
	})

	group := &model.AggregateGroup{
		Name:                    "extra-aggregate",
		DisplayName:             "Extra Aggregate",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.5,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, group.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})))

	seedAggregateGroupControllerUser(t, db, 43, "extra_group_user", "vip", common.RoleCommonUser, dto.UserSetting{
		ExtraUsableGroups: []string{"svip", "extra-aggregate", "missing-group"},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	ctx.Set("id", 43)
	GetUserGroups(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)
	var groups map[string]struct {
		Type string `json:"type"`
		Desc string `json:"desc"`
	}
	require.NoError(t, common.Unmarshal(resp.Data, &groups))
	require.Equal(t, "real", groups["svip"].Type)
	require.Equal(t, "aggregate", groups["extra-aggregate"].Type)
	require.NotContains(t, groups, "missing-group")
}

func TestGetUserModelsIncludesExtraUsableGroups(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组","svip":"SVIP分组"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
	})
	seedAggregateGroupControllerAbilityChannel(t, 2001, "vip", "vip-model", 0)
	seedAggregateGroupControllerAbilityChannel(t, 2002, "svip", "svip-model", 0)
	seedAggregateGroupControllerUser(t, db, 44, "extra_model_user", "vip", common.RoleCommonUser, dto.UserSetting{
		ExtraUsableGroups: []string{"svip"},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/44/models", nil)
	ctx.Set("id", 44)
	GetUserModels(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)
	var models []string
	require.NoError(t, common.Unmarshal(resp.Data, &models))
	require.Contains(t, models, "vip-model")
	require.Contains(t, models, "svip-model")
}

func TestGetPricingMapsExtraAuthorizedAggregateGroupModels(t *testing.T) {
	db := setupAggregateGroupControllerTestDB(t)
	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"video-real":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		model.RefreshPricing()
	})

	seedAggregateGroupControllerAbilityChannel(t, 2003, "video-real", "grok-video-model", 0)
	group := &model.AggregateGroup{
		Name:                    "extra-video-aggregate",
		DisplayName:             "Extra Video Aggregate",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.5,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"svip"}))
	require.NoError(t, group.InsertWithTargets(service.NormalizeAggregateTargets([]string{"video-real"})))
	require.NoError(t, model.DB.Create(&model.AggregateGroupRouteModelRatio{
		AggregateGroupId: group.Id,
		RealGroup:        "video-real",
		ModelName:        "grok-video-model",
		GroupRatio:       3,
		Enabled:          true,
	}).Error)
	seedAggregateGroupControllerUser(t, db, 45, "extra_pricing_user", "vip", common.RoleCommonUser, dto.UserSetting{
		ExtraUsableGroups: []string{"extra-video-aggregate"},
		AggregateGroupRatioOverrides: map[string]float64{
			"extra-video-aggregate": 0.25,
		},
	})
	model.RefreshPricing()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 45)
	GetPricing(ctx)

	resp := decodeAggregateGroupAPIResponse(t, recorder)
	require.True(t, resp.Success, resp.Message)
	var data struct {
		UsableGroup            map[string]string                                 `json:"usable_group"`
		Data                   []model.Pricing                                   `json:"data"`
		ModelGroupRatioDetails map[string]map[string]service.ModelGroupRatioView `json:"model_group_ratio_details"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &data))
	require.Contains(t, data.UsableGroup, "extra-video-aggregate")
	detail := data.ModelGroupRatioDetails["grok-video-model"]["extra-video-aggregate"]
	require.True(t, detail.DynamicRoute)
	require.Equal(t, 0.25, detail.Ratio)
	require.Equal(t, 1.5, detail.OriginalRatio)
	require.Equal(t, 3.0, detail.MaxRatio)
	for _, pricing := range data.Data {
		if pricing.ModelName == "grok-video-model" {
			require.Contains(t, pricing.EnableGroup, "extra-video-aggregate")
			return
		}
	}
	require.Fail(t, "pricing model not found")
}

func TestCreateAggregateGroupRejectsInvalidSmartStrategyConfig(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}],
		"smart_strategy_config":{"failure_rate_threshold_percent":101}
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "错误率阈值")
}

func TestCreateAggregateGroupRejectsNegativeTargetWeight(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":-1}]
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "权重不能小于 0")
}

func TestCreateAggregateGroupRejectsNegativeTargetRPMLimit(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100,"rpm_limit":-1}]
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "RPM 上限不能小于 0")
}

func TestCreateAggregateGroupRejectsInvalidRouteAffinitySource(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"route_affinity_strategy":"request_only",
		"route_affinity_key_sources":[{"type":"header"}],
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}]
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "亲和来源 header 缺少 key")
}

func TestCreateAggregateGroupRejectsNegativeClientRoutePoolWeight(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"default","weight":100}],
		"client_route_pools":{
			"enabled":true,
			"claude_code_cli":{
				"enabled":true,
				"fallback_to_default":true,
				"targets":[{"real_group":"vip","weight":-1}]
			}
		}
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "权重不能小于 0")
}

func TestCreateAggregateGroupRejectsConflictingSharedRouteRPMLimits(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	payload := []byte(`{
		"name":"enterprise-stable",
		"display_name":"企业稳定组",
		"status":1,
		"group_ratio":1.5,
		"routing_mode":"cluster",
		"recovery_enabled":true,
		"recovery_interval_seconds":300,
		"visible_user_groups":["vip"],
		"targets":[{"real_group":"vip","weight":100,"rpm_limit":80}],
		"client_route_pools":{
			"enabled":true,
			"claude_code_cli":{
				"enabled":true,
				"fallback_to_default":true,
				"targets":[{"real_group":"vip","weight":100,"rpm_limit":0}]
			}
		}
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/aggregate_group", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	CreateAggregateGroup(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "RPM 上限必须一致")
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
		{RealGroup: "kiro2", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
		{RealGroup: "kiro1", OrderIndex: 1, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
	}))
	require.NoError(t, model.DB.Create(&model.AggregateGroupRouteModelRatio{
		AggregateGroupId: group.Id,
		RealGroup:        "kiro1",
		ModelName:        "claude-haiku-4-5",
		GroupRatio:       4,
		Enabled:          true,
	}).Error)

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
		UserGroup:       "svip",
		UsingGroup:      "ha-route",
	}

	channel, apiErr := getChannel(ctx, relayInfo, retryParam)
	require.Nil(t, apiErr)
	require.NotNil(t, channel)
	require.Equal(t, 5, channel.Id)
	require.Equal(t, "kiro1", common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup))
	require.Equal(t, 4.0, relayInfo.PriceData.GroupRatioInfo.GroupRatio)
	require.True(t, relayInfo.PriceData.GroupRatioInfo.HasRouteModelGroupRatio)
	require.Equal(t, "kiro1", relayInfo.PriceData.GroupRatioInfo.RouteModelRatioRealGroup)
}

func TestGetAggregateGroupRuntimeDefaultsToSortedModelAndReturnsRouteState(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)
	setting.AggregateGroupSmartStrategyEnabled = true

	group := &model.AggregateGroup{
		Name:                    "runtime-ha",
		DisplayName:             "Runtime HA",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RoutingMode:             model.AggregateGroupRoutingModeCluster,
		SmartRoutingEnabled:     true,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(50)},
		{RealGroup: "vip", OrderIndex: 1, Weight: common.GetPointer(150)},
	}))

	seedAggregateGroupControllerAbilityChannel(t, 1001, "default", "z-model", 0)
	seedAggregateGroupControllerAbilityChannel(t, 1002, "default", "a-model", 0)
	seedAggregateGroupControllerAbilityChannel(t, 1003, "vip", "a-model", 0)

	now := time.Now().Unix()
	setting.AggregateGroupClusterDegradedWeightPct = 50
	require.NoError(t, service.SetAggregateGroupRuntimeState(group.Name, "a-model", &service.AggregateGroupRuntimeState{
		ActiveIndex:   1,
		ActiveGroup:   "vip",
		LastSuccessAt: now,
		LastSwitchAt:  now,
		ActiveSinceAt: now,
	}))
	require.NoError(t, service.SetAggregateGroupRouteStrategyState(group.Name, "a-model", "default", &service.AggregateGroupRouteStrategyState{
		StrategyVersion:             2,
		DegradedUntil:               common.GetTimestamp() + 600,
		DegradeLevel:                1,
		DegradedConsecutiveFailures: 1,
		LastFailureAt:               now,
		LastSlowReason:              service.AggregateSmartSlowReasonFirstResponse,
		LastTriggerReason:           service.AggregateSmartTriggerReasonFailureRate,
		LastTriggerAt:               now - 10,
	}))
	require.NoError(t, service.SetAggregateGroupRouteStrategyState(group.Name, "a-model", "vip", &service.AggregateGroupRouteStrategyState{
		StrategyVersion:  2,
		ConsecutiveSlows: 1,
		LastSuccessAt:    now,
	}))
	rpmRecorder := httptest.NewRecorder()
	rpmCtx, _ := gin.CreateTestContext(rpmRecorder)
	common.SetContextKey(rpmCtx, constant.ContextKeyAggregateGroup, group.Name)
	common.SetContextKey(rpmCtx, constant.ContextKeyRouteGroup, "default")
	service.RecordAggregateRouteRPMAttempt(rpmCtx, "a-model", "default")
	service.RecordAggregateRouteRPMSuccess(rpmCtx, "a-model", "default")
	service.RecordAggregateRouteRPMFailure(rpmCtx, "a-model")
	service.RecordAggregateRouteStrategyFailure(rpmCtx, "a-model", "default")
	service.RecordAggregateRouteSlowSuccess(rpmCtx, "a-model", "default")

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
	require.Equal(t, model.AggregateGroupRoutingModeCluster, resp.Data.AggregateGroup.RoutingMode)
	require.True(t, resp.Data.SmartStrategy.EffectiveEnabled)
	require.NotNil(t, resp.Data.Runtime)
	require.NotNil(t, resp.Data.Runtime.ActiveRoute)
	require.Equal(t, "vip", resp.Data.Runtime.ActiveRoute.ActiveGroup)
	require.Equal(t, now, resp.Data.Runtime.ActiveRoute.ActiveSinceAt)
	require.Len(t, resp.Data.Runtime.Routes, 2)
	require.Equal(t, "default", resp.Data.Runtime.Routes[0].RouteGroup)
	require.Equal(t, 50, resp.Data.Runtime.Routes[0].Weight)
	require.Equal(t, 25, resp.Data.Runtime.Routes[0].EffectiveWeight)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].PriorityCount)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].RPM)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].SuccessRPM)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].FailureRPM)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].StrategyFailureRPM)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].SlowSuccessRPM)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].FailureWindowRequests)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].FailureWindowFailures)
	require.Equal(t, 100, resp.Data.Runtime.Routes[0].FailureRatePercent)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].SlowWindowSuccesses)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].SlowWindowSlowSuccesses)
	require.Equal(t, 100, resp.Data.Runtime.Routes[0].SlowRatePercent)
	require.Equal(t, "global", resp.Data.Runtime.Routes[0].StrategySource)
	require.Equal(t, 2, resp.Data.Runtime.Routes[0].StrategyVersion)
	require.True(t, resp.Data.Runtime.Routes[0].IsDegraded)
	require.False(t, resp.Data.Runtime.Routes[0].IsSoftFallback)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].DegradeLevel)
	require.Equal(t, 1, resp.Data.Runtime.Routes[0].DegradedConsecutiveFailures)
	require.Equal(t, service.AggregateSmartSlowReasonFirstResponse, resp.Data.Runtime.Routes[0].LastSlowReason)
	require.Equal(t, service.AggregateSmartTriggerReasonFailureRate, resp.Data.Runtime.Routes[0].LastTriggerReason)
	require.Equal(t, "vip", resp.Data.Runtime.Routes[1].RouteGroup)
	require.Equal(t, 150, resp.Data.Runtime.Routes[1].Weight)
	require.Equal(t, 150, resp.Data.Runtime.Routes[1].EffectiveWeight)
	require.Equal(t, 1, resp.Data.Runtime.Routes[1].PriorityCount)
	require.True(t, resp.Data.Runtime.Routes[1].IsActive)
	require.False(t, resp.Data.Runtime.Routes[1].IsSoftFallback)
	require.Equal(t, 1, resp.Data.Runtime.Routes[1].ConsecutiveSlows)
}

func TestGetAggregateGroupRuntimeReturnsClientRoutePools(t *testing.T) {
	setupAggregateGroupControllerTestDB(t)

	group := &model.AggregateGroup{
		Name:                    "runtime-client-pool",
		DisplayName:             "Runtime Client Pool",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RoutingMode:             model.AggregateGroupRoutingModeCluster,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.SetClientRoutePools(model.AggregateGroupClientRoutePools{
		Enabled: true,
		ClaudeCodeCLI: model.AggregateGroupClientRoutePool{
			Enabled:           true,
			FallbackToDefault: common.GetPointer(true),
			Targets: []model.AggregateGroupClientRoutePoolTarget{
				{RealGroup: "vip", Weight: common.GetPointer(200)},
			},
		},
	}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(100)},
	}))
	seedAggregateGroupControllerAbilityChannel(t, 1101, "default", "claude-sonnet-4-6", 0)
	seedAggregateGroupControllerAbilityChannel(t, 1102, "vip", "claude-sonnet-4-6", 0)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(group.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/aggregate_group/%d/runtime?model=claude-sonnet-4-6", group.Id), nil)
	GetAggregateGroupRuntime(ctx)

	var resp struct {
		Success bool                          `json:"success"`
		Message string                        `json:"message"`
		Data    aggregateGroupRuntimeResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)
	require.NotNil(t, resp.Data.Runtime)
	require.Len(t, resp.Data.Runtime.ClientRoutePools, 1)
	require.Equal(t, model.AggregateGroupClientRoutePoolClaudeCodeCLI, resp.Data.Runtime.ClientRoutePools[0].PoolName)
	require.True(t, resp.Data.Runtime.ClientRoutePools[0].FallbackToDefault)
	require.Len(t, resp.Data.Runtime.ClientRoutePools[0].Routes, 1)
	require.Equal(t, "vip", resp.Data.Runtime.ClientRoutePools[0].Routes[0].RouteGroup)
	require.Equal(t, 200, resp.Data.Runtime.ClientRoutePools[0].Routes[0].Weight)
	require.Equal(t, 200, resp.Data.Runtime.ClientRoutePools[0].Routes[0].EffectiveWeight)
	require.Equal(t, 1, resp.Data.Runtime.ClientRoutePools[0].Routes[0].PriorityCount)
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
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
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
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
		{RealGroup: "vip", OrderIndex: 1, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
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
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
		{RealGroup: "vip", OrderIndex: 1, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
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
