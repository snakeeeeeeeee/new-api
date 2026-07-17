package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	WebhookAPIVersion              = "2026-07-17"
	WebhookEventImageTaskSucceeded = "image.task.succeeded"
	WebhookEventImageTaskFailed    = "image.task.failed"
	WebhookEventTest               = "webhook.test"
	webhookDeliveryTimeout         = 10 * time.Second
)

var (
	webhookWorkerOnce        sync.Once
	ErrWebhookConfigNotFound = errors.New("webhook configuration not found")
)

func accountWebhookEventTypesJSON() string {
	body, _ := common.Marshal([]string{WebhookEventImageTaskFailed, WebhookEventImageTaskSucceeded})
	return string(body)
}

func accountWebhookToPublic(endpoint *model.WebhookEndpoint, resourceKey *model.AssetKey) *dto.AccountWebhookPublic {
	if endpoint == nil {
		response := &dto.AccountWebhookPublic{Status: model.WebhookEndpointDisabled}
		if resourceKey != nil {
			response.ResourceKeyConfigured = true
		}
		return response
	}
	response := &dto.AccountWebhookPublic{
		Configured: true, URL: endpoint.URL, Status: endpoint.Status, UpdatedAt: endpoint.UpdatedAt,
	}
	if resourceKey != nil {
		response.ResourceKeyConfigured = true
	}
	return response
}

// MigrateAccountWebhookConfigs reduces legacy multi-endpoint settings to one
// account-level task event destination while preserving old delivery records.
func MigrateAccountWebhookConfigs() error {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.WebhookEndpoint{}) {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Model(&model.WebhookEndpoint{}).
			Where("deleted_at IS NOT NULL AND config_owner_id IS NOT NULL").
			Update("config_owner_id", nil).Error; err != nil {
			return err
		}
		var endpoints []*model.WebhookEndpoint
		if err := tx.Where("user_id > 0").Order("user_id ASC, updated_at DESC, id DESC").Find(&endpoints).Error; err != nil {
			return err
		}
		byUser := make(map[int][]*model.WebhookEndpoint)
		for _, endpoint := range endpoints {
			byUser[endpoint.UserID] = append(byUser[endpoint.UserID], endpoint)
		}
		for userID, candidates := range byUser {
			var selected *model.WebhookEndpoint
			for _, candidate := range candidates {
				if candidate.ConfigOwnerID != nil && *candidate.ConfigOwnerID == userID {
					selected = candidate
					break
				}
			}
			if selected == nil {
				for _, candidate := range candidates {
					if candidate.Status == model.WebhookEndpointEnabled {
						selected = candidate
						break
					}
				}
			}
			if selected == nil {
				selected = candidates[0]
			}

			if err := tx.Model(&model.WebhookEndpoint{}).Where("user_id = ? AND id <> ?", userID, selected.ID).
				Updates(map[string]any{"config_owner_id": nil, "status": model.WebhookEndpointDisabled}).Error; err != nil {
				return err
			}
			status := selected.Status
			var activeKeyCount int64
			now := time.Now().Unix()
			if err := tx.Model(&model.AssetKey{}).Where(
				"user_id = ? AND status = ? AND (expired_at = ? OR expired_at = ? OR expired_at >= ?)",
				userID, model.AssetKeyStatusEnabled, -1, 0, now,
			).Count(&activeKeyCount).Error; err != nil {
				return err
			}
			if activeKeyCount == 0 {
				status = model.WebhookEndpointDisabled
			}
			ownerID := userID
			if err := tx.Model(&model.WebhookEndpoint{}).Where("id = ?", selected.ID).Updates(map[string]any{
				"config_owner_id": ownerID, "auth_key_encrypted": "",
				"secret_salt": "", "secret_version": 0,
				"previous_secret_salt": "", "previous_secret_version": 0, "previous_secret_expires_at": 0,
				"event_types": accountWebhookEventTypesJSON(), "api_version": WebhookAPIVersion,
				"status": status,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func GetAccountWebhookConfig(userID int) (*dto.AccountWebhookPublic, error) {
	resourceKey, _, err := model.GetActiveUserAssetKey(userID)
	if err != nil {
		return nil, err
	}
	endpoint, exists, err := model.GetAccountWebhookEndpoint(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return accountWebhookToPublic(nil, resourceKey), nil
	}
	return accountWebhookToPublic(endpoint, resourceKey), nil
}

func PutAccountWebhookConfig(userID int, request dto.AccountWebhookUpdateRequest) (*dto.AccountWebhookPublic, error) {
	request.URL = strings.TrimSpace(request.URL)
	if request.URL != "" {
		if err := ValidateWebhookEndpointURL(context.Background(), request.URL); err != nil {
			return nil, err
		}
	}
	if request.Enabled != nil && *request.Enabled && request.URL == "" {
		return nil, errors.New("Webhook URL is required when enabling Webhook")
	}
	if request.Enabled != nil && *request.Enabled {
		if _, exists, err := model.GetActiveUserAssetKey(userID); err != nil {
			return nil, err
		} else if !exists {
			return nil, errors.New("an enabled Resource Center API Key is required when enabling Webhook")
		}
	}
	var result *model.WebhookEndpoint
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		var endpoint model.WebhookEndpoint
		err := tx.Where("config_owner_id = ? AND user_id = ?", userID, userID).First(&endpoint).Error
		creating := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !creating {
			return err
		}

		status := model.WebhookEndpointDisabled
		if !creating {
			status = endpoint.Status
		}
		if request.Enabled != nil {
			if *request.Enabled {
				status = model.WebhookEndpointEnabled
			} else {
				status = model.WebhookEndpointDisabled
			}
		}

		now := time.Now().Unix()
		ownerID := userID
		if creating {
			endpoint = model.WebhookEndpoint{
				EndpointID: model.NewWebhookEndpointID(), UserID: userID, ConfigOwnerID: &ownerID,
				Name: "Task events", URL: request.URL, Status: status,
				EventTypes: accountWebhookEventTypesJSON(), APIVersion: WebhookAPIVersion,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&endpoint).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{
				"url": request.URL, "status": status,
				"event_types": accountWebhookEventTypesJSON(), "api_version": WebhookAPIVersion,
				"auth_key_encrypted": "", "updated_at": now,
			}
			if err := tx.Model(&endpoint).Updates(updates).Error; err != nil {
				return err
			}
			endpoint.URL = request.URL
			endpoint.Status = status
			endpoint.UpdatedAt = now
		}
		result = &endpoint
		return nil
	})
	if err != nil {
		return nil, err
	}
	resourceKey, _, err := model.GetActiveUserAssetKey(userID)
	if err != nil {
		return nil, err
	}
	return accountWebhookToPublic(result, resourceKey), nil
}

func DisableAccountWebhookConfig(userID int) error {
	endpoint, exists, err := model.GetAccountWebhookEndpoint(userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrWebhookConfigNotFound
	}
	return model.DB.Model(endpoint).Updates(map[string]any{
		"status": model.WebhookEndpointDisabled, "updated_at": time.Now().Unix(),
	}).Error
}

func CreateImageTaskWebhookEventTx(tx *gorm.DB, task *model.Task) error {
	if tx == nil || task == nil {
		return nil
	}
	if !tx.Migrator().HasTable(&model.ImageTaskRequest{}) || !tx.Migrator().HasTable(&model.WebhookEvent{}) {
		return nil
	}
	eventType := ""
	switch task.Status {
	case model.TaskStatusSuccess:
		eventType = WebhookEventImageTaskSucceeded
	case model.TaskStatusFailure:
		eventType = WebhookEventImageTaskFailed
	default:
		return nil
	}
	publicTask, exists, err := BuildPublicImageTaskTx(tx, task)
	if err != nil || !exists {
		return err
	}
	now := time.Now().Unix()
	event := &model.WebhookEvent{
		EventID: model.NewWebhookEventID(), UserID: task.UserId, EventType: eventType,
		ObjectType: "image.task", ObjectID: task.TaskID, APIVersion: WebhookAPIVersion, CreatedAt: now,
	}
	payload, err := common.Marshal(dto.WebhookEventEnvelope{
		ID: event.EventID, Object: "event", APIVersion: event.APIVersion, Type: event.EventType,
		CreatedAt: event.CreatedAt, Data: dto.WebhookEventData{Object: publicTask},
	})
	if err != nil {
		return err
	}
	event.Payload = string(payload)
	if err := tx.Create(event).Error; err != nil {
		return err
	}
	var endpoint model.WebhookEndpoint
	err = tx.Where("config_owner_id = ? AND user_id = ? AND status = ?", task.UserId, task.UserId, model.WebhookEndpointEnabled).
		First(&endpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Create(model.NewWebhookDelivery(event.ID, endpoint.ID)).Error
}

func CreateAccountWebhookTestDelivery(userID int) (*dto.AccountWebhookTestResponse, error) {
	endpoint, exists, err := model.GetAccountWebhookEndpoint(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrWebhookConfigNotFound
	}
	if endpoint.Status != model.WebhookEndpointEnabled {
		return nil, errors.New("webhook configuration is disabled")
	}
	if _, exists, err := model.GetActiveUserAssetKey(userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.New("an enabled Resource Center API Key is required to test Webhook")
	}
	now := time.Now().Unix()
	event := &model.WebhookEvent{
		EventID: model.NewWebhookEventID(), UserID: userID, EventType: WebhookEventTest,
		ObjectType: "webhook.test", ObjectID: model.NewWebhookEventID(), APIVersion: WebhookAPIVersion, CreatedAt: now,
	}
	payload, err := common.Marshal(dto.WebhookEventEnvelope{
		ID: event.EventID, Object: "event", APIVersion: event.APIVersion, Type: event.EventType,
		CreatedAt: event.CreatedAt, Data: dto.WebhookEventData{Object: map[string]any{
			"object": "webhook.test", "created_at": now,
		}},
	})
	if err != nil {
		return nil, err
	}
	event.Payload = string(payload)
	delivery := model.NewWebhookDelivery(0, endpoint.ID)
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		delivery.EventRecordID = event.ID
		return tx.Create(delivery).Error
	})
	if err != nil {
		return nil, err
	}
	return &dto.AccountWebhookTestResponse{EventID: event.EventID, Status: model.WebhookDeliveryPending}, nil
}

func StartOutboundWebhookWorker() {
	if !common.IsMasterNode {
		return
	}
	if err := MigrateAccountWebhookConfigs(); err != nil {
		logger.LogError(context.Background(), "migrate account webhook configuration failed: "+err.Error())
	}
	webhookWorkerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			cleanupTicker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			defer cleanupTicker.Stop()
			for {
				select {
				case <-ticker.C:
					processDueWebhookDeliveries(context.Background())
				case <-cleanupTicker.C:
					cleanupOutboundWebhookLogs(context.Background())
				}
			}
		}()
	})
}

func processDueWebhookDeliveries(ctx context.Context) {
	deliveries, err := model.ClaimDueWebhookDeliveries(20, 30)
	if err != nil {
		logger.LogError(ctx, "claim webhook deliveries failed: "+err.Error())
		return
	}
	for _, delivery := range deliveries {
		processWebhookDelivery(ctx, delivery)
	}
}

func processWebhookDelivery(ctx context.Context, claimed *model.WebhookDelivery) {
	delivery, event, endpoint, err := model.LoadWebhookDeliveryContext(claimed.ID)
	if err != nil {
		completeWebhookFailure(ctx, claimed, 0, "load webhook delivery context: "+err.Error(), 0)
		return
	}
	if delivery.LockToken != claimed.LockToken {
		return
	}
	if endpoint.Status != model.WebhookEndpointEnabled || endpoint.DeletedAt.Valid {
		_, err := model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
			Status: model.WebhookDeliveryDiscarded, LastError: "webhook configuration is disabled",
		})
		if err != nil {
			logger.LogError(ctx, "discard webhook delivery failed: "+err.Error())
		}
		return
	}
	resourceKey, exists, err := model.GetActiveUserAssetKey(endpoint.UserID)
	if err != nil {
		completeWebhookFailure(ctx, delivery, 0, "load Resource Center API Key: "+err.Error(), 0)
		return
	}
	if !exists {
		completeWebhookFailure(ctx, delivery, 0, "enabled Resource Center API Key is unavailable", 0)
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
	defer cancel()
	client, err := newWebhookHTTPClient(requestCtx, endpoint.URL)
	if err != nil {
		completeWebhookFailure(ctx, delivery, 0, err.Error(), 0)
		return
	}
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.URL, bytes.NewBufferString(event.Payload))
	if err != nil {
		completeWebhookFailure(ctx, delivery, 0, err.Error(), 0)
		return
	}
	request.Header.Set("Authorization", "Bearer "+resourceKey.Key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "new-api-webhooks/1.0")
	started := time.Now()
	response, err := client.Do(request)
	durationMS := time.Since(started).Milliseconds()
	if err != nil {
		completeWebhookFailure(ctx, delivery, 0, err.Error(), durationMS)
		return
	}
	_ = response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		completeWebhookFailure(ctx, delivery, response.StatusCode, fmt.Sprintf("webhook receiver returned HTTP %d", response.StatusCode), durationMS)
		return
	}
	_, err = model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
		Status: model.WebhookDeliveryDelivered, HTTPStatus: response.StatusCode, DurationMS: durationMS,
	})
	if err != nil {
		logger.LogError(ctx, "complete webhook delivery failed: "+err.Error())
	}
}

func completeWebhookFailure(ctx context.Context, delivery *model.WebhookDelivery, httpStatus int, reason string, durationMS int64) {
	settings := async_task_setting.NormalizeSetting(*async_task_setting.GetAsyncTaskSetting())
	status := model.WebhookDeliveryFailed
	nextAttemptAt := int64(0)
	if delivery != nil && delivery.Attempts < settings.WebhookMaxAttempts {
		status = model.WebhookDeliveryPending
		nextAttemptAt = time.Now().Add(time.Duration(settings.WebhookRetryIntervalSeconds) * time.Second).Unix()
		if delivery.RetryDeadline > 0 && nextAttemptAt > delivery.RetryDeadline {
			status = model.WebhookDeliveryFailed
			nextAttemptAt = 0
		}
	}
	_, err := model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
		Status: status, NextAttemptAt: nextAttemptAt, HTTPStatus: httpStatus, LastError: reason, DurationMS: durationMS,
	})
	if err != nil {
		logger.LogError(ctx, "record webhook delivery failure failed: "+err.Error())
	}
}

func webhookAllowsInsecureLocal() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WEBHOOK_ALLOW_INSECURE_LOCAL")), "true")
}

func ValidateWebhookEndpointURL(ctx context.Context, rawURL string) error {
	parsed, host, port, err := parseWebhookURL(rawURL)
	if err != nil {
		return err
	}
	if !webhookAllowsInsecureLocal() && parsed.Scheme != "https" {
		return errors.New("webhook endpoints must use HTTPS")
	}
	_, err = resolveWebhookTarget(ctx, host, port, webhookAllowsInsecureLocal())
	return err
}

func parseWebhookURL(rawURL string) (*url.URL, string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return nil, "", "", errors.New("invalid webhook URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, "", "", errors.New("webhook URL must use HTTP or HTTPS")
	}
	if parsed.User != nil {
		return nil, "", "", errors.New("webhook URL must not contain credentials")
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if parsed.Fragment != "" {
		return nil, "", "", errors.New("webhook URL must not contain a fragment")
	}
	return parsed, parsed.Hostname(), port, nil
}

func resolveWebhookTarget(ctx context.Context, host, port string, allowLocal bool) ([]string, error) {
	if parsedPort, err := strconv.Atoi(port); err != nil || parsedPort < 1 || parsedPort > 65535 {
		return nil, errors.New("invalid webhook URL port")
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("webhook DNS resolution failed: %w", err)
		}
		for _, address := range addresses {
			ips = append(ips, address.IP)
		}
	}
	if len(ips) == 0 {
		return nil, errors.New("webhook DNS resolution returned no addresses")
	}
	targets := make([]string, 0, len(ips))
	for _, ip := range ips {
		if !allowLocal && !isPublicWebhookIP(ip) {
			return nil, fmt.Errorf("webhook target resolves to a non-public IP: %s", ip.String())
		}
		targets = append(targets, net.JoinHostPort(ip.String(), port))
	}
	return targets, nil
}

func isPublicWebhookIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	blocked := []string{"100.64.0.0/10", "198.18.0.0/15"}
	for _, cidr := range blocked {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func newWebhookHTTPClient(ctx context.Context, rawURL string) (*http.Client, error) {
	parsed, host, port, err := parseWebhookURL(rawURL)
	if err != nil {
		return nil, err
	}
	allowLocal := webhookAllowsInsecureLocal()
	if !allowLocal && parsed.Scheme != "https" {
		return nil, errors.New("webhook endpoints must use HTTPS")
	}
	targets, err := resolveWebhookTarget(ctx, host, port, allowLocal)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil, ForceAttemptHTTP2: true, MaxIdleConns: 20, MaxIdleConnsPerHost: 4,
		IdleConnTimeout: 30 * time.Second,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			for _, target := range targets {
				connection, err := dialer.DialContext(ctx, network, target)
				if err == nil {
					return connection, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	return &http.Client{
		Transport: transport, Timeout: webhookDeliveryTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

func cleanupOutboundWebhookLogs(ctx context.Context) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	for i := 0; i < 20; i++ {
		count, err := model.CleanupWebhookLogs(cutoff, 500)
		if err != nil {
			logger.LogError(ctx, "cleanup webhook logs failed: "+err.Error())
			return
		}
		if count < 500 {
			return
		}
	}
}
