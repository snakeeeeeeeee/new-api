package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStripeTopUpModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &Log{}))

	t.Cleanup(func() {
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		DB = originalDB
		LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedStripeTopUpModelUser(t *testing.T, db *gorm.DB, userID int) {
	t.Helper()
	user := &User{
		Id:       userID,
		Username: fmt.Sprintf("model_user_%d", userID),
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    0,
	}
	require.NoError(t, db.Create(user).Error)
}

func seedStripeTopUpModelOrder(t *testing.T, db *gorm.DB, userID int, tradeNo string, paymentMethod string, amount int64, money float64) {
	t.Helper()
	topUp := &TopUp{
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

func TestRechargeStripeOnlyCompletesStripeOrders(t *testing.T) {
	db := setupStripeTopUpModelTestDB(t)
	seedStripeTopUpModelUser(t, db, 1)
	seedStripeTopUpModelOrder(t, db, 1, "ref_model_stripe", stripeTopUpPaymentMethod, 100, 2.5)

	require.NoError(t, RechargeStripe("ref_model_stripe", "cus_model"))

	topUp := GetTopUpByTradeNo("ref_model_stripe")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)

	user, err := GetUserById(1, false)
	require.NoError(t, err)
	require.Equal(t, int(2.5*common.QuotaPerUnit), user.Quota)
	require.Equal(t, "cus_model", user.StripeCustomer)
}

func TestRechargeStripeRejectsNonStripeOrders(t *testing.T) {
	db := setupStripeTopUpModelTestDB(t)
	seedStripeTopUpModelUser(t, db, 2)
	seedStripeTopUpModelOrder(t, db, 2, "USR2NOATTACK", "微信", 500, 250)

	err := RechargeStripe("USR2NOATTACK", "cus_attacker")
	require.ErrorIs(t, err, ErrTopUpPaymentMethodMismatch)

	topUp := GetTopUpByTradeNo("USR2NOATTACK")
	require.NotNil(t, topUp)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
	require.Zero(t, topUp.CompleteTime)

	user, err := GetUserById(2, false)
	require.NoError(t, err)
	require.Equal(t, 0, user.Quota)
	require.Empty(t, user.StripeCustomer)
}

func TestStripeTopUpStateTransitionsRejectNonStripeOrders(t *testing.T) {
	db := setupStripeTopUpModelTestDB(t)
	seedStripeTopUpModelUser(t, db, 3)
	seedStripeTopUpModelOrder(t, db, 3, "USR3NOEXPIRE", "微信", 500, 250)
	seedStripeTopUpModelOrder(t, db, 3, "USR3NOFAIL", "微信", 500, 250)

	require.ErrorIs(t, ExpireStripeTopUp("USR3NOEXPIRE"), ErrTopUpPaymentMethodMismatch)
	require.ErrorIs(t, FailStripeTopUp("USR3NOFAIL"), ErrTopUpPaymentMethodMismatch)

	expireTopUp := GetTopUpByTradeNo("USR3NOEXPIRE")
	require.NotNil(t, expireTopUp)
	require.Equal(t, common.TopUpStatusPending, expireTopUp.Status)

	failTopUp := GetTopUpByTradeNo("USR3NOFAIL")
	require.NotNil(t, failTopUp)
	require.Equal(t, common.TopUpStatusPending, failTopUp.Status)
}
