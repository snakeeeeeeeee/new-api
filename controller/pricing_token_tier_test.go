package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestClonePricingItemsUsesCurrentEffectiveTokenTierRule(t *testing.T) {
	original := ratio_setting.TokenTierPricingRules2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString(original))
	})

	stale := types.TokenTierPricingRuleMeta{
		Rule: types.TokenTierPricingRule{
			Enabled: true,
			Tiers:   []types.TokenTier{{UseBasePrice: true}, {}},
		},
	}
	pricing := []model.Pricing{
		{ModelName: "gpt-5.6-luna", QuotaType: 0, TokenTierPricing: &stale},
		{ModelName: "gpt-5.6-sol", QuotaType: 1, TokenTierPricing: &stale},
	}

	require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString(`{"gpt-5.6-luna":{"enabled":false}}`))
	disabled := clonePricingItems(pricing)
	require.Nil(t, disabled[0].TokenTierPricing)
	require.Nil(t, disabled[1].TokenTierPricing)
	require.NotNil(t, pricing[0].TokenTierPricing, "source cache must remain unchanged")

	require.NoError(t, ratio_setting.UpdateTokenTierPricingRulesByJSONString(`{}`))
	reEnabled := clonePricingItems(pricing)
	require.NotNil(t, reEnabled[0].TokenTierPricing)
	require.True(t, reEnabled[0].TokenTierPricing.Rule.Enabled)
	require.Len(t, reEnabled[0].TokenTierPricing.Rule.Tiers, 2)
	require.Nil(t, reEnabled[1].TokenTierPricing)
}
