package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestTaskModel2DtoOnlyReturnsResultURLForSuccessfulTask(t *testing.T) {
	success := &model.Task{
		TaskID: "task_success",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://vidgen.x.ai/video.mp4",
		},
	}
	assert.Equal(t, "https://vidgen.x.ai/video.mp4", TaskModel2Dto(success).ResultURL)

	failure := &model.Task{
		TaskID:     "task_failure",
		Status:     model.TaskStatusFailure,
		FailReason: "Generated video rejected by content moderation.",
	}
	assert.Empty(t, TaskModel2Dto(failure).ResultURL)
}
