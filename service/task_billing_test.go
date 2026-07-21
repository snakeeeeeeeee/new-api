package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Task{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.UserSubscription{},
		&model.Option{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM tasks")
		model.DB.Exec("DELETE FROM users")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM logs")
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM options")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:          id,
		UserId:      userId,
		Key:         key,
		Name:        "test_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedUnlimitedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:             id,
		UserId:         userId,
		Key:            key,
		Name:           "test_unlimited_token",
		Status:         common.TokenStatusEnabled,
		RemainQuota:    remainQuota,
		UnlimitedQuota: true,
		UsedQuota:      0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedSubscription(t *testing.T, id int, userId int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &model.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(ch).Error)
}

func makeTask(userId, channelId, quota, tokenId int, billingSource string, subscriptionId int) *model.Task {
	return &model.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userId,
		ChannelId: channelId,
		Quota:     quota,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionId,
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Read-back helpers
// ---------------------------------------------------------------------------

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func setUserUsageCounters(t *testing.T, id int, usedQuota int, requestCount int) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"used_quota":    usedQuota,
		"request_count": requestCount,
	}).Error)
}

func getUserUsageCounters(t *testing.T, id int) (int, int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("used_quota, request_count").Where("id = ?", id).First(&user).Error)
	return user.UsedQuota, user.RequestCount
}

func setChannelUsedQuota(t *testing.T, id int, usedQuota int64) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", id).Update("used_quota", usedQuota).Error)
}

func getChannelUsedQuota(t *testing.T, id int) int64 {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&channel).Error)
	return channel.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getLastLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	model.LOG_DB.Model(&model.Log{}).Count(&count)
	return count
}

func testGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func TestLogTaskConsumptionIncludesSubscriptionBillingMetadata(t *testing.T) {
	truncate(t)
	const userID = 115
	seedUser(t, userID, 10000)
	ctx := testGinContext()
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/image/tasks", nil)
	ctx.Set("username", "test_user")
	info := &relaycommon.RelayInfo{
		UserId:                                userID,
		OriginModelName:                       "task-model",
		ChannelMeta:                           &relaycommon.ChannelMeta{},
		TaskRelayInfo:                         &relaycommon.TaskRelayInfo{Action: "generate", PublicTaskID: "task_log_link"},
		UsingGroup:                            "default",
		BillingSource:                         BillingSourceSubscription,
		SubscriptionId:                        77,
		SubscriptionPreConsumed:               321,
		SubscriptionPlanId:                    9,
		SubscriptionPlanTitle:                 "Pro",
		SubscriptionAmountTotal:               1000,
		SubscriptionAmountUsedAfterPreConsume: 321,
		PriceData: types.PriceData{
			Quota:          321,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}

	consumeLogId := LogTaskConsumption(ctx, info)
	require.Positive(t, consumeLogId)
	logItem := getLastLog(t)
	require.NotNil(t, logItem)
	require.Equal(t, consumeLogId, logItem.Id)
	other, err := common.StrToMap(logItem.Other)
	require.NoError(t, err)
	require.Equal(t, "task_log_link", other["task_id"])
	require.Equal(t, BillingSourceSubscription, other["billing_source"])
	require.Equal(t, float64(77), other["subscription_id"])
	require.Equal(t, float64(321), other["subscription_consumed"])
	require.Equal(t, float64(0), other["wallet_quota_deducted"])
}

func TestGenerateMjOtherInfoIncludesBillingMetadata(t *testing.T) {
	info := &relaycommon.RelayInfo{
		BillingSource:         BillingSourceWallet,
		OriginModelName:       "mj-model",
		SubscriptionPlanTitle: "ignored",
	}
	other := GenerateMjOtherInfo(info, types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}})
	require.Equal(t, BillingSourceWallet, other["billing_source"])

	info.BillingSource = BillingSourceSubscription
	info.SubscriptionId = 88
	other = GenerateMjOtherInfo(info, types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}})
	require.Equal(t, BillingSourceSubscription, other["billing_source"])
	require.Equal(t, 88, other["subscription_id"])
}

// ===========================================================================
// Atomic quota decrement tests
// ===========================================================================

func TestDecreaseUserQuota_InsufficientDoesNotGoNegative(t *testing.T) {
	truncate(t)
	const userID = 101
	seedUser(t, userID, 100)

	err := model.DecreaseUserQuota(userID, 150)

	require.ErrorIs(t, err, model.ErrQuotaInsufficient)
	assert.Equal(t, 100, getUserQuota(t, userID))
}

func TestDecreaseUserQuota_ConcurrentDoesNotOversell(t *testing.T) {
	truncate(t)
	const userID = 102
	const initialQuota = 10
	const attempts = 100
	seedUser(t, userID, initialQuota)

	var success int64
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := model.DecreaseUserQuota(userID, 1)
			if err == nil {
				atomic.AddInt64(&success, 1)
				return
			}
			require.ErrorIs(t, err, model.ErrQuotaInsufficient)
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(initialQuota), success)
	assert.Equal(t, 0, getUserQuota(t, userID))
}

func TestDecreaseTokenQuota_ConcurrentDoesNotOversell(t *testing.T) {
	truncate(t)
	const userID, tokenID = 103, 103
	const initialRemain = 10
	const attempts = 100
	const tokenKey = "sk-token-concurrent"
	seedUser(t, userID, 1000)
	seedToken(t, tokenID, userID, tokenKey, initialRemain)

	var success int64
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := model.DecreaseTokenQuota(tokenID, tokenKey, 1)
			if err == nil {
				atomic.AddInt64(&success, 1)
				return
			}
			require.ErrorIs(t, err, model.ErrQuotaInsufficient)
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(initialRemain), success)
	assert.Equal(t, 0, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, initialRemain, getTokenUsedQuota(t, tokenID))
}

func TestDecreaseTokenQuota_UnlimitedDoesNotRequireRemainQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 104, 104
	const tokenKey = "sk-token-unlimited"
	seedUser(t, userID, 1000)
	seedUnlimitedToken(t, tokenID, userID, tokenKey, 0)

	require.NoError(t, model.DecreaseTokenQuota(tokenID, tokenKey, 500))

	assert.Equal(t, 0, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 500, getTokenUsedQuota(t, tokenID))
}

func TestAllowNegativeQuotaHelpersOnlyForAsyncSettlement(t *testing.T) {
	truncate(t)
	const userID, tokenID = 106, 106
	const tokenKey = "sk-token-debt"
	seedUser(t, userID, 100)
	seedToken(t, tokenID, userID, tokenKey, 50)

	require.NoError(t, model.DecreaseUserQuotaAllowNegative(userID, 150))
	require.NoError(t, model.DecreaseTokenQuotaAllowNegative(tokenID, tokenKey, 80))

	assert.Equal(t, -50, getUserQuota(t, userID))
	assert.Equal(t, -30, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 80, getTokenUsedQuota(t, tokenID))
}

func TestNewBillingSession_ConcurrentWalletPreConsumeMapsInsufficientQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 105, 105
	const tokenKey = "sk-billing-concurrent"
	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, tokenKey, 10000)

	relayInfo := &relaycommon.RelayInfo{
		UserId:     userID,
		TokenId:    tokenID,
		TokenKey:   tokenKey,
		UsingGroup: "default",
	}

	session := &BillingSession{
		relayInfo: relayInfo,
		funding:   &WalletFunding{userId: userID},
	}
	apiErr := session.preConsume(testGinContext(), 100)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.True(t, errors.Is(apiErr, model.ErrQuotaInsufficient), "expected quota sentinel, got %v", apiErr.Err)
	assert.Equal(t, 0, getUserQuota(t, userID))
	assert.Equal(t, 10000, getTokenRemainQuota(t, tokenID))
}

func TestPreConsumeBillingRejectsNegativeQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 107, 107
	const tokenKey = "sk-negative-preconsume"
	seedUser(t, userID, 1000)
	seedToken(t, tokenID, userID, tokenKey, 500)

	relayInfo := &relaycommon.RelayInfo{UserId: userID, TokenId: tokenID, TokenKey: tokenKey, UsingGroup: "default"}
	apiErr := PreConsumeBilling(testGinContext(), -1, relayInfo)

	require.NotNil(t, apiErr)
	assert.Equal(t, 1000, getUserQuota(t, userID))
	assert.Equal(t, 500, getTokenRemainQuota(t, tokenID))
}

func TestResourceKeyBillingConsumesOnlyAccountQuota(t *testing.T) {
	truncate(t)
	const userID = 113
	seedUser(t, userID, 1000)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(testGinContext(), relayInfo, 100)
	require.Nil(t, apiErr)
	assert.Equal(t, 900, getUserQuota(t, userID))

	require.NoError(t, session.Settle(60))
	assert.Equal(t, 940, getUserQuota(t, userID))
	assert.Zero(t, relayInfo.TokenId)
	assert.Empty(t, relayInfo.TokenKey)
}

func TestSettleBillingRejectsNegativeActualQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 108, 108
	const tokenKey = "sk-negative-settle"
	seedUser(t, userID, 1000)
	seedToken(t, tokenID, userID, tokenKey, 500)

	relayInfo := &relaycommon.RelayInfo{UserId: userID, TokenId: tokenID, TokenKey: tokenKey, UsingGroup: "default"}
	err := SettleBilling(testGinContext(), relayInfo, -1)

	require.Error(t, err)
	assert.Equal(t, 1000, getUserQuota(t, userID))
	assert.Equal(t, 500, getTokenRemainQuota(t, tokenID))
}

func TestBillingSessionSettleRejectsNegativeActualQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 109, 109
	const tokenKey = "sk-negative-session-settle"
	seedUser(t, userID, 1000)
	seedToken(t, tokenID, userID, tokenKey, 500)

	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{UserId: userID, TokenId: tokenID, TokenKey: tokenKey},
		funding:   &WalletFunding{userId: userID},
	}

	require.Error(t, session.Settle(-1))
	assert.Equal(t, 1000, getUserQuota(t, userID))
	assert.Equal(t, 500, getTokenRemainQuota(t, tokenID))
}

func TestBillingSessionSettleAllowsLegitimateRefundDelta(t *testing.T) {
	truncate(t)
	const userID, tokenID = 110, 110
	const tokenKey = "sk-legitimate-refund-delta"
	seedUser(t, userID, 1000)
	seedToken(t, tokenID, userID, tokenKey, 500)

	relayInfo := &relaycommon.RelayInfo{UserId: userID, TokenId: tokenID, TokenKey: tokenKey, UsingGroup: "default"}
	session, apiErr := NewBillingSession(testGinContext(), relayInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(60))
	require.NoError(t, session.Settle(60))

	assert.Equal(t, 940, getUserQuota(t, userID))
	assert.Equal(t, 440, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 60, getTokenUsedQuota(t, tokenID))
}

func TestBillingSessionSettleRefundDeltaDoesNotIncreaseUnlimitedTokenRemainQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID = 112, 112
	const tokenKey = "sk-unlimited-legitimate-refund-delta"
	const initialUserQuota, initialTokenRemain = 1000, 777
	seedUser(t, userID, initialUserQuota)
	seedUnlimitedToken(t, tokenID, userID, tokenKey, initialTokenRemain)

	relayInfo := &relaycommon.RelayInfo{
		UserId:         userID,
		TokenId:        tokenID,
		TokenKey:       tokenKey,
		TokenUnlimited: true,
		UsingGroup:     "default",
		UserSetting:    dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(testGinContext(), relayInfo, 100)
	require.Nil(t, apiErr)
	assert.Equal(t, initialUserQuota-100, getUserQuota(t, userID))
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 100, getTokenUsedQuota(t, tokenID))

	require.NoError(t, session.Settle(60))
	require.NoError(t, session.Settle(60))

	assert.Equal(t, initialUserQuota-60, getUserQuota(t, userID))
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 60, getTokenUsedQuota(t, tokenID))
}

func TestRetryToHigherRouteRatioSettlesAdditionalQuotaOnlyOnce(t *testing.T) {
	truncate(t)
	const userID, tokenID = 111, 111
	const tokenKey = "sk-route-ratio-retry"
	const initialQuota = 10000
	const lowRoutePrecharge = 100
	const highRouteActualQuota = 400
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, tokenKey, initialQuota)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		OriginModelName: "premium-model",
		UsingGroup:      "aggregate-retry",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:                    4,
			OriginalGroupRatio:            1,
			RouteModelGroupRatio:          4,
			HasRouteModelGroupRatio:       true,
			RouteModelRatioAggregateGroup: "aggregate-retry",
			RouteModelRatioRealGroup:      "high-cost-route",
			RouteModelRatioModelName:      "premium-model",
		}},
	}
	require.Nil(t, PreConsumeBilling(testGinContext(), lowRoutePrecharge, relayInfo))
	require.NoError(t, SettleBilling(testGinContext(), relayInfo, highRouteActualQuota))
	require.NoError(t, SettleBilling(testGinContext(), relayInfo, highRouteActualQuota))

	assert.Equal(t, initialQuota-highRouteActualQuota, getUserQuota(t, userID))
	assert.Equal(t, initialQuota-highRouteActualQuota, getTokenRemainQuota(t, tokenID))
}

// ===========================================================================
// RefundTaskQuota tests
// ===========================================================================

func TestRefundTaskQuota_Wallet(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 1, 1, 1
	const initQuota, preConsumed = 10000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-key", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "task failed: upstream error")

	// User quota should increase by preConsumed
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Token remain_quota should increase, used_quota should decrease
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -preConsumed, getTokenUsedQuota(t, tokenID))
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, preConsumed, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(preConsumed), getChannelUsedQuota(t, channelID))

	// A refund log should be created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "test-model", log.ModelName)
}

func TestRefundTaskQuota_Subscription(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 2, 2, 2, 1
	const preConsumed = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-key", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RefundTaskQuota(ctx, task, "subscription task failed")

	// Subscription used should decrease by preConsumed
	assert.Equal(t, subUsed-int64(preConsumed), getSubscriptionUsed(t, subID))

	// Token should also be refunded
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRefundTaskQuotaUnlimitedTokenDoesNotIncreaseRemainQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 5, 5, 5
	const initialUserQuota, initialTokenRemain, preConsumed = 10000, 1234, 3000
	const tokenKey = "sk-task-unlimited-refund"

	seedUser(t, userID, initialUserQuota)
	seedUnlimitedToken(t, tokenID, userID, tokenKey, initialTokenRemain)
	seedChannel(t, channelID)
	require.NoError(t, model.DecreaseUserQuota(userID, preConsumed))
	require.NoError(t, model.DecreaseTokenQuota(tokenID, tokenKey, preConsumed))
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "unlimited token task failed")

	assert.Equal(t, initialUserQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Zero(t, getTokenUsedQuota(t, tokenID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestRefundTaskQuota_ZeroQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 3
	seedUser(t, userID, 5000)

	task := makeTask(userID, 0, 0, 0, BillingSourceWallet, 0)

	RefundTaskQuota(ctx, task, "zero quota task")

	// No change to user quota
	assert.Equal(t, 5000, getUserQuota(t, userID))

	// No log created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRefundTaskQuota_NoToken(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 4, 4
	const initQuota, preConsumed = 10000, 1500

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0) // TokenId=0

	RefundTaskQuota(ctx, task, "no token task failed")

	// User quota refunded
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	// Log created
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestXaiVideoFailedAggregateTaskRefundKeepsDeductedQuotaInLog(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 41, 41, 41
	const initQuota = 20000
	const tokenRemain = 10000
	preConsumed := int(0.5 * common.QuotaPerUnit)

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-xai-aggregate-refund", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Group = "enterprise-stable"
	task.Properties.OriginModelName = "grok-imagine-video-test"
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:         1,
		GroupRatio:         0.5,
		OriginalGroupRatio: 2,
		RatioOverride:      0.5,
		HasRatioOverride:   true,
		OriginModelName:    "grok-imagine-video-test",
		PerCallBilling:     true,
	}

	RefundTaskQuota(ctx, task, "upstream failed")

	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "enterprise-stable", log.Group)
	assert.Equal(t, "grok-imagine-video-test", log.ModelName)
	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	assert.Equal(t, float64(1), other["model_price"])
	assert.Equal(t, float64(0.5), other["group_ratio"])
	assert.Equal(t, float64(2), other["original_group_ratio"])
	assert.Equal(t, float64(2), other["original_ratio"])
	assert.Equal(t, float64(0.5), other["ratio_override"])
	assert.Equal(t, true, other["has_ratio_override"])
	assert.Equal(t, "upstream failed", other["reason"])
}

// ===========================================================================
// RecalculateTaskQuota tests
// ===========================================================================

func TestRecalculate_PositiveDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 10, 10, 10
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000 // under-charged by 1000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-pos", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should decrease by the delta (1000 additional charge)
	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))

	// Token should also be charged the delta
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Consume (additional charge)
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Equal(t, actualQuota-preConsumed, log.Quota)
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, actualQuota, userUsedQuota)
	assert.Equal(t, 2, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))
}

func TestRecalculateImageHandlePositiveDeltaAdjustsUsedQuotaWithoutExtraRequest(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 110, 110, 110
	const initQuota, preConsumed, actualQuota = 10000, 2000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-image-recalc-pos", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = imageHandleTaskPlatform()
	task.PrivateData.BillingContext.BillingMode = "async_image_usage_billing"

	RecalculateTaskQuota(ctx, task, actualQuota, "image-handle actual_quota fallback")

	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, actualQuota, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))
}

func TestRecalculate_LogKeepsAggregateRatioOverrideInfo(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 15, 15, 15
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-override", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.GroupRatio = 0.5
	task.PrivateData.BillingContext.OriginalGroupRatio = 2
	task.PrivateData.BillingContext.RatioOverride = 0.5
	task.PrivateData.BillingContext.HasRatioOverride = true

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	log := getLastLog(t)
	require.NotNil(t, log)
	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	assert.Equal(t, float64(0.5), other["group_ratio"])
	assert.Equal(t, float64(2), other["original_group_ratio"])
	assert.Equal(t, float64(2), other["original_ratio"])
	assert.Equal(t, float64(0.5), other["ratio_override"])
	assert.Equal(t, true, other["has_ratio_override"])
}

func TestTaskBillingLogUsesEffectiveRouteModelRatio(t *testing.T) {
	task := makeTask(1, 1, 100, 1, BillingSourceWallet, 0)
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:               1,
		GroupRatio:               4,
		GroupSpecialRatio:        -1,
		OriginalGroupRatio:       2,
		RatioOverride:            0.5,
		HasRatioOverride:         true,
		RatioOverrideApplied:     false,
		RouteModelGroupRatio:     4,
		HasRouteModelGroupRatio:  true,
		RouteModelAggregateGroup: "aggregate-premium",
		RouteModelRealGroup:      "premium-route",
		RouteModelName:           "premium-model",
		RouteModelRatioSource:    types.RouteModelGroupRatioSourceUser,
	}

	other := taskBillingOther(task)
	assert.Equal(t, float64(4), other["group_ratio"])
	assert.NotContains(t, other, "user_group_ratio")
	assert.Equal(t, float64(0.5), other["ratio_override"])
	assert.Equal(t, false, other["ratio_override_applied"])
	assert.Equal(t, true, other["route_model_group_ratio_applied"])
	assert.Equal(t, float64(4), other["route_model_group_ratio"])
	assert.Equal(t, "aggregate-premium", other["route_model_ratio_aggregate_group"])
	assert.Equal(t, "premium-route", other["route_model_ratio_real_group"])
	assert.Equal(t, "premium-model", other["route_model_ratio_model_name"])
	assert.Equal(t, types.RouteModelGroupRatioSourceUser, other["route_model_group_ratio_source"])
}

func TestTaskBillingRouteModelRatioSnapshotPreservesZero(t *testing.T) {
	task := makeTask(1, 1, 0, 1, BillingSourceWallet, 0)
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		GroupRatio:               0,
		OriginalGroupRatio:       1.5,
		RouteModelGroupRatio:     0,
		HasRouteModelGroupRatio:  true,
		RouteModelAggregateGroup: "aggregate-free-route",
		RouteModelRealGroup:      "free-route",
		RouteModelName:           "free-model",
		RouteModelRatioSource:    types.RouteModelGroupRatioSourceUser,
	}

	info := taskRelayInfoForBilling(task)
	assert.Zero(t, info.PriceData.GroupRatioInfo.GroupRatio)
	assert.Zero(t, info.PriceData.GroupRatioInfo.RouteModelGroupRatio)
	assert.True(t, info.PriceData.GroupRatioInfo.HasRouteModelGroupRatio)
	assert.Equal(t, 1.5, info.PriceData.GroupRatioInfo.OriginalGroupRatio)
	assert.Equal(t, types.RouteModelGroupRatioSourceUser, info.PriceData.GroupRatioInfo.RouteModelGroupRatioSource)
}

func TestGenerateTextOtherInfoDoesNotApplySuppressedUserRatio(t *testing.T) {
	ctx := testGinContext()
	now := time.Now()
	relayInfo := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:                    4,
			GroupSpecialRatio:             -1,
			OriginalGroupRatio:            2,
			RatioOverride:                 0.5,
			HasRatioOverride:              true,
			RatioOverrideApplied:          false,
			RouteModelGroupRatio:          4,
			HasRouteModelGroupRatio:       true,
			RouteModelRatioAggregateGroup: "aggregate-premium",
			RouteModelRatioRealGroup:      "premium-route",
			RouteModelRatioModelName:      "premium-model",
		}},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 4, 1, 0, 1, -1, -1)
	assert.Equal(t, float64(4), other["group_ratio"])
	assert.Equal(t, float64(-1), other["user_group_ratio"])
	assert.Equal(t, false, other["ratio_override_applied"])
	assert.Equal(t, float64(4), other["route_model_group_ratio"])
}

func TestRecalculate_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 11, 11, 11
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged by 2000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-neg", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, actualQuota, "adaptor adjustment")

	// User quota should increase by abs(delta) = 2000 (refund overpayment)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))

	// Token should be refunded the difference
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota updated
	assert.Equal(t, actualQuota, task.Quota)

	// Log type should be Refund
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, preConsumed, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(preConsumed), getChannelUsedQuota(t, channelID))
}

func TestSettleImageHandleUsageNegativeDeltaAdjustsUsedQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 111, 111, 111
	const initQuota, preConsumed, actualQuota = 10000, 5000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-image-usage-neg", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = imageHandleTaskPlatform()
	task.PrivateData.BillingContext.BillingMode = "async_image_usage_billing"

	settleTaskQuotaDeltaWithUsage(ctx, task, textQuotaSummary{
		ModelName: "test-model",
		Quota:     actualQuota,
	}, "异步图片按量真实结算", false)

	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, actualQuota, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))
	assert.Equal(t, actualQuota, task.Quota)
}

func TestRecalculate_ZeroDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 12
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, preConsumed, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, preConsumed, "exact match")

	// No change to user quota
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No log created (delta is zero)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_ActualQuotaZero(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID = 13
	const initQuota = 10000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, 5000, 0, BillingSourceWallet, 0)

	RecalculateTaskQuota(ctx, task, 0, "zero actual")

	// No change (early return)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecalculate_Subscription_NegativeDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 14, 14, 14, 2
	const preConsumed = 5000
	const actualQuota = 2000 // over-charged by 3000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-recalc", tokenRemain)
	seedChannel(t, channelID)
	seedSubscription(t, subID, userID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceSubscription, subID)

	RecalculateTaskQuota(ctx, task, actualQuota, "subscription over-charge")

	// Subscription used should decrease by delta (refund 3000)
	assert.Equal(t, subUsed-int64(preConsumed-actualQuota), getSubscriptionUsed(t, subID))

	// Token refunded
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

// ===========================================================================
// CAS + Billing integration tests
// Simulates the flow in updateVideoSingleTask (service/task_polling.go)
// ===========================================================================

// simulatePollBilling reproduces the CAS + billing logic from updateVideoSingleTask.
// It takes a persisted task (already in DB), applies the new status, and performs
// the conditional update + billing exactly as the polling loop does.
func simulatePollBilling(ctx context.Context, task *model.Task, newStatus model.TaskStatus, actualQuota int) {
	snap := task.Snapshot()

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = newStatus
	switch string(newStatus) {
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
		shouldSettle = true
	case model.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
		if quota != 0 {
			shouldRefund = true
		}
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == model.TaskStatus(model.TaskStatusSuccess) || task.Status == model.TaskStatus(model.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}

	if shouldSettle && actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "test settle")
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS wins: task in DB should now be FAILURE
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)

	// Refund should have happened
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	// Create task with IN_PROGRESS in DB
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate another process already transitioning to FAILURE
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("status", model.TaskStatusFailure)

	// Our process still has the old in-memory state (IN_PROGRESS) and tries to transition
	// task.Status is still IN_PROGRESS in the snapshot
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS lost: user quota should NOT change (no double refund)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// No billing log should be created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged, should get partial refund
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusSuccess), actualQuota)

	// CAS wins: task should be SUCCESS
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)

	// Settlement should refund the over-charge (5000 - 3000 = 2000 back to user)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate a non-terminal poll update (still IN_PROGRESS, progress changed)
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusInProgress), 0)

	// User quota should NOT change
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No billing log
	assert.Equal(t, int64(0), countLogs(t))

	// Task progress should be updated in DB
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

// ===========================================================================
// Mock adaptor for settleTaskBillingOnComplete tests
// ===========================================================================

type mockAdaptor struct {
	adjustReturn int
	fetchResp    *http.Response
	parseCalled  bool
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}
func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return m.fetchResp, nil
}
func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }
func (m *mockAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}
func (m *mockAdaptor) ParseBatchTaskResult([]byte) (map[string]*relaycommon.TaskInfo, error) {
	m.parseCalled = true
	return map[string]*relaycommon.TaskInfo{}, nil
}

func TestUpdateVideoBatchTasksRejectsNon2xxResponse(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	task := makeTask(40, 40, 0, 0, BillingSourceWallet, 0)
	task.TaskID = "task_batch_error"
	task.PrivateData.UpstreamTaskID = "imgtask_batch_error"
	task.Status = model.TaskStatusQueued
	require.NoError(t, model.DB.Create(task).Error)

	baseURL := "http://image-handle.test"
	adaptor := &mockAdaptor{
		fetchResp: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{"error":"upstream unavailable"}`)),
		},
	}
	err := updateVideoBatchTasks(ctx, adaptor, adaptor, &model.Channel{
		Id:      40,
		Type:    constant.ChannelTypeImageHandle,
		Key:     "provider-key",
		BaseURL: &baseURL,
	}, []string{"imgtask_batch_error"}, map[string]*model.Task{
		"imgtask_batch_error": task,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status code 500")
	assert.False(t, adaptor.parseCalled)

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusQueued, reloaded.Status)
}

// ===========================================================================
// PerCallBilling tests — settleTaskBillingOnComplete
// ===========================================================================

func TestSettle_PerCallBilling_AppliesActualQuotaFromAdaptor(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000
	const actualQuota = 9000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: actualQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)
	assert.Equal(t, int64(1), countLogs(t))
}

func TestSettle_PerCallBilling_SkipsTotalTokens(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 9999}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Per-call: no recalculation by tokens
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettle_ImageHandlePerCallSuccessKeepsPrecharge(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 33, 33, 33
	const initQuota, preConsumed = 100, 80
	const actualQuota = 500
	const tokenRemain = 30

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-async-image-debt", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.PrivateData.BillingContext.PerCallBilling = true
	task.PrivateData.BillingContext.BillingMode = "async_image_usage_billing"

	adaptor := &mockAdaptor{adjustReturn: actualQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
}

func TestSettle_ImageParameterPerCallKeepsFrozenSnapshotAndIgnoresExecutorUsage(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 35, 35, 35
	const initQuota, preConsumed = 100000, 40000
	const tokenRemain = 80000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-image-parameter-frozen", tokenRemain)
	seedChannel(t, channelID)

	originalImagePricing := ratio_setting.ImagePricing2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(originalImagePricing))
	})
	require.NoError(t, ratio_setting.UpdateImagePricingByJSONString(`{
		"version":1,
		"profiles":{
			"quality-v2":{
				"name":"changed after submit",
				"parameter":"quality",
				"default_tier":"low",
				"tiers":[{"key":"low","upstream_value":"low","aliases":[],"unit_price":0.99}]
			}
		},
		"model_bindings":{"public-image-count":{"profile":"quality-v2","max_n":10}}
	}`))

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OriginModelName: "public-image-count",
		BillingMode:     types.ImagePricingBillingMode,
		PerCallBilling:  true,
		UsePrice:        true,
		ImagePricing: &types.ImagePricingSnapshot{
			PublicModel:   "public-image-count",
			ProfileID:     "quality-v1",
			ProfileHash:   "frozen-profile-hash",
			Parameter:     types.ImagePricingParameterQuality,
			RawValue:      "low",
			EffectiveTier: "low",
			UpstreamValue: "low",
			ValueSource:   types.ImagePricingValueSourceRequest,
			UnitPrice:     0.04,
			N:             2,
			Subtotal:      0.08,
			GroupRatio:    1,
			FinalQuota:    preConsumed,
		},
	}
	originalOther := taskBillingOther(task)
	originalOther["task_id"] = task.TaskID
	originalLog := &model.Log{
		UserId:    userID,
		CreatedAt: task.CreatedAt,
		Type:      model.LogTypeConsume,
		Content:   imagePricingLogContent(task.PrivateData.BillingContext.ImagePricing),
		ModelName: "public-image-count",
		Quota:     preConsumed,
		ChannelId: channelID,
		TokenId:   tokenID,
		Group:     task.Group,
		Other:     common.MapToJsonStr(originalOther),
	}
	require.NoError(t, model.LOG_DB.Create(originalLog).Error)
	task.PrivateData.BillingContext.ConsumeLogId = originalLog.Id
	task.Data = json.RawMessage(`{
		"result":{
			"images":[{"url":"https://cdn.example.com/one.png"},{"url":"https://cdn.example.com/two.png"}],
			"output":{"quality":"high","size":"2048x2048","resolution":"2k"}
		},
		"usage":{"total_tokens":123,"actual_quota":456}
	}`)

	// Both callback usage and executor-provided quota disagree with the frozen
	// request snapshot. Neither may reprice image_parameter_per_call tasks.
	taskResult := &relaycommon.TaskInfo{
		Status:      model.TaskStatusSuccess,
		ActualQuota: 990000,
		TotalTokens: 9999,
		Usage: &dto.Usage{
			PromptTokens:     111,
			CompletionTokens: 888,
			TotalTokens:      999,
		},
	}
	settleTaskBillingOnComplete(ctx, &mockAdaptor{adjustReturn: 880000}, task, taskResult)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(1), countLogs(t))
	require.NotNil(t, task.PrivateData.BillingContext.ImagePricing)
	assert.Equal(t, "quality-v1", task.PrivateData.BillingContext.ImagePricing.ProfileID)
	assert.Equal(t, "frozen-profile-hash", task.PrivateData.BillingContext.ImagePricing.ProfileHash)
	assert.Equal(t, 0.04, task.PrivateData.BillingContext.ImagePricing.UnitPrice)
	assert.Equal(t, preConsumed, task.PrivateData.BillingContext.ImagePricing.FinalQuota)

	consumeLog := getLastLog(t)
	require.NotNil(t, consumeLog)
	assert.Equal(t, originalLog.Id, consumeLog.Id)
	assert.Equal(t, model.LogTypeConsume, consumeLog.Type)
	assert.Equal(t, preConsumed, consumeLog.Quota)
	assert.Contains(t, consumeLog.Content, "按张（图片）")
	var auditOther map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(consumeLog.Other), &auditOther))
	assert.Nil(t, auditOther["non_billing_audit"])
	assert.Nil(t, auditOther["billing_stage"])
	assert.Equal(t, task.TaskID, auditOther["task_id"])
	snapshot, ok := auditOther["image_pricing_snapshot"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "frozen-profile-hash", snapshot["profile_hash"])
	assert.Equal(t, float64(preConsumed), snapshot["final_quota"])
	audit, ok := auditOther["image_execution_audit"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "high", audit["quality"])
	assert.Equal(t, "2048x2048", audit["size"])
	assert.Equal(t, "2k", audit["resolution"])
	assert.Equal(t, float64(2), audit["image_count"])
	assert.Equal(t, float64(111), audit["input_tokens"])
	assert.Equal(t, float64(888), audit["output_tokens"])
	assert.Equal(t, float64(999), audit["total_tokens"])
	assert.Equal(t, float64(990000), audit["actual_quota"])

	liveProfile, _, _, found := ratio_setting.GetImagePricingForModel("public-image-count")
	require.True(t, found)
	require.Len(t, liveProfile.Tiers, 1)
	assert.Equal(t, 0.99, liveProfile.Tiers[0].UnitPrice)
}

func TestRecordImagePricingExecutionAuditWithoutLinkedConsumeLogDoesNotCreateLog(t *testing.T) {
	truncate(t)
	task := makeTask(50, 50, 20000, 50, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.BillingMode = types.ImagePricingBillingMode
	task.PrivateData.BillingContext.ImagePricing = &types.ImagePricingSnapshot{
		PublicModel:   "public-image-count",
		EffectiveTier: "low",
		UnitPrice:     0.04,
		N:             1,
		FinalQuota:    20000,
	}
	task.Data = json.RawMessage(`{"result":{"images":[{"url":"https://cdn.example.com/one.png"}],"output":{"quality":"low"}}}`)

	recordImagePricingExecutionAudit(task, nil)

	assert.Zero(t, countLogs(t))
}

func TestMergeCompletedImagePricingExecutionAuditHandlesEarlyCallbackRace(t *testing.T) {
	truncate(t)
	task := makeTask(51, 51, 20000, 51, BillingSourceWallet, 0)
	task.Status = model.TaskStatusSuccess
	task.PrivateData.BillingContext.BillingMode = types.ImagePricingBillingMode
	task.PrivateData.BillingContext.ImagePricing = &types.ImagePricingSnapshot{
		PublicModel:   "public-image-count",
		EffectiveTier: "low",
		UnitPrice:     0.04,
		N:             1,
		FinalQuota:    20000,
	}
	task.Data = json.RawMessage(`{"result":{"images":[{"url":"https://cdn.example.com/one.png"}],"output":{"quality":"low","size":"1024x1024"}}}`)
	require.NoError(t, model.DB.Create(task).Error)
	originalLog := &model.Log{
		UserId:    task.UserId,
		CreatedAt: task.CreatedAt,
		Type:      model.LogTypeConsume,
		ModelName: "public-image-count",
		Quota:     task.Quota,
		Other:     common.MapToJsonStr(map[string]interface{}{"task_id": task.TaskID}),
	}
	require.NoError(t, model.LOG_DB.Create(originalLog).Error)
	require.NoError(t, model.PersistTaskSubmitResult(task.ID, "imgtask_early", nil, originalLog.Id))

	MergeCompletedImagePricingExecutionAudit(task.ID)

	require.Equal(t, int64(1), countLogs(t))
	consumeLog := getLastLog(t)
	other, err := common.StrToMap(consumeLog.Other)
	require.NoError(t, err)
	audit, ok := other["image_execution_audit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "low", audit["quality"])
	require.Equal(t, "1024x1024", audit["size"])
}

func TestRecordImagePricingExecutionAuditReloadsConsumeLogIdForStaleCallbackTask(t *testing.T) {
	truncate(t)
	task := makeTask(52, 52, 20000, 52, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.BillingMode = types.ImagePricingBillingMode
	task.PrivateData.BillingContext.ImagePricing = &types.ImagePricingSnapshot{
		PublicModel:   "public-image-count",
		EffectiveTier: "low",
		UnitPrice:     0.04,
		N:             1,
		FinalQuota:    20000,
	}
	task.Data = json.RawMessage(`{"result":{"images":[{"url":"https://cdn.example.com/one.png"}],"output":{"quality":"low"}}}`)
	require.NoError(t, model.DB.Create(task).Error)
	originalLog := &model.Log{
		UserId:    task.UserId,
		CreatedAt: task.CreatedAt,
		Type:      model.LogTypeConsume,
		ModelName: "public-image-count",
		Quota:     task.Quota,
		Other:     common.MapToJsonStr(map[string]interface{}{"task_id": task.TaskID}),
	}
	require.NoError(t, model.LOG_DB.Create(originalLog).Error)
	require.Zero(t, task.PrivateData.BillingContext.ConsumeLogId)
	require.NoError(t, model.PersistTaskSubmitResult(task.ID, "imgtask_racing", nil, originalLog.Id))

	// The callback still holds the task object loaded before PersistTaskSubmitResult.
	recordImagePricingExecutionAudit(task, nil)

	require.Equal(t, int64(1), countLogs(t))
	consumeLog := getLastLog(t)
	other, err := common.StrToMap(consumeLog.Other)
	require.NoError(t, err)
	audit, ok := other["image_execution_audit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "low", audit["quality"])
}

func TestSettle_ImageHandleLegacyUsageTaskWithoutSnapshotUsesActualQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 36, 36, 36
	const initQuota, preConsumed, actualQuota = 10000, 5000, 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-image-legacy-actual-quota", tokenRemain)
	seedChannel(t, channelID)
	setUserUsageCounters(t, userID, preConsumed, 1)
	setChannelUsedQuota(t, channelID, preConsumed)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OriginModelName: "gpt-image-2",
		BillingMode:     "async_image_usage_billing",
		ModelPrice:      -1,
		GroupRatio:      1,
	}

	settleTaskBillingOnComplete(ctx, &mockAdaptor{}, task, &relaycommon.TaskInfo{
		Status:      model.TaskStatusSuccess,
		ActualQuota: actualQuota,
	})

	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)
	userUsedQuota, requestCount := getUserUsageCounters(t, userID)
	assert.Equal(t, actualQuota, userUsedQuota)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, int64(actualQuota), getChannelUsedQuota(t, channelID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
}

func TestSettle_ImageHandleUsageBillingUsesCallbackUsageAndCanDriveDebt(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 34, 34, 34
	const initQuota, preConsumed = 100, 500000
	const tokenRemain = 30

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-async-image-usage-debt", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.TaskID = "task_async_usage_billing"
	task.Properties.OriginModelName = "gpt-image-2"
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OriginModelName:    "gpt-image-2",
		BillingMode:        "async_image_usage_billing",
		ModelPrice:         -1,
		ModelRatio:         2.5,
		CompletionRatio:    6,
		CacheRatio:         0.4,
		GroupRatio:         1.3,
		GroupSpecialRatio:  1.3,
		HasSpecialRatio:    true,
		OriginalGroupRatio: 6,
		OtherRatios: map[string]float64{
			"async_image_precharge_amount_per_image_usd": 1,
			"async_image_precharge_quota_per_image":      500000,
			"async_image_n":                              1,
		},
	}

	taskResult := &relaycommon.TaskInfo{
		Status: model.TaskStatusSuccess,
		Usage: &dto.Usage{
			PromptTokens:     19,
			CompletionTokens: 781,
			TotalTokens:      800,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 5,
			},
		},
	}

	settleTaskBillingOnComplete(ctx, &mockAdaptor{}, task, taskResult)

	// ((19-5)*5/1M + 5*2/1M + 781*30/1M) * 1.3 * 500000 = 15282.15 -> 15282 quota
	const actualQuota = 15282
	delta := actualQuota - preConsumed
	assert.Equal(t, initQuota-delta, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-delta, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
	assert.Equal(t, 19, log.PromptTokens)
	assert.Equal(t, 781, log.CompletionTokens)
	var other map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
	assert.Equal(t, float64(5), other["cache_tokens"])
	assert.Equal(t, float64(1.3), other["user_group_ratio"])
	assert.Equal(t, float64(6), other["original_group_ratio"])
}

func TestSettle_NonPerCall_AdaptorAdjustWorks(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	// PerCallBilling defaults to false

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	// Non-per-call: adaptor adjustment applies (refund 2000)
	assert.Equal(t, initQuota+(preConsumed-adaptorQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-adaptorQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, adaptorQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}
