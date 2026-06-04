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
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight), RPMLimit: 0},
		{RealGroup: "vip", OrderIndex: 1, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight), RPMLimit: 120},
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
	require.Equal(t, 0, loaded.Targets[0].GetRPMLimit())
	require.Equal(t, "vip", loaded.Targets[1].RealGroup)
	require.Equal(t, AggregateGroupTargetDefaultWeight, loaded.Targets[1].GetWeight())
	require.Equal(t, 120, loaded.Targets[1].GetRPMLimit())
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
				{RealGroup: "cli-a", Weight: aggregateTargetWeightForTest(200), RPMLimit: 90},
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
	require.Equal(t, 90, config.ClaudeCodeCLI.Targets[0].GetRPMLimit())

	empty := (&AggregateGroup{}).GetClientRoutePools()
	require.False(t, empty.Enabled)
	require.True(t, empty.ClaudeCodeCLI.GetFallbackToDefault())
	require.Empty(t, empty.ClaudeCodeCLI.Targets)
}

func TestAggregateGroupSmartStrategyConfigRoundTrip(t *testing.T) {
	prepareAggregateGroupModelTest(t)

	group := &AggregateGroup{
		Name:                    "enterprise-smart-strategy",
		DisplayName:             "企业策略覆盖",
		Status:                  AggregateGroupStatusEnabled,
		GroupRatio:              1,
		RoutingMode:             AggregateGroupRoutingModeCluster,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.SetSmartStrategyConfig(&AggregateGroupSmartStrategyConfig{
		FailureRateWindowSeconds:   aggregateTargetWeightForTest(120),
		FailureRateMinRequests:     aggregateTargetWeightForTest(50),
		FailureRateThresholdPct:    aggregateTargetWeightForTest(8),
		SlowRateWindowSeconds:      aggregateTargetWeightForTest(180),
		SlowRateMinRequests:        aggregateTargetWeightForTest(40),
		SlowRateThresholdPct:       aggregateTargetWeightForTest(25),
		DegradeDurationSeconds:     aggregateTargetWeightForTest(300),
		ClusterDegradedWeightPct:   aggregateTargetWeightForTest(35),
		SlowRequestThreshold:       aggregateTargetWeightForTest(20),
		SlowFirstResponseThreshold: aggregateTargetWeightForTest(1),
	}))
	require.NoError(t, group.InsertWithTargets([]AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: aggregateTargetWeightForTest(AggregateGroupTargetDefaultWeight)},
	}))

	loaded, err := GetAggregateGroupByName("enterprise-smart-strategy", true)
	require.NoError(t, err)
	config := loaded.GetSmartStrategyConfig()
	require.NotNil(t, config)
	require.Equal(t, 120, *config.FailureRateWindowSeconds)
	require.Equal(t, 50, *config.FailureRateMinRequests)
	require.Equal(t, 8, *config.FailureRateThresholdPct)
	require.Equal(t, 180, *config.SlowRateWindowSeconds)
	require.Equal(t, 40, *config.SlowRateMinRequests)
	require.Equal(t, 25, *config.SlowRateThresholdPct)
	require.Equal(t, 300, *config.DegradeDurationSeconds)
	require.Equal(t, 35, *config.ClusterDegradedWeightPct)
	require.Equal(t, 20, *config.SlowRequestThreshold)
	require.Equal(t, 1, *config.SlowFirstResponseThreshold)

	require.NoError(t, loaded.SetSmartStrategyConfig(nil))
	require.Empty(t, loaded.SmartStrategyConfig)
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
