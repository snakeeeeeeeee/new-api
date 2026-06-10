package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeImageUsagePrefersInputOutputTokens(t *testing.T) {
	t.Parallel()

	usage := &dto.Usage{
		TotalTokens:  1828,
		InputTokens:  72,
		OutputTokens: 1756,
		InputTokensDetails: &dto.InputTokenDetails{
			TextTokens:  72,
			ImageTokens: 0,
		},
	}

	normalizeImageUsage(usage, 1)

	require.Equal(t, 72, usage.PromptTokens)
	require.Equal(t, 1756, usage.CompletionTokens)
	require.Equal(t, 1828, usage.TotalTokens)
	require.Equal(t, 72, usage.PromptTokensDetails.TextTokens)
}

func TestNormalizeImageUsageFallsBackToImageCount(t *testing.T) {
	t.Parallel()

	usage := &dto.Usage{}

	normalizeImageUsage(usage, 2)

	require.Equal(t, 2, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 2, usage.TotalTokens)
}

func TestImageLogQualityUsesRequestValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, "high", imageLogQuality("high"))
	require.Equal(t, "medium", imageLogQuality(" medium "))
	require.Equal(t, "low", imageLogQuality("low"))
	require.Equal(t, "auto", imageLogQuality("auto"))
	require.Equal(t, "hd", imageLogQuality("hd"))
	require.Equal(t, "standard", imageLogQuality(""))
}

func TestImageHelperForwardsXAIResolutionAndAspectRatio(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	ctx.Set(common.RequestIdKey, "req-xai-image-test")
	ctx.Set(string(constant.ContextKeyChannelType), constant.ChannelTypeXai)
	ctx.Set(string(constant.ContextKeyChannelBaseUrl), "https://api.x.ai")
	ctx.Set(string(constant.ContextKeyChannelKey), "sk-test")
	ctx.Set(string(constant.ContextKeyOriginalModel), "grok-imagine-image-quality")
	ctx.Set("model_mapping", "{}")

	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model": "grok-imagine-image-quality",
		"prompt": "wide mountain landscape",
		"n": 2,
		"response_format": "url",
		"resolution": "2k",
		"aspect_ratio": "16:9",
		"size": "1024x1024"
	}`), &request))

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		RelayFormat:     "openai_image",
		RequestURLPath:  "/v1/images/generations",
		OriginModelName: "grok-imagine-image-quality",
		Request:         &request,
	}

	info.InitChannelMeta(ctx)
	copiedRequest, err := common.DeepCopy(&request)
	require.NoError(t, err)
	require.NoError(t, helper.ModelMappedHelper(ctx, info, copiedRequest))

	converted, err := (&xai.Adaptor{}).ConvertImageRequest(ctx, info, *copiedRequest)
	require.NoError(t, err)
	upstreamBody, err := common.Marshal(converted)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &payload))
	require.Equal(t, "grok-imagine-image-quality", payload["model"])
	require.Equal(t, "wide mountain landscape", payload["prompt"])
	require.Equal(t, float64(2), payload["n"])
	require.Equal(t, "url", payload["response_format"])
	require.Equal(t, "2k", payload["resolution"])
	require.Equal(t, "16:9", payload["aspect_ratio"])
	require.NotContains(t, payload, "size")
}
