package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newVideoProxyTaskContext(t *testing.T, role int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctxRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(ctxRecorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_other/content", nil)
	ctx.Set("role", role)
	ctx.Set("id", 1)
	return ctx
}

func TestGetVideoProxyTaskAdminCanReadAnyUserTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_other",
		UserId:    2,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleAdminUser)
	task, exists, err := getVideoProxyTask(ctx, 1, "task_other")

	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, 2, task.UserId)
}

func TestGetVideoProxyTaskCommonUserCannotReadOtherUserTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_other",
		UserId:    2,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleCommonUser)
	_, exists, err := getVideoProxyTask(ctx, 1, "task_other")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetVideoProxyTaskCommonUserCannotReadBlockedTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_blocked",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		IsBlocked: true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleCommonUser)
	_, exists, err := getVideoProxyTask(ctx, 1, "task_blocked")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestGetVideoProxyTaskAdminCanReadBlockedTask(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_blocked",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		IsBlocked: true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	ctx := newVideoProxyTaskContext(t, common.RoleAdminUser)
	task, exists, err := getVideoProxyTask(ctx, 1, "task_blocked")

	require.NoError(t, err)
	require.True(t, exists)
	assert.True(t, task.IsBlocked)
}

func TestUpdateTaskBlockStatusTogglesTaskAndRecordsLog(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_toggle",
		UserId:    1,
		Status:    model.TaskStatusSuccess,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/task/task_toggle/block", bytes.NewReader([]byte(`{"is_blocked":true}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_toggle"}}
	ctx.Set("id", 99)

	UpdateTaskBlockStatus(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var task model.Task
	require.NoError(t, db.Where("task_id = ?", "task_toggle").First(&task).Error)
	assert.True(t, task.IsBlocked)

	var log model.Log
	require.NoError(t, db.Where("user_id = ? and type = ?", 1, model.LogTypeManage).First(&log).Error)
	assert.Contains(t, log.Content, "屏蔽任务记录")
	assert.Contains(t, log.Content, "任务ID：task_toggle")
}
