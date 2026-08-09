package service

import (
	"context"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

const videoExecutionAuditKey = "video_execution_audit"

func videoPricingLogContent(snapshot *types.VideoPricingSnapshot) string {
	if snapshot == nil {
		return ""
	}
	effectiveUnitPrice := snapshot.EffectiveUnitPrice
	if effectiveUnitPrice == 0 && snapshot.UnitPrice > 0 {
		effectiveUnitPrice = snapshot.UnitPrice
	}
	if snapshot.ReferenceVideoApplied {
		return fmt.Sprintf(
			"按秒（视频）：$%.6f/秒（基础 $%.6f + 参考视频附加 $%.6f） x %d 秒 x 分组倍率 %g = %s",
			effectiveUnitPrice,
			snapshot.UnitPrice,
			snapshot.ReferenceVideoUnitPrice,
			snapshot.Seconds,
			snapshot.GroupRatio,
			formatQuotaUSD(snapshot.FinalQuota),
		)
	}
	return fmt.Sprintf(
		"按秒（视频）：$%.6f/秒 x %d 秒 x 分组倍率 %g = %s",
		effectiveUnitPrice,
		snapshot.Seconds,
		snapshot.GroupRatio,
		formatQuotaUSD(snapshot.FinalQuota),
	)
}

func appendVideoPricingLogOther(other map[string]interface{}, snapshot *types.VideoPricingSnapshot) {
	if other == nil || snapshot == nil {
		return
	}
	cloned := *snapshot
	other["billing_type"] = types.VideoPricingBillingType
	other["video_pricing_snapshot"] = &cloned
}

func recordVideoExecutionAudit(task *model.Task, taskResult *relaycommon.TaskInfo) {
	if task == nil || task.PrivateData.BillingContext == nil || task.PrivateData.BillingContext.VideoPricing == nil {
		return
	}
	durationMS := int64(0)
	if taskResult != nil && len(taskResult.VideoOutputs) > 0 {
		durationMS = taskResult.VideoOutputs[0].DurationMS
	}
	if durationMS <= 0 {
		durationMS = videoDurationMSFromTaskData(task.Data)
	}
	if durationMS <= 0 {
		return
	}
	snapshot := task.PrivateData.BillingContext.VideoPricing
	audit := map[string]interface{}{
		"requested_seconds":    snapshot.Seconds,
		"reported_duration_ms": durationMS,
		"matches_request":      durationMS == int64(snapshot.Seconds)*1000,
	}
	consumeLogId := task.PrivateData.BillingContext.ConsumeLogId
	if consumeLogId <= 0 && task.ID > 0 {
		var persisted model.Task
		if err := model.DB.Select("private_data").Where("id = ?", task.ID).First(&persisted).Error; err == nil && persisted.PrivateData.BillingContext != nil {
			consumeLogId = persisted.PrivateData.BillingContext.ConsumeLogId
		}
	}
	merged, err := model.MergeConsumeLogOther(consumeLogId, task.UserId, task.TaskID, map[string]interface{}{videoExecutionAuditKey: audit})
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("合并视频执行审计失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	if !merged {
		logger.LogInfo(context.Background(), fmt.Sprintf("任务 %s 暂无可关联的消费日志，视频时长保留在任务结果中", task.TaskID))
	}
}

func videoDurationMSFromTaskData(data []byte) int64 {
	if len(data) == 0 {
		return 0
	}
	var payload map[string]interface{}
	if err := common.Unmarshal(data, &payload); err != nil {
		return 0
	}
	for _, candidate := range []interface{}{
		videoMapValue(payload["video"], "duration"),
		payload["duration"],
		videoMapValue(payload["result"], "duration"),
		videoMapValue(payload["output"], "duration"),
	} {
		if seconds := positiveVideoDuration(candidate); seconds > 0 {
			return int64(math.Round(seconds * 1000))
		}
	}
	return 0
}

func videoMapValue(value interface{}, key string) interface{} {
	typed, _ := value.(map[string]interface{})
	return typed[key]
}

func positiveVideoDuration(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		if !math.IsNaN(typed) && !math.IsInf(typed, 0) && typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case int64:
		if typed > 0 {
			return float64(typed)
		}
	}
	return 0
}
