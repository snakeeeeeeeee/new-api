package types

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	RouteModelGroupRatioSource    string
}

const (
	RouteModelGroupRatioSourceGlobal = "global"
	RouteModelGroupRatioSourceUser   = "user"
)

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
	ImagePricing                 *ImagePricingSnapshot
}

const (
	ImagePricingParameterQuality    = "quality"
	ImagePricingParameterSize       = "size"
	ImagePricingParameterResolution = "resolution"

	ImagePricingValueSourceRequest = "request"
	ImagePricingValueSourceAlias   = "alias"
	ImagePricingValueSourceDefault = "default"

	ImagePricingBillingType = "per_image_parameter"
	ImagePricingBillingMode = "image_parameter_per_call"
)

type ImagePricingConfig struct {
	Version       int                            `json:"version"`
	Profiles      map[string]ImagePricingProfile `json:"profiles"`
	ModelBindings map[string]ImagePricingBinding `json:"model_bindings"`
}

type ImagePricingProfile struct {
	Name        string             `json:"name"`
	Parameter   string             `json:"parameter"`
	DefaultTier string             `json:"default_tier"`
	Tiers       []ImagePricingTier `json:"tiers"`
}

type ImagePricingTier struct {
	Key           string   `json:"key"`
	UpstreamValue string   `json:"upstream_value"`
	Aliases       []string `json:"aliases"`
	UnitPrice     float64  `json:"unit_price"`
}

func (t *ImagePricingTier) UnmarshalJSON(data []byte) error {
	type tierAlias ImagePricingTier
	var decoded tierAlias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	rawUnitPrice, exists := fields["unit_price"]
	if !exists || common.GetJsonType(rawUnitPrice) == "null" {
		return fmt.Errorf("unit_price must be a number")
	}
	*t = ImagePricingTier(decoded)
	return nil
}

type ImagePricingBinding struct {
	Profile        string `json:"profile"`
	MaxN           int    `json:"max_n,omitempty"`
	MaxNConfigured bool   `json:"-"`
}

func (b *ImagePricingBinding) UnmarshalJSON(data []byte) error {
	type bindingAlias ImagePricingBinding
	var decoded bindingAlias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	if rawMaxN, exists := fields["max_n"]; exists {
		if common.GetJsonType(rawMaxN) == "null" {
			return fmt.Errorf("max_n must be an integer")
		}
		decoded.MaxNConfigured = true
	}
	*b = ImagePricingBinding(decoded)
	return nil
}

// PublicImagePricing is safe to expose through /api/pricing. Provider-facing
// values intentionally remain server-side configuration.
type PublicImagePricing struct {
	ProfileID   string                   `json:"profile_id"`
	Name        string                   `json:"name"`
	Parameter   string                   `json:"parameter"`
	DefaultTier string                   `json:"default_tier"`
	MaxN        int                      `json:"max_n"`
	Tiers       []PublicImagePricingTier `json:"tiers"`
}

type PublicImagePricingTier struct {
	Key       string   `json:"key"`
	Aliases   []string `json:"aliases,omitempty"`
	UnitPrice float64  `json:"unit_price"`
}

func (p *PublicImagePricing) DefaultUnitPrice() (float64, bool) {
	if p == nil {
		return 0, false
	}
	for _, tier := range p.Tiers {
		if strings.EqualFold(tier.Key, p.DefaultTier) {
			return tier.UnitPrice, true
		}
	}
	return 0, false
}

// ImagePricingSnapshot freezes every input needed to audit or settle an image
// request, so later configuration changes cannot reprice in-flight work.
type ImagePricingSnapshot struct {
	PublicModel   string  `json:"public_model"`
	ProfileID     string  `json:"profile_id"`
	ProfileHash   string  `json:"profile_hash"`
	Parameter     string  `json:"parameter"`
	RawValue      string  `json:"raw_value"`
	EffectiveTier string  `json:"effective_tier"`
	UpstreamValue string  `json:"upstream_value"`
	ValueSource   string  `json:"value_source"`
	UnitPrice     float64 `json:"unit_price"`
	N             int     `json:"n"`
	Subtotal      float64 `json:"subtotal"`
	GroupRatio    float64 `json:"group_ratio"`
	FinalQuota    int     `json:"final_quota"`
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
