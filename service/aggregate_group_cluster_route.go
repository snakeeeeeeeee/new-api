package service

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type aggregateClusterRouteCandidate struct {
	Target          model.AggregateGroupTarget
	Index           int
	PriorityCount   int
	IsDegraded      bool
	Weight          int
	EffectiveWeight int
}

func calculateAggregateClusterEffectiveWeight(weight int, isDegraded bool) int {
	if weight <= 0 {
		return 0
	}
	if !isDegraded {
		return weight
	}
	percent := setting.NormalizeAggregateGroupClusterDegradedWeightPercent(setting.AggregateGroupClusterDegradedWeightPct)
	reducedWeight := int(math.Ceil(float64(weight) * float64(percent) / 100))
	if reducedWeight < 1 {
		return 1
	}
	return reducedWeight
}

func getAggregateAttemptedRouteIndexes(ctx *gin.Context) map[int]bool {
	if ctx == nil {
		return map[int]bool{}
	}
	value, ok := common.GetContextKey(ctx, constant.ContextKeyAggregateAttemptedRoutes)
	if !ok {
		return map[int]bool{}
	}
	attempted, ok := value.(map[int]bool)
	if !ok || attempted == nil {
		return map[int]bool{}
	}
	return attempted
}

func markAggregateRouteAttempted(ctx *gin.Context, index int) {
	if ctx == nil || index < 0 {
		return
	}
	attempted := getAggregateAttemptedRouteIndexes(ctx)
	attempted[index] = true
	common.SetContextKey(ctx, constant.ContextKeyAggregateAttemptedRoutes, attempted)
}

func isAggregateRouteAttempted(ctx *gin.Context, index int) bool {
	if index < 0 {
		return false
	}
	return getAggregateAttemptedRouteIndexes(ctx)[index]
}

func buildAggregateClusterRouteCandidates(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, skipDegraded bool, excludeAttempted bool) ([]aggregateClusterRouteCandidate, []aggregateClusterRouteCandidate, error) {
	healthyCandidates := make([]aggregateClusterRouteCandidate, 0, len(aggregateGroup.Targets))
	supportedCandidates := make([]aggregateClusterRouteCandidate, 0, len(aggregateGroup.Targets))
	for index, target := range aggregateGroup.Targets {
		if excludeAttempted && isAggregateRouteAttempted(ctx, index) {
			continue
		}
		priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, modelName)
		if err != nil {
			return nil, nil, err
		}
		if priorityCount <= 0 {
			continue
		}
		candidate := aggregateClusterRouteCandidate{
			Target:        target,
			Index:         index,
			PriorityCount: priorityCount,
			Weight:        target.GetWeight(),
		}
		if candidate.Weight < 0 {
			candidate.Weight = 0
		}
		if skipDegraded {
			degraded, state, recovered, err := IsAggregateGroupRouteTemporarilyDegraded(aggregateGroup.Name, modelName, target.RealGroup)
			if err != nil {
				return nil, nil, err
			}
			if recovered {
				logger.LogInfo(ctx, fmt.Sprintf("aggregate cluster smart strategy recovered route: aggregate_group=%s, model=%s, route_group=%s", aggregateGroup.Name, modelName, target.RealGroup))
			}
			if degraded {
				candidate.IsDegraded = true
				logger.LogWarn(ctx, fmt.Sprintf("aggregate cluster smart strategy reduced degraded route: aggregate_group=%s, model=%s, route_group=%s(index=%d), weight=%d, effective_weight=%d, degraded_until=%d", aggregateGroup.Name, modelName, target.RealGroup, index, candidate.Weight, calculateAggregateClusterEffectiveWeight(candidate.Weight, true), state.DegradedUntil))
			}
		}
		candidate.EffectiveWeight = calculateAggregateClusterEffectiveWeight(candidate.Weight, candidate.IsDegraded)
		supportedCandidates = append(supportedCandidates, candidate)
		if !candidate.IsDegraded {
			healthyCandidates = append(healthyCandidates, candidate)
		}
	}
	return healthyCandidates, supportedCandidates, nil
}

func findAggregateClusterCandidateByRoute(candidates []aggregateClusterRouteCandidate, routeGroup string) (aggregateClusterRouteCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.Target.RealGroup == routeGroup {
			return candidate, true
		}
	}
	return aggregateClusterRouteCandidate{}, false
}

func pickAggregateClusterCandidateByWeight(candidates []aggregateClusterRouteCandidate) (aggregateClusterRouteCandidate, bool) {
	if len(candidates) == 0 {
		return aggregateClusterRouteCandidate{}, false
	}
	totalWeight := 0
	for _, candidate := range candidates {
		if candidate.EffectiveWeight > 0 {
			totalWeight += candidate.EffectiveWeight
		}
	}
	if totalWeight <= 0 {
		return candidates[common.GetRandomInt(len(candidates))], true
	}
	randomWeight := common.GetRandomInt(totalWeight)
	for _, candidate := range candidates {
		if candidate.EffectiveWeight <= 0 {
			continue
		}
		randomWeight -= candidate.EffectiveWeight
		if randomWeight < 0 {
			return candidate, true
		}
	}
	return candidates[len(candidates)-1], true
}

func chooseAggregateClusterRouteCandidate(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, skipDegraded bool, excludeAttempted bool, affinityRouteGroup string) (aggregateClusterRouteCandidate, bool, bool, error) {
	healthyCandidates, supportedCandidates, err := buildAggregateClusterRouteCandidates(ctx, aggregateGroup, modelName, skipDegraded, excludeAttempted)
	if err != nil {
		return aggregateClusterRouteCandidate{}, false, false, err
	}
	if affinityRouteGroup != "" {
		if candidate, ok := findAggregateClusterCandidateByRoute(healthyCandidates, affinityRouteGroup); ok {
			return candidate, true, true, nil
		}
	}
	candidates := healthyCandidates
	if len(candidates) == 0 {
		candidates = supportedCandidates
	} else {
		for _, candidate := range supportedCandidates {
			if candidate.IsDegraded {
				candidates = append(candidates, candidate)
			}
		}
	}
	candidate, ok := pickAggregateClusterCandidateByWeight(candidates)
	return candidate, ok, false, nil
}

func selectAggregateClusterChannelFromIndex(param *RetryParam, aggregateGroup *model.AggregateGroup, index int, retryBase int) (*model.Channel, string, error) {
	if index < 0 || index >= len(aggregateGroup.Targets) {
		return nil, "", nil
	}
	target := aggregateGroup.Targets[index]
	priorityRetry := param.GetRetry() - retryBase
	if priorityRetry < 0 {
		priorityRetry = 0
	}
	priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, param.ModelName)
	if err != nil {
		return nil, target.RealGroup, err
	}
	if priorityCount <= 0 {
		return nil, target.RealGroup, nil
	}
	channel, err := model.GetRandomSatisfiedChannel(target.RealGroup, param.ModelName, priorityRetry)
	if err != nil {
		return nil, target.RealGroup, err
	}
	if channel == nil {
		return nil, target.RealGroup, nil
	}
	setAggregateGroupStartIndexes(param.Ctx, index)
	common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroup, target.RealGroup)
	common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroupIndex, index)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRetryBase, retryBase)
	markAggregateRouteAttempted(param.Ctx, index)
	RecordAggregateRouteRPMAttempt(param.Ctx, param.ModelName, target.RealGroup)
	return channel, target.RealGroup, nil
}

func selectAggregateGroupClusterChannel(param *RetryParam, aggregateGroup *model.AggregateGroup) (*model.Channel, string, error) {
	skipDegraded := IsAggregateSmartRoutingEnabled(aggregateGroup)
	if retryIndex, ok := resolveExplicitAggregateRetryIndex(param.Ctx); ok {
		retryBase := param.GetRetry()
		if base, ok := resolveExplicitAggregateRetryBase(param.Ctx); ok {
			retryBase = base
		}
		channel, group, err := selectAggregateClusterChannelFromIndex(param, aggregateGroup, retryIndex, retryBase)
		if err != nil || channel != nil {
			return channel, group, err
		}
		markAggregateRouteAttempted(param.Ctx, retryIndex)
	}

	affinityRouteGroup := ""
	if routeGroup, ok := GetAggregateRouteAffinity(param.Ctx, param.ModelName, aggregateGroup.Name); ok {
		affinityRouteGroup = routeGroup
	}
	candidate, ok, usedAffinity, err := chooseAggregateClusterRouteCandidate(param.Ctx, aggregateGroup, param.ModelName, skipDegraded, true, affinityRouteGroup)
	if err != nil || !ok {
		return nil, "", err
	}
	if usedAffinity {
		common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRouteAffinityHit, candidate.Target.RealGroup)
	}
	retryBase := param.GetRetry()
	channel, group, err := selectAggregateClusterChannelFromIndex(param, aggregateGroup, candidate.Index, retryBase)
	if err != nil || channel != nil {
		return channel, group, err
	}
	markAggregateRouteAttempted(param.Ctx, candidate.Index)

	candidate, ok, _, err = chooseAggregateClusterRouteCandidate(param.Ctx, aggregateGroup, param.ModelName, skipDegraded, true, "")
	if err != nil || !ok {
		return nil, "", err
	}
	return selectAggregateClusterChannelFromIndex(param, aggregateGroup, candidate.Index, retryBase)
}

func prepareAggregateClusterRetry(c *gin.Context, aggregateGroup *model.AggregateGroup, transition *AggregateRetryTransition, currentRetry int, modelName string, maxInternalRetries int) *AggregateRetryTransition {
	failedIndex := transition.FailedIndex
	retryBase, _ := resolveExplicitAggregateRetryBase(c)
	priorityCount, err := model.GetSatisfiedChannelPriorityCount(transition.FailedGroup, modelName)
	if err == nil && priorityCount > 0 {
		priorityRetry := currentRetry - retryBase
		if priorityRetry < 0 {
			priorityRetry = 0
		}
		if priorityRetry < maxInternalRetries {
			transition.HasNext = true
			transition.WithinCurrentGroup = true
			transition.NextGroup = transition.FailedGroup
			transition.NextIndex = failedIndex
			common.SetContextKey(c, constant.ContextKeyAggregateRetryIndex, failedIndex)
			common.SetContextKey(c, constant.ContextKeyAggregateRetryBase, retryBase)
			return transition
		}
	}

	markAggregateRouteAttempted(c, failedIndex)
	skipDegraded := IsAggregateSmartRoutingEnabled(aggregateGroup)
	candidate, ok, _, err := chooseAggregateClusterRouteCandidate(c, aggregateGroup, modelName, skipDegraded, true, "")
	if err != nil {
		logger.LogError(c, "aggregate cluster retry choose route failed: "+err.Error())
		return transition
	}
	if !ok {
		return transition
	}
	transition.HasNext = true
	transition.NextGroup = candidate.Target.RealGroup
	transition.NextIndex = candidate.Index
	common.SetContextKey(c, constant.ContextKeyAggregateRetryIndex, candidate.Index)
	common.SetContextKey(c, constant.ContextKeyAggregateRetryBase, currentRetry+1)
	return transition
}
