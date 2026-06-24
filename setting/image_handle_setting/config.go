package image_handle_setting

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const DefaultInternalSecretID = "image_handle_1"

type ImageHandleSetting struct {
	BaseURL                 string  `json:"base_url"`
	APIKey                  string  `json:"api_key"`
	InternalBaseURL         string  `json:"internal_base_url"`
	InternalSecretID        string  `json:"internal_secret_id"`
	InternalSecret          string  `json:"internal_secret"`
	CallbackSecret          string  `json:"callback_secret"`
	DebugUpstream           bool    `json:"debug_upstream"`
	UsagePrechargeEnabled   bool    `json:"usage_precharge_enabled"`
	PrechargeAmountPerImage float64 `json:"precharge_amount_per_image"`
	PrechargeQuotaPerImage  int     `json:"precharge_quota_per_image"`
}

var imageHandleSetting = envFallbackSetting()

func init() {
	config.GlobalConfig.Register("image_handle_setting", &imageHandleSetting)
}

func envFallbackSetting() ImageHandleSetting {
	return NormalizeSetting(ImageHandleSetting{
		BaseURL:                 common.GetEnvOrDefaultString("IMAGE_HANDLE_BASE_URL", ""),
		APIKey:                  common.GetEnvOrDefaultString("IMAGE_HANDLE_API_KEY", ""),
		InternalBaseURL:         common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_BASE_URL", ""),
		InternalSecretID:        common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_SECRET_ID", DefaultInternalSecretID),
		InternalSecret:          common.GetEnvOrDefaultString("IMAGE_HANDLE_INTERNAL_SECRET", ""),
		CallbackSecret:          common.GetEnvOrDefaultString("IMAGE_HANDLE_CALLBACK_SECRET", ""),
		DebugUpstream:           false,
		UsagePrechargeEnabled:   common.GetEnvOrDefaultBool("IMAGE_HANDLE_USAGE_PRECHARGE_ENABLED", true),
		PrechargeAmountPerImage: getEnvOrDefaultFloat64("IMAGE_HANDLE_PRECHARGE_AMOUNT_PER_IMAGE", 0),
		PrechargeQuotaPerImage:  common.GetEnvOrDefault("IMAGE_HANDLE_PRECHARGE_QUOTA_PER_IMAGE", 0),
	})
}

func getEnvOrDefaultFloat64(env string, defaultValue float64) float64 {
	if env == "" || os.Getenv(env) == "" {
		return defaultValue
	}
	num, err := strconv.ParseFloat(os.Getenv(env), 64)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to parse %s: %s, using default value: %f", env, err.Error(), defaultValue))
		return defaultValue
	}
	return num
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
	if setting.PrechargeQuotaPerImage < 0 {
		setting.PrechargeQuotaPerImage = 0
	}
	if setting.PrechargeAmountPerImage < 0 {
		setting.PrechargeAmountPerImage = 0
	}
	if setting.PrechargeAmountPerImage > 0 && common.QuotaPerUnit > 0 {
		quota := decimal.NewFromFloat(setting.PrechargeAmountPerImage).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
			Round(0).
			IntPart()
		if quota > 0 {
			setting.PrechargeQuotaPerImage = int(quota)
		}
	}
	if setting.PrechargeAmountPerImage == 0 && setting.PrechargeQuotaPerImage > 0 && common.QuotaPerUnit > 0 {
		setting.PrechargeAmountPerImage = float64(setting.PrechargeQuotaPerImage) / common.QuotaPerUnit
	}
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
		return fmt.Errorf("internal resolve 访问地址不能为空")
	}
	if setting.InternalSecretID == "" {
		return fmt.Errorf("internal resolve Secret ID 不能为空")
	}
	if setting.InternalSecret == "" {
		return fmt.Errorf("internal resolve Secret 不能为空")
	}
	if setting.CallbackSecret != "" && setting.InternalSecret == setting.CallbackSecret {
		return fmt.Errorf("internal resolve Secret 不能和 callback 兜底 Secret 相同")
	}
	if setting.PrechargeQuotaPerImage < 0 {
		return fmt.Errorf("每张图预扣额度不能为负数")
	}
	if setting.PrechargeAmountPerImage < 0 {
		return fmt.Errorf("每张图预扣费用不能为负数")
	}
	return nil
}
