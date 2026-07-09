package controller

import (
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type aggregateGroupTargetRequest struct {
	RealGroup string `json:"real_group"`
	Weight    *int   `json:"weight"`
	RPMLimit  int    `json:"rpm_limit"`
}

type aggregateGroupRouteModelRatioRequest struct {
	RealGroup  string  `json:"real_group"`
	ModelName  string  `json:"model_name"`
	GroupRatio float64 `json:"group_ratio"`
	Enabled    *bool   `json:"enabled"`
}

type aggregateGroupUpsertRequest struct {
	Id                            int                                          `json:"id"`
	Name                          string                                       `json:"name"`
	DisplayName                   string                                       `json:"display_name"`
	Description                   string                                       `json:"description"`
	Status                        int                                          `json:"status"`
	GroupRatio                    float64                                      `json:"group_ratio"`
	RoutingMode                   string                                       `json:"routing_mode"`
	SmartRoutingEnabled           bool                                         `json:"smart_routing_enabled"`
	RecoveryEnabled               bool                                         `json:"recovery_enabled"`
	RecoveryIntervalSeconds       int                                          `json:"recovery_interval_seconds"`
	ClusterAffinityTTLSeconds     int                                          `json:"cluster_affinity_ttl_seconds"`
	RouteAffinityStrategy         string                                       `json:"route_affinity_strategy"`
	RouteAffinityScope            string                                       `json:"route_affinity_scope"`
	RouteAffinityKeySources       []model.AggregateGroupRouteAffinityKeySource `json:"route_affinity_key_sources"`
	RetryStatusCodes              string                                       `json:"retry_status_codes"`
	VisibleUserGroups             []string                                     `json:"visible_user_groups"`
	Targets                       []aggregateGroupTargetRequest                `json:"targets"`
	ClientRoutePools              model.AggregateGroupClientRoutePools         `json:"client_route_pools"`
	SmartStrategyConfig           *model.AggregateGroupSmartStrategyConfig     `json:"smart_strategy_config"`
	RouteModelGroupRatioOverrides *[]aggregateGroupRouteModelRatioRequest      `json:"route_model_group_ratio_overrides"`
}

type aggregateGroupResponse struct {
	Id                                       int                                          `json:"id"`
	Name                                     string                                       `json:"name"`
	DisplayName                              string                                       `json:"display_name"`
	Description                              string                                       `json:"description"`
	Status                                   int                                          `json:"status"`
	GroupRatio                               float64                                      `json:"group_ratio"`
	RoutingMode                              string                                       `json:"routing_mode"`
	SmartRoutingEnabled                      bool                                         `json:"smart_routing_enabled"`
	RecoveryEnabled                          bool                                         `json:"recovery_enabled"`
	RecoveryIntervalSeconds                  int                                          `json:"recovery_interval_seconds"`
	ClusterAffinityTTLSeconds                int                                          `json:"cluster_affinity_ttl_seconds"`
	RouteAffinityStrategy                    string                                       `json:"route_affinity_strategy"`
	RouteAffinityScope                       string                                       `json:"route_affinity_scope"`
	RouteAffinityKeySources                  []model.AggregateGroupRouteAffinityKeySource `json:"route_affinity_key_sources"`
	RetryStatusCodes                         string                                       `json:"retry_status_codes"`
	VisibleUserGroups                        []string                                     `json:"visible_user_groups"`
	Targets                                  []model.AggregateGroupTarget                 `json:"targets"`
	ClientRoutePools                         model.AggregateGroupClientRoutePools         `json:"client_route_pools"`
	SmartStrategyConfig                      *model.AggregateGroupSmartStrategyConfig     `json:"smart_strategy_config"`
	CreatedTime                              int64                                        `json:"created_time"`
	UpdatedTime                              int64                                        `json:"updated_time"`
	RouteModelGroupRatioOverrides            []model.AggregateGroupRouteModelRatio        `json:"route_model_group_ratio_overrides,omitempty"`
	EnabledRouteModelGroupRatioOverrideCount int                                          `json:"enabled_route_model_group_ratio_override_count"`
}

type aggregateGroupSmartStrategyResponse struct {
	GlobalEnabled     bool                                         `json:"global_enabled"`
	GroupEnabled      bool                                         `json:"group_enabled"`
	EffectiveEnabled  bool                                         `json:"effective_enabled"`
	EffectiveStrategy service.AggregateGroupEffectiveSmartStrategy `json:"effective_strategy"`
}

type aggregateGroupRuntimeResponse struct {
	AggregateGroup *aggregateGroupResponse             `json:"aggregate_group"`
	SmartStrategy  aggregateGroupSmartStrategyResponse `json:"smart_strategy"`
	Models         []string                            `json:"models"`
	SelectedModel  string                              `json:"selected_model"`
	Runtime        *service.AggregateGroupRuntimeView  `json:"runtime,omitempty"`
}

func buildAggregateGroupResponse(group *model.AggregateGroup) *aggregateGroupResponse {
	if group == nil {
		return nil
	}
	targets := make([]model.AggregateGroupTarget, 0, len(group.Targets))
	for _, target := range group.Targets {
		targets = append(targets, target)
	}
	return &aggregateGroupResponse{
		Id:                        group.Id,
		Name:                      group.Name,
		DisplayName:               group.DisplayName,
		Description:               group.Description,
		Status:                    group.Status,
		GroupRatio:                group.GroupRatio,
		RoutingMode:               group.GetRoutingMode(),
		SmartRoutingEnabled:       group.SmartRoutingEnabled,
		RecoveryEnabled:           group.RecoveryEnabled,
		RecoveryIntervalSeconds:   group.RecoveryIntervalSeconds,
		ClusterAffinityTTLSeconds: group.GetClusterAffinityTTLSeconds(),
		RouteAffinityStrategy:     group.GetRouteAffinityStrategy(),
		RouteAffinityScope:        group.GetRouteAffinityScope(),
		RouteAffinityKeySources:   group.GetRouteAffinityKeySources(),
		RetryStatusCodes:          group.RetryStatusCodes,
		VisibleUserGroups:         group.GetVisibleUserGroups(),
		Targets:                   targets,
		ClientRoutePools:          group.GetClientRoutePools(),
		SmartStrategyConfig:       group.GetSmartStrategyConfig(),
		CreatedTime:               group.CreatedTime,
		UpdatedTime:               group.UpdatedTime,
	}
}

func buildAggregateGroupResponseWithRouteModelRatios(group *model.AggregateGroup, rules []model.AggregateGroupRouteModelRatio) *aggregateGroupResponse {
	resp := buildAggregateGroupResponse(group)
	if resp == nil {
		return nil
	}
	resp.RouteModelGroupRatioOverrides = rules
	for _, rule := range rules {
		if rule.Enabled {
			resp.EnabledRouteModelGroupRatioOverrideCount++
		}
	}
	return resp
}

func buildAggregateTargetNames(targets []aggregateGroupTargetRequest) []string {
	realGroups := make([]string, 0, len(targets))
	for _, target := range targets {
		realGroup := strings.TrimSpace(target.RealGroup)
		if realGroup == "" {
			continue
		}
		realGroups = append(realGroups, realGroup)
	}
	return realGroups
}

func buildAggregateTargets(targets []aggregateGroupTargetRequest) []model.AggregateGroupTarget {
	modelTargets := make([]model.AggregateGroupTarget, 0, len(targets))
	for _, target := range targets {
		realGroup := strings.TrimSpace(target.RealGroup)
		if realGroup == "" {
			continue
		}
		weight := model.AggregateGroupTargetDefaultWeight
		if target.Weight != nil {
			weight = *target.Weight
		}
		modelTargets = append(modelTargets, model.AggregateGroupTarget{
			RealGroup: realGroup,
			Weight:    common.GetPointer(weight),
			RPMLimit:  target.RPMLimit,
		})
	}
	return modelTargets
}

func buildAggregateRouteModelRatios(input []aggregateGroupRouteModelRatioRequest) []model.AggregateGroupRouteModelRatio {
	rules := make([]model.AggregateGroupRouteModelRatio, 0, len(input))
	for _, item := range input {
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		rules = append(rules, model.AggregateGroupRouteModelRatio{
			RealGroup:  item.RealGroup,
			ModelName:  item.ModelName,
			GroupRatio: item.GroupRatio,
			Enabled:    enabled,
		})
	}
	return rules
}

func GetAggregateGroups(c *gin.Context) {
	groups, err := model.GetAllAggregateGroups(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groupIds := make([]int, 0, len(groups))
	for _, group := range groups {
		groupIds = append(groupIds, group.Id)
	}
	rulesByGroupId, err := model.GetAggregateGroupRouteModelRatiosByGroupIDs(groupIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resp := make([]*aggregateGroupResponse, 0, len(groups))
	for _, group := range groups {
		resp = append(resp, buildAggregateGroupResponseWithRouteModelRatios(group, rulesByGroupId[group.Id]))
	}
	common.ApiSuccess(c, resp)
}

func GetAggregateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.GetAggregateGroupByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rules, err := model.GetAggregateGroupRouteModelRatios(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildAggregateGroupResponseWithRouteModelRatios(group, rules))
}

func GetAggregateGroupTargetModels(c *gin.Context) {
	realGroup := strings.TrimSpace(c.Query("group"))
	if realGroup == "" {
		common.ApiErrorMsg(c, "缺少真实分组")
		return
	}
	models, err := model.GetGroupEnabledChannelModels(realGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	slices.Sort(models)
	common.ApiSuccess(c, models)
}

func GetAggregateGroupRuntime(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := model.GetAggregateGroupByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	models := service.GetModelsForGroup(group.Name)
	slices.Sort(models)

	selectedModel := strings.TrimSpace(c.Query("model"))
	if selectedModel != "" && !slices.Contains(models, selectedModel) {
		common.ApiErrorMsg(c, "模型不属于当前聚合分组")
		return
	}

	var runtimeView *service.AggregateGroupRuntimeView
	if len(models) > 0 {
		if selectedModel == "" {
			selectedModel = models[0]
		}
		runtimeView, err = service.BuildAggregateGroupRuntimeView(group, selectedModel)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	rules, err := model.GetAggregateGroupRouteModelRatios(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, &aggregateGroupRuntimeResponse{
		AggregateGroup: buildAggregateGroupResponseWithRouteModelRatios(group, rules),
		SmartStrategy: aggregateGroupSmartStrategyResponse{
			GlobalEnabled:     setting.AggregateGroupSmartStrategyEnabled,
			GroupEnabled:      group.SmartRoutingEnabled,
			EffectiveEnabled:  service.IsAggregateSmartRoutingEnabled(group),
			EffectiveStrategy: service.GetAggregateGroupEffectiveSmartStrategy(group),
		},
		Models:        models,
		SelectedModel: selectedModel,
		Runtime:       runtimeView,
	})
}

func CreateAggregateGroup(c *gin.Context) {
	var req aggregateGroupUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status == 0 {
		req.Status = model.AggregateGroupStatusEnabled
	}
	group := &model.AggregateGroup{
		Name:                      req.Name,
		DisplayName:               req.DisplayName,
		Description:               req.Description,
		Status:                    req.Status,
		GroupRatio:                req.GroupRatio,
		RoutingMode:               req.RoutingMode,
		SmartRoutingEnabled:       req.SmartRoutingEnabled,
		RecoveryEnabled:           req.RecoveryEnabled,
		RecoveryIntervalSeconds:   req.RecoveryIntervalSeconds,
		ClusterAffinityTTLSeconds: req.ClusterAffinityTTLSeconds,
		RouteAffinityStrategy:     req.RouteAffinityStrategy,
		RouteAffinityScope:        req.RouteAffinityScope,
		RetryStatusCodes:          req.RetryStatusCodes,
	}
	targetGroupNames := buildAggregateTargetNames(req.Targets)
	if err := service.ValidateAggregateGroupConfig(group, req.VisibleUserGroups, targetGroupNames); err != nil {
		common.ApiError(c, err)
		return
	}
	targets := buildAggregateTargets(req.Targets)
	if err := service.ValidateAggregateTargetLimits(targets); err != nil {
		common.ApiError(c, err)
		return
	}
	clientRoutePools, err := service.NormalizeAndValidateAggregateClientRoutePools(req.ClientRoutePools)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	normalizedTargets := service.NormalizeAggregateTargetsWithWeights(targets)
	if err := service.ValidateAggregateRouteRPMLimitConsistency(normalizedTargets, clientRoutePools); err != nil {
		common.ApiError(c, err)
		return
	}
	routeModelRatios := []model.AggregateGroupRouteModelRatio{}
	if req.RouteModelGroupRatioOverrides != nil {
		routeModelRatios, err = service.NormalizeAndValidateAggregateRouteModelRatios(
			buildAggregateRouteModelRatios(*req.RouteModelGroupRatioOverrides),
			normalizedTargets,
			clientRoutePools,
		)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	smartStrategyConfig, err := service.NormalizeAndValidateAggregateSmartStrategyConfig(req.SmartStrategyConfig)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if dup, err := model.IsAggregateGroupNameDuplicated(0, group.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "聚合分组名称已存在")
		return
	}
	if err := group.SetVisibleUserGroups(req.VisibleUserGroups); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetClientRoutePools(clientRoutePools); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetSmartStrategyConfig(smartStrategyConfig); err != nil {
		common.ApiError(c, err)
		return
	}
	routeAffinityKeySources, err := service.NormalizeAndValidateAggregateRouteAffinityKeySources(req.RouteAffinityKeySources)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetRouteAffinityKeySources(routeAffinityKeySources); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.InsertWithTargetsAndRouteModelRatios(normalizedTargets, routeModelRatios); err != nil {
		common.ApiError(c, err)
		return
	}
	createdGroup, err := model.GetAggregateGroupByID(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	createdRules, err := model.GetAggregateGroupRouteModelRatios(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildAggregateGroupResponseWithRouteModelRatios(createdGroup, createdRules))
}

func UpdateAggregateGroup(c *gin.Context) {
	var req aggregateGroupUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id == 0 {
		common.ApiErrorMsg(c, "缺少聚合分组 ID")
		return
	}
	if req.Status == 0 {
		req.Status = model.AggregateGroupStatusEnabled
	}
	group := &model.AggregateGroup{
		Id:                        req.Id,
		Name:                      req.Name,
		DisplayName:               req.DisplayName,
		Description:               req.Description,
		Status:                    req.Status,
		GroupRatio:                req.GroupRatio,
		RoutingMode:               req.RoutingMode,
		SmartRoutingEnabled:       req.SmartRoutingEnabled,
		RecoveryEnabled:           req.RecoveryEnabled,
		RecoveryIntervalSeconds:   req.RecoveryIntervalSeconds,
		ClusterAffinityTTLSeconds: req.ClusterAffinityTTLSeconds,
		RouteAffinityStrategy:     req.RouteAffinityStrategy,
		RouteAffinityScope:        req.RouteAffinityScope,
		RetryStatusCodes:          req.RetryStatusCodes,
	}
	targetGroupNames := buildAggregateTargetNames(req.Targets)
	if err := service.ValidateAggregateGroupConfig(group, req.VisibleUserGroups, targetGroupNames); err != nil {
		common.ApiError(c, err)
		return
	}
	targets := buildAggregateTargets(req.Targets)
	if err := service.ValidateAggregateTargetLimits(targets); err != nil {
		common.ApiError(c, err)
		return
	}
	clientRoutePools, err := service.NormalizeAndValidateAggregateClientRoutePools(req.ClientRoutePools)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	normalizedTargets := service.NormalizeAggregateTargetsWithWeights(targets)
	if err := service.ValidateAggregateRouteRPMLimitConsistency(normalizedTargets, clientRoutePools); err != nil {
		common.ApiError(c, err)
		return
	}
	var routeModelRatios *[]model.AggregateGroupRouteModelRatio
	if req.RouteModelGroupRatioOverrides != nil {
		normalizedRules, normalizeErr := service.NormalizeAndValidateAggregateRouteModelRatios(
			buildAggregateRouteModelRatios(*req.RouteModelGroupRatioOverrides),
			normalizedTargets,
			clientRoutePools,
		)
		if normalizeErr != nil {
			common.ApiError(c, normalizeErr)
			return
		}
		routeModelRatios = &normalizedRules
	}
	smartStrategyConfig, err := service.NormalizeAndValidateAggregateSmartStrategyConfig(req.SmartStrategyConfig)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if dup, err := model.IsAggregateGroupNameDuplicated(group.Id, group.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "聚合分组名称已存在")
		return
	}
	if err := group.SetVisibleUserGroups(req.VisibleUserGroups); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetClientRoutePools(clientRoutePools); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetSmartStrategyConfig(smartStrategyConfig); err != nil {
		common.ApiError(c, err)
		return
	}
	routeAffinityKeySources, err := service.NormalizeAndValidateAggregateRouteAffinityKeySources(req.RouteAffinityKeySources)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.SetRouteAffinityKeySources(routeAffinityKeySources); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := group.UpdateWithTargetsAndRouteModelRatios(normalizedTargets, routeModelRatios); err != nil {
		common.ApiError(c, err)
		return
	}
	updatedGroup, err := model.GetAggregateGroupByID(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	updatedRules, err := model.GetAggregateGroupRouteModelRatios(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildAggregateGroupResponseWithRouteModelRatios(updatedGroup, updatedRules))
}

func DeleteAggregateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteAggregateGroupByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
