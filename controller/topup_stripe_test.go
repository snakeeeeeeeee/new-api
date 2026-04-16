package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"gorm.io/gorm"
)

func setupStripeTopupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalStripeAPISecret := setting.StripeApiSecret
	originalStripeWebhookSecret := setting.StripeWebhookSecret
	originalStripePriceID := setting.StripePriceId

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.SubscriptionOrder{}))

	t.Cleanup(func() {
		setting.StripeApiSecret = originalStripeAPISecret
		setting.StripeWebhookSecret = originalStripeWebhookSecret
		setting.StripePriceId = originalStripePriceID
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedStripeTopupControllerUser(t *testing.T, db *gorm.DB, userID int) {
	t.Helper()
	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("stripe_user_%d", userID),
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    0,
	}
	require.NoError(t, db.Create(user).Error)
}

func seedStripeTopupOrder(t *testing.T, db *gorm.DB, userID int, tradeNo string, paymentMethod string, amount int64, money float64) {
	t.Helper()
	topUp := &model.TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         money,
		TradeNo:       tradeNo,
		PaymentMethod: paymentMethod,
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(topUp).Error)
}

func signedStripeWebhookRequest(t *testing.T, secret string, eventType string, object map[string]any) *http.Request {
	t.Helper()

	payload := map[string]any{
		"id":          "evt_test",
		"object":      "event",
		"api_version": "2025-03-31.basil",
		"type":        eventType,
		"data": map[string]any{
			"object": object,
		},
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: body,
		Secret:  secret,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signed.Header)
	return req
}

func TestRequestStripePayRejectsWhenWebhookSecretMissing(t *testing.T) {
	db := setupStripeTopupControllerTestDB(t)
	seedStripeTopupControllerUser(t, db, 1)

	setting.StripeApiSecret = "sk_test_valid"
	setting.StripeWebhookSecret = ""
	setting.StripePriceId = "price_test"

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", StripePayRequest{
		Amount:        100,
		PaymentMethod: PaymentMethodStripe,
	}, 1)
	RequestStripePay(ctx)

	var resp map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "error", resp["message"])
	require.Equal(t, "Stripe Webhook 未配置", resp["data"])

	var topUpCount int64
	require.NoError(t, db.Model(&model.TopUp{}).Count(&topUpCount).Error)
	require.Zero(t, topUpCount)
}

func TestRequestStripeAmountRejectsWhenWebhookSecretMissing(t *testing.T) {
	db := setupStripeTopupControllerTestDB(t)
	seedStripeTopupControllerUser(t, db, 2)

	setting.StripeApiSecret = "sk_test_valid"
	setting.StripeWebhookSecret = ""
	setting.StripePriceId = "price_test"

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/amount", StripePayRequest{
		Amount:        100,
		PaymentMethod: PaymentMethodStripe,
	}, 2)
	RequestStripeAmount(ctx)

	var resp map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "error", resp["message"])
	require.Equal(t, "Stripe Webhook 未配置", resp["data"])
}

func TestStripeWebhookRejectsMissingWebhookSecret(t *testing.T) {
	setupStripeTopupControllerTestDB(t)
	setting.StripeWebhookSecret = ""

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = signedStripeWebhookRequest(t, "", string(stripe.EventTypeCheckoutSessionCompleted), map[string]any{
		"status":              "complete",
		"payment_status":      "paid",
		"client_reference_id": "ref_test",
	})

	StripeWebhook(ctx)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestStripeWebhookSkipsUnpaidCheckoutSession(t *testing.T) {
	db := setupStripeTopupControllerTestDB(t)
	seedStripeTopupControllerUser(t, db, 3)
	seedStripeTopupOrder(t, db, 3, "ref_unpaid", PaymentMethodStripe, 100, 2)

	setting.StripeWebhookSecret = "whsec_test"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = signedStripeWebhookRequest(t, setting.StripeWebhookSecret, string(stripe.EventTypeCheckoutSessionCompleted), map[string]any{
		"status":              "complete",
		"payment_status":      "unpaid",
		"client_reference_id": "ref_unpaid",
		"customer":            "cus_unpaid",
		"amount_total":        "100",
		"currency":            "usd",
	})

	StripeWebhook(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	topUp := model.GetTopUpByTradeNo("ref_unpaid")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)

	user, err := model.GetUserById(3, false)
	require.NoError(t, err)
	require.Equal(t, 0, user.Quota)
	require.Empty(t, user.StripeCustomer)
}

func TestStripeWebhookAsyncPaymentSucceededCompletesStripeOrder(t *testing.T) {
	db := setupStripeTopupControllerTestDB(t)
	seedStripeTopupControllerUser(t, db, 4)
	seedStripeTopupOrder(t, db, 4, "ref_async_paid", PaymentMethodStripe, 100, 2.5)

	setting.StripeWebhookSecret = "whsec_test"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = signedStripeWebhookRequest(t, setting.StripeWebhookSecret, string(stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded), map[string]any{
		"client_reference_id": "ref_async_paid",
		"customer":            "cus_async",
		"amount_total":        "250",
		"currency":            "usd",
	})

	StripeWebhook(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	topUp := model.GetTopUpByTradeNo("ref_async_paid")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)

	user, err := model.GetUserById(4, false)
	require.NoError(t, err)
	require.Equal(t, int(2.5*common.QuotaPerUnit), user.Quota)
	require.Equal(t, "cus_async", user.StripeCustomer)
}

func TestStripeWebhookRejectsNonStripeTopUpOrder(t *testing.T) {
	db := setupStripeTopupControllerTestDB(t)
	seedStripeTopupControllerUser(t, db, 5)
	seedStripeTopupOrder(t, db, 5, "USR5NOATTACK", "微信", 500, 250)

	setting.StripeWebhookSecret = "whsec_test"

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = signedStripeWebhookRequest(t, setting.StripeWebhookSecret, string(stripe.EventTypeCheckoutSessionCompleted), map[string]any{
		"status":              "complete",
		"payment_status":      "paid",
		"client_reference_id": "USR5NOATTACK",
		"customer":            "cus_attack",
		"amount_total":        "100",
		"currency":            "usd",
	})

	StripeWebhook(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	topUp := model.GetTopUpByTradeNo("USR5NOATTACK")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
	require.Zero(t, topUp.CompleteTime)

	user, err := model.GetUserById(5, false)
	require.NoError(t, err)
	require.Equal(t, 0, user.Quota)
	require.Empty(t, user.StripeCustomer)
}
