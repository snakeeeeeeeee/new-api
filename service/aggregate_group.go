package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	if strings.TrimSpace(userGroup) == "" {
		return []*model.AggregateGroup{}
	}
	groups, err := model.GetAllAggregateGroups(true)
	if err != nil {
		return []*model.AggregateGroup{}
	}
	visibleGroups := make([]*model.AggregateGroup, 0, len(groups))
	for _, group := range groups {
		if canUserGroupSeeAggregate(userGroup, group) {
			visibleGroups = append(visibleGroups, group)
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

func GetUserVisibleGroups(userGroup string) map[string]string {
	return GetUserUsableGroups(userGroup)
}

func CanUserSelectGroup(userGroup, group string) bool {
	_, ok := GetUserVisibleGroups(userGroup)[group]
	return ok
}

func GetUserGroupRatio(userGroup, group string) float64 {
	if ratio, ok := GetAggregateGroupRatio(group); ok {
		return ratio
	}
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}

func GetModelsForGroup(group string) []string {
	if aggregateGroup, ok := GetAggregateGroup(group, true); ok {
		modelSet := make(map[string]struct{})
		models := make([]string, 0)
		for _, targetGroup := range aggregateGroup.GetTargetGroups() {
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

func MapVisibleModelGroups(userGroup string, realGroups []string) []string {
	visibleGroups := GetUserVisibleGroups(userGroup)
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
	for _, aggregateGroup := range GetVisibleAggregateGroups(userGroup) {
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
		})
	}
	return targets
}

func ValidateAggregateGroupConfig(group *model.AggregateGroup, visibleUserGroups []string, realGroups []string) error {
	if group == nil {
		return errors.New("聚合分组不能为空")
	}
	group.Name = strings.TrimSpace(group.Name)
	group.DisplayName = strings.TrimSpace(group.DisplayName)
	group.Description = strings.TrimSpace(group.Description)
	group.RetryStatusCodes = strings.TrimSpace(group.RetryStatusCodes)
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
	if group.RecoveryEnabled && group.RecoveryIntervalSeconds <= 0 {
		return errors.New("恢复间隔必须大于 0")
	}
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

func ResolveContextGroupRatioInfo(ctx *gin.Context, userGroup string, logicalGroup string) types.GroupRatioInfo {
	groupRatioInfo := types.GroupRatioInfo{
		GroupRatio:        1.0,
		GroupSpecialRatio: -1,
	}
	aggregateGroup := ""
	if ctx != nil {
		aggregateGroup = common.GetContextKeyString(ctx, constant.ContextKeyAggregateGroup)
	}
	if aggregateGroup != "" {
		if ratio, ok := GetAggregateGroupRatio(aggregateGroup); ok {
			groupRatioInfo.GroupRatio = ratio
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
		groupRatioInfo.HasSpecialRatio = true
		return groupRatioInfo
	}
	groupRatioInfo.GroupRatio = ratio_setting.GetGroupRatio(logicalGroup)
	return groupRatioInfo
}

func GetUserUsableGroups(userGroup string) map[string]string {
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
	return groupsCopy
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
