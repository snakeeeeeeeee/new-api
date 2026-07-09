package service

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func IsAggregateGroup(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := model.GetAggregateGroupByName(name, false)
	return err == nil
}

func GetAggregateGroup(name string, enabledOnly bool) (*model.AggregateGroup, bool) {
	group, err := model.GetAggregateGroupByName(name, enabledOnly)
	if err != nil {
		return nil, false
	}
	return group, true
}

func canUserGroupSeeAggregate(userGroup string, aggregateGroup *model.AggregateGroup) bool {
	if aggregateGroup == nil || !aggregateGroup.IsEnabled() {
		return false
	}
	visibleUserGroups := aggregateGroup.GetVisibleUserGroups()
	if len(visibleUserGroups) == 0 {
		return false
	}
	return slices.Contains(visibleUserGroups, userGroup)
}

func GetVisibleAggregateGroups(userGroup string) []*model.AggregateGroup {
	return GetVisibleAggregateGroupsWithSetting(userGroup, dto.UserSetting{})
}

func GetVisibleAggregateGroupsWithSetting(userGroup string, userSetting dto.UserSetting) []*model.AggregateGroup {
	if strings.TrimSpace(userGroup) == "" {
		return []*model.AggregateGroup{}
	}
	groups, err := model.GetAllAggregateGroups(true)
	if err != nil {
		return []*model.AggregateGroup{}
	}
	visibleGroups := make([]*model.AggregateGroup, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if canUserGroupSeeAggregate(userGroup, group) {
			seen[group.Name] = struct{}{}
			visibleGroups = append(visibleGroups, group)
		}
	}
	for _, groupName := range userSetting.ExtraUsableGroups {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" {
			continue
		}
		if _, exists := seen[groupName]; exists {
			continue
		}
		if aggregateGroup, ok := GetAggregateGroup(groupName, true); ok {
			seen[groupName] = struct{}{}
			visibleGroups = append(visibleGroups, aggregateGroup)
		}
	}
	return visibleGroups
}

func GetAggregateGroupRatio(group string) (float64, bool) {
	aggregateGroup, ok := GetAggregateGroup(group, true)
	if !ok {
		return 0, false
	}
	return aggregateGroup.GroupRatio, true
}

type GroupRatioView struct {
	Ratio            float64  `json:"ratio"`
	OriginalRatio    float64  `json:"original_ratio"`
	RatioOverride    *float64 `json:"ratio_override,omitempty"`
	HasRatioOverride bool     `json:"has_ratio_override"`
}

type ModelGroupRatioView struct {
	GroupRatioView
	MaxRatio     float64 `json:"max_ratio"`
	DynamicRoute bool    `json:"dynamic_route"`
}

func GetAggregateGroupRatioOverride(userSetting dto.UserSetting, aggregateGroup string) (float64, bool) {
	if strings.TrimSpace(aggregateGroup) == "" || userSetting.AggregateGroupRatioOverrides == nil {
		return 0, false
	}
	ratio, ok := userSetting.AggregateGroupRatioOverrides[aggregateGroup]
	if !ok || ratio < 0 {
		return 0, false
	}
	return ratio, true
}

func GetAggregateGroupRatioView(userSetting dto.UserSetting, aggregateGroup string) (GroupRatioView, bool) {
	group, ok := GetAggregateGroup(aggregateGroup, true)
	if !ok {
		return GroupRatioView{}, false
	}
	return getAggregateGroupRatioView(userSetting, group), true
}

func getAggregateGroupRatioView(userSetting dto.UserSetting, aggregateGroup *model.AggregateGroup) GroupRatioView {
	ratio := aggregateGroup.GroupRatio
	view := GroupRatioView{
		Ratio:         ratio,
		OriginalRatio: ratio,
	}
	if override, hasOverride := GetAggregateGroupRatioOverride(userSetting, aggregateGroup.Name); hasOverride {
		view.Ratio = override
		view.RatioOverride = common.GetPointer(override)
		view.HasRatioOverride = true
	}
	return view
}

func GetAggregateModelGroupRatioView(
	userSetting dto.UserSetting,
	aggregateGroup *model.AggregateGroup,
	modelName string,
	rules []model.AggregateGroupRouteModelRatio,
	modelEnabledRealGroups []string,
) (ModelGroupRatioView, bool) {
	if aggregateGroup == nil || strings.TrimSpace(modelName) == "" {
		return ModelGroupRatioView{}, false
	}
	baseView := getAggregateGroupRatioView(userSetting, aggregateGroup)
	view := ModelGroupRatioView{
		GroupRatioView: baseView,
		MaxRatio:       baseView.Ratio,
	}

	enabledGroupSet := make(map[string]struct{}, len(modelEnabledRealGroups))
	for _, realGroup := range modelEnabledRealGroups {
		enabledGroupSet[strings.TrimSpace(realGroup)] = struct{}{}
	}
	ruleRatios := make(map[string]float64)
	for _, rule := range rules {
		if !rule.Enabled || rule.ModelName != modelName {
			continue
		}
		ruleRatios[rule.RealGroup] = rule.GroupRatio
	}

	hasReachableRoute := false
	for _, realGroup := range getAggregateGroupModelSourceGroups(aggregateGroup) {
		if _, enabled := enabledGroupSet[realGroup]; !enabled {
			continue
		}
		ratio := baseView.Ratio
		if routeRatio, matched := ruleRatios[realGroup]; matched {
			ratio = routeRatio
			view.DynamicRoute = true
		}
		if !hasReachableRoute || ratio > view.MaxRatio {
			view.MaxRatio = ratio
		}
		hasReachableRoute = true
	}
	return view, hasReachableRoute
}

func GetUserVisibleGroups(userGroup string) map[string]string {
	return GetUserUsableGroups(userGroup)
}

func GetUserVisibleGroupsWithSetting(userGroup string, userSetting dto.UserSetting) map[string]string {
	return GetUserUsableGroupsWithSetting(userGroup, userSetting)
}

func CanUserSelectGroup(userGroup, group string) bool {
	_, ok := GetUserVisibleGroups(userGroup)[group]
	return ok
}

func CanUserSelectGroupWithSetting(userGroup, group string, userSetting dto.UserSetting) bool {
	_, ok := GetUserVisibleGroupsWithSetting(userGroup, userSetting)[group]
	return ok
}

func GetUserGroupRatio(userGroup, group string) float64 {
	return GetUserGroupRatioWithSetting(userGroup, group, dto.UserSetting{})
}

func GetUserGroupRatioWithSetting(userGroup, group string, userSetting dto.UserSetting) float64 {
	return GetUserGroupRatioView(userGroup, group, userSetting).Ratio
}

func GetUserGroupRatioView(userGroup, group string, userSetting dto.UserSetting) GroupRatioView {
	if view, ok := GetAggregateGroupRatioView(userSetting, group); ok {
		return view
	}
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return GroupRatioView{Ratio: ratio, OriginalRatio: ratio}
	}
	ratio = ratio_setting.GetGroupRatio(group)
	return GroupRatioView{Ratio: ratio, OriginalRatio: ratio}
}

func GetModelsForGroup(group string) []string {
	if aggregateGroup, ok := GetAggregateGroup(group, true); ok {
		modelSet := make(map[string]struct{})
		models := make([]string, 0)
		for _, targetGroup := range getAggregateGroupModelSourceGroups(aggregateGroup) {
			for _, modelName := range model.GetGroupEnabledModels(targetGroup) {
				if _, exists := modelSet[modelName]; exists {
					continue
				}
				modelSet[modelName] = struct{}{}
				models = append(models, modelName)
			}
		}
		return models
	}
	return model.GetGroupEnabledModels(group)
}

func getAggregateGroupModelSourceGroups(aggregateGroup *model.AggregateGroup) []string {
	if aggregateGroup == nil {
		return []string{}
	}
	groups := make([]string, 0, len(aggregateGroup.Targets))
	seen := make(map[string]struct{})
	addGroup := func(group string) {
		group = strings.TrimSpace(group)
		if group == "" {
			return
		}
		if _, exists := seen[group]; exists {
			return
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	for _, targetGroup := range aggregateGroup.GetTargetGroups() {
		addGroup(targetGroup)
	}
	clientPools := aggregateGroup.GetClientRoutePools()
	if clientPools.Enabled && clientPools.ClaudeCodeCLI.Enabled {
		for _, target := range clientPools.ClaudeCodeCLI.Targets {
			addGroup(target.RealGroup)
		}
	}
	return groups
}

func MapVisibleModelGroups(userGroup string, realGroups []string) []string {
	return MapVisibleModelGroupsWithSetting(userGroup, realGroups, dto.UserSetting{})
}

func MapVisibleModelGroupsWithSetting(userGroup string, realGroups []string, userSetting dto.UserSetting) []string {
	visibleGroups := GetUserVisibleGroupsWithSetting(userGroup, userSetting)
	if len(visibleGroups) == 0 {
		return []string{}
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, realGroup := range realGroups {
		if _, ok := visibleGroups[realGroup]; ok {
			if _, exists := seen[realGroup]; !exists {
				seen[realGroup] = struct{}{}
				result = append(result, realGroup)
			}
		}
	}
	for _, aggregateGroup := range GetVisibleAggregateGroupsWithSetting(userGroup, userSetting) {
		for _, targetGroup := range aggregateGroup.GetTargetGroups() {
			if !slices.Contains(realGroups, targetGroup) {
				continue
			}
			if _, exists := seen[aggregateGroup.Name]; !exists {
				seen[aggregateGroup.Name] = struct{}{}
				result = append(result, aggregateGroup.Name)
			}
			break
		}
	}
	return result
}

func NormalizeAggregateTargets(realGroups []string) []model.AggregateGroupTarget {
	targets := make([]model.AggregateGroupTarget, 0, len(realGroups))
	seen := make(map[string]struct{})
	for _, group := range realGroups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		targets = append(targets, model.AggregateGroupTarget{
			RealGroup:  group,
			OrderIndex: len(targets),
			Weight:     common.GetPointer(model.AggregateGroupTargetDefaultWeight),
			RPMLimit:   0,
		})
	}
	return targets
}

func NormalizeAggregateTargetsWithWeights(inputTargets []model.AggregateGroupTarget) []model.AggregateGroupTarget {
	targets := make([]model.AggregateGroupTarget, 0, len(inputTargets))
	seen := make(map[string]struct{})
	for _, target := range inputTargets {
		realGroup := strings.TrimSpace(target.RealGroup)
		if realGroup == "" {
			continue
		}
		if _, ok := seen[realGroup]; ok {
			continue
		}
		seen[realGroup] = struct{}{}
		weight := model.AggregateGroupTargetDefaultWeight
		if target.Weight != nil {
			weight = *target.Weight
		}
		if weight < 0 {
			weight = 0
		}
		targets = append(targets, model.AggregateGroupTarget{
			RealGroup:  realGroup,
			OrderIndex: len(targets),
			Weight:     common.GetPointer(weight),
			RPMLimit:   target.GetRPMLimit(),
		})
	}
	return targets
}

func ValidateAggregateTargetLimits(targets []model.AggregateGroupTarget) error {
	for _, target := range targets {
		if target.Weight != nil && *target.Weight < 0 {
			return fmt.Errorf("真实分组 %s 权重不能小于 0", strings.TrimSpace(target.RealGroup))
		}
		if target.RPMLimit < 0 {
			return fmt.Errorf("真实分组 %s RPM 上限不能小于 0", strings.TrimSpace(target.RealGroup))
		}
	}
	return nil
}

func ValidateAggregateTargetWeights(targets []model.AggregateGroupTarget) error {
	return ValidateAggregateTargetLimits(targets)
}

func GetConfiguredAggregateRouteGroups(targets []model.AggregateGroupTarget, clientRoutePools model.AggregateGroupClientRoutePools) []string {
	groups := make([]string, 0, len(targets)+len(clientRoutePools.ClaudeCodeCLI.Targets))
	seen := make(map[string]struct{})
	add := func(realGroup string) {
		realGroup = strings.TrimSpace(realGroup)
		if realGroup == "" {
			return
		}
		if _, exists := seen[realGroup]; exists {
			return
		}
		seen[realGroup] = struct{}{}
		groups = append(groups, realGroup)
	}
	for _, target := range targets {
		add(target.RealGroup)
	}
	for _, target := range clientRoutePools.ClaudeCodeCLI.Targets {
		add(target.RealGroup)
	}
	return groups
}

func NormalizeAndValidateAggregateRouteModelRatios(
	input []model.AggregateGroupRouteModelRatio,
	targets []model.AggregateGroupTarget,
	clientRoutePools model.AggregateGroupClientRoutePools,
) ([]model.AggregateGroupRouteModelRatio, error) {
	allowedGroups := make(map[string]struct{})
	for _, realGroup := range GetConfiguredAggregateRouteGroups(targets, clientRoutePools) {
		allowedGroups[realGroup] = struct{}{}
	}

	rules := make([]model.AggregateGroupRouteModelRatio, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, rule := range input {
		realGroup := strings.TrimSpace(rule.RealGroup)
		modelName := strings.TrimSpace(rule.ModelName)
		if realGroup == "" {
			return nil, errors.New("子分组模型倍率的真实分组不能为空")
		}
		if _, exists := allowedGroups[realGroup]; !exists {
			return nil, fmt.Errorf("子分组模型倍率引用了未配置的真实分组 %s", realGroup)
		}
		if modelName == "" {
			return nil, fmt.Errorf("真实分组 %s 的模型名称不能为空", realGroup)
		}
		if math.IsNaN(rule.GroupRatio) || math.IsInf(rule.GroupRatio, 0) || rule.GroupRatio < 0 {
			return nil, fmt.Errorf("真实分组 %s 模型 %s 的倍率必须是大于等于 0 的有限数值", realGroup, modelName)
		}
		ruleKey := model.AggregateGroupRouteModelRatioRuleKey(realGroup, modelName)
		if _, exists := seen[ruleKey]; exists {
			return nil, fmt.Errorf("真实分组 %s 模型 %s 的倍率重复配置", realGroup, modelName)
		}
		seen[ruleKey] = struct{}{}
		rules = append(rules, model.AggregateGroupRouteModelRatio{
			RealGroup:  realGroup,
			ModelName:  modelName,
			GroupRatio: rule.GroupRatio,
			Enabled:    rule.Enabled,
		})
	}
	return rules, nil
}

func NormalizeAndValidateAggregateRouteAffinityKeySources(inputSources []model.AggregateGroupRouteAffinityKeySource) ([]model.AggregateGroupRouteAffinityKeySource, error) {
	sources := make([]model.AggregateGroupRouteAffinityKeySource, 0, len(inputSources))
	seen := make(map[string]struct{}, len(inputSources))
	for _, source := range inputSources {
		sourceType := strings.TrimSpace(source.Type)
		key := strings.TrimSpace(source.Key)
		path := strings.TrimSpace(source.Path)
		if sourceType == "" && key == "" && path == "" {
			continue
		}
		switch sourceType {
		case "header", "query", "context_int", "context_string":
			if key == "" {
				return nil, fmt.Errorf("亲和来源 %s 缺少 key", sourceType)
			}
		case "gjson":
			if path == "" {
				return nil, errors.New("亲和来源 gjson 缺少 path")
			}
		default:
			return nil, fmt.Errorf("亲和来源类型无效: %s", sourceType)
		}
		dedupeKey := sourceType + "\n" + key + "\n" + path
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}
		sources = append(sources, model.AggregateGroupRouteAffinityKeySource{
			Type: sourceType,
			Key:  key,
			Path: path,
		})
	}
	return sources, nil
}

func NormalizeAndValidateAggregateClientRoutePools(config model.AggregateGroupClientRoutePools) (model.AggregateGroupClientRoutePools, error) {
	config = model.NormalizeAggregateGroupClientRoutePools(config)
	normalizedTargets, err := normalizeAndValidateAggregateClientRoutePoolTargets(
		model.AggregateGroupClientRoutePoolClaudeCodeCLI,
		config.ClaudeCodeCLI.Targets,
	)
	if err != nil {
		return config, err
	}
	config.ClaudeCodeCLI.Targets = normalizedTargets
	return config, nil
}

func normalizeAndValidateAggregateClientRoutePoolTargets(poolName string, inputTargets []model.AggregateGroupClientRoutePoolTarget) ([]model.AggregateGroupClientRoutePoolTarget, error) {
	targets := make([]model.AggregateGroupClientRoutePoolTarget, 0, len(inputTargets))
	seen := make(map[string]struct{})
	for _, target := range inputTargets {
		realGroup := strings.TrimSpace(target.RealGroup)
		if realGroup == "" {
			continue
		}
		if _, exists := seen[realGroup]; exists {
			return nil, fmt.Errorf("客户端专用流量池 %s 重复配置真实分组 %s", poolName, realGroup)
		}
		seen[realGroup] = struct{}{}
		weight := model.AggregateGroupTargetDefaultWeight
		if target.Weight != nil {
			weight = *target.Weight
		}
		if weight < 0 {
			return nil, fmt.Errorf("客户端专用流量池 %s 真实分组 %s 权重不能小于 0", poolName, realGroup)
		}
		if target.RPMLimit < 0 {
			return nil, fmt.Errorf("客户端专用流量池 %s 真实分组 %s RPM 上限不能小于 0", poolName, realGroup)
		}
		if realGroup == "auto" {
			return nil, fmt.Errorf("客户端专用流量池 %s 真实分组不能为 auto", poolName)
		}
		if !ratio_setting.ContainsGroupRatio(realGroup) {
			return nil, fmt.Errorf("客户端专用流量池 %s 真实分组 %s 不存在", poolName, realGroup)
		}
		if IsAggregateGroup(realGroup) {
			return nil, fmt.Errorf("客户端专用流量池 %s 真实分组 %s 不能引用聚合分组", poolName, realGroup)
		}
		targets = append(targets, model.AggregateGroupClientRoutePoolTarget{
			RealGroup: realGroup,
			Weight:    common.GetPointer(weight),
			RPMLimit:  target.GetRPMLimit(),
		})
	}
	return targets, nil
}

func ValidateAggregateRouteRPMLimitConsistency(targets []model.AggregateGroupTarget, clientRoutePools model.AggregateGroupClientRoutePools) error {
	limits := make(map[string]int)
	check := func(realGroup string, rpmLimit int, source string) error {
		realGroup = strings.TrimSpace(realGroup)
		if realGroup == "" {
			return nil
		}
		if rpmLimit < 0 {
			return fmt.Errorf("%s 真实分组 %s RPM 上限不能小于 0", source, realGroup)
		}
		if existing, exists := limits[realGroup]; exists && existing != rpmLimit {
			return fmt.Errorf("真实分组 %s 在多个流量池中的 RPM 上限必须一致", realGroup)
		}
		limits[realGroup] = rpmLimit
		return nil
	}
	for _, target := range targets {
		if err := check(target.RealGroup, target.GetRPMLimit(), "默认流量池"); err != nil {
			return err
		}
	}
	if clientRoutePools.Enabled && clientRoutePools.ClaudeCodeCLI.Enabled {
		for _, target := range clientRoutePools.ClaudeCodeCLI.Targets {
			if err := check(target.RealGroup, target.GetRPMLimit(), "客户端专用流量池"); err != nil {
				return err
			}
		}
	}
	return nil
}

func IsAggregateTargetRPMLimited(aggregateGroup *model.AggregateGroup, target model.AggregateGroupTarget) bool {
	if aggregateGroup == nil || strings.TrimSpace(aggregateGroup.Name) == "" || strings.TrimSpace(target.RealGroup) == "" {
		return false
	}
	rpmLimit := target.GetRPMLimit()
	if rpmLimit <= 0 {
		return false
	}
	return !IsAggregateRouteRPMAllowed(aggregateGroup.Name, target.RealGroup, rpmLimit)
}

func GetAggregateRouteConfiguredRPMLimit(aggregateGroup *model.AggregateGroup, routeGroup string) int {
	if aggregateGroup == nil {
		return 0
	}
	routeGroup = strings.TrimSpace(routeGroup)
	if routeGroup == "" {
		return 0
	}
	for _, target := range aggregateGroup.Targets {
		if strings.TrimSpace(target.RealGroup) == routeGroup {
			return target.GetRPMLimit()
		}
	}
	clientRoutePools := aggregateGroup.GetClientRoutePools()
	if clientRoutePools.Enabled && clientRoutePools.ClaudeCodeCLI.Enabled {
		for _, target := range clientRoutePools.ClaudeCodeCLI.Targets {
			if strings.TrimSpace(target.RealGroup) == routeGroup {
				return target.GetRPMLimit()
			}
		}
	}
	return 0
}

func IsAggregateRouteGroupRPMLimited(aggregateGroup *model.AggregateGroup, routeGroup string) bool {
	if aggregateGroup == nil || strings.TrimSpace(aggregateGroup.Name) == "" {
		return false
	}
	rpmLimit := GetAggregateRouteConfiguredRPMLimit(aggregateGroup, routeGroup)
	if rpmLimit <= 0 {
		return false
	}
	return !IsAggregateRouteRPMAllowed(aggregateGroup.Name, routeGroup, rpmLimit)
}

func ValidateAggregateGroupConfig(group *model.AggregateGroup, visibleUserGroups []string, realGroups []string) error {
	if group == nil {
		return errors.New("聚合分组不能为空")
	}
	group.Name = strings.TrimSpace(group.Name)
	group.DisplayName = strings.TrimSpace(group.DisplayName)
	group.Description = strings.TrimSpace(group.Description)
	group.RetryStatusCodes = strings.TrimSpace(group.RetryStatusCodes)
	group.RoutingMode = strings.TrimSpace(group.RoutingMode)
	if group.Name == "" {
		return errors.New("聚合分组名称不能为空")
	}
	if group.DisplayName == "" {
		return errors.New("聚合分组显示名称不能为空")
	}
	if group.Name == "auto" {
		return errors.New("聚合分组名称不能为 auto")
	}
	if ratio_setting.ContainsGroupRatio(group.Name) {
		return fmt.Errorf("聚合分组名称 %s 与真实分组重名", group.Name)
	}
	if group.GroupRatio < 0 {
		return errors.New("聚合分组倍率不能小于 0")
	}
	if !model.IsValidAggregateGroupRoutingMode(group.RoutingMode) {
		return errors.New("聚合分组路由模式无效")
	}
	group.RoutingMode = model.NormalizeAggregateGroupRoutingMode(group.RoutingMode)
	if !model.IsValidAggregateGroupRouteAffinityStrategy(group.RouteAffinityStrategy) {
		return errors.New("聚合分组亲和策略无效")
	}
	group.RouteAffinityStrategy = model.NormalizeAggregateGroupRouteAffinityStrategy(group.RouteAffinityStrategy)
	if !model.IsValidAggregateGroupRouteAffinityScope(group.RouteAffinityScope) {
		return errors.New("聚合分组亲和范围无效")
	}
	group.RouteAffinityScope = model.NormalizeAggregateGroupRouteAffinityScope(group.RouteAffinityScope)
	if group.RecoveryEnabled && group.RecoveryIntervalSeconds <= 0 {
		return errors.New("恢复间隔必须大于 0")
	}
	group.ClusterAffinityTTLSeconds = model.NormalizeAggregateGroupClusterAffinityTTLSeconds(group.ClusterAffinityTTLSeconds)
	if group.RetryStatusCodes != "" {
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(group.RetryStatusCodes); err != nil {
			return fmt.Errorf("聚合分组重试状态码规则无效: %w", err)
		}
	}
	if len(visibleUserGroups) == 0 {
		return errors.New("至少选择一个可见用户组")
	}
	targets := NormalizeAggregateTargets(realGroups)
	if len(targets) == 0 {
		return errors.New("至少绑定一个真实分组")
	}
	for _, target := range targets {
		if target.RealGroup == "auto" {
			return errors.New("真实分组不能为 auto")
		}
		if !ratio_setting.ContainsGroupRatio(target.RealGroup) {
			return fmt.Errorf("真实分组 %s 不存在", target.RealGroup)
		}
		if IsAggregateGroup(target.RealGroup) {
			return fmt.Errorf("真实分组 %s 不能引用聚合分组", target.RealGroup)
		}
	}
	for _, visibleUserGroup := range visibleUserGroups {
		if strings.TrimSpace(visibleUserGroup) == "" {
			return errors.New("可见用户组不能为空")
		}
	}
	return nil
}

func BuildAggregateGroupVisibilityLabel(group *model.AggregateGroup) string {
	if group == nil {
		return ""
	}
	label := group.GetLabel()
	if label != "" {
		return label
	}
	return group.Name
}

var lookupAggregateRouteModelRatio = model.GetEnabledAggregateGroupRouteModelRatio

func ResolveContextGroupRatioInfo(ctx *gin.Context, userGroup string, logicalGroup string) types.GroupRatioInfo {
	return ResolveContextGroupRatioInfoForModel(ctx, userGroup, logicalGroup, "")
}

func ResolveContextGroupRatioInfoForModel(ctx *gin.Context, userGroup string, logicalGroup string, modelName string) types.GroupRatioInfo {
	groupRatioInfo := types.GroupRatioInfo{
		GroupRatio:           1.0,
		GroupSpecialRatio:    -1,
		OriginalGroupRatio:   1.0,
		RatioOverride:        -1,
		RouteModelGroupRatio: -1,
	}
	aggregateGroup := ""
	userSetting := dto.UserSetting{}
	if ctx != nil {
		aggregateGroup = common.GetContextKeyString(ctx, constant.ContextKeyAggregateGroup)
		if settingFromContext, ok := common.GetContextKeyType[dto.UserSetting](ctx, constant.ContextKeyUserSetting); ok {
			userSetting = settingFromContext
		}
	}
	if aggregateGroup != "" {
		if group, ok := GetAggregateGroup(aggregateGroup, true); ok {
			ratioView := getAggregateGroupRatioView(userSetting, group)
			groupRatioInfo.GroupRatio = ratioView.Ratio
			groupRatioInfo.OriginalGroupRatio = ratioView.OriginalRatio
			if ratioView.HasRatioOverride && ratioView.RatioOverride != nil {
				groupRatioInfo.GroupSpecialRatio = *ratioView.RatioOverride
				groupRatioInfo.HasSpecialRatio = true
				groupRatioInfo.RatioOverride = *ratioView.RatioOverride
				groupRatioInfo.HasRatioOverride = true
				groupRatioInfo.RatioOverrideApplied = true
			}

			realGroup := ""
			if ctx != nil {
				realGroup = common.GetContextKeyString(ctx, constant.ContextKeyRouteGroup)
			}
			if strings.TrimSpace(realGroup) != "" && strings.TrimSpace(modelName) != "" {
				rule, matched, err := lookupAggregateRouteModelRatio(group.Id, realGroup, modelName)
				if err != nil {
					message := fmt.Sprintf(
						"lookup aggregate route model ratio failed: aggregate_group=%s, route_group=%s, model=%s, err=%v",
						aggregateGroup,
						realGroup,
						modelName,
						err,
					)
					if ctx != nil {
						logger.LogError(ctx, message)
					} else {
						common.SysError(message)
					}
				} else if matched {
					groupRatioInfo.GroupRatio = rule.GroupRatio
					groupRatioInfo.GroupSpecialRatio = -1
					groupRatioInfo.HasSpecialRatio = false
					groupRatioInfo.RatioOverrideApplied = false
					groupRatioInfo.RouteModelGroupRatio = rule.GroupRatio
					groupRatioInfo.HasRouteModelGroupRatio = true
					groupRatioInfo.RouteModelRatioAggregateGroup = aggregateGroup
					groupRatioInfo.RouteModelRatioRealGroup = rule.RealGroup
					groupRatioInfo.RouteModelRatioModelName = rule.ModelName
				}
			}
			return groupRatioInfo
		}
	}
	if ctx != nil {
		if autoGroup := common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup); autoGroup != "" {
			logicalGroup = autoGroup
		}
	}
	if ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, logicalGroup); ok {
		groupRatioInfo.GroupSpecialRatio = ratio
		groupRatioInfo.GroupRatio = ratio
		groupRatioInfo.OriginalGroupRatio = ratio
		groupRatioInfo.HasSpecialRatio = true
		return groupRatioInfo
	}
	groupRatioInfo.GroupRatio = ratio_setting.GetGroupRatio(logicalGroup)
	groupRatioInfo.OriginalGroupRatio = groupRatioInfo.GroupRatio
	return groupRatioInfo
}

func GetUserUsableGroups(userGroup string) map[string]string {
	return GetUserUsableGroupsWithSetting(userGroup, dto.UserSetting{})
}

func GetUserUsableGroupsWithSetting(userGroup string, userSetting dto.UserSetting) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					groupsCopy[specialGroup] = desc
				}
			}
		}
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
		for _, aggregateGroup := range GetVisibleAggregateGroups(userGroup) {
			groupsCopy[aggregateGroup.Name] = BuildAggregateGroupVisibilityLabel(aggregateGroup)
		}
	}
	addExtraUsableGroups(groupsCopy, userSetting)
	return groupsCopy
}

func addExtraUsableGroups(groups map[string]string, userSetting dto.UserSetting) {
	if len(userSetting.ExtraUsableGroups) == 0 {
		return
	}
	for _, groupName := range userSetting.ExtraUsableGroups {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" || groupName == "auto" {
			continue
		}
		if _, exists := groups[groupName]; exists {
			continue
		}
		if ratio_setting.ContainsGroupRatio(groupName) {
			groups[groupName] = setting.GetUsableGroupDescription(groupName)
			continue
		}
		if aggregateGroup, ok := GetAggregateGroup(groupName, true); ok {
			groups[groupName] = BuildAggregateGroupVisibilityLabel(aggregateGroup)
		}
	}
}

func ShouldRetryStatusCodeByAggregateGroup(group string, code int) (bool, bool) {
	aggregateGroup, ok := GetAggregateGroup(group, true)
	if !ok || aggregateGroup == nil {
		return false, false
	}
	rules := strings.TrimSpace(aggregateGroup.RetryStatusCodes)
	if rules == "" {
		return false, false
	}
	ranges, err := operation_setting.ParseHTTPStatusCodeRanges(rules)
	if err != nil || len(ranges) == 0 {
		return false, false
	}
	for _, statusRange := range ranges {
		if code >= statusRange.Start && code <= statusRange.End {
			return true, true
		}
	}
	return false, true
}
