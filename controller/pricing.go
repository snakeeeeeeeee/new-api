package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func clonePricingItems(pricing []model.Pricing) []model.Pricing {
	cloned := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		copied := item
		if len(item.EnableGroup) > 0 {
			copied.EnableGroup = append([]string{}, item.EnableGroup...)
		}
		if len(item.SupportedEndpointTypes) > 0 {
			copied.SupportedEndpointTypes = append([]constant.EndpointType{}, item.SupportedEndpointTypes...)
		}
		copied.TokenTierPricing = nil
		if item.QuotaType == 0 {
			if tierPricing, ok := ratio_setting.GetEffectiveTokenTierPricingRule(item.ModelName); ok {
				tierPricing.Rule.Tiers = append([]types.TokenTier(nil), tierPricing.Rule.Tiers...)
				copied.TokenTierPricing = &tierPricing
			}
		}
		cloned = append(cloned, copied)
	}
	return cloned
}

func GetPricing(c *gin.Context) {
	pricing := clonePricingItems(model.GetPricing())
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	groupRatioDetails := map[string]service.GroupRatioView{}
	modelGroupRatioDetails := map[string]map[string]service.ModelGroupRatioView{}
	var group string
	userSetting := dto.UserSetting{}
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			userSetting = user.GetSetting()
		}
	}

	usableGroup = service.GetUserVisibleGroupsWithSetting(group, userSetting)
	for groupName := range usableGroup {
		ratioView := service.GetUserGroupRatioView(group, groupName, userSetting)
		groupRatio[groupName] = ratioView.Ratio
		groupRatioDetails[groupName] = ratioView
	}
	visibleAggregateGroups := service.GetVisibleAggregateGroupsWithSetting(group, userSetting)
	aggregateGroupsByName := make(map[string]*model.AggregateGroup, len(visibleAggregateGroups))
	aggregateGroupIds := make([]int, 0, len(visibleAggregateGroups))
	for _, aggregateGroup := range visibleAggregateGroups {
		aggregateGroupsByName[aggregateGroup.Name] = aggregateGroup
		aggregateGroupIds = append(aggregateGroupIds, aggregateGroup.Id)
	}
	rulesByAggregateGroupId, ruleErr := model.GetAggregateGroupRouteModelRatiosByGroupIDs(aggregateGroupIds)
	if ruleErr != nil {
		common.SysError("failed to load aggregate route model ratios for pricing: " + ruleErr.Error())
		rulesByAggregateGroupId = map[int][]model.AggregateGroupRouteModelRatio{}
	}
	for i := range pricing {
		modelEnabledRealGroups := append([]string{}, pricing[i].EnableGroup...)
		pricing[i].EnableGroup = service.MapVisibleModelGroupsWithSetting(group, pricing[i].EnableGroup, userSetting)
		for _, groupName := range pricing[i].EnableGroup {
			aggregateGroup, isAggregate := aggregateGroupsByName[groupName]
			if !isAggregate {
				continue
			}
			detail, reachable := service.GetAggregateModelGroupRatioView(
				userSetting,
				aggregateGroup,
				pricing[i].ModelName,
				rulesByAggregateGroupId[aggregateGroup.Id],
				modelEnabledRealGroups,
			)
			if !reachable {
				continue
			}
			if modelGroupRatioDetails[pricing[i].ModelName] == nil {
				modelGroupRatioDetails[pricing[i].ModelName] = make(map[string]service.ModelGroupRatioView)
			}
			modelGroupRatioDetails[pricing[i].ModelName][groupName] = detail
		}
	}
	autoGroups := service.MapVisibleModelGroupsWithSetting(group, service.GetUserAutoGroupWithSetting(group, userSetting), userSetting)

	c.JSON(200, gin.H{
		"success":                   true,
		"data":                      pricing,
		"vendors":                   model.GetVendors(),
		"group_ratio":               groupRatio,
		"group_ratio_details":       groupRatioDetails,
		"model_group_ratio_details": modelGroupRatioDetails,
		"usable_group":              usableGroup,
		"supported_endpoint":        model.GetSupportedEndpointMap(),
		"auto_groups":               autoGroups,
		"_":                         "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
