package service

import (
	"fmt"
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

func RefreshAggregateGroupRouteStrategyState(groupName string, modelName string, routeGroup string) (*AggregateGroupRouteStrategyState, bool, error) {
	state, err := GetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup)
	if err != nil || state == nil {
		return state, false, err
	}
	now := common.GetTimestamp()
	if state.DegradedUntil > 0 && state.DegradedUntil <= now {
		state.DegradedUntil = 0
		state.ConsecutiveFailures = 0
		state.ConsecutiveSlows = 0
		if setErr := SetAggregateGroupRouteStrategyState(groupName, modelName, routeGroup, state); setErr != nil {
			return nil, false, setErr
		}
		return state, true, nil
	}
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
	state.ConsecutiveFailures++
	state.ConsecutiveSlows = 0
	state.LastFailureAt = now

	triggered := false
	if setting.AggregateGroupFailureThreshold > 0 && state.ConsecutiveFailures >= setting.AggregateGroupFailureThreshold {
		state.ConsecutiveFailures = 0
		state.ConsecutiveSlows = 0
		state.DegradedUntil = now + int64(setting.AggregateGroupDegradeDurationSeconds)
		state.LastTriggerReason = AggregateSmartTriggerReasonConsecutiveFailures
		state.LastTriggerAt = now
		triggered = true
	}
	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy failure state failed: "+err.Error())
		return
	}
	if triggered {
		logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by consecutive failures: aggregate_group=%s, model=%s, route_group=%s, status_code=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, statusCode, state.DegradedUntil))
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
	state.ConsecutiveFailures = 0
	state.LastSuccessAt = now

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	if useTimeSeconds >= setting.AggregateGroupSlowRequestThreshold {
		state.ConsecutiveSlows++
		state.LastSlowAt = now
		triggered := false
		if setting.AggregateGroupConsecutiveSlowLimit > 0 && state.ConsecutiveSlows >= setting.AggregateGroupConsecutiveSlowLimit {
			state.ConsecutiveFailures = 0
			state.ConsecutiveSlows = 0
			state.DegradedUntil = now + int64(setting.AggregateGroupDegradeDurationSeconds)
			state.LastTriggerReason = AggregateSmartTriggerReasonConsecutiveSlows
			state.LastTriggerAt = now
			triggered = true
		}
		if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
			logger.LogError(c, "save aggregate smart strategy slow state failed: "+err.Error())
			return
		}
		if triggered {
			logger.LogWarn(c, fmt.Sprintf("aggregate smart degrade by consecutive slow requests: aggregate_group=%s, model=%s, route_group=%s, use_time_seconds=%d, degrade_until=%d", aggregateGroup, modelName, routeGroup, useTimeSeconds, state.DegradedUntil))
		}
		return
	}

	state.ConsecutiveSlows = 0
	if err = SetAggregateGroupRouteStrategyState(aggregateGroup, modelName, routeGroup, state); err != nil {
		logger.LogError(c, "save aggregate smart strategy success state failed: "+err.Error())
	}
}
