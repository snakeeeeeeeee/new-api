package xai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestForwardsXAIResolutionAndAspectRatio(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

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
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeXai,
			UpstreamModelName: "grok-imagine-image-quality",
		},
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(ctx, info, request)
	require.NoError(t, err)

	data, err := common.Marshal(converted)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Equal(t, "grok-imagine-image-quality", payload["model"])
	require.Equal(t, "wide mountain landscape", payload["prompt"])
	require.Equal(t, float64(2), payload["n"])
	require.Equal(t, "url", payload["response_format"])
	require.Equal(t, "2k", payload["resolution"])
	require.Equal(t, "16:9", payload["aspect_ratio"])
	require.NotContains(t, payload, "size")
}

func TestXAIModelListIncludesCurrentImageQualityModel(t *testing.T) {
	t.Parallel()

	require.Contains(t, ModelList, "grok-imagine-image-quality")
	require.Contains(t, ModelList, "grok-imagine-image")
}
