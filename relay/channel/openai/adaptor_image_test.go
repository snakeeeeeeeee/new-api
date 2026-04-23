package openai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestStripsResponseFormatForAzureGPTImage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &Adaptor{}
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "future warrior",
		ResponseFormat: "b64_json",
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAzure,
			UpstreamModelName: "gpt-image-2",
		},
	}

	converted, err := adaptor.ConvertImageRequest(ctx, info, request)
	require.NoError(t, err)

	imageRequest, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-image-2", imageRequest.Model)
	require.Empty(t, imageRequest.ResponseFormat)
}

func TestConvertImageRequestKeepsResponseFormatForNonAzureGPTImage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &Adaptor{}
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "future warrior",
		ResponseFormat: "b64_json",
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-image-2",
		},
	}

	converted, err := adaptor.ConvertImageRequest(ctx, info, request)
	require.NoError(t, err)

	imageRequest, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	require.Equal(t, "b64_json", imageRequest.ResponseFormat)
}
