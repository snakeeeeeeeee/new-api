package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const imageExecutionAuditContextKey = "image_execution_audit"

func imagePricingLogContent(snapshot *types.ImagePricingSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return fmt.Sprintf(
		"按张（图片）：%d 张 / %s，$%.6f/张 x %d x 分组倍率 %g = %s",
		snapshot.N,
		snapshot.EffectiveTier,
		snapshot.UnitPrice,
		snapshot.N,
		snapshot.GroupRatio,
		formatQuotaUSD(snapshot.FinalQuota),
	)
}

func appendImagePricingLogOther(other map[string]interface{}, snapshot *types.ImagePricingSnapshot) {
	if other == nil || snapshot == nil {
		return
	}
	cloned := *snapshot
	other["billing_type"] = types.ImagePricingBillingType
	other["image_pricing_snapshot"] = &cloned
}

// CaptureImageExecutionAuditFromJSON records returned execution facts only.
// Billing always remains anchored to the request-time ImagePricingSnapshot.
func CaptureImageExecutionAuditFromJSON(c *gin.Context, responseBody []byte, usage *dto.Usage) {
	if c == nil {
		return
	}
	audit := imageExecutionAuditFromJSON(responseBody)
	appendImageExecutionUsage(audit, usage)
	if len(audit) > 0 {
		c.Set(imageExecutionAuditContextKey, audit)
	}
}

func appendImageExecutionAuditFromContext(c *gin.Context, other map[string]interface{}) {
	if c == nil || other == nil {
		return
	}
	value, exists := c.Get(imageExecutionAuditContextKey)
	if !exists {
		return
	}
	audit, ok := value.(map[string]interface{})
	if !ok || len(audit) == 0 {
		return
	}
	other[imageExecutionAuditContextKey] = cloneImageExecutionAudit(audit)
}

func appendImageExecutionAuditFromTask(taskData []byte, usage *dto.Usage, other map[string]interface{}) bool {
	if other == nil {
		return false
	}
	audit := imageExecutionAuditFromJSON(taskData)
	appendImageExecutionUsage(audit, usage)
	if len(audit) == 0 {
		return false
	}
	other[imageExecutionAuditContextKey] = audit
	return true
}

func imageExecutionAuditFromJSON(data []byte) map[string]interface{} {
	audit := make(map[string]interface{})
	if len(data) == 0 {
		return audit
	}
	var payload map[string]interface{}
	if err := common.Unmarshal(data, &payload); err != nil {
		return audit
	}

	result := imageExecutionMap(payload["result"])
	output := imageExecutionMap(payload["output"])
	if nested := imageExecutionMap(result["output"]); len(nested) > 0 {
		output = nested
	}
	for _, key := range []string{"quality", "size", "resolution"} {
		if value := firstImageExecutionString(output[key], payload[key], result[key]); value != "" {
			audit[key] = value
		}
	}

	imageCount := imageExecutionArrayLength(payload["data"])
	if imageCount == 0 {
		imageCount = imageExecutionArrayLength(payload["images"])
	}
	if imageCount == 0 {
		imageCount = imageExecutionArrayLength(result["images"])
	}
	metadata := imageExecutionMap(result["metadata"])
	if imageCount == 0 {
		if value, ok := imageExecutionInt(metadata["image_count"]); ok && value > 0 {
			imageCount = value
		}
	}
	if imageCount > 0 {
		audit["image_count"] = imageCount
	}

	usageMap := imageExecutionMap(payload["usage"])
	for _, key := range []string{"total_tokens", "input_tokens", "output_tokens", "prompt_tokens", "completion_tokens", "actual_quota"} {
		if value, ok := imageExecutionInt(usageMap[key]); ok {
			audit[key] = value
		}
	}
	return audit
}

func appendImageExecutionUsage(audit map[string]interface{}, usage *dto.Usage) {
	if audit == nil || usage == nil {
		return
	}
	values := map[string]int{
		"total_tokens":      usage.TotalTokens,
		"input_tokens":      firstPositiveImageExecutionInt(usage.InputTokens, usage.PromptTokens),
		"output_tokens":     firstPositiveImageExecutionInt(usage.OutputTokens, usage.CompletionTokens),
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
	}
	for key, value := range values {
		if value > 0 {
			audit[key] = value
		}
	}
}

func imageExecutionAuditLogTokens(audit map[string]interface{}) (*int, *int) {
	return firstImageExecutionAuditToken(audit, "input_tokens", "prompt_tokens"),
		firstImageExecutionAuditToken(audit, "output_tokens", "completion_tokens")
}

func firstImageExecutionAuditToken(audit map[string]interface{}, keys ...string) *int {
	for _, key := range keys {
		value, exists := audit[key]
		if !exists {
			continue
		}
		if tokens, ok := imageExecutionInt(value); ok {
			return &tokens
		}
	}
	return nil
}

func cloneImageExecutionAudit(audit map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(audit))
	for key, value := range audit {
		cloned[key] = value
	}
	return cloned
}

func imageExecutionMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if result, ok := value.(map[string]interface{}); ok {
		return result
	}
	return map[string]interface{}{}
}

func imageExecutionArrayLength(value interface{}) int {
	if values, ok := value.([]interface{}); ok {
		return len(values)
	}
	return 0
}

func firstImageExecutionString(values ...interface{}) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func imageExecutionInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		if typed >= 0 {
			return typed, true
		}
	case int64:
		if typed >= 0 && typed <= int64(^uint(0)>>1) {
			return int(typed), true
		}
	case float64:
		maxInt := int64(^uint(0) >> 1)
		if typed >= 0 && !math.IsNaN(typed) && !math.IsInf(typed, 0) && typed == math.Trunc(typed) && typed <= float64(maxInt) {
			converted := int64(typed)
			if converted >= 0 {
				return int(converted), true
			}
		}
	}
	return 0, false
}

func firstPositiveImageExecutionInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
