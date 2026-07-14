package ratio_setting

import (
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

const imagePricingValidConfig = `{
  "version": 1,
  "profiles": {
    "adobe-quality-v1": {
      "name": " ADOBE quality per image ",
      "parameter": "QUALITY",
      "default_tier": "LOW",
      "tiers": [
        {"key":"low","upstream_value":"provider-low","aliases":[" auto "],"unit_price":0.04},
        {"key":"high","upstream_value":"provider-high","aliases":[],"unit_price":0.15}
      ]
    }
  },
  "model_bindings": {
    "adobe-gpt-image-2-count": {"profile":" adobe-quality-v1 "}
  }
}`

func preserveImagePricingConfig(t *testing.T) {
	t.Helper()
	original := ImagePricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateImagePricingByJSONString(original))
	})
}

func TestImagePricingConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{name: "invalid json", config: `{`},
		{name: "wrong field type", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":true,"default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{}}`},
		{name: "wrong version", config: `{"version":2,"profiles":{},"model_bindings":{}}`},
		{name: "empty profile name", config: `{"version":1,"profiles":{"p":{"name":" ","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{}}`},
		{name: "unsupported parameter", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"style","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{}}`},
		{name: "empty tiers", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[]}},"model_bindings":{}}`},
		{name: "missing default tier", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"medium","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{}}`},
		{name: "empty tier key", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":" ","upstream_value":"low","unit_price":1}]}},"model_bindings":{}}`},
		{name: "empty upstream value", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":" ","unit_price":1}]}},"model_bindings":{}}`},
		{name: "missing unit price", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low"}]}},"model_bindings":{}}`},
		{name: "null unit price", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":null}]}},"model_bindings":{}}`},
		{name: "non numeric unit price", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":"0.04"}]}},"model_bindings":{}}`},
		{name: "duplicate case insensitive keys", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1},{"key":"LOW","upstream_value":"other","unit_price":2}]}},"model_bindings":{}}`},
		{name: "duplicate alias and key", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","aliases":["HIGH"],"unit_price":1},{"key":"high","upstream_value":"high","unit_price":2}]}},"model_bindings":{}}`},
		{name: "empty alias", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","aliases":[" "],"unit_price":1}]}},"model_bindings":{}}`},
		{name: "negative unit price", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":-0.01}]}},"model_bindings":{}}`},
		{name: "trimmed duplicate profile ids", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}," p ":{"name":"other","parameter":"size","default_tier":"1k","tiers":[{"key":"1k","upstream_value":"1024x1024","unit_price":1}]}},"model_bindings":{}}`},
		{name: "missing profile reference", config: `{"version":1,"profiles":{},"model_bindings":{"model":{"profile":"missing"}}}`},
		{name: "negative max n", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{"model":{"profile":"p","max_n":-1}}}`},
		{name: "max n above system limit", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{"model":{"profile":"p","max_n":129}}}`},
		{name: "explicit zero max n", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{"model":{"profile":"p","max_n":0}}}`},
		{name: "null max n", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{"model":{"profile":"p","max_n":null}}}`},
		{name: "trimmed duplicate model names", config: `{"version":1,"profiles":{"p":{"name":"p","parameter":"quality","default_tier":"low","tiers":[{"key":"low","upstream_value":"low","unit_price":1}]}},"model_bindings":{"model":{"profile":"p"}," model ":{"profile":"p"}}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, ValidateImagePricingJSON(test.config))
		})
	}

	require.NoError(t, ValidateImagePricingJSON(strings.Replace(imagePricingValidConfig, `0.04`, `0`, 1)))
	explicitSystemMax := strings.Replace(
		imagePricingValidConfig,
		`{"profile":" adobe-quality-v1 "}`,
		`{"profile":" adobe-quality-v1 ","max_n":128}`,
		1,
	)
	require.NoError(t, ValidateImagePricingJSON(explicitSystemMax))
}

func TestImagePricingRejectsNonFiniteUnitPrices(t *testing.T) {
	for _, price := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		profile := types.ImagePricingProfile{
			Name:        "finite-price",
			Parameter:   types.ImagePricingParameterQuality,
			DefaultTier: "low",
			Tiers: []types.ImagePricingTier{{
				Key:           "low",
				UpstreamValue: "low",
				UnitPrice:     price,
			}},
		}
		_, err := normalizeImagePricingProfile("finite-price", profile)
		require.Error(t, err)
	}
}

func TestImagePricingInvalidUpdateKeepsPreviousSnapshot(t *testing.T) {
	preserveImagePricingConfig(t)
	require.NoError(t, UpdateImagePricingByJSONString(imagePricingValidConfig))

	beforeProfile, beforeBinding, beforeHash, ok := GetImagePricingForModel("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Error(t, UpdateImagePricingByJSONString(`{"version":1,"profiles":{},"model_bindings":{"adobe-gpt-image-2-count":{"profile":"missing"}}}`))
	afterProfile, afterBinding, afterHash, ok := GetImagePricingForModel("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Equal(t, beforeProfile, afterProfile)
	require.Equal(t, beforeBinding, afterBinding)
	require.Equal(t, beforeHash, afterHash)
}

func TestImagePricingDefaultMaxNAndPublicViewAreImmutableAndSanitized(t *testing.T) {
	preserveImagePricingConfig(t)
	require.NoError(t, UpdateImagePricingByJSONString(imagePricingValidConfig))

	profile, binding, profileHash, ok := GetImagePricingForModel("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Equal(t, dto.MaxImageN, binding.MaxN)
	require.Equal(t, "ADOBE quality per image", profile.Name)
	require.Equal(t, types.ImagePricingParameterQuality, profile.Parameter)
	require.Equal(t, "low", profile.DefaultTier)
	require.Equal(t, []string{"auto"}, profile.Tiers[0].Aliases)
	require.NotEmpty(t, profileHash)

	unitPrice, ok := GetImagePricingDefaultUnitPrice("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Equal(t, 0.04, unitPrice)

	public, ok := GetPublicImagePricingForModel("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Equal(t, dto.MaxImageN, public.MaxN)
	require.Equal(t, []string{"auto"}, public.Tiers[0].Aliases)
	serialized, err := common.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "upstream_value")
	require.NotContains(t, string(serialized), "provider-low")
	require.NotContains(t, string(serialized), profileHash)

	copyOfConfig := GetImagePricingConfig()
	mutableProfile := copyOfConfig.Profiles["adobe-quality-v1"]
	mutableProfile.Tiers[0].Key = "mutated"
	mutableProfile.Tiers[0].Aliases[0] = "mutated"
	copyOfConfig.Profiles["adobe-quality-v1"] = mutableProfile
	delete(copyOfConfig.ModelBindings, "adobe-gpt-image-2-count")

	unchangedProfile, unchangedBinding, _, ok := GetImagePricingForModel("adobe-gpt-image-2-count")
	require.True(t, ok)
	require.Equal(t, "low", unchangedProfile.Tiers[0].Key)
	require.Equal(t, []string{"auto"}, unchangedProfile.Tiers[0].Aliases)
	require.Equal(t, "adobe-quality-v1", unchangedBinding.Profile)
}

func TestPublicImagePricingSnapshotIsImmutable(t *testing.T) {
	preserveImagePricingConfig(t)
	require.NoError(t, UpdateImagePricingByJSONString(imagePricingValidConfig))

	snapshot := GetPublicImagePricingSnapshot()
	public := snapshot["adobe-gpt-image-2-count"]
	require.NotNil(t, public)
	price, ok := public.DefaultUnitPrice()
	require.True(t, ok)
	require.Equal(t, 0.04, price)

	require.NoError(t, UpdateImagePricingByJSONString(`{"version":1,"profiles":{},"model_bindings":{}}`))
	require.Empty(t, GetPublicImagePricingSnapshot())
	require.Equal(t, "low", public.Tiers[0].Key)
	require.Equal(t, 0.04, public.Tiers[0].UnitPrice)
}
