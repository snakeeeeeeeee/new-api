package helper

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type ImagePricingRequestError struct {
	message string
}

func (e *ImagePricingRequestError) Error() string {
	return e.message
}

func IsImagePricingRequestError(err error) bool {
	_, ok := err.(*ImagePricingRequestError)
	return ok
}

func imagePricingError(format string, args ...any) error {
	return &ImagePricingRequestError{message: fmt.Sprintf(format, args...)}
}

type ImagePricingRequestParameters struct {
	Quality      string
	Size         string
	Resolution   string
	N            int
	ExplicitZero bool
}

func imagePricingParameterValue(parameter string, params ImagePricingRequestParameters) string {
	switch parameter {
	case types.ImagePricingParameterQuality:
		return params.Quality
	case types.ImagePricingParameterSize:
		return params.Size
	case types.ImagePricingParameterResolution:
		return params.Resolution
	default:
		return ""
	}
}

func resolveImagePricing(publicModel string, params ImagePricingRequestParameters, groupRatio float64) (*types.ImagePricingSnapshot, bool, error) {
	profile, binding, profileHash, bound := ratio_setting.GetImagePricingForModel(publicModel)
	if !bound {
		return nil, false, nil
	}
	if params.ExplicitZero || params.N <= 0 || params.N > binding.MaxN {
		return nil, true, imagePricingError("模型 %s 的 n 必须在 1 到 %d 之间", publicModel, binding.MaxN)
	}

	rawValue := strings.TrimSpace(imagePricingParameterValue(profile.Parameter, params))
	effectiveValue := rawValue
	valueSource := types.ImagePricingValueSourceRequest
	if effectiveValue == "" {
		effectiveValue = profile.DefaultTier
		valueSource = types.ImagePricingValueSourceDefault
	}

	var matched *types.ImagePricingTier
	for index := range profile.Tiers {
		tier := &profile.Tiers[index]
		if strings.EqualFold(effectiveValue, tier.Key) {
			matched = tier
			break
		}
		for _, alias := range tier.Aliases {
			if strings.EqualFold(effectiveValue, alias) {
				matched = tier
				valueSource = types.ImagePricingValueSourceAlias
				break
			}
		}
		if matched != nil {
			break
		}
	}
	if matched == nil {
		allowed := make([]string, 0, len(profile.Tiers))
		for _, tier := range profile.Tiers {
			allowed = append(allowed, tier.Key)
		}
		sort.Strings(allowed)
		return nil, true, imagePricingError("模型 %s 的 %s=%q 不受支持，允许值：%s", publicModel, profile.Parameter, rawValue, strings.Join(allowed, ", "))
	}

	subtotal := decimal.NewFromFloat(matched.UnitPrice).Mul(decimal.NewFromInt(int64(params.N)))
	quota := subtotal.
		Mul(decimal.NewFromFloat(groupRatio)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	subtotalFloat, _ := subtotal.Float64()
	return &types.ImagePricingSnapshot{
		PublicModel:   publicModel,
		ProfileID:     binding.Profile,
		ProfileHash:   profileHash,
		Parameter:     profile.Parameter,
		RawValue:      rawValue,
		EffectiveTier: matched.Key,
		UpstreamValue: matched.UpstreamValue,
		ValueSource:   valueSource,
		UnitPrice:     matched.UnitPrice,
		N:             params.N,
		Subtotal:      subtotalFloat,
		GroupRatio:    groupRatio,
		FinalQuota:    common.QuotaFromDecimalRound(quota),
	}, true, nil
}

func imageRequestResolution(request *dto.ImageRequest) (string, error) {
	if request == nil || request.Extra == nil {
		return "", nil
	}
	raw, exists := request.Extra[types.ImagePricingParameterResolution]
	if !exists || len(raw) == 0 {
		return "", nil
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", imagePricingError("resolution 必须是字符串")
	}
	return value, nil
}

func ResolveImageRequestPricing(request *dto.ImageRequest, publicModel string, groupRatio float64) (*types.ImagePricingSnapshot, bool, error) {
	if request == nil {
		return nil, false, nil
	}
	resolution, err := imageRequestResolution(request)
	if err != nil {
		return nil, ratio_setting.IsImageParameterPricedModel(publicModel), err
	}
	n := 1
	if request.N != nil {
		n = int(*request.N)
	}
	snapshot, bound, err := resolveImagePricing(publicModel, ImagePricingRequestParameters{
		Quality:      request.Quality,
		Size:         request.Size,
		Resolution:   resolution,
		N:            n,
		ExplicitZero: request.NExplicitZero,
	}, groupRatio)
	if err != nil || !bound {
		return snapshot, bound, err
	}

	normalizedN := uint(snapshot.N)
	request.N = &normalizedN
	switch snapshot.Parameter {
	case types.ImagePricingParameterQuality:
		request.Quality = snapshot.UpstreamValue
	case types.ImagePricingParameterSize:
		request.Size = snapshot.UpstreamValue
	case types.ImagePricingParameterResolution:
		if request.Extra == nil {
			request.Extra = map[string]json.RawMessage{}
		}
		value, marshalErr := common.Marshal(snapshot.UpstreamValue)
		if marshalErr != nil {
			return nil, true, marshalErr
		}
		request.Extra[types.ImagePricingParameterResolution] = value
	}
	return snapshot, true, nil
}

func taskMetadataString(metadata map[string]interface{}, key string) (string, error) {
	if metadata == nil {
		return "", nil
	}
	value, exists := metadata[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", imagePricingError("%s 必须是字符串", key)
	}
	return text, nil
}

func taskImagePricingString(metadata map[string]interface{}, key string, fallback *string) (string, error) {
	if metadata == nil {
		if fallback == nil {
			return "", nil
		}
		return *fallback, nil
	}
	if _, exists := metadata[key]; !exists {
		if fallback == nil {
			return "", nil
		}
		return *fallback, nil
	}
	return taskMetadataString(metadata, key)
}

func taskImagePricingN(metadata map[string]interface{}, fallback *int) (int, bool, error) {
	if metadata == nil {
		if fallback == nil {
			return 1, false, nil
		}
		return *fallback, *fallback == 0, nil
	}
	value, exists := metadata["n"]
	if !exists {
		if fallback == nil {
			return 1, false, nil
		}
		return *fallback, *fallback == 0, nil
	}
	if value == nil {
		return 1, false, nil
	}
	var n int64
	switch typed := value.(type) {
	case int:
		n = int64(typed)
	case int64:
		n = typed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) {
			return 0, false, imagePricingError("n 必须是整数")
		}
		n = int64(typed)
	case json.Number:
		parsed, err := strconv.ParseInt(typed.String(), 10, 64)
		if err != nil {
			return 0, false, imagePricingError("n 必须是整数")
		}
		n = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false, imagePricingError("n 必须是整数")
		}
		n = parsed
	default:
		return 0, false, imagePricingError("n 必须是整数")
	}
	return int(n), n == 0, nil
}

func ResolveTaskImagePricing(c *gin.Context, publicModel string, groupRatio float64) (*types.ImagePricingSnapshot, bool, error) {
	if !ratio_setting.IsImageParameterPricedModel(publicModel) {
		return nil, false, nil
	}
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, true, err
	}
	quality, err := taskImagePricingString(request.Metadata, types.ImagePricingParameterQuality, request.Quality)
	if err != nil {
		return nil, true, err
	}
	resolution, err := taskImagePricingString(request.Metadata, types.ImagePricingParameterResolution, request.Resolution)
	if err != nil {
		return nil, true, err
	}
	n, explicitZero, err := taskImagePricingN(request.Metadata, request.N)
	if err != nil {
		return nil, true, err
	}
	snapshot, _, err := resolveImagePricing(publicModel, ImagePricingRequestParameters{
		Quality:      quality,
		Size:         request.Size,
		Resolution:   resolution,
		N:            n,
		ExplicitZero: explicitZero,
	}, groupRatio)
	if err != nil {
		return nil, true, err
	}
	if request.Metadata == nil {
		request.Metadata = map[string]interface{}{}
	}
	normalizedN := snapshot.N
	request.N = &normalizedN
	request.Metadata["n"] = snapshot.N
	switch snapshot.Parameter {
	case types.ImagePricingParameterQuality:
		quality := snapshot.UpstreamValue
		request.Quality = &quality
		request.Metadata[types.ImagePricingParameterQuality] = snapshot.UpstreamValue
	case types.ImagePricingParameterSize:
		request.Size = snapshot.UpstreamValue
	case types.ImagePricingParameterResolution:
		resolution := snapshot.UpstreamValue
		request.Resolution = &resolution
		request.Metadata[types.ImagePricingParameterResolution] = snapshot.UpstreamValue
	}
	c.Set("task_request", request)
	return snapshot, true, nil
}

func ImagePricingPriceData(snapshot *types.ImagePricingSnapshot, groupRatioInfo types.GroupRatioInfo, perCall bool) types.PriceData {
	copySnapshot := *snapshot
	priceData := types.PriceData{
		ModelPrice:     snapshot.Subtotal,
		UsePrice:       true,
		GroupRatioInfo: groupRatioInfo,
		ImagePricing:   &copySnapshot,
	}
	if perCall {
		priceData.Quota = snapshot.FinalQuota
	} else {
		priceData.QuotaToPreConsume = snapshot.FinalQuota
	}
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume && snapshot.FinalQuota == 0 {
		priceData.FreeModel = true
	}
	return priceData
}
