package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userSetting, _ := model.GetUserSetting(userId, false)
	userGroup, _ = model.GetUserGroup(userId, false)
	userVisibleGroups := service.GetUserVisibleGroupsWithSetting(userGroup, userSetting)
	groupNames := make([]string, 0, len(userVisibleGroups))
	for groupName := range userVisibleGroups {
		groupNames = append(groupNames, groupName)
	}
	aggregateGroups, err := model.GetAggregateGroupsByNames(groupNames, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	aggregateGroupsByName := make(map[string]model.AggregateGroup, len(aggregateGroups))
	for _, aggregateGroup := range aggregateGroups {
		aggregateGroupsByName[aggregateGroup.Name] = aggregateGroup
	}
	categories, err := model.GetAggregateGroupCategoriesByID()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for groupName, desc := range userVisibleGroups {
		ratioView := service.GetUserGroupRatioView(userGroup, groupName, userSetting)
		aggregateGroup, isAggregate := aggregateGroupsByName[groupName]
		usableGroups[groupName] = map[string]interface{}{
			"ratio":              ratioView.Ratio,
			"original_ratio":     ratioView.OriginalRatio,
			"has_ratio_override": ratioView.HasRatioOverride,
			"desc":               desc,
			"type": func() string {
				if isAggregate {
					return "aggregate"
				}
				return "real"
			}(),
		}
		if isAggregate {
			categoryId := aggregateGroup.CategoryId
			categoryName := ""
			categoryOrder := 0
			if categoryId != model.AggregateGroupCategoryOtherID {
				category, ok := categories[categoryId]
				if !ok {
					categoryId = model.AggregateGroupCategoryOtherID
				} else {
					categoryName = category.Name
					categoryOrder = category.OrderIndex
				}
			}
			usableGroups[groupName]["category_id"] = categoryId
			usableGroups[groupName]["category_name"] = categoryName
			usableGroups[groupName]["category_order"] = categoryOrder
		}
		if ratioView.RatioOverride != nil {
			usableGroups[groupName]["ratio_override"] = *ratioView.RatioOverride
		}
	}
	if _, ok := service.GetUserUsableGroupsWithSetting(userGroup, userSetting)["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
			"type":  "auto",
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
