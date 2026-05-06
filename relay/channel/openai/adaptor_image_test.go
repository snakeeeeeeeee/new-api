package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestStripsResponseFormatForGPTImageOnOpenAIAdaptor(t *testing.T) {
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
	require.Equal(t, "gpt-image-2", imageRequest.Model)
	require.Empty(t, imageRequest.ResponseFormat)
}

func TestConvertImageRequestKeepsResponseFormatForNonGPTImage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &Adaptor{}
	request := dto.ImageRequest{
		Model:          "dall-e-3",
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

func TestImageRequestIsStreamPreservesExplicitStream(t *testing.T) {
	t.Parallel()

	stream := true
	request := dto.ImageRequest{Stream: &stream}
	require.True(t, request.IsStream(nil))

	stream = false
	request = dto.ImageRequest{Stream: &stream}
	require.False(t, request.IsStream(nil))

	request = dto.ImageRequest{}
	require.False(t, request.IsStream(nil))
}

func TestDoResponseWithoutImageAdapterKeepsOpenAIImageResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	body := `{"created":1776956476,"data":[{"url":"https://example.com/a.png","revised_prompt":"prompt-a"}],"background":"opaque","output_format":"png","quality":"high","size":"1024x1024","usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	usageAny, err := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, err)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.TotalTokens)
	require.Equal(t, body, recorder.Body.String())
}

func TestDoResponseWithoutImageAdapterTreatsOpenAIErrorBodyAsFailure(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	body := `{"error":{"message":"stream disconnected before completion","type":"server_error","code":"internal_server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	usageAny, err := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, usageAny)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.Equal(t, types.ErrorCode("internal_server_error"), err.GetErrorCode())
	require.Equal(t, "stream disconnected before completion", err.ToOpenAIError().Message)
	require.Empty(t, recorder.Body.String())
}

func TestDoResponseWithoutImageAdapterKeepsLargeBase64ImageResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	largeB64 := strings.Repeat("A", 11<<20)
	body := `{"created":1776956476,"data":[{"b64_json":"` + largeB64 + `","revised_prompt":"prompt-a"}],"usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	usageAny, err := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, err)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.TotalTokens)
	require.Equal(t, len(body), recorder.Body.Len())
	require.Equal(t, strconv.Itoa(len(body)), recorder.Header().Get("Content-Length"))
	require.Equal(t, body[:128], recorder.Body.String()[:128])
	require.Equal(t, body[len(body)-128:], recorder.Body.String()[recorder.Body.Len()-128:])
}

func TestDoResponseWithImageStreamPassesThroughSSEAndExtractsUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	body := "data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"abc\"}\n\n" +
		"data: {\"type\":\"image_generation.completed\",\"b64_json\":\"def\",\"usage\":{\"input_tokens\":12,\"output_tokens\":34,\"total_tokens\":46}}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		IsStream:  true,
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	usageAny, err := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, err)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.TotalTokens)
	require.Equal(t, 12, usage.InputTokens)
	require.Equal(t, 34, usage.OutputTokens)
	require.Contains(t, recorder.Body.String(), "event: image_generation.partial_image")
	require.Contains(t, recorder.Body.String(), "data: {\"type\":\"image_generation.partial_image\"")
	require.Contains(t, recorder.Body.String(), "event: image_generation.completed")
	require.Contains(t, recorder.Body.String(), "data: {\"type\":\"image_generation.completed\"")
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
}

func TestDoResponseWithCPAImageAdapterPreservesStandardImageResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	body := `{"created":1776956476,"data":[{"b64_json":"abc","revised_prompt":"prompt-a"}],"background":"opaque","output_format":"png","quality":"high","size":"1024x1024","usage":{"input_tokens":12,"output_tokens":34,"total_tokens":46}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ImageResponseAdapter: "cpa",
			},
		},
	}

	usageAny, err := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, err)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.TotalTokens)
	require.Equal(t, body, recorder.Body.String())
}

func TestDoResponseWithCPAImageAdapterConvertsCustomGenerationResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	payload := map[string]any{
		"result": map[string]any{
			"created":       1777000001,
			"background":    "opaque",
			"output_format": "png",
			"quality":       "high",
			"size":          "1024x1024",
			"images": []map[string]any{
				{
					"image_url":      "https://example.com/generated-a.png",
					"revised_prompt": "prompt-a",
				},
				{
					"binary_data_base64": "YmFzZTY0LWltYWdl",
					"final_prompt":       "prompt-b",
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 34,
			"total_tokens":  46,
		},
	}
	bodyBytes, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1776999999, 0),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ImageResponseAdapter: "cpa",
			},
		},
	}

	usageAny, newAPIError := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, newAPIError)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.TotalTokens)

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Equal(t, int64(1777000001), imageResponse.Created)
	require.Equal(t, "opaque", imageResponse.Background)
	require.Equal(t, "png", imageResponse.OutputFormat)
	require.Equal(t, "high", imageResponse.Quality)
	require.Equal(t, "1024x1024", imageResponse.Size)
	require.NotNil(t, imageResponse.Usage)
	require.Equal(t, 46, imageResponse.Usage.TotalTokens)
	require.Len(t, imageResponse.Data, 2)
	require.Equal(t, "https://example.com/generated-a.png", imageResponse.Data[0].Url)
	require.Equal(t, "prompt-a", imageResponse.Data[0].RevisedPrompt)
	require.Equal(t, "YmFzZTY0LWltYWdl", imageResponse.Data[1].B64Json)
	require.Equal(t, "prompt-b", imageResponse.Data[1].RevisedPrompt)
}

func TestDoResponseWithCPAImageAdapterConvertsCustomEditResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	payload := map[string]any{
		"output": map[string]any{
			"created":    1777000002,
			"background": "transparent",
			"format":     "png",
			"quality":    "medium",
			"size":       "1024x1024",
			"results": []map[string]any{
				{
					"url":            "https://example.com/edited-a.png",
					"revised_prompt": "prompt-edit",
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  22,
			"output_tokens": 44,
			"total_tokens":  66,
		},
	}
	bodyBytes, err := common.Marshal(payload)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		StartTime: time.Unix(1776999990, 0),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ImageResponseAdapter: "cpa",
			},
		},
	}

	usageAny, newAPIError := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, newAPIError)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 66, usage.TotalTokens)

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Equal(t, int64(1777000002), imageResponse.Created)
	require.Equal(t, "transparent", imageResponse.Background)
	require.Equal(t, "png", imageResponse.OutputFormat)
	require.Equal(t, "medium", imageResponse.Quality)
	require.Len(t, imageResponse.Data, 1)
	require.Equal(t, "https://example.com/edited-a.png", imageResponse.Data[0].Url)
	require.Equal(t, "prompt-edit", imageResponse.Data[0].RevisedPrompt)
}
