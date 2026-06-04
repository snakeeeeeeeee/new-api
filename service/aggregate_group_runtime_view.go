package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type AggregateGroupRuntimeActiveRouteView struct {
	ActiveIndex   int    `json:"active_index"`
	ActiveGroup   string `json:"active_group"`
	LastFailAt    int64  `json:"last_fail_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	LastSwitchAt  int64  `json:"last_switch_at"`
	ActiveSinceAt int64  `json:"active_since_at"`
}

type AggregateGroupRuntimeRouteView struct {
	RouteGroup                  string `json:"route_group"`
	RouteIndex                  int    `json:"route_index"`
	RoutePool                   string `json:"route_pool,omitempty"`
	Weight                      int    `json:"weight"`
	EffectiveWeight             int    `json:"effective_weight"`
	IsActive                    bool   `json:"is_active"`
	IsDegraded                  bool   `json:"is_degraded"`
	IsSoftFallback              bool   `json:"is_soft_fallback"`
	PriorityCount               int    `json:"priority_count"`
	RPM                         int    `json:"rpm"`
	TotalRPM                    int    `json:"total_rpm"`
	RPMLimit                    int    `json:"rpm_limit"`
	RPMLimited                  bool   `json:"rpm_limited"`
	SuccessRPM                  int    `json:"success_rpm"`
	FailureRPM                  int    `json:"failure_rpm"`
	StrategyFailureRPM          int    `json:"strategy_failure_rpm"`
	SlowSuccessRPM              int    `json:"slow_success_rpm"`
	FailureWindowSeconds        int    `json:"failure_window_seconds"`
	FailureWindowRequests       int    `json:"failure_window_requests"`
	FailureWindowFailures       int    `json:"failure_window_failures"`
	FailureRatePercent          int    `json:"failure_rate_percent"`
	FailureRateMinRequests      int    `json:"failure_rate_min_requests"`
	FailureRateThresholdPercent int    `json:"failure_rate_threshold_percent"`
	SlowWindowSeconds           int    `json:"slow_window_seconds"`
	SlowWindowSuccesses         int    `json:"slow_window_successes"`
	SlowWindowSlowSuccesses     int    `json:"slow_window_slow_successes"`
	SlowRatePercent             int    `json:"slow_rate_percent"`
	SlowRateMinRequests         int    `json:"slow_rate_min_requests"`
	SlowRateThresholdPercent    int    `json:"slow_rate_threshold_percent"`
	StrategySource              string `json:"strategy_source"`
	DegradedUntil               int64  `json:"degraded_until"`
	ConsecutiveFailures         int    `json:"consecutive_failures"`
	ConsecutiveSlows            int    `json:"consecutive_slows"`
	DegradeLevel                int    `json:"degrade_level"`
	DegradedConsecutiveFailures int    `json:"degraded_consecutive_failures"`
	DegradedConsecutiveSlows    int    `json:"degraded_consecutive_slows"`
	LastFailureAt               int64  `json:"last_failure_at"`
	LastSlowAt                  int64  `json:"last_slow_at"`
	LastSuccessAt               int64  `json:"last_success_at"`
	LastTriggerReason           string `json:"last_trigger_reason"`
	LastTriggerAt               int64  `json:"last_trigger_at"`
	LastSlowReason              string `json:"last_slow_reason"`
	StrategyVersion             int    `json:"strategy_version"`
}

type AggregateGroupRuntimeClientRoutePoolView struct {
	PoolName          string                            `json:"pool_name"`
	ClientType        string                            `json:"client_type"`
	Enabled           bool                              `json:"enabled"`
	FallbackToDefault bool                              `json:"fallback_to_default"`
	Routes            []*AggregateGroupRuntimeRouteView `json:"routes"`
}

type AggregateGroupRuntimeView struct {
	ActiveRoute      *AggregateGroupRuntimeActiveRouteView       `json:"active_route,omitempty"`
	Routes           []*AggregateGroupRuntimeRouteView           `json:"routes"`
	ClientRoutePools []*AggregateGroupRuntimeClientRoutePoolView `json:"client_route_pools,omitempty"`
}

func BuildAggregateGroupRuntimeView(group *model.AggregateGroup, modelName string) (*AggregateGroupRuntimeView, error) {
	runtimeView := &AggregateGroupRuntimeView{
		Routes: make([]*AggregateGroupRuntimeRouteView, 0),
	}
	if group == nil || strings.TrimSpace(modelName) == "" {
		return runtimeView, nil
	}

	activeState, err := GetAggregateGroupRuntimeState(group.Name, modelName)
	if err != nil {
		return nil, err
	}

	hasActiveState := activeState != nil &&
		(strings.TrimSpace(activeState.ActiveGroup) != "" ||
			activeState.LastFailAt > 0 ||
			activeState.LastSuccessAt > 0 ||
			activeState.LastSwitchAt > 0 ||
			activeState.ActiveSinceAt > 0)
	if hasActiveState {
		runtimeView.ActiveRoute = &AggregateGroupRuntimeActiveRouteView{
			ActiveIndex:   activeState.ActiveIndex,
			ActiveGroup:   activeState.ActiveGroup,
			LastFailAt:    activeState.LastFailAt,
			LastSuccessAt: activeState.LastSuccessAt,
			LastSwitchAt:  activeState.LastSwitchAt,
			ActiveSinceAt: activeState.ActiveSinceAt,
		}
	}

	now := common.GetTimestamp()
	isClusterMode := group.GetRoutingMode() == model.AggregateGroupRoutingModeCluster
	strategy := GetAggregateGroupEffectiveSmartStrategy(group)
	buildRouteView := func(routePool string, index int, target model.AggregateGroupTarget, routeActiveState *AggregateGroupRuntimeState, hasRouteActiveState bool) (*AggregateGroupRuntimeRouteView, error) {
		routeView := &AggregateGroupRuntimeRouteView{
			RouteGroup:                  target.RealGroup,
			RouteIndex:                  index,
			RoutePool:                   routePool,
			Weight:                      target.GetWeight(),
			StrategySource:              formatAggregateSmartStrategySource(strategy),
			FailureWindowSeconds:        strategy.FailureRateWindowSeconds,
			FailureRateMinRequests:      strategy.FailureRateMinRequests,
			FailureRateThresholdPercent: strategy.FailureRateThresholdPct,
			SlowWindowSeconds:           strategy.SlowRateWindowSeconds,
			SlowRateMinRequests:         strategy.SlowRateMinRequests,
			SlowRateThresholdPercent:    strategy.SlowRateThresholdPct,
		}
		if routePool == aggregateClusterDefaultRoutePool {
			routeView.RoutePool = ""
		}
		priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, modelName)
		if err != nil {
			return nil, err
		}
		routeView.PriorityCount = priorityCount
		rpmStats := GetAggregateRouteRPMStatsForPool(group.Name, modelName, routePool, target.RealGroup)
		routeView.RPM = rpmStats.RPM
		routeView.TotalRPM = GetAggregateRouteTotalRPM(group.Name, target.RealGroup)
		routeView.RPMLimit = target.GetRPMLimit()
		routeView.RPMLimited = routeView.RPMLimit > 0 && routeView.TotalRPM >= routeView.RPMLimit
		routeView.SuccessRPM = rpmStats.SuccessRPM
		routeView.FailureRPM = rpmStats.FailureRPM
		routeView.StrategyFailureRPM = rpmStats.StrategyFailureRPM
		routeView.SlowSuccessRPM = rpmStats.SlowSuccessRPM
		failureWindowStats := GetAggregateRouteWindowStatsForPool(group.Name, modelName, routePool, target.RealGroup, strategy.FailureRateWindowSeconds)
		routeView.FailureWindowRequests = failureWindowStats.Attempts
		routeView.FailureWindowFailures = failureWindowStats.StrategyFailures
		routeView.FailureRatePercent = calculateAggregateRatePercent(failureWindowStats.StrategyFailures, failureWindowStats.Attempts)
		slowWindowStats := GetAggregateRouteWindowStatsForPool(group.Name, modelName, routePool, target.RealGroup, strategy.SlowRateWindowSeconds)
		routeView.SlowWindowSuccesses = slowWindowStats.Successes
		routeView.SlowWindowSlowSuccesses = slowWindowStats.SlowSuccesses
		routeView.SlowRatePercent = calculateAggregateRatePercent(slowWindowStats.SlowSuccesses, slowWindowStats.Successes)
		if hasRouteActiveState {
			if strings.TrimSpace(routeActiveState.ActiveGroup) != "" {
				routeView.IsActive = routeActiveState.ActiveGroup == target.RealGroup
			} else {
				routeView.IsActive = routeActiveState.ActiveIndex == index
			}
		}

		state, _, err := RefreshAggregateGroupRouteStrategyState(group.Name, modelName, target.RealGroup)
		if err != nil {
			return nil, err
		}
		if state != nil {
			routeView.IsDegraded = state.DegradedUntil > now
			routeView.DegradedUntil = state.DegradedUntil
			routeView.ConsecutiveFailures = state.ConsecutiveFailures
			routeView.ConsecutiveSlows = state.ConsecutiveSlows
			routeView.DegradeLevel = state.DegradeLevel
			routeView.DegradedConsecutiveFailures = state.DegradedConsecutiveFailures
			routeView.DegradedConsecutiveSlows = state.DegradedConsecutiveSlows
			routeView.LastFailureAt = state.LastFailureAt
			routeView.LastSlowAt = state.LastSlowAt
			routeView.LastSuccessAt = state.LastSuccessAt
			routeView.LastTriggerReason = state.LastTriggerReason
			routeView.LastTriggerAt = state.LastTriggerAt
			routeView.LastSlowReason = state.LastSlowReason
			routeView.StrategyVersion = state.StrategyVersion
		}
		if routeView.PriorityCount > 0 {
			if isClusterMode {
				routeView.EffectiveWeight = calculateAggregateClusterEffectiveWeightWithPercent(routeView.Weight, routeView.DegradeLevel, strategy.ClusterDegradedWeightPercent)
			} else {
				routeView.EffectiveWeight = routeView.Weight
			}
		}
		return routeView, nil
	}

	healthySupportedCount := 0
	for index, target := range group.Targets {
		routeView, err := buildRouteView(aggregateClusterDefaultRoutePool, index, target, activeState, hasActiveState)
		if err != nil {
			return nil, err
		}
		if routeView.PriorityCount > 0 && !routeView.IsDegraded {
			healthySupportedCount++
		}
		runtimeView.Routes = append(runtimeView.Routes, routeView)
	}
	if isClusterMode && healthySupportedCount == 0 {
		for _, routeView := range runtimeView.Routes {
			lastRouteActivityAt := routeView.LastSuccessAt
			if routeView.LastFailureAt > lastRouteActivityAt {
				lastRouteActivityAt = routeView.LastFailureAt
			}
			routeView.IsSoftFallback = routeView.IsDegraded &&
				routeView.PriorityCount > 0 &&
				routeView.RPM > 0 &&
				routeView.LastTriggerAt > 0 &&
				lastRouteActivityAt > routeView.LastTriggerAt
		}
	}
	clientRoutePools := group.GetClientRoutePools()
	if clientRoutePools.Enabled && clientRoutePools.ClaudeCodeCLI.Enabled {
		poolView := &AggregateGroupRuntimeClientRoutePoolView{
			PoolName:          model.AggregateGroupClientRoutePoolClaudeCodeCLI,
			ClientType:        model.AggregateGroupClientTypeClaudeCodeCLI,
			Enabled:           clientRoutePools.ClaudeCodeCLI.Enabled,
			FallbackToDefault: clientRoutePools.ClaudeCodeCLI.GetFallbackToDefault(),
			Routes:            make([]*AggregateGroupRuntimeRouteView, 0, len(clientRoutePools.ClaudeCodeCLI.Targets)),
		}
		poolActiveState, err := GetAggregateGroupRuntimeStateForPool(group.Name, modelName, model.AggregateGroupClientRoutePoolClaudeCodeCLI)
		if err != nil {
			return nil, err
		}
		hasPoolActiveState := poolActiveState != nil &&
			(strings.TrimSpace(poolActiveState.ActiveGroup) != "" ||
				poolActiveState.LastFailAt > 0 ||
				poolActiveState.LastSuccessAt > 0 ||
				poolActiveState.LastSwitchAt > 0 ||
				poolActiveState.ActiveSinceAt > 0)
		targets := aggregateClientRoutePoolTargetsToClusterTargets(clientRoutePools.ClaudeCodeCLI.Targets)
		for index, target := range targets {
			routeView, err := buildRouteView(model.AggregateGroupClientRoutePoolClaudeCodeCLI, index, target, poolActiveState, hasPoolActiveState)
			if err != nil {
				return nil, err
			}
			poolView.Routes = append(poolView.Routes, routeView)
		}
		runtimeView.ClientRoutePools = append(runtimeView.ClientRoutePools, poolView)
	}

	return runtimeView, nil
}
