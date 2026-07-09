package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskPriceTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.AggregateGroup{}, &model.AggregateGroupTarget{}, &model.AggregateGroupRouteModelRatio{}))

	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"grok-imagine-video-test":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":5,"vip":3}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	})
}

func buildTaskPriceContext() *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)
	return ctx
}

func seedTaskPriceAggregateGroup(t *testing.T, name string, ratio float64) *model.AggregateGroup {
	t.Helper()
	group := &model.AggregateGroup{
		Name:                    name,
		DisplayName:             name,
		Status:                  model.AggregateGroupStatusEnabled,
		GroupRatio:              ratio,
		RecoveryEnabled:         true,
		RecoveryIntervalSeconds: 300,
		CreatedTime:             time.Now().Unix(),
		UpdatedTime:             time.Now().Unix(),
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, model.DB.Create(group).Error)
	require.NoError(t, model.DB.Create(&model.AggregateGroupTarget{
		AggregateGroupId: group.Id,
		RealGroup:        "default",
		OrderIndex:       0,
	}).Error)
	return group
}

func TestModelPriceHelperPerCallUsesNormalGroupRatio(t *testing.T) {
	setupTaskPriceTest(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2}`))

	ctx := buildTaskPriceContext()
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-test",
		UserGroup:       "default",
		UsingGroup:      "default",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXai},
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, 1.0, priceData.ModelPrice)
	assert.Equal(t, 2.0, priceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, int(2*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperPerCallUsesAggregateGroupRatioAndOverride(t *testing.T) {
	setupTaskPriceTest(t)
	seedTaskPriceAggregateGroup(t, "enterprise-stable", 2)

	ctx := buildTaskPriceContext()
	common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-test",
		UserGroup:       "vip",
		UsingGroup:      "enterprise-stable",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXai},
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, 2.0, priceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 2.0, priceData.GroupRatioInfo.OriginalGroupRatio)
	assert.False(t, priceData.GroupRatioInfo.HasRatioOverride)
	assert.Equal(t, int(2*common.QuotaPerUnit), priceData.Quota)

	overrideCtx := buildTaskPriceContext()
	common.SetContextKey(overrideCtx, constant.ContextKeyAggregateGroup, "enterprise-stable")
	common.SetContextKey(overrideCtx, constant.ContextKeyUserSetting, dto.UserSetting{
		AggregateGroupRatioOverrides: map[string]float64{
			"enterprise-stable": 0.5,
		},
	})
	overrideInfo := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-test",
		UserGroup:       "vip",
		UsingGroup:      "enterprise-stable",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXai},
	}

	overridePriceData, err := ModelPriceHelperPerCall(overrideCtx, overrideInfo)

	require.NoError(t, err)
	assert.Equal(t, 0.5, overridePriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 2.0, overridePriceData.GroupRatioInfo.OriginalGroupRatio)
	assert.True(t, overridePriceData.GroupRatioInfo.HasRatioOverride)
	assert.Equal(t, 0.5, overridePriceData.GroupRatioInfo.RatioOverride)
	assert.Equal(t, int(0.5*common.QuotaPerUnit), overridePriceData.Quota)
}

func TestModelPriceHelpersUseAggregateRouteModelRatio(t *testing.T) {
	setupTaskPriceTest(t)
	group := seedTaskPriceAggregateGroup(t, "enterprise-premium-route", 2)
	require.NoError(t, model.DB.Create(&model.AggregateGroupRouteModelRatio{
		AggregateGroupId: group.Id,
		RealGroup:        "default",
		ModelName:        "grok-imagine-video-test",
		GroupRatio:       4,
		Enabled:          true,
	}).Error)

	buildInfoAndContext := func() (*relaycommon.RelayInfo, *gin.Context) {
		ctx := buildTaskPriceContext()
		common.SetContextKey(ctx, constant.ContextKeyAggregateGroup, group.Name)
		common.SetContextKey(ctx, constant.ContextKeyRouteGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{
			AggregateGroupRatioOverrides: map[string]float64{group.Name: 0.5},
		})
		return &relaycommon.RelayInfo{
			OriginModelName: "grok-imagine-video-test",
			UserGroup:       "vip",
			UsingGroup:      group.Name,
			ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeXai},
		}, ctx
	}

	perCallInfo, perCallCtx := buildInfoAndContext()
	perCallPrice, err := ModelPriceHelperPerCall(perCallCtx, perCallInfo)
	require.NoError(t, err)
	assert.Equal(t, 4.0, perCallPrice.GroupRatioInfo.GroupRatio)
	assert.True(t, perCallPrice.GroupRatioInfo.HasRouteModelGroupRatio)
	assert.False(t, perCallPrice.GroupRatioInfo.RatioOverrideApplied)
	assert.Equal(t, int(4*common.QuotaPerUnit), perCallPrice.Quota)

	tokenInfo, tokenCtx := buildInfoAndContext()
	tokenPrice, err := ModelPriceHelper(tokenCtx, tokenInfo, 100, &types.TokenCountMeta{})
	require.NoError(t, err)
	assert.Equal(t, 4.0, tokenPrice.GroupRatioInfo.GroupRatio)
	assert.True(t, tokenPrice.GroupRatioInfo.HasRouteModelGroupRatio)
	assert.Equal(t, int(4*common.QuotaPerUnit), tokenPrice.QuotaToPreConsume)
}
