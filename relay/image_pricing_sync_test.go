package relay

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	openaiadaptor "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const syncImagePricingConfig = `{
  "version": 1,
  "profiles": {
    "sync-quality-v1": {
      "name": "Sync quality",
      "parameter": "quality",
      "default_tier": "low",
      "tiers": [
        {"key":"low","upstream_value":"provider-low","aliases":["auto"],"unit_price":0.04},
        {"key":"high","upstream_value":"provider-high","aliases":[],"unit_price":0.15}
      ]
    },
    "sync-resolution-v1": {
      "name": "Sync resolution",
      "parameter": "resolution",
      "default_tier": "1k",
      "tiers": [
        {"key":"1k","upstream_value":"1024x1024","aliases":[],"unit_price":0.03},
        {"key":"2k","upstream_value":"2048x2048","aliases":["hd"],"unit_price":0.09}
      ]
    }
  },
  "model_bindings": {
    "public-image-count": {"profile":"sync-quality-v1","max_n":10},
    "public-image-resolution": {"profile":"sync-resolution-v1","max_n":10}
  }
}`

func setupSyncImagePricingTest(t *testing.T) {
	t.Helper()
	originalPricing := ratio_setting.ImagePricing2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(syncImagePricingConfig))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1.25}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalPricing))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
}

func newSyncImageJSONContext(path, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func imagePricingRelayInfo(mode int, request *dto.ImageRequest) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:       mode,
		RelayFormat:     types.RelayFormatOpenAIImage,
		RequestURLPath:  "/v1/images/generations",
		OriginModelName: "public-image-count",
		UserGroup:       "default",
		UsingGroup:      "default",
		Request:         request,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ApiType:     constant.APITypeOpenAI,
		},
	}
}

func normalizeAndMapSyncImageRequest(t *testing.T, ctx *gin.Context, info *relaycommon.RelayInfo, request *dto.ImageRequest) *dto.ImageRequest {
	t.Helper()
	priceData, err := helper.ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	require.NotNil(t, priceData.ImagePricing)

	upstreamRequest, err := common.DeepCopy(request)
	require.NoError(t, err)
	ctx.Set("model_mapping", `{"public-image-count":"gpt-image-2"}`)
	require.NoError(t, helper.ModelMappedHelper(ctx, info, upstreamRequest))
	return upstreamRequest
}

func TestSyncImageGenerationInjectsDefaultTierBeforeModelMapping(t *testing.T) {
	setupSyncImagePricingTest(t)

	ctx := newSyncImageJSONContext(
		"/v1/images/generations",
		`{"model":"public-image-count","prompt":"cat with a bell"}`,
	)
	request, err := helper.GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)
	require.NoError(t, err)
	info := imagePricingRelayInfo(relayconstant.RelayModeImagesGenerations, request)
	upstreamRequest := normalizeAndMapSyncImageRequest(t, ctx, info, request)

	require.Equal(t, "public-image-count", info.PriceData.ImagePricing.PublicModel)
	require.Equal(t, types.ImagePricingValueSourceDefault, info.PriceData.ImagePricing.ValueSource)
	require.Empty(t, info.PriceData.ImagePricing.RawValue)
	require.Equal(t, "low", info.PriceData.ImagePricing.EffectiveTier)
	require.Equal(t, 25000, info.PriceData.ImagePricing.FinalQuota)
	require.Equal(t, "gpt-image-2", info.UpstreamModelName)
	require.Equal(t, "gpt-image-2", upstreamRequest.Model)
	require.Equal(t, "provider-low", upstreamRequest.Quality)
	require.NotNil(t, upstreamRequest.N)
	require.Equal(t, uint(1), *upstreamRequest.N)

	converted, err := (&openaiadaptor.Adaptor{}).ConvertImageRequest(ctx, info, *upstreamRequest)
	require.NoError(t, err)
	upstreamBody, err := common.Marshal(converted)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &payload))
	require.Equal(t, "gpt-image-2", payload["model"])
	require.Equal(t, "provider-low", payload["quality"])
	require.Equal(t, float64(1), payload["n"])
}

func TestSyncImageEditNormalizesAliasAndRewritesMultipartAfterModelMapping(t *testing.T) {
	setupSyncImagePricingTest(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "public-image-count"))
	require.NoError(t, writer.WriteField("prompt", "replace the background"))
	require.NoError(t, writer.WriteField("quality", "AUTO"))
	require.NoError(t, writer.WriteField("n", "2"))
	imagePart, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("fake-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	request, err := helper.GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesEdits)
	require.NoError(t, err)
	info := imagePricingRelayInfo(relayconstant.RelayModeImagesEdits, request)
	info.RequestURLPath = "/v1/images/edits"
	upstreamRequest := normalizeAndMapSyncImageRequest(t, ctx, info, request)

	require.Equal(t, types.ImagePricingValueSourceAlias, info.PriceData.ImagePricing.ValueSource)
	require.Equal(t, "AUTO", info.PriceData.ImagePricing.RawValue)
	require.Equal(t, "low", info.PriceData.ImagePricing.EffectiveTier)
	require.Equal(t, 50000, info.PriceData.ImagePricing.FinalQuota)
	require.Equal(t, "gpt-image-2", upstreamRequest.Model)
	require.Equal(t, "provider-low", upstreamRequest.Quality)
	require.Equal(t, uint(2), *upstreamRequest.N)

	converted, err := (&openaiadaptor.Adaptor{}).ConvertImageRequest(ctx, info, *upstreamRequest)
	require.NoError(t, err)
	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)
	upstreamHTTP := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
	upstreamHTTP.Header.Set("Content-Type", ctx.Request.Header.Get("Content-Type"))
	require.NoError(t, upstreamHTTP.ParseMultipartForm(1<<20))
	require.Equal(t, "gpt-image-2", upstreamHTTP.FormValue("model"))
	require.Equal(t, "replace the background", upstreamHTTP.FormValue("prompt"))
	require.Equal(t, "provider-low", upstreamHTTP.FormValue("quality"))
	require.Equal(t, "2", upstreamHTTP.FormValue("n"))
	require.Len(t, upstreamHTTP.MultipartForm.File["image"], 1)
}

func TestSyncImageGenerationForwardsNormalizedResolutionInFinalJSON(t *testing.T) {
	setupSyncImagePricingTest(t)

	ctx := newSyncImageJSONContext(
		"/v1/images/generations",
		`{"model":"public-image-resolution","prompt":"large poster","resolution":"hd"}`,
	)
	request, err := helper.GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)
	require.NoError(t, err)
	info := imagePricingRelayInfo(relayconstant.RelayModeImagesGenerations, request)
	info.OriginModelName = "public-image-resolution"

	priceData, err := helper.ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	require.NoError(t, err)
	require.NotNil(t, priceData.ImagePricing)
	require.Equal(t, types.ImagePricingParameterResolution, priceData.ImagePricing.Parameter)
	require.Equal(t, types.ImagePricingValueSourceAlias, priceData.ImagePricing.ValueSource)
	require.Equal(t, "2048x2048", priceData.ImagePricing.UpstreamValue)

	upstreamRequest, err := common.DeepCopy(request)
	require.NoError(t, err)
	ctx.Set("model_mapping", `{"public-image-resolution":"gpt-image-2"}`)
	require.NoError(t, helper.ModelMappedHelper(ctx, info, upstreamRequest))
	converted, err := (&openaiadaptor.Adaptor{}).ConvertImageRequest(ctx, info, *upstreamRequest)
	require.NoError(t, err)
	upstreamBody, err := common.Marshal(converted)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &payload))
	require.Equal(t, "gpt-image-2", payload["model"])
	require.Equal(t, "2048x2048", payload["resolution"])
}

func TestSyncImagePricingSnapshotWinsOverChannelParamOverride(t *testing.T) {
	setupSyncImagePricingTest(t)

	ctx := newSyncImageJSONContext(
		"/v1/images/generations",
		`{"model":"public-image-count","prompt":"cat with a bell","quality":"auto","n":2}`,
	)
	request, err := helper.GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesGenerations)
	require.NoError(t, err)
	info := imagePricingRelayInfo(relayconstant.RelayModeImagesGenerations, request)
	upstreamRequest := normalizeAndMapSyncImageRequest(t, ctx, info, request)
	require.Equal(t, "provider-low", info.PriceData.ImagePricing.UpstreamValue)
	require.Equal(t, 2, info.PriceData.ImagePricing.N)

	converted, err := (&openaiadaptor.Adaptor{}).ConvertImageRequest(ctx, info, *upstreamRequest)
	require.NoError(t, err)
	upstreamBody, err := common.Marshal(converted)
	require.NoError(t, err)
	info.ParamOverride = map[string]interface{}{
		"quality": "channel-forced-quality",
		"n":       9,
	}
	upstreamBody, err = relaycommon.ApplyParamOverrideWithRelayInfo(upstreamBody, info)
	require.NoError(t, err)
	upstreamBody, err = restoreImagePricingParameters(upstreamBody, info.PriceData.ImagePricing)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &payload))
	require.Equal(t, "provider-low", payload["quality"])
	require.Equal(t, float64(2), payload["n"])
}
