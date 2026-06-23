package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ImageHandleExecutorConfig holds the new-api side settings for the image-handle executor.
// These values are intentionally global instead of per model channel, because the real
// upstream provider is selected and locked by new-api before the task is sent to image-handle.
type ImageHandleExecutorConfig struct {
	BaseURL          string
	APIKey           string
	InternalBaseURL  string
	InternalSecretID string
	InternalSecret   string
	CallbackSecret   string
	DebugUpstream    bool
}

func GetImageHandleExecutorConfig() ImageHandleExecutorConfig {
	setting := image_handle_setting.GetImageHandleSetting()
	return ImageHandleExecutorConfig{
		BaseURL:          setting.BaseURL,
		APIKey:           setting.APIKey,
		InternalBaseURL:  setting.InternalBaseURL,
		InternalSecretID: setting.InternalSecretID,
		InternalSecret:   setting.InternalSecret,
		CallbackSecret:   setting.CallbackSecret,
		DebugUpstream:    setting.DebugUpstream,
	}
}

func ValidateImageHandleExecutorConfig() error {
	cfg := GetImageHandleExecutorConfig()
	if cfg.BaseURL == "" {
		return fmt.Errorf("IMAGE_HANDLE_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("IMAGE_HANDLE_API_KEY is required")
	}
	if cfg.InternalBaseURL == "" {
		return fmt.Errorf("IMAGE_HANDLE_INTERNAL_BASE_URL is required")
	}
	if cfg.InternalSecretID == "" {
		return fmt.Errorf("IMAGE_HANDLE_INTERNAL_SECRET_ID is required")
	}
	if cfg.InternalSecret == "" {
		return fmt.Errorf("IMAGE_HANDLE_INTERNAL_SECRET is required")
	}
	if cfg.CallbackSecret != "" && cfg.InternalSecret == cfg.CallbackSecret {
		return fmt.Errorf("IMAGE_HANDLE_INTERNAL_SECRET and IMAGE_HANDLE_CALLBACK_SECRET must be different")
	}
	return nil
}

func ValidateImageHandleSubmitConfig() error {
	cfg := GetImageHandleExecutorConfig()
	if cfg.BaseURL == "" {
		return fmt.Errorf("IMAGE_HANDLE_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("IMAGE_HANDLE_API_KEY is required")
	}
	return nil
}

func GetImageHandleSubmitBaseURL() string {
	return GetImageHandleExecutorConfig().BaseURL
}

func GetImageHandleSubmitAPIKey() string {
	return GetImageHandleExecutorConfig().APIKey
}

func BuildImageHandleCredentialLeaseResolveURL(leaseID string) string {
	baseURL := strings.TrimRight(GetImageHandleExecutorConfig().InternalBaseURL, "/")
	return baseURL + "/api/internal/image/credential-leases/" + leaseID + "/resolve"
}

func ResolveImageHandleCallbackSecret(secretID string) (string, bool) {
	cfg := GetImageHandleExecutorConfig()
	if cfg.CallbackSecret == "" {
		return "", false
	}
	if secretID == "" || secretID == "image_handle_callback" || strings.HasPrefix(secretID, "channel_") {
		return cfg.CallbackSecret, true
	}
	return "", false
}

func ResolveImageHandleInternalSecret(secretID string) (string, bool) {
	cfg := GetImageHandleExecutorConfig()
	if cfg.InternalSecret == "" {
		return "", false
	}
	if secretID == "" || secretID == cfg.InternalSecretID {
		return cfg.InternalSecret, true
	}
	return "", false
}

func ImageHandleExecutorConfigured() bool {
	return ValidateImageHandleExecutorConfig() == nil
}

func ImageHandleCallbackAddress() string {
	base := strings.TrimRight(operation_setting.CustomCallbackAddress, "/")
	if base == "" {
		base = strings.TrimRight(GetCallbackAddress(), "/")
	}
	return base
}
