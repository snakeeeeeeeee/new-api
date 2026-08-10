package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ImageTaskRetryStateActive    = "active"
	ImageTaskRetryStateSucceeded = "succeeded"
	ImageTaskRetryStateFailed    = "failed"
	ImageTaskRetryStateExhausted = "exhausted"

	ImageTaskAttemptPending    = "pending"
	ImageTaskAttemptDispatched = "dispatched"
	ImageTaskAttemptSubmitted  = "submitted"
	ImageTaskAttemptQueued     = "queued"
	ImageTaskAttemptProcessing = "processing"
	ImageTaskAttemptSucceeded  = "succeeded"
	ImageTaskAttemptFailed     = "failed"
)

var ErrImageTaskAttemptProviderTaskIDMismatch = errors.New("image task attempt provider task ID mismatch")

type ImageTaskRouteTraceEntry struct {
	AttemptNumber int    `json:"attempt_number"`
	ChannelID     int    `json:"channel_id"`
	RouteGroup    string `json:"route_group,omitempty"`
	RouteIndex    int    `json:"route_index,omitempty"`
	RoutePool     string `json:"route_pool,omitempty"`
	Status        string `json:"status"`
	ErrorCode     string `json:"error_code,omitempty"`
	CreatedAt     int64  `json:"created_at"`
}

// ImageTaskRetryState is the durable routing state for one public normalized image task.
// JSON slices use GORM's JSON serializer over TEXT so all supported databases share one schema.
type ImageTaskRetryState struct {
	ID                      int64                      `json:"id" gorm:"primaryKey"`
	TaskRecordID            int64                      `json:"task_record_id" gorm:"uniqueIndex"`
	TaskID                  string                     `json:"task_id" gorm:"type:varchar(191);uniqueIndex"`
	Status                  string                     `json:"status" gorm:"type:varchar(20);index"`
	RetryLimit              int                        `json:"retry_limit"`
	TokenGroup              string                     `json:"token_group" gorm:"type:varchar(64);index"`
	UserGroup               string                     `json:"user_group" gorm:"type:varchar(64)"`
	OriginalModel           string                     `json:"original_model" gorm:"type:varchar(191)"`
	CrossGroupRetry         bool                       `json:"cross_group_retry"`
	LockedChannel           bool                       `json:"locked_channel"`
	InitialChannelID        int                        `json:"initial_channel_id" gorm:"index"`
	InitialRouteGroup       string                     `json:"initial_route_group" gorm:"type:varchar(64)"`
	PrechargedQuota         int                        `json:"precharged_quota"`
	ChannelQuotaTransferred bool                       `json:"channel_quota_transferred"`
	AggregateGroup          string                     `json:"aggregate_group" gorm:"type:varchar(64);index"`
	RoutingMode             string                     `json:"routing_mode" gorm:"type:varchar(32)"`
	CurrentRouteGroup       string                     `json:"current_route_group" gorm:"type:varchar(64)"`
	CurrentRouteIndex       int                        `json:"current_route_index"`
	CurrentRoutePool        string                     `json:"current_route_pool" gorm:"type:varchar(64)"`
	CurrentGroupAttempts    int                        `json:"current_group_attempts"`
	AttemptCount            int                        `json:"attempt_count"`
	ActiveAttemptRecordID   int64                      `json:"active_attempt_record_id" gorm:"index"`
	FailedChannelIDs        []int                      `json:"failed_channel_ids" gorm:"serializer:json;type:text"`
	AttemptedRouteKeys      []string                   `json:"attempted_route_keys" gorm:"serializer:json;type:text"`
	RouteTrace              []ImageTaskRouteTraceEntry `json:"route_trace" gorm:"serializer:json;type:text"`
	Version                 int64                      `json:"version"`
	CreatedAt               int64                      `json:"created_at" gorm:"index"`
	UpdatedAt               int64                      `json:"updated_at"`
}

// ImageTaskAttempt is one image-handle submission on one real channel.
type ImageTaskAttempt struct {
	ID               int64              `json:"id" gorm:"primaryKey"`
	AttemptID        string             `json:"attempt_id" gorm:"type:varchar(191);uniqueIndex"`
	ClientTaskID     string             `json:"client_task_id" gorm:"type:varchar(191);uniqueIndex"`
	TaskRecordID     int64              `json:"task_record_id" gorm:"index;uniqueIndex:idx_image_attempt_task_number,priority:1"`
	TaskID           string             `json:"task_id" gorm:"type:varchar(191);index"`
	AttemptNumber    int                `json:"attempt_number" gorm:"uniqueIndex:idx_image_attempt_task_number,priority:2"`
	ChannelID        int                `json:"channel_id" gorm:"index"`
	RouteGroup       string             `json:"route_group" gorm:"type:varchar(64);index"`
	RouteIndex       int                `json:"route_index"`
	RoutePool        string             `json:"route_pool" gorm:"type:varchar(64)"`
	OriginModel      string             `json:"origin_model" gorm:"type:varchar(191)"`
	UpstreamModel    string             `json:"upstream_model" gorm:"type:varchar(191)"`
	ExecutionDriver  string             `json:"execution_driver" gorm:"type:varchar(40)"`
	Status           string             `json:"status" gorm:"type:varchar(20);index"`
	ProviderTaskID   string             `json:"provider_task_id" gorm:"type:varchar(191);index"`
	ProgressSequence int64              `json:"progress_sequence"`
	ErrorCode        string             `json:"error_code" gorm:"type:varchar(191)"`
	ErrorMessage     string             `json:"error_message" gorm:"type:text"`
	ErrorRetryable   bool               `json:"error_retryable"`
	Quota            int                `json:"quota"`
	BillingContext   TaskBillingContext `json:"billing_context" gorm:"serializer:json;type:text"`
	RequestID        string             `json:"request_id" gorm:"type:varchar(191);index"`
	CallbackData     string             `json:"callback_data" gorm:"type:text"`
	TaskInfoJSON     string             `json:"task_info_json" gorm:"type:text"`
	StartedAt        int64              `json:"started_at"`
	FinishedAt       int64              `json:"finished_at"`
	CreatedAt        int64              `json:"created_at" gorm:"index"`
	UpdatedAt        int64              `json:"updated_at"`
}

func GenerateImageTaskAttemptClientID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "task_attempt_" + key
}

func NewImageTaskRetryState(task *Task, retryLimit int, tokenGroup, userGroup, originalModel string) *ImageTaskRetryState {
	now := time.Now().Unix()
	if retryLimit < 0 {
		retryLimit = 0
	}
	return &ImageTaskRetryState{
		TaskRecordID:       task.ID,
		TaskID:             task.TaskID,
		Status:             ImageTaskRetryStateActive,
		RetryLimit:         retryLimit,
		TokenGroup:         strings.TrimSpace(tokenGroup),
		UserGroup:          strings.TrimSpace(userGroup),
		OriginalModel:      strings.TrimSpace(originalModel),
		InitialChannelID:   task.ChannelId,
		InitialRouteGroup:  task.Group,
		PrechargedQuota:    task.Quota,
		CurrentRouteIndex:  -1,
		FailedChannelIDs:   []int{},
		AttemptedRouteKeys: []string{},
		RouteTrace:         []ImageTaskRouteTraceEntry{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func cloneTaskBillingContext(source *TaskBillingContext) TaskBillingContext {
	if source == nil {
		return TaskBillingContext{}
	}
	var cloned TaskBillingContext
	data, err := common.Marshal(source)
	if err != nil || common.Unmarshal(data, &cloned) != nil {
		return *source
	}
	return cloned
}

func NewImageTaskAttempt(task *Task, attemptNumber int, clientTaskID string, channelID int, routeGroup string, routeIndex int, routePool, originModel, upstreamModel string, quota int, billingContext *TaskBillingContext, requestID string) *ImageTaskAttempt {
	now := time.Now().Unix()
	if strings.TrimSpace(clientTaskID) == "" {
		clientTaskID = GenerateImageTaskAttemptClientID()
	}
	return &ImageTaskAttempt{
		AttemptID:     clientTaskID,
		ClientTaskID:  clientTaskID,
		TaskRecordID:  task.ID,
		TaskID:        task.TaskID,
		AttemptNumber: attemptNumber,
		ChannelID:     channelID,
		RouteGroup:    strings.TrimSpace(routeGroup),
		RouteIndex:    routeIndex,
		RoutePool:     strings.TrimSpace(routePool),
		OriginModel:   strings.TrimSpace(originModel),
		UpstreamModel: strings.TrimSpace(upstreamModel),
		ExecutionDriver: strings.TrimSpace(
			task.PrivateData.ImageHandleExecutionDriver,
		),
		Status:         ImageTaskAttemptPending,
		Quota:          quota,
		BillingContext: cloneTaskBillingContext(billingContext),
		RequestID:      strings.TrimSpace(requestID),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s *ImageTaskRetryState) HasFailedChannel(channelID int) bool {
	for _, failed := range s.FailedChannelIDs {
		if failed == channelID {
			return true
		}
	}
	return false
}

func (s *ImageTaskRetryState) AddFailedChannel(channelID int) {
	if channelID <= 0 || s.HasFailedChannel(channelID) {
		return
	}
	s.FailedChannelIDs = append(s.FailedChannelIDs, channelID)
}

func (s *ImageTaskRetryState) HasAttemptedRouteKey(key string) bool {
	key = strings.TrimSpace(key)
	for _, attempted := range s.AttemptedRouteKeys {
		if attempted == key {
			return true
		}
	}
	return false
}

func (s *ImageTaskRetryState) AddAttemptedRouteKey(key string) {
	key = strings.TrimSpace(key)
	if key == "" || s.HasAttemptedRouteKey(key) {
		return
	}
	s.AttemptedRouteKeys = append(s.AttemptedRouteKeys, key)
}

func (s *ImageTaskRetryState) AppendTrace(attempt *ImageTaskAttempt, status, errorCode string) {
	if attempt == nil {
		return
	}
	s.RouteTrace = append(s.RouteTrace, ImageTaskRouteTraceEntry{
		AttemptNumber: attempt.AttemptNumber,
		ChannelID:     attempt.ChannelID,
		RouteGroup:    attempt.RouteGroup,
		RouteIndex:    attempt.RouteIndex,
		RoutePool:     attempt.RoutePool,
		Status:        status,
		ErrorCode:     strings.TrimSpace(errorCode),
		CreatedAt:     time.Now().Unix(),
	})
}

func GetImageTaskRetryStateByTaskRecordID(taskRecordID int64) (*ImageTaskRetryState, bool, error) {
	var state ImageTaskRetryState
	result := DB.Where("task_record_id = ?", taskRecordID).Limit(1).Find(&state)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &state, true, nil
}

func GetImageTaskAttemptByClientTaskID(clientTaskID string) (*ImageTaskAttempt, bool, error) {
	clientTaskID = strings.TrimSpace(clientTaskID)
	if clientTaskID == "" {
		return nil, false, nil
	}
	var attempt ImageTaskAttempt
	result := DB.Where("client_task_id = ?", clientTaskID).Limit(1).Find(&attempt)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &attempt, true, nil
}

func GetImageTaskAttemptByID(id int64) (*ImageTaskAttempt, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid image task attempt id")
	}
	var attempt ImageTaskAttempt
	if err := DB.First(&attempt, id).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func GetActiveImageTaskAttemptByID(id int64) (*ImageTaskAttempt, *ImageTaskRetryState, *Task, bool, error) {
	if id <= 0 {
		return nil, nil, nil, false, fmt.Errorf("invalid image task attempt id")
	}
	var attempt ImageTaskAttempt
	if err := DB.First(&attempt, id).Error; err != nil {
		return nil, nil, nil, false, err
	}
	var state ImageTaskRetryState
	if err := DB.Where("task_record_id = ?", attempt.TaskRecordID).First(&state).Error; err != nil {
		return nil, nil, nil, false, err
	}
	parent, err := GetTaskByRecordID(attempt.TaskRecordID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	active := state.Status == ImageTaskRetryStateActive &&
		state.ActiveAttemptRecordID == attempt.ID &&
		!ImageTaskAttemptIsTerminal(attempt.Status) &&
		parent.Status != TaskStatusSuccess && parent.Status != TaskStatusFailure
	return &attempt, &state, parent, active, nil
}

func PersistImageTaskAttemptSubmitResult(attemptRecordID int64, providerTaskID string, taskData []byte) (bool, error) {
	providerTaskID = strings.TrimSpace(providerTaskID)
	if attemptRecordID <= 0 || providerTaskID == "" {
		return false, fmt.Errorf("invalid image task attempt submit result")
	}
	updated := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		attempt, state, parent, err := lockImageTaskAttemptContextTx(tx, attemptRecordID)
		if err != nil {
			return err
		}
		if state.Status != ImageTaskRetryStateActive || state.ActiveAttemptRecordID != attempt.ID ||
			ImageTaskAttemptIsTerminal(attempt.Status) ||
			parent.Status == TaskStatusSuccess || parent.Status == TaskStatusFailure {
			return nil
		}
		if attempt.ProviderTaskID != "" && attempt.ProviderTaskID != providerTaskID {
			return ErrImageTaskAttemptProviderTaskIDMismatch
		}
		updates := map[string]any{
			"provider_task_id": providerTaskID,
			"status":           ImageTaskAttemptSubmitted,
			"updated_at":       time.Now().Unix(),
		}
		if len(taskData) > 0 {
			updates["callback_data"] = string(taskData)
		}
		result := tx.Model(&ImageTaskAttempt{}).
			Where("id = ? AND status NOT IN ?", attempt.ID, []string{ImageTaskAttemptSucceeded, ImageTaskAttemptFailed}).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected > 0
		return nil
	})
	return updated, err
}

// CloseActiveImageTaskAttemptNonRetryable closes an active execution attempt
// before the public parent task is finalized by service.ApplyTaskResult.
func CloseActiveImageTaskAttemptNonRetryable(attemptRecordID int64, errorCode, errorMessage string, taskData []byte) (*Task, bool, error) {
	var parentResult *Task
	ignored := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		attempt, state, parent, err := lockImageTaskAttemptContextTx(tx, attemptRecordID)
		if err != nil {
			return err
		}
		if state.Status != ImageTaskRetryStateActive || state.ActiveAttemptRecordID != attempt.ID ||
			ImageTaskAttemptIsTerminal(attempt.Status) ||
			parent.Status == TaskStatusSuccess || parent.Status == TaskStatusFailure {
			ignored = true
			parentResult = parent
			return nil
		}

		now := time.Now().Unix()
		attempt.Status = ImageTaskAttemptFailed
		attempt.ErrorCode = strings.TrimSpace(errorCode)
		attempt.ErrorMessage = strings.TrimSpace(errorMessage)
		attempt.ErrorRetryable = false
		if len(taskData) > 0 {
			attempt.CallbackData = string(taskData)
		}
		attempt.FinishedAt = now
		attempt.UpdatedAt = now
		if err := tx.Save(attempt).Error; err != nil {
			return err
		}

		state.AddFailedChannel(attempt.ChannelID)
		state.AppendTrace(attempt, ImageTaskAttemptFailed, attempt.ErrorCode)
		state.ActiveAttemptRecordID = 0
		state.Status = ImageTaskRetryStateFailed
		state.Version++
		state.UpdatedAt = now
		if err := tx.Save(state).Error; err != nil {
			return err
		}
		if err := tx.Model(&ImageCredentialLease{}).
			Where("attempt_record_id = ? AND status IN ?", attempt.ID, []string{ImageCredentialLeaseStatusActive, ImageCredentialLeaseStatusResolved}).
			Updates(map[string]any{"status": ImageCredentialLeaseStatusFailed, "updated_at": now}).Error; err != nil {
			return err
		}
		parentResult = parent
		return nil
	})
	return parentResult, ignored, err
}

func ListUnfinalizedImageTaskRetryStates(limit int) ([]*ImageTaskRetryState, error) {
	if limit <= 0 {
		limit = 100
	}
	var states []*ImageTaskRetryState
	err := DB.Model(&ImageTaskRetryState{}).
		Select("image_task_retry_states.*").
		Joins("JOIN tasks ON tasks.id = image_task_retry_states.task_record_id").
		Where("image_task_retry_states.status IN ?", []string{ImageTaskRetryStateSucceeded, ImageTaskRetryStateFailed, ImageTaskRetryStateExhausted}).
		Where("tasks.status NOT IN ?", []TaskStatus{TaskStatusSuccess, TaskStatusFailure}).
		Order("image_task_retry_states.updated_at ASC").
		Limit(limit).
		Find(&states).Error
	return states, err
}

func GetLatestImageTaskAttempt(taskRecordID int64) (*ImageTaskAttempt, error) {
	var attempt ImageTaskAttempt
	if err := DB.Where("task_record_id = ?", taskRecordID).Order("attempt_number DESC").First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func lockImageTaskAttemptContextTx(tx *gorm.DB, attemptRecordID int64) (*ImageTaskAttempt, *ImageTaskRetryState, *Task, error) {
	var attempt ImageTaskAttempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, attemptRecordID).Error; err != nil {
		return nil, nil, nil, err
	}
	var state ImageTaskRetryState
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_record_id = ?", attempt.TaskRecordID).First(&state).Error; err != nil {
		return nil, nil, nil, err
	}
	var parent Task
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent, attempt.TaskRecordID).Error; err != nil {
		return nil, nil, nil, err
	}
	return &attempt, &state, &parent, nil
}

func ListImageTaskAttempts(taskRecordID int64) ([]*ImageTaskAttempt, error) {
	var attempts []*ImageTaskAttempt
	err := DB.Where("task_record_id = ?", taskRecordID).Order("attempt_number ASC").Find(&attempts).Error
	return attempts, err
}

func ImageTaskAttemptIsTerminal(status string) bool {
	return status == ImageTaskAttemptSucceeded || status == ImageTaskAttemptFailed
}
