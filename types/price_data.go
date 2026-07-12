package types

import (
	"fmt"
	"math"
)

type GroupRatioInfo struct {
	GroupRatio                    float64
	GroupSpecialRatio             float64
	HasSpecialRatio               bool
	OriginalGroupRatio            float64
	RatioOverride                 float64
	HasRatioOverride              bool
	RatioOverrideApplied          bool
	RouteModelGroupRatio          float64
	HasRouteModelGroupRatio       bool
	RouteModelRatioAggregateGroup string
	RouteModelRatioRealGroup      string
	RouteModelRatioModelName      string
}

type PriceData struct {
	FreeModel          bool
	ModelPrice         float64
	ModelRatio         float64
	CompletionRatio    float64
	CacheRatio         float64
	CacheCreationRatio float64
	// CacheCreationRatioConfigured distinguishes an explicit configuration,
	// including valid zero and one values, from the default ratio fallback.
	CacheCreationRatioConfigured bool
	CacheCreation5mRatio         float64
	CacheCreation1hRatio         float64
	ImageRatio                   float64
	AudioRatio                   float64
	AudioCompletionRatio         float64
	OtherRatios                  map[string]float64
	UsePrice                     bool
	Quota                        int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume            int // 按量计费的预消耗额度
	GroupRatioInfo               GroupRatioInfo
	TokenTierPricing             *TokenTierPricingSnapshot
}

const (
	TokenTierServiceTierStandard = "standard"
	TokenTierMeterInputTotal     = "input_tokens_total"
	TokenTierBillingWholeRequest = "whole_request"
)

type TokenTierPrices struct {
	Input       float64 `json:"input"`
	CachedInput float64 `json:"cached_input"`
	CacheWrite  float64 `json:"cache_write"`
	Output      float64 `json:"output"`
}

type TokenTier struct {
	UpToInclusive *int            `json:"up_to_inclusive"`
	UseBasePrice  bool            `json:"use_base_price,omitempty"`
	Prices        TokenTierPrices `json:"prices,omitempty"`
}

type TokenTierPricingRule struct {
	ID          string      `json:"id"`
	Enabled     bool        `json:"enabled"`
	ServiceTier string      `json:"service_tier"`
	Meter       string      `json:"meter"`
	BillingMode string      `json:"billing_mode"`
	Tiers       []TokenTier `json:"tiers,omitempty"`
}

type TokenTierPricingRuleMeta struct {
	Rule   TokenTierPricingRule `json:"rule"`
	Source string               `json:"source"`
	Hash   string               `json:"hash"`
}

type TokenTierPricingSnapshot struct {
	RuleID      string      `json:"rule_id"`
	RuleHash    string      `json:"rule_hash"`
	Source      string      `json:"source"`
	ServiceTier string      `json:"service_tier"`
	Meter       string      `json:"meter"`
	BillingMode string      `json:"billing_mode"`
	Tiers       []TokenTier `json:"tiers"`
}

func (s *TokenTierPricingSnapshot) SelectTier(inputTokens int) (TokenTier, int, bool) {
	if s == nil || len(s.Tiers) == 0 {
		return TokenTier{}, 0, false
	}
	for index, tier := range s.Tiers {
		if tier.UpToInclusive == nil || inputTokens <= *tier.UpToInclusive {
			return tier, index, true
		}
	}
	return TokenTier{}, 0, false
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if p.OtherRatios == nil {
		p.OtherRatios = make(map[string]float64)
	}
	if !(ratio > 0) || math.IsInf(ratio, 1) {
		return
	}
	p.OtherRatios[key] = ratio
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreationRatioConfigured: %t, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreationRatioConfigured, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
