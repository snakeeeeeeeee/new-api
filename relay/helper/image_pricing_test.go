package helper

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const imagePricingResolverConfig = `{
  "version": 1,
  "profiles": {
    "quality-v1": {
      "name": "Quality",
      "parameter": "quality",
      "default_tier": "low",
      "tiers": [
        {"key":"low","upstream_value":"provider-low","aliases":["auto"],"unit_price":0.04},
        {"key":"medium","upstream_value":"provider-medium","aliases":[],"unit_price":0.07},
        {"key":"high","upstream_value":"provider-high","aliases":[],"unit_price":0.15}
      ]
    },
    "size-v1": {
      "name": "Size",
      "parameter": "size",
      "default_tier": "1k",
      "tiers": [
        {"key":"1k","upstream_value":"1024x1024","aliases":["square"],"unit_price":0.02},
        {"key":"2k","upstream_value":"2048x2048","aliases":[],"unit_price":0.05}
      ]
    },
    "resolution-v1": {
      "name": "Resolution",
      "parameter": "resolution",
      "default_tier": "sd",
      "tiers": [
        {"key":"sd","upstream_value":"1024x1024","aliases":["1k"],"unit_price":0.03},
        {"key":"hd","upstream_value":"2048x2048","aliases":["2k"],"unit_price":0.09}
      ]
    }
  },
  "model_bindings": {
    "adobe-count": {"profile":"quality-v1","max_n":3},
    "size-count": {"profile":"size-v1","max_n":4},
    "resolution-count": {"profile":"resolution-v1"},
    "gpt-image-1": {"profile":"quality-v1","max_n":3},
    "dall-e-3": {"profile":"size-v1","max_n":3}
  }
}`

func setupImagePricingResolverTest(t *testing.T) {
	t.Helper()
	original := ratio_setting.ImagePricing2JSONString()
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(imagePricingResolverConfig))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(original))
	})
}

func imagePricingN(value uint) *uint {
	return &value
}

func TestResolveImageRequestPricingUsesDefaultTierAndAlias(t *testing.T) {
	setupImagePricingResolverTest(t)

	defaultRequest := &dto.ImageRequest{Model: "gpt-image-2"}
	defaultSnapshot, bound, err := ResolveImageRequestPricing(defaultRequest, "adobe-count", 1)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, "adobe-count", defaultSnapshot.PublicModel)
	require.Equal(t, "quality-v1", defaultSnapshot.ProfileID)
	require.NotEmpty(t, defaultSnapshot.ProfileHash)
	require.Equal(t, types.ImagePricingParameterQuality, defaultSnapshot.Parameter)
	require.Empty(t, defaultSnapshot.RawValue)
	require.Equal(t, "low", defaultSnapshot.EffectiveTier)
	require.Equal(t, "provider-low", defaultSnapshot.UpstreamValue)
	require.Equal(t, types.ImagePricingValueSourceDefault, defaultSnapshot.ValueSource)
	require.Equal(t, 1, defaultSnapshot.N)
	require.Equal(t, "provider-low", defaultRequest.Quality)
	require.NotNil(t, defaultRequest.N)
	require.Equal(t, uint(1), *defaultRequest.N)

	aliasRequest := &dto.ImageRequest{Model: "gpt-image-2", Quality: " AUTO ", N: imagePricingN(2)}
	aliasSnapshot, bound, err := ResolveImageRequestPricing(aliasRequest, "adobe-count", 1.25)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, "AUTO", strings.TrimSpace(aliasSnapshot.RawValue))
	require.Equal(t, "low", aliasSnapshot.EffectiveTier)
	require.Equal(t, types.ImagePricingValueSourceAlias, aliasSnapshot.ValueSource)
	require.Equal(t, "provider-low", aliasRequest.Quality)
	require.InDelta(t, 0.08, aliasSnapshot.Subtotal, 1e-12)
	require.Equal(t, 50000, aliasSnapshot.FinalQuota)
}

func TestResolveImageRequestPricingRejectsUnknownTierAndInvalidN(t *testing.T) {
	setupImagePricingResolverTest(t)
	explicitZeroRequest := &dto.ImageRequest{}
	require.NoError(t, common.Unmarshal([]byte(`{"quality":"low","n":0}`), explicitZeroRequest))
	require.NotNil(t, explicitZeroRequest.N)
	require.Zero(t, *explicitZeroRequest.N)
	require.True(t, explicitZeroRequest.NExplicitZero)

	tests := []struct {
		name        string
		request     *dto.ImageRequest
		wantMessage string
	}{
		{
			name:        "explicit zero",
			request:     explicitZeroRequest,
			wantMessage: "n 必须在 1 到 3 之间",
		},
		{
			name:        "above binding max",
			request:     &dto.ImageRequest{Quality: "low", N: imagePricingN(4)},
			wantMessage: "n 必须在 1 到 3 之间",
		},
		{
			name:        "unknown tier",
			request:     &dto.ImageRequest{Quality: "ultra", N: imagePricingN(1)},
			wantMessage: `quality="ultra" 不受支持`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, bound, err := ResolveImageRequestPricing(test.request, "adobe-count", 1)
			require.Nil(t, snapshot)
			require.True(t, bound)
			require.Error(t, err)
			require.True(t, IsImagePricingRequestError(err))
			require.Contains(t, err.Error(), test.wantMessage)
			if test.name == "unknown tier" {
				require.Contains(t, err.Error(), "high, low, medium")
			}
		})
	}

	atMax := &dto.ImageRequest{Quality: "high", N: imagePricingN(3)}
	snapshot, bound, err := ResolveImageRequestPricing(atMax, "adobe-count", 1)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, 3, snapshot.N)
}

func TestResolveImageRequestPricingNormalizesSizeAndResolution(t *testing.T) {
	setupImagePricingResolverTest(t)

	sizeRequest := &dto.ImageRequest{Quality: "ignored", Size: "SQUARE", N: imagePricingN(2)}
	sizeSnapshot, bound, err := ResolveImageRequestPricing(sizeRequest, "size-count", 1)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, types.ImagePricingParameterSize, sizeSnapshot.Parameter)
	require.Equal(t, "1k", sizeSnapshot.EffectiveTier)
	require.Equal(t, types.ImagePricingValueSourceAlias, sizeSnapshot.ValueSource)
	require.Equal(t, "1024x1024", sizeRequest.Size)
	require.Equal(t, "ignored", sizeRequest.Quality)

	resolutionValue, err := common.Marshal("2K")
	require.NoError(t, err)
	resolutionRequest := &dto.ImageRequest{
		Extra: map[string]json.RawMessage{types.ImagePricingParameterResolution: resolutionValue},
	}
	resolutionSnapshot, bound, err := ResolveImageRequestPricing(resolutionRequest, "resolution-count", 1)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, types.ImagePricingParameterResolution, resolutionSnapshot.Parameter)
	require.Equal(t, "hd", resolutionSnapshot.EffectiveTier)
	require.Equal(t, types.ImagePricingValueSourceAlias, resolutionSnapshot.ValueSource)
	var normalizedResolution string
	require.NoError(t, common.Unmarshal(resolutionRequest.Extra[types.ImagePricingParameterResolution], &normalizedResolution))
	require.Equal(t, "2048x2048", normalizedResolution)

	invalidResolution := &dto.ImageRequest{
		Extra: map[string]json.RawMessage{types.ImagePricingParameterResolution: json.RawMessage(`2`)},
	}
	snapshot, bound, err := ResolveImageRequestPricing(invalidResolution, "resolution-count", 1)
	require.Nil(t, snapshot)
	require.True(t, bound)
	require.Error(t, err)
	require.True(t, IsImagePricingRequestError(err))
	require.Contains(t, err.Error(), "resolution 必须是字符串")
}

func TestResolveImageRequestPricingUsesDecimalQuotaRounding(t *testing.T) {
	setupImagePricingResolverTest(t)

	request := &dto.ImageRequest{Quality: "medium", N: imagePricingN(3)}
	snapshot, bound, err := ResolveImageRequestPricing(request, "adobe-count", 1.3)
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, 0.07, snapshot.UnitPrice)
	require.InDelta(t, 0.21, snapshot.Subtotal, 1e-12)
	require.Equal(t, 1.3, snapshot.GroupRatio)
	require.Equal(t, 136500, snapshot.FinalQuota)

	preConsumed := ImagePricingPriceData(snapshot, types.GroupRatioInfo{GroupRatio: 1.3}, false)
	require.True(t, preConsumed.UsePrice)
	require.Equal(t, 0.21, preConsumed.ModelPrice)
	require.Equal(t, 136500, preConsumed.QuotaToPreConsume)
	require.Zero(t, preConsumed.Quota)
	require.NotSame(t, snapshot, preConsumed.ImagePricing)

	perCall := ImagePricingPriceData(snapshot, types.GroupRatioInfo{GroupRatio: 1.3}, true)
	require.Equal(t, 136500, perCall.Quota)
	require.Zero(t, perCall.QuotaToPreConsume)
}

func TestResolveTaskImagePricingUsesTopLevelFieldsAndMetadataPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupImagePricingResolverTest(t)

	t.Run("json top-level quality and n", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
			"model":"adobe-count",
			"prompt":"cat",
			"quality":"high",
			"n":2
		}`))
		context.Request.Header.Set("Content-Type", "application/json")
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		require.Nil(t, relaycommon.ValidateBasicTaskRequest(context, info, "image_generation"))

		snapshot, bound, err := ResolveTaskImagePricing(context, "adobe-count", 1)
		require.NoError(t, err)
		require.True(t, bound)
		require.Equal(t, "high", snapshot.EffectiveTier)
		require.Equal(t, 2, snapshot.N)
		require.Equal(t, "provider-high", snapshot.UpstreamValue)

		normalized, err := relaycommon.GetTaskRequest(context)
		require.NoError(t, err)
		require.NotNil(t, normalized.Quality)
		require.Equal(t, "provider-high", *normalized.Quality)
		require.Equal(t, "provider-high", normalized.Metadata["quality"])
		require.NotNil(t, normalized.N)
		require.Equal(t, 2, *normalized.N)
		require.Equal(t, 2, normalized.Metadata["n"])
	})

	t.Run("metadata overrides top-level fields", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
			"model":"adobe-count",
			"prompt":"cat",
			"quality":"high",
			"n":2,
			"metadata":{"quality":"auto","n":3}
		}`))
		context.Request.Header.Set("Content-Type", "application/json")
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		require.Nil(t, relaycommon.ValidateBasicTaskRequest(context, info, "image_generation"))

		snapshot, bound, err := ResolveTaskImagePricing(context, "adobe-count", 1)
		require.NoError(t, err)
		require.True(t, bound)
		require.Equal(t, "low", snapshot.EffectiveTier)
		require.Equal(t, types.ImagePricingValueSourceAlias, snapshot.ValueSource)
		require.Equal(t, 3, snapshot.N)

		normalized, err := relaycommon.GetTaskRequest(context)
		require.NoError(t, err)
		require.NotNil(t, normalized.Quality)
		require.Equal(t, "provider-low", *normalized.Quality)
		require.Equal(t, "provider-low", normalized.Metadata["quality"])
		require.NotNil(t, normalized.N)
		require.Equal(t, 3, *normalized.N)
	})

	t.Run("multipart top-level quality and n", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "adobe-count"))
		require.NoError(t, writer.WriteField("prompt", "cat"))
		require.NoError(t, writer.WriteField("quality", "medium"))
		require.NoError(t, writer.WriteField("n", "2"))
		require.NoError(t, writer.Close())

		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", &body)
		context.Request.Header.Set("Content-Type", writer.FormDataContentType())
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		require.Nil(t, relaycommon.ValidateBasicTaskRequest(context, info, "image_generation"))

		snapshot, bound, err := ResolveTaskImagePricing(context, "adobe-count", 1)
		require.NoError(t, err)
		require.True(t, bound)
		require.Equal(t, "medium", snapshot.EffectiveTier)
		require.Equal(t, 2, snapshot.N)
		require.Equal(t, "provider-medium", snapshot.UpstreamValue)
	})

	t.Run("json top-level resolution", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", strings.NewReader(`{
			"model":"resolution-count",
			"prompt":"cat",
			"resolution":"2k"
		}`))
		context.Request.Header.Set("Content-Type", "application/json")
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		require.Nil(t, relaycommon.ValidateBasicTaskRequest(context, info, "image_generation"))

		snapshot, bound, err := ResolveTaskImagePricing(context, "resolution-count", 1)
		require.NoError(t, err)
		require.True(t, bound)
		require.Equal(t, "hd", snapshot.EffectiveTier)
		require.Equal(t, "2048x2048", snapshot.UpstreamValue)

		normalized, err := relaycommon.GetTaskRequest(context)
		require.NoError(t, err)
		require.NotNil(t, normalized.Resolution)
		require.Equal(t, "2048x2048", *normalized.Resolution)
		require.Equal(t, "2048x2048", normalized.Metadata["resolution"])
	})
}

func TestModelPriceHelperPricesPublicModelBeforeMappingAndFallsBackWhenUnbound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupImagePricingResolverTest(t)
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"adobe-count":99,"legacy-image":0.1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1.5}`))

	boundContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	boundRequest := &dto.ImageRequest{Model: "gpt-image-2", Quality: "high", N: imagePricingN(2)}
	boundInfo := &relaycommon.RelayInfo{
		OriginModelName: "adobe-count",
		UserGroup:       "default",
		UsingGroup:      "default",
		Request:         boundRequest,
	}
	boundPrice, err := ModelPriceHelper(boundContext, boundInfo, 0, &types.TokenCountMeta{ImagePriceRatio: 20})
	require.NoError(t, err)
	require.NotNil(t, boundPrice.ImagePricing)
	require.Equal(t, "adobe-count", boundPrice.ImagePricing.PublicModel)
	require.Equal(t, "gpt-image-2", boundRequest.Model)
	require.Equal(t, "provider-high", boundRequest.Quality)
	require.Equal(t, 0.30, boundPrice.ModelPrice)
	require.Equal(t, 225000, boundPrice.QuotaToPreConsume)
	require.Equal(t, boundPrice, boundInfo.PriceData)

	legacyContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	legacyRequest := &dto.ImageRequest{Model: "legacy-upstream", N: imagePricingN(2)}
	legacyInfo := &relaycommon.RelayInfo{
		OriginModelName: "legacy-image",
		UserGroup:       "default",
		UsingGroup:      "default",
		Request:         legacyRequest,
	}
	legacyPrice, err := ModelPriceHelper(legacyContext, legacyInfo, 0, &types.TokenCountMeta{ImagePriceRatio: 3})
	require.NoError(t, err)
	require.Nil(t, legacyPrice.ImagePricing)
	require.True(t, legacyPrice.UsePrice)
	require.InDelta(t, 0.30, legacyPrice.ModelPrice, 1e-12)
	require.Equal(t, 225000, legacyPrice.QuotaToPreConsume)
	require.Equal(t, "legacy-upstream", legacyRequest.Model)
}

func TestCountAndTokenAliasesCanMapToSameUpstreamWithIndependentBilling(t *testing.T) {
	original := ratio_setting.ImagePricing2JSONString()
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(`{
		"version":1,
		"profiles":{
			"adobe-quality-v1":{
				"name":"ADOBE quality per image",
				"parameter":"quality",
				"default_tier":"low",
				"tiers":[{"key":"low","upstream_value":"low","aliases":[],"unit_price":0.04}]
			}
		},
		"model_bindings":{
			"adobe-gpt-image-2-count":{"profile":"adobe-quality-v1","max_n":10}
		}
	}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(original))
	})

	require.True(t, ratio_setting.IsImageParameterPricedModel("adobe-gpt-image-2-count"))
	require.False(t, ratio_setting.IsImageParameterPricedModel("adobe-gpt-image-2-token"))

	const mapping = `{
		"adobe-gpt-image-2-count":"gpt-image-2",
		"adobe-gpt-image-2-token":"gpt-image-2"
	}`
	for _, publicModel := range []string{"adobe-gpt-image-2-count", "adobe-gpt-image-2-token"} {
		t.Run(publicModel, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Set("model_mapping", mapping)
			request := &dto.ImageRequest{Model: publicModel}
			info := &relaycommon.RelayInfo{OriginModelName: publicModel}

			require.NoError(t, ModelMappedHelper(context, info, request))
			require.True(t, info.IsModelMapped)
			require.Equal(t, "gpt-image-2", info.UpstreamModelName)
			require.Equal(t, "gpt-image-2", request.Model)
		})
	}
}
