package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndNormalizeGeminiImageRequestNormalizesOfficialOptions(t *testing.T) {
	count := 1
	normalized, validationErr := ValidateAndNormalizeGeminiImageRequest(GeminiImageValidationInput{
		Count:        &count,
		Quality:      "auto",
		Size:         "",
		OutputFormat: "png",
		ProviderOptions: map[string]any{
			"google": map[string]any{
				"generation_config": map[string]any{
					"temperature": 0.7,
					"top_p":       0.9,
					"top_k":       float64(32),
					"seed":        float64(7),
					"image_config": map[string]any{
						"aspect_ratio": "16:9",
						"image_size":   "2k",
					},
					"thinking_config": map[string]any{
						"thinking_budget":  float64(256),
						"thinking_level":   "HIGH",
						"include_thoughts": true,
					},
				},
				"safety_settings": []any{
					map[string]any{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_ONLY_HIGH"},
				},
			},
		},
	})

	require.Nil(t, validationErr)
	google := normalized["google"].(map[string]any)
	generationConfig := google["generationConfig"].(map[string]any)
	assert.Equal(t, 0.9, generationConfig["topP"])
	assert.Equal(t, int64(32), generationConfig["topK"])
	assert.Equal(t, int64(7), generationConfig["seed"])
	imageConfig := generationConfig["imageConfig"].(map[string]any)
	assert.Equal(t, "16:9", imageConfig["aspectRatio"])
	assert.Equal(t, "2K", imageConfig["imageSize"])
	thinkingConfig := generationConfig["thinkingConfig"].(map[string]any)
	assert.Equal(t, int64(256), thinkingConfig["thinkingBudget"])
	assert.Equal(t, true, thinkingConfig["includeThoughts"])
	safety := google["safetySettings"].([]any)
	require.Len(t, safety, 1)
	assert.Equal(t, "BLOCK_ONLY_HIGH", safety[0].(map[string]any)["threshold"])
}

func TestValidateAndNormalizeGeminiImageRequestRejectsPublicConflicts(t *testing.T) {
	two := 2
	tests := []struct {
		name  string
		input GeminiImageValidationInput
		code  string
		param string
	}{
		{
			name:  "multiple images",
			input: GeminiImageValidationInput{Count: &two},
			code:  "unsupported_image_count",
			param: "n",
		},
		{
			name:  "mask",
			input: GeminiImageValidationInput{HasMask: true},
			code:  "unsupported_mask",
			param: "mask",
		},
		{
			name:  "quality",
			input: GeminiImageValidationInput{Quality: "high"},
			code:  "unsupported_quality",
			param: "quality",
		},
		{
			name:  "format",
			input: GeminiImageValidationInput{OutputFormat: "webp"},
			code:  "unsupported_output_format",
			param: "output_format",
		},
		{
			name:  "compression",
			input: GeminiImageValidationInput{HasOutputCompression: true},
			code:  "unsupported_parameter",
			param: "output_compression",
		},
		{
			name:  "size",
			input: GeminiImageValidationInput{Size: "1792x1024"},
			code:  "unsupported_image_size",
			param: "size",
		},
		{
			name: "size and image config",
			input: GeminiImageValidationInput{
				Size: "1024x1024",
				ProviderOptions: map[string]any{
					"google": map[string]any{
						"generationConfig": map[string]any{
							"imageConfig": map[string]any{"aspectRatio": "1:1"},
						},
					},
				},
			},
			code:  "duplicate_parameter",
			param: "size",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, validationErr := ValidateAndNormalizeGeminiImageRequest(test.input)
			require.NotNil(t, validationErr)
			assert.Equal(t, test.code, validationErr.Code)
			assert.Equal(t, test.param, validationErr.Param)
		})
	}
}

func TestValidateAndNormalizeGeminiImageRequestRejectsUnsafeAndDuplicateOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		code    string
	}{
		{
			name: "unsafe override",
			options: map[string]any{
				"google": map[string]any{"tools": []any{}},
			},
			code: "invalid_provider_options",
		},
		{
			name: "duplicate top p alias",
			options: map[string]any{
				"google": map[string]any{
					"generationConfig": map[string]any{"topP": 0.5, "top_p": 0.6},
				},
			},
			code: "duplicate_parameter",
		},
		{
			name: "non google namespace",
			options: map[string]any{
				"xai": map[string]any{"seed": 1},
			},
			code: "invalid_provider_options",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, validationErr := ValidateAndNormalizeGeminiImageRequest(GeminiImageValidationInput{
				ProviderOptions: test.options,
			})
			require.NotNil(t, validationErr)
			assert.Equal(t, test.code, validationErr.Code)
		})
	}
}

func TestIsGeminiImageModelIsExact(t *testing.T) {
	assert.True(t, IsGeminiImageModel(" GEMINI-3.1-FLASH-IMAGE "))
	assert.True(t, IsGeminiImageModel("gemini-3-pro-image-count"))
	assert.False(t, IsGeminiImageModel("gemini-3-pro-image-preview"))
	assert.False(t, IsGeminiImageModel("imagen-4.0-generate-001"))
}
