package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/async_task_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOutboundWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
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
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createWebhookResourceKey(t *testing.T, userID int, name string) *model.AssetKey {
	t.Helper()
	key, err := model.CreateAssetKey(userID, name, -1, "")
	require.NoError(t, err)
	return key
}

func putWebhookTestConfig(t *testing.T, targetURL string) *dto.AccountWebhookPublic {
	t.Helper()
	if _, exists, err := model.GetActiveUserAssetKey(501); err != nil {
		require.NoError(t, err)
	} else if !exists {
		createWebhookResourceKey(t, 501, "resource-center")
	}
	config, err := PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: targetURL, Enabled: common.GetPointer(true),
	})
	require.NoError(t, err)
	require.True(t, config.ResourceKeyConfigured)
	return config
}

func TestAccountWebhookConfigUsesResourceCenterKey(t *testing.T) {
	setupOutboundWebhookTestDB(t)

	empty, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.False(t, empty.Configured)
	assert.False(t, empty.ResourceKeyConfigured)

	createWebhookResourceKey(t, 501, "resource-center")
	config := putWebhookTestConfig(t, "http://127.0.0.1:18080/hook")
	assert.True(t, config.Configured)
	assert.True(t, config.ResourceKeyConfigured)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)

	var stored model.WebhookEndpoint
	require.NoError(t, model.DB.Where("user_id = ?", 501).First(&stored).Error)
	require.NotNil(t, stored.ConfigOwnerID)
	assert.Equal(t, 501, *stored.ConfigOwnerID)
	assert.Empty(t, stored.AuthKeyEncrypted)

	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{URL: "http://127.0.0.1:18081/new"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:18081/new", config.URL)

	require.NoError(t, DisableAccountWebhookConfig(501))
	disabled, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointDisabled, disabled.Status)
	_, err = CreateAccountWebhookTestDelivery(501)
	assert.ErrorContains(t, err, "disabled")

	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18081/new", Enabled: common.GetPointer(true),
	})
	require.NoError(t, err)
	reenabled, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointEnabled, reenabled.Status)

	other, err := GetAccountWebhookConfig(502)
	require.NoError(t, err)
	assert.False(t, other.Configured)
	assert.False(t, other.ResourceKeyConfigured)
	loaded, err := GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.True(t, loaded.ResourceKeyConfigured)
	var total int64
	require.NoError(t, model.DB.Model(&model.WebhookEndpoint{}).Where("user_id = ?", 501).Count(&total).Error)
	assert.EqualValues(t, 1, total)
}

func TestAccountWebhookRequiresEnabledResourceCenterKey(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	config, err := PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18080", Enabled: common.GetPointer(false),
	})
	require.NoError(t, err)
	assert.True(t, config.Configured)
	assert.False(t, config.ResourceKeyConfigured)
	assert.Equal(t, model.WebhookEndpointDisabled, config.Status)

	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18080", Enabled: common.GetPointer(true),
	})
	assert.ErrorContains(t, err, "Resource Center API Key")

	resourceKey := createWebhookResourceKey(t, 501, "resource-center")
	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18080", Enabled: common.GetPointer(true),
	})
	require.NoError(t, err)
	assert.True(t, config.ResourceKeyConfigured)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)

	_, err = model.UpdateUserAssetKeyStatus(resourceKey.ID, 501, model.AssetKeyStatusDisabled)
	require.NoError(t, err)
	config, err = GetAccountWebhookConfig(501)
	require.NoError(t, err)
	assert.False(t, config.ResourceKeyConfigured)
	_, err = CreateAccountWebhookTestDelivery(501)
	assert.ErrorContains(t, err, "Resource Center API Key")
}

func TestAccountWebhookMigrationCollapsesLegacyEndpoints(t *testing.T) {
	db := setupOutboundWebhookTestDB(t)
	createWebhookResourceKey(t, 501, "resource-center")
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
	assert.Empty(t, selected.AuthKeyEncrypted)
	assert.Empty(t, selected.SecretSalt)
	assert.Zero(t, selected.SecretVersion)
	assert.Equal(t, accountWebhookEventTypesJSON(), selected.EventTypes)

	var enabledCount, deliveryCount int64
	require.NoError(t, db.Model(&model.WebhookEndpoint{}).Where("user_id = ? AND status = ?", 501, model.WebhookEndpointEnabled).Count(&enabledCount).Error)
	require.NoError(t, db.Model(&model.WebhookDelivery{}).Count(&deliveryCount).Error)
	assert.EqualValues(t, 1, enabledCount)
	assert.EqualValues(t, 1, deliveryCount)
	require.NoError(t, MigrateAccountWebhookConfigs())
}

func TestAccountWebhookRequiresURLOnlyWhenEnabled(t *testing.T) {
	setupOutboundWebhookTestDB(t)

	config, err := PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		Enabled: common.GetPointer(false),
	})
	require.NoError(t, err)
	assert.True(t, config.Configured)
	assert.Empty(t, config.URL)
	assert.Equal(t, model.WebhookEndpointDisabled, config.Status)

	_, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		Enabled: common.GetPointer(true),
	})
	assert.ErrorContains(t, err, "URL is required")

	createWebhookResourceKey(t, 501, "resource-center")
	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: "http://127.0.0.1:18080/hook", Enabled: common.GetPointer(true),
	})
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)

	config, err = PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: config.URL, Enabled: common.GetPointer(false),
	})
	require.NoError(t, err)
	assert.Equal(t, model.WebhookEndpointDisabled, config.Status)
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

func TestWebhookDeliveryRetriesNon2xxUntilSuccess(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	configureWebhookRetryTest(t, 3, 30)
	var mu sync.Mutex
	var authorizations, bodies, signatures, timestamps []string
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		bodies = append(bodies, string(body))
		signatures = append(signatures, r.Header.Get("Webhook-Signature"))
		timestamps = append(timestamps, r.Header.Get("Webhook-Timestamp"))
		attempt := len(bodies)
		mu.Unlock()
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	putWebhookTestConfig(t, receiver.URL)
	resourceKey, exists, err := model.GetActiveUserAssetKey(501)
	require.NoError(t, err)
	require.True(t, exists)
	result, err := CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)
	processDueWebhookDeliveries(context.Background())

	var delivery model.WebhookDelivery
	require.NoError(t, model.DB.Joins("JOIN webhook_events ON webhook_events.id = webhook_deliveries.event_record_id").
		Where("webhook_events.event_id = ?", result.EventID).First(&delivery).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	assert.Equal(t, http.StatusInternalServerError, delivery.LastHTTPStatus)
	require.NoError(t, makeWebhookDeliveryDue(delivery.ID))
	processDueWebhookDeliveries(context.Background())
	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 2, delivery.Attempts)
	require.NoError(t, makeWebhookDeliveryDue(delivery.ID))
	processDueWebhookDeliveries(context.Background())
	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.WebhookDeliveryDelivered, delivery.Status)
	assert.Equal(t, 3, delivery.Attempts)
	assert.Equal(t, http.StatusNoContent, delivery.LastHTTPStatus)
	processDueWebhookDeliveries(context.Background())
	mu.Lock()
	require.Len(t, bodies, 3)
	for index := range bodies {
		assert.Equal(t, "Bearer "+resourceKey.Key, authorizations[index])
		assert.Empty(t, signatures[index])
		assert.Empty(t, timestamps[index])
		assert.Contains(t, bodies[index], `"id":"`+result.EventID+`"`)
	}
	mu.Unlock()

	var attempts []model.WebhookDeliveryAttempt
	require.NoError(t, model.DB.Where("delivery_record_id = ?", delivery.ID).Find(&attempts).Error)
	require.Len(t, attempts, 3)
	for _, attempt := range attempts {
		assert.NotContains(t, attempt.Error, resourceKey.Key)
		assert.NotContains(t, attempt.ResponseBody, resourceKey.Key)
	}
}

func TestWebhookDeliveryUsesCurrentResourceCenterKey(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	older := createWebhookResourceKey(t, 501, "older-resource-key")
	current := createWebhookResourceKey(t, 501, "current-resource-key")
	require.Equal(t, older.ID, current.ID)
	require.NotEqual(t, older.Key, current.Key)

	var authorization string
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()

	config, err := PutAccountWebhookConfig(501, dto.AccountWebhookUpdateRequest{
		URL: receiver.URL, Enabled: common.GetPointer(true),
	})
	require.NoError(t, err)
	assert.True(t, config.ResourceKeyConfigured)

	_, err = CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)
	processDueWebhookDeliveries(context.Background())
	assert.Equal(t, "Bearer "+current.Key, authorization)
	assert.NotEqual(t, "Bearer "+older.Key, authorization)
}

func TestDeleteResourceCenterKeyRemovesLegacyRecords(t *testing.T) {
	db := setupOutboundWebhookTestDB(t)
	now := time.Now().Unix()
	older := &model.AssetKey{
		UserID: 501, Name: "older", Key: "ak_older_legacy_key", Status: model.AssetKeyStatusDisabled,
		Scopes: model.AssetKeyScopeRead, ExpiredAt: -1, CreatedAt: now - 10, UpdatedAt: now - 10,
	}
	current := &model.AssetKey{
		UserID: 501, Name: "current", Key: "ak_current_resource_key", Status: model.AssetKeyStatusEnabled,
		Scopes: model.AssetKeyScopeRead, ExpiredAt: -1, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(older).Error)
	require.NoError(t, db.Create(current).Error)

	require.NoError(t, model.DeleteUserAssetKey(current.ID, 501))

	var visible, historical int64
	require.NoError(t, db.Model(&model.AssetKey{}).Where("user_id = ?", 501).Count(&visible).Error)
	require.NoError(t, db.Unscoped().Model(&model.AssetKey{}).Where("user_id = ?", 501).Count(&historical).Error)
	assert.Zero(t, visible)
	assert.EqualValues(t, 2, historical)
}

func TestWebhookDeliveryDoesNotDisableConfigOn410(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	configureWebhookRetryTest(t, 3, 30)
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
	assert.Equal(t, model.WebhookEndpointEnabled, config.Status)
	var delivery model.WebhookDelivery
	require.NoError(t, model.DB.Order("id DESC").First(&delivery).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
}

func TestWebhookDeliveryConnectionFailureStopsAtConfiguredAttempts(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	configureWebhookRetryTest(t, 3, 30)
	receiver := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	targetURL := receiver.URL
	putWebhookTestConfig(t, targetURL)
	receiver.Close()

	result, err := CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)
	processDueWebhookDeliveries(context.Background())

	var delivery model.WebhookDelivery
	require.NoError(t, model.DB.Joins("JOIN webhook_events ON webhook_events.id = webhook_deliveries.event_record_id").
		Where("webhook_events.event_id = ?", result.EventID).First(&delivery).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 1, delivery.Attempts)
	require.NoError(t, makeWebhookDeliveryDue(delivery.ID))
	processDueWebhookDeliveries(context.Background())
	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.WebhookDeliveryPending, delivery.Status)
	assert.Equal(t, 2, delivery.Attempts)
	require.NoError(t, makeWebhookDeliveryDue(delivery.ID))
	processDueWebhookDeliveries(context.Background())
	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, model.WebhookDeliveryFailed, delivery.Status)
	assert.Equal(t, 3, delivery.Attempts)
	processDueWebhookDeliveries(context.Background())
	require.NoError(t, model.DB.First(&delivery, delivery.ID).Error)
	assert.Equal(t, 3, delivery.Attempts)
}

func configureWebhookRetryTest(t *testing.T, maxAttempts, intervalSeconds int) {
	t.Helper()
	setting := async_task_setting.GetAsyncTaskSetting()
	original := *setting
	t.Cleanup(func() {
		*setting = original
		async_task_setting.ApplyNormalization()
	})
	setting.WebhookMaxAttempts = maxAttempts
	setting.WebhookRetryIntervalSeconds = intervalSeconds
	async_task_setting.ApplyNormalization()
}

func makeWebhookDeliveryDue(deliveryID int64) error {
	return model.DB.Model(&model.WebhookDelivery{}).Where("id = ?", deliveryID).
		Update("next_attempt_at", time.Now().Add(-time.Second).Unix()).Error
}

func TestExpiredWebhookDeliveryLeaseIsReclaimedAndFenced(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	putWebhookTestConfig(t, "http://127.0.0.1:18080/hook")
	_, err := CreateAccountWebhookTestDelivery(501)
	require.NoError(t, err)

	claimed, err := model.ClaimDueWebhookDeliveries(20, 30)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, model.DB.Model(&model.WebhookDelivery{}).Where("id = ?", claimed[0].ID).
		Update("locked_until", time.Now().Add(-time.Minute).Unix()).Error)

	reclaimed, err := model.ClaimDueWebhookDeliveries(20, 30)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.NotEqual(t, claimed[0].LockToken, reclaimed[0].LockToken)

	won, err := model.CompleteWebhookDeliveryAttempt(claimed[0], model.WebhookDeliveryResult{Status: model.WebhookDeliveryDelivered})
	require.NoError(t, err)
	assert.False(t, won)
	won, err = model.CompleteWebhookDeliveryAttempt(reclaimed[0], model.WebhookDeliveryResult{Status: model.WebhookDeliveryDelivered})
	require.NoError(t, err)
	assert.True(t, won)
}

func TestWebhookCapacityClaimHonorsPerEndpointLimit(t *testing.T) {
	db := setupOutboundWebhookTestDB(t)
	now := time.Now().Unix()
	endpoints := []*model.WebhookEndpoint{
		{EndpointID: "we_capacity_1", UserID: 501, URL: "http://127.0.0.1:18080/one", Status: model.WebhookEndpointEnabled, CreatedAt: now, UpdatedAt: now},
		{EndpointID: "we_capacity_2", UserID: 502, URL: "http://127.0.0.1:18080/two", Status: model.WebhookEndpointEnabled, CreatedAt: now, UpdatedAt: now},
	}
	for _, endpoint := range endpoints {
		require.NoError(t, db.Create(endpoint).Error)
		for index := 0; index < 5; index++ {
			event := &model.WebhookEvent{
				EventID: fmt.Sprintf("evt_capacity_%d_%d", endpoint.ID, index), UserID: endpoint.UserID,
				EventType: WebhookEventTest, ObjectType: "webhook.test",
				ObjectID: fmt.Sprintf("object_capacity_%d_%d", endpoint.ID, index), Payload: `{}`, CreatedAt: now,
			}
			require.NoError(t, db.Create(event).Error)
			require.NoError(t, db.Create(&model.WebhookDelivery{
				DeliveryID: fmt.Sprintf("whd_capacity_%d_%d", endpoint.ID, index), EventRecordID: event.ID,
				EndpointRecordID: endpoint.ID, Status: model.WebhookDeliveryPending,
				NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
			}).Error)
		}
	}

	claimed, err := model.ClaimDueWebhookDeliveriesForCapacity(10, 30, 2, map[int64]int{endpoints[0].ID: 1})
	require.NoError(t, err)
	require.Len(t, claimed, 3)
	counts := map[int64]int{}
	for _, delivery := range claimed {
		counts[delivery.EndpointRecordID]++
	}
	assert.Equal(t, 1, counts[endpoints[0].ID])
	assert.Equal(t, 2, counts[endpoints[1].ID])
}

func TestWebhookHTTPClientReusesValidatedTransport(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	first, err := newWebhookHTTPClient(context.Background(), "http://127.0.0.1:18080/one")
	require.NoError(t, err)
	second, err := newWebhookHTTPClient(context.Background(), "https://127.0.0.1:18443/two")
	require.NoError(t, err)
	assert.Same(t, first, second)
	assert.Same(t, webhookTransport, first.Transport)
}

func TestWebhookValidationAllowsHTTPAndHTTPSButRejectsPrivateTargetsByDefault(t *testing.T) {
	setupOutboundWebhookTestDB(t)
	t.Setenv("WEBHOOK_ALLOW_INSECURE_LOCAL", "false")
	assert.NoError(t, ValidateWebhookEndpointURL(context.Background(), "http://8.8.8.8/hook"))
	assert.NoError(t, ValidateWebhookEndpointURL(context.Background(), "https://8.8.8.8/hook"))
	client, err := newWebhookHTTPClient(context.Background(), "http://8.8.8.8/hook")
	require.NoError(t, err)
	client.CloseIdleConnections()
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "http://127.0.0.1/hook"), "non-public IP")
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "https://127.0.0.1/hook"), "non-public IP")
	assert.ErrorContains(t, ValidateWebhookEndpointURL(context.Background(), "https://169.254.169.254/latest/meta-data"), "non-public IP")
}
