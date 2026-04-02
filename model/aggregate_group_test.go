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
		{RealGroup: "default", OrderIndex: 0},
		{RealGroup: "vip", OrderIndex: 1},
	}))

	loaded, err := GetAggregateGroupByName("enterprise-stable", true)
	require.NoError(t, err)
	require.Equal(t, []string{"vip", "svip"}, loaded.GetVisibleUserGroups())
	require.Len(t, loaded.Targets, 2)
	require.Equal(t, "default", loaded.Targets[0].RealGroup)
	require.Equal(t, "vip", loaded.Targets[1].RealGroup)
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
		{RealGroup: "default", OrderIndex: 0},
	}))

	require.NoError(t, DeleteAggregateGroupByID(group.Id))

	_, err := GetAggregateGroupByID(group.Id)
	require.Error(t, err)

	var targetCount int64
	require.NoError(t, DB.Model(&AggregateGroupTarget{}).Where("aggregate_group_id = ?", group.Id).Count(&targetCount).Error)
	require.Zero(t, targetCount)
}
