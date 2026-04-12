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
}

type aggregateGroupUpsertRequest struct {
	Id                      int                           `json:"id"`
	Name                    string                        `json:"name"`
	DisplayName             string                        `json:"display_name"`
	Description             string                        `json:"description"`
	Status                  int                           `json:"status"`
	GroupRatio              float64                       `json:"group_ratio"`
	SmartRoutingEnabled     bool                          `json:"smart_routing_enabled"`
	RecoveryEnabled         bool                          `json:"recovery_enabled"`
	RecoveryIntervalSeconds int                           `json:"recovery_interval_seconds"`
	RetryStatusCodes        string                        `json:"retry_status_codes"`
	VisibleUserGroups       []string                      `json:"visible_user_groups"`
	Targets                 []aggregateGroupTargetRequest `json:"targets"`
}

type aggregateGroupResponse struct {
	Id                      int                          `json:"id"`
	Name                    string                       `json:"name"`
	DisplayName             string                       `json:"display_name"`
	Description             string                       `json:"description"`
	Status                  int                          `json:"status"`
	GroupRatio              float64                      `json:"group_ratio"`
	SmartRoutingEnabled     bool                         `json:"smart_routing_enabled"`
	RecoveryEnabled         bool                         `json:"recovery_enabled"`
	RecoveryIntervalSeconds int                          `json:"recovery_interval_seconds"`
	RetryStatusCodes        string                       `json:"retry_status_codes"`
	VisibleUserGroups       []string                     `json:"visible_user_groups"`
	Targets                 []model.AggregateGroupTarget `json:"targets"`
	CreatedTime             int64                        `json:"created_time"`
	UpdatedTime             int64                        `json:"updated_time"`
}

type aggregateGroupSmartStrategyResponse struct {
	GlobalEnabled    bool `json:"global_enabled"`
	GroupEnabled     bool `json:"group_enabled"`
	EffectiveEnabled bool `json:"effective_enabled"`
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
		Id:                      group.Id,
		Name:                    group.Name,
		DisplayName:             group.DisplayName,
		Description:             group.Description,
		Status:                  group.Status,
		GroupRatio:              group.GroupRatio,
		SmartRoutingEnabled:     group.SmartRoutingEnabled,
		RecoveryEnabled:         group.RecoveryEnabled,
		RecoveryIntervalSeconds: group.RecoveryIntervalSeconds,
		RetryStatusCodes:        group.RetryStatusCodes,
		VisibleUserGroups:       group.GetVisibleUserGroups(),
		Targets:                 targets,
		CreatedTime:             group.CreatedTime,
		UpdatedTime:             group.UpdatedTime,
	}
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

func GetAggregateGroups(c *gin.Context) {
	groups, err := model.GetAllAggregateGroups(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resp := make([]*aggregateGroupResponse, 0, len(groups))
	for _, group := range groups {
		resp = append(resp, buildAggregateGroupResponse(group))
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
	common.ApiSuccess(c, buildAggregateGroupResponse(group))
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

	common.ApiSuccess(c, &aggregateGroupRuntimeResponse{
		AggregateGroup: buildAggregateGroupResponse(group),
		SmartStrategy: aggregateGroupSmartStrategyResponse{
			GlobalEnabled:    setting.AggregateGroupSmartStrategyEnabled,
			GroupEnabled:     group.SmartRoutingEnabled,
			EffectiveEnabled: service.IsAggregateSmartRoutingEnabled(group),
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
		Name:                    req.Name,
		DisplayName:             req.DisplayName,
		Description:             req.Description,
		Status:                  req.Status,
		GroupRatio:              req.GroupRatio,
		SmartRoutingEnabled:     req.SmartRoutingEnabled,
		RecoveryEnabled:         req.RecoveryEnabled,
		RecoveryIntervalSeconds: req.RecoveryIntervalSeconds,
		RetryStatusCodes:        req.RetryStatusCodes,
	}
	targetGroupNames := buildAggregateTargetNames(req.Targets)
	if err := service.ValidateAggregateGroupConfig(group, req.VisibleUserGroups, targetGroupNames); err != nil {
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
	if err := group.InsertWithTargets(service.NormalizeAggregateTargets(targetGroupNames)); err != nil {
		common.ApiError(c, err)
		return
	}
	createdGroup, err := model.GetAggregateGroupByID(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildAggregateGroupResponse(createdGroup))
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
		Id:                      req.Id,
		Name:                    req.Name,
		DisplayName:             req.DisplayName,
		Description:             req.Description,
		Status:                  req.Status,
		GroupRatio:              req.GroupRatio,
		SmartRoutingEnabled:     req.SmartRoutingEnabled,
		RecoveryEnabled:         req.RecoveryEnabled,
		RecoveryIntervalSeconds: req.RecoveryIntervalSeconds,
		RetryStatusCodes:        req.RetryStatusCodes,
	}
	targetGroupNames := buildAggregateTargetNames(req.Targets)
	if err := service.ValidateAggregateGroupConfig(group, req.VisibleUserGroups, targetGroupNames); err != nil {
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
	if err := group.UpdateWithTargets(service.NormalizeAggregateTargets(targetGroupNames)); err != nil {
		common.ApiError(c, err)
		return
	}
	updatedGroup, err := model.GetAggregateGroupByID(group.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildAggregateGroupResponse(updatedGroup))
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
