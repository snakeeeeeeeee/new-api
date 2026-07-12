package ratio_setting

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	TokenTierPricingSourceSystem = "system"
	TokenTierPricingSourceCustom = "custom"
)

var defaultTokenTierPricingRules = map[string]types.TokenTierPricingRule{
	"gpt-5.6":       newGPT56StandardRule("openai-gpt-5.6-standard", types.TokenTierPrices{Input: 10, CachedInput: 1, CacheWrite: 12.5, Output: 45}),
	"gpt-5.6-sol":   newGPT56StandardRule("openai-gpt-5.6-sol-standard", types.TokenTierPrices{Input: 10, CachedInput: 1, CacheWrite: 12.5, Output: 45}),
	"gpt-5.6-terra": newGPT56StandardRule("openai-gpt-5.6-terra-standard", types.TokenTierPrices{Input: 5, CachedInput: 0.5, CacheWrite: 6.25, Output: 22.5}),
	"gpt-5.6-luna":  newGPT56StandardRule("openai-gpt-5.6-luna-standard", types.TokenTierPrices{Input: 2, CachedInput: 0.2, CacheWrite: 2.5, Output: 9}),
}

var tokenTierPricingState = struct {
	sync.RWMutex
	overrides map[string]types.TokenTierPricingRule
}{
	overrides: map[string]types.TokenTierPricingRule{},
}

func newGPT56StandardRule(id string, longContextPrices types.TokenTierPrices) types.TokenTierPricingRule {
	limit := 272000
	return types.TokenTierPricingRule{
		ID:          id,
		Enabled:     true,
		ServiceTier: types.TokenTierServiceTierStandard,
		Meter:       types.TokenTierMeterInputTotal,
		BillingMode: types.TokenTierBillingWholeRequest,
		Tiers: []types.TokenTier{
			{UpToInclusive: &limit, UseBasePrice: true},
			{UpToInclusive: nil, Prices: longContextPrices},
		},
	}
}

func cloneTokenTierRule(rule types.TokenTierPricingRule) types.TokenTierPricingRule {
	cloned := rule
	cloned.Tiers = append([]types.TokenTier(nil), rule.Tiers...)
	for index := range cloned.Tiers {
		if rule.Tiers[index].UpToInclusive != nil {
			limit := *rule.Tiers[index].UpToInclusive
			cloned.Tiers[index].UpToInclusive = &limit
		}
	}
	return cloned
}

func validateTokenTierPrices(modelName string, tierIndex int, prices types.TokenTierPrices) error {
	values := []struct {
		name  string
		value float64
	}{
		{"input", prices.Input},
		{"cached_input", prices.CachedInput},
		{"cache_write", prices.CacheWrite},
		{"output", prices.Output},
	}
	for _, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) || item.value < 0 {
			return fmt.Errorf("模型 %s 第 %d 档 %s 价格必须是有限的非负数", modelName, tierIndex+1, item.name)
		}
	}
	return nil
}

func validateTokenTierRule(modelName string, rule types.TokenTierPricingRule) error {
	if strings.TrimSpace(modelName) == "" {
		return fmt.Errorf("Token 阶梯价格模型名不能为空")
	}
	if !rule.Enabled {
		return nil
	}
	if strings.TrimSpace(rule.ID) == "" {
		return fmt.Errorf("模型 %s 的规则 ID 不能为空", modelName)
	}
	if rule.ServiceTier != types.TokenTierServiceTierStandard {
		return fmt.Errorf("模型 %s 仅支持 service_tier=standard", modelName)
	}
	if rule.Meter != types.TokenTierMeterInputTotal {
		return fmt.Errorf("模型 %s 仅支持 meter=input_tokens_total", modelName)
	}
	if rule.BillingMode != types.TokenTierBillingWholeRequest {
		return fmt.Errorf("模型 %s 仅支持 billing_mode=whole_request", modelName)
	}
	if len(rule.Tiers) < 2 {
		return fmt.Errorf("模型 %s 至少需要两个价格档位", modelName)
	}
	previousLimit := -1
	for index, tier := range rule.Tiers {
		isLast := index == len(rule.Tiers)-1
		if isLast {
			if tier.UpToInclusive != nil {
				return fmt.Errorf("模型 %s 最后一档的 up_to_inclusive 必须为 null", modelName)
			}
		} else {
			if tier.UpToInclusive == nil {
				return fmt.Errorf("模型 %s 仅最后一档可以没有上限", modelName)
			}
			if *tier.UpToInclusive < 0 || *tier.UpToInclusive <= previousLimit {
				return fmt.Errorf("模型 %s 的档位上限必须严格递增且非负", modelName)
			}
			previousLimit = *tier.UpToInclusive
		}
		if index == 0 {
			if !tier.UseBasePrice {
				return fmt.Errorf("模型 %s 第一档必须使用基础价格", modelName)
			}
			continue
		}
		if tier.UseBasePrice {
			return fmt.Errorf("模型 %s 仅第一档可以使用基础价格", modelName)
		}
		if err := validateTokenTierPrices(modelName, index, tier.Prices); err != nil {
			return err
		}
	}
	return nil
}

func parseTokenTierPricingOverrides(jsonStr string) (map[string]types.TokenTierPricingRule, error) {
	overrides := make(map[string]types.TokenTierPricingRule)
	if strings.TrimSpace(jsonStr) == "" {
		return overrides, nil
	}
	if err := common.UnmarshalJsonStr(jsonStr, &overrides); err != nil {
		return nil, fmt.Errorf("Token 阶梯价格配置不是有效 JSON: %w", err)
	}
	for modelName, rule := range overrides {
		if err := validateTokenTierRule(modelName, rule); err != nil {
			return nil, err
		}
		overrides[modelName] = cloneTokenTierRule(rule)
	}
	return overrides, nil
}

func ValidateTokenTierPricingRulesJSON(jsonStr string) error {
	_, err := parseTokenTierPricingOverrides(jsonStr)
	return err
}

func UpdateTokenTierPricingRulesByJSONString(jsonStr string) error {
	overrides, err := parseTokenTierPricingOverrides(jsonStr)
	if err != nil {
		return err
	}
	tokenTierPricingState.Lock()
	tokenTierPricingState.overrides = overrides
	tokenTierPricingState.Unlock()
	InvalidateExposedDataCache()
	return nil
}

func TokenTierPricingRules2JSONString() string {
	tokenTierPricingState.RLock()
	overrides := make(map[string]types.TokenTierPricingRule, len(tokenTierPricingState.overrides))
	for modelName, rule := range tokenTierPricingState.overrides {
		overrides[modelName] = cloneTokenTierRule(rule)
	}
	tokenTierPricingState.RUnlock()
	bytes, err := common.Marshal(overrides)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func tokenTierRuleHash(modelName string, rule types.TokenTierPricingRule) string {
	payload := struct {
		Model string                     `json:"model"`
		Rule  types.TokenTierPricingRule `json:"rule"`
	}{Model: modelName, Rule: rule}
	bytes, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(bytes))
}

func GetTokenTierPricingRuleMeta(modelName string) (types.TokenTierPricingRuleMeta, bool) {
	tokenTierPricingState.RLock()
	override, overridden := tokenTierPricingState.overrides[modelName]
	tokenTierPricingState.RUnlock()
	if overridden {
		rule := cloneTokenTierRule(override)
		if !rule.Enabled && len(rule.Tiers) == 0 {
			if defaultRule, ok := defaultTokenTierPricingRules[modelName]; ok {
				rule = cloneTokenTierRule(defaultRule)
				rule.Enabled = false
			}
		}
		return types.TokenTierPricingRuleMeta{Rule: rule, Source: TokenTierPricingSourceCustom, Hash: tokenTierRuleHash(modelName, rule)}, true
	}
	defaultRule, ok := defaultTokenTierPricingRules[modelName]
	if !ok {
		return types.TokenTierPricingRuleMeta{}, false
	}
	rule := cloneTokenTierRule(defaultRule)
	return types.TokenTierPricingRuleMeta{Rule: rule, Source: TokenTierPricingSourceSystem, Hash: tokenTierRuleHash(modelName, rule)}, true
}

func GetEffectiveTokenTierPricingRule(modelName string) (types.TokenTierPricingRuleMeta, bool) {
	meta, ok := GetTokenTierPricingRuleMeta(modelName)
	return meta, ok && meta.Rule.Enabled
}

func GetTokenTierPricingRulesMeta() map[string]types.TokenTierPricingRuleMeta {
	modelNames := make(map[string]struct{}, len(defaultTokenTierPricingRules))
	for modelName := range defaultTokenTierPricingRules {
		modelNames[modelName] = struct{}{}
	}
	tokenTierPricingState.RLock()
	for modelName := range tokenTierPricingState.overrides {
		modelNames[modelName] = struct{}{}
	}
	tokenTierPricingState.RUnlock()

	names := make([]string, 0, len(modelNames))
	for modelName := range modelNames {
		names = append(names, modelName)
	}
	sort.Strings(names)
	result := make(map[string]types.TokenTierPricingRuleMeta, len(names))
	for _, modelName := range names {
		if meta, ok := GetTokenTierPricingRuleMeta(modelName); ok {
			result[modelName] = meta
		}
	}
	return result
}
