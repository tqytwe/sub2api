//go:build unit

package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type mobilePlayBillingGoogleVerifierStub struct {
	purchase *MobilePlayBillingGooglePurchase
	err      error
}

func (s *mobilePlayBillingGoogleVerifierStub) VerifyGooglePlayBillingPurchase(_ context.Context, packageName string, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingGooglePurchase, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.purchase != nil {
		s.purchase.PackageName = packageName
		s.purchase.ProductID = input.ProductID
		return s.purchase, nil
	}
	return &MobilePlayBillingGooglePurchase{PackageName: packageName, ProductID: input.ProductID, ProductType: "inapp", Purchased: true}, nil
}

func TestPublicMobilePlayBillingProductsLoadsSettingMapping(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingMobilePlayBillingProducts: `{"products":[
			{"product_id":"jisudeng.balance.50","product_type":"inapp","order_type":"balance","amount":50,"formatted_price":"$7.99"},
			{"product_id":"jisudeng.plan.pro","product_type":"inapp","order_type":"subscription","plan_id":7,"amount":19.99,"currency":"usd"}
		]}`,
	}}

	products := PublicMobilePlayBillingProducts(context.Background(), repo)
	require.Len(t, products, 2)
	require.Equal(t, "jisudeng.balance.50", products[0].ProductID)
	require.Equal(t, "USD", products[1].Currency)
	require.Equal(t, int64(7), products[1].PlanID)
}

func TestUpdateMobilePlayBillingProductsPersistsValidatedAdminMapping(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
	svc := &PaymentConfigService{settingRepo: repo}
	disabled := false

	cfg, err := svc.UpdateMobilePlayBillingProducts(context.Background(), UpdateMobilePlayBillingProductsRequest{
		Products: []MobilePlayBillingProductMapping{
			{ProductID: " jisudeng.balance.50 ", ProductType: "one_time", OrderType: "topup", Amount: 50, PayAmount: 7.99, Currency: "usd", Title: "50 credits"},
			{ProductID: "jisudeng.plan.pro.30d", ProductType: "inapp", OrderType: "subscription", PlanID: 7, Amount: 19.99, Currency: "USD"},
			{ProductID: "jisudeng.balance.hidden", ProductType: "inapp", OrderType: "balance", Amount: 1, Enabled: &disabled},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "settings", cfg.ConfigSource)
	require.Equal(t, 3, cfg.ProductCount)
	require.Equal(t, 2, cfg.EnabledProductCount)
	require.Len(t, cfg.PublicProducts, 2)
	require.Equal(t, "inapp", cfg.Products[0].ProductType)
	require.Equal(t, "balance", cfg.Products[0].OrderType)
	require.Equal(t, "USD", cfg.Products[0].Currency)

	var saved []MobilePlayBillingProductMapping
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingMobilePlayBillingProducts]), &saved))
	require.Len(t, saved, 3)
	require.Equal(t, "jisudeng.balance.50", saved[0].ProductID)
}

func TestUpdateMobilePlayBillingProductsRejectsInvalidMappings(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
	svc := &PaymentConfigService{settingRepo: repo}

	_, err := svc.UpdateMobilePlayBillingProducts(context.Background(), UpdateMobilePlayBillingProductsRequest{
		Products: []MobilePlayBillingProductMapping{{ProductID: "bad product id", ProductType: "inapp", OrderType: "balance", Amount: 10}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLAY_BILLING_INVALID_PRODUCT_ID")

	_, err = svc.UpdateMobilePlayBillingProducts(context.Background(), UpdateMobilePlayBillingProductsRequest{
		Products: []MobilePlayBillingProductMapping{{ProductID: "JISUDENG-BALANCE-50", ProductType: "inapp", OrderType: "balance", Amount: 10}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLAY_BILLING_INVALID_PRODUCT_ID")

	_, err = svc.UpdateMobilePlayBillingProducts(context.Background(), UpdateMobilePlayBillingProductsRequest{
		Products: []MobilePlayBillingProductMapping{{ProductID: "jisudeng.plan.pro", ProductType: "inapp", OrderType: "subscription"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLAY_BILLING_INVALID_PLAN_ID")
}

func TestMobilePlayBillingServiceRejectsPendingPurchaseBeforeFulfillment(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingMobilePlayBillingProducts: `[{"product_id":"jisudeng.balance.50","product_type":"inapp","order_type":"balance","amount":50}]`,
	}}
	svc := NewMobilePlayBillingServiceWithVerifier(
		&PaymentService{},
		repo,
		&mobilePlayBillingGoogleVerifierStub{purchase: &MobilePlayBillingGooglePurchase{
			ProductType: "inapp",
			Pending:     true,
			Purchased:   false,
		}},
	)

	_, err := svc.VerifyMobilePlayBillingPurchase(context.Background(), 42, MobilePlayBillingPurchaseInput{
		ProductID: "jisudeng.balance.50", ProductType: "inapp", PurchaseToken: "token-1",
	})
	require.ErrorIs(t, err, ErrMobilePlayBillingPurchaseNotApproved)
}

func TestHTTPMobilePlayBillingGoogleVerifierVerifiesInAppPurchase(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	credentials := strings.ReplaceAll(`{
		"client_email":"play-billing@example.iam.gserviceaccount.com",
		"private_key":PRIVATE_KEY,
		"token_uri":TOKEN_URL
	}`, "\n", "")

	var sawTokenRequest bool
	var sawPurchaseRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			require.Equal(t, http.MethodPost, r.Method)
			sawTokenRequest = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token-1","expires_in":3600,"token_type":"Bearer"}`))
		case "/androidpublisher/v3/applications/com.jisudeng.chat/purchases/products/jisudeng.balance.50/tokens/token-1":
			require.Equal(t, "Bearer access-token-1", r.Header.Get("Authorization"))
			sawPurchaseRequest = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"purchaseTimeMillis":"12345","purchaseState":0,"consumptionState":0,"acknowledgementState":0,"orderId":"GPA.1234","quantity":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	credentials = strings.Replace(credentials, "PRIVATE_KEY", strconv.Quote(string(privateKeyPEM)), 1)
	credentials = strings.Replace(credentials, "TOKEN_URL", strconv.Quote(server.URL+"/token"), 1)
	verifier, err := NewHTTPMobilePlayBillingGoogleVerifier(MobilePlayBillingConfig{
		PackageName:        "com.jisudeng.chat",
		ServiceAccountJSON: credentials,
		APIBaseURL:         server.URL,
		TokenURL:           server.URL + "/token",
	})
	require.NoError(t, err)

	purchase, err := verifier.VerifyGooglePlayBillingPurchase(context.Background(), "com.jisudeng.chat", MobilePlayBillingPurchaseInput{
		ProductID: "jisudeng.balance.50", ProductType: "inapp", PurchaseToken: "token-1",
	})
	require.NoError(t, err)
	require.True(t, sawTokenRequest)
	require.True(t, sawPurchaseRequest)
	require.True(t, purchase.Purchased)
	require.False(t, purchase.Pending)
	require.Equal(t, "GPA.1234", purchase.OrderID)
	require.Equal(t, 2, purchase.Quantity)
}

func TestHTTPMobilePlayBillingGoogleVerifierRequiresCredentials(t *testing.T) {
	_, err := NewHTTPMobilePlayBillingGoogleVerifier(MobilePlayBillingConfig{PackageName: "com.jisudeng.chat"})
	require.ErrorIs(t, err, ErrMobilePlayBillingNotConfigured)
}

func TestFindMobilePlayBillingProductMappingRejectsDisabledProducts(t *testing.T) {
	disabled := false
	_, ok := findMobilePlayBillingProductMapping([]MobilePlayBillingProductMapping{{
		ProductID: "jisudeng.balance.50",
		Enabled:   &disabled,
	}}, "jisudeng.balance.50", "inapp")
	require.False(t, ok)
}

func TestMobilePlayBillingAcknowledgeForNonConsumableInAppPlans(t *testing.T) {
	consumable := true
	nonConsumable := false

	require.True(t, mobilePlayBillingShouldConsume(MobilePlayBillingProductMapping{
		ProductType: "inapp",
		Consumable: &consumable,
	}))
	require.False(t, mobilePlayBillingShouldAcknowledge(
		MobilePlayBillingProductMapping{ProductType: "inapp", Consumable: &consumable},
		&MobilePlayBillingGooglePurchase{AcknowledgementState: 0},
	))

	require.False(t, mobilePlayBillingShouldConsume(MobilePlayBillingProductMapping{
		ProductType: "inapp",
		Consumable: &nonConsumable,
	}))
	require.True(t, mobilePlayBillingShouldAcknowledge(
		MobilePlayBillingProductMapping{ProductType: "inapp", Consumable: &nonConsumable},
		&MobilePlayBillingGooglePurchase{AcknowledgementState: 0},
	))
	require.False(t, mobilePlayBillingShouldAcknowledge(
		MobilePlayBillingProductMapping{ProductType: "inapp", Consumable: &nonConsumable},
		&MobilePlayBillingGooglePurchase{AcknowledgementState: 1},
	))
	require.True(t, mobilePlayBillingShouldAcknowledge(
		MobilePlayBillingProductMapping{ProductType: "subs"},
		&MobilePlayBillingGooglePurchase{AcknowledgementState: 0},
	))
}

func TestMobilePlayBillingServiceMapsGoogleErrors(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{
		SettingMobilePlayBillingProducts: `[{"product_id":"jisudeng.balance.50","product_type":"inapp","order_type":"balance","amount":50}]`,
	}}
	svc := NewMobilePlayBillingServiceWithVerifier(
		&PaymentService{},
		repo,
		&mobilePlayBillingGoogleVerifierStub{err: errors.New("google unavailable")},
	)

	_, err := svc.VerifyMobilePlayBillingPurchase(context.Background(), 42, MobilePlayBillingPurchaseInput{
		ProductID: "jisudeng.balance.50", ProductType: "inapp", PurchaseToken: "token-1",
	})
	require.Error(t, err)
}
