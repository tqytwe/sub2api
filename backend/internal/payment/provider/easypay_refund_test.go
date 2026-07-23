package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestNormalizeEasyPayAPIBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "https://zpayz.cn", want: "https://zpayz.cn"},
		{input: "https://zpayz.cn/", want: "https://zpayz.cn"},
		{input: "https://zpayz.cn/mapi.php", want: "https://zpayz.cn"},
		{input: "https://zpayz.cn/submit.php", want: "https://zpayz.cn"},
		{input: "https://zpayz.cn/api.php", want: "https://zpayz.cn"},
		{input: "https://zpayz.cn/api.php?act=refund", want: "https://zpayz.cn"},
		{input: "https://api.kyrenpay.com", want: "https://api.kyrenpay.com/epay"},
		{input: "https://api.kyrenpay.com/epay", want: "https://api.kyrenpay.com/epay"},
		{input: "https://api.kyrenpay.com/epay/mapi.php", want: "https://api.kyrenpay.com/epay"},
		{input: "https://www.xunhupay.com/doc/api/pay.html", want: "https://api.xunhupay.com"},
		{input: "https://xunhupay.com/doc/api/pay.html", want: "https://api.xunhupay.com"},
		{input: "https://admin.dpweixin.com", want: "https://api.dpweixin.com"},
		{input: "https://api.xunhupay.com/payment/do.html", want: "https://api.xunhupay.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeEasyPayAPIBase(tt.input); got != tt.want {
				t.Fatalf("normalizeEasyPayAPIBase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEasyPayRejectsNonURLAPIBase(t *testing.T) {
	t.Parallel()

	_, err := NewEasyPay("test-instance", map[string]string{
		"pid":       "pid-1",
		"pkey":      "pkey-1",
		"apiBase":   "pid-1",
		"notifyUrl": "https://example.com/notify",
		"returnUrl": "https://example.com/return",
	})
	if err == nil || !strings.Contains(err.Error(), "apiBase must be an http(s) URL") {
		t.Fatalf("NewEasyPay error = %v, want apiBase URL validation", err)
	}
}

func TestEasyPayRefundNormalizesAPIBaseAndSendsTradeNoFirst(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotQuery url.Values
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok"}`))
	}))
	defer server.Close()

	provider := newTestEasyPay(t, server.URL+"/mapi.php")
	resp, err := provider.Refund(context.Background(), payment.RefundRequest{
		TradeNo: "trade-123",
		OrderID: "out-456",
		Amount:  "1.50",
	})
	if err != nil {
		t.Fatalf("Refund returned error: %v", err)
	}
	if resp == nil || resp.Status != payment.ProviderStatusSuccess {
		t.Fatalf("Refund response = %+v, want success", resp)
	}
	if gotPath != "/api.php" {
		t.Fatalf("refund path = %q, want /api.php", gotPath)
	}
	if gotQuery.Get("act") != "refund" {
		t.Fatalf("refund act query = %q, want refund", gotQuery.Get("act"))
	}
	for key, want := range map[string]string{
		"pid":      "pid-1",
		"key":      "pkey-1",
		"trade_no": "trade-123",
		"money":    "1.50",
	} {
		if got := gotForm.Get(key); got != want {
			t.Fatalf("form[%s] = %q, want %q (form=%v)", key, got, want, gotForm)
		}
	}
	if got := gotForm.Get("out_trade_no"); got != "" {
		t.Fatalf("form[out_trade_no] = %q, want empty (form=%v)", got, gotForm)
	}
}

func TestEasyPayRefundRetriesWithOutTradeNoWhenTradeNoNotFound(t *testing.T) {
	t.Parallel()

	var gotForms []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api.php" {
			t.Errorf("refund path = %q, want /api.php", r.URL.Path)
		}
		if r.URL.Query().Get("act") != "refund" {
			t.Errorf("refund act query = %q, want refund", r.URL.Query().Get("act"))
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotForms = append(gotForms, r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		if len(gotForms) == 1 {
			_, _ = w.Write([]byte(`{"code":0,"msg":"订单编号不存在！"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok"}`))
	}))
	defer server.Close()

	provider := newTestEasyPay(t, server.URL+"/mapi.php")
	resp, err := provider.Refund(context.Background(), payment.RefundRequest{
		TradeNo: "trade-123",
		OrderID: "out-456",
		Amount:  "1.50",
	})
	if err != nil {
		t.Fatalf("Refund returned error: %v", err)
	}
	if resp == nil || resp.Status != payment.ProviderStatusSuccess || resp.RefundID != "out-456" {
		t.Fatalf("Refund response = %+v, want success with out trade refund id", resp)
	}
	if len(gotForms) != 2 {
		t.Fatalf("refund attempts = %d, want 2", len(gotForms))
	}
	if got := gotForms[0].Get("trade_no"); got != "trade-123" {
		t.Fatalf("first form[trade_no] = %q, want trade-123 (form=%v)", got, gotForms[0])
	}
	if got := gotForms[0].Get("out_trade_no"); got != "" {
		t.Fatalf("first form[out_trade_no] = %q, want empty (form=%v)", got, gotForms[0])
	}
	if got := gotForms[1].Get("out_trade_no"); got != "out-456" {
		t.Fatalf("second form[out_trade_no] = %q, want out-456 (form=%v)", got, gotForms[1])
	}
	if got := gotForms[1].Get("trade_no"); got != "" {
		t.Fatalf("second form[trade_no] = %q, want empty (form=%v)", got, gotForms[1])
	}
}

func TestEasyPayRefundResponseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "html response", statusCode: http.StatusOK, body: "<html>bad config</html>", want: "non-JSON response (HTTP 200): <html>bad config</html>"},
		{name: "non json response", statusCode: http.StatusOK, body: "not json", want: "non-JSON response (HTTP 200): not json"},
		{name: "non 2xx response", statusCode: http.StatusBadGateway, body: "bad gateway", want: "HTTP 502: bad gateway"},
		{name: "empty response", statusCode: http.StatusOK, body: "", want: "empty response (HTTP 200): <empty>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			provider := newTestEasyPay(t, server.URL)
			_, err := provider.Refund(context.Background(), payment.RefundRequest{
				OrderID: "out-456",
				Amount:  "1.50",
			})
			if err == nil {
				t.Fatal("Refund returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Refund error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestEasyPayCustomMethodsUseConfiguredUpstreamType(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":           "pid-1",
		"pkey":          "pkey-1",
		"apiBase":       "https://pay.example.com",
		"notifyUrl":     "https://example.com/notify",
		"returnUrl":     "https://example.com/return",
		"paymentMode":   paymentModePopup,
		"customMethods": `[{"type":"ldc","upstreamType":"epay","displayName":"LDC"},{"type":"usdt_trc20","upstreamType":"usdt","displayName":"USDT-TRC20"}]`,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-custom-1",
		Amount:      "1.00",
		PaymentType: "usdt_trc20",
		Subject:     "Custom EasyPay",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	payURL, err := url.Parse(resp.PayURL)
	if err != nil {
		t.Fatalf("parse pay url: %v", err)
	}
	if got := payURL.Query().Get("type"); got != "usdt" {
		t.Fatalf("pay url type = %q, want usdt (%s)", got, resp.PayURL)
	}
}

func TestEasyPayCreatePaymentSendsConfiguredMoneyType(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok","trade_no":"kyren-1","payurl":"https://payment.example.com/redirect"}`))
	}))
	defer server.Close()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":           "pid-1",
		"pkey":          "pkey-1",
		"apiBase":       server.URL + "/epay/mapi.php",
		"notifyUrl":     "https://example.com/notify",
		"returnUrl":     "https://example.com/return",
		"currency":      "usd",
		"customMethods": `[{"type":"paynow","upstreamType":"paynow","displayName":"PayNow"}]`,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-paynow-1",
		Amount:      "9.99",
		PaymentType: "paynow",
		Subject:     "PayNow Order",
		ClientIP:    "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if gotPath != "/epay/mapi.php" {
		t.Fatalf("path = %q, want /epay/mapi.php", gotPath)
	}
	for key, want := range map[string]string{
		"pid":          "pid-1",
		"type":         "paynow",
		"out_trade_no": "sub2-paynow-1",
		"money":        "9.99",
		"money_type":   "USD",
		"clientip":     "203.0.113.10",
	} {
		if got := gotForm.Get(key); got != want {
			t.Fatalf("form[%s] = %q, want %q (form=%v)", key, got, want, gotForm)
		}
	}
	if gotForm.Get("sign") == "" || gotForm.Get("sign_type") != signTypeMD5 {
		t.Fatalf("form missing signature fields: %v", gotForm)
	}
	if resp.Currency != "USD" {
		t.Fatalf("response currency = %q, want USD", resp.Currency)
	}
}

func TestEasyPayRedirectUsesKyrenEpayPathAndMoneyType(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":           "pid-1",
		"pkey":          "pkey-1",
		"apiBase":       "https://api.kyrenpay.com",
		"notifyUrl":     "https://example.com/notify",
		"returnUrl":     "https://example.com/return",
		"paymentMode":   paymentModePopup,
		"currency":      "HKD",
		"customMethods": `[{"type":"paynow","upstreamType":"paynow","displayName":"PayNow"}]`,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-paynow-redirect",
		Amount:      "9.99",
		PaymentType: "paynow",
		Subject:     "PayNow Redirect",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	payURL, err := url.Parse(resp.PayURL)
	if err != nil {
		t.Fatalf("parse pay url: %v", err)
	}
	if payURL.Scheme != "https" || payURL.Host != "api.kyrenpay.com" || payURL.Path != "/epay/submit.php" {
		t.Fatalf("pay url = %q, want https://api.kyrenpay.com/epay/submit.php", resp.PayURL)
	}
	if got := payURL.Query().Get("type"); got != "paynow" {
		t.Fatalf("pay url type = %q, want paynow", got)
	}
	if got := payURL.Query().Get("money_type"); got != "HKD" {
		t.Fatalf("pay url money_type = %q, want HKD", got)
	}
	if resp.Currency != "HKD" {
		t.Fatalf("response currency = %q, want HKD", resp.Currency)
	}
}

func TestEasyPayDefaultCurrencyOmitsMoneyType(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":         "pid-1",
		"pkey":        "pkey-1",
		"apiBase":     "https://pay.example.com",
		"notifyUrl":   "https://example.com/notify",
		"returnUrl":   "https://example.com/return",
		"paymentMode": paymentModePopup,
		"currency":    payment.DefaultPaymentCurrency,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-cny-redirect",
		Amount:      "9.99",
		PaymentType: payment.TypeAlipay,
		Subject:     "CNY Redirect",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	payURL, err := url.Parse(resp.PayURL)
	if err != nil {
		t.Fatalf("parse pay url: %v", err)
	}
	if got := payURL.Query().Get("money_type"); got != "" {
		t.Fatalf("pay url money_type = %q, want empty", got)
	}
	if resp.Currency != payment.DefaultPaymentCurrency {
		t.Fatalf("response currency = %q, want %s", resp.Currency, payment.DefaultPaymentCurrency)
	}
}

func TestEasyPayCustomMethodsResolveCIDFromConfiguredUpstreamType(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":           "pid-1",
		"pkey":          "pkey-1",
		"apiBase":       "https://pay.example.com",
		"notifyUrl":     "https://example.com/notify",
		"returnUrl":     "https://example.com/return",
		"paymentMode":   paymentModePopup,
		"cidAlipay":     "cid-alipay",
		"cidWxpay":      "cid-wxpay",
		"customMethods": `[{"type":"ldc","upstreamType":"alipay","displayName":"LDC"}]`,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-custom-cid",
		Amount:      "1.00",
		PaymentType: "ldc",
		Subject:     "Custom EasyPay CID",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	payURL, err := url.Parse(resp.PayURL)
	if err != nil {
		t.Fatalf("parse pay url: %v", err)
	}
	if got := payURL.Query().Get("type"); got != "alipay" {
		t.Fatalf("pay url type = %q, want alipay (%s)", got, resp.PayURL)
	}
	if got := payURL.Query().Get("cid"); got != "cid-alipay" {
		t.Fatalf("pay url cid = %q, want cid-alipay (%s)", got, resp.PayURL)
	}
}

func TestEasyPaySupportedTypesIncludeCustomMethods(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":           "pid-1",
		"pkey":          "pkey-1",
		"apiBase":       "https://pay.example.com",
		"notifyUrl":     "https://example.com/notify",
		"returnUrl":     "https://example.com/return",
		"customMethods": `[{"type":"ldc","upstreamType":"epay","displayName":"LDC"},{"type":"usdt_trc20","upstreamType":"usdt","displayName":"USDT-TRC20"}]`,
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	got := strings.Join(provider.SupportedTypes(), ",")
	for _, want := range []string{"alipay", "wxpay", "ldc", "usdt_trc20"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SupportedTypes() = %q, want it to include %q", got, want)
		}
	}
}

func newTestEasyPay(t *testing.T, apiBase string) *EasyPay {
	t.Helper()

	provider, err := NewEasyPay("test-instance", map[string]string{
		"pid":       "pid-1",
		"pkey":      "pkey-1",
		"apiBase":   apiBase,
		"notifyUrl": "https://example.com/notify",
		"returnUrl": "https://example.com/return",
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}
	return provider
}
