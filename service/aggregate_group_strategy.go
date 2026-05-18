package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

func IsAggregateSmartRoutingEnabled(aggregateGroup *model.AggregateGroup) bool {
	return aggregateGroup != nil && aggregateGroup.SmartRoutingEnabled && setting.AggregateGroupSmartStrategyEnabled
}

func IsAggregateSmartRoutingEnabledForContext(ctx *gin.Context) bool {
	if ctx == nil {
		return false
	}
	return common.GetContextKeyBool(ctx, constant.ContextKeyAggregateSmartRouting)
}

type AggregateGroupEffectiveSmartStrategy struct {
	Source                       string `json:"source"`
	FailureRateWindowSeconds     int    `json:"failure_rate_window_seconds"`
	FailureRateMinRequests       int    `json:"failure_rate_min_requests"`
	FailureRateThresholdPct      int    `json:"failure_rate_threshold_percent"`
	SlowRateWindowSeconds        int    `json:"slow_rate_window_seconds"`
	SlowRateMinRequests          int    `json:"slow_rate_min_requests"`
	SlowRateThresholdPct         int    `json:"slow_rate_threshold_percent"`
	DegradeDurationSeconds       int    `json:"degrade_duration_seconds"`
	ClusterDegradedWeightPercent int    `json:"cluster_degraded_weight_percent"`
	SlowRequestThresholdSeconds  int    `json:"slow_request_threshold_seconds"`
	SlowFirstResponseSeconds     int    `json:"slow_first_response_threshold_seconds"`
}

const (
	AggregateSmartStrategySourceGlobal = "global"
	AggregateSmartStrategySourceGroup  = "group"
)

func GetAggregateGroupEffectiveSmartStrategy(group *model.AggregateGroup) AggregateGroupEffectiveSmartStrategy {
	strategy := AggregateGroupEffectiveSmartStrategy{
		Source:                       AggregateSmartStrategySourceGlobal,
		FailureRateWindowSeconds:     setting.AggregateGroupFailureRateWindowSeconds,
		FailureRateMinRequests:       setting.AggregateGroupFailureRateMinRequests,
		FailureRateThresholdPct:      setting.AggregateGroupFailureRateThresholdPct,
		SlowRateWindowSeconds:        setting.AggregateGroupSlowRateWindowSeconds,
		SlowRateMinRequests:          setting.AggregateGroupSlowRateMinRequests,
		SlowRateThresholdPct:         setting.AggregateGroupSlowRateThresholdPct,
		DegradeDurationSeconds:       setting.AggregateGroupDegradeDurationSeconds,
		ClusterDegradedWeightPercent: setting.AggregateGroupClusterDegradedWeightPct,
		SlowRequestThresholdSeconds:  setting.AggregateGroupSlowRequestThreshold,
		SlowFirstResponseSeconds:     setting.AggregateGroupSlowFirstResponseThreshold,
	}
	if group != nil {
		if config := group.GetSmartStrategyConfig(); config != nil {
			strategy.Source = AggregateSmartStrategySourceGroup
			if config.FailureRateWindowSeconds != nil {
				strategy.FailureRateWindowSeconds = *config.FailureRateWindowSeconds
			}
			if config.FailureRateMinRequests != nil {
				strategy.FailureRateMinRequests = *config.FailureRateMinRequests
			}
			if config.FailureRateThresholdPct != nil {
				strategy.FailureRateThresholdPct = *config.FailureRateThresholdPct
			}
			if config.SlowRateWindowSeconds != nil {
				strategy.SlowRateWindowSeconds = *config.SlowRateWindowSeconds
			}
			if config.SlowRateMinRequests != nil {
				strategy.SlowRateMinRequests = *config.SlowRateMinRequests
			}
			if config.SlowRateThresholdPct != nil {
				strategy.SlowRateThresholdPct = *config.SlowRateThresholdPct
			}
			if config.DegradeDurationSeconds != nil {
				strategy.DegradeDurationSeconds = *config.DegradeDurationSeconds
			}
			if config.ClusterDegradedWeightPct != nil {
				strategy.ClusterDegradedWeightPercent = *config.ClusterDegradedWeightPct
			}
			if config.SlowRequestThreshold != nil {
				strategy.SlowRequestThresholdSeconds = *config.SlowRequestThreshold
			}
			if config.SlowFirstResponseThreshold != nil {
				strategy.SlowFirstResponseSeconds = *config.SlowFirstResponseThreshold
			}
		}
	}
	strategy.FailureRateWindowSeconds = setting.NormalizeAggregateGroupRateWindowSeconds(strategy.FailureRateWindowSeconds)
	strategy.FailureRateMinRequests = setting.NormalizeAggregateGroupRateMinRequests(strategy.FailureRateMinRequests, setting.DefaultAggregateGroupFailureRateMinRequests)
	strategy.FailureRateThresholdPct = setting.NormalizeAggregateGroupRateThresholdPercent(strategy.FailureRateThresholdPct, setting.DefaultAggregateGroupFailureRateThresholdPct)
	strategy.SlowRateWindowSeconds = setting.NormalizeAggregateGroupRateWindowSeconds(strategy.SlowRateWindowSeconds)
	strategy.SlowRateMinRequests = setting.NormalizeAggregateGroupRateMinRequests(strategy.SlowRateMinRequests, setting.DefaultAggregateGroupSlowRateMinRequests)
	strategy.SlowRateThresholdPct = setting.NormalizeAggregateGroupRateThresholdPercent(strategy.SlowRateThresholdPct, setting.DefaultAggregateGroupSlowRateThresholdPct)
	strategy.DegradeDurationSeconds = setting.NormalizeAggregateGroupDegradeDurationSeconds(strategy.DegradeDurationSeconds)
	strategy.ClusterDegradedWeightPercent = setting.NormalizeAggregateGroupClusterDegradedWeightPercent(strategy.ClusterDegradedWeightPercent)
	strategy.SlowRequestThresholdSeconds = setting.NormalizeAggregateGroupSlowRequestThreshold(strategy.SlowRequestThresholdSeconds)
	strategy.SlowFirstResponseSeconds = setting.NormalizeAggregateGroupSlowFirstResponseThreshold(strategy.SlowFirstResponseSeconds)
	return strategy
}

func getAggregateEffectiveSmartStrategyByName(groupName string) (AggregateGroupEffectiveSmartStrategy, *model.AggregateGroup) {
	group, _ := GetAggregateGroup(groupName, false)
	return GetAggregateGroupEffectiveSmartStrategy(group), group
}

func getAggregateEffectiveSmartStrategyFromContext(c *gin.Context, groupName string) AggregateGroupEffectiveSmartStrategy {
	if c != nil {
		if strategy, ok := common.GetContextKeyType[AggregateGroupEffectiveSmartStrategy](c, constant.ContextKeyAggregateSmartStrategy); ok {
			return strategy
		}
	}
	strategy, _ := getAggregateEffectiveSmartStrategyByName(groupName)
	return strategy
}

func normalizeAggregateRouteStrategyState(state *AggregateGroupRouteStrategyState, now int64) {
	if state == nil {
		return
	}
	if state.StrategyVersion != aggregateGroupRouteStrategyVersion {
		*state = AggregateGroupRouteStrategyState{StrategyVersion: aggregateGroupRouteStrategyVersion}
		return
	}
	if state.DegradedUntil > now {
		if state.DegradeLevel <= 0 {
			state.DegradeLevel = 1
		}
		return
	}
	if state.DegradedUntil > 0 {
		state.DegradedUntil = 0
	}
	state.DegradeLevel = 0
	state.DegradedConsecutiveFailures = 0
	state.DegradedConsecutiveSlows = 0
}

func triggerAggregateRouteStrategyDegrade(state *AggregateGroupRouteStrategyState, now int64, reason string, strategy AggregateGroupEffectiveSmartStrategy) {
	if state == nil {
		return
	}
	normalizeAggregateRouteStrategyState(state, now)
	state.StrategyVersion = aggregateGroupRouteStrategyVersion
	state.DegradeLevel = 1
	state.DegradedUntil = now + int64(strategy.DegradeDurationSeconds)
	state.LastTriggerReason = reason
	state.LastTriggerAt = now
	state.ConsecutiveFailures = 0
	state.ConsecutiveSlows = 0
	state.DegradedConsecutiveFailures = 0
	state.DegradedConsecutiveSlows = 0
	switch reason {
	case AggregateSmartTriggerReasonConsecutiveFailures:
		state.DegradedConsecutiveFailures = 0
	case AggregateSmartTriggerReasonConsecutiveSlows:
		state.DegradedConsecutiveSlows = 0
	}
}

func getAggregateRouteSlowSignal(c *gin.Context, strategy AggregateGroupEffectiveSmartStrategy) (bool, string, int) {
	if c == nil {
		return false, "", 0
	}
	if strategy.SlowFirstResponseSeconds > 0 &&
		common.GetContextKeyBool(c, constant.ContextKeyRelayIsStream) {
		firstResponseMs := common.GetContextKeyInt(c, constant.ContextKeyFirstResponseMs)
		if firstResponseMs >= strategy.SlowFirstResponseSeconds*1000 {
			return true, AggregateSmartSlowReasonFirstResponse, int(math.Ceil(float64(firstResponseMs) / 1000))
		}
	}
	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	if strategy.SlowRequestThresholdSeconds > 0 && useTimeSeconds >= strategy.SlowRequestThresholdSeconds {
		return true, AggregateSmartSlowReasonTotalTime, useTimeSeconds
	}
	return false, "", useTimeSeconds
}

func calculateAggregateRatePercent(part int, total int) int {
	if total <= 0 || part <= 0 {
		return 0
	}
	return int(math.Ceil(float64(part) * 100 / float64(total)))
}

func RefreshAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string) (*AggregateGroupRouteStrategyState, bool, error) {
	state, err := GetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup)
	if err != nil || state == nil {
		return state, false, err
	}
	now := common.GetTimestamp()
	if state.StrategyVersion != aggregateGroupRouteStrategyVersion {
		normalizeAggregateRouteStrategyState(state, now)
		if setErr := SetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup, state); setErr != nil {
			return nil, false, setErr
		}
		return state, true, nil
	}
	if state.DegradedUntil > 0 && state.DegradedUntil <= now {
		normalizeAggregateRouteStrategyState(state, now)
		state.ConsecutiveFailures = 0
		state.ConsecutiveSlows = 0
		if setErr := SetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup, state); setErr != nil {
			return nil, false, setErr
		}
		return state, true, nil
	}
	normalizeAggregateRouteStrategyState(state, now)
	return state, false, nil
}

func IsAggregateGroupRouteTemporarilyDegraded(groupName string, modelName string, routeGroup string) (bool, *AggregateGroupRouteStrategyState, bool, error) {
	state, recovered, err := RefreshAggregateGroupRouteStrategyState(groupName, modelName, routeGroup)
	if err != nil || state == nil {
		return false, state, recovered, err
	}
	return state.DegradedUntil > common.GetTimestamp(), state, recovered, nil
}

func RecordAggregateRouteSmartFailure(c *gin.Context, modelName string, routeGroup string, statusCode int) {
	if c == nil || modelName == "" || routeGroup == "" || !IsAggregateSmartRoutingEnabledForContext(c) {
		return
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	if aggregateGroup == "" {
		return
	}
	strategy := getAggregateEffectiveSmartStrategyFromContext(c, aggregateGroup)
	RecordAggregateRouteStrategyFailure(c, modelName, routeGroup)
	now := common.GetTimestamp()
	state, err := GetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup)
	if err != nil {
		logger.LogError(c, "load aggregate smart strategy state failed: "+err.Error())
		return
	}
	if state == nil {
		state = &AggregateGroupRouteStrategyState{StrategyVersion: aggregateGroupRouteStrategyVersion}
	}
	normalizeAggregateRouteStrategyState(state, now)
	state.LastFailureAt = now
	routePool := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool)
	stats := GetAggregateRouteWindowStatsForPool(aggregateGroup, modelName, routePool, routeGroup, strategy.FailureRateWindowSeconds)
	failureRate := calculateAggregateRatePercent(stats.StrategyFailures, stats.Attempts)
	state.LastFailureRate = failureRate
	state.LastWindowRequests = stats.Attempts
	state.LastWindowFailures = stats.StrategyFailures
	triggered := stats.Attempts >= strategy.FailureRateMinRequests && failureRate >= strategy.FailureRateThresholdPct
	if triggered {
		triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonFailureRate, strategy)
	}
	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy failure state failed: "+err.Error())
		return
	}
	if triggered {
		logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by failure rate: aggregate_group=%s, model=%s, route_group=%s, status_code=%d, attempts=%d, strategy_failures=%d, failure_rate=%d%%, threshold=%d%%, degrade_until=%d", aggregateGroup, modelName, routeGroup, statusCode, stats.Attempts, stats.StrategyFailures, failureRate, strategy.FailureRateThresholdPct, state.DegradedUntil))
	}
}

func RecordAggregateRouteSmartSuccess(c *gin.Context, modelName string, routeGroup string) {
	if c == nil || modelName == "" || routeGroup == "" || !IsAggregateSmartRoutingEnabledForContext(c) {
		return
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	if aggregateGroup == "" {
		return
	}
	strategy := getAggregateEffectiveSmartStrategyFromContext(c, aggregateGroup)
	now := common.GetTimestamp()
	state, err := GetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup)
	if err != nil {
		logger.LogError(c, "load aggregate smart strategy success state failed: "+err.Error())
		return
	}
	if state == nil {
		state = &AggregateGroupRouteStrategyState{StrategyVersion: aggregateGroupRouteStrategyVersion}
	}
	normalizeAggregateRouteStrategyState(state, now)
	isSlow, slowReason, slowSeconds := getAggregateRouteSlowSignal(c, strategy)
	state.LastSuccessAt = now

	if isSlow {
		RecordAggregateRouteSlowSuccess(c, modelName, routeGroup)
		state.LastSlowAt = now
		state.LastSlowReason = slowReason
		routePool := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool)
		stats := GetAggregateRouteWindowStatsForPool(aggregateGroup, modelName, routePool, routeGroup, strategy.SlowRateWindowSeconds)
		slowRate := calculateAggregateRatePercent(stats.SlowSuccesses, stats.Successes)
		state.LastSlowRate = slowRate
		state.LastWindowRequests = stats.Successes
		state.LastWindowSlowRequests = stats.SlowSuccesses
		triggered := stats.Successes >= strategy.SlowRateMinRequests && slowRate >= strategy.SlowRateThresholdPct
		if triggered {
			triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonSlowRate, strategy)
		}
		if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
			logger.LogError(c, "save aggregate smart strategy slow state failed: "+err.Error())
			return
		}
		if triggered {
			logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by slow rate: aggregate_group=%s, model=%s, route_group=%s, slow_reason=%s, slow_seconds=%d, successes=%d, slow_successes=%d, slow_rate=%d%%, threshold=%d%%, degrade_until=%d", aggregateGroup, modelName, routeGroup, slowReason, slowSeconds, stats.Successes, stats.SlowSuccesses, slowRate, strategy.SlowRateThresholdPct, state.DegradedUntil))
		}
		return
	}

	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy success state failed: "+err.Error())
	}
}

func NormalizeAndValidateAggregateSmartStrategyConfig(config *model.AggregateGroupSmartStrategyConfig) (*model.AggregateGroupSmartStrategyConfig, error) {
	if config == nil {
		return nil, nil
	}
	normalized := *config
	validatePositive := func(name string, value *int) error {
		if value != nil && *value <= 0 {
			return fmt.Errorf("%s 必须大于 0", name)
		}
		return nil
	}
	validatePercent := func(name string, value *int) error {
		if value != nil && (*value <= 0 || *value > 100) {
			return fmt.Errorf("%s 必须在 1 到 100 之间", name)
		}
		return nil
	}
	validateWindow := func(name string, value *int) error {
		if value != nil && (*value <= 0 || *value > setting.MaxAggregateGroupRateWindowSeconds) {
			return fmt.Errorf("%s 必须在 1 到 3600 秒之间", name)
		}
		return nil
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"错误率统计窗口", normalized.FailureRateWindowSeconds},
		{"慢率统计窗口", normalized.SlowRateWindowSeconds},
	} {
		if err := validateWindow(item.name, item.value); err != nil {
			return nil, err
		}
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"错误率最小样本数", normalized.FailureRateMinRequests},
		{"慢率最小样本数", normalized.SlowRateMinRequests},
		{"临时降级时长", normalized.DegradeDurationSeconds},
		{"慢请求阈值", normalized.SlowRequestThreshold},
	} {
		if err := validatePositive(item.name, item.value); err != nil {
			return nil, err
		}
	}
	if normalized.SlowFirstResponseThreshold != nil && *normalized.SlowFirstResponseThreshold < 0 {
		return nil, fmt.Errorf("首字慢阈值必须大于等于 0")
	}
	for _, item := range []struct {
		name  string
		value *int
	}{
		{"错误率阈值", normalized.FailureRateThresholdPct},
		{"慢率阈值", normalized.SlowRateThresholdPct},
		{"Cluster 降级有效权重比例", normalized.ClusterDegradedWeightPct},
	} {
		if err := validatePercent(item.name, item.value); err != nil {
			return nil, err
		}
	}
	if configHasNoAggregateSmartStrategyOverride(normalized) {
		return nil, nil
	}
	return &normalized, nil
}

func configHasNoAggregateSmartStrategyOverride(config model.AggregateGroupSmartStrategyConfig) bool {
	return config.FailureRateWindowSeconds == nil &&
		config.FailureRateMinRequests == nil &&
		config.FailureRateThresholdPct == nil &&
		config.SlowRateWindowSeconds == nil &&
		config.SlowRateMinRequests == nil &&
		config.SlowRateThresholdPct == nil &&
		config.DegradeDurationSeconds == nil &&
		config.ClusterDegradedWeightPct == nil &&
		config.SlowRequestThreshold == nil &&
		config.SlowFirstResponseThreshold == nil
}

func formatAggregateSmartStrategySource(strategy AggregateGroupEffectiveSmartStrategy) string {
	if strings.TrimSpace(strategy.Source) == "" {
		return AggregateSmartStrategySourceGlobal
	}
	return strategy.Source
}
