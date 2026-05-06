package controller

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	externalTopUpCallbackStatusPending = "pending"
	externalTopUpCallbackStatusSuccess = "success"
	externalTopUpCallbackStatusFailed  = "failed"
)

type externalTopUpRequest struct {
	UserID          int64  `json:"user_id"`
	Username        string `json:"username"`
	Amount          int64  `json:"amount"`
	PaymentMethod   string `json:"payment_method"`
	ExternalOrderNo string `json:"external_order_no"`
	CallbackURL     string `json:"callback_url"`
	ReturnURL       string `json:"return_url"`
	Subject         string `json:"subject"`
}

type externalTopUpPaymentData struct {
	OrderID              string            `json:"order_id,omitempty"`
	PayServerTradeNo     string            `json:"pay_server_trade_no,omitempty"`
	Status               string            `json:"status,omitempty"`
	CheckoutURL          string            `json:"checkout_url,omitempty"`
	PaymentURL           string            `json:"payment_url,omitempty"`
	PaymentURL2          string            `json:"payment_url2,omitempty"`
	PaymentQRCodeURL     string            `json:"payment_qrcode_url,omitempty"`
	PaymentQRCodeImage   string            `json:"payment_qrcode_img,omitempty"`
	PaymentQRCodePayload string            `json:"payment_qrcode_payload,omitempty"`
	PaymentFormURL       string            `json:"payment_form_url,omitempty"`
	PaymentFormFields    map[string]string `json:"payment_form_fields,omitempty"`
	WalletAddress        string            `json:"wallet_address,omitempty"`
	QRCodePayload        string            `json:"qr_code_payload,omitempty"`
	Chain                string            `json:"chain,omitempty"`
	TokenSymbol          string            `json:"token_symbol,omitempty"`
	ExpiresAt            string            `json:"expires_at,omitempty"`
}

type payServerCreateOrderRequest struct {
	MerchantCode    string `json:"merchantCode"`
	MerchantOrderNo string `json:"merchantOrderNo"`
	UserID          string `json:"userId,omitempty"`
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	PayMethod       string `json:"payMethod"`
	Subject         string `json:"subject"`
	NotifyURL       string `json:"notifyUrl"`
	ReturnURL       string `json:"returnUrl"`
}

type payServerCreateOrderResponse struct {
	OrderID              string            `json:"order_id"`
	TradeNo              string            `json:"trade_no"`
	Status               string            `json:"status"`
	CheckoutURL          string            `json:"checkout_url"`
	PaymentURL           string            `json:"payment_url"`
	PaymentURL2          string            `json:"payment_url2"`
	PaymentQRCodeURL     string            `json:"payment_qrcode_url"`
	PaymentQRCodeImage   string            `json:"payment_qrcode_img"`
	PaymentQRCodePayload string            `json:"payment_qrcode_payload"`
	PaymentFormURL       string            `json:"payment_form_url"`
	PaymentFormFields    map[string]string `json:"payment_form_fields"`
	WalletAddress        string            `json:"wallet_address"`
	QRCodePayload        string            `json:"qr_code_payload"`
	Chain                string            `json:"chain"`
	TokenSymbol          string            `json:"token_symbol"`
	ExpiresAt            string            `json:"expires_at"`
}

func externalTopUpAuthKeys() []string {
	raw := strings.TrimSpace(common.ExternalTopupAuthKey)
	if raw == "" {
		return nil
	}
	var authKeys []string
	if err := common.UnmarshalJsonStr(raw, &authKeys); err != nil {
		authKeys = []string{raw}
	}
	normalized := make([]string, 0, len(authKeys))
	for _, authKey := range authKeys {
		authKey = strings.TrimSpace(authKey)
		if authKey != "" {
			normalized = append(normalized, authKey)
		}
	}
	return normalized
}

func externalTopUpAuthKeyMatches(requestAuthKey string, authKeys []string) bool {
	for _, authKey := range authKeys {
		if subtle.ConstantTimeCompare([]byte(requestAuthKey), []byte(authKey)) == 1 {
			return true
		}
	}
	return false
}

func validateExternalTopUpAuth(c *gin.Context) bool {
	if !common.ExternalTopupEnabled {
		common.ApiErrorMsg(c, "外部充值未启用")
		return false
	}
	authKeys := externalTopUpAuthKeys()
	if len(authKeys) == 0 {
		common.ApiErrorMsg(c, "外部充值鉴权码未配置")
		return false
	}
	if !externalTopUpAuthKeyMatches(getBearerToken(c), authKeys) {
		common.ApiErrorMsg(c, "外部充值鉴权失败")
		return false
	}
	return true
}

func resolveExternalTopUpUser(req externalTopUpRequest) (*model.User, error) {
	if req.UserID > 0 {
		return model.GetUserById(int(req.UserID), false)
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("user_id or username is required")
	}
	return model.GetUserByUsername(username, false)
}

func normalizeExternalTopUpRequest(req *externalTopUpRequest) {
	req.Username = strings.TrimSpace(req.Username)
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	req.ExternalOrderNo = strings.TrimSpace(req.ExternalOrderNo)
	req.CallbackURL = strings.TrimSpace(req.CallbackURL)
	req.ReturnURL = strings.TrimSpace(req.ReturnURL)
	req.Subject = strings.TrimSpace(req.Subject)
	if req.PaymentMethod == "" {
		req.PaymentMethod = "wxpay"
	}
}

func generateExternalTopUpTradeNo(userID int, externalOrderNo string) string {
	if externalOrderNo != "" {
		return "EXT" + common.Sha1([]byte(externalOrderNo))[:32]
	}
	base := fmt.Sprintf("%d:%d:%s", userID, time.Now().UnixNano(), common.GetRandomString(8))
	return "EXT" + common.Sha1([]byte(base))[:32]
}

func externalTopUpCurrency(paymentMethod string) string {
	if strings.HasPrefix(paymentMethod, "usdt") {
		return "CNY"
	}
	return "CNY"
}

func externalTopUpSubject(req externalTopUpRequest) string {
	if req.Subject != "" {
		return req.Subject
	}
	return fmt.Sprintf("TUC%d", req.Amount)
}

func externalTopUpReturnURL(req externalTopUpRequest) string {
	if req.ReturnURL != "" {
		return req.ReturnURL
	}
	return system_setting.ServerAddress + "/console/topup?show_history=true"
}

func buildExternalTopUpPaymentData(resp payServerCreateOrderResponse) externalTopUpPaymentData {
	return externalTopUpPaymentData{
		OrderID:              resp.OrderID,
		PayServerTradeNo:     resp.TradeNo,
		Status:               resp.Status,
		CheckoutURL:          resp.CheckoutURL,
		PaymentURL:           resp.PaymentURL,
		PaymentURL2:          resp.PaymentURL2,
		PaymentQRCodeURL:     resp.PaymentQRCodeURL,
		PaymentQRCodeImage:   resp.PaymentQRCodeImage,
		PaymentQRCodePayload: resp.PaymentQRCodePayload,
		PaymentFormURL:       resp.PaymentFormURL,
		PaymentFormFields:    resp.PaymentFormFields,
		WalletAddress:        resp.WalletAddress,
		QRCodePayload:        resp.QRCodePayload,
		Chain:                resp.Chain,
		TokenSymbol:          resp.TokenSymbol,
		ExpiresAt:            resp.ExpiresAt,
	}
}

func encodeExternalTopUpPaymentData(data externalTopUpPaymentData) string {
	raw, err := common.Marshal(data)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeExternalTopUpPaymentData(raw string) externalTopUpPaymentData {
	var data externalTopUpPaymentData
	if strings.TrimSpace(raw) == "" {
		return data
	}
	_ = common.UnmarshalJsonStr(raw, &data)
	return data
}

func externalTopUpResponse(topUp *model.TopUp) gin.H {
	paymentData := decodeExternalTopUpPaymentData(topUp.ExternalPaymentData)
	paymentFormFields := paymentData.PaymentFormFields
	if paymentFormFields == nil {
		paymentFormFields = map[string]string{}
	}
	return gin.H{
		"trade_no":                 topUp.TradeNo,
		"external_order_no":        topUp.GetExternalOrderNo(),
		"user_id":                  topUp.UserId,
		"amount":                   topUp.Amount,
		"money":                    topUp.Money,
		"payment_method":           topUp.PaymentMethod,
		"status":                   topUp.Status,
		"pay_server_order_id":      topUp.PayServerOrderId,
		"pay_server_trade_no":      topUp.PayServerTradeNo,
		"checkout_url":             paymentData.CheckoutURL,
		"payment_url":              paymentData.PaymentURL,
		"payment_url2":             paymentData.PaymentURL2,
		"payment_qrcode_url":       paymentData.PaymentQRCodeURL,
		"payment_qrcode_img":       paymentData.PaymentQRCodeImage,
		"payment_qrcode_payload":   paymentData.PaymentQRCodePayload,
		"payment_form_url":         paymentData.PaymentFormURL,
		"payment_form_fields":      paymentFormFields,
		"wallet_address":           paymentData.WalletAddress,
		"qr_code_payload":          paymentData.QRCodePayload,
		"chain":                    paymentData.Chain,
		"token_symbol":             paymentData.TokenSymbol,
		"expires_at":               paymentData.ExpiresAt,
		"external_callback_status": topUp.ExternalCallbackStatus,
	}
}

func createPayServerInternalOrder(c *gin.Context, topUp *model.TopUp, req externalTopUpRequest) (payServerCreateOrderResponse, error) {
	var result payServerCreateOrderResponse
	if operation_setting.PayAddress == "" {
		return result, errors.New("支付地址未配置")
	}

	baseURL, err := url.Parse(operation_setting.PayAddress)
	if err != nil {
		return result, err
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/api/v1/orders"

	callbackAddress := strings.TrimRight(service.GetCallbackAddress(), "/")
	if callbackAddress == "" {
		return result, errors.New("回调地址未配置")
	}

	payload := payServerCreateOrderRequest{
		MerchantCode:    "new-api",
		MerchantOrderNo: topUp.TradeNo,
		UserID:          strconv.Itoa(topUp.UserId),
		Amount:          strconv.FormatFloat(topUp.Money, 'f', 2, 64),
		Currency:        externalTopUpCurrency(topUp.PaymentMethod),
		PayMethod:       topUp.PaymentMethod,
		Subject:         externalTopUpSubject(req),
		NotifyURL:       callbackAddress + "/api/user/epay/notify",
		ReturnURL:       externalTopUpReturnURL(req),
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return result, err
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL.String(), bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if operation_setting.PayServerInternalToken != "" {
		httpReq.Header.Set("x-service-token", operation_setting.PayServerInternalToken)
	}
	if clientIP := c.ClientIP(); clientIP != "" {
		httpReq.Header.Set("X-Forwarded-For", clientIP)
	}

	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("pay-server 创建订单失败: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	if err := common.Unmarshal(respBody, &result); err != nil {
		return result, err
	}
	if result.CheckoutURL == "" && result.PaymentURL == "" && result.PaymentQRCodeImage == "" && result.PaymentQRCodePayload == "" && result.QRCodePayload == "" && result.WalletAddress == "" {
		return result, errors.New("pay-server 未返回可用支付信息")
	}
	return result, nil
}

func createExternalTopUpPayment(c *gin.Context, topUp *model.TopUp, req externalTopUpRequest) (externalTopUpPaymentData, error) {
	order, err := createPayServerInternalOrder(c, topUp, req)
	if err == nil {
		paymentData := buildExternalTopUpPaymentData(order)
		topUp.PayServerOrderId = order.OrderID
		topUp.PayServerTradeNo = order.TradeNo
		return paymentData, nil
	}
	log.Printf("pay-server 内部下单失败，回退易支付兼容层: %v", err)
	paymentData, _, _, fallbackErr := createEpayCheckoutPaymentData(req, topUp)
	if fallbackErr != nil {
		return externalTopUpPaymentData{}, fmt.Errorf("pay-server 内部下单失败: %v; 易支付兼容层下单失败: %v", err, fallbackErr)
	}
	return paymentData, nil
}

func ExternalTopUp(c *gin.Context) {
	if !validateExternalTopUpAuth(c) {
		return
	}

	var req externalTopUpRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	normalizeExternalTopUpRequest(&req)
	if req.Amount < getMinTopup() {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", getMinTopup()))
		return
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}
	if req.CallbackURL != "" {
		if _, err := url.ParseRequestURI(req.CallbackURL); err != nil {
			common.ApiErrorMsg(c, "callback_url 格式错误")
			return
		}
	}

	if req.ExternalOrderNo != "" {
		if existing := model.GetTopUpByExternalOrderNo(req.ExternalOrderNo); existing != nil {
			common.ApiSuccess(c, externalTopUpResponse(existing))
			return
		}
	}

	user, err := resolveExternalTopUpUser(req)
	if err != nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	group, err := model.GetUserGroup(user.Id, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		common.ApiErrorMsg(c, "充值金额过低")
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
		if amount < 1 {
			amount = 1
		}
	}

	callbackStatus := ""
	if req.CallbackURL != "" {
		callbackStatus = externalTopUpCallbackStatusPending
	}
	var externalOrderNo *string
	if req.ExternalOrderNo != "" {
		externalOrderNo = &req.ExternalOrderNo
	}
	topUp := &model.TopUp{
		UserId:                   user.Id,
		Amount:                   amount,
		Money:                    payMoney,
		TradeNo:                  generateExternalTopUpTradeNo(user.Id, req.ExternalOrderNo),
		PaymentMethod:            req.PaymentMethod,
		ExternalOrderNo:          externalOrderNo,
		ExternalCallbackURL:      req.CallbackURL,
		ExternalCallbackStatus:   callbackStatus,
		ExternalCallbackResponse: "",
		CreateTime:               time.Now().Unix(),
		Status:                   common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	paymentData, err := createExternalTopUpPayment(c, topUp, req)
	if err != nil {
		topUp.Status = common.TopUpStatusFailed
		topUp.ExternalCallbackResponse = err.Error()
		_ = topUp.Update()
		common.ApiError(c, err)
		return
	}
	topUp.ExternalPaymentData = encodeExternalTopUpPaymentData(paymentData)
	if err := topUp.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, externalTopUpResponse(topUp))
}

func notifyExternalTopUpSuccessAsync(completed *model.CompletedTopUp) {
	if completed == nil || completed.TopUp == nil || !completed.Updated || strings.TrimSpace(completed.TopUp.ExternalCallbackURL) == "" {
		return
	}
	go func() {
		if err := notifyExternalTopUpSuccess(completed); err != nil {
			log.Printf("外部充值成功回调失败 tradeNo=%s err=%v", completed.TopUp.TradeNo, err)
		}
	}()
}

func notifyExternalTopUpSuccess(completed *model.CompletedTopUp) error {
	topUp := completed.TopUp
	callbackURL := strings.TrimSpace(topUp.ExternalCallbackURL)
	if callbackURL == "" {
		return nil
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(callbackURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		_ = model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusFailed, err.Error())
		return err
	}

	payload := map[string]any{
		"trade_no":            topUp.TradeNo,
		"external_order_no":   topUp.GetExternalOrderNo(),
		"user_id":             topUp.UserId,
		"amount":              topUp.Amount,
		"money":               strconv.FormatFloat(topUp.Money, 'f', 2, 64),
		"quota":               completed.QuotaToAdd,
		"payment_method":      topUp.PaymentMethod,
		"status":              common.TopUpStatusSuccess,
		"pay_server_order_id": topUp.PayServerOrderId,
		"pay_server_trade_no": topUp.PayServerTradeNo,
		"complete_time":       topUp.CompleteTime,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		_ = model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusFailed, err.Error())
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		_ = model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusFailed, err.Error())
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-New-API-Event", "topup.success")
	httpReq.Header.Set("X-New-API-Trade-No", topUp.TradeNo)
	if common.ExternalTopupCallbackSecret != "" {
		httpReq.Header.Set("X-New-API-Signature", common.HmacSha256(string(body), common.ExternalTopupCallbackSecret))
	}

	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		_ = model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusFailed, err.Error())
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	responseText := fmt.Sprintf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusFailed, responseText)
		return fmt.Errorf("callback failed: %s", responseText)
	}
	return model.UpdateExternalCallbackResult(topUp.TradeNo, externalTopUpCallbackStatusSuccess, responseText)
}

func createEpayCheckoutPaymentData(req externalTopUpRequest, topUp *model.TopUp) (externalTopUpPaymentData, map[string]string, string, error) {
	callbackAddress := strings.TrimRight(service.GetCallbackAddress(), "/")
	if callbackAddress == "" {
		return externalTopUpPaymentData{}, nil, "", errors.New("回调地址未配置")
	}
	returnURL := externalTopUpReturnURL(req)
	notifyURL, err := url.Parse(callbackAddress + "/api/user/epay/notify")
	if err != nil {
		return externalTopUpPaymentData{}, nil, "", err
	}
	parsedReturnURL, err := url.Parse(returnURL)
	if err != nil {
		return externalTopUpPaymentData{}, nil, "", err
	}
	client := GetEpayClient()
	if client == nil {
		return externalTopUpPaymentData{}, nil, "", errors.New("当前管理员未配置支付信息")
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           topUp.PaymentMethod,
		ServiceTradeNo: topUp.TradeNo,
		Name:           externalTopUpSubject(req),
		Money:          strconv.FormatFloat(topUp.Money, 'f', 2, 64),
		Device:         epay.PC,
		NotifyUrl:      notifyURL,
		ReturnUrl:      parsedReturnURL,
	})
	if err != nil {
		return externalTopUpPaymentData{}, nil, "", err
	}
	return externalTopUpPaymentData{
		PaymentFormURL:    uri,
		PaymentFormFields: params,
	}, params, uri, nil
}
