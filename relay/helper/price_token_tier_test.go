package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestTokenTierPricingSnapshotResolvesBasePricesAndIsImmutable(t *testing.T) {
	originalRules := ratio_setting.TokenTierPricingRules2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString(originalRules))
	})
	require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString("{}"))

	priceData := types.PriceData{
		ModelRatio:         0.5,
		CompletionRatio:    6,
		CacheRatio:         0.1,
		CacheCreationRatio: 1.25,
	}
	snapshot := buildTokenTierPricingSnapshot("gpt-5.6-luna", priceData)
	require.NotNil(t, snapshot)
	require.Equal(t, types.TokenTierPrices{Input: 1, CachedInput: 0.1, CacheWrite: 1.25, Output: 6}, snapshot.Tiers[0].Prices)
	originalHash := snapshot.RuleHash

	override := `{"gpt-5.6-luna":{"id":"changed","enabled":true,"service_tier":"standard","meter":"input_tokens_total","billing_mode":"whole_request","tiers":[{"up_to_inclusive":10,"use_base_price":true},{"up_to_inclusive":null,"prices":{"input":99,"cached_input":9,"cache_write":100,"output":101}}]}}`
	require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString(override))
	require.Equal(t, originalHash, snapshot.RuleHash)
	selected, index, ok := snapshot.SelectTier(272001)
	require.True(t, ok)
	require.Equal(t, 1, index)
	require.Equal(t, 2.0, selected.Prices.Input)
}

func TestTokenTierPreConsumeUsesEstimatedInputForTierSelection(t *testing.T) {
	limit := 272000
	snapshot := &types.TokenTierPricingSnapshot{Tiers: []types.TokenTier{
		{UpToInclusive: &limit, Prices: types.TokenTierPrices{Input: 1, Output: 6}},
		{UpToInclusive: nil, Prices: types.TokenTierPrices{Input: 2, Output: 9}},
	}}

	quota, ok := calculateTokenTierPreConsumedQuota(snapshot, 272001, 10, 1)
	require.True(t, ok)
	expected := common.QuotaFromFloat((float64(272001*2+10*9) / 1_000_000) * common.QuotaPerUnit)
	require.Equal(t, expected, quota)
}
