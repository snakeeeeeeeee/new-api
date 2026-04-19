package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInviteCodeControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalQuotaForInviter := common.QuotaForInviter
	originalQuotaForInvitee := common.QuotaForInvitee

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.InviteCode{}, &model.TopUp{}, &model.Token{}, &model.Log{}))

	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		common.QuotaForInviter = originalQuotaForInviter
		common.QuotaForInvitee = originalQuotaForInvitee
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedInviteCodeControllerUser(t *testing.T, db *gorm.DB, id int, username string, affCode string) {
	t.Helper()
	user := &model.User{
		Id:       id,
		Username: username,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  affCode,
	}
	require.NoError(t, db.Create(user).Error)
}

func TestAddInviteCodeAndList(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 10, "owner_invite", "aff-owner")

	payload := []byte(`{
		"prefix":"ZY-",
		"count":2,
		"owner_user_id":10,
		"target_group":"vip",
		"reward_quota_per_use":2000,
		"reward_total_uses":3,
		"status":1
	}`)

	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = httptest.NewRequest(http.MethodPost, "/api/invite_code", bytes.NewReader(payload))
	createCtx.Request.Header.Set("Content-Type", "application/json")
	AddInviteCode(createCtx)

	var createResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(createRecorder.Body.Bytes(), &createResp))
	require.True(t, createResp.Success, createResp.Message)
	require.Contains(t, string(createResp.Data), "ZY-")

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/invite_code?p=1&page_size=10", nil)
	GetAllInviteCodes(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"target_group":"vip"`)
	require.Contains(t, string(listResp.Data), `"owner_username":"owner_invite"`)
}

func TestRegisterWithManagedInviteCode(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 20, "invite_owner", "aff-owner-20")
	inviteCode := &model.InviteCode{
		Code:              "ZY-ABC123",
		Prefix:            "ZY-",
		OwnerUserId:       20,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 3000,
		RewardTotalUses:   2,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, inviteCode.Insert())

	payload := []byte(`{
		"username":"new_invited_user",
		"password":"password123",
		"invite_code":"ZY-ABC123"
	}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var created model.User
	require.NoError(t, db.Where("username = ?", "new_invited_user").First(&created).Error)
	require.Equal(t, "vip", created.Group)
	require.Equal(t, inviteCode.Id, created.InviteCodeId)
	require.Equal(t, 20, created.InviteCodeOwnerId)
	require.Equal(t, 20, created.InviterId)
	require.Equal(t, common.QuotaForNewUser+3000, created.Quota)

	updatedInviteCode, err := model.GetInviteCodeByID(inviteCode.Id)
	require.NoError(t, err)
	require.Equal(t, 1, updatedInviteCode.RewardUsedUses)
}

func TestRegisterWithoutInviteCodeKeepsLegacyAffCodeLogic(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 30, "legacy_owner", "AFF1")

	payload := []byte(`{
		"username":"legacy_aff_user",
		"password":"password123",
		"aff_code":"AFF1"
	}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, resp.Message)

	var created model.User
	require.NoError(t, db.Where("username = ?", "legacy_aff_user").First(&created).Error)
	require.Equal(t, 30, created.InviterId)
	require.Zero(t, created.InviteCodeId)
	require.Zero(t, created.InviteCodeOwnerId)
}

func TestUpdateInviteCodeRejectsTotalUsesBelowUsedUses(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	seedInviteCodeControllerUser(t, db, 40, "owner_update", "aff-owner-update")

	inviteCode := &model.InviteCode{
		Code:              "UP-USED001",
		Prefix:            "UP",
		OwnerUserId:       40,
		TargetGroup:       "default",
		RewardQuotaPerUse: 1000,
		RewardTotalUses:   5,
		RewardUsedUses:    2,
		Status:            model.InviteCodeStatusEnabled,
	}
	require.NoError(t, inviteCode.Insert())

	payload := []byte(`{
		"id": ` + fmt.Sprintf("%d", inviteCode.Id) + `,
		"owner_user_id": 40,
		"target_group": "default",
		"reward_quota_per_use": 1000,
		"reward_total_uses": 1,
		"status": 1
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/invite_code", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateInviteCode(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "赠送总次数不能小于已使用次数")
}
