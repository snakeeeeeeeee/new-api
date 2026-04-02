package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func resolveAggregateGroupStartIndex(ctx *gin.Context, aggregateGroup *model.AggregateGroup, modelName string) int {
	if ctx == nil || aggregateGroup == nil {
		return 0
	}
	if value, ok := common.GetContextKey(ctx, constant.ContextKeyRouteGroupIndex); ok {
		if index, ok := value.(int); ok && index >= 0 {
			return index
		}
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
	common.SetContextKey(ctx, constant.ContextKeyAggregateStartIndex, startIndex)
	return startIndex
}

func selectAggregateGroupChannel(param *RetryParam, aggregateGroup *model.AggregateGroup) (*model.Channel, string, error) {
	if param == nil || param.Ctx == nil || aggregateGroup == nil {
		return nil, "", fmt.Errorf("invalid aggregate group route param")
	}
	startIndex := resolveAggregateGroupStartIndex(param.Ctx, aggregateGroup, param.ModelName)
	if startIndex >= len(aggregateGroup.Targets) {
		startIndex = 0
	}
	common.SetContextKey(param.Ctx, constant.ContextKeyAggregateGroup, aggregateGroup.Name)
	for i := startIndex; i < len(aggregateGroup.Targets); i++ {
		target := aggregateGroup.Targets[i]
		priorityRetry := param.GetRetry()
		if i > startIndex {
			priorityRetry = 0
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
		return channel, target.RealGroup, nil
	}
	return nil, "", nil
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
	} else if startIndex != routeGroupIndex {
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
