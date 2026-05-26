package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDistributorTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.AggregateGroup{}, &model.AggregateGroupTarget{}, &model.Channel{}, &model.Ability{}))

	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"vip":"VIP分组"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
}

func TestDistributePlaygroundSelectedAggregateGroupSetsAggregateContext(t *testing.T) {
	setupDistributorTestDB(t)

	weight := uint(10)
	priority := int64(0)
	channel := &model.Channel{
		Id:       1001,
		Name:     "default-channel",
		Key:      "sk-test",
		Status:   common.ChannelStatusEnabled,
		Group:    "default",
		Models:   "gpt-test",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	group := &model.AggregateGroup{
		Name:            "agg-playground",
		DisplayName:     "agg-playground",
		Status:          model.AggregateGroupStatusEnabled,
		GroupRatio:      1.5,
		RoutingMode:     model.AggregateGroupRoutingModeFailover,
		RecoveryEnabled: true,
	}
	require.NoError(t, group.SetVisibleUserGroups([]string{"vip"}))
	require.NoError(t, group.InsertWithTargets([]model.AggregateGroupTarget{
		{RealGroup: "default", OrderIndex: 0},
	}))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-test","group":"agg-playground"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "vip")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "vip")

	called := false
	handler := Distribute()
	handler(c)
	if !c.IsAborted() {
		called = true
	}

	require.True(t, called)
	require.Equal(t, "agg-playground", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	require.Equal(t, "agg-playground", common.GetContextKeyString(c, constant.ContextKeyAggregateGroup))
	require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyRouteGroup))
}
