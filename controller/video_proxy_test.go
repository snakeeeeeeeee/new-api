package controller

import (
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
