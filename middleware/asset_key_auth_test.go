package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAssetKeyAuthTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.AssetKey{}))
	require.NoError(t, db.Create(&model.User{
		Id:       41,
		Username: "asset_key_user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    1000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 51, UserId: 41, Key: "normalassetkey", Name: "resource-api",
		Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 321,
		ModelLimitsEnabled: true, ModelLimits: "gpt-image-2", Group: "default",
	}).Error)

	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func runAssetKeyAuthRequest(keyValue string) (*httptest.ResponseRecorder, *gin.Context, bool) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets", nil)
	if keyValue != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+keyValue)
	}
	called := false
	handler := AssetKeyAuth()
	handler(ctx)
	if !ctx.IsAborted() {
		called = true
	}
	return recorder, ctx, called
}

func TestAssetKeyAuthAcceptsEnabledKey(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	key, err := model.CreateAssetKey(41, "read-assets", -1, "")
	require.NoError(t, err)

	_, ctx, called := runAssetKeyAuthRequest(key.Key)

	require.True(t, called)
	require.Equal(t, 41, ctx.GetInt("id"))
	require.Equal(t, key.ID, ctx.GetInt64("asset_key_id"))
	require.Equal(t, key.Name, ctx.GetString("token_name"))
	require.True(t, ctx.GetBool("token_unlimited_quota"))
	require.False(t, ctx.GetBool("token_model_limit_enabled"))
	require.Zero(t, ctx.GetInt("token_id"))
}

func TestAssetKeyAuthRejectsDisabledKey(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	key, err := model.CreateAssetKey(41, "read-assets", -1, "")
	require.NoError(t, err)
	_, err = model.UpdateUserAssetKeyStatus(key.ID, 41, model.AssetKeyStatusDisabled)
	require.NoError(t, err)

	recorder, _, called := runAssetKeyAuthRequest(key.Key)

	require.False(t, called)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAssetKeyAuthRejectsExpiredKey(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	key, err := model.CreateAssetKey(41, "read-assets", time.Now().Unix()-60, "")
	require.NoError(t, err)

	recorder, _, called := runAssetKeyAuthRequest(key.Key)

	require.False(t, called)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAssetKeyAuthRejectsNormalTokenFormat(t *testing.T) {
	setupAssetKeyAuthTestDB(t)

	recorder, _, called := runAssetKeyAuthRequest("sk-normal-token")

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAssetKeyAuthRejectsUnknownResourceKey(t *testing.T) {
	setupAssetKeyAuthTestDB(t)

	recorder, _, called := runAssetKeyAuthRequest("ak_unknown")

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "无效的资源 API Key")
}

func TestAssetKeyAuthReturnsServerErrorForDatabaseFailure(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	recorder, _, called := runAssetKeyAuthRequest("ak_database_failure")

	require.False(t, called)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeQueryDataError))
	require.NotContains(t, recorder.Body.String(), "无效的资源 API Key")
}

func TestAssetOrTokenAuthAcceptsNormalToken(t *testing.T) {
	setupAssetKeyAuthTestDB(t)

	_, ctx, called := runAssetOrTokenAuthRequest("sk-normalassetkey")

	require.True(t, called)
	require.Equal(t, 41, ctx.GetInt("id"))
	require.Equal(t, 51, ctx.GetInt("token_id"))
}

func TestTokenAuthPreservesTokenBillingAndRoutingContext(t *testing.T) {
	setupAssetKeyAuthTestDB(t)

	_, ctx, called := runTokenAuthRequest("sk-normalassetkey")

	require.True(t, called)
	require.Equal(t, 41, ctx.GetInt("id"))
	require.Equal(t, 51, ctx.GetInt("token_id"))
	require.False(t, ctx.GetBool("token_unlimited_quota"))
	require.Equal(t, 321, ctx.GetInt("token_quota"))
	require.True(t, ctx.GetBool("token_model_limit_enabled"))
	require.Equal(t, map[string]bool{"gpt-image-2": true}, ctx.MustGet("token_model_limit"))
	require.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))
	require.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
}

func TestTokenAuthRejectsResourceKey(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	key, err := model.CreateAssetKey(41, "resource-reader", -1, "")
	require.NoError(t, err)

	recorder, _, called := runTokenAuthRequest(key.Key)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAssetOrTokenAuthKeepsLegacyAssetKeyCompatible(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	key, err := model.CreateAssetKey(41, "legacy-resource-reader", -1, "")
	require.NoError(t, err)

	_, ctx, called := runAssetOrTokenAuthRequest(key.Key)

	require.True(t, called)
	require.Equal(t, 41, ctx.GetInt("id"))
	require.Equal(t, key.ID, ctx.GetInt64("asset_key_id"))
}

func runAssetOrTokenAuthRequest(keyValue string) (*httptest.ResponseRecorder, *gin.Context, bool) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets", nil)
	if keyValue != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+keyValue)
	}
	handler := AssetOrTokenAuth()
	handler(ctx)
	return recorder, ctx, !ctx.IsAborted()
}

func runTokenAuthRequest(keyValue string) (*httptest.ResponseRecorder, *gin.Context, bool) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	if keyValue != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+keyValue)
	}
	TokenAuth()(ctx)
	return recorder, ctx, !ctx.IsAborted()
}

func TestRequireAssetKeyScopeKeepsLegacyKeysReadOnly(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	legacy, err := model.CreateAssetKey(41, "legacy", -1, "")
	require.NoError(t, err)
	_, ctx, called := runAssetKeyAuthRequest(legacy.Key)
	require.True(t, called)

	RequireAssetKeyScope(model.AssetKeyScopeRead)(ctx)
	require.False(t, ctx.IsAborted())
	RequireAssetKeyScope("webhooks:read")(ctx)
	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, ctx.Writer.Status())
}

func TestAssetKeyRejectsRemovedWebhookScopes(t *testing.T) {
	setupAssetKeyAuthTestDB(t)
	_, err := model.CreateAssetKeyWithScopes(41, "webhooks", -1, "", []string{"webhooks:write"})
	require.ErrorContains(t, err, "invalid asset key scope")
}
