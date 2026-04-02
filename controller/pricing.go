package controller

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

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
		cloned = append(cloned, copied)
	}
	return cloned
}

func GetPricing(c *gin.Context) {
	pricing := clonePricingItems(model.GetPricing())
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
		}
	}

	usableGroup = service.GetUserVisibleGroups(group)
	for groupName := range usableGroup {
		groupRatio[groupName] = service.GetUserGroupRatio(group, groupName)
	}
	for i := range pricing {
		pricing[i].EnableGroup = service.MapVisibleModelGroups(group, pricing[i].EnableGroup)
	}
	autoGroups := service.MapVisibleModelGroups(group, service.GetUserAutoGroup(group))

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        autoGroups,
		"_":                  "a42d372ccf0b5dd13ecf71203521f9d2",
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
