package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestVideoPricingConsumptionLogPersistsSnapshotAndAuditOnlyDuration(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 220, 220, 220
	const finalQuota = 270000
	const taskID = "task_video_pricing_audit"
	seedUser(t, userID, 1000000)
	seedToken(t, tokenID, userID, "sk-video-pricing-log", 1000000)
	seedChannel(t, channelID)

	snapshot := &types.VideoPricingSnapshot{
		PublicModel:             "seedance-1.5-pro-720p",
		ProfileID:               "seedance-720p",
		ProfileHash:             "frozen-video-profile",
		BillingMode:             types.VideoPricingModePerSecond,
		UnitPrice:               0.03,
		ReferenceVideoUnitPrice: 0.02,
		ReferenceVideoApplied:   true,
		EffectiveUnitPrice:      0.05,
		Seconds:                 6,
		Basis:                   types.VideoPricingBasisGeneration,
		Subtotal:                0.18,
		GroupRatio:              1.5,
		FinalQuota:              finalQuota,
		SubscriptionEnabled:     false,
	}
	ctx := testGinContext()
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/tasks", nil)
	ctx.Set("username", "test_user")
	ctx.Set("token_name", "test_token")
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		OriginModelName: snapshot.PublicModel,
		UsingGroup:      "default",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action:       constant.TaskActionVideoGeneration,
			PublicTaskID: taskID,
		},
		PriceData: types.PriceData{
			UsePrice:       true,
			Quota:          finalQuota,
			VideoPricing:   snapshot,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.5},
		},
	}

	consumeLogID := LogTaskConsumption(ctx, info)
	require.Positive(t, consumeLogID)
	var before model.Log
	require.NoError(t, model.LOG_DB.First(&before, consumeLogID).Error)
	require.Equal(t, finalQuota, before.Quota)
	require.Contains(t, before.Content, "按秒（视频）")
	require.Contains(t, before.Content, "基础 $0.030000 + 参考视频附加 $0.020000")
	require.NotContains(t, before.Content, "按次计费")
	beforeOther, err := common.StrToMap(before.Other)
	require.NoError(t, err)
	require.Equal(t, types.VideoPricingBillingType, beforeOther["billing_type"])
	require.NotNil(t, beforeOther["video_pricing_snapshot"])
	pricingSnapshot := beforeOther["video_pricing_snapshot"].(map[string]interface{})
	require.Equal(t, true, pricingSnapshot["reference_video_applied"])
	require.Equal(t, 0.05, pricingSnapshot["effective_unit_price"])
	require.Nil(t, beforeOther["video_execution_audit"])

	task := &model.Task{
		TaskID: taskID,
		UserId: userID,
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			ConsumeLogId: consumeLogID,
			VideoPricing: snapshot,
		}},
	}
	recordVideoExecutionAudit(task, &relaycommon.TaskInfo{VideoOutputs: []relaycommon.VideoOutput{{DurationMS: 5800}}})

	var after model.Log
	require.NoError(t, model.LOG_DB.First(&after, consumeLogID).Error)
	require.Equal(t, finalQuota, after.Quota, "reported duration must not reprice the log")
	afterOther, err := common.StrToMap(after.Other)
	require.NoError(t, err)
	audit, ok := afterOther["video_execution_audit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(6), audit["requested_seconds"])
	require.Equal(t, float64(5800), audit["reported_duration_ms"])
	require.Equal(t, false, audit["matches_request"])
}
