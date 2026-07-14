package ratio_setting

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

const ImagePricingVersion = 1

var imagePricingState = struct {
	sync.RWMutex
	config types.ImagePricingConfig
}{
	config: emptyImagePricingConfig(),
}

func emptyImagePricingConfig() types.ImagePricingConfig {
	return types.ImagePricingConfig{
		Version:       ImagePricingVersion,
		Profiles:      map[string]types.ImagePricingProfile{},
		ModelBindings: map[string]types.ImagePricingBinding{},
	}
}

func cloneImagePricingProfile(profile types.ImagePricingProfile) types.ImagePricingProfile {
	cloned := profile
	cloned.Tiers = append([]types.ImagePricingTier(nil), profile.Tiers...)
	for index := range cloned.Tiers {
		cloned.Tiers[index].Aliases = append([]string(nil), profile.Tiers[index].Aliases...)
	}
	return cloned
}

func cloneImagePricingConfig(config types.ImagePricingConfig) types.ImagePricingConfig {
	cloned := types.ImagePricingConfig{
		Version:       config.Version,
		Profiles:      make(map[string]types.ImagePricingProfile, len(config.Profiles)),
		ModelBindings: make(map[string]types.ImagePricingBinding, len(config.ModelBindings)),
	}
	for profileID, profile := range config.Profiles {
		cloned.Profiles[profileID] = cloneImagePricingProfile(profile)
	}
	for modelName, binding := range config.ModelBindings {
		cloned.ModelBindings[modelName] = binding
	}
	return cloned
}

func normalizeImagePricingProfile(profileID string, profile types.ImagePricingProfile) (types.ImagePricingProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Parameter = strings.ToLower(strings.TrimSpace(profile.Parameter))
	profile.DefaultTier = strings.TrimSpace(profile.DefaultTier)
	if profile.Name == "" {
		return profile, fmt.Errorf("图片计价模板 %s 的名称不能为空", profileID)
	}
	switch profile.Parameter {
	case types.ImagePricingParameterQuality, types.ImagePricingParameterSize, types.ImagePricingParameterResolution:
	default:
		return profile, fmt.Errorf("图片计价模板 %s 的 parameter 必须是 quality、size 或 resolution", profileID)
	}
	if len(profile.Tiers) == 0 {
		return profile, fmt.Errorf("图片计价模板 %s 至少需要一个档位", profileID)
	}

	seenValues := make(map[string]string)
	defaultTier := ""
	for index := range profile.Tiers {
		tier := &profile.Tiers[index]
		tier.Key = strings.TrimSpace(tier.Key)
		tier.UpstreamValue = strings.TrimSpace(tier.UpstreamValue)
		if tier.Key == "" {
			return profile, fmt.Errorf("图片计价模板 %s 第 %d 档 key 不能为空", profileID, index+1)
		}
		if tier.UpstreamValue == "" {
			return profile, fmt.Errorf("图片计价模板 %s 档位 %s 的 upstream_value 不能为空", profileID, tier.Key)
		}
		if math.IsNaN(tier.UnitPrice) || math.IsInf(tier.UnitPrice, 0) || tier.UnitPrice < 0 {
			return profile, fmt.Errorf("图片计价模板 %s 档位 %s 的 unit_price 必须是有限的非负数", profileID, tier.Key)
		}

		keyLookup := strings.ToLower(tier.Key)
		if previous, exists := seenValues[keyLookup]; exists {
			return profile, fmt.Errorf("图片计价模板 %s 的值 %s 与 %s 重复", profileID, tier.Key, previous)
		}
		seenValues[keyLookup] = tier.Key
		if strings.EqualFold(profile.DefaultTier, tier.Key) {
			defaultTier = tier.Key
		}

		aliases := make([]string, 0, len(tier.Aliases))
		for _, alias := range tier.Aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				return profile, fmt.Errorf("图片计价模板 %s 档位 %s 的别名不能为空", profileID, tier.Key)
			}
			lookup := strings.ToLower(alias)
			if previous, exists := seenValues[lookup]; exists {
				return profile, fmt.Errorf("图片计价模板 %s 的值 %s 与 %s 重复", profileID, alias, previous)
			}
			seenValues[lookup] = alias
			aliases = append(aliases, alias)
		}
		tier.Aliases = aliases
	}
	if defaultTier == "" {
		return profile, fmt.Errorf("图片计价模板 %s 的 default_tier %s 不存在", profileID, profile.DefaultTier)
	}
	profile.DefaultTier = defaultTier
	return profile, nil
}

func parseImagePricingConfig(jsonStr string) (types.ImagePricingConfig, error) {
	config := emptyImagePricingConfig()
	if strings.TrimSpace(jsonStr) == "" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(jsonStr, &config); err != nil {
		return types.ImagePricingConfig{}, fmt.Errorf("图片参数计价配置不是有效 JSON: %w", err)
	}
	if config.Version != ImagePricingVersion {
		return types.ImagePricingConfig{}, fmt.Errorf("图片参数计价配置 version 必须为 %d", ImagePricingVersion)
	}
	if config.Profiles == nil {
		config.Profiles = map[string]types.ImagePricingProfile{}
	}
	if config.ModelBindings == nil {
		config.ModelBindings = map[string]types.ImagePricingBinding{}
	}

	normalizedProfiles := make(map[string]types.ImagePricingProfile, len(config.Profiles))
	for rawProfileID, profile := range config.Profiles {
		profileID := strings.TrimSpace(rawProfileID)
		if profileID == "" {
			return types.ImagePricingConfig{}, fmt.Errorf("图片计价模板 ID 不能为空")
		}
		normalized, err := normalizeImagePricingProfile(profileID, profile)
		if err != nil {
			return types.ImagePricingConfig{}, err
		}
		if _, exists := normalizedProfiles[profileID]; exists {
			return types.ImagePricingConfig{}, fmt.Errorf("图片计价模板 ID %s 重复", profileID)
		}
		normalizedProfiles[profileID] = normalized
	}

	normalizedBindings := make(map[string]types.ImagePricingBinding, len(config.ModelBindings))
	for rawModelName, binding := range config.ModelBindings {
		modelName := strings.TrimSpace(rawModelName)
		binding.Profile = strings.TrimSpace(binding.Profile)
		if modelName == "" {
			return types.ImagePricingConfig{}, fmt.Errorf("图片参数计价绑定的模型名不能为空")
		}
		if _, exists := normalizedProfiles[binding.Profile]; !exists {
			return types.ImagePricingConfig{}, fmt.Errorf("模型 %s 引用了不存在的图片计价模板 %s", modelName, binding.Profile)
		}
		if binding.MaxNConfigured && (binding.MaxN < 1 || binding.MaxN > dto.MaxImageN) {
			return types.ImagePricingConfig{}, fmt.Errorf("模型 %s 的 max_n 必须在 1 到 %d 之间，或省略", modelName, dto.MaxImageN)
		}
		if _, exists := normalizedBindings[modelName]; exists {
			return types.ImagePricingConfig{}, fmt.Errorf("图片参数计价模型 %s 重复", modelName)
		}
		normalizedBindings[modelName] = binding
	}

	config.Profiles = normalizedProfiles
	config.ModelBindings = normalizedBindings
	return config, nil
}

func ValidateImagePricingJSON(jsonStr string) error {
	_, err := parseImagePricingConfig(jsonStr)
	return err
}

func UpdateImagePricingByJSONString(jsonStr string) error {
	config, err := parseImagePricingConfig(jsonStr)
	if err != nil {
		return err
	}
	imagePricingState.Lock()
	imagePricingState.config = config
	imagePricingState.Unlock()
	InvalidateExposedDataCache()
	return nil
}

func ImagePricing2JSONString() string {
	config := GetImagePricingConfig()
	data, err := common.Marshal(config)
	if err != nil {
		return `{"version":1,"profiles":{},"model_bindings":{}}`
	}
	return string(data)
}

func GetImagePricingConfig() types.ImagePricingConfig {
	imagePricingState.RLock()
	config := cloneImagePricingConfig(imagePricingState.config)
	imagePricingState.RUnlock()
	return config
}

func imagePricingProfileHash(profileID string, profile types.ImagePricingProfile) string {
	payload := struct {
		ProfileID string                    `json:"profile_id"`
		Profile   types.ImagePricingProfile `json:"profile"`
	}{ProfileID: profileID, Profile: profile}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func GetImagePricingForModel(modelName string) (types.ImagePricingProfile, types.ImagePricingBinding, string, bool) {
	imagePricingState.RLock()
	binding, bound := imagePricingState.config.ModelBindings[modelName]
	profile, exists := imagePricingState.config.Profiles[binding.Profile]
	if bound && exists {
		profile = cloneImagePricingProfile(profile)
	}
	imagePricingState.RUnlock()
	if !bound || !exists {
		return types.ImagePricingProfile{}, types.ImagePricingBinding{}, "", false
	}
	if binding.MaxN == 0 {
		binding.MaxN = dto.MaxImageN
	}
	return profile, binding, imagePricingProfileHash(binding.Profile, profile), true
}

func IsImageParameterPricedModel(modelName string) bool {
	_, _, _, ok := GetImagePricingForModel(modelName)
	return ok
}

func publicImagePricing(profileID string, profile types.ImagePricingProfile, binding types.ImagePricingBinding) *types.PublicImagePricing {
	maxN := binding.MaxN
	if maxN == 0 {
		maxN = dto.MaxImageN
	}
	public := &types.PublicImagePricing{
		ProfileID:   profileID,
		Name:        profile.Name,
		Parameter:   profile.Parameter,
		DefaultTier: profile.DefaultTier,
		MaxN:        maxN,
		Tiers:       make([]types.PublicImagePricingTier, 0, len(profile.Tiers)),
	}
	for _, tier := range profile.Tiers {
		public.Tiers = append(public.Tiers, types.PublicImagePricingTier{
			Key:       tier.Key,
			Aliases:   append([]string(nil), tier.Aliases...),
			UnitPrice: tier.UnitPrice,
		})
	}
	return public
}

func GetPublicImagePricingForModel(modelName string) (*types.PublicImagePricing, bool) {
	profile, binding, _, ok := GetImagePricingForModel(modelName)
	if !ok {
		return nil, false
	}
	return publicImagePricing(binding.Profile, profile, binding), true
}

// GetPublicImagePricingSnapshot returns one immutable, redacted view of every
// binding so a pricing response cannot mix fields from two configuration versions.
func GetPublicImagePricingSnapshot() map[string]*types.PublicImagePricing {
	imagePricingState.RLock()
	defer imagePricingState.RUnlock()

	snapshot := make(map[string]*types.PublicImagePricing, len(imagePricingState.config.ModelBindings))
	for modelName, binding := range imagePricingState.config.ModelBindings {
		profile, exists := imagePricingState.config.Profiles[binding.Profile]
		if !exists {
			continue
		}
		snapshot[modelName] = publicImagePricing(binding.Profile, profile, binding)
	}
	return snapshot
}

func GetImagePricingDefaultUnitPrice(modelName string) (float64, bool) {
	profile, _, _, ok := GetImagePricingForModel(modelName)
	if !ok {
		return 0, false
	}
	for _, tier := range profile.Tiers {
		if strings.EqualFold(tier.Key, profile.DefaultTier) {
			return tier.UnitPrice, true
		}
	}
	return 0, false
}

func GetImagePricingBoundModels() []string {
	imagePricingState.RLock()
	models := make([]string, 0, len(imagePricingState.config.ModelBindings))
	for modelName := range imagePricingState.config.ModelBindings {
		models = append(models, modelName)
	}
	imagePricingState.RUnlock()
	sort.Strings(models)
	return models
}
