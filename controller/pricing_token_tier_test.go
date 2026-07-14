package controller

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

const imagePricingCloneTestConfig = `{
  "version": 1,
  "profiles": {
    "clone-quality": {
      "name": "clone quality",
      "parameter": "quality",
      "default_tier": "low",
      "tiers": [{"key":"low","upstream_value":"low","aliases":[],"unit_price":0.04}]
    }
  },
  "model_bindings": {
    "stale-image-pricing-model": {"profile":"clone-quality"}
  }
}`

const emptyImagePricingCloneTestConfig = `{"version":1,"profiles":{},"model_bindings":{}}`

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

func TestClonePricingItemsRestoresLegacyPriceAfterImagePricingUnbind(t *testing.T) {
	originalImagePricing := ratio_setting.ImagePricing2JSONString()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalImagePricing))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"stale-image-pricing-model":0.42}`))
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(imagePricingCloneTestConfig))
	public, ok := ratio_setting.GetPublicImagePricingForModel("stale-image-pricing-model")
	require.True(t, ok)

	cached := []model.Pricing{{
		ModelName:    "stale-image-pricing-model",
		QuotaType:    1,
		ModelPrice:   0.04,
		BillingType:  types.ImagePricingBillingType,
		ImagePricing: public,
	}}
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(emptyImagePricingCloneTestConfig))

	cloned := clonePricingItems(cached)
	require.Len(t, cloned, 1)
	require.Empty(t, cloned[0].BillingType)
	require.Nil(t, cloned[0].ImagePricing)
	require.Equal(t, 1, cloned[0].QuotaType)
	require.Equal(t, 0.42, cloned[0].ModelPrice)
	require.NotNil(t, cached[0].ImagePricing, "source cache must remain unchanged")
}

func TestClonePricingItemsNeverMixesImageBillingFieldsDuringConcurrentUnbind(t *testing.T) {
	originalImagePricing := ratio_setting.ImagePricing2JSONString()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalImagePricing))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"stale-image-pricing-model":0.42}`))
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(imagePricingCloneTestConfig))
	public, ok := ratio_setting.GetPublicImagePricingForModel("stale-image-pricing-model")
	require.True(t, ok)
	cached := []model.Pricing{{
		ModelName:    "stale-image-pricing-model",
		QuotaType:    1,
		ModelPrice:   0.04,
		BillingType:  types.ImagePricingBillingType,
		ImagePricing: public,
	}}

	var updateErr error
	var updateWG sync.WaitGroup
	updateWG.Add(1)
	go func() {
		defer updateWG.Done()
		for i := 0; i < 200; i++ {
			if err := ratio_setting.UpdateImagePricingByJSONString(emptyImagePricingCloneTestConfig); err != nil {
				updateErr = err
				return
			}
			if err := ratio_setting.UpdateImagePricingByJSONString(imagePricingCloneTestConfig); err != nil {
				updateErr = err
				return
			}
		}
	}()

	for i := 0; i < 500; i++ {
		cloned := clonePricingItems(cached)[0]
		if cloned.BillingType == types.ImagePricingBillingType {
			require.NotNil(t, cloned.ImagePricing)
			require.Equal(t, 0.04, cloned.ModelPrice)
			continue
		}
		require.Empty(t, cloned.BillingType)
		require.Nil(t, cloned.ImagePricing)
		require.Equal(t, 0.42, cloned.ModelPrice)
	}
	updateWG.Wait()
	require.NoError(t, updateErr)
}
