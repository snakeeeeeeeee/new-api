package common

import "testing"

func TestIsImageGenerationModelRecognizesCurrentImageFamilies(t *testing.T) {
	for _, modelName := range []string{
		"gpt-image-1",
		"gpt-image-2",
		"adobe-gpt-image-2-count",
		"gemini-2.0-flash-exp-image-generation",
		"gemini-2.5-flash-image",
		"gemini-3-pro-image-count",
		"gemini-3.1-flash-image",
		"nano-banana-pro-preview",
		"grok-imagine-image-quality",
		"grok-2-image-1212",
	} {
		if !IsImageGenerationModel(modelName) {
			t.Errorf("expected %q to be recognized as an image generation model", modelName)
		}
	}
}

func TestIsImageGenerationModelRejectsNonImageFamilies(t *testing.T) {
	for _, modelName := range []string{
		"gpt-5.4",
		"gemini-3.1-pro-preview",
		"grok-imagine-video",
		"adobe-seedance-2.0-fast-480p",
	} {
		if IsImageGenerationModel(modelName) {
			t.Errorf("expected %q not to be recognized as an image generation model", modelName)
		}
	}
}
