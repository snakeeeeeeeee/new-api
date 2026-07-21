package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetAdminAsyncTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tasks, dispatches, total, err := service.ListAdminAsyncTaskRecords(
		pageInfo.GetStartIdx(), pageInfo.GetPageSize(),
		model.AdminAsyncTaskQuery{
			SyncTaskQueryParams: model.SyncTaskQueryParams{
				Platform: constant.TaskPlatform(c.Query("platform")), ChannelID: c.Query("channel_id"),
				TaskID: c.Query("task_id"), UserID: c.Query("user_id"), Action: c.Query("action"),
				AssetType: c.Query("asset_type"), Status: c.Query("status"),
				StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
			},
			DispatchStatus: c.Query("dispatch_status"),
		},
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	taskDTOs := tasksToDto(tasks, true)
	items := make([]*dto.AdminAsyncTaskItem, 0, len(tasks))
	for index, task := range tasks {
		item := &dto.AdminAsyncTaskItem{Task: taskDTOs[index]}
		if dispatch := dispatches[task.ID]; dispatch != nil {
			item.Dispatch = &dto.AdminImageTaskDispatch{
				DispatchID: dispatch.DispatchID, Status: dispatch.Status, Attempts: dispatch.Attempts,
				NextAttemptAt: dispatch.NextAttemptAt, LockedUntil: dispatch.LockedUntil,
				LastHTTPStatus: dispatch.LastHTTPStatus, LastError: dispatch.LastError,
				DeliveredAt: dispatch.DeliveredAt, CreatedAt: dispatch.CreatedAt, UpdatedAt: dispatch.UpdatedAt,
			}
		}
		items = append(items, item)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAdminWebhookDeliveries(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID, _ := strconv.Atoi(c.Query("user_id"))
	httpStatus, _ := strconv.Atoi(c.Query("http_status"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	items, total, err := service.ListAdminWebhookDeliveries(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), model.AdminWebhookDeliveryQuery{
		Status: c.Query("status"), EventType: c.Query("event_type"), DeliveryID: c.Query("delivery_id"),
		UserID: userID, HTTPStatus: httpStatus, StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAdminWebhookDelivery(c *gin.Context) {
	deliveryID := strings.TrimSpace(c.Param("delivery_id"))
	detail, exists, err := service.GetAdminWebhookDeliveryDetail(deliveryID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		writeAsyncOperationsError(c, http.StatusNotFound, "Webhook delivery not found")
		return
	}
	common.ApiSuccess(c, detail)
}

func RetryAdminWebhookDelivery(c *gin.Context) {
	deliveryID := strings.TrimSpace(c.Param("delivery_id"))
	delivery, err := service.RetryAdminWebhookDelivery(deliveryID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			writeAsyncOperationsError(c, http.StatusNotFound, "Webhook delivery not found")
		case errors.Is(err, model.ErrWebhookDeliveryNotRetryable):
			writeAsyncOperationsError(c, http.StatusConflict, err.Error())
		default:
			common.ApiError(c, err)
		}
		return
	}
	model.RecordLog(delivery.UserID, model.LogTypeManage,
		"管理员重新投递 Webhook，管理员ID："+strconv.Itoa(c.GetInt("id"))+"，投递ID："+delivery.DeliveryID)
	common.ApiSuccess(c, delivery)
}

func writeAsyncOperationsError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}
