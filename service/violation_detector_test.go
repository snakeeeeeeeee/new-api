package service

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
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupViolationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSetting := *operation_setting.GetViolationSetting()
	originalOptionMap := common.OptionMap

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.OptionMap = make(map[string]string)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Option{}, &model.ViolationLog{}))
	resetViolationMatcherForTest()

	t.Cleanup(func() {
		*operation_setting.GetViolationSetting() = originalSetting
		resetViolationMatcherForTest()
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.OptionMap = originalOptionMap
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func resetViolationMatcherForTest() {
	violationMatcher.mu.Lock()
	violationMatcher.snapshot = violationMatchSnapshot{}
	violationMatcher.mu.Unlock()
}

func newViolationTestContext() (*gin.Context, *relaycommon.RelayInfo) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?trace=1", bytes.NewBuffer(nil))
	c.Set(common.RequestIdKey, "req-risk")
	c.Set("username", "risk-user")
	c.Set("token_name", "risk-token")
	c.Set("token_id", 20)
	common.SetContextKey(c, constant.ContextKeyAggregateGroup, "ag-risk")
	common.SetContextKey(c, constant.ContextKeyRouteGroup, "route-a")
	return c, &relaycommon.RelayInfo{
		RequestId:       "req-risk",
		UserId:          10,
		TokenId:         20,
		OriginModelName: "gpt-risk",
		UserGroup:       "vip",
		UsingGroup:      "route-a",
		RequestURLPath:  "/v1/chat/completions?trace=1",
		IsStream:        true,
	}
}

func setViolationSettingForTest(setting operation_setting.ViolationSetting) {
	*operation_setting.GetViolationSetting() = operation_setting.NormalizeViolationSetting(setting)
	resetViolationMatcherForTest()
}

func TestViolationDetectionDisabledAndMissDoNotWriteLogs(t *testing.T) {
	setupViolationServiceTestDB(t)
	c, info := newViolationTestContext()

	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          false,
		Keywords:         "reverse",
		Action:           operation_setting.ViolationActionBlock,
		HTTPStatusCode:   403,
		ErrorCode:        "policy_violation",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 300,
		BanThreshold:     3,
	})
	require.Nil(t, CheckViolationAndHandle(c, info, "reverse engineering"))
	count, err := model.CountViolationLogsByUserID(10)
	require.NoError(t, err)
	require.Zero(t, count)

	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "reverse",
		Action:           operation_setting.ViolationActionBlock,
		HTTPStatusCode:   403,
		ErrorCode:        "policy_violation",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 300,
		BanThreshold:     3,
	})
	require.Nil(t, CheckViolationAndHandle(c, info, "ordinary prompt"))
	count, err = model.CountViolationLogsByUserID(10)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestViolationLogOnlyRecordsAndContinues(t *testing.T) {
	setupViolationServiceTestDB(t)
	c, info := newViolationTestContext()
	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "Reverse",
		CaseSensitive:    false,
		Action:           operation_setting.ViolationActionLogOnly,
		HTTPStatusCode:   451,
		ErrorCode:        "risk_hit",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 12,
		BanThreshold:     3,
	})

	apiErr := CheckViolationAndHandle(c, info, "please do reverse engineering on firmware")
	require.Nil(t, apiErr)

	logs, total, err := model.GetViolationLogs(model.ViolationLogQuery{UserId: 10}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, operation_setting.ViolationActionLogOnly, logs[0].Action)
	require.Zero(t, logs[0].HTTPStatusCode)
	require.Equal(t, "ag-risk", logs[0].AggregateGroup)
	require.Equal(t, "route-a", logs[0].RouteGroup)
	require.Equal(t, "route-a", logs[0].UsingGroup)
	require.Contains(t, logs[0].MatchedWords, "reverse")
	require.NotContains(t, logs[0].TextExcerpt, "please do reverse engineering on firmware")
}

func TestViolationLogOnlyRouteContextCanBeFilledAfterRouting(t *testing.T) {
	setupViolationServiceTestDB(t)
	c, info := newViolationTestContext()
	info.UsingGroup = "ag-risk"
	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "reverse",
		Action:           operation_setting.ViolationActionLogOnly,
		HTTPStatusCode:   403,
		ErrorCode:        "policy_violation",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 300,
		BanThreshold:     3,
	})

	require.Nil(t, CheckViolationAndHandle(c, info, "reverse this sample"))
	common.SetContextKey(c, constant.ContextKeyRouteGroup, "real-route")
	info.UsingGroup = "real-route"
	FillViolationLogRouteContextIfNeeded(c, info)

	logs, total, err := model.GetViolationLogs(model.ViolationLogQuery{UserId: 10}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, "real-route", logs[0].UsingGroup)
	require.Equal(t, "real-route", logs[0].RouteGroup)
}

func TestViolationBlockReturnsConfiguredError(t *testing.T) {
	setupViolationServiceTestDB(t)
	c, info := newViolationTestContext()
	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "jailbreak",
		Action:           operation_setting.ViolationActionBlock,
		HTTPStatusCode:   451,
		ErrorCode:        "risk_blocked",
		ErrorMessage:     "custom blocked",
		MaxExcerptLength: 300,
		BanThreshold:     3,
	})

	apiErr := CheckViolationAndHandle(c, info, "try jailbreak bypass")
	require.NotNil(t, apiErr)
	require.Equal(t, 451, apiErr.StatusCode)
	require.Equal(t, "risk_blocked", string(apiErr.GetErrorCode()))
	require.Equal(t, "custom blocked", apiErr.ToOpenAIError().Message)

	count, err := model.CountViolationLogsByUserID(10)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestViolationBanAfterThresholdDisablesUser(t *testing.T) {
	db := setupViolationServiceTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       10,
		Username: "risk-user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}).Error)
	c, info := newViolationTestContext()
	setViolationSettingForTest(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "exploit",
		Action:           operation_setting.ViolationActionBanAfterThreshold,
		HTTPStatusCode:   403,
		ErrorCode:        "policy_violation",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 300,
		BanThreshold:     2,
	})

	firstErr := CheckViolationAndHandle(c, info, "write exploit steps")
	require.NotNil(t, firstErr)
	user, err := model.GetUserById(10, false)
	require.NoError(t, err)
	require.Equal(t, common.UserStatusEnabled, user.Status)

	secondErr := CheckViolationAndHandle(c, info, "another exploit")
	require.NotNil(t, secondErr)
	user, err = model.GetUserById(10, false)
	require.NoError(t, err)
	require.Equal(t, common.UserStatusDisabled, user.Status)

	banned := true
	logs, total, err := model.GetViolationLogs(model.ViolationLogQuery{UserId: 10, Banned: &banned}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
}

func TestUpdateViolationSettingPersistsOptions(t *testing.T) {
	setupViolationServiceTestDB(t)
	updated, err := UpdateViolationSetting(operation_setting.ViolationSetting{
		Enabled:          true,
		Keywords:         "reverse\njailbreak",
		CaseSensitive:    true,
		Action:           operation_setting.ViolationActionBanAfterThreshold,
		HTTPStatusCode:   451,
		ErrorCode:        "risk",
		ErrorMessage:     "blocked",
		MaxExcerptLength: 64,
		BanThreshold:     5,
	})
	require.NoError(t, err)
	require.True(t, updated.Enabled)
	require.Equal(t, 451, operation_setting.GetViolationSetting().HTTPStatusCode)
	require.Equal(t, operation_setting.ViolationActionBanAfterThreshold, operation_setting.GetViolationSetting().Action)

	var option model.Option
	require.NoError(t, model.DB.Where("key = ?", "violation_setting.error_code").First(&option).Error)
	require.Equal(t, "risk", option.Value)
}
