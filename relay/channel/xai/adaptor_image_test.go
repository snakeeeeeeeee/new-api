package xai

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

func TestGetRequestURLAppendsImageGenerationPathTo2KENBaseURL(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/images/generations",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeXai,
			ChannelBaseUrl: "https://apis.2ken.com",
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://apis.2ken.com/v1/images/generations", requestURL)
}

func TestConvertImageEditRequestPreservesOfficialXAISources(t *testing.T) {
	t.Parallel()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"combine references",
		"images":[
			{"url":"https://cdn.example.com/a.png"},
			{"url":"data:image/png;base64,AAAA"},
			{"file_id":"file_official_1"}
		]
	}`), &request))
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXai}}
	copied, err := common.DeepCopy(&request)
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertImageRequest(ctx, info, *copied)
	require.NoError(t, err)
	payload := converted.(ImageRequest)
	require.Nil(t, payload.Image)
	require.Len(t, payload.Images, 3)
	require.Equal(t, "https://cdn.example.com/a.png", payload.Images[0].URL)
	require.Equal(t, "data:image/png;base64,AAAA", payload.Images[1].URL)
	require.Equal(t, "file_official_1", payload.Images[2].FileID)
}

func TestConvertImageEditRequestApplies2KENModelLimitsAndRejectsFileIDs(t *testing.T) {
	t.Parallel()
	newRequest := func(t *testing.T, model string, images string) dto.ImageRequest {
		t.Helper()
		var request dto.ImageRequest
		require.NoError(t, common.Unmarshal([]byte(`{"model":"`+model+`","prompt":"edit","images":`+images+`}`), &request))
		return request
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits, ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:          constant.ChannelTypeXai,
		ChannelOtherSettings: dto.ChannelOtherSettings{XAIAPIVariant: dto.XAIAPIVariant2KEN},
	}}

	five := `[{"url":"https://cdn.example.com/1.png"},{"url":"https://cdn.example.com/2.png"},{"url":"https://cdn.example.com/3.png"},{"url":"https://cdn.example.com/4.png"},{"url":"https://cdn.example.com/5.png"}]`
	converted, err := (&Adaptor{}).ConvertImageRequest(ctx, info, newRequest(t, "grok-imagine-image", five))
	require.NoError(t, err)
	require.Len(t, converted.(ImageRequest).Images, 5)

	six := strings.TrimSuffix(five, "]") + `,{"url":"https://cdn.example.com/6.png"}]`
	_, err = (&Adaptor{}).ConvertImageRequest(ctx, info, newRequest(t, "grok-imagine-image", six))
	require.ErrorContains(t, err, "at most 5")

	four := `[{"url":"https://cdn.example.com/1.png"},{"url":"https://cdn.example.com/2.png"},{"url":"https://cdn.example.com/3.png"},{"url":"https://cdn.example.com/4.png"}]`
	_, err = (&Adaptor{}).ConvertImageRequest(ctx, info, newRequest(t, "grok-imagine-image-quality", four))
	require.ErrorContains(t, err, "at most 3")

	_, err = (&Adaptor{}).ConvertImageRequest(ctx, info, newRequest(t, "grok-imagine-image", `[{"file_id":"file_unverified"}]`))
	require.ErrorContains(t, err, "do not support file_id")
}

func TestConvertMultipartImageEditToXAIJSONDataURIs(t *testing.T) {
	t.Parallel()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-imagine-image"))
	require.NoError(t, writer.WriteField("prompt", "combine both"))
	for _, name := range []string{"first.png", "second.png"} {
		part, err := writer.CreateFormFile("image", name)
		require.NoError(t, err)
		_, err = part.Write([]byte("\x89PNG\r\n\x1a\n"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	_, err := ctx.MultipartForm()
	require.NoError(t, err)
	request := dto.ImageRequest{Model: "grok-imagine-image", Prompt: "combine both"}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits, ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:          constant.ChannelTypeXai,
		ChannelOtherSettings: dto.ChannelOtherSettings{XAIAPIVariant: dto.XAIAPIVariant2KEN},
	}}

	converted, err := (&Adaptor{}).ConvertImageRequest(ctx, info, request)
	require.NoError(t, err)
	payload := converted.(ImageRequest)
	require.Len(t, payload.Images, 2)
	require.True(t, strings.HasPrefix(payload.Images[0].URL, "data:image/png;base64,"))
	require.True(t, strings.HasPrefix(payload.Images[1].URL, "data:image/png;base64,"))

	header := http.Header{"Content-Type": []string{writer.FormDataContentType()}}
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(ctx, &header, info))
	require.Equal(t, "application/json", header.Get("Content-Type"))
}
