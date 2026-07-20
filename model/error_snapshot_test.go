package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestErrorSnapshotCRUDAndFilters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ErrorSnapshot{}))
	oldLogDB := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = oldLogDB })

	now := time.Now().Unix()
	rows := []*ErrorSnapshot{
		{ID: "11111111111111111111111111111111", CreatedAt: now - 1, RequestID: "req-a", UserID: 7, Username: "alice", ChannelID: 11, ErrorMessage: "Claude empty content", RelativePath: "20260720/a.json.gz", CompressedSize: 100, FinalOutcome: "pending"},
		{ID: "22222222222222222222222222222222", CreatedAt: now, RequestID: "req-a", UserID: 8, Username: "bob", ChannelID: 12, ErrorMessage: "upstream timeout", RelativePath: "20260720/b.json.gz", CompressedSize: 200, FinalOutcome: "pending"},
	}
	for _, row := range rows {
		require.NoError(t, CreateErrorSnapshot(row))
	}

	items, total, err := ListErrorSnapshots(ErrorSnapshotQuery{RequestID: "req-a", ErrorKeyword: "EMPTY", Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, rows[0].ID, items[0].ID)

	stats, err := GetErrorSnapshotStorageStats()
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.FileCount)
	require.EqualValues(t, 300, stats.TotalBytes)
	require.Equal(t, now-1, stats.OldestAt)

	require.NoError(t, UpdateErrorSnapshotOutcome("req-a", "fallback_succeeded"))
	updated, err := GetErrorSnapshot(rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, "fallback_succeeded", updated.FinalOutcome)

	require.NoError(t, DeleteErrorSnapshotRecord(rows[0].ID))
	_, err = GetErrorSnapshot(rows[0].ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
