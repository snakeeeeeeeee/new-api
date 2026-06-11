package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAsyncTaskSettingForTest(t *testing.T) {
	t.Helper()
	setting := async_task_setting.GetAsyncTaskSetting()
	original := *setting
	originalOptionMap := common.OptionMap
	t.Cleanup(func() {
		*setting = original
		async_task_setting.ApplyNormalization()
		common.OptionMap = originalOptionMap
	})
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()
	*setting = async_task_setting.AsyncTaskSetting{
		DefaultTimeoutMinutes: 30,
		QueryLimit:            1000,
		TimeoutOverrides:      []async_task_setting.TimeoutOverride{},
	}
	async_task_setting.ApplyNormalization()
}

func createAsyncTimeoutTask(t *testing.T, taskID string, platform constant.TaskPlatform, action string, status model.TaskStatus, submitAge time.Duration, quota int) *model.Task {
	t.Helper()
	now := time.Now().Unix()
	task := &model.Task{
		TaskID:     taskID,
		UserId:     1,
		ChannelId:  1,
		Quota:      quota,
		Status:     status,
		Action:     action,
		Platform:   platform,
		Group:      "default",
		Progress:   "0%",
		SubmitTime: now - int64(submitAge.Seconds()),
		CreatedAt:  now - int64(submitAge.Seconds()),
		UpdatedAt:  now - int64(submitAge.Seconds()),
		PrivateData: model.TaskPrivateData{
			BillingSource: BillingSourceWallet,
			TokenId:       1,
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

func TestSweepTimedOutTasksUsesDefaultThirtyMinutesAndRefunds(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	ctx := context.Background()

	const initQuota, tokenRemain, preConsumed = 10000, 6000, 4000
	seedUser(t, 1, initQuota)
	seedToken(t, 1, 1, "sk-timeout-default", tokenRemain)
	seedChannel(t, 1)
	createAsyncTimeoutTask(t, "task_timeout_default", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusSubmitted, 31*time.Minute, preConsumed)
	createAsyncTimeoutTask(t, "task_not_timeout_default", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusInProgress, 29*time.Minute, preConsumed)

	sweepTimedOutTasks(ctx)

	var timedOut model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_timeout_default").First(&timedOut).Error)
	assert.EqualValues(t, model.TaskStatusFailure, timedOut.Status)
	assert.Equal(t, "100%", timedOut.Progress)
	assert.Contains(t, timedOut.FailReason, "30分钟")
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, 1))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, 1))

	var active model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_not_timeout_default").First(&active).Error)
	assert.EqualValues(t, model.TaskStatusInProgress, active.Status)
}

func TestSweepTimedOutTasksUsesPlatformAndActionOverrides(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	ctx := context.Background()
	setting := async_task_setting.GetAsyncTaskSetting()
	setting.TimeoutOverrides = []async_task_setting.TimeoutOverride{
		{Platform: "48", TimeoutMinutes: 60, Enabled: true},
		{Platform: "48", Action: constant.TaskActionVideoEdit, TimeoutMinutes: 20, Enabled: true},
	}
	async_task_setting.ApplyNormalization()

	seedUser(t, 1, 10000)
	seedToken(t, 1, 1, "sk-timeout-override", 6000)
	seedChannel(t, 1)
	createAsyncTimeoutTask(t, "task_xai_platform_waits", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusQueued, 45*time.Minute, 1000)
	createAsyncTimeoutTask(t, "task_xai_action_timeout", constant.TaskPlatform("48"), constant.TaskActionVideoEdit, model.TaskStatusInProgress, 25*time.Minute, 1000)
	createAsyncTimeoutTask(t, "task_other_default_timeout", constant.TaskPlatform("54"), constant.TaskActionVideoGeneration, model.TaskStatusInProgress, 31*time.Minute, 1000)

	sweepTimedOutTasks(ctx)

	var platformTask model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_xai_platform_waits").First(&platformTask).Error)
	assert.EqualValues(t, model.TaskStatusQueued, platformTask.Status)

	var actionTask model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_xai_action_timeout").First(&actionTask).Error)
	assert.EqualValues(t, model.TaskStatusFailure, actionTask.Status)
	assert.Contains(t, actionTask.FailReason, "20分钟")

	var defaultTask model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_other_default_timeout").First(&defaultTask).Error)
	assert.EqualValues(t, model.TaskStatusFailure, defaultTask.Status)
	assert.Contains(t, defaultTask.FailReason, "30分钟")
}

func TestSweepTimedOutTasksSkipsTerminalTasks(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	seedUser(t, 1, 10000)
	seedToken(t, 1, 1, "sk-timeout-terminal", 6000)
	seedChannel(t, 1)
	createAsyncTimeoutTask(t, "task_success_terminal", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusSuccess, 2*time.Hour, 1000)
	createAsyncTimeoutTask(t, "task_failure_terminal", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusFailure, 2*time.Hour, 1000)

	sweepTimedOutTasks(context.Background())

	var successTask model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_success_terminal").First(&successTask).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, successTask.Status)

	var failureTask model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_failure_terminal").First(&failureTask).Error)
	assert.EqualValues(t, model.TaskStatusFailure, failureTask.Status)
	assert.Equal(t, 10000, getUserQuota(t, 1))
}

func TestGetAsyncTaskStatsAggregatesUnfinishedTasks(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	seedUser(t, 1, 10000)
	seedToken(t, 1, 1, "sk-timeout-stats", 6000)
	seedChannel(t, 1)
	createAsyncTimeoutTask(t, "task_stats_submitted", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusSubmitted, 31*time.Minute, 0)
	createAsyncTimeoutTask(t, "task_stats_progress", constant.TaskPlatform("54"), constant.TaskActionVideoEdit, model.TaskStatusInProgress, 11*time.Minute, 0)
	createAsyncTimeoutTask(t, "task_stats_success", constant.TaskPlatform("48"), constant.TaskActionVideoGeneration, model.TaskStatusSuccess, 90*time.Minute, 0)

	stats := GetAsyncTaskStats()

	assert.Equal(t, int64(2), stats.TotalUnfinished)
	assert.Equal(t, int64(1), stats.TimeoutPending)
	assert.Equal(t, int64(2), stats.Over10Minutes)
	assert.Equal(t, int64(1), stats.Over30Minutes)
	assert.Equal(t, int64(0), stats.Over60Minutes)
	require.Len(t, stats.ByStatus, 2)
	assert.Contains(t, stats.ByStatus, model.AsyncTaskStatusStat{Status: string(model.TaskStatusSubmitted), Count: 1})
	assert.Contains(t, stats.ByStatus, model.AsyncTaskStatusStat{Status: string(model.TaskStatusInProgress), Count: 1})
	assert.Contains(t, stats.ByPlatform, model.AsyncTaskPlatformStat{Platform: "48", Count: 1})
	assert.Contains(t, stats.ByPlatform, model.AsyncTaskPlatformStat{Platform: "54", Count: 1})
	assert.Contains(t, stats.ByAction, model.AsyncTaskActionStat{Action: constant.TaskActionVideoGeneration, Count: 1})
	assert.Contains(t, stats.ByAction, model.AsyncTaskActionStat{Action: constant.TaskActionVideoEdit, Count: 1})
	assert.Contains(t, stats.ByChannel, model.AsyncTaskChannelStat{ChannelID: 1, Count: 2})
}

func TestAsyncTaskOptionUpdateAppliesRuntimeSetting(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))

	require.NoError(t, model.UpdateOption("async_task_setting.default_timeout_minutes", "45"))
	require.NoError(t, model.UpdateOption("async_task_setting.query_limit", "77"))
	require.NoError(t, model.UpdateOption("async_task_setting.timeout_overrides", `[{"platform":"48","action":"videoGeneration","timeout_minutes":12,"enabled":true}]`))

	setting := async_task_setting.GetAsyncTaskSetting()
	assert.Equal(t, 45, setting.DefaultTimeoutMinutes)
	assert.Equal(t, 77, setting.QueryLimit)
	assert.Equal(t, 45, constant.TaskTimeoutMinutes)
	assert.Equal(t, 77, constant.TaskQueryLimit)
	assert.Equal(t, 12, async_task_setting.ResolveTimeoutMinutes(constant.TaskPlatform("48"), constant.TaskActionVideoGeneration))

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, "45", common.OptionMap["async_task_setting.default_timeout_minutes"])
	assert.Equal(t, "77", common.OptionMap["async_task_setting.query_limit"])
	assert.JSONEq(t, `[{"platform":"48","action":"videoGeneration","timeout_minutes":12,"enabled":true}]`, common.OptionMap["async_task_setting.timeout_overrides"])
}

func TestAsyncTaskEnvFallbackBeforeOptionOverride(t *testing.T) {
	truncate(t)
	resetAsyncTaskSettingForTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))

	constant.TaskTimeoutMinutes = 55
	constant.TaskQueryLimit = 66
	model.InitOptionMap()

	setting := async_task_setting.GetAsyncTaskSetting()
	assert.Equal(t, 55, setting.DefaultTimeoutMinutes)
	assert.Equal(t, 66, setting.QueryLimit)
	assert.Equal(t, "55", common.OptionMap["async_task_setting.default_timeout_minutes"])
	assert.Equal(t, "66", common.OptionMap["async_task_setting.query_limit"])

	require.NoError(t, model.UpdateOption("async_task_setting.default_timeout_minutes", "22"))
	require.NoError(t, model.UpdateOption("async_task_setting.query_limit", "33"))

	setting = async_task_setting.GetAsyncTaskSetting()
	assert.Equal(t, 22, setting.DefaultTimeoutMinutes)
	assert.Equal(t, 33, setting.QueryLimit)
	assert.Equal(t, 22, constant.TaskTimeoutMinutes)
	assert.Equal(t, 33, constant.TaskQueryLimit)
}
