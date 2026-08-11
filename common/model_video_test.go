package common

import "testing"

func TestIsGrokVideoTaskModelRecognizesOnlyCanvas720pSKUs(t *testing.T) {
	for _, modelName := range []string{
		"grok-imagine-video-720p",
		"grok-imagine-video-1.5-preview-720p",
	} {
		if !IsGrokVideoTaskModel(modelName) {
			t.Errorf("expected %q to be recognized as a Grok video task model", modelName)
		}
	}

	for _, modelName := range []string{
		"grok-imagine-video",
		"grok-imagine-video-480p",
		"grok-imagine-video-1.5-preview-480p",
		"grok-imagine-video-1.5-preview-15s-720p",
		"grok-imagine-video-1.5-720p",
		"grok-imagine-image",
	} {
		if IsGrokVideoTaskModel(modelName) {
			t.Errorf("expected %q not to be recognized as a Grok video task model", modelName)
		}
	}
}
