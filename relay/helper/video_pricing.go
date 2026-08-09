package helper

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

func ResolveVideoPricing(publicModel string, seconds int, basis string, groupRatio float64, hasReferenceVideo bool) (*types.VideoPricingSnapshot, bool, error) {
	profile, binding, profileHash, bound := ratio_setting.GetVideoPricingForModel(publicModel)
	if !bound {
		return nil, false, nil
	}
	if seconds <= 0 {
		return nil, true, fmt.Errorf("模型 %s 必须提供大于 0 的整数视频时长", publicModel)
	}
	switch basis {
	case types.VideoPricingBasisGeneration, types.VideoPricingBasisExtensionDelta:
	default:
		return nil, true, fmt.Errorf("模型 %s 不支持视频计费依据 %s", publicModel, basis)
	}

	referenceVideoApplied := hasReferenceVideo && profile.ReferenceVideoUnitPrice > 0
	effectiveUnitPrice := decimal.NewFromFloat(profile.UnitPrice)
	if referenceVideoApplied {
		effectiveUnitPrice = effectiveUnitPrice.Add(decimal.NewFromFloat(profile.ReferenceVideoUnitPrice))
	}
	subtotal := effectiveUnitPrice.Mul(decimal.NewFromInt(int64(seconds)))
	quota := subtotal.
		Mul(decimal.NewFromFloat(groupRatio)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	subtotalFloat, _ := subtotal.Float64()
	return &types.VideoPricingSnapshot{
		PublicModel:             publicModel,
		ProfileID:               binding.Profile,
		ProfileHash:             profileHash,
		BillingMode:             profile.BillingMode,
		UnitPrice:               profile.UnitPrice,
		ReferenceVideoUnitPrice: profile.ReferenceVideoUnitPrice,
		ReferenceVideoApplied:   referenceVideoApplied,
		EffectiveUnitPrice:      effectiveUnitPrice.InexactFloat64(),
		Seconds:                 seconds,
		Basis:                   basis,
		Subtotal:                subtotalFloat,
		GroupRatio:              groupRatio,
		FinalQuota:              common.QuotaFromDecimalRound(quota),
		SubscriptionEnabled:     binding.SubscriptionEnabled,
	}, true, nil
}

func VideoPricingPriceData(snapshot *types.VideoPricingSnapshot, groupRatioInfo types.GroupRatioInfo) types.PriceData {
	copySnapshot := *snapshot
	priceData := types.PriceData{
		ModelPrice:     snapshot.Subtotal,
		UsePrice:       true,
		Quota:          snapshot.FinalQuota,
		GroupRatioInfo: groupRatioInfo,
		VideoPricing:   &copySnapshot,
	}
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume && snapshot.FinalQuota == 0 {
		priceData.FreeModel = true
	}
	return priceData
}
