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
