package service

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	canvasTestRedirect = "http://localhost:3000/auth/supertoken/callback"
	canvasTestVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

func setupCanvasAuthorizationTest(t *testing.T) {
	t.Helper()
	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalUsingSQLite, originalUsingMySQL, originalUsingPostgreSQL := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	originalOptions := canvasTestOptionSnapshot()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	model.DB, model.LOG_DB = db, db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Channel{}, &model.Option{},
		&model.AssetKey{}, &model.CanvasGrant{}, &model.CanvasAuthorizationCode{},
		&model.Ability{}, &model.Model{}, &model.Vendor{},
		&model.AggregateGroup{}, &model.AggregateGroupTarget{},
	))
	cleanCanvasAuthorizationTables(t)

	originalGroups := setting.UserUsableGroups2JSONString()
	originalMaxTokens := operation_setting.GetMaxUserTokens()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP 分组"}`))
	operation_setting.GetTokenSetting().MaxUserTokens = 1000

	imageChannel := model.Channel{Id: 9101, Type: constant.ChannelTypeOpenAI, Key: "test", Status: common.ChannelStatusEnabled, Name: "canvas-image", Models: "canvas-image", Group: "default,vip"}
	videoChannel := model.Channel{Id: 9102, Type: constant.ChannelTypeAdobeVideo, Key: "test", Status: common.ChannelStatusEnabled, Name: "canvas-video", Models: "canvas-video", Group: "vip,restricted"}
	require.NoError(t, model.DB.Create(&imageChannel).Error)
	require.NoError(t, model.DB.Create(&videoChannel).Error)
	require.NoError(t, model.DB.Create(&model.Model{ModelName: "canvas-image", Endpoints: `{"image-generation":{}}`, Status: 1}).Error)
	require.NoError(t, model.DB.Create([]model.Ability{
		{Group: "default", Model: "canvas-image", ChannelId: imageChannel.Id, Enabled: true},
		{Group: "vip", Model: "canvas-image", ChannelId: imageChannel.Id, Enabled: true},
		{Group: "vip", Model: "canvas-video", ChannelId: videoChannel.Id, Enabled: true},
		{Group: "restricted", Model: "canvas-video", ChannelId: videoChannel.Id, Enabled: true},
	}).Error)
	model.RefreshPricing()
	setCanvasTestConfig("default", "vip")

	t.Cleanup(func() {
		cleanCanvasAuthorizationTables(t)
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		operation_setting.GetTokenSetting().MaxUserTokens = originalMaxTokens
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = originalUsingSQLite, originalUsingMySQL, originalUsingPostgreSQL
		restoreCanvasTestOptions(originalOptions)
		require.NoError(t, sqlDB.Close())
		model.InvalidatePricing()
	})
}

type canvasTestOptionValue struct {
	value  string
	exists bool
}

func canvasTestOptionSnapshot() map[string]canvasTestOptionValue {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	result := make(map[string]canvasTestOptionValue, 4)
	for _, key := range []string{canvasOptionEnabled, canvasOptionImageGroup, canvasOptionVideoGroup, canvasOptionRedirectUris} {
		value, exists := common.OptionMap[key]
		result[key] = canvasTestOptionValue{value: value, exists: exists}
	}
	return result
}

func restoreCanvasTestOptions(values map[string]canvasTestOptionValue) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	for key, value := range values {
		if value.exists {
			common.OptionMap[key] = value.value
		} else {
			delete(common.OptionMap, key)
		}
	}
}

func cleanCanvasAuthorizationTables(t *testing.T) {
	t.Helper()
	for _, table := range []string{"canvas_authorization_codes", "canvas_grants", "asset_keys", "tokens", "users", "abilities", "channels", "models", "vendors", "aggregate_group_targets", "aggregate_groups", "options"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	setCanvasTestOptions(map[string]string{
		canvasOptionEnabled:      "false",
		canvasOptionImageGroup:   "",
		canvasOptionVideoGroup:   "",
		canvasOptionRedirectUris: "[]",
	})
}

func setCanvasTestConfig(imageGroup, videoGroup string) {
	setCanvasTestOptions(map[string]string{
		canvasOptionEnabled:      "true",
		canvasOptionImageGroup:   imageGroup,
		canvasOptionVideoGroup:   videoGroup,
		canvasOptionRedirectUris: `["` + canvasTestRedirect + `"]`,
	})
}

func setCanvasTestOptions(values map[string]string) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for key, value := range values {
		common.OptionMap[key] = value
	}
}

func seedCanvasUser(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: id, Username: "canvas-user", Password: "password123", Status: common.UserStatusEnabled, Group: "default", AffCode: "canvas-user"}).Error)
}

func canvasAuthorizationRequest() CanvasAuthorizationRequest {
	sum := sha256.Sum256([]byte(canvasTestVerifier))
	return CanvasAuthorizationRequest{
		ClientId: CanvasClientId, RedirectUri: canvasTestRedirect, State: "canvas-state",
		CodeChallenge: base64.RawURLEncoding.EncodeToString(sum[:]), CodeChallengeMethod: "S256",
	}
}

func issueCanvasCode(t *testing.T, userId int) CanvasAuthorizationCodeResult {
	t.Helper()
	result, err := IssueCanvasAuthorizationCode(userId, canvasAuthorizationRequest())
	require.NoError(t, err)
	return result
}

func exchangeCanvasCode(code string) (CanvasTokenExchangeResult, error) {
	return ExchangeCanvasAuthorizationCode(CanvasTokenExchangeRequest{
		GrantType: "authorization_code", ClientId: CanvasClientId, Code: code,
		CodeVerifier: canvasTestVerifier, RedirectUri: canvasTestRedirect,
	})
}

func requireCanvasErrorCode(t *testing.T, err error, code string) *CanvasAuthorizationError {
	t.Helper()
	var typed *CanvasAuthorizationError
	require.ErrorAs(t, err, &typed)
	require.Equal(t, code, typed.Code)
	return typed
}

func TestNormalizeCanvasRedirectUris(t *testing.T) {
	valid, err := normalizeCanvasRedirectUris([]string{"https://canvas.example.com/auth/supertoken/callback", canvasTestRedirect, canvasTestRedirect})
	require.NoError(t, err)
	require.Equal(t, []string{canvasTestRedirect, "https://canvas.example.com/auth/supertoken/callback"}, valid)

	for _, value := range []string{
		"http://canvas.example.com/auth/supertoken/callback",
		"https://canvas.example.com/auth/supertoken/callback?next=1",
		"https://canvas.example.com/auth/supertoken/callback#done",
		"https://*.example.com/auth/supertoken/callback",
		"https://canvas.example.com/auth/*",
	} {
		_, err := normalizeCanvasRedirectUris([]string{value})
		require.Error(t, err, value)
	}
}

func TestCanvasConfigDisabledMayRemainIncomplete(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	result, err := SaveCanvasConfig(CanvasConfig{Enabled: false, ImageGroup: "missing"})
	require.NoError(t, err)
	require.False(t, result.Config.Enabled)

	_, err = SaveCanvasConfig(CanvasConfig{Enabled: true})
	require.ErrorContains(t, err, "必须配置")
}

func TestCanvasImageGroupRecognizesCurrentModelsWithoutEndpointMetadata(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	aggregateGroup := model.AggregateGroup{
		Name: "image-generation", DisplayName: "通用生图分组", Status: model.AggregateGroupStatusEnabled, GroupRatio: 1,
	}
	require.NoError(t, aggregateGroup.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "image-gpt", OrderIndex: 0},
		{RealGroup: "image-gemini", OrderIndex: 1},
	}))
	require.NoError(t, model.DB.Create([]model.Ability{
		{Group: "image-gpt", Model: "gpt-image-2", ChannelId: 9101, Enabled: true},
		{Group: "image-gpt", Model: "adobe-gpt-image-2-count", ChannelId: 9101, Enabled: true},
		{Group: "image-gemini", Model: "gemini-3.1-flash-image", ChannelId: 9101, Enabled: true},
		{Group: "image-gemini", Model: "gemini-3-pro-image-count", ChannelId: 9101, Enabled: true},
	}).Error)
	model.RefreshPricing()

	adminConfig, err := GetCanvasAdminConfig()
	require.NoError(t, err)
	var option *CanvasGroupOption
	for index := range adminConfig.Groups {
		if adminConfig.Groups[index].Name == "image-generation" {
			option = &adminConfig.Groups[index]
			break
		}
	}
	require.NotNil(t, option)
	require.ElementsMatch(t, []string{
		"adobe-gpt-image-2-count",
		"gemini-3-pro-image-count",
		"gemini-3.1-flash-image",
		"gpt-image-2",
	}, option.ImageModels)
}

func TestCanvasAuthorizationFirstRepeatAndCredentialRepair(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9201)

	first, err := exchangeCanvasCode(issueCanvasCode(t, 9201).Code)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(first.ImageApiKey, "sk-"))
	require.True(t, strings.HasPrefix(first.VideoApiKey, "sk-"))
	require.True(t, strings.HasPrefix(first.ResourceApiKey, model.AssetKeyPrefix))
	require.Equal(t, []string{"canvas-image"}, first.ImageModels)
	require.Equal(t, []string{"canvas-video"}, first.VideoModels)

	second, err := exchangeCanvasCode(issueCanvasCode(t, 9201).Code)
	require.NoError(t, err)
	require.Equal(t, first.ImageApiKey, second.ImageApiKey)
	require.Equal(t, first.VideoApiKey, second.VideoApiKey)
	require.Equal(t, first.ResourceApiKey, second.ResourceApiKey)

	var grant model.CanvasGrant
	require.NoError(t, model.DB.Where("user_id = ?", 9201).First(&grant).Error)
	require.NotEqual(t, grant.ImageTokenId, grant.VideoTokenId)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", grant.ImageTokenId).Updates(map[string]any{"status": common.TokenStatusDisabled, "name": "changed", "group": "wrong"}).Error)
	require.NoError(t, model.DB.Delete(&model.Token{}, grant.VideoTokenId).Error)
	require.NoError(t, model.DB.Model(&model.AssetKey{}).Where("user_id = ?", 9201).Update("status", model.AssetKeyStatusDisabled).Error)

	repaired, err := exchangeCanvasCode(issueCanvasCode(t, 9201).Code)
	require.NoError(t, err)
	require.Equal(t, first.ImageApiKey, repaired.ImageApiKey)
	require.NotEqual(t, first.VideoApiKey, repaired.VideoApiKey)
	require.NotEqual(t, first.ResourceApiKey, repaired.ResourceApiKey)
	require.NoError(t, model.DB.Where("user_id = ?", 9201).First(&grant).Error)
	require.NotEqual(t, grant.ImageTokenId, grant.VideoTokenId)

	var imageToken model.Token
	require.NoError(t, model.DB.First(&imageToken, grant.ImageTokenId).Error)
	require.Equal(t, common.TokenStatusEnabled, imageToken.Status)
	require.Equal(t, "canvas-images", imageToken.Name)
	require.Equal(t, "default", imageToken.Group)
	require.Equal(t, "canvas-image", imageToken.ModelLimits)
}

func TestCanvasAuthorizationRepairsSharedGrantToken(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9202)
	_, err := exchangeCanvasCode(issueCanvasCode(t, 9202).Code)
	require.NoError(t, err)

	var grant model.CanvasGrant
	require.NoError(t, model.DB.Where("user_id = ?", 9202).First(&grant).Error)
	require.NoError(t, model.DB.Model(&grant).Update("video_token_id", grant.ImageTokenId).Error)
	_, err = exchangeCanvasCode(issueCanvasCode(t, 9202).Code)
	require.NoError(t, err)
	require.NoError(t, model.DB.Where("user_id = ?", 9202).First(&grant).Error)
	require.NotEqual(t, grant.ImageTokenId, grant.VideoTokenId)
}

func TestCanvasAuthorizationRejectsInvalidGrantWithoutPartialCredentials(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9203)

	code := issueCanvasCode(t, 9203)
	wrong := CanvasTokenExchangeRequest{GrantType: "authorization_code", ClientId: CanvasClientId, Code: code.Code, CodeVerifier: strings.Repeat("x", 43), RedirectUri: canvasTestRedirect}
	_, err := ExchangeCanvasAuthorizationCode(wrong)
	requireCanvasErrorCode(t, err, "invalid_grant")
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", 9203).Count(&count).Error)
	require.Zero(t, count)

	_, err = exchangeCanvasCode(code.Code)
	require.NoError(t, err)
	_, err = exchangeCanvasCode(code.Code)
	requireCanvasErrorCode(t, err, "invalid_grant")
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", 9203).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestCanvasAuthorizationRejectsExpiredOrChangedConfiguration(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9204)

	expired := issueCanvasCode(t, 9204)
	require.NoError(t, model.DB.Model(&model.CanvasAuthorizationCode{}).Where("code_hash = ?", canvasCodeHash(expired.Code)).Update("expired_at", time.Now().Unix()-1).Error)
	_, err := exchangeCanvasCode(expired.Code)
	requireCanvasErrorCode(t, err, "invalid_grant")

	changed := issueCanvasCode(t, 9204)
	setCanvasTestConfig("vip", "vip")
	_, err = exchangeCanvasCode(changed.Code)
	requireCanvasErrorCode(t, err, "invalid_canvas_config")

	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", 9204).Count(&count).Error)
	require.Zero(t, count)
}

func TestCanvasAuthorizationMissingGroupAndTokenLimitCreateNothing(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9205)
	setCanvasTestConfig("default", "restricted")

	_, err := IssueCanvasAuthorizationCode(9205, canvasAuthorizationRequest())
	typed := requireCanvasErrorCode(t, err, "group_forbidden")
	require.Equal(t, []string{"restricted"}, typed.MissingGroups)

	setCanvasTestConfig("default", "vip")
	operation_setting.GetTokenSetting().MaxUserTokens = 1
	code := issueCanvasCode(t, 9205)
	_, err = exchangeCanvasCode(code.Code)
	requireCanvasErrorCode(t, err, "token_limit_reached")

	for _, target := range []any{&model.Token{}, &model.CanvasGrant{}, &model.AssetKey{}} {
		var count int64
		require.NoError(t, model.DB.Model(target).Count(&count).Error)
		require.Zero(t, count)
	}
	var authorizationCode model.CanvasAuthorizationCode
	require.NoError(t, model.DB.Where("code_hash = ?", canvasCodeHash(code.Code)).First(&authorizationCode).Error)
	require.Zero(t, authorizationCode.ConsumedAt)
}

func TestCanvasAuthorizationConcurrentReplayConsumesOnce(t *testing.T) {
	setupCanvasAuthorizationTest(t)
	seedCanvasUser(t, 9206)
	code := issueCanvasCode(t, 9206)

	errorsByAttempt := make([]error, 2)
	var wait sync.WaitGroup
	for index := range errorsByAttempt {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errorsByAttempt[index] = exchangeCanvasCode(code.Code)
		}(index)
	}
	wait.Wait()

	successes := 0
	for _, err := range errorsByAttempt {
		if err == nil {
			successes++
			continue
		}
		var typed *CanvasAuthorizationError
		if errors.As(err, &typed) {
			require.Equal(t, "invalid_grant", typed.Code)
		} else {
			require.ErrorContains(t, err, "locked")
		}
	}
	require.Equal(t, 1, successes)
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", 9206).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
