package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ImageCredentialLeaseStatusActive   = "active"
	ImageCredentialLeaseStatusResolved = "resolved"
	ImageCredentialLeaseStatusFailed   = "failed"
	ImageCredentialLeaseStatusExpired  = "expired"
)

type ImageCredentialLease struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	LeaseID      string `json:"lease_id" gorm:"type:varchar(191);uniqueIndex"`
	TaskID       string `json:"task_id" gorm:"type:varchar(191);index"`
	TaskRecordID int64  `json:"task_record_id" gorm:"index"`
	UserID       int    `json:"user_id" gorm:"index"`
	ChannelID    int    `json:"channel_id" gorm:"index"`
	Operation    string `json:"operation" gorm:"type:varchar(40)"`
	Model        string `json:"model" gorm:"type:varchar(191)"`
	Status       string `json:"status" gorm:"type:varchar(20);index"`
	ExpiresAt    int64  `json:"expires_at" gorm:"index"`
	ResolvedAt   int64  `json:"resolved_at"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at"`
}

func GenerateImageCredentialLeaseID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "lease_" + key
}

func NewImageCredentialLease(task *Task, operation string, modelName string, ttlSeconds int64) *ImageCredentialLease {
	now := time.Now().Unix()
	if ttlSeconds <= 0 {
		ttlSeconds = 1800
	}
	return &ImageCredentialLease{
		LeaseID:      GenerateImageCredentialLeaseID(),
		TaskID:       task.TaskID,
		TaskRecordID: task.ID,
		UserID:       task.UserId,
		ChannelID:    task.ChannelId,
		Operation:    operation,
		Model:        modelName,
		Status:       ImageCredentialLeaseStatusActive,
		ExpiresAt:    now + ttlSeconds,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func CreateImageCredentialLease(lease *ImageCredentialLease) error {
	if lease == nil {
		return fmt.Errorf("lease is nil")
	}
	now := time.Now().Unix()
	if lease.CreatedAt == 0 {
		lease.CreatedAt = now
	}
	lease.UpdatedAt = now
	return DB.Create(lease).Error
}

func GetImageCredentialLeaseByLeaseID(leaseID string) (*ImageCredentialLease, bool, error) {
	var lease ImageCredentialLease
	err := DB.Where("lease_id = ?", leaseID).First(&lease).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &lease, true, nil
}

func MarkImageCredentialLeaseResolved(lease *ImageCredentialLease) error {
	if lease == nil {
		return fmt.Errorf("lease is nil")
	}
	now := time.Now().Unix()
	updates := map[string]any{
		"status":      ImageCredentialLeaseStatusResolved,
		"resolved_at": now,
		"updated_at":  now,
	}
	return DB.Model(lease).Where("lease_id = ?", lease.LeaseID).Updates(updates).Error
}

func MarkImageCredentialLeaseFailed(leaseID string) error {
	return updateImageCredentialLeaseStatus(leaseID, ImageCredentialLeaseStatusFailed)
}

func MarkImageCredentialLeaseExpired(leaseID string) error {
	return updateImageCredentialLeaseStatus(leaseID, ImageCredentialLeaseStatusExpired)
}

func updateImageCredentialLeaseStatus(leaseID string, status string) error {
	if leaseID == "" {
		return nil
	}
	now := time.Now().Unix()
	return DB.Model(&ImageCredentialLease{}).Where("lease_id = ?", leaseID).Updates(map[string]any{
		"status":     status,
		"updated_at": now,
	}).Error
}
