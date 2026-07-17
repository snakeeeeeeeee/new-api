package model

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestImageTaskDispatchLeaseTokenFencesStaleWorker(t *testing.T) {
	require.NoError(t, DB.Where("task_id = ?", "task_dispatch_fencing").Delete(&ImageTaskDispatch{}).Error)
	dispatch := &ImageTaskDispatch{
		DispatchID:    "dispatch_fencing",
		TaskRecordID:  991001,
		TaskID:        "task_dispatch_fencing",
		RequestBody:   `{}`,
		Status:        ImageTaskDispatchPending,
		NextAttemptAt: time.Now().Unix(),
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(dispatch).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", dispatch.ID).Delete(&ImageTaskDispatch{}).Error
	})

	first, err := ClaimDueImageTaskDispatches(1, 60)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.NotEmpty(t, first[0].LockToken)

	require.NoError(t, DB.Model(&ImageTaskDispatch{}).Where("id = ?", dispatch.ID).Update("locked_until", time.Now().Unix()-1).Error)
	second, err := ClaimDueImageTaskDispatches(1, 60)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.NotEqual(t, first[0].LockToken, second[0].LockToken)

	require.Error(t, MarkImageTaskDispatchDelivered(dispatch.ID, first[0].LockToken, 202))
	require.NoError(t, RescheduleImageTaskDispatch(dispatch.ID, second[0].LockToken, 500, "retry", time.Second))

	var stored ImageTaskDispatch
	require.NoError(t, DB.First(&stored, dispatch.ID).Error)
	require.Equal(t, ImageTaskDispatchPending, stored.Status)
	require.Empty(t, stored.LockToken)
}

func TestConcurrentImageTaskIdempotencyCreatesOneDurableRecordSet(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ImageTaskRequest{}, &ImageTaskDispatch{}))
	const userID = 4201
	const idempotencyKeyValue = "same-concurrent-key"
	const taskPrefix = "task_concurrent_idempotency_"
	cleanup := func() {
		_ = DB.Where("task_id LIKE ?", taskPrefix+"%").Delete(&ImageTaskDispatch{}).Error
		_ = DB.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKeyValue).Delete(&ImageTaskRequest{}).Error
		_ = DB.Where("task_id LIKE ?", taskPrefix+"%").Delete(&Task{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	const requests = 16
	errorsByRequest := make([]error, requests)
	var waitGroup sync.WaitGroup
	for index := 0; index < requests; index++ {
		index := index
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errorsByRequest[index] = DB.Transaction(func(tx *gorm.DB) error {
				now := time.Now().Unix()
				task := &Task{
					TaskID: fmt.Sprintf("%s%d", taskPrefix, index), UserId: userID, ChannelId: 7,
					Platform: constant.TaskPlatform("58"), Action: constant.TaskActionImageGeneration,
					Quota: 777, Status: TaskStatusQueued, Progress: "0%", SubmitTime: now, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Create(task).Error; err != nil {
					return err
				}
				idempotencyKey := idempotencyKeyValue
				request := NewImageTaskRequest(task, userID, &idempotencyKey, "same-fingerprint", "", []byte(`{"model":"gpt-image-2"}`))
				if err := tx.Create(request).Error; err != nil {
					return err
				}
				return tx.Create(NewImageTaskDispatch(task, []byte(`{"client_task_id":"`+task.TaskID+`"}`))).Error
			})
		}()
	}
	waitGroup.Wait()

	successes := 0
	for _, err := range errorsByRequest {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var taskCount, requestCount, dispatchCount int64
	require.NoError(t, DB.Model(&Task{}).Where("task_id LIKE ?", taskPrefix+"%").Count(&taskCount).Error)
	require.NoError(t, DB.Model(&ImageTaskRequest{}).Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKeyValue).Count(&requestCount).Error)
	require.NoError(t, DB.Model(&ImageTaskDispatch{}).Where("task_id LIKE ?", taskPrefix+"%").Count(&dispatchCount).Error)
	require.EqualValues(t, 1, taskCount)
	require.EqualValues(t, 1, requestCount)
	require.EqualValues(t, 1, dispatchCount)
}
