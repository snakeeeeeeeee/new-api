package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Key    string `json:"key"`
	Status int    `json:"status"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

type tokenCreateResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AggregateGroup{}, &model.AggregateGroupTarget{}); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedTokenTestUser(t *testing.T, db *gorm.DB, userID int, group string) {
	t.Helper()
	seedTokenTestUserWithSetting(t, db, userID, group, dto.UserSetting{})
}

func seedTokenTestUserWithSetting(t *testing.T, db *gorm.DB, userID int, group string, setting dto.UserSetting) {
	t.Helper()
	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("user_%d", userID),
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    group,
	}
	user.SetSetting(setting)
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestAddTokenAllowsVisibleAggregateGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenTestUser(t, db, 1, "vip")
	originalGroups := setting.UserUsableGroups2JSONString()
	defer func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalGroups)
	}()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`); err != nil {
		t.Fatalf("failed to update usable groups: %v", err)
	}
	aggregateGroup := &model.AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "企业稳定组",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	if err := aggregateGroup.SetVisibleUserGroups([]string{"vip"}); err != nil {
		t.Fatalf("failed to set visible groups: %v", err)
	}
	if err := aggregateGroup.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
	}); err != nil {
		t.Fatalf("failed to create aggregate group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token", map[string]any{
		"name":                 "aggregate-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "enterprise-stable",
		"cross_group_retry":    false,
	}, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var createResponse tokenCreateResponse
	if err := common.Unmarshal(response.Data, &createResponse); err != nil {
		t.Fatalf("failed to decode create token response: %v", err)
	}
	if createResponse.ID == 0 || createResponse.Name != "aggregate-token" || !strings.HasPrefix(createResponse.Key, "sk-") {
		t.Fatalf("unexpected create token response: %+v", createResponse)
	}

	var created model.Token
	if err := db.First(&created, "name = ?", "aggregate-token").Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	if created.Group != "enterprise-stable" {
		t.Fatalf("expected aggregate group, got %s", created.Group)
	}
}

func TestAddTokenAllowsRealGroupWhenAggregateVisible(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenTestUser(t, db, 1, "vip")
	originalGroups := setting.UserUsableGroups2JSONString()
	defer func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalGroups)
	}()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`); err != nil {
		t.Fatalf("failed to update usable groups: %v", err)
	}
	aggregateGroup := &model.AggregateGroup{
		Name:                    "enterprise-stable",
		DisplayName:             "企业稳定组",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	if err := aggregateGroup.SetVisibleUserGroups([]string{"vip"}); err != nil {
		t.Fatalf("failed to set visible groups: %v", err)
	}
	if err := aggregateGroup.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0, Weight: common.GetPointer(model.AggregateGroupTargetDefaultWeight)},
	}); err != nil {
		t.Fatalf("failed to create aggregate group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token", map[string]any{
		"name":                 "real-group-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
}

func TestAddTokenAllowsExtraUsableRealGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenTestUserWithSetting(t, db, 1, "vip", dto.UserSetting{
		ExtraUsableGroups: []string{"svip"},
	})
	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	defer func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalGroups)
		_ = ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios)
	}()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`); err != nil {
		t.Fatalf("failed to update usable groups: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`); err != nil {
		t.Fatalf("failed to update group ratios: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token", map[string]any{
		"name":                 "extra-real-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "svip",
		"cross_group_retry":    false,
	}, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var created model.Token
	if err := db.First(&created, "name = ?", "extra-real-token").Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	if created.Group != "svip" {
		t.Fatalf("expected svip group, got %s", created.Group)
	}
}

func TestAddTokenAllowsExtraUsableAggregateGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	seedTokenTestUserWithSetting(t, db, 1, "vip", dto.UserSetting{
		ExtraUsableGroups: []string{"extra-aggregate"},
	})
	originalGroups := setting.UserUsableGroups2JSONString()
	defer func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalGroups)
	}()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`); err != nil {
		t.Fatalf("failed to update usable groups: %v", err)
	}
	aggregateGroup := &model.AggregateGroup{
		Name:                    "extra-aggregate",
		DisplayName:             "额外聚合组",
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              1.2,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
	}
	if err := aggregateGroup.SetVisibleUserGroups([]string{"svip"}); err != nil {
		t.Fatalf("failed to set visible groups: %v", err)
	}
	if err := aggregateGroup.InsertWithTargets(service.NormalizeAggregateTargets([]string{"default"})); err != nil {
		t.Fatalf("failed to create aggregate group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token", map[string]any{
		"name":                 "extra-aggregate-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "extra-aggregate",
		"cross_group_retry":    false,
	}, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
}

func TestAddTokenRejectsUnassignedExtraGroup(t *testing.T) {
	setupTokenControllerTestDB(t)
	seedTokenTestUser(t, model.DB, 1, "vip")
	originalGroups := setting.UserUsableGroups2JSONString()
	defer func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalGroups)
	}()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`); err != nil {
		t.Fatalf("failed to update usable groups: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token", map[string]any{
		"name":                 "rejected-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "svip",
		"cross_group_retry":    false,
	}, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected failure response")
	}
	if !strings.Contains(response.Message, "无权选择 svip 分组") {
		t.Fatalf("unexpected failure message: %s", response.Message)
	}
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}
