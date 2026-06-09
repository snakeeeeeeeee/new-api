package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenAuthExtraGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.AggregateGroup{}, &model.AggregateGroupTarget{}))

	originalGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP分组"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedTokenAuthExtraGroupUser(t *testing.T, db *gorm.DB, setting dto.UserSetting) {
	t.Helper()
	user := &model.User{
		Id:       1,
		Username: "extra_auth_user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "vip",
		Quota:    1000,
	}
	user.SetSetting(setting)
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             1,
		UserId:         1,
		Key:            "extraauthkey",
		Status:         common.TokenStatusEnabled,
		Name:           "extra-auth-token",
		ExpiredTime:    -1,
		RemainQuota:    1000,
		UnlimitedQuota: true,
		Group:          "svip",
	}).Error)
}

func runTokenAuthExtraGroupRequest() (*httptest.ResponseRecorder, bool) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request.Header.Set("Authorization", "Bearer sk-extraauthkey")
	called := false
	handler := TokenAuth()
	handler(c)
	if !c.IsAborted() {
		called = true
	}
	return recorder, called
}

func TestTokenAuthAllowsExtraUsableGroupToken(t *testing.T) {
	db := setupTokenAuthExtraGroupTestDB(t)
	seedTokenAuthExtraGroupUser(t, db, dto.UserSetting{
		ExtraUsableGroups: []string{"svip"},
	})

	_, called := runTokenAuthExtraGroupRequest()
	require.True(t, called)
}

func TestTokenAuthRejectsRemovedExtraUsableGroupToken(t *testing.T) {
	db := setupTokenAuthExtraGroupTestDB(t)
	seedTokenAuthExtraGroupUser(t, db, dto.UserSetting{})

	recorder, called := runTokenAuthExtraGroupRequest()
	require.False(t, called)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "无权访问 svip 分组")
}
