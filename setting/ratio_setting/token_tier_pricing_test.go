package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultGPT56TokenTierPricingBoundaries(t *testing.T) {
	for _, modelName := range []string{"gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"} {
		meta, ok := GetEffectiveTokenTierPricingRule(modelName)
		require.True(t, ok, modelName)
		require.Equal(t, TokenTierPricingSourceSystem, meta.Source)
		require.NotEmpty(t, meta.Hash)

		snapshot := &types.TokenTierPricingSnapshot{Tiers: meta.Rule.Tiers}
		_, index, ok := snapshot.SelectTier(272000)
		require.True(t, ok)
		require.Equal(t, 0, index)
		_, index, ok = snapshot.SelectTier(272001)
		require.True(t, ok)
		require.Equal(t, 1, index)
	}
}

func TestTokenTierPricingSupportsThreeTiers(t *testing.T) {
	original := TokenTierPricingRules2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateTokenTierPricingRulesByJSONString(original)) })

	rules := `{"test-three-tier":{"id":"test","enabled":true,"service_tier":"standard","meter":"input_tokens_total","billing_mode":"whole_request","tiers":[{"up_to_inclusive":272000,"use_base_price":true},{"up_to_inclusive":500000,"prices":{"input":2,"cached_input":0.2,"cache_write":2.5,"output":9}},{"up_to_inclusive":null,"prices":{"input":3,"cached_input":0.3,"cache_write":3.75,"output":12}}]}}`
	require.NoError(t, UpdateTokenTierPricingRulesByJSONString(rules))
	meta, ok := GetEffectiveTokenTierPricingRule("test-three-tier")
	require.True(t, ok)
	snapshot := &types.TokenTierPricingSnapshot{Tiers: meta.Rule.Tiers}

	for tokens, expectedIndex := range map[int]int{272000: 0, 272001: 1, 500000: 1, 500001: 2} {
		_, index, selected := snapshot.SelectTier(tokens)
		require.True(t, selected)
		require.Equal(t, expectedIndex, index, tokens)
	}
}

func TestInvalidTokenTierPricingUpdateKeepsPreviousSnapshot(t *testing.T) {
	original := TokenTierPricingRules2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateTokenTierPricingRulesByJSONString(original)) })

	valid := `{"custom-model":{"id":"valid","enabled":true,"service_tier":"standard","meter":"input_tokens_total","billing_mode":"whole_request","tiers":[{"up_to_inclusive":10,"use_base_price":true},{"up_to_inclusive":null,"prices":{"input":2,"cached_input":0.2,"cache_write":2.5,"output":9}}]}}`
	require.NoError(t, UpdateTokenTierPricingRulesByJSONString(valid))
	before, ok := GetEffectiveTokenTierPricingRule("custom-model")
	require.True(t, ok)

	invalid := `{"custom-model":{"id":"invalid","enabled":true,"service_tier":"standard","meter":"input_tokens_total","billing_mode":"whole_request","tiers":[{"up_to_inclusive":10,"use_base_price":true},{"up_to_inclusive":5,"prices":{"input":2}},{"up_to_inclusive":null,"prices":{"input":3}}]}}`
	require.Error(t, UpdateTokenTierPricingRulesByJSONString(invalid))
	after, ok := GetEffectiveTokenTierPricingRule("custom-model")
	require.True(t, ok)
	require.Equal(t, before.Hash, after.Hash)
}

func TestDisabledOverrideSuppressesBuiltInTokenTierPricing(t *testing.T) {
	original := TokenTierPricingRules2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateTokenTierPricingRulesByJSONString(original)) })

	require.NoError(t, UpdateTokenTierPricingRulesByJSONString(`{"gpt-5.6-luna":{"enabled":false}}`))
	meta, exists := GetTokenTierPricingRuleMeta("gpt-5.6-luna")
	require.True(t, exists)
	require.Equal(t, TokenTierPricingSourceCustom, meta.Source)
	require.False(t, meta.Rule.Enabled)
	_, enabled := GetEffectiveTokenTierPricingRule("gpt-5.6-luna")
	require.False(t, enabled)
}
