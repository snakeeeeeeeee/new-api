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
	IsActive            bool   `json:"is_active"`
	IsDegraded          bool   `json:"is_degraded"`
	PriorityCount       int    `json:"priority_count"`
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
	for index, target := range group.Targets {
		routeView := &AggregateGroupRuntimeRouteView{
			RouteGroup: target.RealGroup,
			RouteIndex: index,
		}
		priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, modelName)
		if err != nil {
			return nil, err
		}
		routeView.PriorityCount = priorityCount
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
		runtimeView.Routes = append(runtimeView.Routes, routeView)
	}

	return runtimeView, nil
}
