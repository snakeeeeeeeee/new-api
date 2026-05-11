package controller

import (
	"strconv"

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

func EnableSelfInviteeInvitation(c *gin.Context) {
	targetUserID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inviteCode, err := model.EnableInviteeInvitation(c.GetInt("id"), targetUserID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, inviteCode)
}

func GetSelfInviteAgentStats(c *gin.Context) {
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)
	stats, err := model.GetInviteAgentStats(c.GetInt("id"), c.Query("period"), startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
