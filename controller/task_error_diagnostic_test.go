package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskToDtoSeparatesAdministratorAndUserVideoErrors(t *testing.T) {
	const reason = "Account 0a6a4490-e535-4796-8fd4-f14153baacea has 1546 available credits but needs 4536"
	task := &model.Task{
		TaskID: "task_admin_error", Action: constant.TaskActionVideoGeneration,
		Status: model.TaskStatusFailure, FailReason: reason,
		PrivateData: model.TaskPrivateData{LastUpstreamStatus: http.StatusOK},
	}
	task.SetData(map[string]any{
		"error": map[string]any{"code": "insufficient_credits", "message": reason},
	})

	admin := taskToDto(task, true)
	require.NotNil(t, admin.UpstreamError)
	assert.Equal(t, "insufficient_credits", admin.UpstreamError.Code)
	assert.Contains(t, admin.FailReason, "1546 available credits")
	assert.NotContains(t, admin.FailReason, "0a6a4490")
	assert.Empty(t, admin.Data)

	user := taskToDto(task, false)
	assert.Nil(t, user.UpstreamError)
	assert.Equal(t, "Generation capacity is temporarily unavailable for this request. Try again later or reduce the duration or resolution.", user.FailReason)
	assert.Empty(t, user.Data)
}
