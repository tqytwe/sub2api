package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMobileAccountSummaryReturnsPartialErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/summary", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	}, MobileAccountSummary(nil, nil, nil))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/summary", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Orders        []PaymentOrderResult        `json:"orders"`
			Transactions  []walletTransactionDTO      `json:"transactions"`
			Plans         []paymentPlanResult         `json:"plans"`
			Subscriptions []mobileSubscriptionResult  `json:"subscriptions"`
			PartialErrors []mobileAccountSummaryError `json:"partial_errors"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Empty(t, envelope.Data.Orders)
	require.Empty(t, envelope.Data.Transactions)
	require.Empty(t, envelope.Data.Plans)
	require.Empty(t, envelope.Data.Subscriptions)
	require.Len(t, envelope.Data.PartialErrors, 3)
	require.ElementsMatch(t, []mobileAccountSummaryError{
		{Source: "wallet", Message: "账户余额暂时无法同步"},
		{Source: "payment", Message: "支付信息暂时无法同步"},
		{Source: "subscriptions", Message: "订阅信息暂时无法同步"},
	}, envelope.Data.PartialErrors)
}

func TestMobileAccountSummarySafeErrorDoesNotExposeDatabaseDetails(t *testing.T) {
	raw := errors.New(`pq: relation "wallet_entries" does not exist; host=db.internal:5432 password=secret`)
	sources := []string{"wallet", "transactions", "orders", "plans", "payment", "subscriptions", "unknown"}

	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			partial := mobileAccountSummarySafeError(source, raw)
			require.Equal(t, source, partial.Source)
			require.NotEmpty(t, partial.Message)
			require.NotContains(t, partial.Message, raw.Error())
			require.NotContains(t, partial.Message, "wallet_entries")
			require.NotContains(t, partial.Message, "db.internal")
			require.NotContains(t, partial.Message, "password")
		})
	}
}

func TestMobileAccountSummaryResponseDoesNotExposeDatabaseErrors(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.MatchExpectationsInOrder(false)
	rawMessage := `pq: relation "wallet_entries" does not exist; host=db.internal:5432 password=secret`
	mock.ExpectQuery(`(?s)SELECT.*FROM users`).WillReturnError(errors.New(rawMessage))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM balance_transactions`).WillReturnError(errors.New(rawMessage))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/summary", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	}, MobileAccountSummary(NewWalletHandler(service.NewWalletService(db)), nil, nil))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/summary", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.NotContains(t, body, rawMessage)
	require.NotContains(t, body, "wallet_entries")
	require.NotContains(t, body, "db.internal")
	require.NotContains(t, strings.ToLower(body), "password")
	require.Contains(t, body, "账户余额暂时无法同步")
	require.Contains(t, body, "余额明细暂时无法同步")
	require.NoError(t, mock.ExpectationsWereMet())
}
