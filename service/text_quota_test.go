package service

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCalculateTextQuotaSummaryUnifiedForClaudeSemantic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
	}

	priceData := types.PriceData{
		ModelRatio:           1,
		CompletionRatio:      2,
		CacheRatio:           0.1,
		CacheCreationRatio:   1.25,
		CacheCreation5mRatio: 1.25,
		CacheCreation1hRatio: 2,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 1,
		},
	}

	chatRelayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}
	messageRelayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}

	chatSummary := calculateTextQuotaSummary(ctx, chatRelayInfo, usage)
	messageSummary := calculateTextQuotaSummary(ctx, messageRelayInfo, usage)

	require.Equal(t, messageSummary.Quota, chatSummary.Quota)
	require.Equal(t, messageSummary.CacheCreationTokens5m, chatSummary.CacheCreationTokens5m)
	require.Equal(t, messageSummary.CacheCreationTokens1h, chatSummary.CacheCreationTokens1h)
	require.True(t, chatSummary.IsClaudeUsageSemantic)
	require.Equal(t, 1488, chatSummary.Quota)
}

func TestCalculateTextQuotaSummaryUsesSplitClaudeCacheCreationRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      1,
			CacheRatio:           0,
			CacheCreationRatio:   1,
			CacheCreation5mRatio: 2,
			CacheCreation1hRatio: 3,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 0,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedCreationTokens: 10,
		},
		ClaudeCacheCreation5mTokens: 2,
		ClaudeCacheCreation1hTokens: 3,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// 100 + remaining(5)*1 + 2*2 + 3*3 = 118
	require.Equal(t, 118, summary.Quota)
}

func TestCalculateTextQuotaSummaryClaudeCacheTTLBillingCompat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	baseRelayInfo := func() *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			RelayFormat:             types.RelayFormatClaude,
			FinalRequestRelayFormat: types.RelayFormatClaude,
			OriginModelName:         "claude-sonnet-4-6",
			PriceData: types.PriceData{
				ModelRatio:           1,
				CompletionRatio:      1,
				CacheRatio:           0,
				CacheCreationRatio:   1,
				CacheCreation5mRatio: 2,
				CacheCreation1hRatio: 3,
				GroupRatioInfo: types.GroupRatioInfo{
					GroupRatio: 2,
				},
			},
			StartTime: time.Now(),
		}
	}
	usage := func() *dto.Usage {
		return &dto.Usage{
			PromptTokens:     100,
			CompletionTokens: 0,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedCreationTokens: 10,
			},
			ClaudeCacheCreation1hTokens: 10,
		}
	}

	t.Run("switch off keeps upstream one hour billing", func(t *testing.T) {
		relayInfo := baseRelayInfo()

		summary := calculateTextQuotaSummary(ctx, relayInfo, usage())

		require.False(t, summary.ClaudeCacheTTLBillingCompat)
		require.Equal(t, 10, summary.CacheCreationTokens1h)
		require.Equal(t, 0, summary.CacheCreationTokens5m)
		// (100 + 10*3) * group 2 = 260
		require.Equal(t, 260, summary.Quota)
	})

	t.Run("switch on reprices explicit five minute request", func(t *testing.T) {
		relayInfo := baseRelayInfo()
		relayInfo.ClaudeCacheTTLBillingCompat = &relaycommon.ClaudeCacheTTLBillingCompatInfo{
			RequestedTTL:        relaycommon.ClaudeCacheTTL5m,
			UpstreamReportedTTL: relaycommon.ClaudeCacheTTL1h,
		}

		summary := calculateTextQuotaSummary(ctx, relayInfo, usage())

		require.True(t, summary.ClaudeCacheTTLBillingCompat)
		require.Equal(t, 0, summary.CacheCreationTokens1h)
		require.Equal(t, 10, summary.CacheCreationTokens5m)
		require.Equal(t, 10, summary.ClaudeCacheTTLRepricedTokens)
		require.Equal(t, 20, summary.ClaudeCacheTTLSubsidyQuota)
		require.Equal(t, 1.0, summary.ClaudeCacheTTLSubsidyRatioDelta)
		// (100 + 10*2) * group 2 = 240
		require.Equal(t, 240, summary.Quota)

		other := map[string]interface{}{}
		appendClaudeCacheTTLBillingCompatOther(other, summary)
		require.Equal(t, true, other["claude_cache_ttl_billing_compat"])
		require.Equal(t, relaycommon.ClaudeCacheTTL5m, other["claude_cache_ttl_requested"])
		require.Equal(t, relaycommon.ClaudeCacheTTL1h, other["claude_cache_ttl_upstream_reported"])
		require.Equal(t, 10, other["claude_cache_ttl_repriced_tokens"])
		require.Equal(t, 20, other["claude_cache_ttl_subsidy_quota"])
		require.InDelta(t, 0.00004, other["claude_cache_ttl_subsidy_usd"].(float64), 0.000001)
		require.Equal(t, "$0.000040", other["claude_cache_ttl_subsidy_amount"])
		require.Equal(t, 1.0, other["claude_cache_ttl_subsidy_ratio_delta"])
		require.Equal(t, 10, other["claude_cache_ttl_upstream_cache_creation_tokens_1h"])
		require.Equal(t, 10, other["claude_cache_ttl_billed_cache_creation_tokens_5m"])
	})

	t.Run("upstream five minute does not reprice", func(t *testing.T) {
		relayInfo := baseRelayInfo()
		relayInfo.ClaudeCacheTTLBillingCompat = &relaycommon.ClaudeCacheTTLBillingCompatInfo{
			RequestedTTL:        relaycommon.ClaudeCacheTTL5m,
			UpstreamReportedTTL: relaycommon.ClaudeCacheTTL1h,
		}
		upstream5mUsage := usage()
		upstream5mUsage.ClaudeCacheCreation1hTokens = 0
		upstream5mUsage.ClaudeCacheCreation5mTokens = 10

		summary := calculateTextQuotaSummary(ctx, relayInfo, upstream5mUsage)

		require.False(t, summary.ClaudeCacheTTLBillingCompat)
		require.Equal(t, 10, summary.CacheCreationTokens5m)
		require.Equal(t, 0, summary.CacheCreationTokens1h)
		require.Equal(t, 240, summary.Quota)
	})
}

func TestCalculateTextQuotaSummaryDoesNotMultiplyImageRequestCountForTokenBilledImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIImage,
		OriginModelName: "gpt-image-2",
		PriceData: types.PriceData{
			ModelRatio:      2.5,
			CompletionRatio: 6,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1.3,
			},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     420,
		CompletionTokens: 31706,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// (420 input tokens * $5 / 1M + 31706 output tokens * $30 / 1M) * group 1.3
	// = $1.239264, i.e. 619632 quota when QuotaPerUnit is 500000.
	require.Equal(t, 619632, summary.Quota)
}

func TestCalculateTextQuotaSummarySaturatesHugeOtherRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
			OtherRatios:     map[string]float64{"n": math.MaxFloat64},
		},
		StartTime: time.Now(),
	}
	usage := &dto.Usage{PromptTokens: 1}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	require.Equal(t, common.MaxQuota, summary.Quota)
}

func TestCalcViolationFeeQuotaSaturatesHugeAmount(t *testing.T) {
	require.Equal(t, common.MaxQuota, calcViolationFeeQuota(math.MaxFloat64, 1))
}

func TestCalculateAudioQuotaSaturatesHugePrice(t *testing.T) {
	quota := calculateAudioQuota(QuotaInfo{
		UsePrice:   true,
		ModelPrice: math.MaxFloat64,
		GroupRatio: 1,
	})

	require.Equal(t, common.MaxQuota, quota)
}

func TestCalculateTextQuotaSummaryUsesAnthropicUsageSemanticFromUpstreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      2,
			CacheRatio:           0.1,
			CacheCreationRatio:   1.25,
			CacheCreation5mRatio: 1.25,
			CacheCreation1hRatio: 2,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		UsageSemantic:    "anthropic",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	require.True(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, "anthropic", summary.UsageSemantic)
	require.Equal(t, 1488, summary.Quota)
}

func TestCacheWriteTokensTotal(t *testing.T) {
	t.Run("split cache creation", func(t *testing.T) {
		summary := textQuotaSummary{
			CacheCreationTokens:   50,
			CacheCreationTokens5m: 10,
			CacheCreationTokens1h: 20,
		}
		require.Equal(t, 50, cacheWriteTokensTotal(summary))
	})

	t.Run("legacy cache creation", func(t *testing.T) {
		summary := textQuotaSummary{CacheCreationTokens: 50}
		require.Equal(t, 50, cacheWriteTokensTotal(summary))
	})

	t.Run("split cache creation without aggregate remainder", func(t *testing.T) {
		summary := textQuotaSummary{
			CacheCreationTokens5m: 10,
			CacheCreationTokens1h: 20,
		}
		require.Equal(t, 30, cacheWriteTokensTotal(summary))
	})
}

func TestCalculateTextQuotaSummaryHandlesLegacyClaudeDerivedOpenAIUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      5,
			CacheRatio:           0.1,
			CacheCreationRatio:   1.25,
			CacheCreation5mRatio: 1.25,
			CacheCreation1hRatio: 2,
			GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     62,
		CompletionTokens: 95,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 3544,
		},
		ClaudeCacheCreation5mTokens: 586,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// 62 + 3544*0.1 + 586*1.25 + 95*5 = 1624.9 => 1624
	require.Equal(t, 1624, summary.Quota)
}

func TestCalculateTextQuotaSummarySeparatesOpenRouterCacheReadFromPromptBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "openai/gpt-4.1",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 2432,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// OpenRouter OpenAI-format display keeps prompt_tokens as total input,
	// but billing still separates normal input from cache read tokens.
	// quota = (2604 - 2432) + 2432*0.1 + 383 = 798.2 => 798
	require.Equal(t, 2604, summary.PromptTokens)
	require.Equal(t, 798, summary.Quota)
}

func TestCalculateTextQuotaSummarySeparatesOpenRouterCacheCreationFromPromptBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "openai/gpt-4.1",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedCreationTokens: 100,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// prompt_tokens is still logged as total input, but cache creation is billed separately.
	// quota = (2604 - 100) + 100*1.25 + 383 = 3012
	require.Equal(t, 2604, summary.PromptTokens)
	require.Equal(t, 3012, summary.Quota)
}

func TestCalculateTextQuotaSummaryKeepsPrePRClaudeOpenRouterBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "anthropic/claude-3.7-sonnet",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 2432,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// Pre-PR PostClaudeConsumeQuota behavior for OpenRouter:
	// prompt = 2604 - 2432 = 172
	// quota = 172 + 2432*0.1 + 383 = 798.2 => 798
	require.True(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, 172, summary.PromptTokens)
	require.Equal(t, 798, summary.Quota)
}

func TestCalculateTextQuotaSummaryOfficialCacheWriteBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cacheWrite := func(tokens int) *int { return &tokens }
	baseRelayInfo := func(configured bool, ratio float64) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			OriginModelName: "gpt-cache-write-test",
			PriceData: types.PriceData{
				ModelRatio:                   1,
				CompletionRatio:              6,
				CacheRatio:                   0.1,
				CacheCreationRatio:           ratio,
				CacheCreationRatioConfigured: configured,
				GroupRatioInfo:               types.GroupRatioInfo{GroupRatio: 1},
			},
			StartTime: time.Now(),
		}
	}
	baseUsage := func(write *int, legacy int) *dto.Usage {
		return &dto.Usage{
			PromptTokens:     2000,
			CompletionTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         800,
				CachedCreationTokens: legacy,
				CacheWriteTokens:     write,
			},
		}
	}

	tests := []struct {
		name               string
		configured         bool
		ratio              float64
		write              *int
		legacy             int
		wantQuota          int
		wantCreationTokens int
		wantReported       *int
		wantEnabled        bool
		wantWarning        bool
	}{
		{
			name:               "configured one point two five",
			configured:         true,
			ratio:              1.25,
			write:              cacheWrite(400),
			wantQuota:          1980,
			wantCreationTokens: 400,
			wantReported:       cacheWrite(400),
			wantEnabled:        true,
		},
		{
			name:         "unconfigured stays normal input",
			configured:   false,
			ratio:        1.25,
			write:        cacheWrite(400),
			wantQuota:    1880,
			wantReported: cacheWrite(400),
		},
		{
			name:               "configured zero is enabled",
			configured:         true,
			ratio:              0,
			write:              cacheWrite(400),
			wantQuota:          1480,
			wantCreationTokens: 400,
			wantReported:       cacheWrite(400),
			wantEnabled:        true,
		},
		{
			name:               "configured one is enabled",
			configured:         true,
			ratio:              1,
			write:              cacheWrite(400),
			wantQuota:          1880,
			wantCreationTokens: 400,
			wantReported:       cacheWrite(400),
			wantEnabled:        true,
		},
		{
			name:               "legacy fallback keeps old billing",
			configured:         false,
			ratio:              1.25,
			legacy:             400,
			wantQuota:          1980,
			wantCreationTokens: 400,
		},
		{
			name:         "official zero overrides legacy",
			configured:   true,
			ratio:        1.25,
			write:        cacheWrite(0),
			legacy:       400,
			wantQuota:    1880,
			wantReported: cacheWrite(0),
			wantEnabled:  true,
		},
		{
			name:         "negative is rejected",
			configured:   true,
			ratio:        1.25,
			write:        cacheWrite(-5),
			legacy:       400,
			wantQuota:    1880,
			wantReported: cacheWrite(-5),
			wantWarning:  true,
		},
		{
			name:         "more than available input is rejected",
			configured:   true,
			ratio:        1.25,
			write:        cacheWrite(1201),
			wantQuota:    1880,
			wantReported: cacheWrite(1201),
			wantWarning:  true,
		},
		{
			name:       "missing fields has no creation billing",
			configured: true,
			ratio:      1.25,
			wantQuota:  1880,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			summary := calculateTextQuotaSummary(ctx, baseRelayInfo(tt.configured, tt.ratio), baseUsage(tt.write, tt.legacy))

			require.Equal(t, tt.wantQuota, summary.Quota)
			require.Equal(t, tt.wantCreationTokens, summary.CacheCreationTokens)
			require.Equal(t, tt.wantReported, summary.CacheWriteTokensReported)
			require.Equal(t, tt.wantEnabled, summary.CacheWriteBillingEnabled)
			require.Equal(t, tt.wantWarning, summary.CacheWriteBillingWarning != "")
		})
	}
}

func TestAppendCacheWriteBillingOther(t *testing.T) {
	reported := 400
	other := map[string]interface{}{}
	appendCacheWriteBillingOther(other, textQuotaSummary{
		CacheWriteTokensReported: &reported,
		CacheWriteBillingEnabled: false,
	})

	require.Equal(t, 400, other["cache_write_tokens_reported"])
	require.Equal(t, false, other["cache_write_billing_enabled"])

	legacyOther := map[string]interface{}{}
	appendCacheWriteBillingOther(legacyOther, textQuotaSummary{CacheCreationTokens: 400})
	require.NotContains(t, legacyOther, "cache_write_tokens_reported")
	require.NotContains(t, legacyOther, "cache_write_billing_enabled")
}

func TestCalculateTextQuotaSummaryCacheWritePlanAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writeTokens := 400
	usage := &dto.Usage{
		PromptTokens:     2000,
		CompletionTokens: 100,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:     800,
			CacheWriteTokens: &writeTokens,
		},
	}

	for _, tt := range []struct {
		name       string
		configured bool
		wantQuota  int
	}{
		{name: "configured", configured: true, wantQuota: 1089},
		{name: "unconfigured", configured: false, wantQuota: 1034},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			relayInfo := &relaycommon.RelayInfo{
				OriginModelName: "gpt-cache-write-test",
				PriceData: types.PriceData{
					ModelRatio:                   0.5,
					CompletionRatio:              6,
					CacheRatio:                   0.1,
					CacheCreationRatio:           1.25,
					CacheCreationRatioConfigured: tt.configured,
					GroupRatioInfo:               types.GroupRatioInfo{GroupRatio: 1.1},
				},
				StartTime: time.Now(),
			}

			summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

			require.Equal(t, tt.wantQuota, summary.Quota)
		})
	}
}

func TestAppendInputTokensTotalOtherForOfficialCacheWrite(t *testing.T) {
	reported := 400
	usage := &dto.Usage{PromptTokens: 2000}
	summary := textQuotaSummary{CacheWriteTokensReported: &reported}

	other := map[string]interface{}{}
	appendInputTokensTotalOther(other, &relaycommon.RelayInfo{}, usage, summary)
	require.Equal(t, 2000, other["input_tokens_total"])

	legacyOther := map[string]interface{}{}
	appendInputTokensTotalOther(legacyOther, &relaycommon.RelayInfo{}, usage, textQuotaSummary{})
	require.NotContains(t, legacyOther, "input_tokens_total")

	normalizedOther := map[string]interface{}{}
	normalizedUsage := &dto.Usage{PromptTokens: 2000, InputTokens: 2100, UsageSource: "openai_responses"}
	appendInputTokensTotalOther(normalizedOther, &relaycommon.RelayInfo{}, normalizedUsage, summary)
	require.Equal(t, 2100, normalizedOther["input_tokens_total"])

	claudeOther := map[string]interface{}{}
	appendInputTokensTotalOther(claudeOther, &relaycommon.RelayInfo{
		FinalRequestRelayFormat: types.RelayFormatClaude,
	}, usage, summary)
	require.NotContains(t, claudeOther, "input_tokens_total")
}

func TestCalculateTextQuotaSummaryUsePriceSkipsOfficialCacheWriteClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheWrite := func(tokens int) *int { return &tokens }

	tests := []struct {
		name               string
		write              *int
		legacy             int
		creation5m         int
		creation1h         int
		wantCreationTokens int
		wantCreation5m     int
		wantCreation1h     int
	}{
		{name: "official positive", write: cacheWrite(400), legacy: 999, creation5m: 10, creation1h: 20},
		{name: "official explicit zero", write: cacheWrite(0), legacy: 999},
		{name: "official negative", write: cacheWrite(-5), legacy: 999},
		{name: "official exceeds input", write: cacheWrite(2001), legacy: 999},
		{
			name:               "legacy only remains unchanged",
			legacy:             400,
			creation5m:         10,
			creation1h:         20,
			wantCreationTokens: 400,
			wantCreation5m:     10,
			wantCreation1h:     20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			relayInfo := &relaycommon.RelayInfo{
				OriginModelName: "fixed-price-cache-write-test",
				PriceData: types.PriceData{
					UsePrice:                     true,
					ModelPrice:                   0.01,
					CacheCreationRatio:           1.25,
					CacheCreation5mRatio:         1.25,
					CacheCreation1hRatio:         2,
					GroupRatioInfo:               types.GroupRatioInfo{GroupRatio: 1},
					CacheCreationRatioConfigured: true,
				},
				StartTime: time.Now(),
			}
			usage := &dto.Usage{
				PromptTokens:     2000,
				CompletionTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         800,
					CachedCreationTokens: tt.legacy,
					CacheWriteTokens:     tt.write,
				},
				ClaudeCacheCreation5mTokens: tt.creation5m,
				ClaudeCacheCreation1hTokens: tt.creation1h,
			}

			summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

			require.Equal(t, 5000, summary.Quota)
			require.Equal(t, tt.wantCreationTokens, summary.CacheCreationTokens)
			require.Equal(t, tt.wantCreation5m, summary.CacheCreationTokens5m)
			require.Equal(t, tt.wantCreation1h, summary.CacheCreationTokens1h)
			require.Nil(t, summary.CacheWriteTokensReported)
			require.False(t, summary.CacheWriteBillingEnabled)
			require.Empty(t, summary.CacheWriteBillingWarning)
		})
	}
}

func TestCalculateTextQuotaSummaryOfficialCacheWriteSuppressesOpenRouterCostInference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheWrite := func(tokens int) *int { return &tokens }

	priceData := types.PriceData{
		ModelRatio:         1.5,
		CompletionRatio:    1,
		CacheRatio:         0.1,
		CacheCreationRatio: 1.25,
		GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
	}
	// This cost would infer 100 cache creation tokens if the legacy OpenRouter
	// cost fallback were allowed to run.
	const upstreamCost = 0.0024696
	probeUsage := dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		Cost:             upstreamCost,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 2432,
		},
	}
	require.Equal(t, 100, CalcOpenRouterCacheCreateTokens(probeUsage, priceData))

	tests := []struct {
		name         string
		write        *int
		configured   bool
		wantReported int
		wantEnabled  bool
	}{
		{
			name:         "official explicit zero",
			write:        cacheWrite(0),
			configured:   true,
			wantReported: 0,
			wantEnabled:  true,
		},
		{
			name:         "unconfigured official positive",
			write:        cacheWrite(100),
			wantReported: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			testPriceData := priceData
			testPriceData.CacheCreationRatioConfigured = tt.configured
			relayInfo := &relaycommon.RelayInfo{
				FinalRequestRelayFormat: types.RelayFormatClaude,
				OriginModelName:         "claude-3-7-sonnet-20250219",
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType: constant.ChannelTypeOpenRouter,
				},
				PriceData: testPriceData,
				StartTime: time.Now(),
			}
			usage := probeUsage
			usage.PromptTokensDetails.CacheWriteTokens = tt.write

			summary := calculateTextQuotaSummary(ctx, relayInfo, &usage)

			require.Zero(t, summary.CacheCreationTokens)
			require.Equal(t, 172, summary.PromptTokens)
			require.NotNil(t, summary.CacheWriteTokensReported)
			require.Equal(t, tt.wantReported, *summary.CacheWriteTokensReported)
			require.Equal(t, tt.wantEnabled, summary.CacheWriteBillingEnabled)
		})
	}
}
