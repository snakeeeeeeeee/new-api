package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func formatQuotaUSD(quota int) string {
	return fmt.Sprintf("$%.6f", quotaUSDValue(quota))
}

func quotaUSDValue(quota int) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) int {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	if info.PriceData.ImagePricing != nil {
		logContent = imagePricingLogContent(info.PriceData.ImagePricing)
	} else if c != nil && c.Request != nil && c.Request.URL != nil && c.Request.URL.Path == "/v1/image/tasks" {
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
	appendBillingInfo(info, other)
	appendImagePricingLogOther(other, info.PriceData.ImagePricing)
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	if info.TaskRelayInfo != nil && strings.TrimSpace(info.PublicTaskID) != "" {
		other["task_id"] = strings.TrimSpace(info.PublicTaskID)
	}
	consumeLog := model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
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
	if consumeLog == nil {
		return 0
	}
	return consumeLog.Id
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
		err = model.RefundTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
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
		if bc.HasSpecialRatio {
			other["user_group_ratio"] = bc.GroupSpecialRatio
		}
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
		appendImagePricingLogOther(other, bc.ImagePricing)
		if bc.HasRatioOverride {
			other["ratio_override"] = bc.RatioOverride
			other["has_ratio_override"] = true
			other["ratio_override_applied"] = bc.RatioOverrideApplied || !bc.HasRouteModelGroupRatio
		}
		if bc.HasRouteModelGroupRatio {
			other["route_model_group_ratio_applied"] = true
			other["route_model_group_ratio"] = bc.RouteModelGroupRatio
			other["route_model_ratio_aggregate_group"] = bc.RouteModelAggregateGroup
			other["route_model_ratio_real_group"] = bc.RouteModelRealGroup
			other["route_model_ratio_model_name"] = bc.RouteModelName
			other["route_model_group_ratio_source"] = bc.RouteModelRatioSource
		}
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
	}
	appendImageExecutionAuditFromTask(task.Data, nil, other)
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

// adjustTaskUsedQuota preserves legacy task accounting while making
// image-handle counters reflect the final settled quota instead of precharge.
func adjustTaskUsedQuota(task *model.Task, quotaDelta int) {
	if task == nil || quotaDelta == 0 {
		return
	}
	if isImageHandleTask(task) {
		model.UpdateUserUsedQuota(task.UserId, quotaDelta)
		model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
		return
	}
	if quotaDelta > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
		model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
	}
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
	adjustTaskUsedQuota(task, -quota)

	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	content := ""
	if task.PrivateData.BillingContext != nil {
		switch task.PrivateData.BillingContext.BillingMode {
		case types.ImagePricingBillingMode:
			content = fmt.Sprintf("异步图片按张计费失败，退还预扣费 %s", formatQuotaUSD(quota))
		case "async_image_usage_billing":
			content = fmt.Sprintf("异步图片失败，退还预扣费 %s", formatQuotaUSD(quota))
		}
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
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	adjustTaskUsedQuota(task, quotaDelta)
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
	actualQuota := common.QuotaFromFloat(float64(totalTokens) * modelRatio * finalGroupRatio)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f", totalTokens, modelRatio, finalGroupRatio)
	RecalculateTaskQuotaWithDebtOption(ctx, task, actualQuota, reason, allowDebt)
}

func settleAsyncImageBillingOnComplete(ctx context.Context, task *model.Task, taskResult *relaycommon.TaskInfo) bool {
	if !isImageHandleTask(task) {
		return false
	}
	bc := task.PrivateData.BillingContext
	if bc != nil && bc.BillingMode == types.ImagePricingBillingMode && bc.ImagePricing != nil {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 图片参数按张计费，成功后保持请求快照额度 %s", task.TaskID, formatQuotaUSD(task.Quota)))
		recordImagePricingExecutionAudit(task, taskResult)
		return true
	}
	if bc == nil || bc.PerCallBilling || bc.UsePrice {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，成功后保持预扣费 %s", task.TaskID, formatQuotaUSD(task.Quota)))
		return true
	}
	usage := taskResult.Usage
	if usage == nil && taskResult.TotalTokens > 0 {
		usage = &dto.Usage{
			PromptTokens:     taskResult.TotalTokens,
			CompletionTokens: 0,
			TotalTokens:      taskResult.TotalTokens,
			UsageSource:      "image_handle_total_tokens_fallback",
		}
	}
	if usage == nil || usage.TotalTokens <= 0 {
		if taskResult.ActualQuota > 0 {
			RecalculateTaskQuotaWithDebtOption(ctx, task, taskResult.ActualQuota, "image-handle actual_quota 兜底", true)
			return true
		}
		logger.LogWarn(ctx, fmt.Sprintf("任务 %s 成功但未返回 usage，保持预扣费 %s", task.TaskID, formatQuotaUSD(task.Quota)))
		return true
	}
	summary := calculateTextQuotaSummary(taskLogContext(ctx, task), taskRelayInfoForBilling(task), usage)
	reason := "异步图片按量真实结算"
	if summary.Quota > 0 {
		settleTaskQuotaDeltaWithUsage(ctx, task, summary, reason, true)
	}
	return true
}

func recordImagePricingExecutionAudit(task *model.Task, taskResult *relaycommon.TaskInfo) {
	if task == nil || task.PrivateData.BillingContext == nil || task.PrivateData.BillingContext.ImagePricing == nil {
		return
	}
	audit := imageExecutionAuditFromJSON(task.Data)
	if taskResult != nil {
		appendImageExecutionUsage(audit, taskResult.Usage)
		if taskResult.ActualQuota > 0 {
			audit["actual_quota"] = taskResult.ActualQuota
		}
	}
	if len(audit) == 0 {
		return
	}
	consumeLogId := task.PrivateData.BillingContext.ConsumeLogId
	if consumeLogId <= 0 && task.ID > 0 {
		var persistedTask model.Task
		if err := model.DB.Select("private_data").Where("id = ?", task.ID).First(&persistedTask).Error; err == nil &&
			persistedTask.PrivateData.BillingContext != nil {
			consumeLogId = persistedTask.PrivateData.BillingContext.ConsumeLogId
		}
	}
	promptTokens, completionTokens := imageExecutionAuditLogTokens(audit)
	merged, err := model.MergeConsumeLogOtherAndTokens(
		consumeLogId,
		task.UserId,
		task.TaskID,
		map[string]interface{}{imageExecutionAuditContextKey: audit},
		promptTokens,
		completionTokens,
	)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("合并图片执行审计失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	if !merged {
		logger.LogInfo(context.Background(), fmt.Sprintf("任务 %s 暂无可关联的消费日志，执行审计保留在任务结果中", task.TaskID))
	}
}

// MergeCompletedImagePricingExecutionAudit closes the race where image-handle
// completes before the submit request has persisted its consume log ID.
func MergeCompletedImagePricingExecutionAudit(taskId int64) {
	if taskId <= 0 {
		return
	}
	var task model.Task
	if err := model.DB.Where("id = ?", taskId).First(&task).Error; err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("读取图片任务审计信息失败 taskId=%d: %s", taskId, err.Error()))
		return
	}
	if task.Status != model.TaskStatusSuccess {
		return
	}
	recordImagePricingExecutionAudit(&task, nil)
}

func isImageHandleTask(task *model.Task) bool {
	return task != nil && task.Platform == constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeImageHandle))
}

func taskRelayInfoForBilling(task *model.Task) *relaycommon.RelayInfo {
	bc := task.PrivateData.BillingContext
	priceData := types.PriceData{
		ModelPrice:           -1,
		GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1, OriginalGroupRatio: 1, RatioOverride: -1, RouteModelGroupRatio: -1},
		CacheRatio:           1,
		CacheCreationRatio:   1,
		CacheCreation5mRatio: 1,
		CacheCreation1hRatio: 1,
		ImageRatio:           1,
	}
	if bc != nil {
		priceData.ModelPrice = bc.ModelPrice
		priceData.ModelRatio = bc.ModelRatio
		priceData.CompletionRatio = bc.CompletionRatio
		priceData.CacheRatio = defaultFloat(bc.CacheRatio, 1)
		priceData.CacheCreationRatio = defaultFloat(bc.CacheCreationRatio, 1)
		priceData.CacheCreation5mRatio = defaultFloat(bc.CacheCreation5mRatio, priceData.CacheCreationRatio)
		priceData.CacheCreation1hRatio = defaultFloat(bc.CacheCreation1hRatio, priceData.CacheCreationRatio)
		priceData.ImageRatio = defaultFloat(bc.ImageRatio, 1)
		priceData.UsePrice = bc.UsePrice
		priceData.OtherRatios = taskUsageBillingOtherRatios(bc.OtherRatios)
		priceData.GroupRatioInfo = types.GroupRatioInfo{
			GroupRatio:                    defaultFloat(bc.GroupRatio, 1),
			GroupSpecialRatio:             defaultFloat(bc.GroupSpecialRatio, -1),
			HasSpecialRatio:               bc.HasSpecialRatio,
			OriginalGroupRatio:            defaultFloat(bc.OriginalGroupRatio, defaultFloat(bc.GroupRatio, 1)),
			RatioOverride:                 defaultFloat(bc.RatioOverride, -1),
			HasRatioOverride:              bc.HasRatioOverride,
			RatioOverrideApplied:          bc.RatioOverrideApplied,
			RouteModelGroupRatio:          defaultFloat(bc.RouteModelGroupRatio, -1),
			HasRouteModelGroupRatio:       bc.HasRouteModelGroupRatio,
			RouteModelRatioAggregateGroup: bc.RouteModelAggregateGroup,
			RouteModelRatioRealGroup:      bc.RouteModelRealGroup,
			RouteModelRatioModelName:      bc.RouteModelName,
			RouteModelGroupRatioSource:    bc.RouteModelRatioSource,
		}
		if bc.HasRouteModelGroupRatio {
			priceData.GroupRatioInfo.GroupRatio = bc.GroupRatio
			priceData.GroupRatioInfo.OriginalGroupRatio = bc.OriginalGroupRatio
			priceData.GroupRatioInfo.RouteModelGroupRatio = bc.RouteModelGroupRatio
		}
		if bc.HasSpecialRatio {
			priceData.GroupRatioInfo.GroupSpecialRatio = bc.GroupSpecialRatio
		}
		if bc.HasRatioOverride {
			priceData.GroupRatioInfo.RatioOverride = bc.RatioOverride
			if !bc.HasRouteModelGroupRatio {
				priceData.GroupRatioInfo.RatioOverrideApplied = true
			}
		}
	}
	return &relaycommon.RelayInfo{
		UserId:          task.UserId,
		TokenId:         task.PrivateData.TokenId,
		UsingGroup:      task.Group,
		UserGroup:       task.Group,
		OriginModelName: taskModelName(task),
		RequestURLPath:  "/v1/image/tasks",
		StartTime:       time.Unix(defaultInt64(task.StartTime, task.SubmitTime, task.CreatedAt, time.Now().Unix()), 0),
		PriceData:       priceData,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: task.ChannelId},
	}
}

func taskLogContext(ctx context.Context, task *model.Task) *gin.Context {
	ginCtx, ok := ctx.(*gin.Context)
	if ok && ginCtx != nil {
		return ginCtx
	}
	c, _ := gin.CreateTestContext(noopResponseWriter{})
	req, _ := http.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	c.Request = req
	c.Set("token_name", taskTokenName(task))
	return c
}

type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header       { return http.Header{} }
func (noopResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (noopResponseWriter) WriteHeader(int)           {}

func taskTokenName(task *model.Task) string {
	if task == nil || task.PrivateData.TokenId <= 0 {
		return ""
	}
	if token, err := model.GetTokenById(task.PrivateData.TokenId); err == nil {
		return token.Name
	}
	return ""
}

func asyncImageUsageLogOther(ctx context.Context, task *model.Task, summary textQuotaSummary, preConsumedQuota int) map[string]interface{} {
	logCtx := taskLogContext(ctx, task)
	relayInfo := taskRelayInfoForBilling(task)
	other := GenerateTextOtherInfo(logCtx, relayInfo, summary.ModelRatio, summary.GroupRatio, summary.CompletionRatio, summary.CacheTokens, summary.CacheRatio, summary.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = summary.Quota
	other["billing_stage"] = "async_image_final"
	if summary.ImageTokens != 0 {
		other["image"] = true
		other["image_ratio"] = summary.ImageRatio
		other["image_output"] = summary.ImageTokens
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
	if cacheWriteTokens := cacheWriteTokensTotal(summary); cacheWriteTokens > 0 {
		other["cache_write_tokens"] = cacheWriteTokens
	}
	return other
}

func settleTaskQuotaDelta(ctx context.Context, task *model.Task, actualQuota int, reason string, allowDebt bool, recordLog bool) {
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota
	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）", task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))
	if err := taskAdjustFundingWithDebtOption(task, quotaDelta, allowDebt); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	taskAdjustTokenQuotaWithDebtOption(ctx, task, quotaDelta, allowDebt)
	task.Quota = actualQuota
	if task.ID > 0 {
		if err := model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("quota", actualQuota).Error; err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("更新任务实际额度失败 task %s: %s", task.TaskID, err.Error()))
		}
	}
	adjustTaskUsedQuota(task, quotaDelta)
	if !recordLog {
		return
	}
	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
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

func settleTaskQuotaDeltaWithUsage(ctx context.Context, task *model.Task, summary textQuotaSummary, reason string, allowDebt bool) {
	actualQuota := summary.Quota
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota
	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）", task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))
	if err := taskAdjustFundingWithDebtOption(task, quotaDelta, allowDebt); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
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
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	adjustTaskUsedQuota(task, quotaDelta)
	content := ""
	if quotaDelta > 0 {
		content = fmt.Sprintf("异步图片按量真实结算：实际 %s，预扣 %s，补扣 %s",
			formatQuotaUSD(actualQuota), formatQuotaUSD(preConsumedQuota), formatQuotaUSD(quotaDelta))
	} else {
		content = fmt.Sprintf("异步图片按量真实结算：实际 %s，预扣 %s，退还 %s",
			formatQuotaUSD(actualQuota), formatQuotaUSD(preConsumedQuota), formatQuotaUSD(-quotaDelta))
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:           task.UserId,
		LogType:          logType,
		Content:          content,
		ChannelId:        task.ChannelId,
		ModelName:        summary.ModelName,
		Quota:            logQuota,
		PromptTokens:     summary.PromptTokens,
		CompletionTokens: summary.CompletionTokens,
		TokenId:          task.PrivateData.TokenId,
		Group:            task.Group,
		UseTimeSeconds:   int(summary.UseTimeSeconds),
		Other:            asyncImageUsageLogOther(ctx, task, summary, preConsumedQuota),
	})
}

func defaultFloat(value float64, fallback float64) float64 {
	if value != 0 {
		return value
	}
	return fallback
}

func taskUsageBillingOtherRatios(otherRatios map[string]float64) map[string]float64 {
	if len(otherRatios) == 0 {
		return nil
	}
	filtered := make(map[string]float64)
	for key, value := range otherRatios {
		if strings.HasPrefix(key, "async_image_precharge_") || key == "async_image_n" {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func defaultInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
