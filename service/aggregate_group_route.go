package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AggregateRetryTransition struct {
	AggregateGroup     string
	FailedGroup        string
	FailedIndex        int
	NextGroup          string
	NextIndex          int
	HasNext            bool
	WithinCurrentGroup bool
}

func resolveExplicitAggregateRetryIndex(ctx *gin.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	value, ok := common.GetContextKey(ctx, constant.ContextKeyAggregateRetryIndex)
	if !ok {
		return 0, false
	}
	index, ok := value.(int)
	if !ok || index < 0 {
		return 0, false
	}
	return index, true
}

func resolveExplicitAggregateRetryBase(ctx *gin.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	value, ok := common.GetContextKey(ctx, constant.ContextKeyAggregateRetryBase)
	if !ok {
		return 0, false
	}
	base, ok := value.(int)
	if !ok || base < 0 {
		return 0, false
	}
	return base, true
}

func setAggregateGroupStartIndexes(ctx *gin.Context, startIndex int) {
	if ctx == nil || startIndex < 0 {
		return
	}
	common.SetContextKey(ctx, constant.ContextKeyAggregateStartIndex, startIndex)
	if _, exists := common.GetContextKey(ctx, constant.ContextKeyAggregateInitialStartIndex); !exists {
		common.SetContextKey(ctx, constant.ContextKeyAggregateInitialStartIndex, startIndex)
	}
}

func resolveAggregateGroupStartIndex(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string) (int, int) {
	return resolveAggregateGroupStartIndexForPool(ctx, aggregateGroup, modelName, aggregateClusterDefaultRoutePool, aggregateGroup.Targets)
}

func resolveAggregateGroupStartIndexForPool(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string, routePool string, targets []model.AggregateGroupTarget) (int, int) {
	if ctx == nil || aggregateGroup == nil {
		return 0, 0
	}
	routePool = normalizeAggregateClusterRoutePool(routePool)
	retryBase := 0
	if retryPool, _, retryIndex, ok := resolveExplicitAggregateRetryRoute(ctx); ok {
		if normalizeAggregateClusterRoutePool(retryPool) == routePool {
			setAggregateGroupStartIndexes(ctx, retryIndex)
			if base, ok := resolveExplicitAggregateRetryBase(ctx); ok {
				retryBase = base
			}
			return retryIndex, retryBase
		}
	}
	if routePool == aggregateClusterDefaultRoutePool {
		if retryIndex, ok := resolveExplicitAggregateRetryIndex(ctx); ok {
			setAggregateGroupStartIndexes(ctx, retryIndex)
			if base, ok := resolveExplicitAggregateRetryBase(ctx); ok {
				retryBase = base
			}
			return retryIndex, retryBase
		}
	}

	startIndex := 0
	state, err := GetAggregateGroupRuntimeStateForPool(aggregateGroup.Name, modelName, routePool)
	if err == nil && shouldUseAggregateFailoverRuntimeState(state, len(targets)) {
		startIndex = state.ActiveIndex
		if aggregateGroup.RecoveryEnabled && startIndex > 0 && state.LastFailAt > 0 {
			if common.GetTimestamp()-state.LastFailAt >= int64(aggregateGroup.RecoveryIntervalSeconds) {
				startIndex = 0
			}
		}
	}
	setAggregateGroupStartIndexes(ctx, startIndex)
	return startIndex, retryBase
}

func shouldUseAggregateFailoverRuntimeState(state *AggregateGroupRuntimeState, targetCount int) bool {
	if state == nil || state.ActiveIndex < 0 || state.ActiveIndex >= targetCount {
		return false
	}
	if state.RoutingMode != "" && state.RoutingMode != model.AggregateGroupRoutingModeFailover {
		return false
	}
	if state.ActiveIndex > 0 && state.LastFailAt <= 0 {
		return false
	}
	return true
}

func selectAggregateGroupChannelFromIndex(param *RetryParam, aggregateGroup *model.AggregateGroup, startIndex int, retryBase int, skipDegraded bool) (*model.Channel, string, error) {
	return selectAggregateGroupChannelFromTargets(param, aggregateGroup, aggregateGroup.Targets, aggregateClusterDefaultRoutePool, startIndex, retryBase, skipDegraded)
}

func selectAggregateGroupChannelFromTargets(param *RetryParam, aggregateGroup *model.AggregateGroup, targets []model.AggregateGroupTarget, routePool string, startIndex int, retryBase int, skipDegraded bool) (*model.Channel, string, error) {
	routePool = normalizeAggregateClusterRoutePool(routePool)
	for i := startIndex; i < len(targets); i++ {
		target := targets[i]
		if skipDegraded {
			degraded, state, recovered, err := IsAggregateGroupRouteTemporarilyDegraded(aggregateGroup.Name, param.ModelName, target.RealGroup)
			if err != nil {
				return nil, target.RealGroup, err
			}
			if recovered {
				logger.LogInfo(param.Ctx, fmt.Sprintf("aggregate smart strategy recovered route: aggregate_group=%s, model=%s, route_group=%s", aggregateGroup.Name, param.ModelName, target.RealGroup))
			}
			if degraded {
				logger.LogWarn(param.Ctx, fmt.Sprintf("aggregate smart strategy skipped degraded route: aggregate_group=%s, model=%s, route_group=%s(index=%d), degraded_until=%d", aggregateGroup.Name, param.ModelName, target.RealGroup, i, state.DegradedUntil))
				continue
			}
		}

		groupRetryBase := retryBase
		if i > startIndex {
			groupRetryBase = param.GetRetry()
		}
		priorityRetry := param.GetRetry() - groupRetryBase
		if priorityRetry < 0 {
			priorityRetry = 0
		}

		priorityCount, err := model.GetSatisfiedChannelPriorityCount(target.RealGroup, param.ModelName)
		if err != nil {
			return nil, target.RealGroup, err
		}
		if priorityCount <= 0 {
			continue
		}

		channel, err := model.GetRandomSatisfiedChannel(target.RealGroup, param.ModelName, priorityRetry)
		if err != nil {
			return nil, target.RealGroup, err
		}
		if channel == nil {
			continue
		}

		common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroup, target.RealGroup)
		common.SetContextKey(param.Ctx, constant.ContextKeyRouteGroupIndex, i)
		common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRoutePool, routePool)
		common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRetryBase, groupRetryBase)
		if clientType := common.GetContextKeyString(param.Ctx, constant.ContextKeyAggregateClientType); clientType != "" {
			clientRoutePool := common.GetContextKeyString(param.Ctx, constant.ContextKeyAggregateClientRoutePool)
			if clientRoutePool != "" && routePool == clientRoutePool {
				setAggregateClientRouteContext(param.Ctx, clientType, clientRoutePool, target.RealGroup, false)
			} else if common.GetContextKeyBool(param.Ctx, constant.ContextKeyAggregateClientRouteFallback) {
				setAggregateClientRouteContext(param.Ctx, clientType, common.GetContextKeyString(param.Ctx, constant.ContextKeyAggregateClientRoutePool), target.RealGroup, true)
			}
		}
		RecordAggregateRouteRPMAttempt(param.Ctx, param.ModelName, target.RealGroup)
		return channel, target.RealGroup, nil
	}

	return nil, "", nil
}

func selectAggregateClientRoutePoolFailoverChannel(param *RetryParam, aggregateGroup *model.AggregateGroup, selection aggregateClientRoutePoolSelection, skipDegraded bool) (*model.Channel, string, error) {
	targets := aggregateClientRoutePoolTargetsToClusterTargets(selection.Pool.Targets)
	if len(targets) == 0 {
		return nil, "", nil
	}
	startIndex, retryBase := resolveAggregateGroupStartIndexForPool(param.Ctx, aggregateGroup, param.ModelName, selection.PoolName, targets)
	if startIndex >= len(targets) {
		return nil, "", nil
	}
	setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, "", false)
	if skipDegraded {
		channel, group, err := selectAggregateGroupChannelFromTargets(param, aggregateGroup, targets, selection.PoolName, startIndex, retryBase, true)
		if err != nil || channel != nil {
			return channel, group, err
		}
	}
	return selectAggregateGroupChannelFromTargets(param, aggregateGroup, targets, selection.PoolName, startIndex, retryBase, false)
}

func selectAggregateGroupChannel(param *RetryParam, aggregateGroup *model.AggregateGroup) (*model.Channel, string, error) {
	if param == nil || param.Ctx == nil || aggregateGroup == nil {
		return nil, "", fmt.Errorf("invalid aggregate group route param")
	}

	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateGroup, aggregateGroup.Name)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateSmartRouting, IsAggregateSmartRoutingEnabled(aggregateGroup))
	routingMode := aggregateGroup.GetRoutingMode()
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRoutingMode, routingMode)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRecoveryEnabled, aggregateGroup.RecoveryEnabled)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRecoveryInterval, aggregateGroup.RecoveryIntervalSeconds)
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateClusterAffinityTTL, aggregateGroup.GetClusterAffinityTTLSeconds())
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRouteAffinityStrategy, aggregateGroup.GetRouteAffinityStrategy())
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRouteAffinitySources, aggregateGroup.GetRouteAffinityKeySources())

	if routingMode == model.AggregateGroupRoutingModeCluster {
		return selectAggregateGroupClusterChannel(param, aggregateGroup)
	}

	if selection, ok := resolveAggregateClientRoutePoolSelection(param.Ctx, aggregateGroup, param.ModelName); ok {
		retryPool, _, _, hasRetryPool := resolveExplicitAggregateRetryRoute(param.Ctx)
		shouldTryClientPool := !(hasRetryPool &&
			normalizeAggregateClusterRoutePool(retryPool) == aggregateClusterDefaultRoutePool &&
			common.GetContextKeyBool(param.Ctx, constant.ContextKeyAggregateClientRouteFallback))
		if shouldTryClientPool {
			channel, group, err := selectAggregateClientRoutePoolFailoverChannel(param, aggregateGroup, selection, IsAggregateSmartRoutingEnabled(aggregateGroup))
			if err != nil || channel != nil {
				return channel, group, err
			}
			if !selection.Pool.GetFallbackToDefault() {
				setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, "", false)
				return nil, "", nil
			}
			setAggregateClientRouteContext(param.Ctx, selection.ClientType, selection.PoolName, "", true)
		}
	}

	startIndex, retryBase := resolveAggregateGroupStartIndex(param.Ctx, aggregateGroup, param.ModelName)
	if startIndex >= len(aggregateGroup.Targets) {
		return nil, "", nil
	}

	if IsAggregateSmartRoutingEnabled(aggregateGroup) {
		channel, group, err := selectAggregateGroupChannelFromIndex(param, aggregateGroup, startIndex, retryBase, true)
		if err != nil {
			return nil, group, err
		}
		if channel != nil {
			return channel, group, nil
		}
	}

	return selectAggregateGroupChannelFromIndex(param, aggregateGroup, startIndex, retryBase, false)
}

func PrepareAggregateGroupRetry(c *gin.Context, currentRetry int, modelName string, maxInternalRetries int) *AggregateRetryTransition {
	if c == nil {
		return nil
	}

	aggregateGroupName := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	failedGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	if aggregateGroupName == "" || failedGroup == "" {
		return nil
	}

	aggregateGroup, ok := GetAggregateGroup(aggregateGroupName, true)
	if !ok || aggregateGroup == nil {
		return nil
	}

	failedIndex := common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex)
	retryBase, _ := resolveExplicitAggregateRetryBase(c)

	transition := &AggregateRetryTransition{
		AggregateGroup: aggregateGroupName,
		FailedGroup:    failedGroup,
		FailedIndex:    failedIndex,
	}

	if aggregateGroup.GetRoutingMode() == model.AggregateGroupRoutingModeCluster {
		return prepareAggregateClusterRetry(c, aggregateGroup, transition, currentRetry, modelName, maxInternalRetries)
	}

	currentRoutePool := normalizeAggregateClusterRoutePool(common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool))
	targets := aggregateGroup.Targets
	if currentRoutePool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
		targets = aggregateClientRoutePoolTargetsToClusterTargets(aggregateGroup.GetClientRoutePools().ClaudeCodeCLI.Targets)
	}

	priorityCount, err := model.GetSatisfiedChannelPriorityCount(failedGroup, modelName)
	if err == nil && priorityCount > 0 {
		priorityRetry := currentRetry - retryBase
		if priorityRetry < 0 {
			priorityRetry = 0
		}
		if priorityRetry < maxInternalRetries {
			transition.HasNext = true
			transition.WithinCurrentGroup = true
			transition.NextGroup = failedGroup
			transition.NextIndex = failedIndex
			setAggregateClusterNextRetryRoute(c, aggregateClusterRouteCandidate{
				Target: model.AggregateGroupTarget{
					RealGroup: failedGroup,
				},
				Index:     failedIndex,
				RoutePool: currentRoutePool,
			}, retryBase)
			return transition
		}
	}

	nextIndex := failedIndex + 1
	transition.NextIndex = nextIndex
	if nextIndex >= 0 && nextIndex < len(targets) {
		transition.HasNext = true
		transition.NextGroup = targets[nextIndex].RealGroup
		setAggregateClusterNextRetryRoute(c, aggregateClusterRouteCandidate{
			Target:    targets[nextIndex],
			Index:     nextIndex,
			RoutePool: currentRoutePool,
		}, currentRetry+1)
		if currentRoutePool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
			setAggregateClientRouteContext(c, model.AggregateGroupClientTypeClaudeCodeCLI, currentRoutePool, targets[nextIndex].RealGroup, false)
		}
		return transition
	}
	if currentRoutePool == model.AggregateGroupClientRoutePoolClaudeCodeCLI {
		clientConfig := aggregateGroup.GetClientRoutePools().ClaudeCodeCLI
		if !clientConfig.GetFallbackToDefault() || len(aggregateGroup.Targets) == 0 {
			return transition
		}
		transition.HasNext = true
		transition.NextIndex = 0
		transition.NextGroup = aggregateGroup.Targets[0].RealGroup
		setAggregateClientRouteContext(c, model.AggregateGroupClientTypeClaudeCodeCLI, currentRoutePool, "", true)
		setAggregateClusterNextRetryRoute(c, aggregateClusterRouteCandidate{
			Target:    aggregateGroup.Targets[0],
			Index:     0,
			RoutePool: aggregateClusterDefaultRoutePool,
		}, currentRetry+1)
		return transition
	}

	return transition
}

func RecordAggregateRouteSuccess(c *gin.Context, modelName string) {
	if c == nil || modelName == "" {
		return
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	if aggregateGroup == "" || routeGroup == "" {
		return
	}
	routeGroupIndex := common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex)
	startIndex := common.GetContextKeyInt(c, constant.ContextKeyAggregateStartIndex)
	initialStartIndex := common.GetContextKeyInt(c, constant.ContextKeyAggregateInitialStartIndex)
	if initialStartIndex < 0 {
		initialStartIndex = startIndex
	}
	now := common.GetTimestamp()
	routingMode := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutingMode)
	if routingMode == "" {
		routingMode = model.AggregateGroupRoutingModeFailover
	}
	routePool := normalizeAggregateClusterRoutePool(common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool))
	clientType := common.GetContextKeyString(c, constant.ContextKeyAggregateClientType)
	clientRoutePool := common.GetContextKeyString(c, constant.ContextKeyAggregateClientRoutePool)
	clientFallback := common.GetContextKeyBool(c, constant.ContextKeyAggregateClientRouteFallback)
	shouldRecordRuntimeState := true
	runtimeRoutePool := routePool
	if clientType != "" && clientRoutePool != "" {
		if clientFallback {
			shouldRecordRuntimeState = false
		} else {
			runtimeRoutePool = clientRoutePool
		}
	}

	var state *AggregateGroupRuntimeState
	if shouldRecordRuntimeState {
		state, _ = GetAggregateGroupRuntimeStateForPool(aggregateGroup, modelName, runtimeRoutePool)
	}
	previousActiveGroup := ""
	previousActiveIndex := -1
	previousActiveSinceAt := int64(0)
	newState := &AggregateGroupRuntimeState{
		ActiveIndex:   routeGroupIndex,
		ActiveGroup:   routeGroup,
		RoutingMode:   routingMode,
		LastSuccessAt: now,
	}
	if state != nil {
		previousActiveGroup = state.ActiveGroup
		previousActiveIndex = state.ActiveIndex
		previousActiveSinceAt = state.ActiveSinceAt
		newState.LastFailAt = state.LastFailAt
		newState.LastSwitchAt = state.LastSwitchAt
		newState.ActiveSinceAt = state.ActiveSinceAt
	}
	currentRouteChanged := previousActiveGroup != routeGroup || previousActiveIndex != routeGroupIndex
	switchOccurred := initialStartIndex != routeGroupIndex || (state != nil && currentRouteChanged)
	if currentRouteChanged || previousActiveSinceAt == 0 {
		newState.ActiveSinceAt = now
	}
	if routeGroupIndex == 0 {
		newState.LastFailAt = 0
	} else if initialStartIndex != routeGroupIndex {
		newState.LastFailAt = now
	}
	if switchOccurred {
		newState.LastSwitchAt = now
	}
	if shouldRecordRuntimeState {
		_ = SetAggregateGroupRuntimeStateForPool(aggregateGroup, modelName, runtimeRoutePool, newState)
	}
	RecordAggregateRouteRPMSuccess(c, modelName, routeGroup)
	if routingMode == model.AggregateGroupRoutingModeCluster {
		if clientType != "" && clientRoutePool != "" {
			if !clientFallback && routePool == clientRoutePool {
				RecordAggregateRouteAffinityForPool(c, modelName, aggregateGroup, clientRoutePool, routeGroup)
			}
		} else {
			RecordAggregateRouteAffinity(c, modelName, aggregateGroup, routeGroup)
		}
	}
	RecordAggregateRouteSmartSuccess(c, modelName, routeGroup)
}

func AppendAggregateGroupAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	aggregateGroup := common.GetContextKeyString(c, constant.ContextKeyAggregateGroup)
	routeGroup := common.GetContextKeyString(c, constant.ContextKeyRouteGroup)
	if aggregateGroup == "" || routeGroup == "" {
		return
	}
	adminInfo["aggregate_group"] = aggregateGroup
	adminInfo["route_group"] = routeGroup
	adminInfo["route_group_index"] = common.GetContextKeyInt(c, constant.ContextKeyRouteGroupIndex)
	adminInfo["aggregate_start_index"] = common.GetContextKeyInt(c, constant.ContextKeyAggregateStartIndex)
	if routingMode := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutingMode); routingMode != "" {
		adminInfo["aggregate_routing_mode"] = routingMode
	}
	if routePool := common.GetContextKeyString(c, constant.ContextKeyAggregateRoutePool); routePool != "" {
		adminInfo["aggregate_route_pool"] = routePool
	}
	if strategy := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityStrategy); strategy != "" {
		affinityInfo := map[string]interface{}{
			"strategy": strategy,
		}
		if sourceType := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinitySourceType); sourceType != "" {
			affinityInfo["source_type"] = sourceType
		}
		if sourceKey := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinitySourceKey); sourceKey != "" {
			affinityInfo["source_key"] = sourceKey
		}
		if sourcePath := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinitySourcePath); sourcePath != "" {
			affinityInfo["source_path"] = sourcePath
		}
		if keyHint := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityKeyHint); keyHint != "" {
			affinityInfo["key_hint"] = keyHint
		}
		if keyFP := common.GetContextKeyString(c, constant.ContextKeyAggregateRouteAffinityKeyFP); keyFP != "" {
			affinityInfo["key_fp"] = keyFP
		}
		adminInfo["aggregate_route_affinity"] = affinityInfo
	}
	if clientType := common.GetContextKeyString(c, constant.ContextKeyAggregateClientType); clientType != "" {
		adminInfo["client_type"] = clientType
	}
	if clientRoutePool := common.GetContextKeyString(c, constant.ContextKeyAggregateClientRoutePool); clientRoutePool != "" {
		adminInfo["client_route_pool"] = clientRoutePool
	}
	if clientRouteTarget := common.GetContextKeyString(c, constant.ContextKeyAggregateClientRouteTarget); clientRouteTarget != "" {
		adminInfo["client_route_target"] = clientRouteTarget
	}
	if _, ok := common.GetContextKey(c, constant.ContextKeyAggregateClientRouteFallback); ok {
		adminInfo["client_route_fallback"] = common.GetContextKeyBool(c, constant.ContextKeyAggregateClientRouteFallback)
	}
}
