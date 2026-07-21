package service

import (
	"context"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

type AsyncOperationsSection struct {
	Queue  model.AsyncQueueStats   `json:"queue"`
	Worker AsyncWorkerRuntimeStats `json:"worker"`
}

type AdminAsyncTaskStats struct {
	model.AsyncTaskStats
	RecentWindowSeconds int64                  `json:"recent_window_seconds"`
	ImageDispatch       AsyncOperationsSection `json:"image_dispatch"`
	WebhookDelivery     AsyncOperationsSection `json:"webhook_delivery"`
}

func GetAsyncTaskStats() AdminAsyncTaskStats {
	now := time.Now().Unix()
	result := AdminAsyncTaskStats{
		AsyncTaskStats:      model.GetAsyncTaskStats(now, CountTimeoutPendingTasks(now)),
		RecentWindowSeconds: model.AsyncOperationsRecentWindowSeconds,
		ImageDispatch: AsyncOperationsSection{
			Worker: GetImageTaskDispatchWorkerRuntimeStats(),
		},
		WebhookDelivery: AsyncOperationsSection{
			Worker: GetWebhookDeliveryWorkerRuntimeStats(),
		},
	}
	if queue, err := model.GetImageTaskDispatchQueueStats(now); err != nil {
		logger.LogError(context.Background(), "load image task dispatch queue stats failed: "+err.Error())
	} else {
		result.ImageDispatch.Queue = queue
	}
	if queue, err := model.GetWebhookDeliveryQueueStats(now); err != nil {
		logger.LogError(context.Background(), "load webhook delivery queue stats failed: "+err.Error())
	} else {
		result.WebhookDelivery.Queue = queue
	}
	return result
}

func ListAdminAsyncTaskRecords(startIdx int, limit int, params model.AdminAsyncTaskQuery) ([]*model.Task, map[int64]*model.ImageTaskDispatch, int64, error) {
	return model.ListAdminAsyncTasks(startIdx, limit, params)
}

func ListAdminWebhookDeliveries(startIdx int, limit int, params model.AdminWebhookDeliveryQuery) ([]*dto.AdminWebhookDelivery, int64, error) {
	deliveries, events, endpoints, total, err := model.ListAdminWebhookDeliveries(startIdx, limit, params)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*dto.AdminWebhookDelivery, 0, len(deliveries))
	usernames := make(map[int]string)
	for _, delivery := range deliveries {
		event := events[delivery.EventRecordID]
		endpoint := endpoints[delivery.EndpointRecordID]
		if event == nil || endpoint == nil {
			continue
		}
		username, loaded := usernames[endpoint.UserID]
		if !loaded {
			if user, userErr := model.GetUserCache(endpoint.UserID); userErr == nil && user != nil {
				username = user.Username
			}
			usernames[endpoint.UserID] = username
		}
		items = append(items, adminWebhookDeliveryToDTO(delivery, event, endpoint, username))
	}
	return items, total, nil
}

func GetAdminWebhookDeliveryDetail(deliveryID string) (*dto.AdminWebhookDeliveryDetail, bool, error) {
	delivery, event, endpoint, exists, err := model.GetAdminWebhookDelivery(deliveryID)
	if err != nil || !exists {
		return nil, exists, err
	}
	username := ""
	if user, userErr := model.GetUserCache(endpoint.UserID); userErr == nil && user != nil {
		username = user.Username
	}
	var payload any
	if err := common.UnmarshalJsonStr(event.Payload, &payload); err != nil {
		payload = event.Payload
	}
	attemptRows, err := model.ListWebhookDeliveryAttempts(delivery.ID)
	if err != nil {
		return nil, false, err
	}
	attempts := make([]*dto.AdminWebhookDeliveryAttempt, 0, len(attemptRows))
	for index := len(attemptRows) - 1; index >= 0; index-- {
		attempt := attemptRows[index]
		attempts = append(attempts, &dto.AdminWebhookDeliveryAttempt{
			AttemptID: attempt.AttemptID, AttemptNumber: attempt.AttemptNumber,
			HTTPStatus: attempt.HTTPStatus, Error: truncateOperationsText(attempt.Error, 4096),
			ResponseBody: truncateOperationsText(attempt.ResponseBody, 4096),
			DurationMS:   attempt.DurationMS, CreatedAt: attempt.CreatedAt,
		})
	}
	return &dto.AdminWebhookDeliveryDetail{
		Delivery: adminWebhookDeliveryToDTO(delivery, event, endpoint, username),
		Payload:  payload, Attempts: attempts,
	}, true, nil
}

func RetryAdminWebhookDelivery(deliveryID string) (*dto.AdminWebhookDelivery, error) {
	delivery, err := model.ResetWebhookDeliveryForAdminRetry(deliveryID)
	if err != nil {
		return nil, err
	}
	loaded, event, endpoint, exists, err := model.GetAdminWebhookDelivery(delivery.DeliveryID)
	if err != nil || !exists {
		return nil, err
	}
	return adminWebhookDeliveryToDTO(loaded, event, endpoint, ""), nil
}

func adminWebhookDeliveryToDTO(delivery *model.WebhookDelivery, event *model.WebhookEvent, endpoint *model.WebhookEndpoint, username string) *dto.AdminWebhookDelivery {
	return &dto.AdminWebhookDelivery{
		DeliveryID: delivery.DeliveryID, EventID: event.EventID, EventType: event.EventType,
		ObjectID: event.ObjectID, EndpointID: endpoint.EndpointID, EndpointURL: safeWebhookEndpointURL(endpoint.URL),
		EndpointStatus: endpoint.Status, UserID: endpoint.UserID, Username: username,
		Status: delivery.Status, Attempts: delivery.Attempts, NextAttemptAt: delivery.NextAttemptAt,
		LockedUntil: delivery.LockedUntil, RetryDeadline: delivery.RetryDeadline,
		LastHTTPStatus: delivery.LastHTTPStatus, LastError: truncateOperationsText(delivery.LastError, 4096),
		DeliveredAt: delivery.DeliveredAt, CreatedAt: delivery.CreatedAt, UpdatedAt: delivery.UpdatedAt,
	}
}

func safeWebhookEndpointURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func truncateOperationsText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
