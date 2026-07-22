package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageHandleTaskPlatform() constant.TaskPlatform {
	return constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeImageHandle))
}

func createPollingRefundTask(
	t *testing.T,
	taskID string,
	upstreamTaskID string,
	platform constant.TaskPlatform,
	userID int,
	channelID int,
	quota int,
	tokenID int,
	billingSource string,
	subscriptionID int,
) *model.Task {
	t.Helper()
	task := makeTask(userID, channelID, quota, tokenID, billingSource, subscriptionID)
	task.TaskID = taskID
	task.Platform = platform
	task.Status = model.TaskStatusQueued
	task.Progress = "0%"
	task.SubmitTime = time.Now().Unix()
	task.PrivateData.UpstreamTaskID = upstreamTaskID
	if platform == imageHandleTaskPlatform() && task.PrivateData.BillingContext != nil {
		task.PrivateData.BillingContext.RequestId = "req-" + taskID
	}
	require.NoError(t, model.DB.Create(task).Error)
	if platform == imageHandleTaskPlatform() {
		logItem := seedAsyncImageConsumeLog(t, task, "")
		require.NoError(t, model.PersistTaskSubmitResult(task.ID, upstreamTaskID, nil, logItem.Id))
	}
	return task
}

func loadPollingRefundTask(t *testing.T, id int64) *model.Task {
	t.Helper()
	var task model.Task
	require.NoError(t, model.DB.First(&task, id).Error)
	return &task
}

func TestFailImageHandleTaskWithRefundUsesCASAndRefundsWalletAndTokenOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 301, 301, 9301
	const initialUserQuota, initialTokenQuota, prechargedQuota = 20000, 12000, 4000
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-polling-missing-upstream", initialTokenQuota)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, prechargedQuota, 1)
	setChannelUsedQuota(t, channelID, prechargedQuota)

	task := createPollingRefundTask(
		t, "task_public_missing_upstream", "", imageHandleTaskPlatform(),
		userID, channelID, prechargedQuota, tokenID, BillingSourceWallet, 0,
	)
	now := time.Now().Unix()
	task.SubmitTime = now - imageHandleMissingUpstreamIDGraceSeconds - 1
	require.NoError(t, model.DB.Model(task).Update("submit_time", task.SubmitTime).Error)
	staleTask := loadPollingRefundTask(t, task.ID)

	upstreamID, missing := resolveTaskPollingUpstreamID(task, now)
	assert.Empty(t, upstreamID)
	require.True(t, missing, "public task ID must not mask a missing image-handle upstream ID")
	if missing {
		failImageHandleTaskWithRefund(ctx, task, "image-handle task has no upstream task ID")
	}
	_, missing = resolveTaskPollingUpstreamID(staleTask, now)
	if missing {
		failImageHandleTaskWithRefund(ctx, staleTask, "image-handle task has no upstream task ID")
	}

	reloaded := loadPollingRefundTask(t, task.ID)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
	assert.Contains(t, reloaded.FailReason, "no upstream task ID")
	assert.NotZero(t, reloaded.FinishTime)
	assert.Equal(t, initialUserQuota+prechargedQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota+prechargedQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -prechargedQuota, getTokenUsedQuota(t, tokenID))
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Zero(t, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Zero(t, getChannelUsedQuota(t, channelID))
	assert.Equal(t, int64(1), countLogs(t))
	logItem := getLastLog(t)
	require.NotNil(t, logItem)
	assert.Equal(t, model.LogTypeConsume, logItem.Type)
	assert.Zero(t, logItem.Quota)
	assert.Equal(t, "req-task_public_missing_upstream", logItem.RequestId)
}

func TestResolveTaskPollingUpstreamIDPreservesSubmitGraceAndLegacyFallback(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name           string
		task           *model.Task
		wantUpstreamID string
		wantMissing    bool
	}{
		{
			name: "image-handle missing private ID remains pending during submit grace",
			task: &model.Task{
				TaskID:     "task_public_within_grace",
				Platform:   imageHandleTaskPlatform(),
				SubmitTime: now - imageHandleMissingUpstreamIDGraceSeconds + 1,
			},
			wantUpstreamID: "",
			wantMissing:    false,
		},
		{
			name: "image-handle private ID is authoritative",
			task: &model.Task{
				TaskID:     "task_public_with_private_id",
				Platform:   imageHandleTaskPlatform(),
				SubmitTime: now - imageHandleMissingUpstreamIDGraceSeconds - 1,
				PrivateData: model.TaskPrivateData{
					UpstreamTaskID: "  imgtask_provider_id  ",
				},
			},
			wantUpstreamID: "imgtask_provider_id",
			wantMissing:    false,
		},
		{
			name: "legacy non-image task keeps public task ID fallback",
			task: &model.Task{
				TaskID:     "legacy_upstream_task_id",
				Platform:   constant.TaskPlatform("48"),
				SubmitTime: now - imageHandleMissingUpstreamIDGraceSeconds - 1,
			},
			wantUpstreamID: "legacy_upstream_task_id",
			wantMissing:    false,
		},
		{
			name:           "nil task is missing",
			task:           nil,
			wantUpstreamID: "",
			wantMissing:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamID, missing := resolveTaskPollingUpstreamID(tt.task, now)
			assert.Equal(t, tt.wantUpstreamID, upstreamID)
			assert.Equal(t, tt.wantMissing, missing)
		})
	}
}

func TestImageHandleChannelReadFailureUsesCASAndRefundsSubscriptionAndTokenOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, subscriptionID, missingChannelID = 302, 302, 302, 9302
	const initialUserQuota, initialTokenQuota, prechargedQuota = 500, 9000, 3000
	const subscriptionTotal, initialSubscriptionUsed int64 = 100000, 42000
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-polling-channel-read", initialTokenQuota)
	seedSubscription(t, subscriptionID, userID, subscriptionTotal, initialSubscriptionUsed)
	setUserUsageCounters(t, userID, prechargedQuota, 1)

	const upstreamTaskID = "imgtask_channel_read_failure"
	task := createPollingRefundTask(
		t, "task_public_channel_read_failure", upstreamTaskID, imageHandleTaskPlatform(),
		userID, missingChannelID, prechargedQuota, tokenID, BillingSourceSubscription, subscriptionID,
	)
	staleTask := loadPollingRefundTask(t, task.ID)

	err := updateVideoTasks(ctx, imageHandleTaskPlatform(), missingChannelID, []string{upstreamTaskID}, map[string]*model.Task{
		upstreamTaskID: task,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CacheGetChannel failed")

	err = updateVideoTasks(ctx, imageHandleTaskPlatform(), missingChannelID, []string{upstreamTaskID}, map[string]*model.Task{
		upstreamTaskID: staleTask,
	})
	require.Error(t, err)

	reloaded := loadPollingRefundTask(t, task.ID)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
	assert.Contains(t, reloaded.FailReason, fmt.Sprintf("channel ID: %d", missingChannelID))
	assert.NotZero(t, reloaded.FinishTime)
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialSubscriptionUsed-int64(prechargedQuota), getSubscriptionUsed(t, subscriptionID))
	assert.Equal(t, initialTokenQuota+prechargedQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -prechargedQuota, getTokenUsedQuota(t, tokenID))
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Zero(t, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(1), countLogs(t))
	logItem := getLastLog(t)
	require.NotNil(t, logItem)
	assert.Equal(t, model.LogTypeConsume, logItem.Type)
	assert.Zero(t, logItem.Quota)
}

func TestNonImageHandleChannelReadFailureKeepsLegacyBulkFailureWithoutRefund(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, missingChannelID = 303, 303, 9303
	const initialUserQuota, initialTokenQuota, prechargedQuota = 15000, 7000, 2500
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-polling-legacy-channel-read", initialTokenQuota)

	const upstreamTaskID = "upstream_legacy_channel_read_failure"
	legacyPlatform := constant.TaskPlatform("48")
	task := createPollingRefundTask(
		t, "task_public_legacy_channel_read_failure", upstreamTaskID, legacyPlatform,
		userID, missingChannelID, prechargedQuota, tokenID, BillingSourceWallet, 0,
	)

	err := updateVideoTasks(ctx, legacyPlatform, missingChannelID, []string{upstreamTaskID}, map[string]*model.Task{
		upstreamTaskID: task,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CacheGetChannel failed")

	reloaded := loadPollingRefundTask(t, task.ID)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
	assert.Contains(t, reloaded.FailReason, fmt.Sprintf("channel ID: %d", missingChannelID))
	assert.Zero(t, reloaded.FinishTime)
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestFailImageHandleTaskWithRefundIgnoresOtherPlatforms(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 304, 304, 9304
	const initialUserQuota, initialTokenQuota, prechargedQuota = 10000, 6000, 2000
	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-polling-non-image-guard", initialTokenQuota)

	task := createPollingRefundTask(
		t, "task_public_non_image_guard", "", constant.TaskPlatform("48"),
		userID, channelID, prechargedQuota, tokenID, BillingSourceWallet, 0,
	)

	failImageHandleTaskWithRefund(ctx, task, "must not apply")

	reloaded := loadPollingRefundTask(t, task.ID)
	assert.EqualValues(t, model.TaskStatusQueued, reloaded.Status)
	assert.Equal(t, "0%", reloaded.Progress)
	assert.Empty(t, reloaded.FailReason)
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(0), countLogs(t))
}
