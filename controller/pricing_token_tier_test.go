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

const videoPricingCloneTestConfig = `{
  "version": 1,
  "profiles": {
    "clone-video": {
	      "name": "clone video",
	      "billing_mode": "per_second",
	      "unit_price": 0.03,
	      "reference_video_unit_price": 0.02
    }
  },
  "model_bindings": {
    "stale-video-pricing-model": {"profile":"clone-video","subscription_enabled":false},
    "legacy-video-policy-model": {"subscription_enabled":true}
  }
}`

const emptyVideoPricingCloneTestConfig = `{"version":1,"profiles":{},"model_bindings":{}}`

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

func TestClonePricingItemsAppliesVideoPricingPolicyAndRestoresLegacyPriceAfterUnbind(t *testing.T) {
	originalVideoPricing := ratio_setting.VideoPricing2JSONString()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(originalVideoPricing))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"stale-video-pricing-model":0.42,"legacy-video-policy-model":0.25}`))
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(videoPricingCloneTestConfig))

	base := []model.Pricing{
		{ModelName: "stale-video-pricing-model", QuotaType: 1, ModelPrice: 0.42},
		{ModelName: "legacy-video-policy-model", QuotaType: 1, ModelPrice: 0.25},
	}
	bound := clonePricingItems(base)
	require.Len(t, bound, 2)
	assertVideo := bound[0]
	require.Equal(t, types.VideoPricingBillingType, assertVideo.BillingType)
	require.NotNil(t, assertVideo.VideoPricing)
	require.Equal(t, 0.03, assertVideo.ModelPrice)
	require.Equal(t, 0.02, assertVideo.VideoPricing.ReferenceVideoUnitPrice)
	require.False(t, assertVideo.VideoPricing.SubscriptionEnabled)

	policyOnly := bound[1]
	require.Empty(t, policyOnly.BillingType)
	require.NotNil(t, policyOnly.VideoPricing)
	require.Empty(t, policyOnly.VideoPricing.ProfileID)
	require.True(t, policyOnly.VideoPricing.SubscriptionEnabled)
	require.Equal(t, 0.25, policyOnly.ModelPrice)

	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(emptyVideoPricingCloneTestConfig))
	restored := clonePricingItems(bound)
	require.Len(t, restored, 2)
	require.Empty(t, restored[0].BillingType)
	require.Nil(t, restored[0].VideoPricing)
	require.Equal(t, 0.42, restored[0].ModelPrice)
	require.Nil(t, restored[1].VideoPricing)
	require.Equal(t, 0.25, restored[1].ModelPrice)
	require.NotNil(t, bound[0].VideoPricing, "source cache must remain unchanged")
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
