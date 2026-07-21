package async_task_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeWebhookDeliverySettings(t *testing.T) {
	normalized := NormalizeSetting(AsyncTaskSetting{})
	assert.Equal(t, DefaultWebhookMaxAttempts, normalized.WebhookMaxAttempts)
	assert.Equal(t, DefaultWebhookRetryIntervalSeconds, normalized.WebhookRetryIntervalSeconds)

	normalized = NormalizeSetting(AsyncTaskSetting{
		WebhookMaxAttempts:          MaxWebhookMaxAttempts + 1,
		WebhookRetryIntervalSeconds: MaxWebhookRetryIntervalSeconds + 1,
	})
	assert.Equal(t, MaxWebhookMaxAttempts, normalized.WebhookMaxAttempts)
	assert.Equal(t, MaxWebhookRetryIntervalSeconds, normalized.WebhookRetryIntervalSeconds)

	normalized = NormalizeSetting(AsyncTaskSetting{
		WebhookMaxAttempts:          5,
		WebhookRetryIntervalSeconds: 90,
	})
	assert.Equal(t, 5, normalized.WebhookMaxAttempts)
	assert.Equal(t, 90, normalized.WebhookRetryIntervalSeconds)
}

func TestNormalizeWorkerSettings(t *testing.T) {
	normalized := NormalizeSetting(AsyncTaskSetting{})
	assert.Equal(t, DefaultImageDispatchConcurrency, normalized.ImageDispatchConcurrency)
	assert.Equal(t, DefaultWebhookDeliveryConcurrency, normalized.WebhookDeliveryConcurrency)
	assert.Equal(t, DefaultWebhookEndpointConcurrency, normalized.WebhookEndpointConcurrency)
	assert.Equal(t, DefaultImageDispatchTimeoutSeconds, normalized.ImageDispatchTimeoutSeconds)
	assert.Equal(t, DefaultWebhookDeliveryTimeoutSecs, normalized.WebhookDeliveryTimeoutSecs)

	normalized = NormalizeSetting(AsyncTaskSetting{
		ImageDispatchConcurrency:    MaxWorkerConcurrency + 1,
		WebhookDeliveryConcurrency:  3,
		WebhookEndpointConcurrency:  MaxWebhookEndpointConcurrency,
		ImageDispatchTimeoutSeconds: MaxWorkerRequestTimeoutSeconds + 1,
		WebhookDeliveryTimeoutSecs:  MaxWorkerRequestTimeoutSeconds + 1,
	})
	assert.Equal(t, MaxWorkerConcurrency, normalized.ImageDispatchConcurrency)
	assert.Equal(t, 3, normalized.WebhookDeliveryConcurrency)
	assert.Equal(t, 3, normalized.WebhookEndpointConcurrency)
	assert.Equal(t, MaxWorkerRequestTimeoutSeconds, normalized.ImageDispatchTimeoutSeconds)
	assert.Equal(t, MaxWorkerRequestTimeoutSeconds, normalized.WebhookDeliveryTimeoutSecs)
}
