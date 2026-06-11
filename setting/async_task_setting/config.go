package async_task_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
)

const (
	DefaultTimeoutMinutes = 30
	DefaultQueryLimit     = 1000
)

type TimeoutOverride struct {
	Platform       string `json:"platform"`
	Action         string `json:"action,omitempty"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	Enabled        bool   `json:"enabled"`
}

type AsyncTaskSetting struct {
	DefaultTimeoutMinutes int               `json:"default_timeout_minutes"`
	QueryLimit            int               `json:"query_limit"`
	TimeoutOverrides      []TimeoutOverride `json:"timeout_overrides"`
}

var asyncTaskSetting = AsyncTaskSetting{
	DefaultTimeoutMinutes: DefaultTimeoutMinutes,
	QueryLimit:            DefaultQueryLimit,
	TimeoutOverrides:      []TimeoutOverride{},
}

func init() {
	config.GlobalConfig.Register("async_task_setting", &asyncTaskSetting)
}

func GetAsyncTaskSetting() *AsyncTaskSetting {
	return &asyncTaskSetting
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
	constant.TaskTimeoutMinutes = asyncTaskSetting.DefaultTimeoutMinutes
	constant.TaskQueryLimit = asyncTaskSetting.QueryLimit
}

func ApplyEnvFallback() {
	asyncTaskSetting.DefaultTimeoutMinutes = NormalizeDefaultTimeoutMinutes(constant.TaskTimeoutMinutes)
	asyncTaskSetting.QueryLimit = NormalizeQueryLimit(constant.TaskQueryLimit)
	asyncTaskSetting.TimeoutOverrides = NormalizeSetting(asyncTaskSetting).TimeoutOverrides
	ApplyNormalization()
}

func ResolveTimeoutMinutes(platform constant.TaskPlatform, action string) int {
	setting := NormalizeSetting(asyncTaskSetting)
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
