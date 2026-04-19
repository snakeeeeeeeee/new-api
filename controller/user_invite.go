package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetSelfInviteCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	inviteCodes, total, err := model.GetInviteCodesByOwnerUserID(
		c.GetInt("id"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(inviteCodes)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfInvitees(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	invitees, total, err := model.GetInviteeSummariesByOwnerUserID(
		c.GetInt("id"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invitees)
	common.ApiSuccess(c, pageInfo)
}
