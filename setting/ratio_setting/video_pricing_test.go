package ratio_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

const videoPricingValidConfig = `{
  "version": 1,
  "profiles": {
    "seedance-720p": {
	      "name": " Seedance 720p ",
	      "billing_mode": "PER_SECOND",
	      "unit_price": 0.03,
	      "reference_video_unit_price": 0.02
    }
  },
  "model_bindings": {
    "seedance-1.5-pro-720p": {
      "profile": " seedance-720p ",
      "subscription_enabled": false
    },
    "legacy-video-model": {
      "subscription_enabled": true
    }
  }
}`

func preserveVideoPricingConfig(t *testing.T) {
	t.Helper()
	original := VideoPricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateVideoPricingByJSONString(original))
	})
}

func TestVideoPricingConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{name: "invalid json", config: `{`},
		{name: "wrong version", config: `{"version":2,"profiles":{},"model_bindings":{}}`},
		{name: "empty profile name", config: `{"version":1,"profiles":{"p":{"name":" ","billing_mode":"per_second","unit_price":1}},"model_bindings":{}}`},
		{name: "wrong billing mode", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_call","unit_price":1}},"model_bindings":{}}`},
		{name: "missing unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second"}},"model_bindings":{}}`},
		{name: "null unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":null}},"model_bindings":{}}`},
		{name: "non numeric unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":"1"}},"model_bindings":{}}`},
		{name: "negative unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":-1}},"model_bindings":{}}`},
		{name: "null reference video unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":1,"reference_video_unit_price":null}},"model_bindings":{}}`},
		{name: "negative reference video unit price", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":1,"reference_video_unit_price":-1}},"model_bindings":{}}`},
		{name: "trimmed duplicate profiles", config: `{"version":1,"profiles":{"p":{"name":"p","billing_mode":"per_second","unit_price":1}," p ":{"name":"other","billing_mode":"per_second","unit_price":2}},"model_bindings":{}}`},
		{name: "empty model", config: `{"version":1,"profiles":{},"model_bindings":{" ":{"subscription_enabled":true}}}`},
		{name: "missing profile", config: `{"version":1,"profiles":{},"model_bindings":{"model":{"profile":"missing"}}}`},
		{name: "empty policy binding", config: `{"version":1,"profiles":{},"model_bindings":{"model":{"subscription_enabled":false}}}`},
		{name: "trimmed duplicate models", config: `{"version":1,"profiles":{},"model_bindings":{"model":{"subscription_enabled":true}," model ":{"subscription_enabled":true}}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, ValidateVideoPricingJSON(test.config))
		})
	}

	require.NoError(t, ValidateVideoPricingJSON(videoPricingValidConfig))
	require.NoError(t, ValidateVideoPricingJSON(`{"version":1,"profiles":{"free":{"name":"free","billing_mode":"per_second","unit_price":0}},"model_bindings":{"free-video":{"profile":"free"}}}`))
}

func TestVideoPricingRejectsNonFiniteUnitPrices(t *testing.T) {
	for _, price := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := normalizeVideoPricingProfile("finite-price", types.VideoPricingProfile{
			Name:        "finite-price",
			BillingMode: types.VideoPricingModePerSecond,
			UnitPrice:   price,
		})
		require.Error(t, err)
		_, err = normalizeVideoPricingProfile("finite-reference-price", types.VideoPricingProfile{
			Name:                    "finite-reference-price",
			BillingMode:             types.VideoPricingModePerSecond,
			UnitPrice:               1,
			ReferenceVideoUnitPrice: price,
		})
		require.Error(t, err)
	}
}

func TestVideoPricingExactBindingsAndPublicView(t *testing.T) {
	preserveVideoPricingConfig(t)
	require.NoError(t, UpdateVideoPricingByJSONString(videoPricingValidConfig))

	profile, binding, profileHash, priced := GetVideoPricingForModel("seedance-1.5-pro-720p")
	require.True(t, priced)
	require.Equal(t, "Seedance 720p", profile.Name)
	require.Equal(t, types.VideoPricingModePerSecond, profile.BillingMode)
	require.Equal(t, 0.03, profile.UnitPrice)
	require.Equal(t, 0.02, profile.ReferenceVideoUnitPrice)
	require.Equal(t, "seedance-720p", binding.Profile)
	require.False(t, binding.SubscriptionEnabled)
	require.NotEmpty(t, profileHash)

	_, _, _, priced = GetVideoPricingForModel("SEEDANCE-1.5-PRO-720P")
	require.False(t, priced)
	require.False(t, IsVideoSubscriptionEnabled("seedance-1.5-pro-720p"))
	require.True(t, IsVideoSubscriptionEnabled("legacy-video-model"))

	public := GetPublicVideoPricingSnapshot()
	require.Equal(t, types.VideoPricingUnitSecond, public["seedance-1.5-pro-720p"].Unit)
	require.Equal(t, 0.03, public["seedance-1.5-pro-720p"].UnitPrice)
	require.Equal(t, 0.02, public["seedance-1.5-pro-720p"].ReferenceVideoUnitPrice)
	require.False(t, public["seedance-1.5-pro-720p"].SubscriptionEnabled)
	require.Empty(t, public["legacy-video-model"].ProfileID)
	require.True(t, public["legacy-video-model"].SubscriptionEnabled)
}

func TestHasModelPricingRecognizesPricedVideoBindingOnly(t *testing.T) {
	preserveVideoPricingConfig(t)
	require.NoError(t, UpdateVideoPricingByJSONString(videoPricingValidConfig))

	require.True(t, HasModelPricing("seedance-1.5-pro-720p"))
	require.False(t, HasModelPricing("legacy-video-model"), "policy-only bindings are not prices")
	require.False(t, HasModelPricing("unpriced-video-model"))
}

func TestVideoPricingInvalidUpdateKeepsPreviousSnapshot(t *testing.T) {
	preserveVideoPricingConfig(t)
	require.NoError(t, UpdateVideoPricingByJSONString(videoPricingValidConfig))

	beforeProfile, beforeBinding, beforeHash, ok := GetVideoPricingForModel("seedance-1.5-pro-720p")
	require.True(t, ok)
	require.Error(t, UpdateVideoPricingByJSONString(`{"version":1,"profiles":{},"model_bindings":{"seedance-1.5-pro-720p":{"profile":"missing"}}}`))
	afterProfile, afterBinding, afterHash, ok := GetVideoPricingForModel("seedance-1.5-pro-720p")
	require.True(t, ok)
	require.Equal(t, beforeProfile, afterProfile)
	require.Equal(t, beforeBinding, afterBinding)
	require.Equal(t, beforeHash, afterHash)
}

func TestVideoPricingSnapshotsAreImmutable(t *testing.T) {
	preserveVideoPricingConfig(t)
	require.NoError(t, UpdateVideoPricingByJSONString(videoPricingValidConfig))

	config := GetVideoPricingConfig()
	profile := config.Profiles["seedance-720p"]
	profile.UnitPrice = 99
	config.Profiles["seedance-720p"] = profile
	delete(config.ModelBindings, "seedance-1.5-pro-720p")

	unchanged, _, _, ok := GetVideoPricingForModel("seedance-1.5-pro-720p")
	require.True(t, ok)
	require.Equal(t, 0.03, unchanged.UnitPrice)

	public := GetPublicVideoPricingSnapshot()
	retained := public["seedance-1.5-pro-720p"]
	require.NoError(t, UpdateVideoPricingByJSONString(`{"version":1,"profiles":{},"model_bindings":{}}`))
	require.Empty(t, GetPublicVideoPricingSnapshot())
	require.Equal(t, 0.03, retained.UnitPrice)
}
