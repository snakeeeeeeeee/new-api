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
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/image_handle_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// UpdateTaskBulk 薄入口，实际轮询逻辑在 service 层
func UpdateTaskBulk() {
	service.TaskPollingLoop()
}

func GetAllTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 解析其他查询参数
	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		AssetType:      c.Query("asset_type"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
	}

	items := model.TaskGetAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllTasks(queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, true))
	common.ApiSuccess(c, pageInfo)
}

func GetUserTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	userId := c.GetInt("id")

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		AssetType:      c.Query("asset_type"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}

	items := model.TaskGetAllUserTask(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllUserTask(userId, queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, false))
	common.ApiSuccess(c, pageInfo)
}

func GetAsyncTaskStats(c *gin.Context) {
	common.ApiSuccess(c, service.GetAsyncTaskStats())
}

type ImageHandleConfigRequest struct {
	BaseURL                 string  `json:"base_url"`
	APIKey                  string  `json:"api_key"`
	InternalBaseURL         string  `json:"internal_base_url"`
	InternalSecretID        string  `json:"internal_secret_id"`
	InternalSecret          string  `json:"internal_secret"`
	CallbackSecret          string  `json:"callback_secret"`
	DebugUpstream           bool    `json:"debug_upstream"`
	SyncImageEnabled        bool    `json:"sync_image_enabled"`
	SyncImageResultPolicy   string  `json:"sync_image_result_policy"`
	SyncImageDefaultFormat  string  `json:"sync_image_default_format"`
	UsagePrechargeEnabled   *bool   `json:"usage_precharge_enabled"`
	PrechargeAmountPerImage float64 `json:"precharge_amount_per_image"`
	PrechargeQuotaPerImage  int     `json:"precharge_quota_per_image"`
}

func imageHandleConfigResponse(setting image_handle_setting.ImageHandleSetting) gin.H {
	setting = image_handle_setting.NormalizeSetting(setting)
	return gin.H{
		"base_url":                   setting.BaseURL,
		"api_key":                    setting.APIKey,
		"internal_base_url":          setting.InternalBaseURL,
		"internal_secret_id":         setting.InternalSecretID,
		"internal_secret":            setting.InternalSecret,
		"callback_secret":            setting.CallbackSecret,
		"debug_upstream":             setting.DebugUpstream,
		"sync_image_enabled":         setting.SyncImageEnabled,
		"sync_image_result_policy":   setting.SyncImageResultPolicy,
		"sync_image_default_format":  setting.SyncImageDefaultFormat,
		"usage_precharge_enabled":    setting.UsagePrechargeEnabled,
		"precharge_amount_per_image": setting.PrechargeAmountPerImage,
		"precharge_quota_per_image":  setting.PrechargeQuotaPerImage,
		"configured":                 image_handle_setting.Validate(setting) == nil,
	}
}

func GetImageHandleConfig(c *gin.Context) {
	common.ApiSuccess(c, imageHandleConfigResponse(*image_handle_setting.GetImageHandleSetting()))
}

func UpdateImageHandleConfig(c *gin.Context) {
	var req ImageHandleConfigRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	usagePrechargeEnabled := true
	if req.UsagePrechargeEnabled != nil {
		usagePrechargeEnabled = *req.UsagePrechargeEnabled
	}
	next := image_handle_setting.NormalizeSetting(image_handle_setting.ImageHandleSetting{
		BaseURL:                 req.BaseURL,
		APIKey:                  req.APIKey,
		InternalBaseURL:         req.InternalBaseURL,
		InternalSecretID:        req.InternalSecretID,
		InternalSecret:          req.InternalSecret,
		CallbackSecret:          req.CallbackSecret,
		DebugUpstream:           req.DebugUpstream,
		SyncImageEnabled:        req.SyncImageEnabled,
		SyncImageResultPolicy:   req.SyncImageResultPolicy,
		SyncImageDefaultFormat:  req.SyncImageDefaultFormat,
		UsagePrechargeEnabled:   usagePrechargeEnabled,
		PrechargeAmountPerImage: req.PrechargeAmountPerImage,
		PrechargeQuotaPerImage:  req.PrechargeQuotaPerImage,
	})
	if err := image_handle_setting.Validate(next); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	for key, value := range map[string]string{
		"image_handle_setting.base_url":                   next.BaseURL,
		"image_handle_setting.api_key":                    next.APIKey,
		"image_handle_setting.internal_base_url":          next.InternalBaseURL,
		"image_handle_setting.internal_secret_id":         next.InternalSecretID,
		"image_handle_setting.internal_secret":            next.InternalSecret,
		"image_handle_setting.callback_secret":            strings.TrimSpace(next.CallbackSecret),
		"image_handle_setting.debug_upstream":             strconv.FormatBool(next.DebugUpstream),
		"image_handle_setting.sync_image_enabled":         strconv.FormatBool(next.SyncImageEnabled),
		"image_handle_setting.sync_image_result_policy":   next.SyncImageResultPolicy,
		"image_handle_setting.sync_image_default_format":  next.SyncImageDefaultFormat,
		"image_handle_setting.usage_precharge_enabled":    strconv.FormatBool(next.UsagePrechargeEnabled),
		"image_handle_setting.precharge_amount_per_image": strconv.FormatFloat(next.PrechargeAmountPerImage, 'f', -1, 64),
		"image_handle_setting.precharge_quota_per_image":  strconv.Itoa(next.PrechargeQuotaPerImage),
	} {
		if err := model.UpdateOption(key, value); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeManage, "管理员更新 image-handle 异步图片执行器配置")
	common.ApiSuccess(c, imageHandleConfigResponse(next))
}

type UpdateTaskBlockRequest struct {
	IsBlocked bool `json:"is_blocked"`
}

func UpdateTaskBlockStatus(c *gin.Context) {
	taskId := c.Param("task_id")
	if taskId == "" {
		common.ApiError(c, errors.New("task_id is required"))
		return
	}

	req := UpdateTaskBlockRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	task, exists, err := model.UpdateTaskBlocked(taskId, req.IsBlocked)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiError(c, errors.New("task_not_exist"))
		return
	}

	action := "解除屏蔽"
	if req.IsBlocked {
		action = "屏蔽"
	}
	model.RecordLog(task.UserId, model.LogTypeManage, "管理员"+action+"任务记录，管理员ID："+strconv.Itoa(c.GetInt("id"))+"，任务ID："+task.TaskID)
	common.ApiSuccess(c, relay.TaskModel2Dto(task))
}

func tasksToDto(tasks []*model.Task, fillUser bool) []*dto.TaskDto {
	var userIdMap map[int]*model.UserBase
	if fillUser {
		userIdMap = make(map[int]*model.UserBase)
		userIds := types.NewSet[int]()
		for _, task := range tasks {
			userIds.Add(task.UserId)
		}
		for _, userId := range userIds.Items() {
			cacheUser, err := model.GetUserCache(userId)
			if err == nil {
				userIdMap[userId] = cacheUser
			}
		}
	}
	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIdMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		item := relay.TaskModel2Dto(task)
		if !fillUser {
			item.ChannelId = 0
		}
		result[i] = item
	}
	return result
}
