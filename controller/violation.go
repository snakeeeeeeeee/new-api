package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetViolationStatus(c *gin.Context) {
	setting := service.CurrentViolationSetting()
	keywords := operation_setting.ViolationKeywordsFromString(setting.Keywords)
	common.ApiSuccess(c, gin.H{
		"setting":       setting,
		"keyword_count": len(keywords),
		"actions": []string{
			operation_setting.ViolationActionLogOnly,
			operation_setting.ViolationActionBlock,
			operation_setting.ViolationActionBanAfterThreshold,
		},
	})
}

func UpdateViolationSetting(c *gin.Context) {
	setting := service.CurrentViolationSetting()
	if err := common.DecodeJson(c.Request.Body, &setting); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := service.UpdateViolationSetting(setting)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func GetViolationLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.ViolationLogQuery{
		Username:       c.Query("username"),
		TokenName:      c.Query("token_name"),
		ModelName:      c.Query("model_name"),
		UserGroup:      c.Query("user_group"),
		UsingGroup:     c.Query("using_group"),
		AggregateGroup: c.Query("aggregate_group"),
		RouteGroup:     c.Query("route_group"),
		RequestId:      c.Query("request_id"),
		MatchedWord:    c.Query("matched_word"),
		Action:         c.Query("action"),
	}
	query.UserId, _ = strconv.Atoi(c.Query("user_id"))
	query.TokenId, _ = strconv.Atoi(c.Query("token_id"))
	query.StartTimestamp, _ = strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	query.EndTimestamp, _ = strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if banned := c.Query("banned"); banned != "" {
		bannedValue := banned == "true" || banned == "1"
		query.Banned = &bannedValue
	}

	logs, total, err := model.GetViolationLogs(query, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func DeleteViolationLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if err := service.ValidateViolationLogDeleteTimestamp(targetTimestamp); err != nil {
		common.ApiError(c, err)
		return
	}
	count, err := model.DeleteViolationLogsBefore(targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, count)
}
