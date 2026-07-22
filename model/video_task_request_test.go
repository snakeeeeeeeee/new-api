package model

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupVideoTaskRequestTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Task{}, &VideoTaskRequest{}))
	t.Cleanup(func() {
		DB = originalDB
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createPublicVideoTaskFixture(t *testing.T, userID int, taskID, operation, reference string, idempotencyKey *string) *Task {
	t.Helper()
	now := time.Now().Unix()
	action := normalizedVideoOperationAction(operation)
	task := &Task{
		TaskID: taskID, UserId: userID, Platform: constant.TaskPlatform("48"), Action: action,
		Status: TaskStatusQueued, Progress: "20%", SubmitTime: now,
		Properties: Properties{OriginModelName: "video-model", AssetType: constant.TaskAssetTypeVideo, Operation: operation},
	}
	require.NoError(t, DB.Create(task).Error)
	request := NewVideoTaskRequest(task, userID, idempotencyKey, "fingerprint-"+taskID, reference, []byte(`{"model":"video-model"}`))
	require.NoError(t, DB.Create(request).Error)
	return task
}

func TestVideoTaskRequestIdempotencyIsUniquePerUser(t *testing.T) {
	setupVideoTaskRequestTestDB(t)
	key := "same-key"
	createPublicVideoTaskFixture(t, 1, "task_user_one", "generation", "order-1", &key)

	second := &Task{TaskID: "task_user_one_duplicate", UserId: 1, Status: TaskStatusQueued}
	require.NoError(t, DB.Create(second).Error)
	duplicate := NewVideoTaskRequest(second, 1, &key, "other-fingerprint", "", []byte(`{}`))
	require.Error(t, DB.Create(duplicate).Error)

	createPublicVideoTaskFixture(t, 2, "task_user_two", "generation", "order-2", &key)
	request, exists, err := GetVideoTaskRequestByIdempotencyKey(2, key)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "task_user_two", request.TaskID)
}

func TestVideoTaskRequestConcurrentIdempotencyReservation(t *testing.T) {
	db := setupVideoTaskRequestTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	const workers = 12
	key := "concurrent-key"
	var successes atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			err := db.Transaction(func(tx *gorm.DB) error {
				task := &Task{TaskID: fmt.Sprintf("task_concurrent_%d", index), UserId: 1, Status: TaskStatusQueued}
				if err := tx.Create(task).Error; err != nil {
					return err
				}
				request := NewVideoTaskRequest(task, 1, &key, "same-fingerprint", "", []byte(`{}`))
				return tx.Create(request).Error
			})
			if err == nil {
				successes.Add(1)
			}
		}(i)
	}
	wait.Wait()
	assert.EqualValues(t, 1, successes.Load())
	var requestCount, taskCount int64
	require.NoError(t, db.Model(&VideoTaskRequest{}).Count(&requestCount).Error)
	require.NoError(t, db.Model(&Task{}).Count(&taskCount).Error)
	assert.EqualValues(t, 1, requestCount)
	assert.EqualValues(t, 1, taskCount)
}

func TestPublicVideoTaskQueriesEnforceOwnershipAndCursor(t *testing.T) {
	setupVideoTaskRequestTestDB(t)
	first := createPublicVideoTaskFixture(t, 1, "task_first", "generation", "order-a", nil)
	second := createPublicVideoTaskFixture(t, 1, "task_second", "edit", "order-b", nil)
	createPublicVideoTaskFixture(t, 2, "task_other_user", "edit", "order-b", nil)

	loaded, exists, err := GetPublicVideoTask(1, "task_other_user")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, loaded)

	edits, hasMore, err := ListPublicVideoTasks(1, VideoTaskListQuery{Operation: "edit", Limit: 10})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, edits, 1)
	assert.Equal(t, second.ID, edits[0].ID)

	tasks, hasMore, err := ListPublicVideoTasks(1, VideoTaskListQuery{Limit: 1})
	require.NoError(t, err)
	require.True(t, hasMore)
	require.Len(t, tasks, 1)
	assert.Equal(t, second.TaskID, tasks[0].TaskID)
	tasks, hasMore, err = ListPublicVideoTasks(1, VideoTaskListQuery{Limit: 1, AfterTaskID: second.TaskID})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, tasks, 1)
	assert.Equal(t, first.TaskID, tasks[0].TaskID)
}
