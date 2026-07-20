package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetErrorSnapshotStatus(c *gin.Context) {
	status, err := service.GetErrorSnapshotStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func UpdateErrorSnapshotSettings(c *gin.Context) {
	var update service.ErrorSnapshotSettingsUpdate
	if err := common.DecodeJson(c.Request.Body, &update); err != nil {
		common.ApiError(c, err)
		return
	}
	status, err := service.UpdateErrorSnapshotSettings(update)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetErrorSnapshots(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	userID, _ := strconv.Atoi(c.Query("user_id"))
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	items, total, err := service.ListErrorSnapshots(model.ErrorSnapshotQuery{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		RequestID:      c.Query("request_id"),
		UserID:         userID,
		Username:       c.Query("username"),
		ChannelID:      channelID,
		ErrorKeyword:   c.Query("error_keyword"),
		StartIndex:     pageInfo.GetStartIdx(),
		Limit:          pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetErrorSnapshotSelectOptions(c *gin.Context) {
	keyword := c.Query("keyword")
	switch strings.ToLower(strings.TrimSpace(c.Query("type"))) {
	case "user":
		options, err := model.SearchErrorSnapshotUserOptions(keyword, 20)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, options)
	case "channel":
		options, err := model.SearchErrorSnapshotChannelOptions(keyword, 20)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, options)
	default:
		common.ApiErrorMsg(c, "invalid error snapshot option type")
	}
}

func GetErrorSnapshot(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	snapshot, err := model.GetErrorSnapshot(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := service.ReadErrorSnapshot(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"snapshot": snapshot, "payload": payload})
}

func DownloadErrorSnapshot(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	data, err := service.ReadCompressedErrorSnapshot(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+".json.gz"))
	c.Data(http.StatusOK, "application/gzip", data)
}

func DeleteErrorSnapshot(c *gin.Context) {
	if err := service.DeleteErrorSnapshot(c.Param("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func CleanupErrorSnapshots(c *gin.Context) {
	if err := service.CleanupErrorSnapshots(); err != nil {
		common.ApiError(c, err)
		return
	}
	status, err := service.GetErrorSnapshotStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func DeleteAllErrorSnapshots(c *gin.Context) {
	if err := service.DeleteAllErrorSnapshots(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
