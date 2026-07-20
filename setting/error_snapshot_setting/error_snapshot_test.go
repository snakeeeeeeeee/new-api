package error_snapshot_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOptionValue(t *testing.T) {
	value, err := NormalizeOptionValue("ttl_minutes", "45")
	require.NoError(t, err)
	require.Equal(t, "45", value)

	value, err = NormalizeOptionValue("priority_user_ids", "3, 1,3")
	require.NoError(t, err)
	require.Equal(t, "1,3", value)

	_, err = NormalizeOptionValue("ttl_minutes", "4")
	require.Error(t, err)
	_, err = NormalizeOptionValue("max_storage_mib", "10241")
	require.Error(t, err)
	_, err = NormalizeOptionValue("priority_channel_ids", "1,bad")
	require.Error(t, err)
}

func TestRefreshSnapshotAndPriorityMatching(t *testing.T) {
	cfg := config.GlobalConfig.Get("error_snapshot")
	original, err := config.ConfigToMap(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(cfg, original))
		RefreshSnapshot()
	})

	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"enabled":              "true",
		"ttl_minutes":          "45",
		"max_storage_mib":      "512",
		"max_files":            "2000",
		"priority_user_ids":    "7,9",
		"priority_channel_ids": "11,13",
	}))
	snapshot := RefreshSnapshot()
	require.True(t, snapshot.Enabled)
	require.Equal(t, 45, snapshot.TTLMinutes)
	require.Equal(t, 512, snapshot.MaxStorageMiB)
	require.Equal(t, 2000, snapshot.MaxFiles)
	require.True(t, IsPriority(7, 0))
	require.True(t, IsPriority(0, 13))
	require.False(t, IsPriority(8, 12))
}
