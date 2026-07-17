package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	WebhookEndpointEnabled    = "enabled"
	WebhookEndpointDisabled   = "disabled"
	WebhookDeliveryPending    = "pending"
	WebhookDeliveryProcessing = "processing"
	WebhookDeliveryDelivered  = "delivered"
	WebhookDeliveryFailed     = "failed"
	WebhookDeliveryDiscarded  = "discarded"
)

type WebhookEndpoint struct {
	ID                      int64          `json:"id" gorm:"primaryKey"`
	EndpointID              string         `json:"endpoint_id" gorm:"type:varchar(191);uniqueIndex"`
	UserID                  int            `json:"user_id" gorm:"index"`
	ConfigOwnerID           *int           `json:"-" gorm:"uniqueIndex:idx_webhook_config_owner"`
	Name                    string         `json:"name" gorm:"type:varchar(100)"`
	URL                     string         `json:"url" gorm:"type:text"`
	Status                  string         `json:"status" gorm:"type:varchar(20);index"`
	EventTypes              string         `json:"event_types" gorm:"type:text"`
	APIVersion              string         `json:"api_version" gorm:"type:varchar(20)"`
	AuthKeyEncrypted        string         `json:"-" gorm:"type:text"`
	SecretSalt              string         `json:"-" gorm:"type:varchar(191)"`
	SecretVersion           int            `json:"secret_version"`
	PreviousSecretSalt      string         `json:"-" gorm:"type:varchar(191)"`
	PreviousSecretVersion   int            `json:"-"`
	PreviousSecretExpiresAt int64          `json:"previous_secret_expires_at,omitempty"`
	CreatedAt               int64          `json:"created_at" gorm:"index"`
	UpdatedAt               int64          `json:"updated_at"`
	DeletedAt               gorm.DeletedAt `json:"-" gorm:"index"`
}

type WebhookEvent struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	EventID    string `json:"event_id" gorm:"type:varchar(191);uniqueIndex"`
	UserID     int    `json:"user_id" gorm:"index"`
	EventType  string `json:"event_type" gorm:"type:varchar(64);uniqueIndex:idx_webhook_event_object,priority:3"`
	ObjectType string `json:"object_type" gorm:"type:varchar(40);uniqueIndex:idx_webhook_event_object,priority:1"`
	ObjectID   string `json:"object_id" gorm:"type:varchar(191);uniqueIndex:idx_webhook_event_object,priority:2"`
	APIVersion string `json:"api_version" gorm:"type:varchar(20)"`
	Payload    string `json:"payload" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
}

type WebhookDelivery struct {
	ID               int64  `json:"id" gorm:"primaryKey"`
	DeliveryID       string `json:"delivery_id" gorm:"type:varchar(191);uniqueIndex"`
	EventRecordID    int64  `json:"event_record_id" gorm:"index;uniqueIndex:idx_webhook_delivery_target,priority:1"`
	EndpointRecordID int64  `json:"endpoint_record_id" gorm:"index;uniqueIndex:idx_webhook_delivery_target,priority:2"`
	Status           string `json:"status" gorm:"type:varchar(20);index:idx_webhook_delivery_due,priority:1"`
	Attempts         int    `json:"attempts"`
	NextAttemptAt    int64  `json:"next_attempt_at" gorm:"index:idx_webhook_delivery_due,priority:2"`
	LockedUntil      int64  `json:"locked_until" gorm:"index"`
	LockToken        string `json:"-" gorm:"type:varchar(64);index"`
	RetryDeadline    int64  `json:"retry_deadline" gorm:"index"`
	LastHTTPStatus   int    `json:"last_http_status"`
	LastError        string `json:"last_error" gorm:"type:text"`
	DeliveredAt      int64  `json:"delivered_at"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
	UpdatedAt        int64  `json:"updated_at"`
}

type WebhookDeliveryAttempt struct {
	ID               int64  `json:"id" gorm:"primaryKey"`
	AttemptID        string `json:"attempt_id" gorm:"type:varchar(191);uniqueIndex"`
	DeliveryRecordID int64  `json:"delivery_record_id" gorm:"index"`
	AttemptNumber    int    `json:"attempt_number"`
	HTTPStatus       int    `json:"http_status"`
	Error            string `json:"error" gorm:"type:text"`
	ResponseBody     string `json:"response_body" gorm:"type:text"`
	DurationMS       int64  `json:"duration_ms"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
}

func webhookPublicID(prefix string) string {
	key, _ := common.GenerateRandomCharsKey(32)
	return prefix + key
}

func NewWebhookEndpointID() string { return webhookPublicID("we_") }
func NewWebhookEventID() string    { return webhookPublicID("evt_") }
func NewWebhookDeliveryID() string { return webhookPublicID("whd_") }
func NewWebhookAttemptID() string  { return webhookPublicID("wha_") }

func NewWebhookDelivery(eventID, endpointID int64) *WebhookDelivery {
	now := time.Now().Unix()
	return &WebhookDelivery{
		DeliveryID:       NewWebhookDeliveryID(),
		EventRecordID:    eventID,
		EndpointRecordID: endpointID,
		Status:           WebhookDeliveryPending,
		NextAttemptAt:    now,
		RetryDeadline:    now + 24*60*60,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func CountWebhookEndpoints(userID int) (int64, error) {
	var total int64
	err := DB.Model(&WebhookEndpoint{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func ListWebhookEndpoints(userID int) ([]*WebhookEndpoint, error) {
	var endpoints []*WebhookEndpoint
	err := DB.Where("user_id = ?", userID).Order("id DESC").Find(&endpoints).Error
	return endpoints, err
}

func GetWebhookEndpoint(userID int, endpointID string) (*WebhookEndpoint, bool, error) {
	var endpoint WebhookEndpoint
	err := DB.Where("user_id = ? AND endpoint_id = ?", userID, endpointID).First(&endpoint).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &endpoint, true, nil
}

func GetWebhookEndpointByRecordID(id int64) (*WebhookEndpoint, error) {
	var endpoint WebhookEndpoint
	if err := DB.First(&endpoint, id).Error; err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func GetAccountWebhookEndpoint(userID int) (*WebhookEndpoint, bool, error) {
	var endpoint WebhookEndpoint
	err := DB.Where("config_owner_id = ? AND user_id = ?", userID, userID).First(&endpoint).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &endpoint, true, nil
}

func ClaimDueWebhookDeliveries(limit int, leaseSeconds int64) ([]*WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}
	if leaseSeconds < 30 {
		leaseSeconds = 30
	}
	now := time.Now().Unix()
	var candidates []WebhookDelivery
	if err := DB.Where("next_attempt_at <= ? AND status = ?", now, WebhookDeliveryPending).
		Order("next_attempt_at ASC, id ASC").Limit(limit * 2).Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]*WebhookDelivery, 0, limit)
	for i := range candidates {
		if len(claimed) >= limit {
			break
		}
		candidate := &candidates[i]
		lockToken, err := common.GenerateRandomCharsKey(32)
		if err != nil {
			return nil, err
		}
		result := DB.Model(&WebhookDelivery{}).
			Where("id = ? AND next_attempt_at <= ? AND status = ?", candidate.ID, now, WebhookDeliveryPending).
			Updates(map[string]any{
				"status":       WebhookDeliveryProcessing,
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

func LoadWebhookDeliveryContext(deliveryID int64) (*WebhookDelivery, *WebhookEvent, *WebhookEndpoint, error) {
	var delivery WebhookDelivery
	if err := DB.First(&delivery, deliveryID).Error; err != nil {
		return nil, nil, nil, err
	}
	var event WebhookEvent
	if err := DB.First(&event, delivery.EventRecordID).Error; err != nil {
		return nil, nil, nil, err
	}
	var endpoint WebhookEndpoint
	if err := DB.Unscoped().First(&endpoint, delivery.EndpointRecordID).Error; err != nil {
		return nil, nil, nil, err
	}
	return &delivery, &event, &endpoint, nil
}

type WebhookDeliveryResult struct {
	Status          string
	NextAttemptAt   int64
	HTTPStatus      int
	LastError       string
	ResponseBody    string
	DurationMS      int64
	DisableEndpoint bool
}

func CompleteWebhookDeliveryAttempt(delivery *WebhookDelivery, result WebhookDeliveryResult) (bool, error) {
	if delivery == nil || delivery.ID == 0 || delivery.LockToken == "" {
		return false, fmt.Errorf("invalid webhook delivery lease")
	}
	now := time.Now().Unix()
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":           result.Status,
			"next_attempt_at":  result.NextAttemptAt,
			"locked_until":     0,
			"lock_token":       "",
			"last_http_status": result.HTTPStatus,
			"last_error":       result.LastError,
			"updated_at":       now,
		}
		if result.Status == WebhookDeliveryDelivered {
			updates["delivered_at"] = now
		}
		updated := tx.Model(&WebhookDelivery{}).
			Where("id = ? AND status = ? AND lock_token = ?", delivery.ID, WebhookDeliveryProcessing, delivery.LockToken).
			Updates(updates)
		if updated.Error != nil || updated.RowsAffected == 0 {
			return updated.Error
		}
		won = true
		attempt := &WebhookDeliveryAttempt{
			AttemptID: NewWebhookAttemptID(), DeliveryRecordID: delivery.ID, AttemptNumber: delivery.Attempts,
			HTTPStatus: result.HTTPStatus, Error: result.LastError, ResponseBody: result.ResponseBody,
			DurationMS: result.DurationMS, CreatedAt: now,
		}
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		if result.DisableEndpoint {
			return tx.Model(&WebhookEndpoint{}).Where("id = ?", delivery.EndpointRecordID).Updates(map[string]any{
				"status": WebhookEndpointDisabled, "updated_at": now,
			}).Error
		}
		return nil
	})
	return won, err
}

func ListWebhookDeliveryAttempts(deliveryRecordID int64) ([]*WebhookDeliveryAttempt, error) {
	var attempts []*WebhookDeliveryAttempt
	err := DB.Where("delivery_record_id = ?", deliveryRecordID).Order("attempt_number DESC, id DESC").Find(&attempts).Error
	return attempts, err
}

func ResetWebhookDeliveryForRetry(userID int, deliveryID string) (*WebhookDelivery, error) {
	var delivery WebhookDelivery
	err := DB.Table("webhook_deliveries").
		Select("webhook_deliveries.*").
		Joins("JOIN webhook_endpoints ON webhook_endpoints.id = webhook_deliveries.endpoint_record_id").
		Where("webhook_deliveries.delivery_id = ? AND webhook_endpoints.user_id = ? AND webhook_endpoints.deleted_at IS NULL", deliveryID, userID).
		First(&delivery).Error
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	result := DB.Model(&delivery).Where("status IN ?", []string{WebhookDeliveryFailed, WebhookDeliveryDiscarded}).Updates(map[string]any{
		"status": WebhookDeliveryPending, "attempts": 0, "next_attempt_at": now,
		"locked_until": 0, "lock_token": "", "retry_deadline": now + 24*60*60,
		"last_error": "", "updated_at": now,
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("only failed or discarded deliveries can be retried")
	}
	if err := DB.First(&delivery, delivery.ID).Error; err != nil {
		return nil, err
	}
	return &delivery, nil
}

type WebhookDeliveryListQuery struct {
	EndpointID string
	Status     string
	EventType  string
	AfterID    string
	Limit      int
}

func ListUserWebhookDeliveries(userID int, params WebhookDeliveryListQuery) ([]*WebhookDelivery, bool, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := DB.Table("webhook_deliveries").
		Select("webhook_deliveries.*").
		Joins("JOIN webhook_endpoints ON webhook_endpoints.id = webhook_deliveries.endpoint_record_id").
		Joins("JOIN webhook_events ON webhook_events.id = webhook_deliveries.event_record_id").
		Where("webhook_endpoints.user_id = ?", userID)
	if params.EndpointID != "" {
		query = query.Where("webhook_endpoints.endpoint_id = ?", params.EndpointID)
	}
	if params.Status != "" {
		query = query.Where("webhook_deliveries.status = ?", params.Status)
	}
	if params.EventType != "" {
		query = query.Where("webhook_events.event_type = ?", params.EventType)
	}
	if params.AfterID != "" {
		var cursor WebhookDelivery
		if err := DB.Table("webhook_deliveries").Select("webhook_deliveries.*").
			Joins("JOIN webhook_endpoints ON webhook_endpoints.id = webhook_deliveries.endpoint_record_id").
			Where("webhook_deliveries.delivery_id = ? AND webhook_endpoints.user_id = ?", params.AfterID, userID).
			First(&cursor).Error; err != nil {
			return nil, false, err
		}
		query = query.Where("webhook_deliveries.id < ?", cursor.ID)
	}
	var deliveries []*WebhookDelivery
	if err := query.Order("webhook_deliveries.id DESC").Limit(limit + 1).Find(&deliveries).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(deliveries) > limit
	if hasMore {
		deliveries = deliveries[:limit]
	}
	return deliveries, hasMore, nil
}

func GetUserWebhookDelivery(userID int, deliveryID string) (*WebhookDelivery, *WebhookEvent, *WebhookEndpoint, bool, error) {
	var delivery WebhookDelivery
	err := DB.Table("webhook_deliveries").
		Select("webhook_deliveries.*").
		Joins("JOIN webhook_endpoints ON webhook_endpoints.id = webhook_deliveries.endpoint_record_id").
		Where("webhook_deliveries.delivery_id = ? AND webhook_endpoints.user_id = ?", deliveryID, userID).
		First(&delivery).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, nil, false, err
	}
	var event WebhookEvent
	if err := DB.First(&event, delivery.EventRecordID).Error; err != nil {
		return nil, nil, nil, false, err
	}
	var endpoint WebhookEndpoint
	if err := DB.Unscoped().First(&endpoint, delivery.EndpointRecordID).Error; err != nil {
		return nil, nil, nil, false, err
	}
	return &delivery, &event, &endpoint, true, nil
}

func CleanupWebhookLogs(cutoff int64, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	var deliveries []WebhookDelivery
	if err := DB.Where("created_at < ?", cutoff).Order("id ASC").Limit(batchSize).Find(&deliveries).Error; err != nil {
		return 0, err
	}
	if len(deliveries) == 0 {
		return 0, nil
	}
	ids := make([]int64, 0, len(deliveries))
	for i := range deliveries {
		ids = append(ids, deliveries[i].ID)
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("delivery_record_id IN ?", ids).Delete(&WebhookDeliveryAttempt{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", ids).Delete(&WebhookDelivery{}).Error; err != nil {
			return err
		}
		return tx.Where("created_at < ? AND id NOT IN (?)", cutoff,
			tx.Model(&WebhookDelivery{}).Select("event_record_id")).Delete(&WebhookEvent{}).Error
	})
	return len(ids), err
}
