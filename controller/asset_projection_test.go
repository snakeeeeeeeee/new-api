package controller

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAssetProjectionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalMemoryCache := common.MemoryCacheEnabled
	originalServerAddress := system_setting.ServerAddress
	common.MemoryCacheEnabled = false
	system_setting.ServerAddress = "https://gateway.example"
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCache
		system_setting.ServerAddress = originalServerAddress
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestAssetProjectionPreservesAdobeDirectURLAndHidesInternalMetadata(t *testing.T) {
	db := setupAssetProjectionTestDB(t)
	baseURL := "http://adobe2api:6001"
	require.NoError(t, db.Create(&model.Channel{
		Id: 59, Type: constant.ChannelTypeAdobeVideo, BaseURL: &baseURL, Status: common.ChannelStatusEnabled,
	}).Error)
	directURL := "https://pre-signed-firefly-prod.s3.amazonaws.com/result.mp4?X-Amz-Signature=secret&X-Amz-Expires=3600"
	asset := &model.Asset{
		AssetID: "asset_adobe_direct", TaskID: "task_adobe_direct", AssetType: model.AssetTypeVideo,
		ChannelID: 59, URL: directURL,
		Metadata: model.AssetMetadata{"source": "task_info.video_outputs", "internal_secret": "hidden", "visible": "value"},
	}

	item := assetToDto(asset)
	require.NotNil(t, item)
	assert.Equal(t, directURL, item.URL)
	assert.Equal(t, service.VideoURLAuthNone, item.URLAuth)
	assert.True(t, item.Temporary)
	assert.Equal(t, map[string]any{"visible": "value"}, item.Metadata)

	urls := assetsToURLItems([]*model.Asset{asset})
	require.Len(t, urls, 1)
	assert.Equal(t, directURL, urls[0].URL)
	assert.Equal(t, service.VideoURLAuthNone, urls[0].URLAuth)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writeAssetCSV(c, []*model.Asset{asset})
	assert.Contains(t, recorder.Body.String(), directURL)
}

func TestAssetProjectionUsesContentEndpointForHistoricalAdobeReference(t *testing.T) {
	setupAssetProjectionTestDB(t)
	asset := &model.Asset{
		AssetID: "asset_adobe_legacy", TaskID: "task_NsDG9fOflSKh2e3DfvY7uckpdtJnpp3q",
		AssetType: model.AssetTypeVideo, ChannelID: 59, URL: "provider-task-internal",
		Metadata: model.AssetMetadata{
			"resolver": "adobe-video-content", "provider_reference": "provider-task-internal",
		},
	}

	item := assetToDto(asset)
	require.NotNil(t, item)
	assert.Equal(t, "https://gateway.example/v1/assets/asset_adobe_legacy/content", item.URL)
	assert.Equal(t, service.VideoURLAuthResourceAPIKey, item.URLAuth)
	assert.NotContains(t, item.Metadata, "provider_reference")
	assert.NotContains(t, item.Metadata, "resolver")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writeAssetCSV(c, []*model.Asset{asset})
	assert.Contains(t, recorder.Body.String(), "/v1/assets/asset_adobe_legacy/content")
	assert.NotContains(t, recorder.Body.String(), "provider-task-internal")
}
