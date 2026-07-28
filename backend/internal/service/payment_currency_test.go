package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderCurrencyUsesLegacyProviderSnapshot(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentCurrency: payment.DefaultPaymentCurrency,
		ProviderSnapshot: map[string]any{
			"currency": "USD",
		},
	}
	require.Equal(t, "USD", PaymentOrderCurrency(order))
}

func TestPaymentOrderCurrencyUsesPersistedSettlementCurrency(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		ListAmount:      10,
		PaymentCurrency: "HKD",
		ProviderSnapshot: map[string]any{
			"currency": "USD",
		},
	}
	require.Equal(t, "HKD", PaymentOrderCurrency(order))
}
