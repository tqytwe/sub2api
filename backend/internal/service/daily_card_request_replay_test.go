package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestDailyCardSettlementRequestIDWinsOverClientCorrelationID(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-replayed")
	ctx = context.WithValue(ctx, ctxkey.DailyCardSettlementRequestID, "daily:internal-settlement-1")
	require.Equal(t, "daily:internal-settlement-1", resolveUsageBillingRequestID(ctx, "upstream-request"))
}

func TestDailyCardRemainingQuotaDoesNotUseRetiredReservations(t *testing.T) {
	card := &DailyCardEntitlement{QuotaLimitUSD: 35, QuotaUsedUSD: 1.48661919, QuotaReservedUSD: 12}
	require.InDelta(t, 33.51338081, card.RemainingQuotaUSD(), 0.00000001)
}
