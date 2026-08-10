package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AsyncImageRouteSelection struct {
	Channel       *model.Channel
	RouteGroup    string
	RouteIndex    int
	RoutePool     string
	SwitchedGroup bool
}

func asyncImageMaxAttemptsPerGroup(state *model.ImageTaskRetryState) int {
	if state == nil || state.RetryLimit < 0 {
		return 1
	}
	return state.RetryLimit + 1
}

func selectAsyncImageChannel(state *model.ImageTaskRetryState, group string) (*model.Channel, error) {
	if state == nil || strings.TrimSpace(group) == "" {
		return nil, nil
	}
	return model.GetRandomSatisfiedChannelExcluding(group, state.OriginalModel, state.FailedChannelIDs)
}

func applyAsyncImageRouteSelection(state *model.ImageTaskRetryState, selection *AsyncImageRouteSelection) {
	if state == nil || selection == nil || selection.Channel == nil {
		return
	}
	if selection.SwitchedGroup || state.CurrentRouteGroup != selection.RouteGroup {
		state.CurrentGroupAttempts = 1
	} else {
		state.CurrentGroupAttempts++
	}
	state.CurrentRouteGroup = selection.RouteGroup
	state.CurrentRouteIndex = selection.RouteIndex
	state.CurrentRoutePool = normalizeAggregateClusterRoutePool(selection.RoutePool)
}

func RecordAsyncImageRouteAttempt(ctx *gin.Context, state *model.ImageTaskRetryState, selection *AsyncImageRouteSelection) {
	if ctx == nil || state == nil || selection == nil || state.AggregateGroup == "" {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, state.AggregateGroup)
	common.SetContextKey(ctx, constant.ContextKeyRouteGroup, selection.RouteGroup)
	common.SetContextKey(ctx, constant.ContextKeyRouteGroupIndex, selection.RouteIndex)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRoutePool, normalizeAggregateClusterRoutePool(selection.RoutePool))
	RecordAggregateRouteRPMAttempt(ctx, state.OriginalModel, selection.RouteGroup)
}

// SelectNextAsyncImageRoute mutates the locked retry state to reflect the
// selected route. Callers persist it in the same transaction as the attempt.
func SelectNextAsyncImageRoute(ctx *gin.Context, state *model.ImageTaskRetryState) (*AsyncImageRouteSelection, error) {
	if state == nil {
		return nil, fmt.Errorf("image task retry state is nil")
	}
	if state.LockedChannel || state.Status != model.ImageTaskRetryStateActive {
		return nil, nil
	}
	currentGroup := strings.TrimSpace(state.CurrentRouteGroup)
	if currentGroup != "" && state.CurrentGroupAttempts < asyncImageMaxAttemptsPerGroup(state) {
		channel, err := selectAsyncImageChannel(state, currentGroup)
		if err != nil {
			return nil, err
		}
		if channel != nil {
			selection := &AsyncImageRouteSelection{
				Channel: channel, RouteGroup: currentGroup, RouteIndex: state.CurrentRouteIndex,
				RoutePool: state.CurrentRoutePool,
			}
			applyAsyncImageRouteSelection(state, selection)
			return selection, nil
		}
	}

	if state.AggregateGroup == "" {
		return nil, nil
	}
	aggregateGroup, ok := GetAggregateGroup(state.AggregateGroup, true)
	if !ok || aggregateGroup == nil || !aggregateGroup.IsEnabled() {
		return nil, nil
	}
	state.AddAttemptedRouteKey(buildAggregateRouteAttemptKey(state.CurrentRoutePool, currentGroup))
	if aggregateGroup.GetRoutingMode() == model.AggregateGroupRoutingModeCluster {
		return selectNextAsyncImageClusterRoute(ctx, state, aggregateGroup)
	}
	return selectNextAsyncImageFailoverRoute(ctx, state, aggregateGroup)
}

func selectNextAsyncImageFailoverRoute(ctx *gin.Context, state *model.ImageTaskRetryState, aggregateGroup *model.AggregateGroup) (*AsyncImageRouteSelection, error) {
	startIndex := state.CurrentRouteIndex + 1
	if startIndex < 0 {
		startIndex = 0
	}
	tryTargets := func(skipDegraded bool) (*AsyncImageRouteSelection, error) {
		for index := startIndex; index < len(aggregateGroup.Targets); index++ {
			target := aggregateGroup.Targets[index]
			key := buildAggregateRouteAttemptKey(aggregateClusterDefaultRoutePool, target.RealGroup)
			if state.HasAttemptedRouteKey(key) || IsAggregateTargetRPMLimited(aggregateGroup, target) {
				continue
			}
			if skipDegraded {
				degraded, _, _, err := IsAggregateGroupRouteTemporarilyDegraded(aggregateGroup.Name, state.OriginalModel, target.RealGroup)
				if err != nil {
					return nil, err
				}
				if degraded {
					continue
				}
			}
			channel, err := selectAsyncImageChannel(state, target.RealGroup)
			if err != nil {
				return nil, err
			}
			if channel == nil {
				state.AddAttemptedRouteKey(key)
				continue
			}
			selection := &AsyncImageRouteSelection{
				Channel: channel, RouteGroup: target.RealGroup, RouteIndex: index,
				RoutePool: aggregateClusterDefaultRoutePool, SwitchedGroup: true,
			}
			applyAsyncImageRouteSelection(state, selection)
			return selection, nil
		}
		return nil, nil
	}
	if IsAggregateSmartRoutingEnabled(aggregateGroup) {
		selection, err := tryTargets(true)
		if err != nil || selection != nil {
			return selection, err
		}
	}
	return tryTargets(false)
}

func restoreAsyncImageAttemptedRoutes(ctx *gin.Context, state *model.ImageTaskRetryState) {
	if ctx == nil || state == nil {
		return
	}
	attemptedKeys := make(map[string]bool, len(state.AttemptedRouteKeys))
	for _, key := range state.AttemptedRouteKeys {
		if key != "" {
			attemptedKeys[key] = true
		}
	}
	common.SetContextKey(ctx, constant.ContextKeyAggregateAttemptedRouteKeys, attemptedKeys)
}

func selectNextAsyncImageClusterRoute(ctx *gin.Context, state *model.ImageTaskRetryState, aggregateGroup *model.AggregateGroup) (*AsyncImageRouteSelection, error) {
	if ctx == nil {
		ctx, _ = gin.CreateTestContext(noopResponseWriter{})
	}
	restoreAsyncImageAttemptedRoutes(ctx, state)
	skipDegraded := IsAggregateSmartRoutingEnabled(aggregateGroup)
	for tries := 0; tries < len(aggregateGroup.Targets); tries++ {
		candidate, ok, _, err := chooseAggregateClusterRouteCandidate(ctx, aggregateGroup, state.OriginalModel, skipDegraded, true, "")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		channel, err := selectAsyncImageChannel(state, candidate.Target.RealGroup)
		if err != nil {
			return nil, err
		}
		if channel == nil {
			markAggregateRouteCandidateAttempted(ctx, candidate)
			state.AddAttemptedRouteKey(buildAggregateRouteAttemptKey(candidate.RoutePool, candidate.Target.RealGroup))
			continue
		}
		selection := &AsyncImageRouteSelection{
			Channel: channel, RouteGroup: candidate.Target.RealGroup, RouteIndex: candidate.Index,
			RoutePool: candidate.RoutePool, SwitchedGroup: true,
		}
		applyAsyncImageRouteSelection(state, selection)
		return selection, nil
	}
	return nil, nil
}
