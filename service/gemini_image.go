package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	GeminiImageModelFlash = "gemini-3.1-flash-image"
	GeminiImageModelPro   = "gemini-3-pro-image-count"
)

var geminiImageSizes = map[string]struct{}{
	"1024x1024": {},
	"1024x1536": {},
	"1536x1024": {},
	"2048x2048": {},
	"2048x3072": {},
	"3072x2048": {},
}

var geminiImageAspectRatios = map[string]struct{}{
	"1:1":  {},
	"2:3":  {},
	"3:2":  {},
	"3:4":  {},
	"4:3":  {},
	"4:5":  {},
	"5:4":  {},
	"9:16": {},
	"16:9": {},
	"21:9": {},
}

var geminiImageImageSizes = map[string]struct{}{
	"1K": {},
	"2K": {},
	"4K": {},
}

type GeminiImageValidationInput struct {
	Count                   *int
	HasMask                 bool
	Quality                 string
	Size                    string
	OutputFormat            string
	ResponseFormat          string
	HasOutputCompression    bool
	HasBackground           bool
	HasInputFidelity        bool
	HasResolution           bool
	HasStyle                bool
	HasModeration           bool
	HasPartialImages        bool
	HasStream               bool
	HasWatermark            bool
	HasWatermarkEnabled     bool
	HasLegacyExtraFields    bool
	UnsupportedPublicFields []string
	ProviderOptions         map[string]any
}

type GeminiImageValidationError struct {
	Code    string
	Param   string
	Message string
}

func (e *GeminiImageValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func IsGeminiImageModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case GeminiImageModelFlash, GeminiImageModelPro:
		return true
	default:
		return false
	}
}

func ValidateAndNormalizeGeminiImageRequest(input GeminiImageValidationInput) (map[string]any, *GeminiImageValidationError) {
	if input.Count != nil && *input.Count != 1 {
		return nil, geminiImageValidationError(
			"unsupported_image_count",
			"n",
			"Gemini image requests support exactly one output image",
		)
	}
	if input.HasMask {
		return nil, geminiImageValidationError(
			"unsupported_mask",
			"mask",
			"Gemini image editing does not support masks",
		)
	}
	if quality := strings.ToLower(strings.TrimSpace(input.Quality)); quality != "" && quality != "auto" {
		return nil, geminiImageValidationError(
			"unsupported_quality",
			"quality",
			"Gemini image requests only support quality=auto",
		)
	}
	if format := strings.ToLower(strings.TrimSpace(input.OutputFormat)); format != "" && format != "png" {
		return nil, geminiImageValidationError(
			"unsupported_output_format",
			"output_format",
			"Gemini image requests only support PNG output",
		)
	}
	if responseFormat := strings.ToLower(strings.TrimSpace(input.ResponseFormat)); responseFormat != "" &&
		responseFormat != "url" && responseFormat != "b64_json" {
		return nil, geminiImageValidationError(
			"unsupported_response_format",
			"response_format",
			"Gemini image requests only support response_format=url or b64_json",
		)
	}

	unsupported := []struct {
		present bool
		param   string
	}{
		{input.HasOutputCompression, "output_compression"},
		{input.HasBackground, "background"},
		{input.HasInputFidelity, "input_fidelity"},
		{input.HasResolution, "resolution"},
		{input.HasStyle, "style"},
		{input.HasModeration, "moderation"},
		{input.HasPartialImages, "partial_images"},
		{input.HasStream, "stream"},
		{input.HasWatermark, "watermark"},
		{input.HasWatermarkEnabled, "watermark_enabled"},
		{input.HasLegacyExtraFields, "extra_fields"},
	}
	for _, field := range unsupported {
		if field.present {
			return nil, geminiImageValidationError(
				"unsupported_parameter",
				field.param,
				fmt.Sprintf("Gemini image requests do not support %s", field.param),
			)
		}
	}
	for _, param := range input.UnsupportedPublicFields {
		if strings.TrimSpace(param) != "" {
			return nil, geminiImageValidationError(
				"unsupported_parameter",
				param,
				fmt.Sprintf("Gemini image requests do not support %s", param),
			)
		}
	}

	size := strings.TrimSpace(input.Size)
	if size != "" {
		if _, ok := geminiImageSizes[size]; !ok {
			return nil, geminiImageValidationError(
				"unsupported_image_size",
				"size",
				"Gemini image size is unsupported",
			)
		}
	}

	normalized, validationErr := normalizeGeminiProviderOptions(input.ProviderOptions)
	if validationErr != nil {
		return nil, validationErr
	}
	if size != "" && geminiProviderOptionsHasImageConfig(normalized) {
		return nil, geminiImageValidationError(
			"duplicate_parameter",
			"size",
			"size duplicates provider_options.google.generationConfig.imageConfig",
		)
	}
	return normalized, nil
}

func geminiImageValidationError(code string, param string, message string) *GeminiImageValidationError {
	return &GeminiImageValidationError{Code: code, Param: param, Message: message}
}

func normalizeGeminiProviderOptions(value map[string]any) (map[string]any, *GeminiImageValidationError) {
	if len(value) == 0 {
		return nil, nil
	}
	top, validationErr := normalizeGeminiAliasedObject(
		value,
		nil,
		map[string]struct{}{"google": {}},
		"provider_options",
	)
	if validationErr != nil {
		return nil, validationErr
	}
	googleSource, validationErr := geminiOptionalObject(top["google"], "provider_options.google")
	if validationErr != nil {
		return nil, validationErr
	}
	if googleSource == nil {
		googleSource = map[string]any{}
	}
	google, validationErr := normalizeGeminiAliasedObject(
		googleSource,
		map[string]string{
			"generation_config": "generationConfig",
			"safety_settings":   "safetySettings",
		},
		map[string]struct{}{
			"generationConfig": {},
			"safetySettings":   {},
		},
		"provider_options.google",
	)
	if validationErr != nil {
		return nil, validationErr
	}

	generationConfig, validationErr := normalizeGeminiGenerationConfig(google["generationConfig"])
	if validationErr != nil {
		return nil, validationErr
	}
	safetySettings, validationErr := normalizeGeminiSafetySettings(google["safetySettings"])
	if validationErr != nil {
		return nil, validationErr
	}

	normalizedGoogle := map[string]any{}
	if len(generationConfig) > 0 {
		normalizedGoogle["generationConfig"] = generationConfig
	}
	if safetySettings != nil {
		normalizedGoogle["safetySettings"] = safetySettings
	}
	return map[string]any{"google": normalizedGoogle}, nil
}

func normalizeGeminiGenerationConfig(value any) (map[string]any, *GeminiImageValidationError) {
	source, validationErr := geminiOptionalObject(value, "provider_options.google.generationConfig")
	if validationErr != nil {
		return nil, validationErr
	}
	if source == nil {
		return nil, nil
	}
	normalized, validationErr := normalizeGeminiAliasedObject(
		source,
		map[string]string{
			"top_p":           "topP",
			"top_k":           "topK",
			"image_config":    "imageConfig",
			"thinking_config": "thinkingConfig",
		},
		map[string]struct{}{
			"temperature":    {},
			"topP":           {},
			"topK":           {},
			"seed":           {},
			"imageConfig":    {},
			"thinkingConfig": {},
		},
		"provider_options.google.generationConfig",
	)
	if validationErr != nil {
		return nil, validationErr
	}
	for _, key := range []string{"temperature", "topP"} {
		if item, exists := normalized[key]; exists {
			number, ok := geminiFiniteNumber(item)
			if !ok {
				return nil, geminiImageValidationError(
					"invalid_provider_options",
					"provider_options.google.generationConfig."+key,
					"provider_options.google.generationConfig."+key+" must be a finite number",
				)
			}
			normalized[key] = number
		}
	}
	for _, key := range []string{"topK", "seed"} {
		if item, exists := normalized[key]; exists {
			number, ok := geminiInteger(item)
			if !ok {
				return nil, geminiImageValidationError(
					"invalid_provider_options",
					"provider_options.google.generationConfig."+key,
					"provider_options.google.generationConfig."+key+" must be an integer",
				)
			}
			normalized[key] = number
		}
	}

	imageConfig, validationErr := normalizeGeminiImageConfig(normalized["imageConfig"])
	if validationErr != nil {
		return nil, validationErr
	}
	if imageConfig != nil {
		normalized["imageConfig"] = imageConfig
	} else {
		delete(normalized, "imageConfig")
	}
	thinkingConfig, validationErr := normalizeGeminiThinkingConfig(normalized["thinkingConfig"])
	if validationErr != nil {
		return nil, validationErr
	}
	if thinkingConfig != nil {
		normalized["thinkingConfig"] = thinkingConfig
	} else {
		delete(normalized, "thinkingConfig")
	}
	return normalized, nil
}

func normalizeGeminiImageConfig(value any) (map[string]any, *GeminiImageValidationError) {
	const path = "provider_options.google.generationConfig.imageConfig"
	source, validationErr := geminiOptionalObject(value, path)
	if validationErr != nil || source == nil {
		return source, validationErr
	}
	normalized, validationErr := normalizeGeminiAliasedObject(
		source,
		map[string]string{
			"aspect_ratio": "aspectRatio",
			"image_size":   "imageSize",
		},
		map[string]struct{}{
			"aspectRatio": {},
			"imageSize":   {},
		},
		path,
	)
	if validationErr != nil {
		return nil, validationErr
	}
	if item, exists := normalized["aspectRatio"]; exists {
		aspectRatio, ok := geminiNonEmptyString(item)
		if !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".aspectRatio", path+".aspectRatio must be a string")
		}
		if _, ok := geminiImageAspectRatios[aspectRatio]; !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".aspectRatio", path+".aspectRatio is unsupported")
		}
		normalized["aspectRatio"] = aspectRatio
	}
	if item, exists := normalized["imageSize"]; exists {
		imageSize, ok := geminiNonEmptyString(item)
		imageSize = strings.ToUpper(imageSize)
		if !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".imageSize", path+".imageSize must be a string")
		}
		if _, ok := geminiImageImageSizes[imageSize]; !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".imageSize", path+".imageSize is unsupported")
		}
		normalized["imageSize"] = imageSize
	}
	return normalized, nil
}

func normalizeGeminiThinkingConfig(value any) (map[string]any, *GeminiImageValidationError) {
	const path = "provider_options.google.generationConfig.thinkingConfig"
	source, validationErr := geminiOptionalObject(value, path)
	if validationErr != nil || source == nil {
		return source, validationErr
	}
	normalized, validationErr := normalizeGeminiAliasedObject(
		source,
		map[string]string{
			"thinking_budget":  "thinkingBudget",
			"thinking_level":   "thinkingLevel",
			"include_thoughts": "includeThoughts",
		},
		map[string]struct{}{
			"thinkingBudget":  {},
			"thinkingLevel":   {},
			"includeThoughts": {},
		},
		path,
	)
	if validationErr != nil {
		return nil, validationErr
	}
	if item, exists := normalized["thinkingBudget"]; exists {
		number, ok := geminiInteger(item)
		if !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".thinkingBudget", path+".thinkingBudget must be an integer")
		}
		normalized["thinkingBudget"] = number
	}
	if item, exists := normalized["thinkingLevel"]; exists {
		level, ok := geminiNonEmptyString(item)
		if !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".thinkingLevel", path+".thinkingLevel must be a string")
		}
		normalized["thinkingLevel"] = level
	}
	if item, exists := normalized["includeThoughts"]; exists {
		includeThoughts, ok := item.(bool)
		if !ok {
			return nil, geminiImageValidationError("invalid_provider_options", path+".includeThoughts", path+".includeThoughts must be a boolean")
		}
		normalized["includeThoughts"] = includeThoughts
	}
	return normalized, nil
}

func normalizeGeminiSafetySettings(value any) ([]any, *GeminiImageValidationError) {
	const path = "provider_options.google.safetySettings"
	if value == nil {
		return nil, nil
	}
	source, ok := value.([]any)
	if !ok {
		return nil, geminiImageValidationError("invalid_provider_options", path, path+" must be an array")
	}
	normalized := make([]any, 0, len(source))
	for index, item := range source {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		object, validationErr := geminiOptionalObject(item, itemPath)
		if validationErr != nil {
			return nil, validationErr
		}
		entry, validationErr := normalizeGeminiAliasedObject(
			object,
			nil,
			map[string]struct{}{"category": {}, "threshold": {}},
			itemPath,
		)
		if validationErr != nil {
			return nil, validationErr
		}
		category, categoryOK := geminiNonEmptyString(entry["category"])
		threshold, thresholdOK := geminiNonEmptyString(entry["threshold"])
		if !categoryOK || !thresholdOK {
			return nil, geminiImageValidationError(
				"invalid_provider_options",
				itemPath,
				"Gemini safety settings require category and threshold",
			)
		}
		normalized = append(normalized, map[string]any{"category": category, "threshold": threshold})
	}
	return normalized, nil
}

func normalizeGeminiAliasedObject(
	source map[string]any,
	aliases map[string]string,
	allowed map[string]struct{},
	path string,
) (map[string]any, *GeminiImageValidationError) {
	normalized := make(map[string]any, len(source))
	for rawKey, value := range source {
		key := rawKey
		if alias, ok := aliases[rawKey]; ok {
			key = alias
		}
		if _, ok := allowed[key]; !ok {
			param := path + "." + rawKey
			return nil, geminiImageValidationError(
				"invalid_provider_options",
				param,
				param+" is not supported",
			)
		}
		if _, exists := normalized[key]; exists {
			param := path + "." + rawKey
			return nil, geminiImageValidationError(
				"duplicate_parameter",
				param,
				param+" duplicates "+path+"."+key,
			)
		}
		normalized[key] = value
	}
	return normalized, nil
}

func geminiOptionalObject(value any, param string) (map[string]any, *GeminiImageValidationError) {
	if value == nil {
		return nil, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, geminiImageValidationError(
			"invalid_provider_options",
			param,
			param+" must be an object",
		)
	}
	return object, nil
}

func geminiProviderOptionsHasImageConfig(options map[string]any) bool {
	google, ok := options["google"].(map[string]any)
	if !ok {
		return false
	}
	generationConfig, ok := google["generationConfig"].(map[string]any)
	if !ok {
		return false
	}
	imageConfig, ok := generationConfig["imageConfig"].(map[string]any)
	return ok && len(imageConfig) > 0
}

func geminiFiniteNumber(value any) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int8:
		number = float64(typed)
	case int16:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case uint:
		number = float64(typed)
	case uint8:
		number = float64(typed)
	case uint16:
		number = float64(typed)
	case uint32:
		number = float64(typed)
	case uint64:
		number = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

func geminiInteger(value any) (int64, bool) {
	number, ok := geminiFiniteNumber(value)
	if !ok || math.Trunc(number) != number || number < math.MinInt64 || number > math.MaxInt64 {
		return 0, false
	}
	return int64(number), true
}

func geminiNonEmptyString(value any) (string, bool) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	return text, ok && text != ""
}
