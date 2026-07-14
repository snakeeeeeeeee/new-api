package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func syncImagePricingSnapshot(finalQuota int) *types.ImagePricingSnapshot {
	return &types.ImagePricingSnapshot{
		PublicModel:   "public-image-count",
		ProfileID:     "sync-quality-v1",
		ProfileHash:   "frozen-sync-profile",
		Parameter:     types.ImagePricingParameterQuality,
		RawValue:      "",
		EffectiveTier: "low",
		UpstreamValue: "provider-low",
		ValueSource:   types.ImagePricingValueSourceDefault,
		UnitPrice:     0.04,
		N:             2,
		Subtotal:      0.08,
		GroupRatio:    1.25,
		FinalQuota:    finalQuota,
	}
}

func TestSyncImagePricingSettlesSnapshotQuotaWhenUpstreamReturnsNoUsage(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 210, 210, 210
	const initialQuota, finalQuota = 200000, 50000
	const tokenKey = "sk-sync-image-no-usage"
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, tokenKey, initialQuota)
	seedChannel(t, channelID)

	ctx := testGinContext()
	ctx.Set("username", "test_user")
	ctx.Set(common.RequestIdKey, "req-sync-image-no-usage")
	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		OriginModelName: "public-image-count",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		PriceData: types.PriceData{
			UsePrice:          true,
			ModelPrice:        0.08,
			QuotaToPreConsume: finalQuota,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1.25},
			ImagePricing:      syncImagePricingSnapshot(finalQuota),
		},
	}

	require.Nil(t, PreConsumeBilling(ctx, finalQuota, relayInfo))
	assert.Equal(t, initialQuota-finalQuota, getUserQuota(t, userID))
	assert.Equal(t, initialQuota-finalQuota, getTokenRemainQuota(t, tokenID))

	CaptureImageExecutionAuditFromJSON(ctx, []byte(`{
		"result":{
			"images":[{"url":"https://cdn.example.com/one.png"},{"url":"https://cdn.example.com/two.png"}],
			"output":{"quality":"high","size":"2048x2048","resolution":"2k"}
		},
		"usage":{"input_tokens":17,"output_tokens":23,"total_tokens":40,"actual_quota":999999}
	}`), nil)
	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{}, nil)

	assert.Equal(t, initialQuota-finalQuota, getUserQuota(t, userID))
	assert.Equal(t, initialQuota-finalQuota, getTokenRemainQuota(t, tokenID))
	logItem := getLastLog(t)
	require.NotNil(t, logItem)
	assert.Equal(t, model.LogTypeConsume, logItem.Type)
	assert.Equal(t, finalQuota, logItem.Quota)
	assert.Contains(t, logItem.Content, "按张（图片）")
	other, err := common.StrToMap(logItem.Other)
	require.NoError(t, err)
	assert.Equal(t, types.ImagePricingBillingType, other["billing_type"])
	require.NotNil(t, other["image_pricing_snapshot"])
	audit, ok := other["image_execution_audit"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "high", audit["quality"])
	assert.Equal(t, "2048x2048", audit["size"])
	assert.Equal(t, "2k", audit["resolution"])
	assert.Equal(t, float64(2), audit["image_count"])
	assert.Equal(t, float64(17), audit["input_tokens"])
	assert.Equal(t, float64(23), audit["output_tokens"])
	assert.Equal(t, float64(40), audit["total_tokens"])
	assert.Equal(t, float64(999999), audit["actual_quota"])
}

func TestSyncImagePricingFailureRefundsEntireSnapshotPrechargeOnce(t *testing.T) {
	truncate(t)
	const userID, tokenID = 211, 211
	const initialQuota, finalQuota = 200000, 50000
	const tokenKey = "sk-sync-image-refund"
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, tokenKey, initialQuota)

	ctx := testGinContext()
	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		OriginModelName: "public-image-count",
		UsingGroup:      "default",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{
			QuotaToPreConsume: finalQuota,
			ImagePricing:      syncImagePricingSnapshot(finalQuota),
		},
	}

	require.Nil(t, PreConsumeBilling(ctx, finalQuota, relayInfo))
	require.NotNil(t, relayInfo.Billing)
	assert.Equal(t, initialQuota-finalQuota, getUserQuota(t, userID))
	assert.Equal(t, initialQuota-finalQuota, getTokenRemainQuota(t, tokenID))

	relayInfo.Billing.Refund(ctx)
	relayInfo.Billing.Refund(ctx)
	require.Eventually(t, func() bool {
		return getUserQuota(t, userID) == initialQuota && getTokenRemainQuota(t, tokenID) == initialQuota
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assert.Equal(t, initialQuota, getTokenRemainQuota(t, tokenID))
}

func TestSyncImagePricingFailureRefundsUnlimitedTokenWithoutIncreasingRemainQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 212, 212
	const initialUserQuota, initialTokenRemain, finalQuota = 200000, 300000, 50000
	const tokenKey = "sk-sync-image-unlimited-refund"
	seedUser(t, userID, initialUserQuota)
	seedUnlimitedToken(t, tokenID, userID, tokenKey, initialTokenRemain)

	ctx := testGinContext()
	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		TokenUnlimited:  true,
		OriginModelName: "public-image-count",
		UsingGroup:      "default",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{
			QuotaToPreConsume: finalQuota,
			ImagePricing:      syncImagePricingSnapshot(finalQuota),
		},
	}

	require.Nil(t, PreConsumeBilling(ctx, finalQuota, relayInfo))
	require.NotNil(t, relayInfo.Billing)
	assert.Equal(t, initialUserQuota-finalQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, finalQuota, getTokenUsedQuota(t, tokenID))

	relayInfo.Billing.Refund(ctx)
	relayInfo.Billing.Refund(ctx)
	require.Eventually(t, func() bool {
		return getUserQuota(t, userID) == initialUserQuota &&
			getTokenRemainQuota(t, tokenID) == initialTokenRemain &&
			getTokenUsedQuota(t, tokenID) == 0
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
}
