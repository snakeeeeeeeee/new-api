package model

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

type VideoTaskRequest struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	TaskRecordID       int64   `json:"task_record_id" gorm:"uniqueIndex"`
	TaskID             string  `json:"task_id" gorm:"type:varchar(191);uniqueIndex"`
	UserID             int     `json:"user_id" gorm:"index;uniqueIndex:idx_video_task_idempotency,priority:1"`
	IdempotencyKey     *string `json:"idempotency_key,omitempty" gorm:"type:varchar(191);uniqueIndex:idx_video_task_idempotency,priority:2"`
	RequestFingerprint string  `json:"request_fingerprint" gorm:"type:varchar(64)"`
	ClientReferenceID  string  `json:"client_reference_id" gorm:"type:varchar(191);index"`
	RequestJSON        string  `json:"request_json" gorm:"type:text"`
	CreatedAt          int64   `json:"created_at" gorm:"index"`
	UpdatedAt          int64   `json:"updated_at"`
}

func NewVideoTaskRequest(task *Task, userID int, idempotencyKey *string, fingerprint, clientReferenceID string, requestJSON []byte) *VideoTaskRequest {
	now := time.Now().Unix()
	return &VideoTaskRequest{
		TaskRecordID: task.ID, TaskID: task.TaskID, UserID: userID,
		IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint,
		ClientReferenceID: clientReferenceID, RequestJSON: string(requestJSON),
		CreatedAt: now, UpdatedAt: now,
	}
}

func GetVideoTaskRequestByIdempotencyKey(userID int, key string) (*VideoTaskRequest, bool, error) {
	var request VideoTaskRequest
	err := DB.Where("user_id = ? AND idempotency_key = ?", userID, key).First(&request).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &request, true, nil
}

type VideoTaskListQuery struct {
	Statuses          []TaskStatus
	Operation         string
	ClientReferenceID string
	CreatedAfter      int64
	CreatedBefore     int64
	AfterTaskID       string
	Limit             int
}

func GetPublicVideoTask(userID int, taskID string) (*Task, bool, error) {
	var task Task
	err := visibleTaskQuery(DB.Model(&Task{})).
		Joins("JOIN video_task_requests ON video_task_requests.task_record_id = tasks.id").
		Where("tasks.user_id = ? AND tasks.task_id = ?", userID, strings.TrimSpace(taskID)).
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &task, true, nil
}

func ListPublicVideoTasks(userID int, params VideoTaskListQuery) ([]*Task, bool, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := visibleTaskQuery(DB.Model(&Task{})).
		Joins("JOIN video_task_requests ON video_task_requests.task_record_id = tasks.id").
		Where("tasks.user_id = ?", userID)
	if len(params.Statuses) > 0 {
		query = query.Where("tasks.status IN ?", params.Statuses)
	}
	if params.Operation != "" {
		query = query.Where("tasks.action = ?", normalizedVideoOperationAction(params.Operation))
	}
	if params.ClientReferenceID != "" {
		query = query.Where("video_task_requests.client_reference_id = ?", params.ClientReferenceID)
	}
	if params.CreatedAfter > 0 {
		query = query.Where("tasks.created_at >= ?", params.CreatedAfter)
	}
	if params.CreatedBefore > 0 {
		query = query.Where("tasks.created_at <= ?", params.CreatedBefore)
	}
	if after := strings.TrimSpace(params.AfterTaskID); after != "" {
		var cursor VideoTaskRequest
		if err := DB.Where("user_id = ? AND task_id = ?", userID, after).First(&cursor).Error; err != nil {
			return nil, false, err
		}
		query = query.Where("tasks.id < ?", cursor.TaskRecordID)
	}
	var tasks []*Task
	if err := query.Order("tasks.id DESC").Limit(limit + 1).Find(&tasks).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(tasks) > limit
	if hasMore {
		tasks = tasks[:limit]
	}
	return tasks, hasMore, nil
}

func GetPublicVideoTasksByIDs(userID int, taskIDs []string) ([]*Task, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	var tasks []*Task
	err := visibleTaskQuery(DB.Model(&Task{})).
		Joins("JOIN video_task_requests ON video_task_requests.task_record_id = tasks.id").
		Where("tasks.user_id = ? AND tasks.task_id IN ?", userID, taskIDs).
		Find(&tasks).Error
	return tasks, err
}

func GetVideoTaskRequestsByTaskIDs(userID int, taskIDs []string) ([]*VideoTaskRequest, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	var requests []*VideoTaskRequest
	err := DB.Where("user_id = ? AND task_id IN ?", userID, taskIDs).Find(&requests).Error
	return requests, err
}

func normalizedVideoOperationAction(operation string) string {
	switch operation {
	case "edit":
		return constant.TaskActionVideoEdit
	case "extension":
		return constant.TaskActionVideoExtension
	case "remix":
		return constant.TaskActionRemix
	default:
		return constant.TaskActionVideoGeneration
	}
}
