package service

import (
	"strconv"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func BuildOpenAIVideoCompatibilityTask(task *model.Task) (*dto.OpenAIVideo, error) {
	items, err := BuildOpenAIVideoCompatibilityTasks([]*model.Task{task})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func BuildOpenAIVideoCompatibilityTasks(tasks []*model.Task) ([]*dto.OpenAIVideo, error) {
	publicTasks, err := BuildPublicVideoTasks(tasks)
	if err != nil {
		return nil, err
	}
	publicByID := make(map[string]*dto.VideoTaskPublic, len(publicTasks))
	for _, public := range publicTasks {
		publicByID[public.ID] = public
	}
	result := make([]*dto.OpenAIVideo, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		public := publicByID[task.TaskID]
		if public == nil {
			continue
		}
		video := dto.NewOpenAIVideo()
		video.ID = public.ID
		video.Model = public.Model
		switch public.Status {
		case "succeeded":
			video.Status = dto.VideoStatusCompleted
		case "in_progress":
			video.Status = dto.VideoStatusInProgress
		case "failed":
			video.Status = dto.VideoStatusFailed
		default:
			video.Status = dto.VideoStatusQueued
		}
		video.Progress = public.Progress
		video.CreatedAt = public.CreatedAt
		video.Metadata = public.Metadata
		if public.CompletedAt != nil {
			video.CompletedAt = *public.CompletedAt
		}
		if public.Error != nil {
			video.Error = &dto.OpenAIVideoError{Code: public.Error.Code, Message: public.Error.Message}
		}
		if compatibility := task.PrivateData.OpenAIVideoCompatibility; compatibility != nil {
			if compatibility.Seconds > 0 {
				video.Seconds = strconv.Itoa(compatibility.Seconds)
			}
			video.Size = compatibility.Size
			video.RemixedFromVideoID = compatibility.RemixedFromVideo
		}
		result = append(result, video)
	}
	return result, nil
}

func IsOpenAIVideoCompatibilityTask(task *model.Task) bool {
	return task != nil && task.PrivateData.OpenAIVideoCompatibility != nil &&
		task.PrivateData.OpenAIVideoCompatibility.Version == dto.OpenAIVideoCompatibilityVersion
}

func IsOpenAIVideoCompatibilityDeleted(task *model.Task) bool {
	return IsOpenAIVideoCompatibilityTask(task) && task.PrivateData.OpenAIVideoCompatibility.DeletedAt > 0
}
