package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type AggregateRetryTransition struct {
	AggregateGroup string
	FailedGroup    string
	FailedIndex    int
	NextGroup      string
	NextIndex      int
	HasNext        bool
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
	if ctx == nil || aggregateGroup == nil {
		return 0, 0
	}
	retryBase := 0
	if retryIndex, ok := resolveExplicitAggregateRetryIndex(ctx); ok {
		setAggregateGroupStartIndexes(ctx, retryIndex)
		if base, ok := resolveExplicitAggregateRetryBase(ctx); ok {
			retryBase = base
		}
		return retryIndex, retryBase
	}

	startIndex := 0
	state, err := GetAggregateGroupRuntimeState(aggregateGroup.Name, modelName)
	if err == nil && state != nil && state.ActiveIndex >= 0 && state.ActiveIndex < len(aggregateGroup.Targets) {
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

func selectAggregateGroupChannel(param *RetryParam, aggregateGroup *model.AggregateGroup) (*model.Channel, string, error) {
	if param == nil || param.Ctx == nil || aggregateGroup == nil {
		return nil, "", fmt.Errorf("invalid aggregate group route param")
	}

	startIndex, retryBase := resolveAggregateGroupStartIndex(param.Ctx, aggregateGroup, param.ModelName)
	if startIndex >= len(aggregateGroup.Targets) {
		return nil, "", nil
	}

	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateGroup, aggregateGroup.Name)

	for i := startIndex; i < len(aggregateGroup.Targets); i++ {
		target := aggregateGroup.Targets[i]

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
		if priorityCount > 0 && priorityRetry >= priorityCount {
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
		common.SetContextKey(param.Ctx, constant.ContextKeyAggregateRetryBase, groupRetryBase)
		return channel, target.RealGroup, nil
	}

	return nil, "", nil
}

func PrepareAggregateGroupRetry(c *gin.Context, currentRetry int, modelName string) *AggregateRetryTransition {
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

	priorityCount, err := model.GetSatisfiedChannelPriorityCount(failedGroup, modelName)
	if err == nil && priorityCount > 0 {
		priorityRetry := currentRetry - retryBase
		if priorityRetry < 0 {
			priorityRetry = 0
		}
		if priorityRetry+1 < priorityCount {
			transition.HasNext = true
			transition.NextGroup = failedGroup
			transition.NextIndex = failedIndex
			common.SetContextKey(c, constant.ContextKeyAggregateRetryIndex, failedIndex)
			common.SetContextKey(c, constant.ContextKeyAggregateRetryBase, retryBase)
			return transition
		}
	}

	nextIndex := failedIndex + 1
	transition.NextIndex = nextIndex
	if nextIndex >= 0 && nextIndex < len(aggregateGroup.Targets) {
		transition.HasNext = true
		transition.NextGroup = aggregateGroup.Targets[nextIndex].RealGroup
		common.SetContextKey(c, constant.ContextKeyAggregateRetryIndex, nextIndex)
		common.SetContextKey(c, constant.ContextKeyAggregateRetryBase, currentRetry+1)
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

	state, _ := GetAggregateGroupRuntimeState(aggregateGroup, modelName)
	newState := &AggregateGroupRuntimeState{
		ActiveIndex:   routeGroupIndex,
		ActiveGroup:   routeGroup,
		LastSuccessAt: now,
	}
	if state != nil {
		newState.LastFailAt = state.LastFailAt
		newState.LastSwitchAt = state.LastSwitchAt
	}
	if routeGroupIndex == 0 {
		newState.LastFailAt = 0
		newState.LastSwitchAt = 0
	} else if initialStartIndex != routeGroupIndex {
		newState.LastFailAt = now
		newState.LastSwitchAt = now
	}
	_ = SetAggregateGroupRuntimeState(aggregateGroup, modelName, newState)
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
}
