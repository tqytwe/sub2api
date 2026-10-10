//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerCommittedJSONDoesNotBecomeSSE(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	router := gin.New()
	router.Use(requestledger.Middleware(ledger))
	var privateID string
	router.POST("/v1/responses", func(c *gin.Context) {
		privateID = requestledger.FromContext(c.Request.Context()).ID
		require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
		c.JSON(400, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "synthetic rejection"}})
		h := &OpenAIGatewayHandler{}
		h.handleStreamingAwareError(c, 502, "upstream_error", "synthetic later failure", true)
	})
	writer := httptest.NewRecorder()
	router.ServeHTTP(writer, httptest.NewRequest("POST", "/v1/responses", nil))
	require.Equal(t, 400, writer.Code)
	require.True(t, json.Valid(writer.Body.Bytes()), "a committed JSON response must remain one JSON document")
	require.NotContains(t, writer.Body.String(), "data:")
	require.Equal(t, 1, strings.Count(writer.Body.String(), "synthetic rejection"))
	record, err := ledger.Get(context.Background(), requestledger.Viewer{UserID: 101}, privateID)
	require.NoError(t, err)
	require.Equal(t, "failed", record.ExecutionState)
	require.EqualValues(t, 400, *record.HTTPStatus)
	require.Zero(t, record.AttemptCount)
}
