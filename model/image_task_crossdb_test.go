package model

import (
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TEST_MYSQL_DSN and TEST_POSTGRES_DSN must point to disposable, empty databases.
// MySQL must use a Chinese-capable charset accepted by checkMySQLChineseSupport.
func TestImageTaskAndWebhookSchemaAcrossDatabases(t *testing.T) {
	targets := []struct {
		name      string
		dialector gorm.Dialector
	}{
		{name: "sqlite", dialector: sqlite.Open(":memory:")},
	}
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		targets = append(targets, struct {
			name      string
			dialector gorm.Dialector
		}{name: "mysql", dialector: mysql.Open(dsn)})
	}
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		targets = append(targets, struct {
			name      string
			dialector gorm.Dialector
		}{name: "postgres", dialector: postgres.Open(dsn)})
	}

	for _, target := range targets {
		target := target
		t.Run(target.name, func(t *testing.T) {
			db, err := gorm.Open(target.dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			originalDB := DB
			DB = db
			t.Cleanup(func() { DB = originalDB })

			models := []any{
				&ImageTaskRequest{}, &VideoTaskRequest{}, &ImageTaskRetryState{}, &ImageTaskAttempt{},
				&ImageTaskDispatch{}, &ImageCredentialLease{}, &WebhookEndpoint{},
				&WebhookEvent{}, &WebhookDelivery{}, &WebhookDeliveryAttempt{},
			}
			require.NoError(t, db.AutoMigrate(models...))
			require.True(t, db.Migrator().HasIndex(&ImageTaskRequest{}, "idx_image_task_idempotency"))
			require.True(t, db.Migrator().HasIndex(&VideoTaskRequest{}, "idx_video_task_idempotency"))
			require.True(t, db.Migrator().HasIndex(&ImageTaskAttempt{}, "idx_image_attempt_task_number"))
			require.True(t, db.Migrator().HasIndex(&ImageTaskDispatch{}, "idx_image_dispatch_task_record"))
			require.True(t, db.Migrator().HasIndex(&ImageTaskDispatch{}, "idx_image_dispatch_attempt"))
			require.True(t, db.Migrator().HasIndex(&WebhookEvent{}, "idx_webhook_event_object"))
			require.True(t, db.Migrator().HasIndex(&WebhookDelivery{}, "idx_webhook_delivery_target"))
			require.True(t, db.Migrator().HasIndex(&WebhookEndpoint{}, "idx_webhook_config_owner"))

			now := time.Now().Unix()
			idempotencyKey := "cross-db-key"
			requestJSON := `{"model":"gpt-image-2","count":0,"enabled":false}`
			request := &ImageTaskRequest{
				TaskRecordID: 1, TaskID: "task_cross_db_1", UserID: 101,
				IdempotencyKey: &idempotencyKey, RequestFingerprint: "fingerprint-1",
				ClientReferenceID: "order_cross_db", RequestJSON: requestJSON,
				CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(request).Error)
			videoKey := "cross-db-video-key"
			videoRequest := &VideoTaskRequest{
				TaskRecordID: 2, TaskID: "task_cross_db_video", UserID: 101,
				IdempotencyKey: &videoKey, RequestFingerprint: "video-fingerprint",
				ClientReferenceID: "video_order", RequestJSON: `{"operation":"edit","output":{"duration":0}}`,
				CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(videoRequest).Error)

			dispatch := &ImageTaskDispatch{
				DispatchID: "dispatch_cross_db_1", TaskRecordID: 1, TaskID: request.TaskID,
				RequestBody: requestJSON, Status: ImageTaskDispatchPending,
				NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(dispatch).Error)

			state := &ImageTaskRetryState{
				TaskRecordID: 1, TaskID: request.TaskID, Status: ImageTaskRetryStateActive,
				RetryLimit: 1, TokenGroup: "aggregate-images", UserGroup: "default", OriginalModel: "gpt-image-2",
				CurrentRouteGroup: "image-primary", CurrentRouteIndex: 0, CurrentGroupAttempts: 2, AttemptCount: 2,
				FailedChannelIDs: []int{7}, AttemptedRouteKeys: []string{"default:image-primary"},
				RouteTrace: []ImageTaskRouteTraceEntry{{AttemptNumber: 1, ChannelID: 7, RouteGroup: "image-primary", Status: ImageTaskAttemptFailed, CreatedAt: now}},
				CreatedAt:  now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(state).Error)
			firstAttempt := &ImageTaskAttempt{
				AttemptID: "task_attempt_cross_db_1", ClientTaskID: "task_attempt_cross_db_1",
				TaskRecordID: 1, TaskID: request.TaskID, AttemptNumber: 1, ChannelID: 7,
				RouteGroup: "image-primary", RouteIndex: 0, OriginModel: "gpt-image-2", UpstreamModel: "gpt-image-2-upstream",
				Status: ImageTaskAttemptFailed, ErrorRetryable: true, Quota: 123,
				BillingContext: TaskBillingContext{OriginModelName: "gpt-image-2", ModelPrice: 0.01, GroupRatio: 1.5},
				CreatedAt:      now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(firstAttempt).Error)
			secondAttempt := &ImageTaskAttempt{
				AttemptID: "task_attempt_cross_db_2", ClientTaskID: "task_attempt_cross_db_2",
				TaskRecordID: 1, TaskID: request.TaskID, AttemptNumber: 2, ChannelID: 8,
				RouteGroup: "image-primary", RouteIndex: 0, OriginModel: "gpt-image-2", UpstreamModel: "gpt-image-2-b",
				Status: ImageTaskAttemptPending, Quota: 150,
				BillingContext: TaskBillingContext{OriginModelName: "gpt-image-2", ModelPrice: 0.02, GroupRatio: 1.5},
				CreatedAt:      now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(secondAttempt).Error)
			state.ActiveAttemptRecordID = secondAttempt.ID
			require.NoError(t, db.Model(state).Update("active_attempt_record_id", secondAttempt.ID).Error)
			for index, attempt := range []*ImageTaskAttempt{firstAttempt, secondAttempt} {
				attemptID := attempt.ID
				retryDispatch := &ImageTaskDispatch{
					DispatchID: "dispatch_cross_db_retry_" + string(rune('1'+index)), TaskRecordID: 1,
					AttemptRecordID: &attemptID, TaskID: request.TaskID, RequestBody: requestJSON,
					Status: ImageTaskDispatchPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
				}
				require.NoError(t, db.Create(retryDispatch).Error)
				lease := &ImageCredentialLease{
					LeaseID: "lease_cross_db_retry_" + string(rune('1'+index)), TaskID: request.TaskID,
					TaskRecordID: 1, AttemptRecordID: &attemptID, UserID: 101, ChannelID: attempt.ChannelID,
					Operation: "generation", Model: attempt.UpstreamModel, Status: ImageCredentialLeaseStatusActive,
					ExpiresAt: now + 1800, CreatedAt: now, UpdatedAt: now,
				}
				require.NoError(t, db.Create(lease).Error)
			}
			duplicateAttemptDispatchID := firstAttempt.ID
			require.Error(t, db.Create(&ImageTaskDispatch{
				DispatchID: "dispatch_cross_db_retry_duplicate", TaskRecordID: 1, AttemptRecordID: &duplicateAttemptDispatchID,
				TaskID: request.TaskID, RequestBody: requestJSON, Status: ImageTaskDispatchPending,
				NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
			}).Error)

			var restoredState ImageTaskRetryState
			require.NoError(t, db.First(&restoredState, state.ID).Error)
			require.Equal(t, 1, restoredState.RetryLimit)
			require.Equal(t, []int{7}, restoredState.FailedChannelIDs)
			require.Equal(t, []string{"default:image-primary"}, restoredState.AttemptedRouteKeys)
			require.Len(t, restoredState.RouteTrace, 1)
			var restoredAttempt ImageTaskAttempt
			require.NoError(t, db.First(&restoredAttempt, secondAttempt.ID).Error)
			require.Equal(t, 0.02, restoredAttempt.BillingContext.ModelPrice)

			configOwnerID := 101
			endpoint := &WebhookEndpoint{
				EndpointID: "we_cross_db_1", UserID: 101, Name: "cross-db",
				ConfigOwnerID: &configOwnerID, AuthKeyEncrypted: "v1:encrypted",
				URL: "https://example.com/hooks/image", Status: WebhookEndpointEnabled,
				EventTypes: `["image.task.succeeded","image.task.failed"]`, APIVersion: "2026-07-17",
				SecretSalt: "salt", SecretVersion: 1, CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(endpoint).Error)
			duplicateConfig := *endpoint
			duplicateConfig.ID = 0
			duplicateConfig.EndpointID = "we_cross_db_2"
			require.Error(t, db.Create(&duplicateConfig).Error)

			event := &WebhookEvent{
				EventID: "evt_cross_db_1", UserID: 101, EventType: "image.task.succeeded",
				ObjectType: "image.task", ObjectID: request.TaskID, APIVersion: "2026-07-17",
				Payload: `{"object":"event","metadata":{"language":"中文"}}`, CreatedAt: now,
			}
			require.NoError(t, db.Create(event).Error)

			delivery := &WebhookDelivery{
				DeliveryID: "whd_cross_db_1", EventRecordID: event.ID, EndpointRecordID: endpoint.ID,
				Status: WebhookDeliveryPending, NextAttemptAt: now, RetryDeadline: now + 3600,
				CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, db.Create(delivery).Error)
			require.NoError(t, db.Create(&WebhookDeliveryAttempt{
				AttemptID: "wha_cross_db_1", DeliveryRecordID: delivery.ID, AttemptNumber: 1,
				HTTPStatus: 500, Error: "temporary", ResponseBody: `{"retry":true}`,
				DurationMS: 12, CreatedAt: now,
			}).Error)
			imageQueue, err := GetImageTaskDispatchQueueStats(now)
			require.NoError(t, err)
			require.EqualValues(t, 3, imageQueue.Pending)
			webhookQueue, err := GetWebhookDeliveryQueueStats(now)
			require.NoError(t, err)
			require.EqualValues(t, 1, webhookQueue.Pending)
			deliveries, events, endpoints, total, err := ListAdminWebhookDeliveries(0, 20, AdminWebhookDeliveryQuery{UserID: 101})
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Len(t, deliveries, 1)
			require.Contains(t, events, event.ID)
			require.Contains(t, endpoints, endpoint.ID)

			var storedRequest ImageTaskRequest
			require.NoError(t, db.First(&storedRequest, request.ID).Error)
			require.Equal(t, requestJSON, storedRequest.RequestJSON)
			var storedEvent WebhookEvent
			require.NoError(t, db.First(&storedEvent, event.ID).Error)
			require.Equal(t, event.Payload, storedEvent.Payload)

			conflictingRequest := *request
			conflictingRequest.ID = 0
			conflictingRequest.TaskRecordID = 3
			conflictingRequest.TaskID = "task_cross_db_2"
			require.Error(t, db.Create(&conflictingRequest).Error)
			conflictingVideoRequest := *videoRequest
			conflictingVideoRequest.ID = 0
			conflictingVideoRequest.TaskRecordID = 4
			conflictingVideoRequest.TaskID = "task_cross_db_video_2"
			require.Error(t, db.Create(&conflictingVideoRequest).Error)

			duplicateEvent := *event
			duplicateEvent.ID = 0
			duplicateEvent.EventID = "evt_cross_db_2"
			require.Error(t, db.Create(&duplicateEvent).Error)

			duplicateDelivery := *delivery
			duplicateDelivery.ID = 0
			duplicateDelivery.DeliveryID = "whd_cross_db_2"
			require.Error(t, db.Create(&duplicateDelivery).Error)
		})
	}
}
