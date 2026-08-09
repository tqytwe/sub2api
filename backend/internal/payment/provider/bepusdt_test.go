//go:build unit

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func testBepusdtConfig(apiBase string) map[string]string {
	return map[string]string{
		"apiBase":   apiBase,
		"apiToken":  "test-token",
		"notifyUrl": "https://merchant.example.com/api/v1/payment/webhook/bepusdt",
		"returnUrl": "https://merchant.example.com/payment/result",
		"currency":  "CNY",
	}
}

func TestBepusdtCreatePaymentUsesServerCalculatedCNYAmountAndTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/order/create-transaction", r.URL.Path)

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "sub2api-123", payload["order_id"])
		require.Equal(t, "CNY", payload["fiat"])
		require.Equal(t, "usdt.trc20", payload["trade_type"])
		require.Equal(t, 28.88, payload["amount"])
		require.Equal(t, float64(900), payload["timeout"])
		require.Equal(t, "https://merchant.example.com/api/v1/payment/webhook/bepusdt", payload["notify_url"])
		require.Equal(t, "https://app.example.com/payment/result?order_id=123&resume_token=signed", payload["redirect_url"])
		require.Equal(t, bepusdtSign(payload, "test-token"), payload["signature"])

		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"payment_url\":\"" + serverURLForResponse(r) + "/pay/checkout/be-trade-1\",\"amount\":\"28.88\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\",\"expiration_time\":900,\"status\":1}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	result, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:   "sub2api-123",
		Amount:    "28.88",
		Subject:   "Subscription",
		ExpiresAt: time.Now().Add(15 * time.Minute),
		ReturnURL: "https://app.example.com/payment/result?order_id=123&resume_token=signed",
	})
	require.NoError(t, err)
	require.Equal(t, "be-trade-1", result.TradeNo)
	require.Equal(t, server.URL+"/pay/checkout/be-trade-1", result.PayURL)
	require.Empty(t, result.QRCode)
	require.Equal(t, payment.DefaultPaymentCurrency, result.Currency)
}

func serverURLForResponse(r *http.Request) string {
	return "http://" + r.Host
}

func TestBepusdtCreatePaymentRejectsMismatchedGatewayOrder(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"another-order\",\"payment_url\":\"" + serverURLForResponse(r) + "/pay/checkout/be-trade-1\",\"amount\":\"28.88\",\"actual_amount\":\"4.00\",\"token\":\"TRON_ADDRESS\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\",\"expiration_time\":900,\"status\":1}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	_, err = provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2api-123", Amount: "28.88", Subject: "Subscription", ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.ErrorContains(t, err, "order_id mismatch")
}

func TestBepusdtCreatePaymentAllowsSeparateHostedCheckoutOrigin(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"payment_url\":\"https://checkout.example.com/pay/checkout/be-trade-1\",\"amount\":\"28.88\",\"actual_amount\":\"4.00\",\"token\":\"TRON_ADDRESS\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\",\"expiration_time\":900,\"status\":1}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	result, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2api-123", Amount: "28.88"})
	require.NoError(t, err)
	require.Equal(t, "https://checkout.example.com/pay/checkout/be-trade-1", result.PayURL)
}

func TestBepusdtCreatePaymentAcceptsResponseWithoutOptionalQuoteFields(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-legacy\",\"order_id\":\"sub2-legacy\",\"payment_url\":\"https://checkout.example.com/pay/checkout-legacy\",\"amount\":\"28.88\",\"expiration_time\":900,\"status\":1}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	result, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2-legacy",
		Amount:  "28.88",
	})
	require.NoError(t, err)
	require.Equal(t, "be-trade-legacy", result.TradeNo)
	require.Equal(t, "https://checkout.example.com/pay/checkout-legacy", result.PayURL)
}

func TestBepusdtCreatePaymentRejectsNonHTTPPaymentURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"be-trade-1","order_id":"sub2api-123","payment_url":"javascript://checkout.example.com/pay/be-trade-1","amount":"28.88","actual_amount":"4.00","token":"TRON_ADDRESS","fiat":"CNY","trade_type":"usdt.trc20","expiration_time":900,"status":1}}`))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	_, err = provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2api-123", Amount: "28.88"})
	require.ErrorContains(t, err, "payment_url must use http or https")
}

func TestBepusdtCreatePaymentRejectsNonPendingGatewayOrder(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"payment_url\":\"" + serverURLForResponse(r) + "/pay/checkout/be-trade-1\",\"amount\":\"28.88\",\"actual_amount\":\"4.00\",\"token\":\"TRON_ADDRESS\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\",\"expiration_time\":900,\"status\":2}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	_, err = provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "sub2api-123", Amount: "28.88", Subject: "Subscription", ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.ErrorContains(t, err, "order status")
}

func TestBepusdtCreatePaymentRejectsFractionalGatewayStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"payment_url\":\"" + serverURLForResponse(r) + "/pay/checkout/be-trade-1\",\"amount\":\"28.88\",\"actual_amount\":\"4.00\",\"token\":\"TRON_ADDRESS\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\",\"expiration_time\":900,\"status\":1.5}}"))
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	_, err = provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "sub2api-123", Amount: "28.88", Subject: "Plan", ExpiresAt: time.Now().Add(15 * time.Minute)})
	require.ErrorContains(t, err, "order status")
}

func TestBepusdtVerifyNotificationRequiresChainEvidenceForSuccess(t *testing.T) {
	t.Parallel()
	provider, err := NewBepusdt("1", testBepusdtConfig("https://bepusdt.example.com"))
	require.NoError(t, err)

	payload := map[string]any{
		"trade_id": "be-trade-1", "order_id": "sub2api-123", "amount": 28.88,
		"actual_amount": "4.00", "token": "TRON_ADDRESS", "block_transaction_id": "chain-hash", "status": 2,
	}
	payload["signature"] = bepusdtSign(payload, "test-token")
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	notification, err := provider.VerifyNotification(context.Background(), string(raw), nil)
	require.NoError(t, err)
	require.Equal(t, payment.NotificationStatusSuccess, notification.Status)
	require.Equal(t, 28.88, notification.Amount)
	require.Equal(t, "sub2api-123", notification.OrderID)
	require.Equal(t, "be-trade-1", notification.TradeNo)
	require.Equal(t, "4.00", notification.Metadata["actual_amount"])
	require.Equal(t, "chain-hash", notification.Metadata["block_transaction_id"])

	delete(payload, "block_transaction_id")
	payload["signature"] = bepusdtSign(payload, "test-token")
	raw, err = json.Marshal(payload)
	require.NoError(t, err)
	_, err = provider.VerifyNotification(context.Background(), string(raw), nil)
	require.ErrorContains(t, err, "block_transaction_id")
}

func TestBepusdtVerifyNotificationAcceptsTimeoutWithoutChainEvidence(t *testing.T) {
	t.Parallel()
	provider, err := NewBepusdt("1", testBepusdtConfig("https://bepusdt.example.com"))
	require.NoError(t, err)

	payload := map[string]any{
		"trade_id":  "be-trade-timeout",
		"order_id":  "sub2-timeout",
		"amount":    28.88,
		"status":    3,
		"signature": "",
	}
	payload["signature"] = bepusdtSign(payload, "test-token")
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	notification, err := provider.VerifyNotification(context.Background(), string(raw), nil)
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusFailed, notification.Status)
	require.Equal(t, "sub2-timeout", notification.OrderID)
}

func TestBepusdtVerifyNotificationRejectsInvalidSignature(t *testing.T) {
	t.Parallel()
	provider, err := NewBepusdt("1", testBepusdtConfig("https://bepusdt.example.com"))
	require.NoError(t, err)
	payload := map[string]any{
		"trade_id": "be-trade-1", "order_id": "sub2api-123", "amount": 28.88,
		"actual_amount": "4.00", "token": "TRON_ADDRESS", "block_transaction_id": "chain-hash", "status": 2,
		"signature": "invalid",
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = provider.VerifyNotification(context.Background(), string(raw), nil)
	require.ErrorContains(t, err, "signature is invalid")
}

func TestBepusdtVerifyNotificationRejectsUnknownOrFractionalStatus(t *testing.T) {
	t.Parallel()
	provider, err := NewBepusdt("1", testBepusdtConfig("https://bepusdt.example.com"))
	require.NoError(t, err)

	for _, status := range []any{4, 2.5, "unknown"} {
		payload := map[string]any{
			"trade_id": "be-trade-1", "order_id": "sub2api-123", "amount": 28.88,
			"actual_amount": "4.00", "token": "TRON_ADDRESS", "block_transaction_id": "chain-hash", "status": status,
		}
		payload["signature"] = bepusdtSign(payload, "test-token")
		raw, marshalErr := json.Marshal(payload)
		require.NoError(t, marshalErr)

		_, verifyErr := provider.VerifyNotification(context.Background(), string(raw), nil)
		require.ErrorContains(t, verifyErr, "invalid status")
	}
}

func TestBepusdtQueryAndCancel(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		switch r.URL.Path {
		case "/api/v1/pay/info":
			require.Equal(t, "be-trade-1", payload["trade_id"])
			_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"status\":2,\"money\":\"28.88\",\"actual_amount\":\"4.00\",\"trade_url\":\"https://tronscan.org/#/transaction/chain-hash\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\"}}"))
		case "/api/v1/order/cancel-transaction":
			require.Equal(t, bepusdtSign(payload, "test-token"), payload["signature"])
			_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\"}}"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
	require.NoError(t, err)
	query, err := provider.QueryOrder(context.Background(), "be-trade-1")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPaid, query.Status)
	require.Equal(t, 28.88, query.Amount)
	require.Equal(t, "sub2api-123", query.Metadata["order_id"])
	require.Equal(t, "https://tronscan.org/#/transaction/chain-hash", query.Metadata["trade_url"])
	require.NoError(t, provider.CancelPayment(context.Background(), "be-trade-1"))
}

func TestBepusdtQueryOrderRejectsInvalidStatus(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"2.5", "7", `"unknown"`} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("{\"status_code\":200,\"message\":\"success\",\"data\":{\"trade_id\":\"be-trade-1\",\"order_id\":\"sub2api-123\",\"status\":" + status + ",\"money\":\"28.88\",\"actual_amount\":\"4.00\",\"trade_url\":\"https://tronscan.org/#/transaction/chain-hash\",\"fiat\":\"CNY\",\"trade_type\":\"usdt.trc20\"}}"))
			}))
			defer server.Close()

			provider, err := NewBepusdt("1", testBepusdtConfig(server.URL))
			require.NoError(t, err)
			_, err = provider.QueryOrder(context.Background(), "be-trade-1")
			require.ErrorContains(t, err, "invalid status")
		})
	}
}

func TestNewBepusdtRejectsUnsafeOrInvalidConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mutate   func(map[string]string)
		contains string
	}{
		{name: "non CNY", mutate: func(config map[string]string) { config["currency"] = "USD" }, contains: "CNY"},
		{name: "base path", mutate: func(config map[string]string) { config["apiBase"] = "https://bepusdt.example.com/api" }, contains: "path"},
		{name: "credentials", mutate: func(config map[string]string) { config["apiBase"] = "https://user:pass@bepusdt.example.com" }, contains: "credentials"},
		{name: "query", mutate: func(config map[string]string) { config["apiBase"] = "https://bepusdt.example.com?token=secret" }, contains: "query"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testBepusdtConfig("https://bepusdt.example.com")
			test.mutate(config)
			_, err := NewBepusdt("1", config)
			require.Error(t, err)
			require.True(t, strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.contains)), err.Error())
		})
	}
}
