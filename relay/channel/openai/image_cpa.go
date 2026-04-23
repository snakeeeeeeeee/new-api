package openai

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func isCPAImageResponseAdapter(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(info.ChannelOtherSettings.ImageResponseAdapter), "cpa")
}

func cpaImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	if usage, ok := tryHandleStandardOpenAIImageResponse(c, resp, responseBody); ok {
		return usage, nil
	}

	imageResponse, usage, err := normalizeCPAImageResponse(responseBody, info)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	jsonResponse, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return usage, nil
}

func tryHandleStandardOpenAIImageResponse(c *gin.Context, resp *http.Response, responseBody []byte) (*dto.Usage, bool) {
	if len(responseBody) == 0 {
		return nil, false
	}
	var imageResponse dto.ImageResponse
	if err := common.Unmarshal(responseBody, &imageResponse); err != nil {
		return nil, false
	}
	if len(imageResponse.Data) == 0 {
		return nil, false
	}
	hasImagePayload := false
	for _, item := range imageResponse.Data {
		if strings.TrimSpace(item.Url) != "" || strings.TrimSpace(item.B64Json) != "" {
			hasImagePayload = true
			break
		}
	}
	if !hasImagePayload {
		return nil, false
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	if imageResponse.Usage == nil {
		return &dto.Usage{}, true
	}
	return imageResponse.Usage, true
}

func normalizeCPAImageResponse(responseBody []byte, info *relaycommon.RelayInfo) (*dto.ImageResponse, *dto.Usage, error) {
	usage := extractImageUsage(responseBody)
	imageResponse := &dto.ImageResponse{
		Created: extractImageCreated(responseBody, info),
		Data:    extractCPAImageData(responseBody),
	}
	if len(imageResponse.Data) == 0 {
		return nil, nil, errors.New("unsupported cpa image response format")
	}

	if usage != nil {
		imageResponse.Usage = usage
	}
	imageResponse.Background = extractStringByPaths(responseBody, "background", "result.background", "output.background")
	imageResponse.OutputFormat = extractStringByPaths(responseBody, "output_format", "result.output_format", "output.output_format", "format", "result.format", "output.format")
	imageResponse.Quality = extractStringByPaths(responseBody, "quality", "result.quality", "output.quality")
	imageResponse.Size = extractStringByPaths(responseBody, "size", "result.size", "output.size")

	return imageResponse, usageOrEmpty(usage), nil
}

func extractImageCreated(responseBody []byte, info *relaycommon.RelayInfo) int64 {
	createdPaths := []string{
		"created",
		"result.created",
		"output.created",
		"timestamp",
		"result.timestamp",
		"output.timestamp",
	}
	for _, path := range createdPaths {
		result := gjson.GetBytes(responseBody, path)
		if result.Exists() && result.Type == gjson.Number {
			return result.Int()
		}
	}
	if info != nil && !info.StartTime.IsZero() {
		return info.StartTime.Unix()
	}
	return common.GetTimestamp()
}

func extractImageUsage(responseBody []byte) *dto.Usage {
	usagePaths := []string{
		"usage",
		"result.usage",
		"output.usage",
		"data.usage",
	}
	for _, path := range usagePaths {
		result := gjson.GetBytes(responseBody, path)
		if !result.Exists() || !result.IsObject() {
			continue
		}
		var usage dto.Usage
		if err := common.Unmarshal([]byte(result.Raw), &usage); err != nil {
			continue
		}
		return &usage
	}
	return nil
}

func extractCPAImageData(responseBody []byte) []dto.ImageData {
	arrayPaths := []string{
		"data",
		"images",
		"result.images",
		"result.data",
		"result.results",
		"output.images",
		"output.data",
		"output.results",
	}
	for _, path := range arrayPaths {
		result := gjson.GetBytes(responseBody, path)
		if !result.Exists() {
			continue
		}
		data := parseCPAImageArray(result)
		if len(data) > 0 {
			return data
		}
	}
	return nil
}

func parseCPAImageArray(result gjson.Result) []dto.ImageData {
	if result.IsArray() {
		items := result.Array()
		parsed := make([]dto.ImageData, 0, len(items))
		for _, item := range items {
			if imageData, ok := parseCPAImageItem(item); ok {
				parsed = append(parsed, imageData)
			}
		}
		return parsed
	}
	if imageData, ok := parseCPAImageItem(result); ok {
		return []dto.ImageData{imageData}
	}
	return nil
}

func parseCPAImageItem(item gjson.Result) (dto.ImageData, bool) {
	if item.Type == gjson.String {
		value := strings.TrimSpace(item.String())
		if value == "" {
			return dto.ImageData{}, false
		}
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			return dto.ImageData{Url: value}, true
		}
		return dto.ImageData{B64Json: value}, true
	}

	if !item.IsObject() {
		return dto.ImageData{}, false
	}

	imageData := dto.ImageData{
		Url: extractStringFromResult(item,
			"url",
			"image_url",
			"imageUrl",
			"oss_url",
			"cdn_url",
			"output_url",
		),
		B64Json: extractStringFromResult(item,
			"b64_json",
			"base64",
			"b64",
			"binary_data_base64",
			"image_base64",
			"bytesBase64Encoded",
		),
		RevisedPrompt: extractStringFromResult(item,
			"revised_prompt",
			"revisedPrompt",
			"final_prompt",
			"prompt",
		),
	}

	if strings.TrimSpace(imageData.Url) == "" && strings.TrimSpace(imageData.B64Json) == "" {
		return dto.ImageData{}, false
	}
	return imageData, true
}

func extractStringByPaths(responseBody []byte, paths ...string) string {
	for _, path := range paths {
		result := gjson.GetBytes(responseBody, path)
		if result.Exists() && result.Type == gjson.String {
			value := strings.TrimSpace(result.String())
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func extractStringFromResult(result gjson.Result, paths ...string) string {
	for _, path := range paths {
		value := result.Get(path)
		if value.Exists() && value.Type == gjson.String {
			text := strings.TrimSpace(value.String())
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func usageOrEmpty(usage *dto.Usage) *dto.Usage {
	if usage == nil {
		return &dto.Usage{}
	}
	return usage
}
