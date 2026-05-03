package service

import (
	"fmt"
	"math"
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

func normalizeAggregateRouteStrategyState(state *AggregateGroupRouteStrategyState, now int64) {
	if state == nil {
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

func triggerAggregateRouteStrategyDegrade(state *AggregateGroupRouteStrategyState, now int64, reason string) {
	if state == nil {
		return
	}
	normalizeAggregateRouteStrategyState(state, now)
	if state.DegradeLevel < 0 {
		state.DegradeLevel = 0
	}
	state.DegradeLevel++
	state.DegradedUntil = now + int64(setting.AggregateGroupDegradeDurationSeconds)
	state.LastTriggerReason = reason
	state.LastTriggerAt = now
	switch reason {
	case AggregateSmartTriggerReasonConsecutiveFailures:
		state.DegradedConsecutiveFailures = 0
	case AggregateSmartTriggerReasonConsecutiveSlows:
		state.DegradedConsecutiveSlows = 0
	}
}

func getAggregateRouteSlowSignal(c *gin.Context) (bool, string, int) {
	if c == nil {
		return false, "", 0
	}
	if setting.AggregateGroupSlowFirstResponseThreshold > 0 &&
		common.GetContextKeyBool(c, constant.ContextKeyRelayIsStream) {
		firstResponseMs := common.GetContextKeyInt(c, constant.ContextKeyFirstResponseMs)
		if firstResponseMs >= setting.AggregateGroupSlowFirstResponseThreshold*1000 {
			return true, AggregateSmartSlowReasonFirstResponse, int(math.Ceil(float64(firstResponseMs) / 1000))
		}
	}
	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	if setting.AggregateGroupSlowRequestThreshold > 0 && useTimeSeconds >= setting.AggregateGroupSlowRequestThreshold {
		return true, AggregateSmartSlowReasonTotalTime, useTimeSeconds
	}
	return false, "", useTimeSeconds
}

func RefreshAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string) (*AggregateGroupRouteStrategyState, bool, error) {
	state, err := GetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup)
	if err != nil || state == nil {
		return state, false, err
	}
	now := common.GetTimestamp()
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
	now := common.GetTimestamp()
	state, err := GetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup)
	if err != nil {
		logger.LogError(c, "load aggregate smart strategy state failed: "+err.Error())
		return
	}
	if state == nil {
		state = &AggregateGroupRouteStrategyState{}
	}
	normalizeAggregateRouteStrategyState(state, now)
	if state.DegradedUntil > now {
		state.LastFailureAt = now
		state.DegradedConsecutiveFailures++
		state.ConsecutiveFailures = 0
		triggered := false
		if setting.AggregateGroupFailureThreshold > 0 && state.DegradedConsecutiveFailures >= setting.AggregateGroupFailureThreshold {
			triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonConsecutiveFailures)
			triggered = true
		}
		if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
			logger.LogError(c, "save aggregate smart strategy degraded failure state failed: "+err.Error())
		}
		if triggered {
			logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade level increased by consecutive failures: aggregate_group=%s, model=%s, route_group=%s, status_code=%d, degrade_level=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, statusCode, state.DegradeLevel, state.DegradedUntil))
		}
		return
	}
	state.ConsecutiveFailures++
	state.ConsecutiveSlows = 0
	state.LastFailureAt = now

	triggered := false
	if setting.AggregateGroupFailureThreshold > 0 && state.ConsecutiveFailures >= setting.AggregateGroupFailureThreshold {
		triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonConsecutiveFailures)
		triggered = true
	}
	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy failure state failed: "+err.Error())
		return
	}
	if triggered {
		logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by consecutive failures: aggregate_group=%s, model=%s, route_group=%s, status_code=%d, degrade_level=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, statusCode, state.DegradeLevel, state.DegradedUntil))
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
	now := common.GetTimestamp()
	state, err := GetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup)
	if err != nil {
		logger.LogError(c, "load aggregate smart strategy success state failed: "+err.Error())
		return
	}
	if state == nil {
		state = &AggregateGroupRouteStrategyState{}
	}
	normalizeAggregateRouteStrategyState(state, now)
	isSlow, slowReason, slowSeconds := getAggregateRouteSlowSignal(c)
	if state.DegradedUntil > now {
		state.LastSuccessAt = now
		if isSlow {
			state.LastSlowAt = now
			state.LastSlowReason = slowReason
			state.DegradedConsecutiveSlows++
			triggered := false
			if setting.AggregateGroupConsecutiveSlowLimit > 0 && state.DegradedConsecutiveSlows >= setting.AggregateGroupConsecutiveSlowLimit {
				triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonConsecutiveSlows)
				triggered = true
			}
			if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
				logger.LogError(c, "save aggregate smart strategy degraded slow success state failed: "+err.Error())
			}
			if triggered {
				logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade level increased by consecutive slow requests: aggregate_group=%s, model=%s, route_group=%s, slow_reason=%s, slow_seconds=%d, degrade_level=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, slowReason, slowSeconds, state.DegradeLevel, state.DegradedUntil))
			}
			return
		}
		if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
			logger.LogError(c, "save aggregate smart strategy degraded success state failed: "+err.Error())
		}
		return
	}
	state.ConsecutiveFailures = 0
	state.LastSuccessAt = now

	if isSlow {
		state.ConsecutiveSlows++
		state.LastSlowAt = now
		state.LastSlowReason = slowReason
		triggered := false
		if setting.AggregateGroupConsecutiveSlowLimit > 0 && state.ConsecutiveSlows >= setting.AggregateGroupConsecutiveSlowLimit {
			triggerAggregateRouteStrategyDegrade(state, now, AggregateSmartTriggerReasonConsecutiveSlows)
			triggered = true
		}
		if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
			logger.LogError(c, "save aggregate smart strategy slow state failed: "+err.Error())
			return
		}
		if triggered {
			logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by consecutive slow requests: aggregate_group=%s, model=%s, route_group=%s, slow_reason=%s, slow_seconds=%d, degrade_level=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, slowReason, slowSeconds, state.DegradeLevel, state.DegradedUntil))
		}
		return
	}

	state.ConsecutiveSlows = 0
	state.LastSlowReason = ""
	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy success state failed: "+err.Error())
	}
}
