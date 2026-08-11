package xai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const (
	xaiOfficialMaxImageEditSources = 3
	xai2KENMaxImageEditSources     = 5
)

func applyXAIImageEditSources(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest, target *ImageRequest) error {
	sources, plural, err := xaiImageEditSources(c, request, info.ChannelOtherSettings.IsXAI2KEN())
	if err != nil {
		return err
	}
	limit := xaiImageEditSourceLimit(request.Model, info.ChannelOtherSettings.IsXAI2KEN())
	if len(sources) > limit {
		return fmt.Errorf("xAI image edit model %s supports at most %d source images", request.Model, limit)
	}
	if plural || len(sources) > 1 {
		target.Images = sources
		return nil
	}
	target.Image = &sources[0]
	return nil
}

func xaiImageEditSourceLimit(model string, is2KEN bool) int {
	if !is2KEN || strings.EqualFold(strings.TrimSpace(model), "grok-imagine-image-quality") {
		return xaiOfficialMaxImageEditSources
	}
	return xai2KENMaxImageEditSources
}

func xaiImageEditSources(c *gin.Context, request dto.ImageRequest, is2KEN bool) ([]ImageSource, bool, error) {
	if c != nil && c.Request != nil && strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "multipart/form-data") {
		return xaiMultipartImageEditSources(c, is2KEN)
	}

	imagesRaw, hasImages := request.Extra["images"]
	hasImage := len(request.Image) > 0 && string(request.Image) != "null"
	if hasImage && hasImages {
		return nil, false, fmt.Errorf("xAI image edit accepts either image or images, not both")
	}
	if !hasImage && !hasImages {
		return nil, false, fmt.Errorf("xAI image edit requires image or images")
	}
	raw := request.Image
	plural := false
	if hasImages {
		raw = imagesRaw
		plural = true
	}
	sources, rawPlural, err := parseXAIImageEditSources(raw, is2KEN)
	if err != nil {
		return nil, false, err
	}
	if len(sources) == 0 {
		return nil, false, fmt.Errorf("xAI image edit requires at least one source image")
	}
	return sources, plural || rawPlural, nil
}

func parseXAIImageEditSources(raw json.RawMessage, is2KEN bool) ([]ImageSource, bool, error) {
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, false, fmt.Errorf("invalid xAI image edit source: %w", err)
	}
	values, plural := value.([]any)
	if !plural {
		values = []any{value}
	}
	sources := make([]ImageSource, 0, len(values))
	for index, item := range values {
		source, err := parseXAIImageEditSource(item, is2KEN)
		if err != nil {
			return nil, plural, fmt.Errorf("invalid xAI image edit source %d: %w", index, err)
		}
		sources = append(sources, source)
	}
	return sources, plural, nil
}

func parseXAIImageEditSource(value any, is2KEN bool) (ImageSource, error) {
	switch typed := value.(type) {
	case string:
		return xaiImageURLSource(typed)
	case map[string]any:
		urlValue := firstXAIImageSourceString(typed, "url", "image_url")
		fileID := firstXAIImageSourceString(typed, "file_id")
		if urlValue != "" && fileID != "" {
			return ImageSource{}, fmt.Errorf("url and file_id are mutually exclusive")
		}
		if urlValue != "" {
			return xaiImageURLSource(urlValue)
		}
		if fileID != "" {
			if is2KEN {
				return ImageSource{}, fmt.Errorf("2KEN image edits do not support file_id")
			}
			return ImageSource{FileID: fileID}, nil
		}
		return ImageSource{}, fmt.Errorf("source object must contain url or file_id")
	default:
		return ImageSource{}, fmt.Errorf("source must be a URL string or object")
	}
}

func firstXAIImageSourceString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func xaiImageURLSource(value string) (ImageSource, error) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") {
		marker := strings.Index(lower, ";base64,")
		if marker < 0 || marker+len(";base64,") >= len(value) {
			return ImageSource{}, fmt.Errorf("image data URI must contain base64 image data")
		}
		return ImageSource{URL: value}, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ImageSource{}, fmt.Errorf("image URL must use http, https, or a base64 data URI")
	}
	return ImageSource{URL: value}, nil
}

func xaiMultipartImageEditSources(c *gin.Context, is2KEN bool) ([]ImageSource, bool, error) {
	form := c.Request.MultipartForm
	if form == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, false, fmt.Errorf("failed to parse xAI image edit multipart form: %w", err)
		}
		form = c.Request.MultipartForm
	}
	if form == nil {
		return nil, false, fmt.Errorf("xAI image edit requires multipart form data")
	}
	if len(form.Value["mask"]) > 0 || len(form.File["mask"]) > 0 {
		return nil, false, fmt.Errorf("xAI image edits do not support mask")
	}

	fieldNames := xaiMultipartImageFieldNames(form)
	sources := make([]ImageSource, 0)
	for _, fieldName := range fieldNames {
		for _, value := range form.Value[fieldName] {
			source, err := parseXAIImageEditSource(value, is2KEN)
			if err != nil {
				return nil, false, err
			}
			sources = append(sources, source)
		}
		for _, fileHeader := range form.File[fieldName] {
			source, err := xaiMultipartFileSource(fileHeader)
			if err != nil {
				return nil, false, err
			}
			sources = append(sources, source)
		}
	}
	if len(sources) == 0 {
		return nil, false, fmt.Errorf("xAI image edit requires image files or image URLs")
	}
	return sources, len(sources) > 1 || len(fieldNames) > 1 || (len(fieldNames) == 1 && fieldNames[0] != "image"), nil
}

func xaiMultipartImageFieldNames(form *multipart.Form) []string {
	known := map[string]bool{"image": true, "image[]": true, "images": true}
	ordered := make([]string, 0, 3)
	for _, key := range []string{"image", "image[]", "images"} {
		if len(form.Value[key]) > 0 || len(form.File[key]) > 0 {
			ordered = append(ordered, key)
		}
	}
	indexed := make([]string, 0)
	for key := range form.Value {
		if !known[key] && strings.HasPrefix(key, "image[") {
			indexed = append(indexed, key)
		}
	}
	for key := range form.File {
		if !known[key] && strings.HasPrefix(key, "image[") {
			indexed = append(indexed, key)
		}
	}
	sort.Strings(indexed)
	for _, key := range indexed {
		if len(ordered) == 0 || ordered[len(ordered)-1] != key {
			ordered = append(ordered, key)
		}
	}
	return ordered
}

func xaiMultipartFileSource(fileHeader *multipart.FileHeader) (ImageSource, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return ImageSource{}, fmt.Errorf("open xAI image edit file %s: %w", fileHeader.Filename, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return ImageSource{}, fmt.Errorf("read xAI image edit file %s: %w", fileHeader.Filename, err)
	}
	if len(data) == 0 {
		return ImageSource{}, fmt.Errorf("xAI image edit file %s is empty", fileHeader.Filename)
	}
	mimeType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = strings.ToLower(http.DetectContentType(data))
	}
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return ImageSource{}, fmt.Errorf("xAI image edit file %s must be PNG, JPEG, or WebP", fileHeader.Filename)
	}
	return ImageSource{URL: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)}, nil
}
