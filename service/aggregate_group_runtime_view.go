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
	RouteGroup          string `json:"route_group"`
	RouteIndex          int    `json:"route_index"`
	Weight              int    `json:"weight"`
	EffectiveWeight     int    `json:"effective_weight"`
	IsActive            bool   `json:"is_active"`
	IsDegraded          bool   `json:"is_degraded"`
	IsSoftFallback      bool   `json:"is_soft_fallback"`
	PriorityCount       int    `json:"priority_count"`
	RPM                 int    `json:"rpm"`
	SuccessRPM          int    `json:"success_rpm"`
	FailureRPM          int    `json:"failure_rpm"`
	DegradedUntil       int64  `json:"degraded_until"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	ConsecutiveSlows    int    `json:"consecutive_slows"`
	LastFailureAt       int64  `json:"last_failure_at"`
	LastSlowAt          int64  `json:"last_slow_at"`
	LastSuccessAt       int64  `json:"last_success_at"`
	LastTriggerReason   string `json:"last_trigger_reason"`
	LastTriggerAt       int64  `json:"last_trigger_at"`
}

type AggregateGroupRuntimeView struct {
	ActiveRoute *AggregateGroupRuntimeActiveRouteView `json:"active_route,omitempty"`
	Routes      []*AggregateGroupRuntimeRouteView     `json:"routes"`
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
	healthySupportedCount := 0
	for index, target := range group.Targets {
		routeView := &AggregateGroupRuntimeRouteView{
			RouteGroup: target.RealGroup,
			RouteIndex: index,
			Weight:     target.GetWeight(),
		}
		priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, modelName)
		if err != nil {
			return nil, err
		}
		routeView.PriorityCount = priorityCount
		rpmStats := GetAggregateRouteRPMStats(group.Name, modelName, target.RealGroup)
		routeView.RPM = rpmStats.RPM
		routeView.SuccessRPM = rpmStats.SuccessRPM
		routeView.FailureRPM = rpmStats.FailureRPM
		if hasActiveState {
			if strings.TrimSpace(activeState.ActiveGroup) != "" {
				routeView.IsActive = activeState.ActiveGroup == target.RealGroup
			} else {
				routeView.IsActive = activeState.ActiveIndex == index
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
			routeView.LastFailureAt = state.LastFailureAt
			routeView.LastSlowAt = state.LastSlowAt
			routeView.LastSuccessAt = state.LastSuccessAt
			routeView.LastTriggerReason = state.LastTriggerReason
			routeView.LastTriggerAt = state.LastTriggerAt
		}
		if routeView.PriorityCount > 0 {
			if isClusterMode {
				routeView.EffectiveWeight = calculateAggregateClusterEffectiveWeight(routeView.Weight, routeView.IsDegraded)
			} else {
				routeView.EffectiveWeight = routeView.Weight
			}
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

	return runtimeView, nil
}
