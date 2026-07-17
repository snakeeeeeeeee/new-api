package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOutboundWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalCryptoSecret := common.CryptoSecret
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	common.CryptoSecret = "webhook-test-crypto-secret"
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Setenv("WEBHOOK_ALLOW_INSECURE_LOCAL", "true")
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Task{}, &model.ImageTaskRequest{}, &model.Asset{}, &model.AssetKey{},
		&model.WebhookEndpoint{}, &model.WebhookEvent{}, &model.WebhookDelivery{}, &model.WebhookDeliveryAttempt{},
	))
	require.NoError(t, db.Create(&model.User{Id: 501, Username: "webhook-user", Status: common.UserStatusEnabled, Group: "default", AffCode: "webhook501"}).Error)
	require.NoError(t, db.Create(&model.User{Id: 502, Username: "other-user", Status: common.UserStatusEnabled, Group: "default", AffCode: "webhook502"}).Error)
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.CryptoSecret = originalCryptoSecret
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func putWebhookTestConfig(t *testing.T, targetURL string) *dto.AccountWebhookPublic {
	t.Helper()
	config, err := PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: targetURL})
	require.NoError(t, err)
	require.NotEmpty(t, config.Key)
	return config
}

func TestAccountWebhookConfigLifecycleAndEncryptedKey(t *testing.T) {
	setupOutboundWebhookTestDB(t)

	empty, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.False(t, empty.Configured)
	assert.False(t, empty.KeyConfigured)

	config := putWebhookTestConfig(t, "http://127.0.0.1:18080/hook")
	firstKey := config.Key
	assert.True(t, config.Configured)
	assert.True(t, config.KeyConfigured)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)
	assert.True(t, strings.HasPrefix(firstKey, webhookKeyPrefix))
	assert.Len(t, firstKey, len(webhookKeyPrefix)+48)

	var stored model.WebhookEndpoint
	require.NoError(t, model.DB.Where("user_id = ?", 501).First(&stored).Error)
	require.NotNil(t, stored.ConfigOwnerID)
	assert.Equal(t, 501, *stored.ConfigOwnerID)
	assert.NotContains(t, stored.AuthKeyEncrypted, firstKey)
	decrypted, err := decryptWebhookKey(stored.AuthKeyEncrypted)
	require.NoError(t, err)
	assert.Equal(t, firstKey, decrypted)

	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: "http://127.0.0.1:18081/new"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:18081/new", config.URL)
	assert.Equal(t, firstKey, config.Key)
	require.NoError(t, model.DB.First(&stored, stored.ID).Error)
	decrypted, err = decryptWebhookKey(stored.AuthKeyEncrypted)
	require.NoError(t, err)
	assert.Equal(t, firstKey, decrypted)

	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18081/new", RegenerateKey: true,
	})
	require.NoError(t, err)
	replacementKey := config.Key
	assert.True(t, strings.HasPrefix(replacementKey, webhookKeyPrefix))
	assert.NotEqual(t, firstKey, replacementKey)
	require.NoError(t, model.DB.First(&stored, stored.ID).Error)
	decrypted, err = decryptWebhookKey(stored.AuthKeyEncrypted)
	require.NoError(t, err)
	assert.Equal(t, replacementKey, decrypted)

	require.NoError(t, DisableAccountWebhookConfig(501))
	disabled, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointDisabled, disabled.Status)
	_, err = CreateAccountWebhookTestDelivery(501)
	assert.ErrorContains(t, err, "disabled")

	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: "http://127.0.0.1:18081/new"})
	require.NoError(t, err)
	reenabled, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointEnabled, reenabled.Status)

	other, err := GetAccountWebhookConfig(502)
	require.NoError(t, err)
	assert.False(t, other.Configured)
	assert.Empty(t, other.Key)
	loaded, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, replacementKey, loaded.Key)
	var total int64
	require.NoError(t, model.DB.Model(&model.WebhookEndpoint{}).Where("user_id = ?", 501).Count(&total).Error)
	assert.EqualValues(t, 1, total)
}

func TestAccountWebhookKeyGenerationAndCryptoSecretFailure(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	initial := putWebhookTestConfig(t, "http://127.0.0.1:18080")
	require.NoError(t, model.DB.Model(&model.WebhookEndpoint{}).Where("user_id = ?", 501).
		Update("updated_at", int64(1)).Error)
	common.CryptoSecret = "different-crypto-secret"
	config, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.True(t, config.Configured)
	assert.False(t, config.KeyConfigured)
	assert.Equal(t, model.WebhookEndpointDisabled, config.Status)
	assert.Greater(t, config.UpdatedAt, int64(1))
	var disabled model.WebhookEndpoint
	require.NoError(t, model.DB.Where("user_id = ?", 501).First(&disabled).Error)
	assert.Equal(t, config.UpdatedAt, disabled.UpdatedAt)

	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: "http://127.0.0.1:18080"})
	assert.ErrorIs(t, err, ErrWebhookStoredKeyUnavailable)
	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: "http://127.0.0.1:18080", RegenerateKey: true})
	require.NoError(t, err)
	assert.True(t, config.KeyConfigured)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)
	assert.True(t, strings.HasPrefix(config.Key, webhookKeyPrefix))
	assert.NotEqual(t, initial.Key, config.Key)
}

func TestAccountWebhookMigrationCollapsesLegacyEndpoints(t *testing.T) {
	db := setupOutboundWebhookTestDB(t)
	now := time.Now().Unix()
	legacy := []*model.WebhookEndpoint{
		{EndpointID: "we_old_enabled", UserID: 501, Name: "old", URL: "http://127.0.0.1:18080", Status: model.WebhookEndpointEnabled, EventTypes: `["image.task.failed"]`, APIVersion: WebhookAPIVersion, SecretSalt: "salt-old", SecretVersion: 1, CreatedAt: now - 30, UpdatedAt: now - 30},
		{EndpointID: "we_new_enabled", UserID: 501, Name: "new", URL: "http://127.0.0.1:18081", Status: model.WebhookEndpointEnabled, EventTypes: `["image.task.succeeded"]`, APIVersion: WebhookAPIVersion, SecretSalt: "salt-new", SecretVersion: 2, CreatedAt: now - 20, UpdatedAt: now - 10},
		{EndpointID: "we_newest_disabled", UserID: 501, Name: "disabled", URL: "http://127.0.0.1:18082", Status: model.WebhookEndpointDisabled, EventTypes: `[]`, APIVersion: WebhookAPIVersion, SecretSalt: "salt-disabled", SecretVersion: 1, CreatedAt: now - 10, UpdatedAt: now},
	}
	for _, endpoint := range legacy {
		require.NoError(t, db.Create(endpoint).Error)
	}
	event := &model.WebhookEvent{EventID: "evt_legacy", UserID: 501, EventType: WebhookEventImageTaskSucceeded, ObjectType: "image.task", ObjectID: "task_legacy", APIVersion: WebhookAPIVersion, Payload: `{}`, CreatedAt: now}
	require.NoError(t, db.Create(event).Error)
	require.NoError(t, db.Create(model.NewWebhookDelivery(event.ID, legacy[0].ID)).Error)

	require.NoError(t, MigrateAccountWebhookConfigs())
	selected, exists, err := model.GetAccountWebhookEndpoint(501)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "we_new_enabled", selected.EndpointID)
	decrypted, err := decryptWebhookKey(selected.AuthKeyEncrypted)
	require.NoError(t, err)
	assert.Equal(t, deriveLegacyWebhookSecret("we_new_enabled", "salt-new", 2), decrypted)
	assert.Equal(t, accountWebhookEventTypesJSON(), selected.EventTypes)

	var enabledCount, deliveryCount int64
	require.NoError(t, db.Model(&model.WebhookEndpoint{}).Where("user_id = ? AND status = ?", 501, model.WebhookEndpointEnabled).Count(&enabledCount).Error)
	require.NoError(t, db.Model(&model.WebhookDelivery{}).Count(&deliveryCount).Error)
	assert.EqualValues(t, 1, enabledCount)
	assert.EqualValues(t, 1, deliveryCount)
	require.NoError(t, MigrateAccountWebhookConfigs())
}

func TestImageTaskTerminalEventCreatesOneAccountDelivery(t *testing.T) {
	db := setupOutboundWebhookTestDB(t)
	putWebhookTestConfig(t, "http://127.0.0.1:18080/hook")
	task := &model.Task{
		TaskID: "task_webhook_success", UserId: 501, ChannelId: 77, Group: "default",
		Platform: constant.TaskPlatform("58"), Action: constant.TaskActionImageGeneration,
		Status: model.TaskStatusQueued, Progress: "0%", SubmitTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}
	task.SetData(map[string]any{"result": map[string]any{"images": []map[string]any{{
		"url": "https://cdn.example.com/final.png", "mime_type": "image/png", "width": 1024, "height": 1024,
	}}}})
	require.NoError(t, db.Create(task).Error)
	requestJSON, err := common.Marshal(dto.ImageTaskCreateRequest{
		Model: "gpt-image-2", Operation: "generation", Input: dto.ImageTaskInputRequest{Prompt: "draw"},
		ClientReferenceID: "order_webhook", Metadata: map[string]any{"tenant": "acme"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(model.NewImageTaskRequest(task, 501, nil, "fingerprint", "order_webhook", requestJSON)).Error)

	updated, _ := ApplyTaskResult(context.Background(), &mockAdaptor{}, task, &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess, Progress: "100%", Url: "https://cdn.example.com/final.png",
	})
	require.True(t, updated)

	var eventCount, deliveryCount, assetCount int64
	require.NoError(t, db.Model(&model.WebhookEvent{}).Count(&eventCount).Error)
	require.NoError(t, db.Model(&model.WebhookDelivery{}).Count(&deliveryCount).Error)
	require.NoError(t, db.Model(&model.Asset{}).Count(&assetCount).Error)
	assert.EqualValues(t, 1, eventCount)
	assert.EqualValues(t, 1, deliveryCount)
	assert.EqualValues(t, 1, assetCount)

	var event model.WebhookEvent
	require.NoError(t, db.First(&event).Error)
	assert.Equal(t, WebhookEventImageTaskSucceeded, event.EventType)
	assert.Contains(t, event.Payload, `"id":"task_webhook_success"`)
	assert.Contains(t, event.Payload, `"client_reference_id":"order_webhook"`)
	assert.NotContains(t, event.Payload, "channel_id")
	assert.NotContains(t, event.Payload, "user_id")
	assert.NotContains(t, event.Payload, "quota")

	var saved model.Task
	require.NoError(t, db.First(&saved, task.ID).Error)
	updated, _ = ApplyTaskResult(context.Background(), &mockAdaptor{}, &saved, &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess, Progress: "100%", Url: "https://cdn.example.com/final.png",
	})
	assert.False(t, updated)
	require.NoError(t, db.Model(&model.WebhookEvent{}).Count(&eventCount).Error)
	require.NoError(t, db.Model(&model.WebhookDelivery{}).Count(&deliveryCount).Error)
	assert.EqualValues(t, 1, eventCount)
	assert.EqualValues(t, 1, deliveryCount)
}

func TestWebhookDeliveryUsesBearerAndRetryKeepsEventID(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	var responseStatus atomic.Int64
	responseStatus.Store(http.StatusInternalServerError)
	var mu sync.Mutex
	var authorizations, bodies, signatures, timestamps []string
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		bodies = append(bodies, string(body))
		signatures = append(signatures, r.Header.Get("Webhook-Signature"))
		timestamps = append(timestamps, r.Header.Get("Webhook-Timestamp"))
		mu.Unlock()
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(int(responseStatus.Load()))
	}))
	defer receiver.Close()
	config := putWebhookTestConfig(t, receiver.URL)
	result, err := CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)
	processDueWebhookDeliveries(context.Background())

	var delivery model.WebhookDelivery
	require.NoError(t, model.DB.Joins("JOIN webhook_events ON webhook_events.id = webhook_deliveries.event_record_id").
		Where("webhook_events.event_id = ?", result.EventID).First(&delivery).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	require.NoError(t, model.DB.Model(&delivery).Update("next_attempt_at", time.Now().Unix()).Error)
	responseStatus.Store(http.StatusNoContent)
	processDueWebhookDeliveries(context.Background())

	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.WebhookDeliveryDelivered, delivery.Status)
	mu.Lock()
	require.Len(t, bodies, 2)
	assert.Equal(t, "Bearer "+config.Key, authorizations[0])
	assert.Equal(t, authorizations[0], authorizations[1])
	assert.Empty(t, signatures[0])
	assert.Empty(t, timestamps[0])
	assert.Equal(t, bodies[0], bodies[1])
	assert.Contains(t, bodies[0], `"id":"`+result.EventID+`"`)
	mu.Unlock()

	var attempts []model.WebhookDeliveryAttempt
	require.NoError(t, model.DB.Where("delivery_record_id = ?", delivery.ID).Find(&attempts).Error)
	require.Len(t, attempts, 2)
	for _, attempt := range attempts {
		assert.NotContains(t, attempt.Error, config.Key)
		assert.NotContains(t, attempt.ResponseBody, config.Key)
	}
}

func TestWebhookDeliveryDisablesConfigOn410(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer receiver.Close()
	putWebhookTestConfig(t, receiver.URL)
	_, err := CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)
	processDueWebhookDeliveries(context.Background())

	config, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointDisabled, config.Status)
	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: receiver.URL})
	require.NoError(t, err)
	config, err = GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)
}

func TestWebhookValidationRejectsInsecureAndPrivateTargetsByDefault(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	t.Setenv("WEBHOOK_ALLOW_INSECURE_LOCAL", "false")
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "http://example.com/hook"), "HTTPS")
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "https://127.0.0.1/hook"), "non-public IP")
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "https://169.254.169.254/latest/meta-data"), "non-public IP")
}
