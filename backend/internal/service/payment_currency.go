package service

import (
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func paymentProviderConfigCurrency(providerKey string, cfg map[string]string) string {
	switch strings.TrimSpace(providerKey) {
	case payment.TypeEasyPay, payment.TypeStripe, payment.TypeAirwallex:
		currency, err := payment.NormalizePaymentCurrency(cfg["currency"])
		if err == nil {
			return currency
		}
	}
	return payment.DefaultPaymentCurrency
}

func PaymentOrderCurrency(order *dbent.PaymentOrder) string {
	// list_amount is populated for every order created after the coupon
	// settlement rollout. Older rows received the migration default for
	// payment_currency, so their provider snapshot remains the source of truth.
	if order != nil && order.ListAmount > 0 {
		if currency, err := payment.NormalizePaymentCurrency(order.PaymentCurrency); err == nil && order.PaymentCurrency != "" {
			return currency
		}
	}
	if snapshot := psOrderProviderSnapshot(order); snapshot != nil {
		if currency, err := payment.NormalizePaymentCurrency(snapshot.Currency); err == nil {
			return currency
		}
	}
	if order != nil {
		if currency, err := payment.NormalizePaymentCurrency(order.PaymentCurrency); err == nil && order.PaymentCurrency != "" {
			return currency
		}
	}
	return payment.DefaultPaymentCurrency
}
