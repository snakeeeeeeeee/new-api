package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const inviteCommissionMaxReportRangeSeconds = int64(90 * 24 * 60 * 60)

type inviteCommissionUserConfigRequest struct {
	Enabled       *bool  `json:"enabled"`
	Level1RateBps *int   `json:"level1_rate_bps"`
	Level2RateBps *int   `json:"level2_rate_bps"`
	Remark        string `json:"remark"`
}

func GetInviteCommissionSettings(c *gin.Context) {
	common.ApiSuccess(c, model.GetInviteCommissionSettings())
}

func UpdateInviteCommissionSettings(c *gin.Context) {
	var settings model.InviteCommissionSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	updated, err := model.UpdateInviteCommissionSettings(settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func GetInviteCommissionUserConfigs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	configs, total, err := model.ListInviteCommissionUserConfigs(
		c.Query("keyword"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(configs)
	common.ApiSuccess(c, pageInfo)
}

func UpdateInviteCommissionUserConfig(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req inviteCommissionUserConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	settings := model.GetInviteCommissionSettings()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	level1RateBps := settings.DefaultLevel1RateBps
	if req.Level1RateBps != nil {
		level1RateBps = *req.Level1RateBps
	}
	level2RateBps := settings.DefaultLevel2RateBps
	if req.Level2RateBps != nil {
		level2RateBps = *req.Level2RateBps
	}
	config, err := model.UpsertInviteCommissionUserConfig(&model.InviteCommissionUserConfig{
		UserID:        userID,
		Enabled:       enabled,
		Level1RateBps: level1RateBps,
		Level2RateBps: level2RateBps,
		Remark:        req.Remark,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func GetInviteCommissionGroupProfitRules(c *gin.Context) {
	rows, err := model.ListInviteCommissionGroupProfitRuleRows(c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func UpdateInviteCommissionGroupProfitRule(c *gin.Context) {
	var req model.InviteCommissionGroupProfitRule
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	rule, err := model.UpsertInviteCommissionGroupProfitRule(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rule)
}

func DeleteInviteCommissionGroupProfitRule(c *gin.Context) {
	group := c.Query("group")
	if err := model.DeleteInviteCommissionGroupProfitRule(group); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetInviteCommissionAdminReport(c *gin.Context) {
	ownerUserID, err := strconv.Atoi(c.Query("owner_user_id"))
	if err != nil || ownerUserID <= 0 {
		common.ApiErrorMsg(c, "请选择有效的邀请人")
		return
	}
	start, end, ok := parseInviteCommissionReportRange(c)
	if !ok {
		return
	}
	report, err := model.BuildInviteCommissionReport(model.InviteCommissionReportRequest{
		OwnerUserID:    ownerUserID,
		StartTimestamp: start,
		EndTimestamp:   end,
		IncludeDetails: true,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func GetInviteCommissionSelfReport(c *gin.Context) {
	userID := c.GetInt("id")
	start, end, ok := parseInviteCommissionReportRange(c)
	if !ok {
		return
	}
	report, err := model.BuildInviteCommissionReport(model.InviteCommissionReportRequest{
		OwnerUserID:    userID,
		StartTimestamp: start,
		EndTimestamp:   end,
		IncludeDetails: false,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.StripInviteCommissionProfitDetailsForUser(report)
	common.ApiSuccess(c, report)
}

func parseInviteCommissionReportRange(c *gin.Context) (int64, int64, bool) {
	now := time.Now().Unix()
	end := parseInt64Query(c, "end_timestamp", now)
	start := parseInt64Query(c, "start_timestamp", end-7*24*60*60)
	if end <= 0 {
		end = now
	}
	if start <= 0 {
		start = end - 7*24*60*60
	}
	if end < start {
		common.ApiErrorMsg(c, "结束时间不能早于开始时间")
		return 0, 0, false
	}
	if end-start > inviteCommissionMaxReportRangeSeconds {
		common.ApiErrorMsg(c, "查询范围不能超过90天")
		return 0, 0, false
	}
	return start, end, true
}

func parseInt64Query(c *gin.Context, key string, fallback int64) int64 {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
