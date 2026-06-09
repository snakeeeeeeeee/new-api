package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func GroupInUserUsableGroupsWithSetting(userGroup, groupName string, userSetting dto.UserSetting) bool {
	_, ok := GetUserUsableGroupsWithSetting(userGroup, userSetting)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetUserAutoGroupWithSetting(userGroup, dto.UserSetting{})
}

func GetUserAutoGroupWithSetting(userGroup string, userSetting dto.UserSetting) []string {
	groups := GetUserUsableGroupsWithSetting(userGroup, userSetting)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}
