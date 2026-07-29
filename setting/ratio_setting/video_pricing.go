package ratio_setting

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const VideoPricingVersion = 1

var videoPricingState = struct {
	sync.RWMutex
	config types.VideoPricingConfig
}{
	config: emptyVideoPricingConfig(),
}

func emptyVideoPricingConfig() types.VideoPricingConfig {
	return types.VideoPricingConfig{
		Version:       VideoPricingVersion,
		Profiles:      map[string]types.VideoPricingProfile{},
		ModelBindings: map[string]types.VideoPricingBinding{},
	}
}

func cloneVideoPricingConfig(config types.VideoPricingConfig) types.VideoPricingConfig {
	cloned := types.VideoPricingConfig{
		Version:       config.Version,
		Profiles:      make(map[string]types.VideoPricingProfile, len(config.Profiles)),
		ModelBindings: make(map[string]types.VideoPricingBinding, len(config.ModelBindings)),
	}
	for profileID, profile := range config.Profiles {
		cloned.Profiles[profileID] = profile
	}
	for modelName, binding := range config.ModelBindings {
		cloned.ModelBindings[modelName] = binding
	}
	return cloned
}

func normalizeVideoPricingProfile(profileID string, profile types.VideoPricingProfile) (types.VideoPricingProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.BillingMode = strings.ToLower(strings.TrimSpace(profile.BillingMode))
	if profile.Name == "" {
		return profile, fmt.Errorf("视频计价模板 %s 的名称不能为空", profileID)
	}
	if profile.BillingMode != types.VideoPricingModePerSecond {
		return profile, fmt.Errorf("视频计价模板 %s 的 billing_mode 必须是 %s", profileID, types.VideoPricingModePerSecond)
	}
	if math.IsNaN(profile.UnitPrice) || math.IsInf(profile.UnitPrice, 0) || profile.UnitPrice < 0 {
		return profile, fmt.Errorf("视频计价模板 %s 的 unit_price 必须是有限的非负数", profileID)
	}
	return profile, nil
}

func parseVideoPricingConfig(jsonStr string) (types.VideoPricingConfig, error) {
	config := emptyVideoPricingConfig()
	if strings.TrimSpace(jsonStr) == "" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(jsonStr, &config); err != nil {
		return types.VideoPricingConfig{}, fmt.Errorf("视频按秒计价配置不是有效 JSON: %w", err)
	}
	if config.Version != VideoPricingVersion {
		return types.VideoPricingConfig{}, fmt.Errorf("视频按秒计价配置 version 必须为 %d", VideoPricingVersion)
	}
	if config.Profiles == nil {
		config.Profiles = map[string]types.VideoPricingProfile{}
	}
	if config.ModelBindings == nil {
		config.ModelBindings = map[string]types.VideoPricingBinding{}
	}

	normalizedProfiles := make(map[string]types.VideoPricingProfile, len(config.Profiles))
	for rawProfileID, profile := range config.Profiles {
		profileID := strings.TrimSpace(rawProfileID)
		if profileID == "" {
			return types.VideoPricingConfig{}, fmt.Errorf("视频计价模板 ID 不能为空")
		}
		normalized, err := normalizeVideoPricingProfile(profileID, profile)
		if err != nil {
			return types.VideoPricingConfig{}, err
		}
		if _, exists := normalizedProfiles[profileID]; exists {
			return types.VideoPricingConfig{}, fmt.Errorf("视频计价模板 ID %s 重复", profileID)
		}
		normalizedProfiles[profileID] = normalized
	}

	normalizedBindings := make(map[string]types.VideoPricingBinding, len(config.ModelBindings))
	for rawModelName, binding := range config.ModelBindings {
		modelName := strings.TrimSpace(rawModelName)
		binding.Profile = strings.TrimSpace(binding.Profile)
		if modelName == "" {
			return types.VideoPricingConfig{}, fmt.Errorf("视频计价绑定的模型名不能为空")
		}
		if binding.Profile == "" {
			if !binding.SubscriptionEnabled {
				return types.VideoPricingConfig{}, fmt.Errorf("模型 %s 的策略绑定必须启用订阅或引用计价模板", modelName)
			}
		} else if _, exists := normalizedProfiles[binding.Profile]; !exists {
			return types.VideoPricingConfig{}, fmt.Errorf("模型 %s 引用了不存在的视频计价模板 %s", modelName, binding.Profile)
		}
		if _, exists := normalizedBindings[modelName]; exists {
			return types.VideoPricingConfig{}, fmt.Errorf("视频计价模型 %s 重复", modelName)
		}
		normalizedBindings[modelName] = binding
	}

	config.Profiles = normalizedProfiles
	config.ModelBindings = normalizedBindings
	return config, nil
}

func ValidateVideoPricingJSON(jsonStr string) error {
	_, err := parseVideoPricingConfig(jsonStr)
	return err
}

func UpdateVideoPricingByJSONString(jsonStr string) error {
	config, err := parseVideoPricingConfig(jsonStr)
	if err != nil {
		return err
	}
	videoPricingState.Lock()
	videoPricingState.config = config
	videoPricingState.Unlock()
	InvalidateExposedDataCache()
	return nil
}

func VideoPricing2JSONString() string {
	config := GetVideoPricingConfig()
	data, err := common.Marshal(config)
	if err != nil {
		return `{"version":1,"profiles":{},"model_bindings":{}}`
	}
	return string(data)
}

func GetVideoPricingConfig() types.VideoPricingConfig {
	videoPricingState.RLock()
	config := cloneVideoPricingConfig(videoPricingState.config)
	videoPricingState.RUnlock()
	return config
}

func videoPricingProfileHash(profileID string, profile types.VideoPricingProfile) string {
	payload := struct {
		ProfileID string                    `json:"profile_id"`
		Profile   types.VideoPricingProfile `json:"profile"`
	}{ProfileID: profileID, Profile: profile}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func GetVideoPricingBinding(modelName string) (types.VideoPricingBinding, bool) {
	videoPricingState.RLock()
	binding, exists := videoPricingState.config.ModelBindings[modelName]
	videoPricingState.RUnlock()
	return binding, exists
}

func GetVideoPricingForModel(modelName string) (types.VideoPricingProfile, types.VideoPricingBinding, string, bool) {
	videoPricingState.RLock()
	binding, bound := videoPricingState.config.ModelBindings[modelName]
	profile, exists := videoPricingState.config.Profiles[binding.Profile]
	videoPricingState.RUnlock()
	if !bound || binding.Profile == "" || !exists {
		return types.VideoPricingProfile{}, binding, "", false
	}
	return profile, binding, videoPricingProfileHash(binding.Profile, profile), true
}

func IsVideoSubscriptionEnabled(modelName string) bool {
	binding, exists := GetVideoPricingBinding(modelName)
	return exists && binding.SubscriptionEnabled
}

func publicVideoPricing(profileID string, profile types.VideoPricingProfile, binding types.VideoPricingBinding) *types.PublicVideoPricing {
	public := &types.PublicVideoPricing{SubscriptionEnabled: binding.SubscriptionEnabled}
	if profileID != "" {
		public.ProfileID = profileID
		public.Name = profile.Name
		public.BillingMode = profile.BillingMode
		public.Unit = types.VideoPricingUnitSecond
		public.UnitPrice = profile.UnitPrice
	}
	return public
}

func GetPublicVideoPricingSnapshot() map[string]*types.PublicVideoPricing {
	videoPricingState.RLock()
	snapshot := make(map[string]*types.PublicVideoPricing, len(videoPricingState.config.ModelBindings))
	for modelName, binding := range videoPricingState.config.ModelBindings {
		profile := videoPricingState.config.Profiles[binding.Profile]
		snapshot[modelName] = publicVideoPricing(binding.Profile, profile, binding)
	}
	videoPricingState.RUnlock()
	return snapshot
}
