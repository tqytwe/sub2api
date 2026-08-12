//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mobilePlayBillingVerifierStub struct {
	userID int64
	input  MobilePlayBillingPurchaseInput
	result *MobilePlayBillingPurchaseResult
	err    error
}

func (s *mobilePlayBillingVerifierStub) VerifyMobilePlayBillingPurchase(_ context.Context, userID int64, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingPurchaseResult, error) {
	s.userID, s.input = userID, input
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &MobilePlayBillingPurchaseResult{Accepted: true, Verified: true, Credited: true, Acknowledge: true}, nil
}

func TestMobilePlayBillingHandlerRequiresAuthenticatedUserAndToken(t *testing.T) {
	router := newMobilePlayBillingHandlerTestRouter(nil)

	unauthenticated := httptest.NewRecorder()
	unauthenticatedReq := httptest.NewRequest(http.MethodPost, "/play-billing/purchases", bytes.NewBufferString(`{}`))
	unauthenticatedReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(unauthenticated, unauthenticatedReq)
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	invalid := httptest.NewRecorder()
	invalidReq := httptest.NewRequest(http.MethodPost, "/play-billing/purchases", bytes.NewBufferString(`{"product_id":"jisudeng.balance.50"}`))
	invalidReq.Header.Set("Authorization", "Bearer valid")
	invalidReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(invalid, invalidReq)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	require.Contains(t, invalid.Body.String(), "purchase_token")
}

func TestMobilePlayBillingHandlerReturns503WhenVerifierIsNotConfigured(t *testing.T) {
	router := newMobilePlayBillingHandlerTestRouter(nil)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/play-billing/purchases", bytes.NewBufferString(`{
		"product_id":"jisudeng.balance.50",
		"product_type":"inapp",
		"purchase_token":"purchase-token-1",
		"client_request_id":"play-billing-1"
	}`))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.ClientRequestIDHeader, "play-billing-1")
	req.Header.Set("Idempotency-Key", "play-billing-1")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "PLAY_BILLING_NOT_CONFIGURED")
}

func TestMobilePlayBillingHandlerPassesPurchaseToVerifier(t *testing.T) {
	verifier := &mobilePlayBillingVerifierStub{result: &MobilePlayBillingPurchaseResult{
		Accepted: true,
		Verified: true,
		Credited: true,
		Consume:  true,
		OrderID:  "gp-1",
		Balance:  50,
		Amount:   50,
		Message:  "credited",
	}}
	router := newMobilePlayBillingHandlerTestRouter(verifier)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/play-billing/purchases", bytes.NewBufferString(`{
		"product_id":"jisudeng.balance.50",
		"product_type":"inapp",
		"purchase_token":"purchase-token-1",
		"order_id":"GPA.1234-5678",
		"package_name":"com.jisudeng.chat",
		"purchase_state":1,
		"client_request_id":"play-billing-1"
	}`))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), verifier.userID)
	require.Equal(t, "jisudeng.balance.50", verifier.input.ProductID)
	require.Equal(t, "purchase-token-1", verifier.input.PurchaseToken)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var data MobilePlayBillingPurchaseResult
	require.NoError(t, json.Unmarshal(raw, &data))
	require.True(t, data.Consume)
	require.Equal(t, "gp-1", data.OrderID)
}

func TestMobilePlayBillingHandlerMapsVerifierErrors(t *testing.T) {
	router := newMobilePlayBillingHandlerTestRouter(&mobilePlayBillingVerifierStub{err: ErrMobilePlayBillingVerificationFailed})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/play-billing/purchases", bytes.NewBufferString(`{
		"product_id":"jisudeng.balance.50",
		"purchase_token":"purchase-token-1",
		"client_request_id":"play-billing-1"
	}`))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Contains(t, recorder.Body.String(), "PLAY_BILLING_VERIFICATION_FAILED")
}

func newMobilePlayBillingHandlerTestRouter(verifier MobilePlayBillingVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "Bearer valid" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
	})
	router.POST("/play-billing/purchases", NewMobilePlayBillingHandler(verifier).SubmitPurchase)
	return router
}
