package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestCalculateCouponAdjustedPaymentSettlementAppliesDiscountBeforeFee(t *testing.T) {
	t.Parallel()

	settlement, err := calculateCouponAdjustedPaymentSettlement(paymentCouponSettlementInput{
		ListAmount: 100,
		FeeRate:    2.5,
		Currency:   payment.DefaultPaymentCurrency,
		CouponQuote: &CouponQuote{
			UserCouponID:   41,
			TemplateID:     9,
			OriginalAmount: 100,
			DiscountAmount: 20,
			PayableAmount:  80,
			Currency:       payment.DefaultPaymentCurrency,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 100.0, settlement.ListAmount)
	require.Equal(t, 20.0, settlement.DiscountAmount)
	require.Equal(t, 80.0, settlement.GatewayBaseAmount)
	require.Equal(t, 2.0, settlement.FeeAmount)
	require.Equal(t, 82.0, settlement.PayAmount)
	require.Equal(t, "82.00", settlement.PayAmountText)
	// The recharge balance remains the original product entitlement; revenue,
	// VIP and affiliate attribution only count the cash-equivalent base.
	require.Equal(t, 80.0, settlement.QualifyingRechargeAmount)
}

func TestCalculateCouponAdjustedPaymentSettlementAllowsFullyDiscountedOrder(t *testing.T) {
	t.Parallel()

	settlement, err := calculateCouponAdjustedPaymentSettlement(paymentCouponSettlementInput{
		ListAmount: 10,
		FeeRate:    2.5,
		Currency:   payment.DefaultPaymentCurrency,
		CouponQuote: &CouponQuote{
			UserCouponID:   41,
			TemplateID:     9,
			OriginalAmount: 10,
			DiscountAmount: 10,
			PayableAmount:  0,
			Currency:       payment.DefaultPaymentCurrency,
		},
	})
	require.NoError(t, err)
	require.Zero(t, settlement.GatewayBaseAmount)
	require.Zero(t, settlement.FeeAmount)
	require.Zero(t, settlement.PayAmount)
	require.Equal(t, "0.00", settlement.PayAmountText)
	require.Zero(t, settlement.QualifyingRechargeAmount)
}

func TestPaymentCouponSnapshotCapturesIssuedCouponTerms(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	coupon := UserCoupon{
		ID:         41,
		TemplateID: 9,
		UserID:     7,
		ValidFrom:  now,
		ExpiresAt:  now.Add(72 * time.Hour),
		TermsSnapshot: CouponTermsSnapshot{
			TemplateID:         9,
			TemplateVersion:    2,
			TemplateKey:        "recharge-10-off-1",
			Name:               "Recharge 10 off 1",
			BenefitType:        CouponBenefitTypeFixedAmount,
			BenefitValue:       1,
			Currency:           payment.DefaultPaymentCurrency,
			ApplicableScopes:   []CouponScope{CouponScopeBalance},
			MinimumOrderAmount: 10,
			ValidityMode:       CouponValidityModeRelativeDays,
			ValidityDays:       3,
		},
	}
	quote := CouponQuote{UserCouponID: 41, TemplateID: 9, OriginalAmount: 10, DiscountAmount: 1, PayableAmount: 9, Currency: payment.DefaultPaymentCurrency}

	snapshot := paymentCouponSnapshot(coupon, quote)
	require.Equal(t, 1, snapshot["schema_version"])
	require.Equal(t, int64(41), snapshot["user_coupon_id"])
	require.Equal(t, int64(9), snapshot["template_id"])
	require.Equal(t, "Recharge 10 off 1", snapshot["name"])
	require.Equal(t, 1.0, snapshot["discount_amount"])
	require.Equal(t, now.Add(72*time.Hour), snapshot["expires_at"])
}
