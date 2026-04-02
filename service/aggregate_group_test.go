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
