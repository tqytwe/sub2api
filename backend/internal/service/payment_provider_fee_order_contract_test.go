//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v85"
)

type providerFeeCheckoutCouponStub struct {
	*paymentCouponLifecycleStub
}

type providerFeeZeroRedeemRepo struct {
	*paymentOrderLifecycleRedeemRepo
}

func (r *providerFeeZeroRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	copy := *code
	copy.ID = int64(len(r.codesByCode) + 1)
	code.ID = copy.ID
	r.codesByCode[copy.Code] = &copy
	return nil
}

func (s *providerFeeCheckoutCouponStub) GetUserCouponForUser(context.Context, int64, int64) (*UserCoupon, error) {
	coupon := s.lockResult.Coupon
	return &coupon, nil
}

func (s *providerFeeCheckoutCouponStub) QuoteUserCoupon(context.Context, int64, CouponOrderContext) (*CouponQuote, error) {
	quote := s.lockResult.Quote
	return &quote, nil
}

func newProviderFeeCheckoutFixture(t *testing.T, coupon bool) (*PaymentService, *dbent.Client, *dbent.User) {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingPaymentEnabled: "true", SettingRechargeFeeRate: "9",
	}}
	config := NewPaymentConfigService(client, settings, nil)
	_, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).SetName("Fee contract gateway").
		SetConfig(`{"pid":"fixture","pkey":"synthetic","apiBase":"https://gateway.example.test","notifyUrl":"https://merchant.example.test/notify","returnUrl":"https://merchant.example.test/payment/result"}`).
		SetSupportedTypes("alipay,wxpay").SetPaymentMode("popup").
		SetFeeRate(2).SetMethodFeeRates(map[string]float64{"alipay": 0}).Save(ctx)
	require.NoError(t, err)
	user, err := client.User.Create().SetEmail("provider-fees@example.test").
		SetUsername("provider-fees").SetPasswordHash("synthetic").Save(ctx)
	require.NoError(t, err)
	svc := &PaymentService{
		entClient: client, configService: config,
		loadBalancer: payment.NewDefaultLoadBalancer(client, nil), registry: payment.NewRegistry(),
		userRepo: &mockUserRepo{getByIDUser: &User{
			ID: user.ID, Email: user.Email, Username: user.Username, Status: StatusActive,
		}},
	}
	if coupon {
		result := couponLifecycleLockResult()
		result.Coupon.UserID = user.ID
		svc.couponService = &providerFeeCheckoutCouponStub{&paymentCouponLifecycleStub{lockResult: result}}
	}
	return svc, client, user
}

func TestProviderFeeOrderUsesSelectedRateAfterCoupon(t *testing.T) {
	for _, tc := range []struct {
		method, money, source string
		rate, fee             float64
	}{
		{"alipay", "80.00", "method", 0, 0},
		{"wxpay", "81.60", "provider", 2, 1.6},
	} {
		t.Run(tc.method, func(t *testing.T) {
			ctx := context.Background()
			svc, client, user := newProviderFeeCheckoutFixture(t, true)
			response, err := svc.CreateOrder(ctx, CreateOrderRequest{
				UserID: user.ID, Amount: 100, CouponID: 41, PaymentType: tc.method,
				ReturnURL: "https://merchant.example.test/payment/result", SrcHost: "merchant.example.test",
			})
			require.NoError(t, err)
			parsed, err := url.Parse(response.PayURL)
			require.NoError(t, err)
			require.Equal(t, tc.money, parsed.Query().Get("money"), "gateway must charge the selected fee after the coupon")
			order, err := client.PaymentOrder.Get(ctx, response.OrderID)
			require.NoError(t, err)
			require.Equal(t, tc.rate, response.FeeRate)
			require.Equal(t, tc.rate, order.FeeRate)
			require.Equal(t, 100.0, order.ListAmount)
			require.Equal(t, 20.0, order.DiscountAmount)
			require.Equal(t, 80.0, order.GatewayBaseAmount)
			require.Equal(t, tc.fee, order.FeeAmount)
			require.Equal(t, 80.0, order.QualifyingRechargeAmount)
			require.Equal(t, tc.source, order.ProviderSnapshot["fee_source"])
			require.Equal(t, order.PayAmount, response.PayAmount)
			stub := svc.couponService.(*providerFeeCheckoutCouponStub)
			require.Len(t, stub.lockCalls, 1)
			require.True(t, stub.lockInTx)
		})
	}
}

func TestProviderFeeCouponQuoteUsesEffectiveFee(t *testing.T) {
	for _, tc := range []struct {
		method   string
		fee, pay float64
	}{{"alipay", 0, 80}, {"wxpay", 1.6, 81.6}} {
		t.Run(tc.method, func(t *testing.T) {
			svc, _, user := newProviderFeeCheckoutFixture(t, true)
			quote, err := svc.QuoteCouponPayment(context.Background(), CouponPaymentQuoteRequest{
				UserID: user.ID, CouponID: 41, Amount: 100, PaymentType: tc.method,
			})
			require.NoError(t, err)
			require.Equal(t, 100.0, quote.ListAmount)
			require.Equal(t, 20.0, quote.DiscountAmount)
			require.Equal(t, 80.0, quote.GatewayBaseAmount)
			require.Equal(t, tc.fee, quote.FeeAmount)
			require.Equal(t, tc.pay, quote.PayAmount)
			require.Equal(t, 80.0, quote.QualifyingRechargeAmount)
		})
	}
}

func TestProviderFeeCouponQuoteReportsRangeWithoutSelectingInstance(t *testing.T) {
	ctx := context.Background()
	svc, client, user := newProviderFeeCheckoutFixture(t, true)
	_, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).SetName("Second fee candidate").
		SetConfig(`{}`).SetSupportedTypes("alipay").SetFeeRate(4).Save(ctx)
	require.NoError(t, err)
	// A preview must not select or reserve an instance or advance round-robin.
	svc.loadBalancer = nil
	quote, err := svc.QuoteCouponPayment(ctx, CouponPaymentQuoteRequest{
		UserID: user.ID, CouponID: 41, Amount: 100, PaymentType: payment.TypeAlipay,
	})
	require.NoError(t, err)
	require.Equal(t, 80.0, quote.GatewayBaseAmount)
	require.Equal(t, 0.0, quote.FeeAmount)
	require.Equal(t, 80.0, quote.PayAmount)
	raw, err := json.Marshal(quote)
	require.NoError(t, err)
	var response map[string]any
	require.NoError(t, json.Unmarshal(raw, &response))
	require.Equal(t, 0.0, response["fee_rate_min"])
	require.Equal(t, 4.0, response["fee_rate_max"])
	require.Equal(t, 3.2, response["fee_amount_max"])
	require.Equal(t, 83.2, response["pay_amount_max"])
	require.Equal(t, true, response["estimated"])
}

func TestProviderFeeZeroCouponBypassesStripeAndProviderSelection(t *testing.T) {
	ctx := context.Background()
	svc, client, user := newProviderFeeCheckoutFixture(t, true)
	_, err := client.PaymentProviderInstance.Delete().Exec(ctx)
	require.NoError(t, err)
	svc.loadBalancer = nil
	svc.registry = nil
	coupons := svc.couponService.(*providerFeeCheckoutCouponStub)
	coupons.lockResult.Quote.DiscountAmount = 100
	coupons.lockResult.Quote.PayableAmount = 0
	coupons.lockResult.Coupon.TermsSnapshot.BenefitValue = 100
	redeems := &providerFeeZeroRedeemRepo{&paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{}}}
	users := svc.userRepo.(*mockUserRepo)
	users.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, user.ID, id)
		users.getByIDUser.Balance += amount
		return nil
	}
	svc.redeemService = NewRedeemService(redeems, users, nil, nil, nil, client, nil, nil, nil)
	quote, err := svc.QuoteCouponPayment(ctx, CouponPaymentQuoteRequest{
		UserID: user.ID, CouponID: 41, Amount: 100, PaymentType: payment.TypeStripe,
	})
	require.NoError(t, err)
	require.Zero(t, quote.PayAmount)
	result, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID: user.ID, CouponID: 41, Amount: 100, PaymentType: payment.TypeStripe,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, result.Status)
	require.Zero(t, result.PayAmount)
	order, err := client.PaymentOrder.Get(ctx, result.OrderID)
	require.NoError(t, err)
	require.Zero(t, order.QualifyingRechargeAmount)
	require.Zero(t, order.FeeAmount)
	require.Nil(t, order.ProviderInstanceID)
	require.Equal(t, []int64{41}, coupons.consumeIDs)
	require.Equal(t, 100.0, users.getByIDUser.Balance)
}

func TestProviderFeeCreateOrderRestrictsActualStripePaymentIntent(t *testing.T) {
	// Non-parallel: temporarily replace the SDK backend, never the provider
	// factory or the CreateOrder/invokeProvider path being verified.
	forms := make(chan url.Values, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/payment_intents" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid synthetic form", http.StatusBadRequest)
			return
		}
		forms <- r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"pi_fixture","object":"payment_intent","client_secret":"fixture_secret","currency":"cny"}`))
	}))
	t.Cleanup(server.Close)
	previous := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL: stripe.String(server.URL), HTTPClient: server.Client(), MaxNetworkRetries: stripe.Int64(0),
	}))
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, previous) })
	for _, tc := range []struct {
		subtype, gatewayMethod, minor string
		rate                          float64
	}{
		{"card", "card", "8000", 0},
		{"link", "link", "8240", 3},
		{"alipay", "alipay", "8160", 2},
		{"wxpay", "wechat_pay", "8320", 4},
	} {
		t.Run(tc.subtype, func(t *testing.T) {
			ctx := context.Background()
			svc, client, user := newProviderFeeCheckoutFixture(t, true)
			_, err := client.PaymentProviderInstance.Create().
				SetProviderKey(payment.TypeStripe).SetName("Synthetic Stripe").
				SetConfig(`{"secretKey":"sk_test_fixture","publishableKey":"pk_test_fixture","currency":"CNY"}`).
				SetSupportedTypes("card,link,alipay,wxpay").SetFeeRate(2).
				SetMethodFeeRates(map[string]float64{"card": 0, "link": 3, "alipay": 2, "wxpay": 4}).Save(ctx)
			require.NoError(t, err)
			result, err := svc.CreateOrder(ctx, CreateOrderRequest{
				UserID: user.ID, Amount: 100, CouponID: 41,
				PaymentType: payment.TypeStripe, PaymentSubtype: tc.subtype,
				ReturnURL: "https://merchant.example.test/payment/result", SrcHost: "merchant.example.test",
			})
			require.NoError(t, err)
			var form url.Values
			select {
			case form = <-forms:
			default:
				t.Fatal("CreateOrder must reach the actual Stripe SDK payment-intent request")
			}
			require.Equal(t, tc.minor, form.Get("amount"))
			var methods []string
			for key, values := range form {
				if strings.HasPrefix(key, "payment_method_types[") {
					methods = append(methods, values...)
				}
			}
			require.Equal(t, []string{tc.gatewayMethod}, methods, "the actual gateway must only accept the method whose fee was charged")
			order, err := client.PaymentOrder.Get(ctx, result.OrderID)
			require.NoError(t, err)
			require.Equal(t, tc.subtype, order.ProviderSnapshot["payment_subtype"])
			require.Equal(t, tc.subtype, result.PaymentSubtype)
			require.Equal(t, tc.rate, order.FeeRate)
		})
	}
}
