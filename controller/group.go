package controller

import (
	"net/http"

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
	userVisibleGroups := service.GetUserVisibleGroups(userGroup)
	for groupName, desc := range userVisibleGroups {
		ratioView := service.GetUserGroupRatioView(userGroup, groupName, userSetting)
		usableGroups[groupName] = map[string]interface{}{
			"ratio":              ratioView.Ratio,
			"original_ratio":     ratioView.OriginalRatio,
			"has_ratio_override": ratioView.HasRatioOverride,
			"desc":               desc,
			"type": func() string {
				if service.IsAggregateGroup(groupName) {
					return "aggregate"
				}
				return "real"
			}(),
		}
		if ratioView.RatioOverride != nil {
			usableGroups[groupName]["ratio_override"] = *ratioView.RatioOverride
		}
	}
	if _, ok := service.GetUserUsableGroups(userGroup)["auto"]; ok {
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
