package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestXunhuPayCreatePaymentUsesPaymentDoEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotContentType string
	var gotPayload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{
			"openid":     "xh-order-123",
			"url":        "https://api.xunhupay.com/alipay/pay/index.html?id=1",
			"url_qrcode": "weixin://wxpay/bizpayurl?pr=test",
			"errcode":    "0",
			"errmsg":     "success!",
		}
		resp["hash"] = xunhuPaySign(resp, "pkey-1")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := newTestXunhuPay(t, server.URL)
	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2_xh_123",
		Amount:      "9.90",
		PaymentType: payment.TypeWxpay,
		Subject:     "Jisudeng Recharge",
		NotifyURL:   "https://merchant.example.com/api/v1/payment/webhook/easypay",
		ReturnURL:   "https://merchant.example.com/payment/result",
	})
	if err != nil {
		t.Fatalf("CreatePayment returned error: %v", err)
	}
	if gotPath != "/payment/do.html" {
		t.Fatalf("path = %q, want /payment/do.html", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q, want application/json", gotContentType)
	}
	for key, want := range map[string]string{
		"version":        xunhuPayAPIVersion,
		"appid":          "pid-1",
		"trade_order_id": "sub2_xh_123",
		"total_fee":      "9.90",
		"title":          "Jisudeng Recharge",
		"notify_url":     "https://merchant.example.com/api/v1/payment/webhook/easypay",
		"return_url":     "https://merchant.example.com/payment/result",
	} {
		if got := gotPayload[key]; got != want {
			t.Fatalf("payload[%s] = %q, want %q (payload=%v)", key, got, want, gotPayload)
		}
	}
	if gotPayload["hash"] == "" {
		t.Fatalf("payload hash is empty: %v", gotPayload)
	}
	if !xunhuPayVerifySign(gotPayload, "pkey-1", gotPayload["hash"]) {
		t.Fatalf("payload hash is invalid: %v", gotPayload)
	}
	if resp.TradeNo != "xh-order-123" {
		t.Fatalf("trade no = %q, want xh-order-123", resp.TradeNo)
	}
	if resp.QRCode != "weixin://wxpay/bizpayurl?pr=test" {
		t.Fatalf("qrcode = %q", resp.QRCode)
	}
	if resp.PayURL == "" {
		t.Fatal("pay url is empty")
	}
}

func TestXunhuPayVerifyNotificationUsesTradeOrderID(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"trade_order_id": "sub2_xh_notify",
		"total_fee":      "12.34",
		"transaction_id": "420000-xh",
		"open_order_id":  "xh-open-1",
		"status":         xunhuPayStatusPaid,
		"appid":          "pid-1",
		"time":           "1784380000",
		"nonce_str":      "nonce-1",
	}
	params["hash"] = xunhuPaySign(params, "pkey-1")
	raw := url.Values{}
	for k, v := range params {
		raw.Set(k, v)
	}

	provider := newTestXunhuPay(t, "https://api.xunhupay.com")
	n, err := provider.VerifyNotification(context.Background(), raw.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification returned error: %v", err)
	}
	if n.OrderID != "sub2_xh_notify" {
		t.Fatalf("order id = %q, want sub2_xh_notify", n.OrderID)
	}
	if n.TradeNo != "420000-xh" {
		t.Fatalf("trade no = %q, want 420000-xh", n.TradeNo)
	}
	if n.Amount != 12.34 {
		t.Fatalf("amount = %v, want 12.34", n.Amount)
	}
	if n.Status != payment.ProviderStatusSuccess {
		t.Fatalf("status = %q, want success", n.Status)
	}
}

func TestXunhuPayQueryOrderUsesPaymentQueryEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotPayload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"errcode": 0,
			"errmsg":  "success!",
			"data": map[string]string{
				"status":         xunhuPayStatusPaid,
				"open_order_id":  "xh-open-1",
				"transaction_id": "420000-query",
				"total_fee":      "7.89",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := newTestXunhuPay(t, server.URL)
	resp, err := provider.QueryOrder(context.Background(), "sub2_xh_query")
	if err != nil {
		t.Fatalf("QueryOrder returned error: %v", err)
	}
	if gotPath != "/payment/query.html" {
		t.Fatalf("path = %q, want /payment/query.html", gotPath)
	}
	if gotPayload["appid"] != "pid-1" || gotPayload["out_trade_order"] != "sub2_xh_query" {
		t.Fatalf("query payload = %v", gotPayload)
	}
	if resp.Status != payment.ProviderStatusPaid {
		t.Fatalf("status = %q, want paid", resp.Status)
	}
	if resp.TradeNo != "420000-query" {
		t.Fatalf("trade no = %q, want 420000-query", resp.TradeNo)
	}
	if resp.Amount != 7.89 {
		t.Fatalf("amount = %v, want 7.89", resp.Amount)
	}
}

func newTestXunhuPay(t *testing.T, apiBase string) *EasyPay {
	t.Helper()

	provider, err := NewEasyPay("test-xunhupay", map[string]string{
		"pid":       "pid-1",
		"pkey":      "pkey-1",
		"apiBase":   apiBase,
		"notifyUrl": "https://example.com/notify",
		"returnUrl": "https://example.com/return",
		"protocol":  easyPayProtocolXunhu,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}
	return provider
}
