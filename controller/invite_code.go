package controller

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type InviteCodeUpsertRequest struct {
	Id                int    `json:"id"`
	Prefix            string `json:"prefix"`
	Count             int    `json:"count"`
	OwnerUserId       int    `json:"owner_user_id"`
	TargetGroup       string `json:"target_group"`
	RewardQuotaPerUse int    `json:"reward_quota_per_use"`
	RewardTotalUses   int    `json:"reward_total_uses"`
	Status            int    `json:"status"`
}

func validateInviteCodeRequest(req *InviteCodeUpsertRequest, isCreate bool) error {
	if req.OwnerUserId <= 0 {
		return errors.New("归属用户不能为空")
	}
	if _, err := model.GetUserById(req.OwnerUserId, false); err != nil {
		return errors.New("归属用户不存在")
	}
	req.TargetGroup = strings.TrimSpace(req.TargetGroup)
	if req.TargetGroup == "" {
		return errors.New("目标分组不能为空")
	}
	if _, ok := ratio_setting.GetGroupRatioCopy()[req.TargetGroup]; !ok {
		return errors.New("目标分组不存在")
	}
	if req.RewardQuotaPerUse < 0 {
		return errors.New("单次赠送额度不能小于 0")
	}
	if req.RewardTotalUses < 0 {
		return errors.New("赠送总次数不能小于 0")
	}
	if isCreate {
		req.Prefix = strings.TrimSpace(req.Prefix)
		if req.Prefix == "" {
			return errors.New("邀请码前缀不能为空")
		}
		if utf8.RuneCountInString(req.Prefix) > 24 {
			return errors.New("邀请码前缀过长")
		}
		if req.Count <= 0 {
			return errors.New("生成数量必须大于 0")
		}
		if req.Count > 100 {
			return errors.New("生成数量不能超过 100")
		}
	}
	if req.Status == 0 {
		req.Status = model.InviteCodeStatusEnabled
	}
	if req.Status != model.InviteCodeStatusEnabled && req.Status != model.InviteCodeStatusDisabled {
		return errors.New("邀请码状态无效")
	}
	return nil
}

func GetAllInviteCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	inviteCodes, total, err := model.GetAllInviteCodes(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.PopulateInviteCodeStats(inviteCodes); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(inviteCodes)
	common.ApiSuccess(c, pageInfo)
}

func SearchInviteCodes(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	inviteCodes, total, err := model.SearchInviteCodes(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.PopulateInviteCodeStats(inviteCodes); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(inviteCodes)
	common.ApiSuccess(c, pageInfo)
}

func GetInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inviteCode, err := model.GetInviteCodeByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.PopulateInviteCodeStats([]*model.InviteCode{inviteCode}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, inviteCode)
}

func AddInviteCode(c *gin.Context) {
	var req InviteCodeUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateInviteCodeRequest(&req, true); err != nil {
		common.ApiError(c, err)
		return
	}

	codes, err := model.CreateInviteCodes(
		req.Prefix,
		req.Count,
		req.OwnerUserId,
		req.TargetGroup,
		req.RewardQuotaPerUse,
		req.RewardTotalUses,
		req.Status,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, codes)
}

func UpdateInviteCode(c *gin.Context) {
	var req InviteCodeUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Id <= 0 {
		common.ApiError(c, errors.New("邀请码 ID 不能为空"))
		return
	}
	if err := validateInviteCodeRequest(&req, false); err != nil {
		common.ApiError(c, err)
		return
	}

	inviteCode, err := model.GetInviteCodeByID(req.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inviteCode.OwnerUserId = req.OwnerUserId
	inviteCode.TargetGroup = req.TargetGroup
	inviteCode.RewardQuotaPerUse = req.RewardQuotaPerUse
	if req.RewardTotalUses < inviteCode.RewardUsedUses {
		common.ApiError(c, errors.New("赠送总次数不能小于已使用次数"))
		return
	}
	inviteCode.RewardTotalUses = req.RewardTotalUses
	inviteCode.Status = req.Status
	if err := inviteCode.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.PopulateInviteCodeStats([]*model.InviteCode{inviteCode}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, inviteCode)
}

func DeleteInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteInviteCodeByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
