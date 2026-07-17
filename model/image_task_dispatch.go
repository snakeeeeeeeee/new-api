package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ImageTaskDispatchPending    = "pending"
	ImageTaskDispatchProcessing = "processing"
	ImageTaskDispatchDelivered  = "delivered"
	ImageTaskDispatchFailed     = "failed"
)

type ImageTaskDispatch struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	DispatchID     string `json:"dispatch_id" gorm:"type:varchar(191);uniqueIndex"`
	TaskRecordID   int64  `json:"task_record_id" gorm:"uniqueIndex"`
	TaskID         string `json:"task_id" gorm:"type:varchar(191);index"`
	RequestBody    string `json:"request_body" gorm:"type:text"`
	Status         string `json:"status" gorm:"type:varchar(20);index:idx_image_dispatch_due,priority:1"`
	Attempts       int    `json:"attempts"`
	NextAttemptAt  int64  `json:"next_attempt_at" gorm:"index:idx_image_dispatch_due,priority:2"`
	LockedUntil    int64  `json:"locked_until" gorm:"index"`
	LockToken      string `json:"lock_token" gorm:"type:varchar(64);index"`
	LastHTTPStatus int    `json:"last_http_status"`
	LastError      string `json:"last_error" gorm:"type:text"`
	DeliveredAt    int64  `json:"delivered_at"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
	UpdatedAt      int64  `json:"updated_at"`
}

func ClaimDueImageTaskDispatches(limit int, leaseSeconds int64) ([]*ImageTaskDispatch, error) {
	if limit <= 0 {
		limit = 20
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	now := time.Now().Unix()
	var candidates []ImageTaskDispatch
	err := DB.Where("next_attempt_at <= ? AND (locked_until = 0 OR locked_until < ?) AND status IN ?", now, now,
		[]string{ImageTaskDispatchPending, ImageTaskDispatchProcessing}).
		Order("next_attempt_at ASC, id ASC").Limit(limit * 2).Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	claimed := make([]*ImageTaskDispatch, 0, limit)
	for i := range candidates {
		if len(claimed) >= limit {
			break
		}
		candidate := &candidates[i]
		lockToken, err := common.GenerateRandomCharsKey(32)
		if err != nil {
			return nil, err
		}
		result := DB.Model(&ImageTaskDispatch{}).
			Where("id = ? AND next_attempt_at <= ? AND (locked_until = 0 OR locked_until < ?) AND status IN ?", candidate.ID, now, now,
				[]string{ImageTaskDispatchPending, ImageTaskDispatchProcessing}).
			Updates(map[string]any{
				"status":       ImageTaskDispatchProcessing,
				"attempts":     gorm.Expr("attempts + 1"),
				"locked_until": now + leaseSeconds,
				"lock_token":   lockToken,
				"updated_at":   now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		if err := DB.First(candidate, candidate.ID).Error; err != nil {
			return nil, err
		}
		claimed = append(claimed, candidate)
	}
	return claimed, nil
}

func MarkImageTaskDispatchDelivered(id int64, lockToken string, httpStatus int) error {
	now := time.Now().Unix()
	result := DB.Model(&ImageTaskDispatch{}).Where("id = ? AND lock_token = ?", id, lockToken).Updates(map[string]any{
		"status":           ImageTaskDispatchDelivered,
		"last_http_status": httpStatus,
		"last_error":       "",
		"locked_until":     0,
		"lock_token":       "",
		"delivered_at":     now,
		"updated_at":       now,
	})
	return claimedDispatchUpdateError(result)
}

func RescheduleImageTaskDispatch(id int64, lockToken string, httpStatus int, reason string, delay time.Duration) error {
	if delay < time.Second {
		delay = time.Second
	}
	now := time.Now().Unix()
	result := DB.Model(&ImageTaskDispatch{}).Where("id = ? AND lock_token = ?", id, lockToken).Updates(map[string]any{
		"status":           ImageTaskDispatchPending,
		"last_http_status": httpStatus,
		"last_error":       reason,
		"next_attempt_at":  now + int64(delay/time.Second),
		"locked_until":     0,
		"lock_token":       "",
		"updated_at":       now,
	})
	return claimedDispatchUpdateError(result)
}

func MarkImageTaskDispatchFailed(id int64, lockToken string, httpStatus int, reason string) error {
	now := time.Now().Unix()
	result := DB.Model(&ImageTaskDispatch{}).Where("id = ? AND lock_token = ?", id, lockToken).Updates(map[string]any{
		"status":           ImageTaskDispatchFailed,
		"last_http_status": httpStatus,
		"last_error":       reason,
		"locked_until":     0,
		"lock_token":       "",
		"updated_at":       now,
	})
	return claimedDispatchUpdateError(result)
}

func claimedDispatchUpdateError(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("image task dispatch lease is no longer owned")
	}
	return nil
}

func GetTaskByRecordID(id int64) (*Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid task record id")
	}
	var task Task
	if err := DB.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func GenerateImageTaskDispatchID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "dispatch_" + key
}

func NewImageTaskDispatch(task *Task, body []byte) *ImageTaskDispatch {
	now := time.Now().Unix()
	return &ImageTaskDispatch{
		DispatchID:    GenerateImageTaskDispatchID(),
		TaskRecordID:  task.ID,
		TaskID:        task.TaskID,
		RequestBody:   string(body),
		Status:        ImageTaskDispatchPending,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
