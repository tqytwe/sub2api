package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	bepusdtHTTPTimeout       = 10 * time.Second
	bepusdtMaxResponseSize   = 1 << 20
	bepusdtAssetCurrency     = "USDT"
	bepusdtMinTimeoutSeconds = int64(180)
	bepusdtMaxSubjectRunes   = 64
)

// Bepusdt implements the BEpusdt ePusdt JSON API. The payable amount remains
// the immutable CNY amount calculated by this application; BEpusdt converts it
// to and locks the corresponding USDT amount for the hosted checkout. Network
// selection is delegated to BEpusdt so enabled wallets such as TRC20, BSC,
// Arbitrum, Base, and other USDT networks can be offered without trusting the
// browser to submit a crypto amount.
type Bepusdt struct {
	instanceID string
	config     map[string]string
	apiBaseURL *url.URL
	httpClient *http.Client
}

func NewBepusdt(instanceID string, config map[string]string) (*Bepusdt, error) {
	for _, key := range []string{"apiBase", "apiToken", "notifyUrl", "returnUrl"} {
		if strings.TrimSpace(config[key]) == "" {
			return nil, fmt.Errorf("bepusdt config missing required key: %s", key)
		}
	}
	apiBase, parsedBase, err := normalizeBepusdtAPIBase(config["apiBase"])
	if err != nil {
		return nil, err
	}
	currency, err := payment.NormalizePaymentCurrency(config["currency"])
	if err != nil {
		return nil, fmt.Errorf("bepusdt config invalid currency: %w", err)
	}
	if currency != payment.DefaultPaymentCurrency {
		return nil, fmt.Errorf("bepusdt only supports CNY settlement")
	}
	for _, key := range []string{"notifyUrl", "returnUrl"} {
		if err := validateBepusdtCallbackURL(key, config[key]); err != nil {
			return nil, err
		}
	}

	cfg := make(map[string]string, len(config))
	for key, value := range config {
		cfg[key] = strings.TrimSpace(value)
	}
	cfg["apiBase"] = apiBase
	cfg["currency"] = currency
	return &Bepusdt{
		instanceID: instanceID,
		config:     cfg,
		apiBaseURL: parsedBase,
		httpClient: &http.Client{
			Timeout: bepusdtHTTPTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func normalizeBepusdtAPIBase(raw string) (string, *url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", nil, fmt.Errorf("bepusdt config apiBase must be an http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("bepusdt config apiBase must use http or https")
	}
	if parsed.User != nil {
		return "", nil, fmt.Errorf("bepusdt config apiBase must not contain credentials")
	}
	if parsed.RawQuery != "" {
		return "", nil, fmt.Errorf("bepusdt config apiBase must not contain a query")
	}
	if parsed.Fragment != "" {
		return "", nil, fmt.Errorf("bepusdt config apiBase must not contain a fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", nil, fmt.Errorf("bepusdt config apiBase path must be empty")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), parsed, nil
}

func validateBepusdtCallbackURL(key, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("bepusdt config %s must be an absolute http(s) URL", key)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("bepusdt config %s must use http or https", key)
	}
	if parsed.User != nil {
		return fmt.Errorf("bepusdt config %s must not contain credentials", key)
	}
	return nil
}

func (b *Bepusdt) Name() string        { return "BEpusdt" }
func (b *Bepusdt) ProviderKey() string { return payment.TypeBepusdt }
func (b *Bepusdt) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeBepusdt}
}

func (b *Bepusdt) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	amount, err := bepusdtCNYAmount(req.Amount)
	if err != nil {
		return nil, err
	}
	redirectURL := b.config["returnUrl"]
	if strings.TrimSpace(req.ReturnURL) != "" {
		if err := validateBepusdtCallbackURL("returnUrl", req.ReturnURL); err != nil {
			return nil, err
		}
		redirectURL = strings.TrimSpace(req.ReturnURL)
	}
	payload := map[string]any{
		"order_id":     strings.TrimSpace(req.OrderID),
		"amount":       amount,
		"fiat":         payment.DefaultPaymentCurrency,
		"currencies":   bepusdtAssetCurrency,
		"name":         truncateBepusdtSubject(req.Subject),
		"notify_url":   b.config["notifyUrl"],
		"redirect_url": redirectURL,
	}
	if payload["order_id"] == "" {
		return nil, fmt.Errorf("bepusdt order_id is required")
	}
	if timeout := bepusdtTimeoutSeconds(req.ExpiresAt); timeout > 0 {
		payload["timeout"] = timeout
	}
	payload["signature"] = bepusdtSign(payload, b.config["apiToken"])

	var response bepusdtCreateResponse
	if err := b.postJSON(ctx, "/api/v1/order/create-order", payload, &response); err != nil {
		return nil, fmt.Errorf("bepusdt create order: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt create order failed: %s", bepusdtMessage(response.Message))
	}
	if err := b.validateCreateResponse(req, amount, response.Data); err != nil {
		return nil, err
	}
	return &payment.CreatePaymentResponse{
		TradeNo: strings.TrimSpace(response.Data.TradeID),
		PayURL:  strings.TrimSpace(response.Data.PaymentURL),
		// token is the wallet address, not a payment QR containing the locked
		// USDT amount. Keep the hosted checkout as the only launch path so the
		// user sees BEpusdt's live quote and cannot accidentally enter an amount.
		Currency: payment.DefaultPaymentCurrency,
	}, nil
}

type bepusdtCreateResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       struct {
		TradeID       string          `json:"trade_id"`
		OrderID       string          `json:"order_id"`
		PaymentURL    string          `json:"payment_url"`
		Amount        json.RawMessage `json:"amount"`
		ActualAmount  json.RawMessage `json:"actual_amount"`
		Token         string          `json:"token"`
		Fiat          string          `json:"fiat"`
		Currencies    string          `json:"currencies"`
		TradeType     string          `json:"trade_type"`
		ExpirationSec int64           `json:"expiration_time"`
		Status        json.RawMessage `json:"status"`
	} `json:"data"`
}

func (b *Bepusdt) validateCreateResponse(req payment.CreatePaymentRequest, expectedAmount float64, data struct {
	TradeID       string          `json:"trade_id"`
	OrderID       string          `json:"order_id"`
	PaymentURL    string          `json:"payment_url"`
	Amount        json.RawMessage `json:"amount"`
	ActualAmount  json.RawMessage `json:"actual_amount"`
	Token         string          `json:"token"`
	Fiat          string          `json:"fiat"`
	Currencies    string          `json:"currencies"`
	TradeType     string          `json:"trade_type"`
	ExpirationSec int64           `json:"expiration_time"`
	Status        json.RawMessage `json:"status"`
}) error {
	if strings.TrimSpace(data.TradeID) == "" || strings.TrimSpace(data.PaymentURL) == "" {
		return fmt.Errorf("bepusdt create order returned incomplete order data")
	}
	if strings.TrimSpace(data.OrderID) != strings.TrimSpace(req.OrderID) {
		return fmt.Errorf("bepusdt create order order_id mismatch")
	}
	// Older BEpusdt builds omit optional response fields even though they honor
	// the request fields. Validate fields when present, but do not reject a valid
	// hosted checkout solely because those optional fields are absent.
	if fiat := strings.TrimSpace(data.Fiat); fiat != "" &&
		!strings.EqualFold(fiat, payment.DefaultPaymentCurrency) {
		return fmt.Errorf("bepusdt create order fiat mismatch")
	}
	if currencies := strings.TrimSpace(data.Currencies); currencies != "" &&
		!bepusdtIsUSDTAssetCurrency(currencies) {
		return fmt.Errorf("bepusdt create order currencies mismatch")
	}
	if tradeType := strings.TrimSpace(data.TradeType); tradeType != "" &&
		!bepusdtIsUSDTTradeType(tradeType) {
		return fmt.Errorf("bepusdt create order trade_type mismatch")
	}
	amount, err := bepusdtNumber(data.Amount)
	if err != nil || !bepusdtAmountsEqual(amount, expectedAmount) {
		return fmt.Errorf("bepusdt create order amount mismatch")
	}
	// BEpusdt returns the live USDT quote and wallet token from the hosted
	// checkout/query endpoints. If the create response includes a quote, validate
	// it so malformed gateway data still fails closed.
	if len(data.ActualAmount) > 0 && string(data.ActualAmount) != "null" {
		actualAmount, actualErr := bepusdtNumber(data.ActualAmount)
		if actualErr != nil || actualAmount <= 0 {
			return fmt.Errorf("bepusdt create order returned invalid USDT quote")
		}
	}
	if data.ExpirationSec <= 0 {
		return fmt.Errorf("bepusdt create order returned invalid expiration_time")
	}
	statusCode, statusErr := bepusdtStatus(data.Status)
	if statusErr != nil || statusCode != 1 {
		return fmt.Errorf("bepusdt create order returned invalid order status")
	}
	if err := b.validatePaymentURL(data.PaymentURL); err != nil {
		return err
	}
	return nil
}

func (b *Bepusdt) validatePaymentURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("bepusdt returned an invalid payment_url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("bepusdt payment_url must use http or https")
	}
	// BEpusdt may serve its API and hosted checkout from different public
	// origins when api_app_uri is configured behind a separate reverse proxy.
	// The URL is returned to the browser only; this service never follows it.
	if parsed.User != nil {
		return fmt.Errorf("bepusdt payment_url must not contain credentials")
	}
	return nil
}

func (b *Bepusdt) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, fmt.Errorf("bepusdt trade_id is required")
	}
	var response struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID      string          `json:"trade_id"`
			OrderID      string          `json:"order_id"`
			Status       json.RawMessage `json:"status"`
			Money        json.RawMessage `json:"money"`
			ActualAmount json.RawMessage `json:"actual_amount"`
			Token        string          `json:"token"`
			Fiat         string          `json:"fiat"`
			Currency     string          `json:"currency"`
			Network      string          `json:"network"`
			TradeType    string          `json:"trade_type"`
			TradeURL     string          `json:"trade_url"`
		} `json:"data"`
	}
	if err := b.postJSON(ctx, "/api/v1/pay/info", map[string]any{"trade_id": tradeNo}, &response); err != nil {
		return nil, fmt.Errorf("bepusdt query transaction: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt query transaction failed: %s", bepusdtMessage(response.Message))
	}
	resolvedTradeNo := strings.TrimSpace(response.Data.TradeID)
	if resolvedTradeNo == "" || !hmac.Equal([]byte(resolvedTradeNo), []byte(tradeNo)) {
		return nil, fmt.Errorf("bepusdt query transaction trade_id mismatch")
	}
	if strings.TrimSpace(response.Data.OrderID) == "" {
		return nil, fmt.Errorf("bepusdt query transaction missing order_id")
	}
	if fiat := strings.TrimSpace(response.Data.Fiat); fiat != "" && !strings.EqualFold(fiat, payment.DefaultPaymentCurrency) {
		return nil, fmt.Errorf("bepusdt query transaction fiat mismatch")
	}
	if tradeType := strings.TrimSpace(response.Data.TradeType); tradeType != "" && !bepusdtIsUSDTTradeType(tradeType) {
		return nil, fmt.Errorf("bepusdt query transaction trade_type mismatch")
	}
	if assetCurrency := strings.TrimSpace(response.Data.Currency); assetCurrency != "" && !bepusdtIsUSDTAssetCurrency(assetCurrency) {
		return nil, fmt.Errorf("bepusdt query transaction asset currency mismatch")
	}
	statusCode, statusErr := bepusdtStatus(response.Data.Status)
	if statusErr != nil {
		return nil, fmt.Errorf("bepusdt query transaction returned invalid status")
	}
	status := payment.ProviderStatusPending
	switch statusCode {
	case 2:
		status = payment.ProviderStatusPaid
	case 3, 4, 6:
		status = payment.ProviderStatusFailed
	}
	amount, err := bepusdtNumber(response.Data.Money)
	if err != nil || amount <= 0 {
		return nil, fmt.Errorf("bepusdt query transaction returned invalid CNY amount")
	}
	metadata := map[string]string{
		"order_id":       strings.TrimSpace(response.Data.OrderID),
		"trade_id":       resolvedTradeNo,
		"currency":       payment.DefaultPaymentCurrency,
		"asset_currency": bepusdtAssetCurrency,
	}
	for key, value := range map[string]string{
		"actual_amount": bepusdtRawString(response.Data.ActualAmount),
		"token":         strings.TrimSpace(response.Data.Token),
		"trade_url":     strings.TrimSpace(response.Data.TradeURL),
		"trade_type":    strings.TrimSpace(response.Data.TradeType),
		"network":       strings.TrimSpace(response.Data.Network),
		"asset":         strings.TrimSpace(response.Data.Currency),
	} {
		if value != "" {
			metadata[key] = value
		}
	}
	if status == payment.ProviderStatusPaid {
		actualAmount, actualErr := bepusdtNumber(response.Data.ActualAmount)
		if actualErr != nil || actualAmount <= 0 || metadata["trade_url"] == "" {
			return nil, fmt.Errorf("bepusdt paid query is missing chain evidence")
		}
	}
	return &payment.QueryOrderResponse{TradeNo: resolvedTradeNo, Status: status, Amount: amount, Metadata: metadata}, nil
}

func (b *Bepusdt) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
		return nil, fmt.Errorf("bepusdt parse notification: %w", err)
	}
	signature, ok := payload["signature"].(string)
	if !ok || strings.TrimSpace(signature) == "" {
		return nil, fmt.Errorf("bepusdt notification missing signature")
	}
	expectedSignature := bepusdtSign(payload, b.config["apiToken"])
	if !hmac.Equal([]byte(expectedSignature), []byte(strings.ToLower(strings.TrimSpace(signature)))) {
		return nil, fmt.Errorf("bepusdt notification signature is invalid")
	}
	orderID := bepusdtString(payload["order_id"])
	tradeID := bepusdtString(payload["trade_id"])
	if orderID == "" || tradeID == "" {
		return nil, fmt.Errorf("bepusdt notification missing order_id or trade_id")
	}
	amount, err := bepusdtAnyNumber(payload["amount"])
	if err != nil || amount <= 0 {
		return nil, fmt.Errorf("bepusdt notification has invalid CNY amount")
	}
	statusCode, err := bepusdtNotificationStatus(payload["status"])
	if err != nil {
		return nil, err
	}
	status := payment.ProviderStatusFailed
	if statusCode == 2 {
		status = payment.NotificationStatusSuccess
	}
	actualAmount := bepusdtString(payload["actual_amount"])
	blockTransactionID := bepusdtString(payload["block_transaction_id"])
	if statusCode == 2 {
		parsedActualAmount, actualErr := bepusdtAnyNumber(payload["actual_amount"])
		if actualErr != nil || parsedActualAmount <= 0 {
			return nil, fmt.Errorf("bepusdt paid notification has invalid actual_amount")
		}
		if blockTransactionID == "" {
			return nil, fmt.Errorf("bepusdt paid notification missing block_transaction_id")
		}
	}
	metadata := map[string]string{
		"order_id":       orderID,
		"trade_id":       tradeID,
		"currency":       payment.DefaultPaymentCurrency,
		"asset_currency": bepusdtAssetCurrency,
	}
	if tradeType := bepusdtString(payload["trade_type"]); tradeType != "" {
		if !bepusdtIsUSDTTradeType(tradeType) {
			return nil, fmt.Errorf("bepusdt notification trade_type mismatch")
		}
		metadata["trade_type"] = tradeType
	}
	assetCurrency := bepusdtString(payload["currency"])
	if assetCurrency != "" && !bepusdtIsUSDTAssetCurrency(assetCurrency) {
		return nil, fmt.Errorf("bepusdt notification asset currency mismatch")
	}
	for key, value := range map[string]string{
		"actual_amount":        actualAmount,
		"token":                bepusdtString(payload["token"]),
		"block_transaction_id": blockTransactionID,
		"network":              bepusdtString(payload["network"]),
		"asset":                assetCurrency,
	} {
		if value != "" {
			metadata[key] = value
		}
	}
	return &payment.PaymentNotification{
		TradeNo: tradeID, OrderID: orderID, Amount: amount, Status: status, RawData: rawBody, Metadata: metadata,
	}, nil
}

func (b *Bepusdt) Refund(_ context.Context, _ payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("bepusdt does not support automatic refunds; process the on-chain refund manually")
}

func (b *Bepusdt) CancelPayment(ctx context.Context, tradeNo string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return fmt.Errorf("bepusdt trade_id is required")
	}
	payload := map[string]any{"trade_id": tradeNo}
	payload["signature"] = bepusdtSign(payload, b.config["apiToken"])
	var response struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID string `json:"trade_id"`
		} `json:"data"`
	}
	if err := b.postJSON(ctx, "/api/v1/order/cancel-transaction", payload, &response); err != nil {
		return fmt.Errorf("bepusdt cancel transaction: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bepusdt cancel transaction failed: %s", bepusdtMessage(response.Message))
	}
	if strings.TrimSpace(response.Data.TradeID) != tradeNo {
		return fmt.Errorf("bepusdt cancel transaction trade_id mismatch")
	}
	return nil
}

func (b *Bepusdt) postJSON(ctx context.Context, endpoint string, request any, target any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	client := b.httpClient
	if client == nil {
		client = &http.Client{Timeout: bepusdtHTTPTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, bepusdtMaxResponseSize+1))
	if err != nil {
		return err
	}
	if len(responseBody) > bepusdtMaxResponseSize {
		return fmt.Errorf("response exceeds %d bytes", bepusdtMaxResponseSize)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, summarizeBepusdtResponse(responseBody))
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

func bepusdtCNYAmount(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if _, err := payment.AmountToMinorUnit(raw, payment.DefaultPaymentCurrency); err != nil {
		return 0, fmt.Errorf("bepusdt invalid CNY amount: %q", raw)
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, fmt.Errorf("bepusdt invalid CNY amount: %q", raw)
	}
	return amount, nil
}

func bepusdtTimeoutSeconds(expiresAt time.Time) int64 {
	if expiresAt.IsZero() {
		return 0
	}
	seconds := int64(math.Ceil(time.Until(expiresAt).Seconds()))
	if seconds < bepusdtMinTimeoutSeconds {
		return bepusdtMinTimeoutSeconds
	}
	return seconds
}

func truncateBepusdtSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if utf8.RuneCountInString(subject) <= bepusdtMaxSubjectRunes {
		return subject
	}
	runes := []rune(subject)
	return string(runes[:bepusdtMaxSubjectRunes])
}

func bepusdtAmountsEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.000001
}

func bepusdtIsUSDTTradeType(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return normalized == strings.ToLower(bepusdtAssetCurrency) ||
		strings.HasPrefix(normalized, strings.ToLower(bepusdtAssetCurrency)+".")
}

func bepusdtIsUSDTAssetCurrency(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), bepusdtAssetCurrency)
}

func bepusdtMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "unknown BEpusdt error"
	}
	return message
}

func bepusdtSign(payload map[string]any, token string) string {
	keys := make([]string, 0, len(payload))
	for key, value := range payload {
		if key != "signature" && !bepusdtEmptyValue(value) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprint(payload[key]))
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(parts, "&")+token)))
}

func bepusdtEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	if stringValue, ok := value.(string); ok {
		return stringValue == ""
	}
	return false
}

func bepusdtStatus(raw json.RawMessage) (int, error) {
	value, err := bepusdtNumber(raw)
	if err != nil || value != math.Trunc(value) {
		return 0, fmt.Errorf("invalid status")
	}
	statusCode := int(value)
	if statusCode < 1 || statusCode > 6 {
		return 0, fmt.Errorf("invalid status")
	}
	return statusCode, nil
}

func bepusdtNumber(raw json.RawMessage) (float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, errors.New("missing number")
	}
	return bepusdtAnyNumber(strings.Trim(strings.TrimSpace(string(raw)), "\""))
}

func bepusdtRawString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func bepusdtAnyNumber(value any) (float64, error) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(bepusdtString(value)), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, fmt.Errorf("invalid number")
	}
	return amount, nil
}

func bepusdtNotificationStatus(value any) (int, error) {
	status, err := bepusdtAnyNumber(value)
	if err != nil || status != math.Trunc(status) {
		return 0, fmt.Errorf("bepusdt notification has invalid status")
	}
	statusCode := int(status)
	if statusCode < 1 || statusCode > 3 {
		return 0, fmt.Errorf("bepusdt notification has invalid status")
	}
	return statusCode, nil
}

func bepusdtString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func summarizeBepusdtResponse(body []byte) string {
	summary := strings.TrimSpace(string(body))
	if len(summary) > 512 {
		return summary[:512] + "..."
	}
	return summary
}

var (
	_ payment.Provider           = (*Bepusdt)(nil)
	_ payment.CancelableProvider = (*Bepusdt)(nil)
)
