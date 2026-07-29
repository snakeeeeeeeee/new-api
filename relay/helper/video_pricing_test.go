package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

const videoPricingResolverConfig = `{
  "version": 1,
  "profiles": {
    "video-720p": {"name":"Video 720p","billing_mode":"per_second","unit_price":0.03}
  },
  "model_bindings": {
    "video-model-720p": {"profile":"video-720p","subscription_enabled":false},
    "legacy-video-model": {"subscription_enabled":true}
  }
}`

func setupVideoPricingResolverTest(t *testing.T) {
	t.Helper()
	original := ratio_setting.VideoPricing2JSONString()
	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(videoPricingResolverConfig))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(original))
	})
}

func TestResolveVideoPricingCalculatesPerSecondQuota(t *testing.T) {
	setupVideoPricingResolverTest(t)

	snapshot, bound, err := ResolveVideoPricing("video-model-720p", 8, types.VideoPricingBasisGeneration, 1.5)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, "video-model-720p", snapshot.PublicModel)
	require.Equal(t, "video-720p", snapshot.ProfileID)
	require.NotEmpty(t, snapshot.ProfileHash)
	require.Equal(t, 0.03, snapshot.UnitPrice)
	require.Equal(t, 8, snapshot.Seconds)
	require.Equal(t, types.VideoPricingBasisGeneration, snapshot.Basis)
	require.InDelta(t, 0.24, snapshot.Subtotal, 1e-12)
	require.Equal(t, common.QuotaFromFloat(0.03*8*1.5*common.QuotaPerUnit), snapshot.FinalQuota)
	require.False(t, snapshot.SubscriptionEnabled)

	priceData := VideoPricingPriceData(snapshot, types.GroupRatioInfo{GroupRatio: 1.5})
	require.True(t, priceData.UsePrice)
	require.Equal(t, snapshot.FinalQuota, priceData.Quota)
	require.Empty(t, priceData.OtherRatios)
	require.NotNil(t, priceData.VideoPricing)
}

func TestResolveVideoPricingRejectsInvalidQuantityAndPolicyOnlyBinding(t *testing.T) {
	setupVideoPricingResolverTest(t)

	_, bound, err := ResolveVideoPricing("video-model-720p", 0, types.VideoPricingBasisGeneration, 1)
	require.True(t, bound)
	require.Error(t, err)

	_, bound, err = ResolveVideoPricing("video-model-720p", 5, "actual_output", 1)
	require.True(t, bound)
	require.Error(t, err)

	_, bound, err = ResolveVideoPricing("legacy-video-model", 5, types.VideoPricingBasisGeneration, 1)
	require.False(t, bound)
	require.NoError(t, err)
}

func TestResolvedVideoPricingSnapshotDoesNotChangeWithConfig(t *testing.T) {
	setupVideoPricingResolverTest(t)
	snapshot, bound, err := ResolveVideoPricing("video-model-720p", 5, types.VideoPricingBasisGeneration, 1)
	require.NoError(t, err)
	require.True(t, bound)

	require.NoError(t, ratio_setting.UpdateVideoPricingByJSONString(`{"version":1,"profiles":{"video-720p":{"name":"changed","billing_mode":"per_second","unit_price":9}},"model_bindings":{"video-model-720p":{"profile":"video-720p"}}}`))
	require.Equal(t, 0.03, snapshot.UnitPrice)
	require.InDelta(t, 0.15, snapshot.Subtotal, 1e-12)
}
