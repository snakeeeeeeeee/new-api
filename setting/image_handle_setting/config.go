package image_handle_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const DefaultInternalSecretID = "image_handle_1"

type ImageHandleSetting struct {
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
	InternalBaseURL  string `json:"internal_base_url"`
	InternalSecretID string `json:"internal_secret_id"`
	InternalSecret   string `json:"internal_secret"`
	CallbackSecret   string `json:"callback_secret"`
}

var imageHandleSetting = envFallbackSetting()

func init() {
	config.GlobalConfig.Register("image_handle_setting", &imageHandleSetting)
}

func envFallbackSetting() ImageHandleSetting {
	return NormalizeSetting(ImageHandleSetting{
		BaseURL:          common.GetEnvOrDefaultString("IMAGE_HANDLE_BASE_URL", ""),
		APIKey:           common.GetEnvOrDefaultString("IMAGE_HANDLE_API_KEY", ""),
		InternalBaseURL:  common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_BASE_URL", ""),
		InternalSecretID: common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_SECRET_ID", DefaultInternalSecretID),
		InternalSecret:   common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_SECRET", ""),
		CallbackSecret:   common.GetEnvOrDefaultString("IMAGE_HANDLE_CALLBACK_SECRET", ""),
	})
}

func GetImageHandleSetting() *ImageHandleSetting {
	return &imageHandleSetting
}

func NormalizeSetting(setting ImageHandleSetting) ImageHandleSetting {
	setting.BaseURL = strings.TrimRight(strings.TrimSpace(setting.BaseURL), "/")
	setting.APIKey = strings.TrimSpace(setting.APIKey)
	setting.InternalBaseURL = strings.TrimRight(strings.TrimSpace(setting.InternalBaseURL), "/")
	setting.InternalSecretID = strings.TrimSpace(setting.InternalSecretID)
	if setting.InternalSecretID == "" {
		setting.InternalSecretID = DefaultInternalSecretID
	}
	setting.InternalSecret = strings.TrimSpace(setting.InternalSecret)
	setting.CallbackSecret = strings.TrimSpace(setting.CallbackSecret)
	return setting
}

func ApplyEnvFallback() {
	imageHandleSetting = envFallbackSetting()
}

func ApplyNormalization() {
	imageHandleSetting = NormalizeSetting(imageHandleSetting)
}

func Validate(setting ImageHandleSetting) error {
	setting = NormalizeSetting(setting)
	if setting.BaseURL == "" {
		return fmt.Errorf("image-handle 服务地址不能为空")
	}
	if setting.APIKey == "" {
		return fmt.Errorf("image-handle API Key 不能为空")
	}
	if setting.InternalBaseURL == "" {
		return fmt.Errorf("internal execute 访问地址不能为空")
	}
	if setting.InternalSecretID == "" {
		return fmt.Errorf("internal execute Secret ID 不能为空")
	}
	if setting.InternalSecret == "" {
		return fmt.Errorf("internal execute Secret 不能为空")
	}
	if setting.CallbackSecret != "" && setting.InternalSecret == setting.CallbackSecret {
		return fmt.Errorf("internal execute Secret 不能和 callback 兜底 Secret 相同")
	}
	return nil
}
