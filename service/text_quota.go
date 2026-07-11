package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type textQuotaSummary struct {
	PromptTokens                                int
	CompletionTokens                            int
	TotalTokens                                 int
	CacheTokens                                 int
	CacheCreationTokens                         int
	CacheCreationTokens5m                       int
	CacheCreationTokens1h                       int
	CacheWriteTokensReported                    *int
	CacheWriteBillingEnabled                    bool
	CacheWriteBillingWarning                    string
	ImageTokens                                 int
	AudioTokens                                 int
	ModelName                                   string
	TokenName                                   string
	UseTimeSeconds                              int64
	CompletionRatio                             float64
	CacheRatio                                  float64
	ImageRatio                                  float64
	ModelRatio                                  float64
	GroupRatio                                  float64
	ModelPrice                                  float64
	CacheCreationRatio                          float64
	CacheCreationRatio5m                        float64
	CacheCreationRatio1h                        float64
	Quota                                       int
	IsClaudeUsageSemantic                       bool
	UsageSemantic                               string
	ClaudeCacheTTLBillingCompat                 bool
	ClaudeCacheTTLRepricedTokens                int
	ClaudeCacheTTLSubsidyQuota                  int
	ClaudeCacheTTLSubsidyRatioDelta             float64
	ClaudeCacheTTLUpstreamCacheCreation1hTokens int
	ClaudeCacheTTLBilledCacheCreation5mTokens   int
	WebSearchPrice                              float64
	WebSearchCallCount                          int
	ClaudeWebSearchPrice                        float64
	ClaudeWebSearchCallCount                    int
	FileSearchPrice                             float64
	FileSearchCallCount                         int
	AudioInputPrice                             float64
	ImageGenerationCallPrice                    float64
}

func cacheWriteTokensTotal(summary textQuotaSummary) int {
	if summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0 {
		splitCacheWriteTokens := summary.CacheCreationTokens5m + summary.CacheCreationTokens1h
		if summary.CacheCreationTokens > splitCacheWriteTokens {
			return summary.CacheCreationTokens
		}
		return splitCacheWriteTokens
	}
	return summary.CacheCreationTokens
}

func isLegacyClaudeDerivedOpenAIUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) bool {
	if relayInfo == nil || usage == nil {
		return false
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return false
	}
	if usage.UsageSource != "" || usage.UsageSemantic != "" {
		return false
	}
	return usage.ClaudeCacheCreation5mTokens > 0 || usage.ClaudeCacheCreation1hTokens > 0
}

func applyClaudeCacheTTLBillingCompat(relayInfo *relaycommon.RelayInfo, summary *textQuotaSummary) {
	if relayInfo == nil || summary == nil || relayInfo.PriceData.UsePrice {
		return
	}
	compat := relayInfo.ClaudeCacheTTLBillingCompat
	if compat == nil || compat.RequestedTTL != relaycommon.ClaudeCacheTTL5m {
		return
	}
	if !summary.IsClaudeUsageSemantic || summary.CacheCreationTokens1h <= 0 {
		return
	}
	repricedTokens := summary.CacheCreationTokens1h
	ratioDelta := summary.CacheCreationRatio1h - summary.CacheCreationRatio5m
	subsidyQuota := 0
	if ratioDelta > 0 {
		subsidyDecimal := decimal.NewFromInt(int64(repricedTokens)).
			Mul(decimal.NewFromFloat(ratioDelta)).
			Mul(decimal.NewFromFloat(summary.ModelRatio)).
			Mul(decimal.NewFromFloat(summary.GroupRatio))
		subsidyQuota = common.QuotaFromDecimalRound(subsidyDecimal)
	}

	summary.ClaudeCacheTTLBillingCompat = true
	summary.ClaudeCacheTTLRepricedTokens = repricedTokens
	summary.ClaudeCacheTTLSubsidyQuota = subsidyQuota
	summary.ClaudeCacheTTLSubsidyRatioDelta = ratioDelta
	summary.ClaudeCacheTTLUpstreamCacheCreation1hTokens = repricedTokens
	summary.CacheCreationTokens5m += repricedTokens
	summary.CacheCreationTokens1h = 0
	summary.ClaudeCacheTTLBilledCacheCreation5mTokens = summary.CacheCreationTokens5m

	compat.RepricedTokens = repricedTokens
	compat.SubsidyQuota = subsidyQuota
	compat.SubsidyRatioDelta = ratioDelta
	compat.UpstreamCacheCreation1hTokens = repricedTokens
	compat.BilledCacheCreation5mTokens = summary.CacheCreationTokens5m
}

func appendClaudeCacheTTLBillingCompatOther(other map[string]interface{}, summary textQuotaSummary) {
	if other == nil || !summary.ClaudeCacheTTLBillingCompat {
		return
	}
	other["claude_cache_ttl_billing_compat"] = true
	other["claude_cache_ttl_requested"] = relaycommon.ClaudeCacheTTL5m
	other["claude_cache_ttl_upstream_reported"] = relaycommon.ClaudeCacheTTL1h
	other["claude_cache_ttl_repriced_tokens"] = summary.ClaudeCacheTTLRepricedTokens
	other["claude_cache_ttl_subsidy_quota"] = summary.ClaudeCacheTTLSubsidyQuota
	other["claude_cache_ttl_subsidy_usd"] = quotaUSDValue(summary.ClaudeCacheTTLSubsidyQuota)
	other["claude_cache_ttl_subsidy_amount"] = formatQuotaUSD(summary.ClaudeCacheTTLSubsidyQuota)
	other["claude_cache_ttl_subsidy_ratio_delta"] = summary.ClaudeCacheTTLSubsidyRatioDelta
	other["claude_cache_ttl_upstream_cache_creation_tokens_1h"] = summary.ClaudeCacheTTLUpstreamCacheCreation1hTokens
	other["claude_cache_ttl_billed_cache_creation_tokens_5m"] = summary.ClaudeCacheTTLBilledCacheCreation5mTokens
}

func appendCacheWriteBillingOther(other map[string]interface{}, summary textQuotaSummary) {
	if other == nil || summary.CacheWriteTokensReported == nil {
		return
	}
	other["cache_write_tokens_reported"] = *summary.CacheWriteTokensReported
	other["cache_write_billing_enabled"] = summary.CacheWriteBillingEnabled
}

func appendInputTokensTotalOther(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, summary textQuotaSummary) {
	if other == nil || relayInfo == nil || usage == nil || relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return
	}
	if usage.UsageSource != "" && usage.InputTokens > 0 {
		// Only trust normalized input_tokens when a conversion path has tagged
		// the usage source; older payloads may use different token semantics.
		other["input_tokens_total"] = usage.InputTokens
		return
	}
	if summary.CacheWriteTokensReported != nil && usage.PromptTokens > 0 {
		// Official cache_write_tokens is reported alongside a reliable OpenAI
		// prompt token total even when an adapter has not tagged UsageSource.
		other["input_tokens_total"] = usage.PromptTokens
	}
}

func calculateTextQuotaSummary(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage) textQuotaSummary {
	summary := textQuotaSummary{
		ModelName:            relayInfo.OriginModelName,
		TokenName:            ctx.GetString("token_name"),
		UseTimeSeconds:       time.Now().Unix() - relayInfo.StartTime.Unix(),
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		CacheRatio:           relayInfo.PriceData.CacheRatio,
		ImageRatio:           relayInfo.PriceData.ImageRatio,
		ModelRatio:           relayInfo.PriceData.ModelRatio,
		GroupRatio:           relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelPrice:           relayInfo.PriceData.ModelPrice,
		CacheCreationRatio:   relayInfo.PriceData.CacheCreationRatio,
		CacheCreationRatio5m: relayInfo.PriceData.CacheCreation5mRatio,
		CacheCreationRatio1h: relayInfo.PriceData.CacheCreation1hRatio,
		UsageSemantic:        usageSemanticFromUsage(relayInfo, usage),
	}
	summary.IsClaudeUsageSemantic = summary.UsageSemantic == "anthropic"

	if usage == nil {
		usage = &dto.Usage{
			PromptTokens:     relayInfo.GetEstimatePromptTokens(),
			CompletionTokens: 0,
			TotalTokens:      relayInfo.GetEstimatePromptTokens(),
		}
	}

	summary.PromptTokens = usage.PromptTokens
	summary.CompletionTokens = usage.CompletionTokens
	summary.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	summary.CacheTokens = usage.PromptTokensDetails.CachedTokens
	cacheCreationTokens, cacheWriteTokensReported := usage.PromptTokensDetails.ResolveCacheCreationTokens()
	summary.CacheCreationTokens = cacheCreationTokens
	summary.CacheCreationTokens5m = usage.ClaudeCacheCreation5mTokens
	summary.CacheCreationTokens1h = usage.ClaudeCacheCreation1hTokens
	summary.ImageTokens = usage.PromptTokensDetails.ImageTokens
	summary.AudioTokens = usage.PromptTokensDetails.AudioTokens
	legacyClaudeDerived := isLegacyClaudeDerivedOpenAIUsage(relayInfo, usage)
	if cacheWriteTokensReported {
		summary.CacheCreationTokens = 0
		if relayInfo.PriceData.UsePrice {
			summary.CacheCreationTokens5m = 0
			summary.CacheCreationTokens1h = 0
		} else {
			reportedTokens := *usage.PromptTokensDetails.CacheWriteTokens
			summary.CacheWriteTokensReported = &reportedTokens

			switch {
			case reportedTokens < 0:
				summary.CacheWriteBillingWarning = fmt.Sprintf("上游返回的缓存写入 Tokens 无效：%d，已按普通输入计费", reportedTokens)
			case cacheCreationTokens > summary.PromptTokens:
				summary.CacheWriteBillingWarning = fmt.Sprintf("上游返回的缓存写入 Tokens %d 超过输入 Tokens %d，已按普通输入计费", reportedTokens, summary.PromptTokens)
			case relayInfo.PriceData.CacheCreationRatioConfigured:
				summary.CacheCreationTokens = cacheCreationTokens
				summary.CacheWriteBillingEnabled = true
			}
			if summary.CacheWriteBillingWarning != "" {
				logger.LogWarn(ctx, summary.CacheWriteBillingWarning)
			}
		}
	}
	isOpenRouterClaudeBilling := relayInfo.ChannelMeta != nil &&
		relayInfo.ChannelType == constant.ChannelTypeOpenRouter &&
		summary.IsClaudeUsageSemantic

	if isOpenRouterClaudeBilling {
		summary.PromptTokens -= summary.CacheTokens
		isUsingCustomSettings := relayInfo.PriceData.UsePrice || hasCustomModelRatio(summary.ModelName, relayInfo.PriceData.ModelRatio)
		if summary.CacheCreationTokens == 0 && summary.CacheWriteTokensReported == nil && relayInfo.PriceData.CacheCreationRatio != 1 && usage.Cost != 0 && !isUsingCustomSettings {
			maybeCacheCreationTokens := CalcOpenRouterCacheCreateTokens(*usage, relayInfo.PriceData)
			if maybeCacheCreationTokens >= 0 && summary.PromptTokens >= maybeCacheCreationTokens {
				summary.CacheCreationTokens = maybeCacheCreationTokens
			}
		}
		summary.PromptTokens -= summary.CacheCreationTokens
	}
	applyClaudeCacheTTLBillingCompat(relayInfo, &summary)

	dPromptTokens := decimal.NewFromInt(int64(summary.PromptTokens))
	dCacheTokens := decimal.NewFromInt(int64(summary.CacheTokens))
	dImageTokens := decimal.NewFromInt(int64(summary.ImageTokens))
	dAudioTokens := decimal.NewFromInt(int64(summary.AudioTokens))
	dCompletionTokens := decimal.NewFromInt(int64(summary.CompletionTokens))
	dCachedCreationTokens := decimal.NewFromInt(int64(summary.CacheCreationTokens))
	dCompletionRatio := decimal.NewFromFloat(summary.CompletionRatio)
	dCacheRatio := decimal.NewFromFloat(summary.CacheRatio)
	dImageRatio := decimal.NewFromFloat(summary.ImageRatio)
	dModelRatio := decimal.NewFromFloat(summary.ModelRatio)
	dGroupRatio := decimal.NewFromFloat(summary.GroupRatio)
	dModelPrice := decimal.NewFromFloat(summary.ModelPrice)
	dCacheCreationRatio := decimal.NewFromFloat(summary.CacheCreationRatio)
	dCacheCreationRatio5m := decimal.NewFromFloat(summary.CacheCreationRatio5m)
	dCacheCreationRatio1h := decimal.NewFromFloat(summary.CacheCreationRatio1h)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	ratio := dModelRatio.Mul(dGroupRatio)

	var dWebSearchQuota decimal.Decimal
	if relayInfo.ResponsesUsageInfo != nil {
		if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool.CallCount > 0 {
			summary.WebSearchCallCount = webSearchTool.CallCount
			summary.WebSearchPrice = operation_setting.GetWebSearchPricePerThousand(summary.ModelName, webSearchTool.SearchContextSize)
			dWebSearchQuota = decimal.NewFromFloat(summary.WebSearchPrice).
				Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
	} else if strings.HasSuffix(summary.ModelName, "search-preview") {
		searchContextSize := ctx.GetString("chat_completion_web_search_context_size")
		if searchContextSize == "" {
			searchContextSize = "medium"
		}
		summary.WebSearchCallCount = 1
		summary.WebSearchPrice = operation_setting.GetWebSearchPricePerThousand(summary.ModelName, searchContextSize)
		dWebSearchQuota = decimal.NewFromFloat(summary.WebSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
	}

	var dClaudeWebSearchQuota decimal.Decimal
	summary.ClaudeWebSearchCallCount = ctx.GetInt("claude_web_search_requests")
	if summary.ClaudeWebSearchCallCount > 0 {
		summary.ClaudeWebSearchPrice = operation_setting.GetClaudeWebSearchPricePerThousand()
		dClaudeWebSearchQuota = decimal.NewFromFloat(summary.ClaudeWebSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit).
			Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount)))
	}

	var dFileSearchQuota decimal.Decimal
	if relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch]; exists && fileSearchTool.CallCount > 0 {
			summary.FileSearchCallCount = fileSearchTool.CallCount
			summary.FileSearchPrice = operation_setting.GetFileSearchPricePerThousand()
			dFileSearchQuota = decimal.NewFromFloat(summary.FileSearchPrice).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
	}

	var dImageGenerationCallQuota decimal.Decimal
	if ctx.GetBool("image_generation_call") {
		summary.ImageGenerationCallPrice = operation_setting.GetGPTImage1PriceOnceCall(ctx.GetString("image_generation_call_quality"), ctx.GetString("image_generation_call_size"))
		dImageGenerationCallQuota = decimal.NewFromFloat(summary.ImageGenerationCallPrice).Mul(dGroupRatio).Mul(dQuotaPerUnit)
	}

	var audioInputQuota decimal.Decimal
	if !relayInfo.PriceData.UsePrice {
		baseTokens := dPromptTokens

		var cachedTokensWithRatio decimal.Decimal
		if !dCacheTokens.IsZero() {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCacheTokens)
			}
			cachedTokensWithRatio = dCacheTokens.Mul(dCacheRatio)
		}

		var cachedCreationTokensWithRatio decimal.Decimal
		hasSplitCacheCreationTokens := summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0
		if !dCachedCreationTokens.IsZero() || hasSplitCacheCreationTokens {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCachedCreationTokens)
				cachedCreationTokensWithRatio = dCachedCreationTokens.Mul(dCacheCreationRatio)
			} else {
				remaining := summary.CacheCreationTokens - summary.CacheCreationTokens5m - summary.CacheCreationTokens1h
				if remaining < 0 {
					remaining = 0
				}
				cachedCreationTokensWithRatio = decimal.NewFromInt(int64(remaining)).Mul(dCacheCreationRatio)
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens5m)).Mul(dCacheCreationRatio5m))
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens1h)).Mul(dCacheCreationRatio1h))
			}
		}

		var imageTokensWithRatio decimal.Decimal
		if !dImageTokens.IsZero() {
			baseTokens = baseTokens.Sub(dImageTokens)
			imageTokensWithRatio = dImageTokens.Mul(dImageRatio)
		}

		if !dAudioTokens.IsZero() {
			summary.AudioInputPrice = operation_setting.GetGeminiInputAudioPricePerMillionTokens(summary.ModelName)
			if summary.AudioInputPrice > 0 {
				baseTokens = baseTokens.Sub(dAudioTokens)
				audioInputQuota = decimal.NewFromFloat(summary.AudioInputPrice).
					Div(decimal.NewFromInt(1000000)).Mul(dAudioTokens).Mul(dGroupRatio).Mul(dQuotaPerUnit)
			}
		}
		if summary.CacheWriteBillingEnabled && baseTokens.IsNegative() {
			baseTokens = decimal.Zero
		}

		promptQuota := baseTokens.Add(cachedTokensWithRatio).Add(imageTokensWithRatio).Add(cachedCreationTokensWithRatio)
		completionQuota := dCompletionTokens.Mul(dCompletionRatio)
		quotaCalculateDecimal := promptQuota.Add(completionQuota).Mul(ratio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)

		if len(relayInfo.PriceData.OtherRatios) > 0 {
			for _, otherRatio := range relayInfo.PriceData.OtherRatios {
				quotaCalculateDecimal = quotaCalculateDecimal.Mul(decimal.NewFromFloat(otherRatio))
			}
		}

		if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
			quotaCalculateDecimal = decimal.NewFromInt(1)
		}
		summary.Quota = common.QuotaFromDecimalRound(quotaCalculateDecimal)
	} else {
		quotaCalculateDecimal := dModelPrice.Mul(dQuotaPerUnit).Mul(dGroupRatio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)
		if len(relayInfo.PriceData.OtherRatios) > 0 {
			for _, otherRatio := range relayInfo.PriceData.OtherRatios {
				quotaCalculateDecimal = quotaCalculateDecimal.Mul(decimal.NewFromFloat(otherRatio))
			}
		}
		summary.Quota = common.QuotaFromDecimalRound(quotaCalculateDecimal)
	}

	if summary.TotalTokens == 0 {
		summary.Quota = 0
	} else if !ratio.IsZero() && summary.Quota == 0 {
		summary.Quota = 1
	}

	return summary
}

func usageSemanticFromUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) string {
	if usage != nil && usage.UsageSemantic != "" {
		return usage.UsageSemantic
	}
	if relayInfo != nil && relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return "anthropic"
	}
	return "openai"
}

func PostTextConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string) {
	originUsage := usage
	if usage == nil {
		extraContent = append(extraContent, "上游无计费信息")
	}
	if originUsage != nil {
		ObserveChannelAffinityUsageCacheByRelayFormat(ctx, usage, relayInfo.GetFinalRequestRelayFormat())
	}

	adminRejectReason := common.GetContextKeyString(ctx, constant.ContextKeyAdminRejectReason)
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	if summary.CacheWriteBillingWarning != "" {
		extraContent = append(extraContent, summary.CacheWriteBillingWarning)
	}

	if summary.WebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 %d 次，调用花费 %s", summary.WebSearchCallCount, decimal.NewFromFloat(summary.WebSearchPrice).Mul(decimal.NewFromInt(int64(summary.WebSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ClaudeWebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Claude Web Search 调用 %d 次，调用花费 %s", summary.ClaudeWebSearchCallCount, decimal.NewFromFloat(summary.ClaudeWebSearchPrice).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount))).String()))
	}
	if summary.FileSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("File Search 调用 %d 次，调用花费 %s", summary.FileSearchCallCount, decimal.NewFromFloat(summary.FileSearchPrice).Mul(decimal.NewFromInt(int64(summary.FileSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Audio Input 花费 %s", decimal.NewFromFloat(summary.AudioInputPrice).Div(decimal.NewFromInt(1000000)).Mul(decimal.NewFromInt(int64(summary.AudioTokens))).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ImageGenerationCallPrice > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Image Generation Call 花费 %s", decimal.NewFromFloat(summary.ImageGenerationCallPrice).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ClaudeCacheTTLBillingCompat {
		extraContent = append(extraContent, fmt.Sprintf("Claude cache TTL 计费兼容：上游按 1h 返回 %d tokens，本地按客户端声明 5m 计费，本次补贴 %d quota（约 %s）",
			summary.ClaudeCacheTTLRepricedTokens,
			summary.ClaudeCacheTTLSubsidyQuota,
			formatQuotaUSD(summary.ClaudeCacheTTLSubsidyQuota),
		))
	}

	if summary.TotalTokens == 0 {
		extraContent = append(extraContent, "上游没有返回计费信息，无法扣费（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, summary.ModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, summary.Quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, summary.Quota)
	}

	if err := SettleBilling(ctx, relayInfo, summary.Quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
	}

	logModel := summary.ModelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}

	logContent := strings.Join(extraContent, ", ")
	var other map[string]interface{}
	if summary.IsClaudeUsageSemantic {
		other = GenerateClaudeOtherInfo(ctx, relayInfo,
			summary.ModelRatio, summary.GroupRatio, summary.CompletionRatio,
			summary.CacheTokens, summary.CacheRatio,
			summary.CacheCreationTokens, summary.CacheCreationRatio,
			summary.CacheCreationTokens5m, summary.CacheCreationRatio5m,
			summary.CacheCreationTokens1h, summary.CacheCreationRatio1h,
			summary.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
		other["usage_semantic"] = "anthropic"
	} else {
		other = GenerateTextOtherInfo(ctx, relayInfo, summary.ModelRatio, summary.GroupRatio, summary.CompletionRatio, summary.CacheTokens, summary.CacheRatio, summary.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	}
	if adminRejectReason != "" {
		other["reject_reason"] = adminRejectReason
	}
	appendClaudeCacheTTLBillingCompatOther(other, summary)
	appendCacheWriteBillingOther(other, summary)
	if summary.ImageTokens != 0 {
		other["image"] = true
		other["image_ratio"] = summary.ImageRatio
		other["image_output"] = summary.ImageTokens
	}
	if summary.WebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.WebSearchCallCount
		other["web_search_price"] = summary.WebSearchPrice
	} else if summary.ClaudeWebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.ClaudeWebSearchCallCount
		other["web_search_price"] = summary.ClaudeWebSearchPrice
	}
	if summary.FileSearchCallCount > 0 {
		other["file_search"] = true
		other["file_search_call_count"] = summary.FileSearchCallCount
		other["file_search_price"] = summary.FileSearchPrice
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		other["audio_input_seperate_price"] = true
		other["audio_input_token_count"] = summary.AudioTokens
		other["audio_input_price"] = summary.AudioInputPrice
	}
	if summary.ImageGenerationCallPrice > 0 {
		other["image_generation_call"] = true
		other["image_generation_call_price"] = summary.ImageGenerationCallPrice
	}
	if summary.CacheCreationTokens > 0 {
		other["cache_creation_tokens"] = summary.CacheCreationTokens
		other["cache_creation_ratio"] = summary.CacheCreationRatio
	}
	if summary.CacheCreationTokens5m > 0 {
		other["cache_creation_tokens_5m"] = summary.CacheCreationTokens5m
		other["cache_creation_ratio_5m"] = summary.CacheCreationRatio5m
	}
	if summary.CacheCreationTokens1h > 0 {
		other["cache_creation_tokens_1h"] = summary.CacheCreationTokens1h
		other["cache_creation_ratio_1h"] = summary.CacheCreationRatio1h
	}
	cacheWriteTokens := cacheWriteTokensTotal(summary)
	if cacheWriteTokens > 0 {
		// cache_write_tokens: normalized cache creation total for UI display.
		// If split 5m/1h values are present, this is their sum; otherwise it falls back
		// to cache_creation_tokens.
		other["cache_write_tokens"] = cacheWriteTokens
	}
	appendInputTokensTotalOther(other, relayInfo, usage, summary)

	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     summary.PromptTokens,
		CompletionTokens: summary.CompletionTokens,
		ModelName:        logModel,
		TokenName:        summary.TokenName,
		Quota:            summary.Quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(summary.UseTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}
