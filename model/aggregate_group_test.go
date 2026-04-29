package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func prepareAggregateGroupModelTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AggregateGroup{}, &AggregateGroupTarget{}))
	DB.Exec("DELETE FROM aggregate_group_targets")
	DB.Exec("DELETE FROM aggregate_groups")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM aggregate_group_targets")
		DB.Exec("DELETE FROM aggregate_groups")
	})
}

func aggregateTargetWeightForTest(weight int) *int {
	return &weight
}

func TestAggregateGroupVisibleUserGroupsRoundTrip(t *testing.T) {
	prepareAggregateGroupModelTest(t)

	group := &AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "企业稳定组",
		Status:                  AggregateGroupStatusEnabled,
		GroupRatio:              1.2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip", "svip"}))
	require.NoError(t, group.InsertWithTargets([]AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
		{RealGroup: "vip", OrderIndex: 1, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
	}))

	loaded, err := GetAggregateGroupByName("enterprise-stable", true)
	require.NoError(t, err)
	require.Equal(t, []string{"vip", "svip"}, loaded.GetVisibleUserGroups())
	require.Equal(t, AggregateGroupRoutingModeFailover, loaded.GetRoutingMode())
	require.Equal(t, AggregateGroupClusterAffinityTTLDefaultSeconds, loaded.GetClusterAffinityTTLSeconds())
	require.Equal(t, AggregateGroupRouteAffinityStrategyPlatformUser, loaded.GetRouteAffinityStrategy())
	require.Len(t, loaded.Targets, 2)
	require.Equal(t, "default", loaded.Targets[0].RealGroup)
	require.Equal(t, AggregateGroupTargetDefaultWeight, loaded.Targets[0].GetWeight())
	require.Equal(t, "vip", loaded.Targets[1].RealGroup)
	require.Equal(t, AggregateGroupTargetDefaultWeight, loaded.Targets[1].GetWeight())
}

func TestAggregateGroupRouteAffinityKeySourcesRoundTrip(t *testing.T) {
	prepareAggregateGroupModelTest(t)

	group := &AggregateGroup{
		Name:                  "enterprise-request-affinity",
		DisplayName:           "企业请求亲和",
		Status:                AggregateGroupStatusEnabled,
		GroupRatio:            1,
		RoutingMode:           AggregateGroupRoutingModeCluster,
		RouteAffinityStrategy: AggregateGroupRouteAffinityStrategyRequestOnly,
		RecoveryEnabled:       true,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.SetRouteAffinityKeySources([]AggregateGroupRouteAffinityKeySource{
		{Type: "header", Key: "X-Aggregate-Affinity-Key"},
		{Type: "gjson", Path: "metadata.user_id"},
	}))
	require.NoError(t, group.InsertWithTargets([]AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
	}))

	loaded, err := GetAggregateGroupByName("enterprise-request-affinity", true)
	require.NoError(t, err)
	require.Equal(t, AggregateGroupRouteAffinityStrategyRequestOnly, loaded.GetRouteAffinityStrategy())
	sources := loaded.GetRouteAffinityKeySources()
	require.Len(t, sources, 2)
	require.Equal(t, "header", sources[0].Type)
	require.Equal(t, "X-Aggregate-Affinity-Key", sources[0].Key)
	require.Equal(t, "gjson", sources[1].Type)
	require.Equal(t, "metadata.user_id", sources[1].Path)
}

func TestAggregateGroupClientRoutePoolsRoundTrip(t *testing.T) {
	prepareAggregateGroupModelTest(t)

	fallbackToDefault := false
	group := &AggregateGroup{
		Name:                    "enterprise-client-pool",
		DisplayName:             "企业客户端池",
		Status:                  AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RoutingMode:             AggregateGroupRoutingModeCluster,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.SetClientRoutePools(AggregateGroupClientRoutePools{
		Enabled: true,
		ClaudeCodeCLI: AggregateGroupClientRoutePool{
			Enabled:           true,
			FallbackToDefault: &fallbackToDefault,
			Targets: []AggregateGroupClientRoutePoolTarget{
				{RealGroup: "cli-a", Weight: aggregateTargetWeightForTest(200)},
			},
		},
	}))
	require.NoError(t, group.InsertWithTargets([]AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
	}))

	loaded, err := GetAggregateGroupByName("enterprise-client-pool", true)
	require.NoError(t, err)
	config := loaded.GetClientRoutePools()
	require.True(t, config.Enabled)
	require.True(t, config.ClaudeCodeCLI.Enabled)
	require.False(t, config.ClaudeCodeCLI.GetFallbackToDefault())
	require.Len(t, config.ClaudeCodeCLI.Targets, 1)
	require.Equal(t, "cli-a", config.ClaudeCodeCLI.Targets[0].RealGroup)
	require.Equal(t, 200, config.ClaudeCodeCLI.Targets[0].GetWeight())

	empty := (&AggregateGroup{}).GetClientRoutePools()
	require.False(t, empty.Enabled)
	require.True(t, empty.ClaudeCodeCLI.GetFallbackToDefault())
	require.Empty(t, empty.ClaudeCodeCLI.Targets)
}

func TestDeleteAggregateGroupByIDDeletesTargets(t *testing.T) {
	prepareAggregateGroupModelTest(t)

	group := &AggregateGroup{
		Name:                    "enterprise-premium",
		DisplayName:             "企业高级组",
		Status:                  AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
	}))

	require.NoError(t, DeleteAggregateGroupByID(group.Id))

	_, err := GetAggregateGroupByID(group.Id)
	require.Error(t, err)

	var targetCount int64
	require.NoError(t, DB.Model(&AggregateGroupTarget{}).Where("aggregate_group_id = ?", group.Id).Count(&targetCount).Error)
	require.Zero(t, targetCount)
}
