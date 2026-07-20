package controller

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/error_snapshot_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupErrorSnapshotControllerTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.ErrorSnapshot{}, &model.User{}, &model.Channel{}))
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})

	cfg := config.GlobalConfig.Get("error_snapshot")
	original, err := config.ConfigToMap(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(cfg, original))
		error_snapshot_setting.RefreshSnapshot()
	})
	model.InitOptionMap()
	t.Setenv("ERROR_SNAPSHOT_DIR", filepath.Join(t.TempDir(), "snapshots"))
}

func TestErrorSnapshotControllerSettingsAndList(t *testing.T) {
	setupErrorSnapshotControllerTest(t)

	updateCtx, updateRecorder := requestDumpControllerCtx(http.MethodPut, "/api/request_dump/error_snapshots/settings", `{
		"enabled":true,
		"ttl_minutes":45,
		"max_storage_mib":512,
		"max_files":2000,
		"priority_user_ids":[7,9],
		"priority_channel_ids":[11]
	}`)
	UpdateErrorSnapshotSettings(updateCtx)
	var updateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(updateRecorder.Body.Bytes(), &updateResp))
	require.True(t, updateResp.Success, updateResp.Message)
	require.Contains(t, string(updateResp.Data), `"ttl_minutes":45`)
	require.Contains(t, string(updateResp.Data), `"priority_user_ids":[7,9]`)

	now := time.Now().Unix()
	require.NoError(t, model.CreateErrorSnapshot(&model.ErrorSnapshot{
		ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: now, RequestID: "req-controller",
		UserID: 7, Username: "alice", ChannelID: 11, ErrorMessage: "empty content",
		RelativePath: "20260720/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.json.gz", FinalOutcome: "final_failure",
	}))
	listCtx, listRecorder := requestDumpControllerCtx(http.MethodGet, "/api/request_dump/error_snapshots?request_id=req-controller&p=1&page_size=20", "")
	GetErrorSnapshots(listCtx)
	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"request_id":"req-controller"`)
}

func TestErrorSnapshotControllerRejectsInvalidSettings(t *testing.T) {
	setupErrorSnapshotControllerTest(t)
	ctx, recorder := requestDumpControllerCtx(http.MethodPut, "/api/request_dump/error_snapshots/settings", `{
		"enabled":true,
		"ttl_minutes":1,
		"max_storage_mib":256,
		"max_files":1000
	}`)
	UpdateErrorSnapshotSettings(ctx)
	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "between 5 and 10080")
}

func TestErrorSnapshotSelectOptionsUseRequestDumpPermissionBoundary(t *testing.T) {
	setupErrorSnapshotControllerTest(t)
	user := model.User{Username: "snapshot-user", DisplayName: "Snapshot User"}
	channel := model.Channel{Name: "snapshot-channel"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&channel).Error)

	userCtx, userRecorder := requestDumpControllerCtx(http.MethodGet, "/api/request_dump/error_snapshots/select_options?type=user&keyword=snapshot-user", "")
	GetErrorSnapshotSelectOptions(userCtx)
	var userResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(userRecorder.Body.Bytes(), &userResp))
	require.True(t, userResp.Success, userResp.Message)
	require.Contains(t, string(userResp.Data), `"username":"snapshot-user"`)
	require.NotContains(t, string(userResp.Data), "password")

	channelCtx, channelRecorder := requestDumpControllerCtx(http.MethodGet, "/api/request_dump/error_snapshots/select_options?type=channel&keyword=snapshot-channel", "")
	GetErrorSnapshotSelectOptions(channelCtx)
	var channelResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(channelRecorder.Body.Bytes(), &channelResp))
	require.True(t, channelResp.Success, channelResp.Message)
	require.Contains(t, string(channelResp.Data), `"name":"snapshot-channel"`)
	require.NotContains(t, string(channelResp.Data), "key")
}
