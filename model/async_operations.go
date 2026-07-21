package model

import (
	"database/sql"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const AsyncOperationsRecentWindowSeconds int64 = 3600

var ErrWebhookDeliveryNotRetryable = errors.New("only failed or discarded deliveries can be retried")

type AsyncQueueStats struct {
	Pending             int64 `json:"pending"`
	Due                 int64 `json:"due"`
	Processing          int64 `json:"processing"`
	Stale               int64 `json:"stale"`
	Failed              int64 `json:"failed"`
	Discarded           int64 `json:"discarded,omitempty"`
	CompletedRecent     int64 `json:"completed_recent"`
	OldestDueAgeSeconds int64 `json:"oldest_due_age_seconds"`
}

func GetImageTaskDispatchQueueStats(now int64) (AsyncQueueStats, error) {
	stats := AsyncQueueStats{}
	queries := []struct {
		target *int64
		query  *gorm.DB
	}{
		{&stats.Pending, DB.Model(&ImageTaskDispatch{}).Where("status = ?", ImageTaskDispatchPending)},
		{&stats.Due, DB.Model(&ImageTaskDispatch{}).Where("status = ? AND next_attempt_at <= ?", ImageTaskDispatchPending, now)},
		{&stats.Processing, DB.Model(&ImageTaskDispatch{}).Where("status = ? AND locked_until >= ?", ImageTaskDispatchProcessing, now)},
		{&stats.Stale, DB.Model(&ImageTaskDispatch{}).Where("status = ? AND (locked_until = 0 OR locked_until < ?)", ImageTaskDispatchProcessing, now)},
		{&stats.Failed, DB.Model(&ImageTaskDispatch{}).Where("status = ?", ImageTaskDispatchFailed)},
		{&stats.CompletedRecent, DB.Model(&ImageTaskDispatch{}).Where("status = ? AND delivered_at >= ?", ImageTaskDispatchDelivered, now-AsyncOperationsRecentWindowSeconds)},
	}
	for _, item := range queries {
		if err := item.query.Count(item.target).Error; err != nil {
			return stats, err
		}
	}
	oldest, err := oldestDueTimestamp(DB.Model(&ImageTaskDispatch{}).
		Where("(status = ? AND next_attempt_at <= ?) OR (status = ? AND (locked_until = 0 OR locked_until < ?))",
			ImageTaskDispatchPending, now, ImageTaskDispatchProcessing, now))
	if err != nil {
		return stats, err
	}
	if oldest > 0 && oldest < now {
		stats.OldestDueAgeSeconds = now - oldest
	}
	return stats, nil
}

func GetWebhookDeliveryQueueStats(now int64) (AsyncQueueStats, error) {
	stats := AsyncQueueStats{}
	queries := []struct {
		target *int64
		query  *gorm.DB
	}{
		{&stats.Pending, DB.Model(&WebhookDelivery{}).Where("status = ?", WebhookDeliveryPending)},
		{&stats.Due, DB.Model(&WebhookDelivery{}).Where("status = ? AND next_attempt_at <= ?", WebhookDeliveryPending, now)},
		{&stats.Processing, DB.Model(&WebhookDelivery{}).Where("status = ? AND locked_until >= ?", WebhookDeliveryProcessing, now)},
		{&stats.Stale, DB.Model(&WebhookDelivery{}).Where("status = ? AND (locked_until = 0 OR locked_until < ?)", WebhookDeliveryProcessing, now)},
		{&stats.Failed, DB.Model(&WebhookDelivery{}).Where("status = ?", WebhookDeliveryFailed)},
		{&stats.Discarded, DB.Model(&WebhookDelivery{}).Where("status = ?", WebhookDeliveryDiscarded)},
		{&stats.CompletedRecent, DB.Model(&WebhookDelivery{}).Where("status = ? AND delivered_at >= ?", WebhookDeliveryDelivered, now-AsyncOperationsRecentWindowSeconds)},
	}
	for _, item := range queries {
		if err := item.query.Count(item.target).Error; err != nil {
			return stats, err
		}
	}
	oldest, err := oldestDueTimestamp(DB.Model(&WebhookDelivery{}).
		Where("(status = ? AND next_attempt_at <= ?) OR (status = ? AND (locked_until = 0 OR locked_until < ?))",
			WebhookDeliveryPending, now, WebhookDeliveryProcessing, now))
	if err != nil {
		return stats, err
	}
	if oldest > 0 && oldest < now {
		stats.OldestDueAgeSeconds = now - oldest
	}
	return stats, nil
}

func oldestDueTimestamp(query *gorm.DB) (int64, error) {
	var value sql.NullInt64
	if err := query.Select("MIN(next_attempt_at)").Scan(&value).Error; err != nil {
		return 0, err
	}
	if !value.Valid {
		return 0, nil
	}
	return value.Int64, nil
}

type AdminAsyncTaskQuery struct {
	SyncTaskQueryParams
	DispatchStatus string
}

func ListAdminAsyncTasks(startIdx int, limit int, params AdminAsyncTaskQuery) ([]*Task, map[int64]*ImageTaskDispatch, int64, error) {
	query := applyAdminAsyncTaskFilters(DB.Model(&Task{}), params)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}
	var tasks []*Task
	if err := applyAdminAsyncTaskFilters(DB.Model(&Task{}), params).
		Select("tasks.*").Order("tasks.id DESC").Offset(startIdx).Limit(limit).Find(&tasks).Error; err != nil {
		return nil, nil, 0, err
	}
	dispatches := make(map[int64]*ImageTaskDispatch)
	if len(tasks) == 0 {
		return tasks, dispatches, total, nil
	}
	taskRecordIDs := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		taskRecordIDs = append(taskRecordIDs, task.ID)
	}
	var rows []*ImageTaskDispatch
	if err := DB.Where("task_record_id IN ?", taskRecordIDs).Find(&rows).Error; err != nil {
		return nil, nil, 0, err
	}
	for _, dispatch := range rows {
		dispatches[dispatch.TaskRecordID] = dispatch
	}
	return tasks, dispatches, total, nil
}

func applyAdminAsyncTaskFilters(query *gorm.DB, params AdminAsyncTaskQuery) *gorm.DB {
	if params.DispatchStatus != "" {
		query = query.Joins("LEFT JOIN image_task_dispatches ON image_task_dispatches.task_record_id = tasks.id")
		if params.DispatchStatus == "none" {
			query = query.Where("image_task_dispatches.id IS NULL")
		} else {
			query = query.Where("image_task_dispatches.status = ?", params.DispatchStatus)
		}
	}
	filters := params.SyncTaskQueryParams
	if filters.ChannelID != "" {
		query = query.Where("tasks.channel_id = ?", filters.ChannelID)
	}
	if filters.Platform != "" {
		query = query.Where("tasks.platform = ?", filters.Platform)
	}
	if filters.UserID != "" {
		query = query.Where("tasks.user_id = ?", filters.UserID)
	}
	if filters.TaskID != "" {
		query = query.Where("tasks.task_id = ?", filters.TaskID)
	}
	if filters.Action != "" {
		query = query.Where("tasks.action = ?", filters.Action)
	} else if filters.AssetType != "" {
		if actions := constant.TaskActionsByAssetType(filters.AssetType); len(actions) > 0 {
			query = query.Where("tasks.action IN ?", actions)
		}
	}
	if filters.Status != "" {
		query = query.Where("tasks.status = ?", filters.Status)
	}
	if filters.StartTimestamp != 0 {
		query = query.Where("tasks.submit_time >= ?", filters.StartTimestamp)
	}
	if filters.EndTimestamp != 0 {
		query = query.Where("tasks.submit_time <= ?", filters.EndTimestamp)
	}
	return query
}

type AdminWebhookDeliveryQuery struct {
	Status         string
	EventType      string
	DeliveryID     string
	UserID         int
	HTTPStatus     int
	StartTimestamp int64
	EndTimestamp   int64
}

func ListAdminWebhookDeliveries(startIdx int, limit int, params AdminWebhookDeliveryQuery) ([]*WebhookDelivery, map[int64]*WebhookEvent, map[int64]*WebhookEndpoint, int64, error) {
	query := applyAdminWebhookDeliveryFilters(adminWebhookDeliveryQuery(), params)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	var deliveries []*WebhookDelivery
	if err := applyAdminWebhookDeliveryFilters(adminWebhookDeliveryQuery(), params).
		Select("webhook_deliveries.*").Order("webhook_deliveries.id DESC").Offset(startIdx).Limit(limit).Find(&deliveries).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	events, endpoints, err := loadWebhookDeliveryRelations(deliveries)
	return deliveries, events, endpoints, total, err
}

func adminWebhookDeliveryQuery() *gorm.DB {
	return DB.Table("webhook_deliveries").
		Joins("JOIN webhook_events ON webhook_events.id = webhook_deliveries.event_record_id").
		Joins("JOIN webhook_endpoints ON webhook_endpoints.id = webhook_deliveries.endpoint_record_id")
}

func applyAdminWebhookDeliveryFilters(query *gorm.DB, params AdminWebhookDeliveryQuery) *gorm.DB {
	if params.Status != "" {
		query = query.Where("webhook_deliveries.status = ?", params.Status)
	}
	if params.EventType != "" {
		query = query.Where("webhook_events.event_type = ?", params.EventType)
	}
	if params.DeliveryID != "" {
		query = query.Where("webhook_deliveries.delivery_id = ?", params.DeliveryID)
	}
	if params.UserID > 0 {
		query = query.Where("webhook_endpoints.user_id = ?", params.UserID)
	}
	if params.HTTPStatus > 0 {
		query = query.Where("webhook_deliveries.last_http_status = ?", params.HTTPStatus)
	}
	if params.StartTimestamp > 0 {
		query = query.Where("webhook_deliveries.created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp > 0 {
		query = query.Where("webhook_deliveries.created_at <= ?", params.EndTimestamp)
	}
	return query
}

func loadWebhookDeliveryRelations(deliveries []*WebhookDelivery) (map[int64]*WebhookEvent, map[int64]*WebhookEndpoint, error) {
	events := make(map[int64]*WebhookEvent)
	endpoints := make(map[int64]*WebhookEndpoint)
	if len(deliveries) == 0 {
		return events, endpoints, nil
	}
	eventIDs := make([]int64, 0, len(deliveries))
	endpointIDs := make([]int64, 0, len(deliveries))
	for _, delivery := range deliveries {
		eventIDs = append(eventIDs, delivery.EventRecordID)
		endpointIDs = append(endpointIDs, delivery.EndpointRecordID)
	}
	var eventRows []*WebhookEvent
	if err := DB.Where("id IN ?", eventIDs).Find(&eventRows).Error; err != nil {
		return nil, nil, err
	}
	var endpointRows []*WebhookEndpoint
	if err := DB.Unscoped().Where("id IN ?", endpointIDs).Find(&endpointRows).Error; err != nil {
		return nil, nil, err
	}
	for _, event := range eventRows {
		events[event.ID] = event
	}
	for _, endpoint := range endpointRows {
		endpoints[endpoint.ID] = endpoint
	}
	return events, endpoints, nil
}

func GetAdminWebhookDelivery(deliveryID string) (*WebhookDelivery, *WebhookEvent, *WebhookEndpoint, bool, error) {
	var delivery WebhookDelivery
	if err := DB.Where("delivery_id = ?", deliveryID).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, false, nil
		}
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

func ResetWebhookDeliveryForAdminRetry(deliveryID string) (*WebhookDelivery, error) {
	var delivery WebhookDelivery
	if err := DB.Where("delivery_id = ?", deliveryID).First(&delivery).Error; err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	result := DB.Model(&WebhookDelivery{}).
		Where("id = ? AND status IN ?", delivery.ID, []string{WebhookDeliveryFailed, WebhookDeliveryDiscarded}).
		Updates(map[string]any{
			"status": WebhookDeliveryPending, "attempts": 0, "next_attempt_at": now,
			"locked_until": 0, "lock_token": "", "retry_deadline": now + 24*60*60,
			"last_http_status": 0, "last_error": "", "delivered_at": 0, "updated_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrWebhookDeliveryNotRetryable
	}
	if err := DB.First(&delivery, delivery.ID).Error; err != nil {
		return nil, err
	}
	return &delivery, nil
}
