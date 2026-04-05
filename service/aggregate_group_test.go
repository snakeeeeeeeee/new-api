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
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组","svip":"SVIP分组"}`))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM aggregate_group_targets")
		model.DB.Exec("DELETE FROM aggregate_groups")
		model.DB.Exec("DELETE FROM abilities")
		model.DB.Exec("DELETE FROM channels")
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		aggregateGroupRuntimeStateMemory = sync.Map{}
		common.MemoryCacheEnabled = false
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
	require.NoError(t, group.InsertWithTargets(NormalizeAggregateTargets(targets)))
	return group
}

func seedAggregateAbilityChannel(t *testing.T, id int, group string, modelName string, priority int64) {
	t.Helper()
	weight := uint(10)
	channel := &model.Channel{
		Id:          id,
		Name:        group + "-channel",
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

func TestPrepareAggregateGroupRetrySwitchesToNextRealGroup(t *testing.T) {
	prepareAggregateGroupServiceTest(t)

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1")
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
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

	seedAggregateGroup(t, "enterprise-stable", 1.2, 10, []string{"vip"}, []string{"default", "vip"})
	seedAggregateAbilityChannel(t, 1001, "default", "gpt-4.1", 10)
	seedAggregateAbilityChannel(t, 1002, "default", "gpt-4.1", 0)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, 0)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, 0)

	transition := PrepareAggregateGroupRetry(ctx, 0, "gpt-4.1")
	require.NotNil(t, transition)
	require.True(t, transition.HasNext)
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
