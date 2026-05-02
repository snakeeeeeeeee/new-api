package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const aggregateClusterDefaultRoutePool = "default"

type aggregateClusterRouteCandidate struct {
	Target          model.AggregateGroupTarget
	Index           int
	RoutePool       string
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

func normalizeAggregateClusterRoutePool(routePool string) string {
	routePool = strings.TrimSpace(routePool)
	if routePool == "" {
		return aggregateClusterDefaultRoutePool
	}
	return routePool
}

func buildAggregateRouteAttemptKey(routePool string, routeGroup string) string {
	routePool = normalizeAggregateClusterRoutePool(routePool)
	routeGroup = strings.TrimSpace(routeGroup)
	if routeGroup == "" {
		return ""
	}
	return routePool + "\n" + routeGroup
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

func getAggregateAttemptedRouteKeys(ctx *gin.Context) map[string]bool {
	if ctx == nil {
		return map[string]bool{}
	}
	value, ok := common.GetContextKey(ctx, constant.ContextKeyAggregateAttemptedRouteKeys)
	if !ok {
		return map[string]bool{}
	}
	attempted, ok := value.(map[string]bool)
	if !ok || attempted == nil {
		return map[string]bool{}
	}
	return attempted
}

func markAggregateRouteKeyAttempted(ctx *gin.Context, routePool string, routeGroup string) {
	if ctx == nil {
		return
	}
	key := buildAggregateRouteAttemptKey(routePool, routeGroup)
	if key == "" {
		return
	}
	attempted := getAggregateAttemptedRouteKeys(ctx)
	attempted[key] = true
	common.SetContextKey(ctx, constant.ContextKeyAggregateAttemptedRouteKeys, attempted)
}

func isAggregateRouteKeyAttempted(ctx *gin.Context, routePool string, routeGroup string) bool {
	key := buildAggregateRouteAttemptKey(routePool, routeGroup)
	if key == "" {
		return false
	}
	return getAggregateAttemptedRouteKeys(ctx)[key]
}

func markAggregateRouteCandidateAttempted(ctx *gin.Context, candidate aggregateClusterRouteCandidate) {
	markAggregateRouteKeyAttempted(ctx, candidate.RoutePool, candidate.Target.RealGroup)
	if normalizeAggregateClusterRoutePool(candidate.RoutePool) == aggregateClusterDefaultRoutePool {
		markAggregateRouteAttempted(ctx, candidate.Index)
	}
}

func isAggregateRouteCandidateAttempted(ctx *gin.Context, routePool string, index int, routeGroup string) bool {
	if isAggregateRouteKeyAttempted(ctx, routePool, routeGroup) {
		return true
	}
	if normalizeAggregateClusterRoutePool(routePool) == aggregateClusterDefaultRoutePool && isAggregateRouteAttempted(ctx, index) {
		return true
	}
	return false
}

func buildAggregateClusterRouteCandidatesFromTargets(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, routePool string, targets []model.AggregateGroupTarget, skipDegraded bool, excludeAttempted bool) ([]aggregateClusterRouteCandidate, []aggregateClusterRouteCandidate, error) {
	routePool = normalizeAggregateClusterRoutePool(routePool)
	healthyCandidates := make([]aggregateClusterRouteCandidate, 0, len(targets))
	supportedCandidates := make([]aggregateClusterRouteCandidate, 0, len(targets))
	for index, target := range targets {
		if excludeAttempted && isAggregateRouteCandidateAttempted(ctx, routePool, index, target.RealGroup) {
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
			RoutePool:     routePool,
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

func buildAggregateClusterRouteCandidates(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, skipDegraded bool, excludeAttempted bool) ([]aggregateClusterRouteCandidate, []aggregateClusterRouteCandidate, error) {
	if aggregateGroup == nil {
		return nil, nil, fmt.Errorf("aggregate group is nil")
	}
	return buildAggregateClusterRouteCandidatesFromTargets(ctx, aggregateGroup, modelName, aggregateClusterDefaultRoutePool, aggregateGroup.Targets, skipDegraded, excludeAttempted)
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
		return aggregateClusterRouteCandidate{}, false
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

func chooseAggregateClusterRouteCandidateFromTargets(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, routePool string, targets []model.AggregateGroupTarget, skipDegraded bool, excludeAttempted bool, affinityRouteGroup string) (aggregateClusterRouteCandidate, bool, bool, error) {
	healthyCandidates, supportedCandidates, err := buildAggregateClusterRouteCandidatesFromTargets(ctx, aggregateGroup, modelName, routePool, targets, skipDegraded, excludeAttempted)
	if err != nil {
		return aggregateClusterRouteCandidate{}, false, false, err
	}
	if affinityRouteGroup != "" {
		if candidate, ok := findAggregateClusterCandidateByRoute(healthyCandidates, affinityRouteGroup); ok {
			if candidate.EffectiveWeight > 0 {
				return candidate, true, true, nil
			}
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

func chooseAggregateClusterRouteCandidate(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, skipDegraded bool, excludeAttempted bool, affinityRouteGroup string) (aggregateClusterRouteCandidate, bool, bool, error) {
	return chooseAggregateClusterRouteCandidateFromTargets(ctx, aggregateGroup, modelName, aggregateClusterDefaultRoutePool, aggregateGroup.Targets, skipDegraded, excludeAttempted, affinityRouteGroup)
}

func aggregateClientRoutePoolTargetsToClusterTargets(inputTargets []model.AggregateGroupClientRoutePoolTarget) []model.AggregateGroupTarget {
	targets := make([]model.AggregateGroupTarget, 0, len(inputTargets))
	for _, target := range inputTargets {
		realGroup := strings.TrimSpace(target.RealGroup)
		if realGroup == "" {
			continue
		}
		weight := model.AggregateGroupTargetDefaultWeight
		if target.Weight != nil {
			weight = *target.Weight
		}
		targets = append(targets, model.AggregateGroupTarget{
			RealGroup:  realGroup,
			OrderIndex: len(targets),
			Weight:     common.GetPointer(weight),
		})
	}
	return targets
}

func setAggregateClientRouteContext(ctx *gin.Context, clientType string, routePool string, routeTarget string, fallback bool) {
	if ctx == nil || strings.TrimSpace(clientType) == "" || strings.TrimSpace(routePool) == "" {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyAggregateClientType, clientType)
	common.SetContextKey(ctx, constant.ContextKeyAggregateClientRoutePool, routePool)
	common.SetContextKey(ctx, constant.ContextKeyAggregateClientRouteFallback, fallback)
	common.SetContextKey(ctx, constant.ContextKeyAggregateClientRouteTarget, strings.TrimSpace(routeTarget))
}

func setAggregateClusterNextRetryRoute(ctx *gin.Context, candidate aggregateClusterRouteCandidate, retryBase int) {
	if ctx == nil {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryRoutePool, normalizeAggregateClusterRoutePool(candidate.RoutePool))
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryRouteGroup, candidate.Target.RealGroup)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryIndex, candidate.Index)
	common.SetContextKey(ctx, constant.ContextKeyAggregateRetryBase, retryBase)
}

func resolveExplicitAggregateRetryRoute(ctx *gin.Context) (string, string, int, bool) {
	if ctx == nil {
		return "", "", -1, false
	}
	routeGroup := strings.TrimSpace(common.GetContextKeyString(ctx, constant.ContextKeyAggregateRetryRouteGroup))
	if routeGroup == "" {
		return "", "", -1, false
	}
	routePool := normalizeAggregateClusterRoutePool(common.GetContextKeyString(ctx, constant.ContextKeyAggregateRetryRoutePool))
	index := -1
	if retryIndex, ok := resolveExplicitAggregateRetryIndex(ctx); ok {
		index = retryIndex
	}
	return routePool, routeGroup, index, true
}

func findExplicitAggregateClusterCandidate(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, routePool string, routeGroup string, routeIndex int, skipDegraded bool) (aggregateClusterRouteCandidate, bool, error) {
	var targets []model.AggregateGroupTarget
	routePool = normalizeAggregateClusterRoutePool(routePool)
	if routePool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
		targets = aggregateClientRoutePoolTargetsToClusterTargets(aggregateGroup.GetClientRoutePools().ClaudeCodeCLI.Targets)
	} else {
		targets = aggregateGroup.Targets
	}
	_, supportedCandidates, err := buildAggregateClusterRouteCandidatesFromTargets(ctx, aggregateGroup, modelName, routePool, targets, skipDegraded, false)
	if err != nil {
		return aggregateClusterRouteCandidate{}, false, err
	}
	if routeGroup != "" {
		candidate, ok := findAggregateClusterCandidateByRoute(supportedCandidates, routeGroup)
		return candidate, ok, nil
	}
	if routeIndex >= 0 && routeIndex < len(supportedCandidates) {
		return supportedCandidates[routeIndex], true, nil
	}
	return aggregateClusterRouteCandidate{}, false, nil
}

func selectAggregateClusterChannelFromIndex(param *RetryParam, aggregateGroup *model.AggregateGroup, index int, retryBase int) (*model.Channel, string, error) {
	if index < 0 || index >= len(aggregateGroup.Targets) {
		return nil, "", nil
	}
	candidate := aggregateClusterRouteCandidate{
		Target:    aggregateGroup.Targets[index],
		Index:     index,
		RoutePool: aggregateClusterDefaultRoutePool,
	}
	return selectAggregateClusterChannelFromCandidate(param, aggregateGroup, candidate, retryBase)
}

func selectAggregateClusterChannelFromCandidate(param *RetryParam, aggregateGroup *model.AggregateGroup, candidate aggregateClusterRouteCandidate, retryBase int) (*model.Channel, string, error) {
	target := candidate.Target
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
	setAggregateGroupStartIndexes(param.Ctx, candidate.Index)
	common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroup, target.RealGroup)
	common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroupIndex, candidate.Index)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRoutePool, normalizeAggregateClusterRoutePool(candidate.RoutePool))
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRetryBase, retryBase)
	if common.GetContextKeyBool(param.Ctx, constant.ContextKeyAggregateClientRouteFallback) {
		if clientType := common.GetContextKeyString(param.Ctx, constant.ContextKeyAggregateClientType); clientType != "" {
			setAggregateClientRouteContext(param.Ctx, clientType, common.GetContextKeyString(param.Ctx, constant.ContextKeyAggregateClientRoutePool), target.RealGroup, true)
		}
	}
	markAggregateRouteCandidateAttempted(param.Ctx, candidate)
	RecordAggregateRouteRPMAttempt(param.Ctx, param.ModelName, target.RealGroup)
	return channel, target.RealGroup, nil
}

type aggregateClientRoutePoolSelection struct {
	ClientType string
	PoolName   string
	Pool       model.AggregateGroupClientRoutePool
	Detection  AggregateClientDetection
}

func resolveAggregateClientRoutePoolSelection(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string) (aggregateClientRoutePoolSelection, bool) {
	if ctx == nil || aggregateGroup == nil {
		return aggregateClientRoutePoolSelection{}, false
	}
	config := aggregateGroup.GetClientRoutePools()
	if !config.Enabled || !config.ClaudeCodeCLI.Enabled {
		return aggregateClientRoutePoolSelection{}, false
	}
	detection := DetectAggregateClientType(ctx, modelName)
	if !detection.Matched || detection.ClientType != model.AggregateGroupClientTypeClaudeCodeCLI {
		return aggregateClientRoutePoolSelection{}, false
	}
	return aggregateClientRoutePoolSelection{
		ClientType: model.AggregateGroupClientTypeClaudeCodeCLI,
		PoolName:   model.AggregateGroupClientRoutePoolClaudeCodeCLI,
		Pool:       config.ClaudeCodeCLI,
		Detection:  detection,
	}, true
}

func selectAggregateClientRoutePoolChannel(param *RetryParam, aggregateGroup *model.AggregateGroup, selection aggregateClientRoutePoolSelection, skipDegraded bool) (*model.Channel, string, error) {
	targets := aggregateClientRoutePoolTargetsToClusterTargets(selection.Pool.Targets)
	if len(targets) == 0 {
		return nil, "", nil
	}
	affinityRouteGroup := ""
	if routeGroup, ok := GetAggregateRouteAffinityForPool(param.Ctx, param.ModelName, aggregateGroup.Name, selection.PoolName); ok {
		affinityRouteGroup = routeGroup
	}
	candidate, ok, usedAffinity, err := chooseAggregateClusterRouteCandidateFromTargets(param.Ctx, aggregateGroup, param.ModelName, selection.PoolName, targets, skipDegraded, true, affinityRouteGroup)
	if err != nil || !ok {
		return nil, "", err
	}
	if usedAffinity {
		common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRouteAffinityHit, candidate.Target.RealGroup)
	}
	retryBase := param.GetRetry()
	setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, candidate.Target.RealGroup, false)
	channel, group, err := selectAggregateClusterChannelFromCandidate(param, aggregateGroup, candidate, retryBase)
	if err != nil || channel != nil {
		return channel, group, err
	}
	markAggregateRouteCandidateAttempted(param.Ctx, candidate)

	candidate, ok, _, err = chooseAggregateClusterRouteCandidateFromTargets(param.Ctx, aggregateGroup, param.ModelName, selection.PoolName, targets, skipDegraded, true, "")
	if err != nil || !ok {
		return nil, "", err
	}
	setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, candidate.Target.RealGroup, false)
	return selectAggregateClusterChannelFromCandidate(param, aggregateGroup, candidate, retryBase)
}

func selectAggregateGroupClusterChannel(param *RetryParam, aggregateGroup *model.AggregateGroup) (*model.Channel, string, error) {
	skipDegraded := IsAggregateSmartRoutingEnabled(aggregateGroup)
	explicitRouteHandled := false
	if retryPool, retryRouteGroup, retryIndex, ok := resolveExplicitAggregateRetryRoute(param.Ctx); ok {
		explicitRouteHandled = true
		retryBase := param.GetRetry()
		if base, ok := resolveExplicitAggregateRetryBase(param.Ctx); ok {
			retryBase = base
		}
		candidate, found, err := findExplicitAggregateClusterCandidate(param.Ctx, aggregateGroup, param.ModelName, retryPool, retryRouteGroup, retryIndex, skipDegraded)
		if err != nil {
			return nil, retryRouteGroup, err
		}
		if found {
			if retryPool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
				setAggregateClientRouteContext(param.Ctx, model.AggregateGroupClientTypeClaudeCodeCLI, retryPool, candidate.Target.RealGroup, false)
			}
			channel, group, err := selectAggregateClusterChannelFromCandidate(param, aggregateGroup, candidate, retryBase)
			if err != nil || channel != nil {
				return channel, group, err
			}
			markAggregateRouteCandidateAttempted(param.Ctx, candidate)
		}
	}

	if !explicitRouteHandled {
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
	}

	if selection, ok := resolveAggregateClientRoutePoolSelection(param.Ctx, aggregateGroup, param.ModelName); ok {
		channel, group, err := selectAggregateClientRoutePoolChannel(param, aggregateGroup, selection, skipDegraded)
		if err != nil || channel != nil {
			return channel, group, err
		}
		if !selection.Pool.GetFallbackToDefault() {
			setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, "", false)
			return nil, "", nil
		}
		setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, "", true)
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
	currentRoutePool := normalizeAggregateClusterRoutePool(common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool))
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
			setAggregateClusterNextRetryRoute(c, aggregateClusterRouteCandidate{
				Target: model.AggregateGroupTarget{
					RealGroup: transition.FailedGroup,
				},
				Index:     failedIndex,
				RoutePool: currentRoutePool,
			}, retryBase)
			return transition
		}
	}

	skipDegraded := IsAggregateSmartRoutingEnabled(aggregateGroup)
	markAggregateRouteKeyAttempted(c, currentRoutePool, transition.FailedGroup)
	if currentRoutePool == aggregateClusterDefaultRoutePool {
		markAggregateRouteAttempted(c, failedIndex)
	}
	if currentRoutePool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
		clientConfig := aggregateGroup.GetClientRoutePools().ClaudeCodeCLI
		targets := aggregateClientRoutePoolTargetsToClusterTargets(clientConfig.Targets)
		candidate, ok, _, err := chooseAggregateClusterRouteCandidateFromTargets(c, aggregateGroup, modelName, currentRoutePool, targets, skipDegraded, true, "")
		if err != nil {
			logger.LogError(c, "aggregate cluster client route pool retry choose route failed: "+err.Error())
			return transition
		}
		if ok {
			transition.HasNext = true
			transition.NextGroup = candidate.Target.RealGroup
			transition.NextIndex = candidate.Index
			setAggregateClusterNextRetryRoute(c, candidate, currentRetry+1)
			setAggregateClientRouteContext(c, model.AggregateGroupClientTypeClaudeCodeCLI, currentRoutePool, candidate.Target.RealGroup, false)
			return transition
		}
		if !clientConfig.GetFallbackToDefault() {
			return transition
		}
		setAggregateClientRouteContext(c, model.AggregateGroupClientTypeClaudeCodeCLI, currentRoutePool, "", true)
		candidate, ok, _, err = chooseAggregateClusterRouteCandidate(c, aggregateGroup, modelName, skipDegraded, true, "")
		if err != nil {
			logger.LogError(c, "aggregate cluster client route pool fallback choose route failed: "+err.Error())
			return transition
		}
		if !ok {
			return transition
		}
		transition.HasNext = true
		transition.NextGroup = candidate.Target.RealGroup
		transition.NextIndex = candidate.Index
		setAggregateClusterNextRetryRoute(c, candidate, currentRetry+1)
		return transition
	}

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
	setAggregateClusterNextRetryRoute(c, candidate, currentRetry+1)
	return transition
}
