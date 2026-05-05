package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func seedExternalRegisterInviteCode(t *testing.T, ownerID int, code string, status int) *model.InviteCode {
	t.Helper()
	inviteCode := &model.InviteCode{
		Code:              code,
		Prefix:            "EXT",
		OwnerUserId:       ownerID,
		TargetGroup:       "vip",
		RewardQuotaPerUse: 3000,
		RewardTotalUses:   5,
		Status:            status,
	}
	require.NoError(t, inviteCode.Insert())
	return inviteCode
}

func callExternalRegister(t *testing.T, authCode string, payload []byte) tokenAPIResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/external_register", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if authCode != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+authCode)
	}
	ExternalRegister(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestExternalRegisterRequiresEnabledAndAuthCode(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()
	payload := []byte(`{"username":"ext_guard_user","password":"password123","invite_code":"EXT-GUARD"}`)

	common.ExternalRegisterEnabled = false
	common.ExternalRegisterAuthKey = `["secret-code"]`
	resp := callExternalRegister(t, "secret-code", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "未启用")

	common.ExternalRegisterEnabled = true
	common.ExternalRegisterAuthKey = ""
	resp = callExternalRegister(t, "secret-code", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "未配置")

	common.ExternalRegisterAuthKey = "secret-code"
	resp = callExternalRegister(t, "", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "鉴权失败")

	resp = callExternalRegister(t, "wrong-code", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "鉴权失败")
}

func TestExternalRegisterCreatesUserWithInviteCodeWhenRegularRegisterDisabled(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()
	seedInviteCodeControllerUser(t, db, 1001, "ext_owner", "aff-ext-owner")
	inviteCode := seedExternalRegisterInviteCode(t, 1001, "EXT-CREATE", model.InviteCodeStatusEnabled)

	common.RegisterEnabled = false
	common.PasswordRegisterEnabled = false
	common.EmailVerificationEnabled = true
	common.ExternalRegisterEnabled = true
	common.ExternalRegisterAuthKey = `["external-secret"]`

	resp := callExternalRegister(t, "external-secret", []byte(`{
		"username":"ext_created_user",
		"password":"password123",
		"invite_code":"EXT-CREATE"
	}`))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"username":"ext_created_user"`)
	require.Contains(t, string(resp.Data), `"invite_code_owner_id":1001`)

	var created model.User
	require.NoError(t, db.Where("username = ?", "ext_created_user").First(&created).Error)
	require.Equal(t, "vip", created.Group)
	require.Equal(t, inviteCode.Id, created.InviteCodeId)
	require.Equal(t, 1001, created.InviteCodeOwnerId)
	require.Equal(t, 1001, created.InviterId)
	require.Equal(t, common.QuotaForNewUser+3000, created.Quota)
}

func TestExternalRegisterAllowsRegistrationWithoutInviteCode(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()

	common.RegisterEnabled = false
	common.PasswordRegisterEnabled = false
	common.EmailVerificationEnabled = true
	common.ExternalRegisterEnabled = true
	common.ExternalRegisterAuthKey = `["external-secret"]`

	resp := callExternalRegister(t, "external-secret", []byte(`{
		"username":"ext_plain_user",
		"password":"password123"
	}`))
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"username":"ext_plain_user"`)

	var created model.User
	require.NoError(t, db.Where("username = ?", "ext_plain_user").First(&created).Error)
	require.Equal(t, "default", created.Group)
	require.Zero(t, created.InviteCodeId)
	require.Zero(t, created.InviteCodeOwnerId)
	require.Zero(t, created.InviterId)
	require.Equal(t, common.QuotaForNewUser, created.Quota)
}

func TestExternalRegisterRejectsInvalidInputAndInviteCodes(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()
	seedInviteCodeControllerUser(t, db, 1002, "ext_invalid_owner", "aff-ext-invalid-owner")
	seedExternalRegisterInviteCode(t, 1002, "EXT-DISABLED", model.InviteCodeStatusDisabled)

	common.ExternalRegisterEnabled = true
	common.ExternalRegisterAuthKey = `["external-secret"]`

	resp := callExternalRegister(t, "external-secret", []byte(`{
		"username":"",
		"password":"password123",
		"invite_code":"EXT-DISABLED"
	}`))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "username and password")

	resp = callExternalRegister(t, "external-secret", []byte(`{
		"username":"ext_no_code",
		"password":"password123",
		"invite_code":"EXT-NOT-FOUND"
	}`))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "invite code")

	resp = callExternalRegister(t, "external-secret", []byte(`{
		"username":"ext_disabled_code",
		"password":"password123",
		"invite_code":"EXT-DISABLED"
	}`))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "disabled")
}

func TestExternalRegisterRejectsDuplicateUsername(t *testing.T) {
	db := setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()
	seedInviteCodeControllerUser(t, db, 1003, "ext_dup_owner", "aff-ext-dup-owner")
	seedInviteCodeControllerUser(t, db, 1004, "ext_duplicate_user", "aff-ext-duplicate")
	seedExternalRegisterInviteCode(t, 1003, "EXT-DUP", model.InviteCodeStatusEnabled)

	common.ExternalRegisterEnabled = true
	common.ExternalRegisterAuthKey = `["external-secret"]`

	resp := callExternalRegister(t, "external-secret", []byte(`{
		"username":"ext_duplicate_user",
		"password":"password123",
		"invite_code":"EXT-DUP"
	}`))
	require.False(t, resp.Success)
	require.NotEmpty(t, resp.Message)
}

func TestExternalRegisterAuthCodeManagementLifecycle(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	model.InitOptionMap()

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(listCtx)

	var listResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.True(t, listResp.Success, listResp.Message)
	require.Contains(t, string(listResp.Data), `"key":"ExternalRegisterEnabled"`)
	require.NotContains(t, string(listResp.Data), "ExternalRegisterAuthKey")

	updateSecretRecorder := httptest.NewRecorder()
	updateSecretCtx, _ := gin.CreateTestContext(updateSecretRecorder)
	updateSecretCtx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option",
		bytes.NewReader([]byte(`{"key":"ExternalRegisterAuthKey","value":"leaked-secret"}`)),
	)
	updateSecretCtx.Request.Header.Set("Content-Type", "application/json")
	UpdateOption(updateSecretCtx)

	var updateSecretResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(updateSecretRecorder.Body.Bytes(), &updateSecretResp))
	require.False(t, updateSecretResp.Success)
	require.Empty(t, common.ExternalRegisterAuthKey)

	firstRecorder := httptest.NewRecorder()
	firstCtx, _ := gin.CreateTestContext(firstRecorder)
	firstCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/external_register_auth_code", nil)
	GenerateExternalRegisterAuthCode(firstCtx)

	var firstResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(firstRecorder.Body.Bytes(), &firstResp))
	require.True(t, firstResp.Success, firstResp.Message)
	require.True(t, common.ExternalRegisterEnabled)
	firstKey := common.ExternalRegisterAuthKey
	firstKeys := parseExternalRegisterAuthKeys(firstKey)
	require.Len(t, firstKeys, 1)
	require.Len(t, firstKeys[0], 48)
	require.Contains(t, string(firstResp.Data), firstKeys[0])

	secondRecorder := httptest.NewRecorder()
	secondCtx, _ := gin.CreateTestContext(secondRecorder)
	secondCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/external_register_auth_code", nil)
	GenerateExternalRegisterAuthCode(secondCtx)

	var secondResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(secondRecorder.Body.Bytes(), &secondResp))
	require.True(t, secondResp.Success, secondResp.Message)
	secondKey := common.ExternalRegisterAuthKey
	secondKeys := parseExternalRegisterAuthKeys(secondKey)
	require.Len(t, secondKeys, 2)
	require.Len(t, secondKeys[1], 48)
	require.NotEqual(t, firstKey, secondKey)
	resp := callExternalRegister(t, firstKeys[0], []byte(`{
		"username":"ext_first_key_user",
		"password":"password123",
		"invite_code":"EXT-NOPE"
	}`))
	require.False(t, resp.Success)
	require.NotContains(t, resp.Message, "鉴权失败")
	resp = callExternalRegister(t, secondKeys[1], []byte(`{
		"username":"ext_second_key_user",
		"password":"password123",
		"invite_code":"EXT-NOPE"
	}`))
	require.False(t, resp.Success)
	require.NotContains(t, resp.Message, "鉴权失败")

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/option/external_register_auth_code", nil)
	GetExternalRegisterAuthCode(getCtx)

	var getResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getResp))
	require.True(t, getResp.Success, getResp.Message)
	require.Contains(t, string(getResp.Data), secondKeys[0])
	require.Contains(t, string(getResp.Data), secondKeys[1])

	deleteOneRecorder := httptest.NewRecorder()
	deleteOneCtx, _ := gin.CreateTestContext(deleteOneRecorder)
	deleteOneCtx.Request = httptest.NewRequest(
		http.MethodDelete,
		"/api/option/external_register_auth_code",
		bytes.NewReader([]byte(`{"auth_key":"`+secondKeys[0]+`"}`)),
	)
	deleteOneCtx.Request.Header.Set("Content-Type", "application/json")
	DeleteExternalRegisterAuthCode(deleteOneCtx)

	var deleteOneResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(deleteOneRecorder.Body.Bytes(), &deleteOneResp))
	require.True(t, deleteOneResp.Success, deleteOneResp.Message)
	require.True(t, common.ExternalRegisterEnabled)
	require.NotContains(t, common.ExternalRegisterAuthKey, secondKeys[0])
	require.Contains(t, common.ExternalRegisterAuthKey, secondKeys[1])
	resp = callExternalRegister(t, secondKeys[0], []byte(`{
		"username":"ext_deleted_one_user",
		"password":"password123"
	}`))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "鉴权失败")
	resp = callExternalRegister(t, secondKeys[1], []byte(`{
		"username":"ext_kept_key_user",
		"password":"password123",
		"invite_code":"EXT-NOPE"
	}`))
	require.False(t, resp.Success)
	require.NotContains(t, resp.Message, "鉴权失败")

	deleteRecorder := httptest.NewRecorder()
	deleteCtx, _ := gin.CreateTestContext(deleteRecorder)
	deleteCtx.Request = httptest.NewRequest(http.MethodDelete, "/api/option/external_register_auth_codes", nil)
	DeleteAllExternalRegisterAuthCodes(deleteCtx)

	var deleteResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(deleteRecorder.Body.Bytes(), &deleteResp))
	require.True(t, deleteResp.Success, deleteResp.Message)
	require.False(t, common.ExternalRegisterEnabled)
	require.Empty(t, common.ExternalRegisterAuthKey)
	resp = callExternalRegister(t, secondKeys[1], []byte(`{
		"username":"ext_deleted_key_user",
		"password":"password123"
	}`))
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "未启用")
}
