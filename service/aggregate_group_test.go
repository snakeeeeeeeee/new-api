package service

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func prepareAggregateGroupServiceTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.AggregateGroup{},
		&model.AggregateGroupTarget{},
		&model.Ability{},
		&model.Channel{},
	))
	model.DB.Exec("DELETE FROM aggregate_group_targets")
	model.DB.Exec("DELETE FROM aggregate_groups")
	model.DB.Exec("DELETE FROM abilities")
	model.DB.Exec("DELETE FROM channels")
	originalGroups := setting.UserUsableGroups2JSONString()
	originalRetryTimes := common.RetryTimes
	originalSmartStrategyEnabled := setting.AggregateGroupSmartStrategyEnabled
	originalFailureThreshold := setting.AggregateGroupFailureThreshold
	originalDegradeDurationSeconds := setting.AggregateGroupDegradeDurationSeconds
	originalClusterDegradedWeightPercent := setting.AggregateGroupClusterDegradedWeightPct
	originalSlowRequestThreshold := setting.AggregateGroupSlowRequestThreshold
	originalConsecutiveSlowLimit := setting.AggregateGroupConsecutiveSlowLimit
	originalAffinitySetting := *operation_setting.GetChannelAffinitySetting()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组","svip":"SVIP分组"}`))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM aggregate_group_targets")
		model.DB.Exec("DELETE FROM aggregate_groups")
		model.DB.Exec("DELETE FROM abilities")
		model.DB.Exec("DELETE FROM channels")
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		aggregateGroupRuntimeStateMemory = sync.Map{}
		aggregateGroupRouteStrategyStateMemory = sync.Map{}
		aggregateRouteRPMMemoryMu.Lock()
		aggregateRouteRPMMemoryData = map[string]aggregateRouteRPMMemoryEntry{}
		aggregateRouteRPMMemoryMu.Unlock()
		aggregateRouteRPMNow = time.Now
		ClearAggregateRouteAffinityCacheAll()
		common.MemoryCacheEnabled = false
		common.RetryTimes = originalRetryTimes
		setting.AggregateGroupSmartStrategyEnabled = originalSmartStrategyEnabled
		setting.AggregateGroupFailureThreshold = originalFailureThreshold
		setting.AggregateGroupDegradeDurationSeconds = originalDegradeDurationSeconds
		setting.AggregateGroupClusterDegradedWeightPct = originalClusterDegradedWeightPercent
		setting.AggregateGroupSlowRequestThreshold = originalSlowRequestThreshold
		setting.AggregateGroupConsecutiveSlowLimit = originalConsecutiveSlowLimit
		*operation_setting.GetChannelAffinitySetting() = originalAffinitySetting
	})
}

func seedAggregateGroup(t *testing.T, name string, ratio float64, recoveryInterval int, visibleUserGroups []string, targets []string) *model.AggregateGroup {
	t.Helper()
	group := &model.AggregateGroup{
		Name:                    name,
		DisplayName:             name + "-display",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              ratio,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: recoveryInterval,
	}
	require.NoError(t, group.SetVisibleUserGroups(visibleUserGroups))
	normalizedTargets := NormalizeAggregateTargets(targets)
	require.NoError(t, group.InsertWithTargets(normalizedTargets))
	group.Targets = normalizedTargets
	return group
}

func seedAggregateGroupWithWeightedTargets(t *testing.T, name string, routingMode string, smartRouting bool, targets []model.AggregateGroupTarget) *model.AggregateGroup {
	t.Helper()
	group := &model.AggregateGroup{
		Name:                    name,
		DisplayName:             name + "-display",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RoutingMode:             routingMode,
		SmartRoutingEnabled:     smartRouting,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 10,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	normalizedTargets := NormalizeAggregateTargetsWithWeights(targets)
	require.NoError(t, group.InsertWithTargets(normalizedTargets))
	group.Targets = normalizedTargets
	return group
}

func seedAggregateAbilityChannel(t *testing.T, id int, group string, modelName string, priority int64) {
	seedAggregateAbilityChannelWithStatus(t, id, group, modelName, priority, common.ChannelStatusEnabled)
}

func seedAggregateAbilityChannelWithStatus(t *testing.T, id int, group string, modelName string, priority int64, status int) {
	t.Helper()
	weight := uint(10)
	channel := &model.Channel{
		Id:          id,
		Name:        group + "-channel",
		Key:         "sk-test",
		Status:      status,
		Group:       group,
		Models:      modelName,
		Priority:    &priority,
		Weight:      &weight,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func enableAggregateGroupSmartRouting(t *testing.T, group *model.AggregateGroup) {
	t.Helper()
	require.NotNil(t, group)
	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.SmartRoutingEnabled = true
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargets(loadedGroup.GetTargetGroups())))
	group.SmartRoutingEnabled = true
}

func TestVisibleAggregateGroupsAndRatios(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	seedAggregateGroup(t, "enterprise-stable", 1.5, 300, []string{"vip"}, []string{"default"})

	usableGroups := GetUserUsableGroups("vip")
	require.Contains(t, usableGroups, "default")
	require.Contains(t, usableGroups, "enterprise-stable")

	visibleGroups := GetUserVisibleGroups("vip")
	require.Contains(t, visibleGroups, "default")
	require.Contains(t, visibleGroups, "enterprise-stable")
	require.Equal(t, 1.5, GetUserGroupRatio("vip", "enterprise-stable"))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	ratioInfo := ResolveContextGroupRatioInfo(ctx, "vip", "enterprise-stable")
	require.Equal(t, 1.5, ratioInfo.GroupRatio)
	require.False(t, ratioInfo.HasSpecialRatio)
}

func TestCacheGetRandomSatisfiedChannelUsesAggregateFallbackAndRecovery(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 1, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, "default", common.GetContextKeyString(firstCtx, constant.ContextKeyRouteGroup))

	require.NoError(t, SetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1", &AggregateGroupRuntimeState{
		ActiveIndex: 1,
		ActiveGroup: "vip",
		LastFailAt:  time.Now().Unix(),
	}))
	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)

	require.NoError(t, SetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1", &AggregateGroupRuntimeState{
		ActiveIndex: 1,
		ActiveGroup: "vip",
		LastFailAt:  time.Now().Add(-2 * time.Second).Unix(),
	}))
	thirdCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
}

func TestFailoverIgnoresClusterRuntimeStateAfterRoutingModeSwitch(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)

	require.NoError(t, SetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1", &AggregateGroupRuntimeState{
		ActiveIndex:   1,
		ActiveGroup:   "vip",
		RoutingMode:   model.AggregateGroupRoutingModeCluster,
		LastSuccessAt: common.GetTimestamp(),
		LastSwitchAt:  common.GetTimestamp(),
		ActiveSinceAt: common.GetTimestamp(),
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateStartIndex))
}

func TestFailoverIgnoresLegacyRuntimeStateWithoutFailureTime(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)

	require.NoError(t, SetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1", &AggregateGroupRuntimeState{
		ActiveIndex:   1,
		ActiveGroup:   "vip",
		LastSuccessAt: common.GetTimestamp(),
		LastSwitchAt:  common.GetTimestamp(),
		ActiveSinceAt: common.GetTimestamp(),
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateStartIndex))
}

func TestPrepareAggregateGroupRetrySwitchesToNextRealGroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.False(t, transition.WithinCurrentGroup)
	require.Equal(t, "enterprise-stable", transition.AggregateGroup)
	require.Equal(t, "default", transition.FailedGroup)
	require.Equal(t, 0, transition.FailedIndex)
	require.Equal(t, "vip", transition.NextGroup)
	require.Equal(t, 1, transition.NextIndex)
	require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateRetryIndex))
	require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateRetryBase))
}

func TestPrepareAggregateGroupRetryStaysInCurrentRealGroupWhenLowerPriorityExists(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	common.RetryTimes = 1

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.True(t, transition.WithinCurrentGroup)
	require.Equal(t, "default", transition.NextGroup)
	require.Equal(t, 0, transition.NextIndex)
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateRetryIndex))
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateRetryBase))
}

func TestCacheGetRandomSatisfiedChannelUsesAggregateRetryIndex(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 1)

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup))
	require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateStartIndex))
}

func TestResolveAggregateGroupStartIndexPreservesInitialStartIndexAcrossRetry(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateStartIndex))
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateInitialStartIndex))

	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 1)

	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateStartIndex))
	require.Equal(t, 0, common.GetContextKeyInt(ctx, constant.ContextKeyAggregateInitialStartIndex))
}

func TestRecordAggregateRouteSuccessSetsFailureWindowAfterFallbackSuccess(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "vip")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateStartIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateInitialStartIndex, 0)

	RecordAggregateRouteSuccess(ctx, "gpt-4.1")

	state, err := GetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 1, state.ActiveIndex)
	require.Equal(t, "vip", state.ActiveGroup)
	require.Greater(t, state.LastFailAt, int64(0))
	require.Greater(t, state.LastSwitchAt, int64(0))
	require.Greater(t, state.ActiveSinceAt, int64(0))
}

func TestRecordAggregateRouteSuccessMarksSwitchBackToPrimary(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	now := common.GetTimestamp()
	require.NoError(t, SetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1", &AggregateGroupRuntimeState{
		ActiveIndex:   1,
		ActiveGroup:   "vip",
		LastFailAt:    now - 10,
		LastSuccessAt: now - 10,
		LastSwitchAt:  now - 10,
		ActiveSinceAt: now - 10,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateStartIndex, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateInitialStartIndex, 1)

	RecordAggregateRouteSuccess(ctx, "gpt-4.1")

	state, err := GetAggregateGroupRuntimeState("enterprise-stable", "gpt-4.1")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 0, state.ActiveIndex)
	require.Equal(t, "default", state.ActiveGroup)
	require.Equal(t, int64(0), state.LastFailAt)
	require.Greater(t, state.LastSwitchAt, now-10)
	require.Greater(t, state.ActiveSinceAt, now-10)
}

func TestCacheGetRandomSatisfiedChannelUsesAggregateRetryBaseWithinCurrentRealGroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)
	seedAggregateAbilityChannel(t, 1003, "vip", "gpt-4.1", 0)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1002, channel.Id)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup))
}

func TestPrepareAggregateGroupRetrySwitchesToNextRealGroupWhenInternalRetryDisabled(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	common.RetryTimes = 0

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip", "svip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)
	seedAggregateAbilityChannel(t, 1003, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.False(t, transition.WithinCurrentGroup)
	require.Equal(t, "vip", transition.NextGroup)
	require.Equal(t, 1, transition.NextIndex)
}

func TestPrepareAggregateGroupRetryStaysInCurrentRealGroupWithinConfiguredRetries(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	common.RetryTimes = 1

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.True(t, transition.WithinCurrentGroup)
	require.Equal(t, "default", transition.NextGroup)
	require.Equal(t, 0, transition.NextIndex)
}

func TestValidateAggregateGroupConfigRejectsInvalidRetryStatusCodes(t *testing.T) {
	prepareAggregateGroupServiceTest(t)

	group := &model.AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "enterprise-stable-display",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 10,
		RetryStatusCodes:        "401,abc,500-599",
	}

	err := ValidateAggregateGroupConfig(group, []string{"vip"}, []string{"default"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "重试状态码规则无效")
}

func TestShouldRetryStatusCodeByAggregateGroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)

	group := seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	group.RetryStatusCodes = "401,429,500-599"
	require.NoError(t, group.UpdateWithTargets(NormalizeAggregateTargets([]string{"default", "vip"})))

	shouldRetry, configured := ShouldRetryStatusCodeByAggregateGroup("enterprise-stable", 401)
	require.True(t, configured)
	require.True(t, shouldRetry)

	shouldRetry, configured = ShouldRetryStatusCodeByAggregateGroup("enterprise-stable", 400)
	require.True(t, configured)
	require.False(t, shouldRetry)

	shouldRetry, configured = ShouldRetryStatusCodeByAggregateGroup("default", 500)
	require.False(t, configured)
	require.False(t, shouldRetry)
}

func TestAggregateSmartStrategyDisabledKeepsOriginalSelection(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	require.NoError(t, SetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
}

func TestAggregateSmartStrategySkipsTemporarilyDegradedRoute(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	enableAggregateGroupSmartRouting(t, group)
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	require.NoError(t, SetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup))
}

func TestAggregateSmartStrategyFailureThresholdTriggersTemporaryDegrade(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	setting.AggregateGroupSmartStrategyEnabled = true
	setting.AggregateGroupFailureThreshold = 2
	setting.AggregateGroupDegradeDurationSeconds = 600

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyAggregateSmartRouting, true)

	RecordAggregateRouteSmartFailure(ctx, "gpt-4.1", "default", 503)

	state, err := GetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 1, state.ConsecutiveFailures)
	require.Equal(t, int64(0), state.DegradedUntil)
	require.Empty(t, state.LastTriggerReason)
	require.Equal(t, int64(0), state.LastTriggerAt)

	RecordAggregateRouteSmartFailure(ctx, "gpt-4.1", "default", 503)

	state, err = GetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 2, state.ConsecutiveFailures)
	require.Greater(t, state.DegradedUntil, common.GetTimestamp())
	require.Equal(t, AggregateSmartTriggerReasonConsecutiveFailures, state.LastTriggerReason)
	require.Greater(t, state.LastTriggerAt, int64(0))
}

func TestAggregateSmartStrategySlowSuccessOnlyAffectsLaterRequests(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	setting.AggregateGroupSmartStrategyEnabled = true
	setting.AggregateGroupSlowRequestThreshold = 1
	setting.AggregateGroupConsecutiveSlowLimit = 2
	setting.AggregateGroupDegradeDurationSeconds = 600

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateStartIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateInitialStartIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateSmartRouting, true)
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-2*time.Second))

	RecordAggregateRouteSuccess(ctx, "gpt-4.1")

	state, err := GetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 1, state.ConsecutiveSlows)
	require.Equal(t, int64(0), state.DegradedUntil)
	require.Empty(t, state.LastTriggerReason)
	require.Equal(t, int64(0), state.LastTriggerAt)

	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-2*time.Second))
	RecordAggregateRouteSuccess(ctx, "gpt-4.1")

	state, err = GetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, 2, state.ConsecutiveSlows)
	require.Greater(t, state.DegradedUntil, common.GetTimestamp())
	require.Equal(t, AggregateSmartTriggerReasonConsecutiveSlows, state.LastTriggerReason)
	require.Greater(t, state.LastTriggerAt, int64(0))
}

func TestAggregateSmartStrategyRecoveredRouteParticipatesAgainAfterWindow(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	enableAggregateGroupSmartRouting(t, group)
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	require.NoError(t, SetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		ConsecutiveFailures: 4,
		ConsecutiveSlows:    2,
		DegradedUntil:       common.GetTimestamp() - 1,
		LastTriggerReason:   AggregateSmartTriggerReasonConsecutiveFailures,
		LastTriggerAt:       common.GetTimestamp() - 10,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)

	state, err := GetAggregateGroupRouteStrategyState("enterprise-stable", "gpt-4.1", "default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, int64(0), state.DegradedUntil)
	require.Equal(t, 0, state.ConsecutiveFailures)
	require.Equal(t, 0, state.ConsecutiveSlows)
	require.Equal(t, AggregateSmartTriggerReasonConsecutiveFailures, state.LastTriggerReason)
	require.Greater(t, state.LastTriggerAt, int64(0))
}

func TestAggregateSmartStrategyStillSkipsUnsupportedModelByExistingAbilityLookup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	enableAggregateGroupSmartRouting(t, group)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
}

func TestAggregateGroupDefaultRoutingModeKeepsFailoverSelection(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "enterprise-stable",
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, model.AggregateGroupRoutingModeFailover, common.GetContextKeyString(ctx, constant.ContextKeyAggregateRoutingMode))
}

func TestAggregateClusterSelectsWeightedSupportedRouteAndSkipsUnsupportedModel(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	seedAggregateGroupWithWeightedTargets(t, "cluster-route", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
		{RealGroup: "svip", Weight: common.GetPointer(100)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1003, "svip", "claude-haiku-4-5", 10)

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
			Ctx:        ctx,
			TokenGroup: "cluster-route",
			ModelName:  "gpt-4.1",
			Retry:      common.GetPointer(0),
		})
		require.NoError(t, err)
		require.NotNil(t, channel)
		require.Equal(t, "vip", selectedGroup)
		require.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyRouteGroupIndex))
	}
}

func TestAggregateClusterManuallyDisabledRouteIsHardUnavailable(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-hard-unavailable", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannelWithStatus(t, 1001, "default", "gpt-4.1", 10, common.ChannelStatusManuallyDisabled)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)

	_, candidates, err := buildAggregateClusterRouteCandidates(ctx, group, "gpt-4.1", true, false)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "vip", candidates[0].Target.RealGroup)
}

func TestAggregateClusterSmartStrategyReducesDegradedRouteWeightInsteadOfSkipping(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-reduced", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)
	require.NoError(t, SetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)

	_, candidates, err := buildAggregateClusterRouteCandidates(ctx, group, "gpt-4.1", true, false)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, "default", candidates[0].Target.RealGroup)
	require.True(t, candidates[0].IsDegraded)
	require.Equal(t, 100, candidates[0].Weight)
	require.Equal(t, 20, candidates[0].EffectiveWeight)
	require.Equal(t, "vip", candidates[1].Target.RealGroup)
	require.False(t, candidates[1].IsDegraded)
	require.Equal(t, 0, candidates[1].EffectiveWeight)
}

func TestAggregateClusterSmartStrategyUsesConfiguredDegradedWeightPercent(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true
	setting.AggregateGroupClusterDegradedWeightPct = 35

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-configured-reduced", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(101)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	require.NoError(t, SetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	_, candidates, err := buildAggregateClusterRouteCandidates(ctx, group, "gpt-4.1", true, false)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.True(t, candidates[0].IsDegraded)
	require.Equal(t, 101, candidates[0].Weight)
	require.Equal(t, 36, candidates[0].EffectiveWeight)
}

func TestAggregateClusterZeroWeightDegradedRouteKeepsZeroEffectiveWeight(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-zero-reduced", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)
	require.NoError(t, SetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	_, candidates, err := buildAggregateClusterRouteCandidates(ctx, group, "gpt-4.1", true, false)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, "default", candidates[0].Target.RealGroup)
	require.True(t, candidates[0].IsDegraded)
	require.Equal(t, 0, candidates[0].Weight)
	require.Equal(t, 0, candidates[0].EffectiveWeight)
	require.Equal(t, "vip", candidates[1].Target.RealGroup)
	require.Equal(t, 100, candidates[1].EffectiveWeight)
}

func TestAggregateClusterDegradedRouteEffectiveWeightRestoresAfterWindow(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-recovered-weight", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	require.NoError(t, SetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		ConsecutiveFailures: 2,
		DegradedUntil:       common.GetTimestamp() - 1,
		LastTriggerReason:   AggregateSmartTriggerReasonConsecutiveFailures,
		LastTriggerAt:       common.GetTimestamp() - 10,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	_, candidates, err := buildAggregateClusterRouteCandidates(ctx, group, "gpt-4.1", true, false)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.False(t, candidates[0].IsDegraded)
	require.Equal(t, 100, candidates[0].Weight)
	require.Equal(t, 100, candidates[0].EffectiveWeight)
}

func TestAggregateClusterRouteAffinityStableReducesDegradedAndUpdatesOnSuccess(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	setting.AggregateGroupSmartStrategyEnabled = true

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity", model.AggregateGroupRoutingModeCluster, true, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "vip", "gpt-4.1", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)

	require.NoError(t, SetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default", &AggregateGroupRouteStrategyState{
		DegradedUntil: common.GetTimestamp() + 600,
	}))

	thirdCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Empty(t, common.GetContextKeyString(thirdCtx, constant.ContextKeyAggregateRouteAffinityHit))
	RecordAggregateRouteSuccess(thirdCtx, "gpt-4.1")

	loadedGroup, err = model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})))

	require.NoError(t, ResetAggregateGroupRouteStrategyState(group.Name, "gpt-4.1", "default"))
	fourthCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        fourthCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
}

func TestAggregateClusterRouteAffinityTTLAllowsWeightedReselect(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity-ttl", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})))

	seedAggregateAbilityChannel(t, 1101, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1102, "vip", "gpt-4.1", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	loadedGroup, err = model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)

	time.Sleep(1100 * time.Millisecond)

	thirdCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
}

func TestAggregateRouteAffinityTTLUsesClusterConfig(t *testing.T) {
	prepareAggregateGroupServiceTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRecoveryEnabled, false)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRecoveryInterval, 1)
	common.SetContextKey(ctx, constant.ContextKeyAggregateClusterAffinityTTL, 45)

	require.Equal(t, 45*time.Second, resolveAggregateRouteAffinityTTL(ctx))

	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	require.Equal(t, time.Duration(model.AggregateGroupClusterAffinityTTLDefaultSeconds)*time.Second, resolveAggregateRouteAffinityTTL(ctx))
}

func TestAggregateClusterRouteAffinitySuccessDoesNotExtendTTL(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity-no-slide", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})))

	seedAggregateAbilityChannel(t, 1301, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1302, "vip", "gpt-4.1", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	time.Sleep(650 * time.Millisecond)

	loadedGroup, err = model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(secondCtx, "gpt-4.1")

	time.Sleep(500 * time.Millisecond)

	thirdCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
}

func TestAggregateClusterRouteAffinityHitDoesNotRewriteAfterExpiry(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity-inflight-expiry", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})))

	seedAggregateAbilityChannel(t, 1401, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1402, "vip", "gpt-4.1", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	loadedGroup, err = model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	loadedGroup.ClusterAffinityTTLSeconds = 1
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, "default", common.GetContextKeyString(secondCtx, constant.ContextKeyAggregateRouteAffinityHit))

	time.Sleep(1100 * time.Millisecond)
	RecordAggregateRouteSuccess(secondCtx, "gpt-4.1")

	thirdCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
}

func TestAggregateClusterRouteAffinityFollowsUserAcrossSupportedModels(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity-user-model", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannel(t, 1201, "default", "gpt-4.1,gpt-5", 10)
	seedAggregateAbilityChannel(t, 1202, "vip", "gpt-4.1,gpt-5", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(0)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-5",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	require.Equal(t, "default", common.GetContextKeyString(secondCtx, constant.ContextKeyAggregateRouteAffinityHit))
}

func TestAggregateClusterRouteAffinitySkipsUserRouteWhenModelUnsupported(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-affinity-user-unsupported", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannel(t, 1251, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1252, "vip", "gpt-5", 10)

	buildCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 42)
		return ctx
	}

	firstCtx := buildCtx()
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", selectedGroup)
	RecordAggregateRouteSuccess(firstCtx, "gpt-4.1")

	loadedGroup, err := model.GetAggregateGroupByID(group.Id)
	require.NoError(t, err)
	require.NoError(t, loadedGroup.UpdateWithTargets(NormalizeAggregateTargetsWithWeights([]model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(100)},
	})))

	secondCtx := buildCtx()
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        secondCtx,
		TokenGroup: group.Name,
		ModelName:  "gpt-5",
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Empty(t, common.GetContextKeyString(secondCtx, constant.ContextKeyAggregateRouteAffinityHit))
}

func TestAggregateClusterRetryExhaustionSwitchesToAnotherRoute(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.MemoryCacheEnabled = false
	common.RetryTimes = 1

	group := seedAggregateGroupWithWeightedTargets(t, "cluster-retry", model.AggregateGroupRoutingModeCluster, false, []model.AggregateGroupTarget{
		{RealGroup: "default", Weight: common.GetPointer(100)},
		{RealGroup: "vip", Weight: common.GetPointer(0)},
	})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)
	seedAggregateAbilityChannel(t, 1003, "vip", "gpt-4.1", 10)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	retry := 0
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      &retry,
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1001, channel.Id)
	require.Equal(t, "default", selectedGroup)

	transition := PrepareAggregateGroupRetry(ctx, retry, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.True(t, transition.WithinCurrentGroup)
	require.Equal(t, "default", transition.NextGroup)

	retry = 1
	channel, selectedGroup, err = CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group.Name,
		ModelName:  "gpt-4.1",
		Retry:      &retry,
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1002, channel.Id)
	require.Equal(t, "default", selectedGroup)

	transition = PrepareAggregateGroupRetry(ctx, retry, "gpt-4.1", common.RetryTimes)
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
	require.False(t, transition.WithinCurrentGroup)
	require.Equal(t, "vip", transition.NextGroup)
	require.Equal(t, 1, transition.NextIndex)
}

func TestAggregateRouteRPMCountsRecentWindowOnly(t *testing.T) {
	prepareAggregateGroupServiceTest(t)
	common.RedisEnabled = false

	base := time.Unix(2000, 0)
	aggregateRouteRPMNow = func() time.Time { return base.Add(-61 * time.Second) }
	recordAggregateRouteRPM("cluster-route", "gpt-4.1", "default", aggregateRouteRPMMetricAttempt)
	recordAggregateRouteRPM("cluster-route", "gpt-4.1", "default", aggregateRouteRPMMetricFailure)

	aggregateRouteRPMNow = func() time.Time { return base }
	recordAggregateRouteRPM("cluster-route", "gpt-4.1", "default", aggregateRouteRPMMetricAttempt)
	recordAggregateRouteRPM("cluster-route", "gpt-4.1", "default", aggregateRouteRPMMetricSuccess)

	stats := GetAggregateRouteRPMStats("cluster-route", "gpt-4.1", "default")
	require.Equal(t, 1, stats.RPM)
	require.Equal(t, 1, stats.SuccessRPM)
	require.Equal(t, 0, stats.FailureRPM)

	aggregateRouteRPMNow = func() time.Time { return base.Add(59 * time.Second) }
	stats = GetAggregateRouteRPMStats("cluster-route", "gpt-4.1", "default")
	require.Equal(t, 1, stats.RPM)
	require.Equal(t, 1, stats.SuccessRPM)

	aggregateRouteRPMNow = func() time.Time { return base.Add(61 * time.Second) }
	stats = GetAggregateRouteRPMStats("cluster-route", "gpt-4.1", "default")
	require.Equal(t, 0, stats.RPM)
	require.Equal(t, 0, stats.SuccessRPM)
	require.Equal(t, 0, stats.FailureRPM)
}
