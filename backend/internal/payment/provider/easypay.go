// Package provider contains concrete payment provider implementations.
package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// EasyPay constants.
const (
	easypayCodeSuccess     = 1
	easypayStatusPaid      = 1
	easypayHTTPTimeout     = 10 * time.Second
	maxEasypayResponseSize = 1 << 20 // 1MB
	maxEasypayErrorSummary = 512
	tradeStatusSuccess     = "TRADE_SUCCESS"
	signTypeMD5            = "MD5"
	paymentModePopup       = "popup"
	deviceMobile           = "mobile"
	easyPayProtocolXunhu   = "xunhupay"
	xunhuPayAPIVersion     = "1.1"
	xunhuPayStatusPaid     = "OD"
	xunhuPayStatusPending  = "WP"
	xunhuPayStatusCanceled = "CD"
)

// EasyPay implements payment.Provider for the EasyPay aggregation platform.
type EasyPay struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

type easyPayCustomMethod struct {
	Type         string `json:"type"`
	UpstreamType string `json:"upstreamType"`
	DisplayName  string `json:"displayName"`
}

// NewEasyPay creates a new EasyPay provider.
// config keys: pid, pkey, apiBase, notifyUrl, returnUrl, cid, cidAlipay, cidWxpay, currency
func NewEasyPay(instanceID string, config map[string]string) (*EasyPay, error) {
	for _, k := range []string{"pid", "pkey", "apiBase", "notifyUrl", "returnUrl"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("easypay config missing required key: %s", k)
		}
	}
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	cfg["apiBase"] = normalizeEasyPayAPIBase(cfg["apiBase"])
	if err := validateEasyPayAPIBase(cfg["apiBase"]); err != nil {
		return nil, err
	}
	if currency := strings.TrimSpace(cfg["currency"]); currency != "" {
		normalized, err := payment.NormalizePaymentCurrency(currency)
		if err != nil {
			return nil, fmt.Errorf("easypay config invalid currency: %w", err)
		}
		cfg["currency"] = normalized
	}
	return &EasyPay{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: easypayHTTPTimeout},
	}, nil
}

func normalizeEasyPayAPIBase(apiBase string) string {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		return ""
	}
	if parsed, err := url.Parse(base); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		parsed.RawPath = ""
		host := strings.ToLower(parsed.Hostname())
		switch host {
		case "www.xunhupay.com", "xunhupay.com":
			parsed.Host = "api.xunhupay.com"
			parsed.Path = ""
		case "admin.dpweixin.com", "www.dpweixin.com", "dpweixin.com":
			parsed.Host = "api.dpweixin.com"
			parsed.Path = ""
		case "api.kyrenpay.com":
			parsed.Path = trimEasyPayEndpointPath(parsed.Path)
			if parsed.Path == "" {
				parsed.Path = "/epay"
			}
			return strings.TrimRight(parsed.String(), "/")
		}
		parsed.Path = trimEasyPayEndpointPath(parsed.Path)
		return strings.TrimRight(parsed.String(), "/")
	}
	return strings.TrimRight(trimEasyPayEndpointPath(base), "/")
}

func validateEasyPayAPIBase(apiBase string) error {
	parsed, err := url.Parse(strings.TrimSpace(apiBase))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("easypay config apiBase must be an http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("easypay config apiBase must use http or https")
	}
	return nil
}

func trimEasyPayEndpointPath(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	lower := strings.ToLower(path)
	for _, endpoint := range []string{"/submit.php", "/mapi.php", "/api.php", "/payment/do.html", "/payment/query.html"} {
		if strings.HasSuffix(lower, endpoint) {
			return strings.TrimRight(path[:len(path)-len(endpoint)], "/")
		}
	}
	return path
}

func (e *EasyPay) apiBase() string {
	if e == nil {
		return ""
	}
	return normalizeEasyPayAPIBase(e.config["apiBase"])
}

func (e *EasyPay) Name() string        { return "EasyPay" }
func (e *EasyPay) ProviderKey() string { return payment.TypeEasyPay }
func (e *EasyPay) SupportedTypes() []payment.PaymentType {
	types := []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay}
	for _, method := range e.customMethods() {
		if method.Type != "" {
			types = append(types, method.Type)
		}
	}
	return types
}

func (e *EasyPay) MerchantIdentityMetadata() map[string]string {
	if e == nil {
		return nil
	}
	pid := strings.TrimSpace(e.config["pid"])
	if pid == "" {
		return nil
	}
	return map[string]string{"pid": pid}
}

func (e *EasyPay) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if e.usesXunhuPay() {
		return e.createXunhuPayPayment(ctx, req)
	}
	// Payment mode determined by instance config, not payment type.
	// "popup" → hosted page (submit.php); "qrcode"/default → API call (mapi.php).
	mode := e.config["paymentMode"]
	if mode == paymentModePopup {
		return e.createRedirectPayment(req)
	}
	return e.createAPIPayment(ctx, req)
}

// createRedirectPayment builds a submit.php URL for browser redirect.
// No server-side API call — the user is redirected to EasyPay's hosted page.
// TradeNo is empty; it arrives via the notify callback after payment.
func (e *EasyPay) createRedirectPayment(req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := e.resolveURLs(req)
	paymentType := e.upstreamPaymentType(req.PaymentType)
	params := map[string]string{
		"pid": e.config["pid"], "type": paymentType,
		"out_trade_no": req.OrderID, "notify_url": notifyURL,
		"return_url": returnURL, "name": req.Subject,
		"money": req.Amount,
	}
	if moneyType := e.moneyTypeParam(); moneyType != "" {
		params["money_type"] = moneyType
	}
	if cid := e.resolveCID(paymentType); cid != "" {
		params["cid"] = cid
	}
	if req.IsMobile {
		params["device"] = deviceMobile
	}
	params["sign"] = easyPaySign(params, e.config["pkey"])
	params["sign_type"] = signTypeMD5

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	payURL := e.apiBase() + "/submit.php?" + q.Encode()
	return &payment.CreatePaymentResponse{PayURL: payURL, Currency: e.paymentCurrency()}, nil
}

// createAPIPayment calls mapi.php to get payurl/qrcode (existing behavior).
func (e *EasyPay) createAPIPayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := e.resolveURLs(req)
	paymentType := e.upstreamPaymentType(req.PaymentType)
	params := map[string]string{
		"pid": e.config["pid"], "type": paymentType,
		"out_trade_no": req.OrderID, "notify_url": notifyURL,
		"return_url": returnURL, "name": req.Subject,
		"money": req.Amount, "clientip": req.ClientIP,
	}
	if moneyType := e.moneyTypeParam(); moneyType != "" {
		params["money_type"] = moneyType
	}
	if cid := e.resolveCID(paymentType); cid != "" {
		params["cid"] = cid
	}
	if req.IsMobile {
		params["device"] = deviceMobile
	}
	params["sign"] = easyPaySign(params, e.config["pkey"])
	params["sign_type"] = signTypeMD5

	body, err := e.post(ctx, e.apiBase()+"/mapi.php", params)
	if err != nil {
		return nil, fmt.Errorf("easypay create: %w", err)
	}
	var resp struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
		PayURL2 string `json:"payurl2"` // H5 mobile payment URL
		QRCode  string `json:"qrcode"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("easypay parse: %w", err)
	}
	if resp.Code != easypayCodeSuccess {
		return nil, fmt.Errorf("easypay error: %s", resp.Msg)
	}
	payURL := resp.PayURL
	if req.IsMobile && resp.PayURL2 != "" {
		payURL = resp.PayURL2
	}
	return &payment.CreatePaymentResponse{TradeNo: resp.TradeNo, PayURL: payURL, QRCode: resp.QRCode, Currency: e.paymentCurrency()}, nil
}

func (e *EasyPay) moneyTypeParam() string {
	if e == nil {
		return ""
	}
	for _, key := range []string{"money_type", "moneyType"} {
		if value := strings.TrimSpace(e.config[key]); value != "" {
			return value
		}
	}
	currency := e.paymentCurrency()
	if currency == "" || currency == payment.DefaultPaymentCurrency {
		return ""
	}
	return currency
}

func (e *EasyPay) paymentCurrency() string {
	if e == nil {
		return payment.DefaultPaymentCurrency
	}
	currency, err := payment.NormalizePaymentCurrency(e.config["currency"])
	if err != nil {
		return payment.DefaultPaymentCurrency
	}
	return currency
}

func (e *EasyPay) usesXunhuPay() bool {
	if e == nil {
		return false
	}
	for _, key := range []string{"protocol", "apiProtocol", "gatewayProtocol"} {
		switch strings.ToLower(strings.TrimSpace(e.config[key])) {
		case easyPayProtocolXunhu, "xunhu", "hupijiao":
			return true
		}
	}
	base := strings.ToLower(strings.TrimSpace(e.config["apiBase"]))
	return strings.Contains(base, "xunhupay.com") ||
		strings.Contains(base, "dpweixin.com") ||
		strings.Contains(base, "diypc.com.cn") ||
		strings.Contains(base, "/payment/do.html") ||
		strings.Contains(base, "/payment/query.html")
}

func (e *EasyPay) createXunhuPayPayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	notifyURL, returnURL := e.resolveURLs(req)
	params := map[string]string{
		"version":        xunhuPayAPIVersion,
		"appid":          e.config["pid"],
		"trade_order_id": req.OrderID,
		"total_fee":      req.Amount,
		"title":          req.Subject,
		"time":           strconv.FormatInt(time.Now().Unix(), 10),
		"notify_url":     notifyURL,
		"return_url":     returnURL,
		"nonce_str":      xunhuPayNonce(),
	}
	for _, key := range []string{"callback_url", "plugins", "attach", "type", "wap_url", "wap_name"} {
		if value := strings.TrimSpace(e.config[key]); value != "" {
			params[key] = value
		}
	}
	params["hash"] = xunhuPaySign(params, e.config["pkey"])

	body, status, err := e.postJSONRaw(ctx, e.xunhuPayEndpoint("/payment/do.html"), params)
	if err != nil {
		return nil, fmt.Errorf("xunhupay create: %w", err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("xunhupay create HTTP %d: %s", status, summarizeEasyPayResponse(body))
	}
	var resp struct {
		OpenID    any    `json:"openid"`
		URL       string `json:"url"`
		URLQRCode string `json:"url_qrcode"`
		ErrCode   any    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("xunhupay parse: %w", err)
	}
	if !xunhuPayErrCodeIsSuccess(resp.ErrCode) {
		msg := strings.TrimSpace(resp.ErrMsg)
		if msg == "" {
			msg = summarizeEasyPayResponse(body)
		}
		return nil, fmt.Errorf("xunhupay error: %s", msg)
	}
	payURL := strings.TrimSpace(resp.URL)
	return &payment.CreatePaymentResponse{TradeNo: xunhuPayStringValue(resp.OpenID), PayURL: payURL, QRCode: strings.TrimSpace(resp.URLQRCode), Currency: e.paymentCurrency()}, nil
}

// resolveURLs returns (notifyURL, returnURL) preferring request values,
// falling back to instance config.
func (e *EasyPay) resolveURLs(req payment.CreatePaymentRequest) (string, string) {
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = e.config["notifyUrl"]
	}
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = e.config["returnUrl"]
	}
	return notifyURL, returnURL
}

func (e *EasyPay) customMethods() []easyPayCustomMethod {
	if e == nil {
		return nil
	}
	raw := strings.TrimSpace(e.config["customMethods"])
	if raw == "" {
		return nil
	}
	var methods []easyPayCustomMethod
	if err := json.Unmarshal([]byte(raw), &methods); err != nil {
		return nil
	}
	result := make([]easyPayCustomMethod, 0, len(methods))
	for _, method := range methods {
		method.Type = strings.TrimSpace(method.Type)
		method.UpstreamType = strings.TrimSpace(method.UpstreamType)
		method.DisplayName = strings.TrimSpace(method.DisplayName)
		if method.Type == "" || method.UpstreamType == "" {
			continue
		}
		result = append(result, method)
	}
	return result
}

func (e *EasyPay) upstreamPaymentType(paymentType string) string {
	paymentType = strings.TrimSpace(paymentType)
	for _, method := range e.customMethods() {
		if paymentType == method.Type {
			return method.UpstreamType
		}
	}
	return paymentType
}

func (e *EasyPay) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	if e.usesXunhuPay() {
		return e.queryXunhuPayOrder(ctx, tradeNo)
	}
	params := map[string]string{
		"act": "order", "pid": e.config["pid"],
		"key": e.config["pkey"], "out_trade_no": tradeNo,
	}
	body, err := e.post(ctx, e.apiBase()+"/api.php", params)
	if err != nil {
		return nil, fmt.Errorf("easypay query: %w", err)
	}
	type easyPayQueryData struct {
		TradeStatus *string `json:"trade_status"`
		Status      *int    `json:"status"`
		Money       *string `json:"money"`
		TradeNo     *string `json:"trade_no"`
	}
	var resp struct {
		Code        int              `json:"code"`
		Msg         string           `json:"msg"`
		TradeStatus *string          `json:"trade_status"`
		Status      *int             `json:"status"`
		Money       *string          `json:"money"`
		TradeNo     *string          `json:"trade_no"`
		Data        easyPayQueryData `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("easypay parse query: %w", err)
	}
	status := payment.ProviderStatusPending
	if resp.TradeStatus != nil {
		if *resp.TradeStatus == tradeStatusSuccess {
			status = payment.ProviderStatusPaid
		}
	} else if resp.Data.TradeStatus != nil {
		if *resp.Data.TradeStatus == tradeStatusSuccess {
			status = payment.ProviderStatusPaid
		}
	} else if resp.Status != nil {
		if *resp.Status == easypayStatusPaid {
			status = payment.ProviderStatusPaid
		}
	} else if resp.Data.Status != nil && *resp.Data.Status == easypayStatusPaid {
		status = payment.ProviderStatusPaid
	}

	money := ""
	if resp.Money != nil {
		money = *resp.Money
	} else if resp.Data.Money != nil {
		money = *resp.Data.Money
	}
	responseTradeNo := tradeNo
	if resp.TradeNo != nil {
		if *resp.TradeNo != "" {
			responseTradeNo = *resp.TradeNo
		}
	} else if resp.Data.TradeNo != nil && *resp.Data.TradeNo != "" {
		responseTradeNo = *resp.Data.TradeNo
	}

	amount, _ := strconv.ParseFloat(money, 64)
	return &payment.QueryOrderResponse{
		TradeNo:  responseTradeNo,
		Status:   status,
		Amount:   amount,
		Metadata: e.MerchantIdentityMetadata(),
	}, nil
}

func (e *EasyPay) queryXunhuPayOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	params := map[string]string{
		"appid":           e.config["pid"],
		"out_trade_order": tradeNo,
		"time":            strconv.FormatInt(time.Now().Unix(), 10),
		"nonce_str":       xunhuPayNonce(),
	}
	params["hash"] = xunhuPaySign(params, e.config["pkey"])

	body, status, err := e.postJSONRaw(ctx, e.xunhuPayEndpoint("/payment/query.html"), params)
	if err != nil {
		return nil, fmt.Errorf("xunhupay query: %w", err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("xunhupay query HTTP %d: %s", status, summarizeEasyPayResponse(body))
	}
	var resp struct {
		ErrCode any    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Data    struct {
			Status        string `json:"status"`
			OpenOrderID   string `json:"open_order_id"`
			TradeOrderID  string `json:"trade_order_id"`
			TransactionID string `json:"transaction_id"`
			TotalFee      string `json:"total_fee"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("xunhupay parse query: %w", err)
	}
	statusValue := payment.ProviderStatusPending
	if xunhuPayErrCodeIsSuccess(resp.ErrCode) {
		statusValue = xunhuPayProviderStatus(resp.Data.Status)
	}
	responseTradeNo := firstNonEmpty(resp.Data.TransactionID, resp.Data.OpenOrderID, resp.Data.TradeOrderID, tradeNo)
	amount, _ := strconv.ParseFloat(resp.Data.TotalFee, 64)
	return &payment.QueryOrderResponse{
		TradeNo:  responseTradeNo,
		Status:   statusValue,
		Amount:   amount,
		Metadata: e.MerchantIdentityMetadata(),
	}, nil
}

func (e *EasyPay) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	if e.usesXunhuPay() {
		return e.verifyXunhuPayNotification(rawBody)
	}
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("parse notify: %w", err)
	}
	// url.ParseQuery already decodes values — no additional decode needed.
	params := make(map[string]string)
	for k := range values {
		params[k] = values.Get(k)
	}
	sign := params["sign"]
	if sign == "" {
		return nil, fmt.Errorf("missing sign")
	}
	if !easyPayVerifySign(params, e.config["pkey"], sign) {
		return nil, fmt.Errorf("invalid signature")
	}
	status := payment.ProviderStatusFailed
	if params["trade_status"] == tradeStatusSuccess {
		status = payment.ProviderStatusSuccess
	}
	amount, _ := strconv.ParseFloat(params["money"], 64)

	metadata := e.MerchantIdentityMetadata()
	if pid := strings.TrimSpace(params["pid"]); pid != "" {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata["pid"] = pid
	}
	return &payment.PaymentNotification{
		TradeNo: params["trade_no"], OrderID: params["out_trade_no"],
		Amount: amount, Status: status, RawData: rawBody, Metadata: metadata,
	}, nil
}

func (e *EasyPay) verifyXunhuPayNotification(rawBody string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("parse xunhupay notify: %w", err)
	}
	params := make(map[string]string)
	for k := range values {
		params[k] = values.Get(k)
	}
	hash := params["hash"]
	if hash == "" {
		return nil, fmt.Errorf("missing hash")
	}
	if !xunhuPayVerifySign(params, e.config["pkey"], hash) {
		return nil, fmt.Errorf("invalid signature")
	}
	status := payment.ProviderStatusFailed
	if strings.EqualFold(params["status"], xunhuPayStatusPaid) {
		status = payment.ProviderStatusSuccess
	}
	amount, _ := strconv.ParseFloat(params["total_fee"], 64)

	metadata := e.MerchantIdentityMetadata()
	if appID := strings.TrimSpace(params["appid"]); appID != "" {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata["pid"] = appID
	}
	return &payment.PaymentNotification{
		TradeNo:  firstNonEmpty(params["transaction_id"], params["open_order_id"]),
		OrderID:  params["trade_order_id"],
		Amount:   amount,
		Status:   status,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

func (e *EasyPay) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	attempts := e.refundAttempts(req)
	if len(attempts) == 0 {
		return nil, fmt.Errorf("easypay refund missing order identifier")
	}
	var firstErr error
	for i, attempt := range attempts {
		body, status, err := e.postRaw(ctx, e.apiBase()+"/api.php?act=refund", attempt.params)
		if err != nil {
			return nil, fmt.Errorf("easypay refund request: %w", err)
		}
		if err := parseEasyPayRefundResponse(status, body); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if i+1 < len(attempts) && isEasyPayRefundOrderNotFound(err) {
				continue
			}
			return nil, err
		}
		return &payment.RefundResponse{RefundID: attempt.refundID, Status: payment.ProviderStatusSuccess}, nil
	}
	return nil, firstErr
}

type easyPayRefundAttempt struct {
	params   map[string]string
	refundID string
}

func (e *EasyPay) refundAttempts(req payment.RefundRequest) []easyPayRefundAttempt {
	base := map[string]string{
		"pid": e.config["pid"], "key": e.config["pkey"], "money": req.Amount,
	}
	var attempts []easyPayRefundAttempt
	if tradeNo := strings.TrimSpace(req.TradeNo); tradeNo != "" {
		params := cloneStringMap(base)
		params["trade_no"] = tradeNo
		attempts = append(attempts, easyPayRefundAttempt{params: params, refundID: tradeNo})
	}
	if orderID := strings.TrimSpace(req.OrderID); orderID != "" {
		params := cloneStringMap(base)
		params["out_trade_no"] = orderID
		attempts = append(attempts, easyPayRefundAttempt{params: params, refundID: orderID})
	}
	return attempts
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isEasyPayRefundOrderNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	return strings.Contains(msg, "订单编号不存在") ||
		strings.Contains(msg, "订单不存在") ||
		strings.Contains(lower, "order not found") ||
		strings.Contains(lower, "not exist")
}

func parseEasyPayRefundResponse(status int, body []byte) error {
	summary := summarizeEasyPayResponse(body)
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("easypay refund HTTP %d: %s", status, summary)
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return fmt.Errorf("easypay refund empty response (HTTP %d): %s", status, summary)
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") ||
		(strings.HasPrefix(lower, "<") && strings.Contains(lower, "html")) {
		return fmt.Errorf("easypay refund non-JSON response (HTTP %d): %s", status, summary)
	}

	var resp struct {
		Code any    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("easypay refund non-JSON response (HTTP %d): %s", status, summary)
	}
	if !easyPayResponseCodeIsSuccess(resp.Code) {
		msg := strings.TrimSpace(resp.Msg)
		if msg == "" {
			msg = summary
		}
		return fmt.Errorf("easypay refund failed (HTTP %d): %s", status, msg)
	}
	return nil
}

func easyPayResponseCodeIsSuccess(code any) bool {
	switch v := code.(type) {
	case float64:
		return int(v) == easypayCodeSuccess
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return err == nil && n == easypayCodeSuccess
	default:
		return false
	}
}

func summarizeEasyPayResponse(body []byte) string {
	summary := strings.Join(strings.Fields(string(body)), " ")
	if summary == "" {
		return "<empty>"
	}
	if len(summary) > maxEasypayErrorSummary {
		return summary[:maxEasypayErrorSummary] + "..."
	}
	return summary
}

func (e *EasyPay) resolveCID(paymentType string) string {
	if strings.HasPrefix(paymentType, "alipay") {
		if v := e.config["cidAlipay"]; v != "" {
			return v
		}
		return e.config["cid"]
	}
	if v := e.config["cidWxpay"]; v != "" {
		return v
	}
	return e.config["cid"]
}

func (e *EasyPay) post(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	body, _, err := e.postRaw(ctx, endpoint, params)
	return body, err
}

func (e *EasyPay) postRaw(ctx context.Context, endpoint string, params map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := e.httpClient
	if client == nil {
		client = &http.Client{Timeout: easypayHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEasypayResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (e *EasyPay) postJSONRaw(ctx context.Context, endpoint string, params map[string]string) ([]byte, int, error) {
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := e.httpClient
	if client == nil {
		client = &http.Client{Timeout: easypayHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEasypayResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (e *EasyPay) xunhuPayEndpoint(path string) string {
	base := e.apiBase()
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + path
}

func easyPaySign(params map[string]string, pkey string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			_ = buf.WriteByte('&')
		}
		_, _ = buf.WriteString(k + "=" + params[k])
	}
	_, _ = buf.WriteString(pkey)
	hash := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

func easyPayVerifySign(params map[string]string, pkey string, sign string) bool {
	return hmac.Equal([]byte(easyPaySign(params, pkey)), []byte(sign))
}

func xunhuPaySign(params map[string]string, pkey string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "hash" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			_ = buf.WriteByte('&')
		}
		_, _ = buf.WriteString(k + "=" + params[k])
	}
	_, _ = buf.WriteString(pkey)
	hash := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

func xunhuPayVerifySign(params map[string]string, pkey string, hash string) bool {
	return hmac.Equal([]byte(xunhuPaySign(params, pkey)), []byte(hash))
}

func xunhuPayStringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func xunhuPayErrCodeIsSuccess(code any) bool {
	switch v := code.(type) {
	case float64:
		return int(v) == 0
	case int:
		return v == 0
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return err == nil && n == 0
	default:
		return false
	}
}

func xunhuPayProviderStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case xunhuPayStatusPaid:
		return payment.ProviderStatusPaid
	case xunhuPayStatusPending:
		return payment.ProviderStatusPending
	case xunhuPayStatusCanceled:
		return payment.ProviderStatusFailed
	default:
		return payment.ProviderStatusPending
	}
}

func xunhuPayNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
