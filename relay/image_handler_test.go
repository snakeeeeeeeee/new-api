package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
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
