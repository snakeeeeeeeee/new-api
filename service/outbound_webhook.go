package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	WebhookAPIVersion              = "2026-07-17"
	WebhookEventImageTaskSucceeded = "image.task.succeeded"
	WebhookEventImageTaskFailed    = "image.task.failed"
	WebhookEventTest               = "webhook.test"
	webhookKeyPrefix               = "wk-"
	webhookKeyEnvelopeVersion      = "v1"
	webhookDeliveryTimeout         = 10 * time.Second
)

var (
	webhookWorkerOnce              sync.Once
	ErrWebhookConfigNotFound       = errors.New("webhook configuration not found")
	ErrWebhookStoredKeyUnavailable = errors.New("stored webhook key cannot be decrypted; regenerate the key")
)

func accountWebhookEventTypesJSON() string {
	body, _ := common.Marshal([]string{WebhookEventImageTaskFailed, WebhookEventImageTaskSucceeded})
	return string(body)
}

func deriveLegacyWebhookSecret(endpointID, salt string, version int) string {
	message := fmt.Sprintf("webhook:%s:%s:%d", endpointID, salt, version)
	digest := common.HmacSha256Raw([]byte(message), []byte(common.CryptoSecret))
	return "whsec_" + base64.RawURLEncoding.EncodeToString(digest)
}

func webhookKeyCipher() (cipher.AEAD, error) {
	digest := sha256.Sum256([]byte("new-api:account-webhook-key:v1:" + common.CryptoSecret))
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptWebhookKey(value string) (string, error) {
	aead, err := webhookKeyCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, []byte(value), []byte(webhookKeyEnvelopeVersion))
	payload := append(nonce, sealed...)
	return webhookKeyEnvelopeVersion + ":" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decryptWebhookKey(value string) (string, error) {
	version, encoded, ok := strings.Cut(value, ":")
	if !ok || version != webhookKeyEnvelopeVersion {
		return "", errors.New("unsupported webhook key envelope")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("invalid webhook key envelope")
	}
	aead, err := webhookKeyCipher()
	if err != nil {
		return "", err
	}
	if len(payload) < aead.NonceSize() {
		return "", errors.New("invalid webhook key envelope")
	}
	plaintext, err := aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], []byte(version))
	if err != nil {
		return "", errors.New("webhook key decryption failed")
	}
	return string(plaintext), nil
}

func generateWebhookKey() (string, error) {
	value, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return "", err
	}
	return webhookKeyPrefix + value, nil
}

func accountWebhookToPublic(endpoint *model.WebhookEndpoint, keyConfigured bool) *dto.AccountWebhookPublic {
	if endpoint == nil {
		return &dto.AccountWebhookPublic{Status: model.WebhookEndpointDisabled}
	}
	return &dto.AccountWebhookPublic{
		Configured: true, URL: endpoint.URL, KeyConfigured: keyConfigured,
		Status: endpoint.Status, UpdatedAt: endpoint.UpdatedAt,
	}
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
			encryptedKey := selected.AuthKeyEncrypted
			if encryptedKey == "" && selected.SecretSalt != "" && selected.SecretVersion > 0 {
				var err error
				encryptedKey, err = encryptWebhookKey(deriveLegacyWebhookSecret(selected.EndpointID, selected.SecretSalt, selected.SecretVersion))
				if err != nil {
					return err
				}
			}
			status := selected.Status
			if encryptedKey == "" {
				status = model.WebhookEndpointDisabled
			}
			ownerID := userID
			if err := tx.Model(&model.WebhookEndpoint{}).Where("id = ?", selected.ID).Updates(map[string]any{
				"config_owner_id": ownerID, "auth_key_encrypted": encryptedKey,
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
	endpoint, exists, err := model.GetAccountWebhookEndpoint(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return accountWebhookToPublic(nil, false), nil
	}
	keyValue, decryptErr := decryptWebhookKey(endpoint.AuthKeyEncrypted)
	if decryptErr != nil {
		now := time.Now().Unix()
		endpoint.Status = model.WebhookEndpointDisabled
		if err := model.DB.Model(endpoint).Updates(map[string]any{
			"status": model.WebhookEndpointDisabled, "updated_at": now,
		}).Error; err != nil {
			return nil, err
		}
		endpoint.UpdatedAt = now
		return accountWebhookToPublic(endpoint, false), nil
	}
	response := accountWebhookToPublic(endpoint, true)
	response.Key = keyValue
	return response, nil
}

func PutAccountWebhookConfig(userID int, request dto.AccountWebhookUpdateRequest) (*dto.AccountWebhookPublic, error) {
	request.URL = strings.TrimSpace(request.URL)
	if err := ValidateWebhookEndpointURL(context.Background(), request.URL); err != nil {
		return nil, err
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

		if !creating && !request.RegenerateKey {
			if _, err := decryptWebhookKey(endpoint.AuthKeyEncrypted); err != nil {
				return ErrWebhookStoredKeyUnavailable
			}
		} else {
			keyValue, err := generateWebhookKey()
			if err != nil {
				return err
			}
			encrypted, err := encryptWebhookKey(keyValue)
			if err != nil {
				return err
			}
			endpoint.AuthKeyEncrypted = encrypted
		}

		now := time.Now().Unix()
		ownerID := userID
		if creating {
			endpoint = model.WebhookEndpoint{
				EndpointID: model.NewWebhookEndpointID(), UserID: userID, ConfigOwnerID: &ownerID,
				Name: "Task events", URL: request.URL, Status: model.WebhookEndpointEnabled,
				EventTypes: accountWebhookEventTypesJSON(), APIVersion: WebhookAPIVersion,
				AuthKeyEncrypted: endpoint.AuthKeyEncrypted, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&endpoint).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{
				"url": request.URL, "status": model.WebhookEndpointEnabled,
				"event_types": accountWebhookEventTypesJSON(), "api_version": WebhookAPIVersion,
				"auth_key_encrypted": endpoint.AuthKeyEncrypted, "updated_at": now,
			}
			if err := tx.Model(&endpoint).Updates(updates).Error; err != nil {
				return err
			}
			endpoint.URL = request.URL
			endpoint.Status = model.WebhookEndpointEnabled
			endpoint.UpdatedAt = now
		}
		result = &endpoint
		return nil
	})
	if err != nil {
		return nil, err
	}
	keyValue, err := decryptWebhookKey(result.AuthKeyEncrypted)
	if err != nil {
		return nil, ErrWebhookStoredKeyUnavailable
	}
	response := accountWebhookToPublic(result, true)
	response.Key = keyValue
	return response, nil
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
	if _, err := decryptWebhookKey(endpoint.AuthKeyEncrypted); err != nil {
		_ = model.DB.Model(endpoint).Update("status", model.WebhookEndpointDisabled).Error
		return nil, ErrWebhookStoredKeyUnavailable
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
		completeWebhookFailure(ctx, claimed, "load webhook delivery context: "+err.Error(), 0)
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
	authKey, err := decryptWebhookKey(endpoint.AuthKeyEncrypted)
	if err != nil {
		_, completeErr := model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
			Status: model.WebhookDeliveryDiscarded, LastError: "webhook key decryption failed", DisableEndpoint: true,
		})
		if completeErr != nil {
			logger.LogError(ctx, "disable webhook with unreadable key failed: "+completeErr.Error())
		}
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
	defer cancel()
	client, err := newWebhookHTTPClient(requestCtx, endpoint.URL)
	if err != nil {
		completeWebhookFailure(ctx, delivery, err.Error(), 0)
		return
	}
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.URL, bytes.NewBufferString(event.Payload))
	if err != nil {
		completeWebhookFailure(ctx, delivery, err.Error(), 0)
		return
	}
	request.Header.Set("Authorization", "Bearer "+authKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "new-api-webhooks/1.0")
	started := time.Now()
	response, err := client.Do(request)
	durationMS := time.Since(started).Milliseconds()
	if err != nil {
		completeWebhookFailure(ctx, delivery, err.Error(), durationMS)
		return
	}
	_ = response.Body.Close()
	_, err = model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
		Status: model.WebhookDeliveryDelivered, HTTPStatus: response.StatusCode, DurationMS: durationMS,
	})
	if err != nil {
		logger.LogError(ctx, "complete webhook delivery failed: "+err.Error())
	}
}

func completeWebhookFailure(ctx context.Context, delivery *model.WebhookDelivery, reason string, durationMS int64) {
	_, err := model.CompleteWebhookDeliveryAttempt(delivery, model.WebhookDeliveryResult{
		Status: model.WebhookDeliveryFailed, LastError: reason, DurationMS: durationMS,
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
