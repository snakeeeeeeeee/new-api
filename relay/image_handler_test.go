package relay

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/types"
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

func TestShouldUseImageHandleSyncHonorsGlobalAndChannelMode(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()

	baseInfo := func(mode string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeImagesGenerations,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType: constant.ChannelTypeOpenAI,
				ChannelOtherSettings: dto.ChannelOtherSettings{
					ImageHandleSyncMode: mode,
				},
			},
		}
	}

	image_handle_setting.GetImageHandleSetting().SyncImageEnabled = false
	require.False(t, shouldUseImageHandleSync(baseInfo("inherit")))
	require.True(t, shouldUseImageHandleSync(baseInfo("force_on")))

	image_handle_setting.GetImageHandleSetting().SyncImageEnabled = true
	require.True(t, shouldUseImageHandleSync(baseInfo("inherit")))
	require.False(t, shouldUseImageHandleSync(baseInfo("force_off")))

	unsupported := baseInfo("force_on")
	unsupported.ChannelType = constant.ChannelTypeGemini
	unsupported.ChannelMeta.ChannelType = constant.ChannelTypeGemini
	require.False(t, shouldUseImageHandleSync(unsupported))
}

func TestCanUseImageHandleSyncForRequestAllowsEditUploads(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().SyncImageEnabled = true

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ImageHandleSyncMode: "inherit",
			},
		},
	}

	urlEdit := dto.ImageRequest{
		Prompt: "edit",
		Image:  []byte(`"https://cdn.example.com/input.png"`),
		Extra: map[string]json.RawMessage{
			"mask": []byte(`"https://cdn.example.com/mask.png"`),
		},
	}
	require.True(t, canUseImageHandleSyncForRequest(info, &urlEdit))

	base64Edit := dto.ImageRequest{
		Prompt: "edit",
		Image:  []byte(`"data:image/png;base64,AAAA"`),
	}
	require.True(t, canUseImageHandleSyncForRequest(info, &base64Edit))

	multipartWithoutURL := dto.ImageRequest{Prompt: "edit"}
	require.True(t, canUseImageHandleSyncForRequest(info, &multipartWithoutURL))
}

func TestImageHandleSyncToOpenAIResponseMapsUsageAndImages(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status: "succeeded",
		Result: &imageHandleSyncResult{Images: []imageHandleSyncImage{
			{URL: "https://example.com/a.png", MimeType: "image/png", RevisedPrompt: "prompt-a"},
			{URL: "https://example.com/b.png", MimeType: "image/png"},
		}},
		Usage: &imageHandleSyncUsage{
			InputTokens:              19,
			OutputTokens:             781,
			TotalTokens:              800,
			CacheCreationInputTokens: 5,
			CacheReadTokens:          7,
		},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{Quality: "auto", Size: "1024x1024"})

	require.Len(t, imageResp.Data, 2)
	require.Equal(t, "https://example.com/a.png", imageResp.Data[0].Url)
	require.Equal(t, "prompt-a", imageResp.Data[0].RevisedPrompt)
	require.NotNil(t, imageResp.Usage)
	require.Equal(t, 19, imageResp.Usage.PromptTokens)
	require.Equal(t, 781, imageResp.Usage.CompletionTokens)
	require.Equal(t, 800, imageResp.Usage.TotalTokens)
	require.Equal(t, 7, imageResp.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 5, imageResp.Usage.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, "image_handle_sync", imageResp.Usage.UsageSource)
}

func TestImageHandleSyncPreservesSignedURLAcrossJSONBoundaries(t *testing.T) {
	t.Parallel()

	const expectedURL = "https://signed.example.com/out.png?x=1&X-Amz-Credential=AKIA%2F20260714%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Signature=abc123"
	var response imageHandleSyncResponse
	require.NoError(t, common.Unmarshal([]byte(`{"status":"succeeded","result":{"images":[{"url":"https://signed.example.com/out.png?x=1\u0026X-Amz-Credential=AKIA%2F20260714%2Fus-west-2%2Fs3%2Faws4_request\u0026X-Amz-Signature=abc123"}]}}`), &response))

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{})
	require.Len(t, imageResp.Data, 1)
	require.Equal(t, expectedURL, imageResp.Data[0].Url)

	clientJSON, err := common.Marshal(imageHandleSyncClientImageResponse(imageResp))
	require.NoError(t, err)
	require.Contains(t, string(clientJSON), `\u0026`)
	require.NotContains(t, string(clientJSON), `\\u0026`)
	var clientRoundTrip imageHandleSyncOpenAIImageResponse
	require.NoError(t, common.Unmarshal(clientJSON, &clientRoundTrip))
	require.Equal(t, expectedURL, clientRoundTrip.Data[0].Url)

	var taskData map[string]any
	require.NoError(t, common.Unmarshal(imageHandleSyncTaskData(response), &taskData))
	result := taskData["result"].(map[string]any)
	images := result["images"].([]any)
	require.Equal(t, expectedURL, images[0].(map[string]any)["url"])
}

func TestImageHandleSyncClientImageResponseUsesSingleImageUsageVocabulary(t *testing.T) {
	t.Parallel()

	imageResp := &dto.ImageResponse{
		Created: 1782935561,
		Data:    []dto.ImageData{{Url: "https://example.com/a.png"}},
		Usage: &dto.Usage{
			PromptTokens:     14,
			CompletionTokens: 196,
			TotalTokens:      210,
			InputTokens:      14,
			OutputTokens:     196,
			UsageSource:      "image_handle_sync",
			PromptTokensDetails: dto.InputTokenDetails{
				TextTokens: 14,
			},
			InputTokensDetails: &dto.InputTokenDetails{
				TextTokens: 14,
			},
		},
	}

	data, err := common.Marshal(imageHandleSyncClientImageResponse(imageResp))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(data, &body))
	usage, ok := body["usage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(14), usage["input_tokens"])
	require.Equal(t, float64(196), usage["output_tokens"])
	require.Equal(t, float64(210), usage["total_tokens"])
	require.Equal(t, "image_handle_sync", usage["usage_source"])
	require.NotContains(t, usage, "prompt_tokens")
	require.NotContains(t, usage, "completion_tokens")
	require.NotContains(t, usage, "prompt_tokens_details")
	details, ok := usage["input_tokens_details"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(14), details["text_tokens"])
}

func TestImageHandleSyncToOpenAIResponseBackfillsOpenAITopFieldsFromRawResponse(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status: "succeeded",
		Result: &imageHandleSyncResult{
			Images: []imageHandleSyncImage{{URL: "https://example.com/a.png"}},
			RawResponse: map[string]any{
				"created":       float64(1782507006),
				"background":    "opaque",
				"output_format": "png",
				"quality":       "low",
				"size":          "1024x1024",
				"usage": map[string]any{
					"input_tokens":  14,
					"output_tokens": 196,
					"total_tokens":  210,
				},
			},
		},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{Quality: "auto", Size: "256x256"})

	require.Len(t, imageResp.Data, 1)
	require.Equal(t, "https://example.com/a.png", imageResp.Data[0].Url)
	require.Equal(t, int64(1782507006), imageResp.Created)
	require.Equal(t, "opaque", imageResp.Background)
	require.Equal(t, "png", imageResp.OutputFormat)
	require.Equal(t, "low", imageResp.Quality)
	require.Equal(t, "1024x1024", imageResp.Size)
	require.NotNil(t, imageResp.Usage)
	require.Equal(t, 14, imageResp.Usage.InputTokens)
	require.Equal(t, 196, imageResp.Usage.OutputTokens)
	require.Equal(t, 210, imageResp.Usage.TotalTokens)
}

func TestImageHandleSyncToOpenAIResponseBackfillsOpenAITopFieldsFromResultOutput(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status: "succeeded",
		Result: &imageHandleSyncResult{
			Images: []imageHandleSyncImage{{URL: "https://example.com/a.png"}},
			Output: map[string]any{
				"created":       float64(1782581166),
				"background":    "opaque",
				"output_format": "png",
				"quality":       "high",
				"size":          "1024x1024",
			},
		},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{Quality: "auto", Size: "auto"})

	require.Len(t, imageResp.Data, 1)
	require.Equal(t, int64(1782581166), imageResp.Created)
	require.Equal(t, "opaque", imageResp.Background)
	require.Equal(t, "png", imageResp.OutputFormat)
	require.Equal(t, "high", imageResp.Quality)
	require.Equal(t, "1024x1024", imageResp.Size)
}

func TestImageHandleSyncToOpenAIResponseMapsBase64Images(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status:           "succeeded",
		ResultDataFormat: "base64",
		Result: &imageHandleSyncResult{Images: []imageHandleSyncImage{
			{B64Json: "iVBORw0KGgo=", MimeType: "image/png", RevisedPrompt: "prompt-a"},
		}},
		Usage: &imageHandleSyncUsage{
			InputTokens:  19,
			OutputTokens: 781,
			TotalTokens:  800,
		},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{ResponseFormat: "b64_json"})

	require.Len(t, imageResp.Data, 1)
	require.Empty(t, imageResp.Data[0].Url)
	require.Equal(t, "iVBORw0KGgo=", imageResp.Data[0].B64Json)
	require.Equal(t, "prompt-a", imageResp.Data[0].RevisedPrompt)
	require.NotNil(t, imageResp.Usage)
	require.Equal(t, 19, imageResp.Usage.PromptTokens)
	require.Equal(t, 781, imageResp.Usage.CompletionTokens)
	require.Equal(t, 800, imageResp.Usage.TotalTokens)
}

func TestImageHandleSyncTaskDataKeepsExtendedResultFields(t *testing.T) {
	t.Parallel()

	data := imageHandleSyncTaskData(imageHandleSyncResponse{
		TaskID:           "imgtask_123",
		ProviderTaskID:   "imgtask_123",
		ClientTaskID:     "task_123",
		Status:           "succeeded",
		ResultDataFormat: "url",
		Result: &imageHandleSyncResult{
			Images: []imageHandleSyncImage{{
				URL:           "https://example.com/a.png",
				MimeType:      "image/png",
				Format:        "png",
				Filename:      "a.png",
				SizeBytes:     1234,
				Width:         1024,
				Height:        768,
				RevisedPrompt: "prompt-a",
			}},
			Output: map[string]any{
				"quality":       "high",
				"output_format": "png",
				"size":          "1024x768",
			},
			Metadata: map[string]any{
				"image_count":       float64(1),
				"input_image_count": float64(1),
				"mask_used":         true,
			},
		},
	})

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	result := payload["result"].(map[string]any)
	images := result["images"].([]any)
	image := images[0].(map[string]any)
	require.Equal(t, "https://example.com/a.png", image["url"])
	require.Equal(t, "image/png", image["mime_type"])
	require.Equal(t, "png", image["format"])
	require.Equal(t, "a.png", image["filename"])
	require.Equal(t, float64(1234), image["size_bytes"])
	require.Equal(t, float64(1024), image["width"])
	require.Equal(t, float64(768), image["height"])
	output := result["output"].(map[string]any)
	require.Equal(t, "high", output["quality"])
	metadata := result["metadata"].(map[string]any)
	require.Equal(t, true, metadata["mask_used"])
}

func TestImageHandleSyncAssetMetadataKeepsExtendedResultFields(t *testing.T) {
	t.Parallel()

	metadata := imageHandleSyncAssetMetadata(imageHandleSyncResponse{
		TaskID:         "imgtask_123",
		ProviderTaskID: "imgtask_123",
		ClientTaskID:   "task_123",
		Result: &imageHandleSyncResult{
			Output: map[string]any{
				"quality": "high",
			},
			Metadata: map[string]any{
				"mask_used": true,
			},
		},
	}, imageHandleSyncImage{
		Format:        "png",
		RevisedPrompt: "prompt-a",
	})

	require.Equal(t, "image_handle_sync", metadata["source"])
	require.Equal(t, "png", metadata["format"])
	require.Equal(t, "prompt-a", metadata["revised_prompt"])
	require.Equal(t, map[string]any{"quality": "high"}, metadata["output"])
	require.Equal(t, map[string]any{"mask_used": true}, metadata["execution"])
}

func TestImageHandleSyncToOpenAIResponseRequiresBase64WhenRequested(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status:           "succeeded",
		ResultDataFormat: "url",
		Result: &imageHandleSyncResult{Images: []imageHandleSyncImage{
			{URL: "https://example.com/a.png"},
		}},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{ResponseFormat: "b64_json"})

	require.Empty(t, imageResp.Data)
}

func TestImageHandleSyncStatusErrorTreatsAcceptedAsTimeout(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	err := imageHandleSyncStatusError(ctx, http.StatusAccepted, []byte(`{"status":"processing"}`))

	require.NotNil(t, err)
	require.Equal(t, http.StatusGatewayTimeout, err.StatusCode)
	require.Equal(t, "image_handle_sync_timeout", string(err.GetErrorCode()))
}

func TestImageHandleSyncStatusErrorUsesImageHandleHTTPErrorBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"error":{"message":"Unknown parameter: 'seed'.","type":"invalid_request_error","code":"unknown_parameter","param":"seed"}}`)

	apiErr := imageHandleSyncStatusError(ctx, http.StatusBadRequest, body)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, types.ErrorCode("unknown_parameter"), apiErr.GetErrorCode())

	openAIError, ok := apiErr.RelayError.(types.OpenAIError)
	require.True(t, ok)
	require.Equal(t, "Unknown parameter: 'seed'.", openAIError.Message)
	require.Equal(t, "invalid_request_error", openAIError.Type)
	require.Equal(t, "seed", openAIError.Param)
	require.NotEmpty(t, apiErr.Metadata)
	require.Contains(t, string(apiErr.Metadata), "Unknown parameter")
}

func TestImageHandleSyncFailedErrorKeepsProviderErrorMetadata(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		TaskID:           "imgtask_123",
		ProviderTaskID:   "imgtask_123",
		ClientTaskID:     "task_123",
		Status:           "failed",
		ResultDataFormat: "url",
		Error: &imageHandleSyncError{
			Code:                 "upstream_error",
			Message:              "upstream returned an error",
			Retryable:            false,
			UpstreamStatus:       400,
			ProviderErrorCode:    "unsupported_size",
			ProviderErrorType:    "invalid_request_error",
			ProviderErrorMessage: "size is not supported by this channel",
			ProviderErrorParam:   "size",
			UpstreamError: map[string]any{
				"code": "unsupported_size",
			},
		},
		RawResponse: map[string]any{
			"error": map[string]any{
				"message": "size is not supported by this channel",
				"type":    "invalid_request_error",
				"param":   "size",
				"code":    "unsupported_size",
			},
		},
		RawResponseTruncated:     false,
		RawResponseOmittedFields: []string{"data[].b64_json"},
	}

	err := imageHandleSyncFailedError(response)

	require.NotNil(t, err)
	require.Equal(t, http.StatusBadGateway, err.StatusCode)
	require.Equal(t, "upstream_error", string(err.GetErrorCode()))
	openAIError := err.ToOpenAIError()
	require.Equal(t, "size is not supported by this channel", openAIError.Message)
	require.Equal(t, "invalid_request_error", openAIError.Type)
	require.Equal(t, "size", openAIError.Param)

	var metadata map[string]any
	require.NoError(t, common.Unmarshal(err.Metadata, &metadata))
	require.Equal(t, float64(400), metadata["upstream_status"])
	require.Equal(t, "unsupported_size", metadata["provider_error_code"])
	require.Equal(t, "invalid_request_error", metadata["provider_error_type"])
	require.Equal(t, "size is not supported by this channel", metadata["provider_error_message"])
	require.Equal(t, "size", metadata["provider_error_param"])
	require.Equal(t, "task_123", metadata["client_task_id"])
	require.Contains(t, metadata, "raw_response_summary")
}

func TestRecordImageHandleSyncErrorDetailStoresContextMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	recordImageHandleSyncErrorDetail(ctx, imageHandleSyncResponse{
		TaskID:           "imgtask_ctx",
		ClientTaskID:     "task_ctx",
		ResultDataFormat: "url",
		Error: &imageHandleSyncError{
			Code:                 "upstream_error",
			Message:              "upstream returned an error",
			UpstreamStatus:       400,
			ProviderErrorMessage: "bad size",
		},
	})

	value, ok := common.GetContextKey(ctx, constant.ContextKeyImageHandleSyncErrorDetail)
	require.True(t, ok)
	detail, ok := value.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "upstream_error", detail["code"])
	require.Equal(t, "bad size", detail["provider_error_message"])
	require.Equal(t, float64(400), detail["upstream_status"])
}

func TestRecordImageHandleSyncErrorDetailMasksNestedNetworkLocations(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	recordImageHandleSyncErrorDetail(ctx, imageHandleSyncResponse{
		TaskID:       "imgtask_masked",
		ClientTaskID: "task_masked",
		Error: &imageHandleSyncError{
			Code:                 "upstream_error",
			ProviderErrorMessage: `Post "http://192.0.2.236:28787/v1/images/edits": context canceled`,
			UpstreamError: map[string]any{
				"endpoint": "https://internal.example.com:9443/private/upstream",
			},
		},
		RawResponse: map[string]any{
			"debug_url": "http://198.51.100.9:8080/raw/response",
		},
	})

	value, ok := common.GetContextKey(ctx, constant.ContextKeyImageHandleSyncErrorDetail)
	require.True(t, ok)
	serialized, err := common.Marshal(value)
	require.NoError(t, err)
	result := string(serialized)
	for _, secret := range []string{
		"192.0.2.236",
		"198.51.100.9",
		"internal.example.com",
		"28787",
		"9443",
		"8080",
		"/v1/images/edits",
		"/private/upstream",
		"/raw/response",
	} {
		require.NotContains(t, result, secret)
	}
	require.Contains(t, result, "context canceled")
}

func TestImageHandleSyncToOpenAIResponseFallsBackToRawResponseUsage(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status: "succeeded",
		Result: &imageHandleSyncResult{
			Images: []imageHandleSyncImage{{URL: "https://example.com/a.png"}},
			RawResponse: map[string]any{
				"usage": map[string]any{
					"input_tokens":  19,
					"output_tokens": 781,
					"total_tokens":  800,
					"input_tokens_details": map[string]any{
						"text_tokens":  19,
						"image_tokens": 2,
					},
				},
			},
		},
		Usage: &imageHandleSyncUsage{TotalTokens: 0},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{})

	require.NotNil(t, imageResp.Usage)
	require.Equal(t, 19, imageResp.Usage.PromptTokens)
	require.Equal(t, 781, imageResp.Usage.CompletionTokens)
	require.Equal(t, 800, imageResp.Usage.TotalTokens)
	require.Equal(t, 19, imageResp.Usage.InputTokens)
	require.Equal(t, 781, imageResp.Usage.OutputTokens)
	require.Equal(t, 19, imageResp.Usage.PromptTokensDetails.TextTokens)
	require.Equal(t, 2, imageResp.Usage.PromptTokensDetails.ImageTokens)
}

func TestImageHandleSyncToOpenAIResponseMergesIncompleteTopLevelUsage(t *testing.T) {
	t.Parallel()

	response := imageHandleSyncResponse{
		Status: "succeeded",
		Result: &imageHandleSyncResult{
			Images: []imageHandleSyncImage{{URL: "https://example.com/a.png"}},
			RawResponse: map[string]any{
				"usage": map[string]any{
					"input_tokens":  19,
					"output_tokens": 781,
					"total_tokens":  800,
					"input_tokens_details": map[string]any{
						"cached_tokens": 2,
					},
				},
			},
		},
		Usage: &imageHandleSyncUsage{TotalTokens: 800},
	}

	imageResp := imageHandleSyncToOpenAIResponse(response, &relaycommon.RelayInfo{}, dto.ImageRequest{})

	require.NotNil(t, imageResp.Usage)
	require.Equal(t, 19, imageResp.Usage.PromptTokens)
	require.Equal(t, 781, imageResp.Usage.CompletionTokens)
	require.Equal(t, 800, imageResp.Usage.TotalTokens)
	require.Equal(t, 2, imageResp.Usage.PromptTokensDetails.CachedTokens)
}

func TestBuildImageHandleSyncPayloadUsesLeaseAndKeepsImageParams(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-payload")

	n := uint(2)
	request := dto.ImageRequest{
		Prompt:            "test prompt",
		N:                 &n,
		Size:              "1024x1024",
		Quality:           "auto",
		OutputFormat:      []byte(`"png"`),
		OutputCompression: []byte(`0`),
		Background:        []byte(`"transparent"`),
		ExtraFields:       []byte(`{"seed":123}`),
	}
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, request, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.NotContains(t, payload, "callback")
	require.NotContains(t, string(body), "api_key")
	require.Equal(t, "task_sync", payload["client_task_id"])
	require.Equal(t, "gpt-image-2", payload["model"])
	require.Equal(t, "url", payload["result_data_format"])

	executor := payload["executor"].(map[string]any)
	require.Equal(t, "provider_direct_lease", executor["type"])
	require.Equal(t, "lease_sync", executor["lease_id"])
	require.Equal(t, "http://new-api.internal/api/internal/image/credential-leases/lease_sync/resolve", executor["resolve_url"])
	require.Equal(t, "image_handle_1", executor["secret_id"])

	params := payload["parameters"].(map[string]any)
	require.Equal(t, "1024x1024", params["size"])
	require.Equal(t, "auto", params["quality"])
	require.Equal(t, float64(2), params["n"])
	require.Equal(t, "png", params["output_format"])
	require.Equal(t, float64(0), params["output_compression"])
	require.Equal(t, "transparent", params["background"])

	require.NotContains(t, payload, "provider_options")
}

func TestBuildImageHandleSyncPayloadKeepsProviderOptionsForNonGPTImage(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-payload-seed")

	request := dto.ImageRequest{
		Model:       "custom-image-model",
		Prompt:      "test prompt",
		ExtraFields: []byte(`{"seed":123}`),
	}
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "custom-image-model",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "custom-image-model",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, request, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	providerOptions := payload["provider_options"].(map[string]any)
	require.Equal(t, float64(123), providerOptions["seed"])
}

func TestBuildImageHandleSyncPayloadForEditURLInputs(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-edit-payload")

	request := dto.ImageRequest{
		Prompt:         "edit prompt",
		Image:          []byte(`"https://cdn.example.com/input.png"`),
		Size:           "1024x1024",
		Quality:        "auto",
		ResponseFormat: "url",
		OutputFormat:   []byte(`"png"`),
		Extra: map[string]json.RawMessage{
			"mask":           []byte(`"https://cdn.example.com/mask.png"`),
			"input_fidelity": []byte(`"high"`),
		},
	}
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, request, "task_edit_sync", "lease_edit_sync", "edit")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "edit", payload["operation"])
	require.Equal(t, "url", payload["result_data_format"])
	input := payload["input"].(map[string]any)
	require.Equal(t, "edit prompt", input["text"])
	require.Equal(t, []any{"https://cdn.example.com/input.png"}, input["images"])
	require.Equal(t, "https://cdn.example.com/mask.png", input["mask"])
	params := payload["parameters"].(map[string]any)
	require.Equal(t, "1024x1024", params["size"])
	require.Equal(t, "auto", params["quality"])
	require.Equal(t, "url", params["response_format"])
	require.Equal(t, "png", params["output_format"])
	require.Equal(t, "high", params["input_fidelity"])
}

func TestImageEditMultipartRequestKeepsImageParameters(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit prompt"))
	require.NoError(t, writer.WriteField("n", "2"))
	require.NoError(t, writer.WriteField("size", "2560x1440"))
	require.NoError(t, writer.WriteField("quality", "auto"))
	require.NoError(t, writer.WriteField("response_format", "url"))
	require.NoError(t, writer.WriteField("output_format", "png"))
	require.NoError(t, writer.WriteField("output_compression", "85"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	imagePart, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("fake-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, err := helper.GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesEdits)
	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", request.Model)
	require.Equal(t, "edit prompt", request.Prompt)
	require.Equal(t, uint(2), *request.N)
	require.Equal(t, "2560x1440", request.Size)
	require.Equal(t, "auto", request.Quality)
	require.Equal(t, "url", request.ResponseFormat)
	require.JSONEq(t, `"png"`, string(request.OutputFormat))
	require.JSONEq(t, `"85"`, string(request.OutputCompression))
	require.JSONEq(t, `"high"`, string(request.Extra["input_fidelity"]))
}

func TestImageHandleBase64UploadsFromRequestBuildsExplicitUploadItems(t *testing.T) {
	t.Parallel()

	request := dto.ImageRequest{
		Prompt: "edit",
		Image:  []byte(`["data:image/png;base64,AAAA","BBBB"]`),
		Extra: map[string]json.RawMessage{
			"mask": []byte(`{"b64_json":"CCCC","filename":"mask.png"}`),
		},
	}

	uploadReq, ok, err := imageHandleBase64UploadsFromRequest(request)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, uploadReq.Uploads, 3)
	require.Equal(t, "image", uploadReq.Uploads[0].Field)
	require.Equal(t, "data:image/png;base64,AAAA", uploadReq.Uploads[0].B64Json)
	require.Equal(t, "image", uploadReq.Uploads[1].Field)
	require.Equal(t, "BBBB", uploadReq.Uploads[1].B64Json)
	require.Equal(t, "mask", uploadReq.Uploads[2].Field)
	require.Equal(t, "mask.png", uploadReq.Uploads[2].Filename)
	require.Equal(t, "CCCC", uploadReq.Uploads[2].B64Json)
}

func TestApplyImageHandleUploadResponseToRequestMergesExistingURLInputs(t *testing.T) {
	t.Parallel()

	existingMask := "https://cdn.example.com/uploaded-mask.png"
	request := dto.ImageRequest{
		Prompt: "edit",
		Image:  []byte(`"https://cdn.example.com/original.png"`),
		Extra: map[string]json.RawMessage{
			"mask": []byte(`"data:image/png;base64,AAAA"`),
		},
	}
	normalized, err := applyImageHandleUploadResponseToRequest(request, imageHandleUploadResponse{
		Images: nil,
		Mask:   &existingMask,
	})
	require.NoError(t, err)

	images, mask, err := imageHandleSyncInputsFromRequest(normalized)
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example.com/original.png"}, images)
	require.NotNil(t, mask)
	require.Equal(t, existingMask, *mask)
}

func TestNormalizeImageHandleSyncEditRequestUploadsMultipartFiles(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()

	var receivedPath string
	var receivedAuth string
	var receivedContentType string
	var receivedFields []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")
		require.NoError(t, r.ParseMultipartForm(32<<20))
		for field := range r.MultipartForm.File {
			receivedFields = append(receivedFields, field)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["https://img.example.com/input.png"],"mask":"https://img.example.com/mask.png"}`))
	}))
	defer server.Close()

	image_handle_setting.GetImageHandleSetting().BaseURL = server.URL
	image_handle_setting.GetImageHandleSetting().APIKey = "provider-key"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	imagePart, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("fake-image"))
	require.NoError(t, err)
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("fake-mask"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	normalized, err := normalizeImageHandleSyncEditRequest(ctx, dto.ImageRequest{Prompt: "edit"})
	require.NoError(t, err)
	require.Equal(t, "/v1/image/uploads", receivedPath)
	require.Equal(t, "Bearer provider-key", receivedAuth)
	require.True(t, strings.HasPrefix(receivedContentType, "multipart/form-data"))
	require.ElementsMatch(t, []string{"image", "mask"}, receivedFields)

	images, mask, err := imageHandleSyncInputsFromRequest(normalized)
	require.NoError(t, err)
	require.Equal(t, []string{"https://img.example.com/input.png"}, images)
	require.NotNil(t, mask)
	require.Equal(t, "https://img.example.com/mask.png", *mask)
}

func TestNormalizeImageHandleSyncEditRequestUploadsBase64Inputs(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()

	var receivedPath string
	var uploadReq imageHandleBase64UploadRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &uploadReq))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":["https://img.example.com/input.png"],"mask":null}`))
	}))
	defer server.Close()

	image_handle_setting.GetImageHandleSetting().BaseURL = server.URL
	image_handle_setting.GetImageHandleSetting().APIKey = "provider-key"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	normalized, err := normalizeImageHandleSyncEditRequest(ctx, dto.ImageRequest{
		Prompt: "edit",
		Image:  []byte(`"data:image/png;base64,AAAA"`),
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/image/uploads/base64", receivedPath)
	require.Len(t, uploadReq.Uploads, 1)
	require.Equal(t, "image", uploadReq.Uploads[0].Field)
	require.Equal(t, "data:image/png;base64,AAAA", uploadReq.Uploads[0].B64Json)

	images, mask, err := imageHandleSyncInputsFromRequest(normalized)
	require.NoError(t, err)
	require.Equal(t, []string{"https://img.example.com/input.png"}, images)
	require.Nil(t, mask)
}

func TestBuildImageHandleSyncPayloadUsesNormalizedUploadedEditURLs(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	normalized, err := applyImageHandleUploadResponseToRequest(dto.ImageRequest{
		Prompt: "edit prompt",
		Image:  []byte(`"data:image/png;base64,AAAA"`),
	}, imageHandleUploadResponse{
		Images: []string{"https://img.example.com/input.png"},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-edit-uploaded-payload")

	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, normalized, "task_edit_sync", "lease_edit_sync", "edit")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	input := payload["input"].(map[string]any)
	require.Equal(t, []any{"https://img.example.com/input.png"}, input["images"])
	require.NotContains(t, input, "mask")
	require.NotContains(t, string(body), "data:image/png;base64")
}

func TestBuildImageHandleSyncPayloadUsesBase64ResultFormat(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-base64")

	request := dto.ImageRequest{
		Prompt:         "test prompt",
		ResponseFormat: " b64_json ",
	}
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, request, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "base64", payload["result_data_format"])
	params := imageHandleSyncPayloadParams(payload)
	require.Equal(t, "b64_json", params["response_format"])
}

func TestBuildImageHandleSyncPayloadKeepsResponseFormatForNonGPTImage(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	image_handle_setting.GetImageHandleSetting().InternalBaseURL = "http://new-api.internal"
	image_handle_setting.GetImageHandleSetting().InternalSecretID = "image_handle_1"

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-base64-non-gpt")

	request := dto.ImageRequest{
		Prompt:         "test prompt",
		ResponseFormat: "b64_json",
	}
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "dall-e-3",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "dall-e-3",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, request, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "base64", payload["result_data_format"])
	params := payload["parameters"].(map[string]any)
	require.Equal(t, "b64_json", params["response_format"])
}

func TestImageHandleSyncResultDataFormatUsesConfiguredDefault(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()

	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		SyncImageResultPolicy:  image_handle_setting.SyncImageResultFormatPolicyFollowRequest,
		SyncImageDefaultFormat: image_handle_setting.SyncImageDefaultResultFormatBase64,
	})

	require.Equal(t, imageHandleResultFormatBase64, imageHandleSyncResultDataFormat(dto.ImageRequest{}))
	require.Equal(t, imageHandleResultFormatURL, imageHandleSyncResultDataFormat(dto.ImageRequest{ResponseFormat: "url"}))
}

func TestBuildImageHandleSyncPayloadDoesNotInventDefaultResponseFormat(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		InternalBaseURL:        "http://new-api.internal",
		InternalSecretID:       "image_handle_1",
		SyncImageResultPolicy:  image_handle_setting.SyncImageResultFormatPolicyFollowRequest,
		SyncImageDefaultFormat: image_handle_setting.SyncImageDefaultResultFormatBase64,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-default-base64")
	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, dto.ImageRequest{Prompt: "test"}, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "base64", payload["result_data_format"])
	require.NotContains(t, imageHandleSyncPayloadParams(payload), "response_format")
}

func TestImageHandleSyncResultDataFormatForceModesOverrideRequest(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()

	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		SyncImageResultPolicy:  image_handle_setting.SyncImageResultFormatPolicyForceURL,
		SyncImageDefaultFormat: image_handle_setting.SyncImageDefaultResultFormatBase64,
	})
	require.Equal(t, imageHandleResultFormatURL, imageHandleSyncResultDataFormat(dto.ImageRequest{ResponseFormat: "b64_json"}))

	image_handle_setting.GetImageHandleSetting().SyncImageResultPolicy = image_handle_setting.SyncImageResultFormatPolicyForceBase64
	require.Equal(t, imageHandleResultFormatBase64, imageHandleSyncResultDataFormat(dto.ImageRequest{ResponseFormat: "url"}))
}

func TestBuildImageHandleSyncPayloadForceURLOverridesRequestResponseFormat(t *testing.T) {
	originalSetting := *image_handle_setting.GetImageHandleSetting()
	defer func() {
		*image_handle_setting.GetImageHandleSetting() = originalSetting
	}()
	*image_handle_setting.GetImageHandleSetting() = image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		InternalBaseURL:        "http://new-api.internal",
		InternalSecretID:       "image_handle_1",
		SyncImageResultPolicy:  image_handle_setting.SyncImageResultFormatPolicyForceURL,
		SyncImageDefaultFormat: image_handle_setting.SyncImageDefaultResultFormatURL,
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "req-sync-force-url")

	info := &relaycommon.RelayInfo{
		UserId:          44,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         123,
			UpstreamModelName: "gpt-image-2",
		},
	}

	body, err := buildImageHandleSyncPayload(ctx, info, dto.ImageRequest{Prompt: "test", ResponseFormat: "b64_json"}, "task_sync", "lease_sync", "generation")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "url", payload["result_data_format"])
	params := imageHandleSyncPayloadParams(payload)
	require.Equal(t, "b64_json", params["response_format"])
}

func imageHandleSyncPayloadParams(payload map[string]any) map[string]any {
	params, ok := payload["parameters"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return params
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
