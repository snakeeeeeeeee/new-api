package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultVendorRulesClassifyWrappedVideoModels(t *testing.T) {
	tests := []struct {
		model  string
		vendor string
	}{
		{model: "adobe-veo-3.1-480p", vendor: "Google"},
		{model: "adobe-veo-3.1-fast-1080p", vendor: "Google"},
		{model: "adobe-seedance-2.0-480p", vendor: "即梦"},
		{model: "adobe-seedance-2.0-fast-720p", vendor: "即梦"},
		{model: "adobe-kling-3.0-720p", vendor: "快手"},
		{model: "adobe-kling-o3-1080p", vendor: "快手"},
		{model: "leonardo-minimax-h3-1440p", vendor: "MiniMax"},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			require.Equal(t, test.vendor, getDefaultVendorName(test.model))
		})
	}
}
