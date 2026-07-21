package async_task_setting

import (
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	DefaultTimeoutMinutes              = 30
	DefaultQueryLimit                  = 1000
	DefaultWebhookMaxAttempts          = 3
	DefaultWebhookRetryIntervalSeconds = 30
	MaxWebhookMaxAttempts              = 10
	MaxWebhookRetryIntervalSeconds     = 3600
	DefaultImageDispatchConcurrency    = 20
	DefaultWebhookDeliveryConcurrency  = 20
	DefaultWebhookEndpointConcurrency  = 2
	MaxWorkerConcurrency               = 100
	MaxWebhookEndpointConcurrency      = 10
	DefaultImageDispatchTimeoutSeconds = 30
	DefaultWebhookDeliveryTimeoutSecs  = 10
	MaxWorkerRequestTimeoutSeconds     = 300
)

type TimeoutOverride struct {
	Platform       string `json:"platform"`
	Action         string `json:"action,omitempty"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	Enabled        bool   `json:"enabled"`
}

type AsyncTaskSetting struct {
	DefaultTimeoutMinutes       int               `json:"default_timeout_minutes"`
	QueryLimit                  int               `json:"query_limit"`
	WebhookMaxAttempts          int               `json:"webhook_max_attempts"`
	WebhookRetryIntervalSeconds int               `json:"webhook_retry_interval_seconds"`
	ImageDispatchConcurrency    int               `json:"image_dispatch_concurrency"`
	WebhookDeliveryConcurrency  int               `json:"webhook_delivery_concurrency"`
	WebhookEndpointConcurrency  int               `json:"webhook_endpoint_concurrency"`
	ImageDispatchTimeoutSeconds int               `json:"image_dispatch_request_timeout_seconds"`
	WebhookDeliveryTimeoutSecs  int               `json:"webhook_delivery_request_timeout_seconds"`
	TimeoutOverrides            []TimeoutOverride `json:"timeout_overrides"`
}

var asyncTaskSetting = AsyncTaskSetting{
	DefaultTimeoutMinutes:       DefaultTimeoutMinutes,
	QueryLimit:                  DefaultQueryLimit,
	WebhookMaxAttempts:          DefaultWebhookMaxAttempts,
	WebhookRetryIntervalSeconds: DefaultWebhookRetryIntervalSeconds,
	ImageDispatchConcurrency:    DefaultImageDispatchConcurrency,
	WebhookDeliveryConcurrency:  DefaultWebhookDeliveryConcurrency,
	WebhookEndpointConcurrency:  DefaultWebhookEndpointConcurrency,
	ImageDispatchTimeoutSeconds: DefaultImageDispatchTimeoutSeconds,
	WebhookDeliveryTimeoutSecs:  DefaultWebhookDeliveryTimeoutSecs,
	TimeoutOverrides:            []TimeoutOverride{},
}

var asyncTaskSettingSnapshot atomic.Value

func init() {
	config.GlobalConfig.Register("async_task_setting", &asyncTaskSetting)
	asyncTaskSettingSnapshot.Store(asyncTaskSetting)
}

func GetAsyncTaskSetting() *AsyncTaskSetting {
	return &asyncTaskSetting
}

// GetSnapshot returns an immutable normalized copy for concurrent workers.
func GetSnapshot() AsyncTaskSetting {
	if snapshot := asyncTaskSettingSnapshot.Load(); snapshot != nil {
		return snapshot.(AsyncTaskSetting)
	}
	return NormalizeSetting(asyncTaskSetting)
}

func NormalizeDefaultTimeoutMinutes(v int) int {
	if v <= 0 {
		return DefaultTimeoutMinutes
	}
	return v
}

func NormalizeQueryLimit(v int) int {
	if v <= 0 {
		return DefaultQueryLimit
	}
	return v
}

func NormalizeWebhookMaxAttempts(v int) int {
	if v <= 0 {
		return DefaultWebhookMaxAttempts
	}
	if v > MaxWebhookMaxAttempts {
		return MaxWebhookMaxAttempts
	}
	return v
}

func NormalizeWebhookRetryIntervalSeconds(v int) int {
	if v <= 0 {
		return DefaultWebhookRetryIntervalSeconds
	}
	if v > MaxWebhookRetryIntervalSeconds {
		return MaxWebhookRetryIntervalSeconds
	}
	return v
}

func normalizeWorkerConcurrency(v int, defaultValue int) int {
	if v <= 0 {
		return defaultValue
	}
	if v > MaxWorkerConcurrency {
		return MaxWorkerConcurrency
	}
	return v
}

func NormalizeImageDispatchConcurrency(v int) int {
	return normalizeWorkerConcurrency(v, DefaultImageDispatchConcurrency)
}

func NormalizeWebhookDeliveryConcurrency(v int) int {
	return normalizeWorkerConcurrency(v, DefaultWebhookDeliveryConcurrency)
}

func NormalizeWebhookEndpointConcurrency(v int) int {
	if v <= 0 {
		return DefaultWebhookEndpointConcurrency
	}
	if v > MaxWebhookEndpointConcurrency {
		return MaxWebhookEndpointConcurrency
	}
	return v
}

func normalizeWorkerRequestTimeout(v int, defaultValue int) int {
	if v <= 0 {
		return defaultValue
	}
	if v > MaxWorkerRequestTimeoutSeconds {
		return MaxWorkerRequestTimeoutSeconds
	}
	return v
}

func NormalizeImageDispatchTimeoutSeconds(v int) int {
	return normalizeWorkerRequestTimeout(v, DefaultImageDispatchTimeoutSeconds)
}

func NormalizeWebhookDeliveryTimeoutSeconds(v int) int {
	return normalizeWorkerRequestTimeout(v, DefaultWebhookDeliveryTimeoutSecs)
}

func NormalizeOverride(override TimeoutOverride) TimeoutOverride {
	override.Platform = strings.TrimSpace(override.Platform)
	override.Action = strings.TrimSpace(override.Action)
	if override.TimeoutMinutes <= 0 {
		override.TimeoutMinutes = DefaultTimeoutMinutes
	}
	return override
}

func NormalizeSetting(setting AsyncTaskSetting) AsyncTaskSetting {
	setting.DefaultTimeoutMinutes = NormalizeDefaultTimeoutMinutes(setting.DefaultTimeoutMinutes)
	setting.QueryLimit = NormalizeQueryLimit(setting.QueryLimit)
	setting.WebhookMaxAttempts = NormalizeWebhookMaxAttempts(setting.WebhookMaxAttempts)
	setting.WebhookRetryIntervalSeconds = NormalizeWebhookRetryIntervalSeconds(setting.WebhookRetryIntervalSeconds)
	setting.ImageDispatchConcurrency = NormalizeImageDispatchConcurrency(setting.ImageDispatchConcurrency)
	setting.WebhookDeliveryConcurrency = NormalizeWebhookDeliveryConcurrency(setting.WebhookDeliveryConcurrency)
	setting.WebhookEndpointConcurrency = NormalizeWebhookEndpointConcurrency(setting.WebhookEndpointConcurrency)
	if setting.WebhookEndpointConcurrency > setting.WebhookDeliveryConcurrency {
		setting.WebhookEndpointConcurrency = setting.WebhookDeliveryConcurrency
	}
	setting.ImageDispatchTimeoutSeconds = NormalizeImageDispatchTimeoutSeconds(setting.ImageDispatchTimeoutSeconds)
	setting.WebhookDeliveryTimeoutSecs = NormalizeWebhookDeliveryTimeoutSeconds(setting.WebhookDeliveryTimeoutSecs)
	overrides := make([]TimeoutOverride, 0, len(setting.TimeoutOverrides))
	for _, override := range setting.TimeoutOverrides {
		override = NormalizeOverride(override)
		if override.Platform == "" || !override.Enabled {
			continue
		}
		overrides = append(overrides, override)
	}
	setting.TimeoutOverrides = overrides
	return setting
}

func ApplyNormalization() {
	asyncTaskSetting = NormalizeSetting(asyncTaskSetting)
	asyncTaskSettingSnapshot.Store(asyncTaskSetting)
	constant.TaskTimeoutMinutes = asyncTaskSetting.DefaultTimeoutMinutes
	constant.TaskQueryLimit = asyncTaskSetting.QueryLimit
}

func ApplyEnvFallback() {
	asyncTaskSetting.DefaultTimeoutMinutes = NormalizeDefaultTimeoutMinutes(constant.TaskTimeoutMinutes)
	asyncTaskSetting.QueryLimit = NormalizeQueryLimit(constant.TaskQueryLimit)
	asyncTaskSetting.WebhookMaxAttempts = NormalizeWebhookMaxAttempts(asyncTaskSetting.WebhookMaxAttempts)
	asyncTaskSetting.WebhookRetryIntervalSeconds = NormalizeWebhookRetryIntervalSeconds(asyncTaskSetting.WebhookRetryIntervalSeconds)
	asyncTaskSetting.ImageDispatchConcurrency = NormalizeImageDispatchConcurrency(asyncTaskSetting.ImageDispatchConcurrency)
	asyncTaskSetting.WebhookDeliveryConcurrency = NormalizeWebhookDeliveryConcurrency(asyncTaskSetting.WebhookDeliveryConcurrency)
	asyncTaskSetting.WebhookEndpointConcurrency = NormalizeWebhookEndpointConcurrency(asyncTaskSetting.WebhookEndpointConcurrency)
	asyncTaskSetting.ImageDispatchTimeoutSeconds = NormalizeImageDispatchTimeoutSeconds(asyncTaskSetting.ImageDispatchTimeoutSeconds)
	asyncTaskSetting.WebhookDeliveryTimeoutSecs = NormalizeWebhookDeliveryTimeoutSeconds(asyncTaskSetting.WebhookDeliveryTimeoutSecs)
	asyncTaskSetting.TimeoutOverrides = NormalizeSetting(asyncTaskSetting).TimeoutOverrides
	ApplyNormalization()
}

func ResolveTimeoutMinutes(platform constant.TaskPlatform, action string) int {
	setting := GetSnapshot()
	platformValue := strings.TrimSpace(string(platform))
	actionValue := strings.TrimSpace(action)
	platformTimeout := 0
	for _, override := range setting.TimeoutOverrides {
		if override.Platform != platformValue {
			continue
		}
		if override.Action == actionValue && override.Action != "" {
			return override.TimeoutMinutes
		}
		if override.Action == "" && platformTimeout == 0 {
			platformTimeout = override.TimeoutMinutes
		}
	}
	if platformTimeout > 0 {
		return platformTimeout
	}
	return setting.DefaultTimeoutMinutes
}
