package controller

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupExternalTopupControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalExternalTopupEnabled := common.ExternalTopupEnabled
	originalExternalTopupAuthKey := common.ExternalTopupAuthKey
	originalExternalTopupCallbackSecret := common.ExternalTopupCallbackSecret
	originalPayAddress := operation_setting.PayAddress
	originalCustomCallbackAddress := operation_setting.CustomCallbackAddress
	originalEpayId := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayServerInternalToken := operation_setting.PayServerInternalToken
	originalPrice := operation_setting.Price
	originalMinTopUp := operation_setting.MinTopUp
	originalPayMethods := operation_setting.PayMethods
	originalServerAddress := system_setting.ServerAddress
	originalFetchSetting := *system_setting.GetFetchSetting()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.ExternalTopupEnabled = false
	common.ExternalTopupAuthKey = ""
	common.ExternalTopupCallbackSecret = ""
	operation_setting.PayAddress = ""
	operation_setting.CustomCallbackAddress = "https://new-api.example"
	operation_setting.EpayId = "10001"
	operation_setting.EpayKey = "epay-secret"
	operation_setting.PayServerInternalToken = ""
	operation_setting.Price = 1
	operation_setting.MinTopUp = 1
	operation_setting.PayMethods = []map[string]string{
		{"name": "微信", "type": "wxpay"},
		{"name": "支付宝", "type": "alipay"},
	}
	system_setting.ServerAddress = "https://new-api.example"
	*system_setting.GetFetchSetting() = originalFetchSetting

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.Option{}))

	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.ExternalTopupEnabled = originalExternalTopupEnabled
		common.ExternalTopupAuthKey = originalExternalTopupAuthKey
		common.ExternalTopupCallbackSecret = originalExternalTopupCallbackSecret
		operation_setting.PayAddress = originalPayAddress
		operation_setting.CustomCallbackAddress = originalCustomCallbackAddress
		operation_setting.EpayId = originalEpayId
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayServerInternalToken = originalPayServerInternalToken
		operation_setting.Price = originalPrice
		operation_setting.MinTopUp = originalMinTopUp
		operation_setting.PayMethods = originalPayMethods
		system_setting.ServerAddress = originalServerAddress
		*system_setting.GetFetchSetting() = originalFetchSetting
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedExternalTopupUser(t *testing.T, db *gorm.DB, id int, username string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: username,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aff-" + username,
	}).Error)
}

func callExternalTopup(t *testing.T, authCode string, payload []byte) tokenAPIResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/external_topup", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if authCode != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+authCode)
	}
	ExternalTopUp(ctx)

	var resp tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestExternalTopupCreatesPayServerOrderAndIsIdempotent(t *testing.T) {
	db := setupExternalTopupControllerTestDB(t)
	seedExternalTopupUser(t, db, 88, "pay_user")

	common.ExternalTopupEnabled = true
	common.ExternalTopupAuthKey = `["topup-secret"]`
	operation_setting.PayServerInternalToken = "svc-token"

	var callCount int
	payServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		require.Equal(t, "/api/v1/orders", r.URL.Path)
		require.Equal(t, "svc-token", r.Header.Get("x-service-token"))
		var req payServerCreateOrderRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		require.Equal(t, "new-api", req.MerchantCode)
		require.Equal(t, "88", req.UserID)
		require.Equal(t, "10.00", req.Amount)
		require.Equal(t, "wxpay", req.PayMethod)
		require.Equal(t, "https://new-api.example/api/user/epay/notify", req.NotifyURL)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"order_id":"po_test",
			"trade_no":"trade_test",
			"status":"awaiting_payment",
			"checkout_url":"https://pay.example/checkout/chk_test",
			"payment_qrcode_img":"https://pay.example/qr.png",
			"payment_qrcode_payload":"weixin://pay/test",
			"expires_at":"2026-05-06T00:00:00.000Z"
		}`))
	}))
	defer payServer.Close()
	operation_setting.PayAddress = payServer.URL

	payload := []byte(`{
		"username":"pay_user",
		"amount":10,
		"payment_method":"wxpay",
		"external_order_no":"merchant-order-1",
		"callback_url":"https://merchant.example/callback"
	}`)
	resp := callExternalTopup(t, "topup-secret", payload)
	require.True(t, resp.Success, resp.Message)
	require.Contains(t, string(resp.Data), `"checkout_url":"https://pay.example/checkout/chk_test"`)
	require.Contains(t, string(resp.Data), `"payment_qrcode_img":"https://pay.example/qr.png"`)

	var topUp model.TopUp
	require.NoError(t, db.Where("external_order_no = ?", "merchant-order-1").First(&topUp).Error)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
	require.Equal(t, "po_test", topUp.PayServerOrderId)
	require.Equal(t, "trade_test", topUp.PayServerTradeNo)
	require.Equal(t, "https://merchant.example/callback", topUp.ExternalCallbackURL)
	require.Equal(t, externalTopUpCallbackStatusPending, topUp.ExternalCallbackStatus)

	resp = callExternalTopup(t, "topup-secret", payload)
	require.True(t, resp.Success, resp.Message)
	require.Equal(t, 1, callCount)
	require.Contains(t, string(resp.Data), topUp.TradeNo)
}

func TestExternalTopupRequiresAuth(t *testing.T) {
	setupExternalTopupControllerTestDB(t)
	payload := []byte(`{"username":"pay_user","amount":10,"payment_method":"wxpay"}`)

	common.ExternalTopupEnabled = false
	common.ExternalTopupAuthKey = `["topup-secret"]`
	resp := callExternalTopup(t, "topup-secret", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "未启用")

	common.ExternalTopupEnabled = true
	common.ExternalTopupAuthKey = ""
	resp = callExternalTopup(t, "topup-secret", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "未配置")

	common.ExternalTopupAuthKey = `["topup-secret"]`
	resp = callExternalTopup(t, "wrong", payload)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "鉴权失败")
}

func TestNotifyExternalTopupSuccessSignsAndStoresCallbackResult(t *testing.T) {
	db := setupExternalTopupControllerTestDB(t)
	seedExternalTopupUser(t, db, 89, "callback_user")
	common.ExternalTopupCallbackSecret = "callback-secret"
	fetchSetting := system_setting.GetFetchSetting()
	fetchSetting.EnableSSRFProtection = false

	var receivedBody []byte
	var receivedSignature string
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-New-API-Signature")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedBody = body
		_, _ = w.Write([]byte("ok"))
	}))
	defer callbackServer.Close()

	externalOrderNo := "merchant-callback-1"
	topUp := &model.TopUp{
		UserId:                 89,
		Amount:                 10,
		Money:                  10,
		TradeNo:                "EXT-CALLBACK-1",
		PaymentMethod:          "wxpay",
		PayServerOrderId:       "po_callback",
		PayServerTradeNo:       "trade_callback",
		ExternalOrderNo:        &externalOrderNo,
		ExternalCallbackURL:    callbackServer.URL,
		ExternalCallbackStatus: externalTopUpCallbackStatusPending,
		CreateTime:             common.GetTimestamp(),
		CompleteTime:           common.GetTimestamp(),
		Status:                 common.TopUpStatusSuccess,
	}
	require.NoError(t, topUp.Insert())

	err := notifyExternalTopUpSuccess(&model.CompletedTopUp{
		TopUp:      topUp,
		QuotaToAdd: 5000000,
		Updated:    true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, receivedBody)
	require.Equal(t, common.HmacSha256(string(receivedBody), "callback-secret"), receivedSignature)
	require.Contains(t, string(receivedBody), `"external_order_no":"merchant-callback-1"`)

	var updated model.TopUp
	require.NoError(t, db.Where("trade_no = ?", "EXT-CALLBACK-1").First(&updated).Error)
	require.Equal(t, externalTopUpCallbackStatusSuccess, updated.ExternalCallbackStatus)
	require.Contains(t, updated.ExternalCallbackResponse, "status=200")
}
