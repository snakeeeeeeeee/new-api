package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createImageTaskAttemptFixture(t *testing.T, suffix string) (*Task, *ImageTaskRetryState, *ImageTaskAttempt) {
	t.Helper()
	truncateTables(t)
	now := time.Now().Unix()
	parent := &Task{
		TaskID: "task_retry_" + suffix, Platform: constant.TaskPlatform("58"),
		Action: constant.TaskActionImageGeneration, UserId: 1, ChannelId: 11,
		Status: TaskStatusQueued, Progress: "0%", SubmitTime: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, DB.Create(parent).Error)
	state := NewImageTaskRetryState(parent, 1, "default", "default", "gpt-image-2")
	state.CurrentRouteGroup = "fallback"
	require.NoError(t, DB.Create(state).Error)
	attempt := NewImageTaskAttempt(parent, 1, "task_attempt_"+suffix, 22, "fallback", 1, "primary", "gpt-image-2", "mapped-image", 250, &TaskBillingContext{OriginModelName: "gpt-image-2", RouteQuota: 250}, "req_retry")
	require.NoError(t, DB.Create(attempt).Error)
	state.ActiveAttemptRecordID = attempt.ID
	state.AttemptCount = 1
	require.NoError(t, DB.Save(state).Error)
	return parent, state, attempt
}

func TestNewImageCredentialLeaseForAttemptUsesAttemptChannel(t *testing.T) {
	parent, _, attempt := createImageTaskAttemptFixture(t, "lease_channel")

	lease := NewImageCredentialLeaseForAttempt(parent, attempt, "generation", attempt.UpstreamModel, 60)

	require.NotNil(t, lease.AttemptRecordID)
	assert.Equal(t, attempt.ID, *lease.AttemptRecordID)
	assert.Equal(t, attempt.ChannelID, lease.ChannelID)
	assert.NotEqual(t, parent.ChannelId, lease.ChannelID)
}

func TestPersistImageTaskAttemptSubmitResultUpdatesOnlyActiveAttempt(t *testing.T) {
	parent, _, attempt := createImageTaskAttemptFixture(t, "submit")

	updated, err := PersistImageTaskAttemptSubmitResult(attempt.ID, "provider_attempt_1", []byte(`{"ok":true}`))
	require.NoError(t, err)
	require.True(t, updated)

	stored, err := GetImageTaskAttemptByID(attempt.ID)
	require.NoError(t, err)
	assert.Equal(t, ImageTaskAttemptSubmitted, stored.Status)
	assert.Equal(t, "provider_attempt_1", stored.ProviderTaskID)
	assert.JSONEq(t, `{"ok":true}`, stored.CallbackData)

	storedParent, err := GetTaskByRecordID(parent.ID)
	require.NoError(t, err)
	assert.Empty(t, storedParent.PrivateData.UpstreamTaskID)

	_, _, _, active, err := GetActiveImageTaskAttemptByID(attempt.ID)
	require.NoError(t, err)
	assert.True(t, active)

	_, _, err = CloseActiveImageTaskAttemptNonRetryable(attempt.ID, "submit_failed", "closed", nil)
	require.NoError(t, err)
	updated, err = PersistImageTaskAttemptSubmitResult(attempt.ID, "provider_late", nil)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestCloseActiveImageTaskAttemptNonRetryableClosesExecutionOnly(t *testing.T) {
	parent, state, attempt := createImageTaskAttemptFixture(t, "close")
	lease := NewImageCredentialLeaseForAttempt(parent, attempt, "generation", attempt.UpstreamModel, 60)
	require.NoError(t, DB.Create(lease).Error)

	returnedParent, ignored, err := CloseActiveImageTaskAttemptNonRetryable(attempt.ID, "dispatch_rejected", "invalid request", []byte(`{"error":"invalid"}`))
	require.NoError(t, err)
	require.False(t, ignored)
	require.NotNil(t, returnedParent)
	assert.Equal(t, parent.ID, returnedParent.ID)
	assert.EqualValues(t, TaskStatusQueued, returnedParent.Status)

	storedAttempt, err := GetImageTaskAttemptByID(attempt.ID)
	require.NoError(t, err)
	assert.Equal(t, ImageTaskAttemptFailed, storedAttempt.Status)
	assert.Equal(t, "dispatch_rejected", storedAttempt.ErrorCode)
	assert.False(t, storedAttempt.ErrorRetryable)

	storedState, exists, err := GetImageTaskRetryStateByTaskRecordID(parent.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, ImageTaskRetryStateFailed, storedState.Status)
	assert.Zero(t, storedState.ActiveAttemptRecordID)
	assert.Contains(t, storedState.FailedChannelIDs, attempt.ChannelID)
	assert.Greater(t, storedState.Version, state.Version)

	storedLease, exists, err := GetImageCredentialLeaseByLeaseID(lease.LeaseID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, ImageCredentialLeaseStatusFailed, storedLease.Status)

	storedParent, err := GetTaskByRecordID(parent.ID)
	require.NoError(t, err)
	assert.EqualValues(t, TaskStatusQueued, storedParent.Status)
}
