package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAssetControllerData(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Asset{}))
	model.DB.Exec("DELETE FROM assets")
	require.NoError(t, model.DB.Create(&model.User{Id: 11, Username: "asset_user"}).Error)
	require.NoError(t, model.DB.Create(&model.Asset{
		AssetID:    "asset_user_image",
		TaskID:     "task_asset_user",
		AssetIndex: 0,
		UserID:     11,
		ChannelID:  7,
		AssetType:  model.AssetTypeImage,
		Status:     model.AssetStatusAvailable,
		URL:        "https://cdn.example.com/image.webp",
		Model:      "gpt-image-2",
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}).Error)
	require.NoError(t, model.DB.Create(&model.Asset{
		AssetID:    "asset_other_image",
		TaskID:     "task_asset_other",
		AssetIndex: 0,
		UserID:     12,
		ChannelID:  7,
		AssetType:  model.AssetTypeImage,
		Status:     model.AssetStatusAvailable,
		URL:        "https://cdn.example.com/other.webp",
		Model:      "gpt-image-2",
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}).Error)
	require.NoError(t, model.DB.Create(&model.Asset{
		AssetID:    "asset_blocked",
		TaskID:     "task_asset_blocked",
		AssetIndex: 0,
		UserID:     11,
		AssetType:  model.AssetTypeVideo,
		Status:     model.AssetStatusBlocked,
		URL:        "https://cdn.example.com/blocked.mp4",
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}).Error)
}

func TestGetUserAssetsOnlyReturnsVisibleOwnAssets(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/assets/self?p=1&page_size=20", nil)
	ctx.Set("id", 11)

	GetUserAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "asset_user_image")
	assert.NotContains(t, body, "asset_other_image")
	assert.NotContains(t, body, "asset_blocked")
}

func TestGetUserAssetsCannotRequestBlockedAssets(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/assets/self?p=1&page_size=20&status=blocked", nil)
	ctx.Set("id", 11)

	GetUserAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "asset_blocked")
}

func TestGetAllAssetsCanFilterByType(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/assets/?p=1&page_size=20&asset_type=video", nil)

	GetAllAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "asset_blocked")
	assert.NotContains(t, body, "asset_user_image")
}

func TestExportUserAssetsCSV(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/assets/self/export", nil)
	ctx.Set("id", 11)

	ExportUserAssets(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "asset_id,task_id,asset_type,url,filename,model,platform,action,created_at")
	assert.Contains(t, recorder.Body.String(), "asset_user_image")
	assert.NotContains(t, recorder.Body.String(), "asset_blocked")
}

func TestCreateAndListAssetKeyReturnsFullKey(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 11, Username: "asset-key-user"}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/assets/keys", bytes.NewReader([]byte(`{"name":"asset-reader","expired_at":-1}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 11)

	CreateUserAssetKey(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var createResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &createResp))
	require.True(t, createResp.Success)
	require.Contains(t, string(createResp.Data), `"key":"ak_`)
	var firstKey dto.AssetKeyDto
	require.NoError(t, common.Unmarshal(createResp.Data, &firstKey))

	rotateRecorder := httptest.NewRecorder()
	rotateCtx, _ := gin.CreateTestContext(rotateRecorder)
	rotateCtx.Request = httptest.NewRequest(http.MethodPost, "/api/assets/keys", bytes.NewReader([]byte(`{"name":"resource-center","expired_at":-1}`)))
	rotateCtx.Request.Header.Set("Content-Type", "application/json")
	rotateCtx.Set("id", 11)
	CreateUserAssetKey(rotateCtx)

	require.Equal(t, http.StatusOK, rotateRecorder.Code)
	var rotateResp tokenAPIResponse
	require.NoError(t, common.Unmarshal(rotateRecorder.Body.Bytes(), &rotateResp))
	require.True(t, rotateResp.Success)
	var rotatedKey dto.AssetKeyDto
	require.NoError(t, common.Unmarshal(rotateResp.Data, &rotatedKey))
	assert.Equal(t, firstKey.ID, rotatedKey.ID)
	assert.NotEqual(t, firstKey.Key, rotatedKey.Key)

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/assets/keys?p=1&page_size=20", nil)
	listCtx.Set("id", 11)

	GetUserAssetKeys(listCtx)

	require.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Contains(t, listRecorder.Body.String(), "resource-center")
	assert.Contains(t, listRecorder.Body.String(), rotatedKey.Key)
	assert.NotContains(t, listRecorder.Body.String(), firstKey.Key)
	assert.Contains(t, listRecorder.Body.String(), `"total":1`)
}

func TestListAssetsByAPIKeyOnlyReturnsOwnVisibleAssets(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets?p=1&page_size=20", nil)
	ctx.Set("id", 11)

	ListAssetsByAPIKey(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"object":"list"`)
	assert.Contains(t, body, `"id":"asset_user_image"`)
	assert.NotContains(t, body, "asset_other_image")
	assert.NotContains(t, body, "asset_blocked")
}

func TestGetAssetByAPIKeyRejectsOtherUserAsset(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset_other_image", nil)
	ctx.Params = gin.Params{{Key: "asset_id", Value: "asset_other_image"}}
	ctx.Set("id", 11)

	GetAssetByAPIKey(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "asset not found")
}

func TestQueryAssetsByAPIKeySupportsBatchAssetIDs(t *testing.T) {
	setupInviteCodeControllerTestDB(t)
	seedAssetControllerData(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets/query", bytes.NewReader([]byte(`{"asset_ids":["asset_user_image","asset_other_image","asset_blocked"]}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 11)

	QueryAssetsByAPIKey(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"id":"asset_user_image"`)
	assert.NotContains(t, body, "asset_other_image")
	assert.NotContains(t, body, "asset_blocked")
}
