package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func formatQuotaUSD(quota int) string {
	if common.QuotaPerUnit <= 0 {
		return "$0.000000"
	}
	return fmt.Sprintf("$%.6f", float64(quota)/common.QuotaPerUnit)
}

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	if c != nil && c.Request != nil && c.Request.URL != nil && c.Request.URL.Path == "/v1/image/tasks" {
		if amount, ok := info.PriceData.OtherRatios["async_image_precharge_amount_per_image_usd"]; ok && amount > 0 {
			n := info.PriceData.OtherRatios["async_image_n"]
			if n <= 0 {
				n = 1
			}
			logContent = fmt.Sprintf("异步图片预扣费：每张 $%.6f，数量 %.0f，合计 %s", amount, n, formatQuotaUSD(info.PriceData.Quota))
		} else {
			logContent = fmt.Sprintf("异步图片预扣费：%s", formatQuotaUSD(info.PriceData.Quota))
		}
	}
	// 支持任务仅按次计费
	if strings.HasPrefix(logContent, "异步图片预扣费") {
		// 已经写明预扣费和估算参数，避免再追加通用按次计费文案。
	} else if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) || info.ChannelType == constant.ChannelTypeXai {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		if len(info.PriceData.OtherRatios) > 0 {
			var contents []string
			for key, ra := range info.PriceData.OtherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
			if len(contents) > 0 {
				logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
			}
		}
	}
	other := make(map[string]interface{})
	other["request_path"] = c.Request.URL.Path
	other["model_price"] = info.PriceData.ModelPrice
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendGroupRatioOverrideInfo(info, other)
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
	model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	return taskAdjustFundingWithDebtOption(task, delta, false)
}

func taskAdjustFundingWithDebtOption(task *model.Task, delta int, allowDebt bool) error {
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		if allowDebt {
			return model.DecreaseUserQuotaAllowNegative(task.UserId, delta)
		}
		return model.DecreaseUserQuota(task.UserId, delta)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	taskAdjustTokenQuotaWithDebtOption(ctx, task, delta, false)
}

func taskAdjustTokenQuotaWithDebtOption(ctx context.Context, task *model.Task, delta int, allowDebt bool) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}
	var err error
	if delta > 0 {
		if allowDebt {
			err = model.DecreaseTokenQuotaAllowNegative(task.PrivateData.TokenId, tokenKey, delta)
		} else {
			err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
		}
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		other["group_ratio"] = bc.GroupRatio
		if bc.OriginalGroupRatio != 0 {
			other["original_group_ratio"] = bc.OriginalGroupRatio
			other["original_ratio"] = bc.OriginalGroupRatio
		}
		if bc.PrechargeAmountPerImage > 0 {
			other["precharge_amount_per_image_usd"] = bc.PrechargeAmountPerImage
		}
		if bc.PrechargePerImage > 0 {
			other["precharge_quota_per_image"] = bc.PrechargePerImage
		}
		if bc.ImageCount > 0 {
			other["image_count"] = bc.ImageCount
		}
		if bc.HasRatioOverride {
			other["ratio_override"] = bc.RatioOverride
			other["has_ratio_override"] = true
		}
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	return other
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，将预扣的 quota 退还给用户（支持钱包和订阅），并退还令牌额度。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) {
	quota := task.Quota
	if quota == 0 {
		return
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 2. 退还令牌额度
	taskAdjustTokenQuota(ctx, task, -quota)

	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	content := ""
	if task.PrivateData.BillingContext != nil && task.PrivateData.BillingContext.BillingMode == "async_image_usage_billing" {
		content = fmt.Sprintf("异步图片失败，退还预扣费 %s", formatQuotaUSD(quota))
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   content,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string) {
	RecalculateTaskQuotaWithDebtOption(ctx, task, actualQuota, reason, false)
}

// RecalculateTaskQuotaWithDebtOption 用于终态真实结算。
// allowDebt 只应在异步任务已成功拿到真实 usage 后开启，提交预扣不能使用。
func RecalculateTaskQuotaWithDebtOption(ctx context.Context, task *model.Task, actualQuota int, reason string, allowDebt bool) {
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// 调整资金来源
	if err := taskAdjustFundingWithDebtOption(task, quotaDelta, allowDebt); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 调整令牌额度
	taskAdjustTokenQuotaWithDebtOption(ctx, task, quotaDelta, allowDebt)

	task.Quota = actualQuota
	if task.ID > 0 {
		if err := model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("quota", actualQuota).Error; err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("更新任务实际额度失败 task %s: %s", task.TaskID, err.Error()))
		}
	}

	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
		model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
		model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	//other["reason"] = reason
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	content := reason
	if quotaDelta > 0 {
		content = fmt.Sprintf("异步任务真实结算：实际 %s，预扣 %s，补扣 %s（%s）",
			formatQuotaUSD(actualQuota), formatQuotaUSD(preConsumedQuota), formatQuotaUSD(quotaDelta), reason)
	} else {
		content = fmt.Sprintf("异步任务真实结算：实际 %s，预扣 %s，退还 %s（%s）",
			formatQuotaUSD(actualQuota), formatQuotaUSD(preConsumedQuota), formatQuotaUSD(-quotaDelta), reason)
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   content,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) {
	RecalculateTaskQuotaByTokensWithDebtOption(ctx, task, totalTokens, false)
}

func RecalculateTaskQuotaByTokensWithDebtOption(ctx context.Context, task *model.Task, totalTokens int, allowDebt bool) {
	if totalTokens <= 0 {
		return
	}

	modelName := taskModelName(task)

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return
	}

	var finalGroupRatio float64
	if bc := task.PrivateData.BillingContext; bc != nil {
		finalGroupRatio = bc.GroupRatio
	} else {
		group := task.Group
		if group == "" {
			user, err := model.GetUserById(task.UserId, false)
			if err == nil {
				group = user.Group
			}
		}
		if group == "" {
			return
		}
		groupRatio := ratio_setting.GetGroupRatio(group)
		userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)
		if hasUserGroupRatio {
			finalGroupRatio = userGroupRatio
		} else {
			finalGroupRatio = groupRatio
		}
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio
	actualQuota := int(float64(totalTokens) * modelRatio * finalGroupRatio)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f", totalTokens, modelRatio, finalGroupRatio)
	RecalculateTaskQuotaWithDebtOption(ctx, task, actualQuota, reason, allowDebt)
}
