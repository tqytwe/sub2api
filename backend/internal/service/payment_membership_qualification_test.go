package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderMembershipAmountUsesQualifyingCNYSnapshot(t *testing.T) {
	order := &dbent.PaymentOrder{
		OrderType:                "balance",
		PaymentCurrency:          "CNY",
		ListAmount:               100,
		GatewayBaseAmount:        90,
		FeeAmount:                9,
		QualifyingRechargeAmount: 90,
	}
	amount, ok := paymentOrderMembershipAmount(order)
	require.True(t, ok)
	require.Equal(t, 90.0, amount)
}

func TestPaymentOrderMembershipAmountRejectsIncompleteOrNonCNYSnapshot(t *testing.T) {
	base := &dbent.PaymentOrder{OrderType: "balance", PaymentCurrency: "CNY", ListAmount: 100, GatewayBaseAmount: 90, QualifyingRechargeAmount: 90}
	for name, order := range map[string]*dbent.PaymentOrder{
		"missing list amount":   {OrderType: base.OrderType, PaymentCurrency: base.PaymentCurrency, GatewayBaseAmount: 90, QualifyingRechargeAmount: 90},
		"unsupported currency":  {OrderType: base.OrderType, PaymentCurrency: "USD", ListAmount: 100, GatewayBaseAmount: 90, QualifyingRechargeAmount: 90},
		"subscription snapshot": {OrderType: "subscription", PaymentCurrency: "CNY", ListAmount: 100, GatewayBaseAmount: 90, QualifyingRechargeAmount: 90},
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := paymentOrderMembershipAmount(order)
			require.False(t, ok)
		})
	}
}
