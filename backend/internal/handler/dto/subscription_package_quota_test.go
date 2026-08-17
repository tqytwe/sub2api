package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionFromServiceAdminIncludesPackageEntitlement(t *testing.T) {
	requestLimit := int64(12000)
	amountLimit := 700.0
	tokenLimit := int64(1_000_000_000)
	expiresAt := time.Date(2026, 9, 16, 17, 19, 0, 0, time.UTC)

	out := UserSubscriptionFromServiceAdmin(&service.UserSubscription{
		ID: 42,
		PackageEntitlement: &service.PackageEntitlement{
			ID:             9,
			PaymentOrderID: 331,
			ExpiresAt:      expiresAt,
			Status:         service.PackageEntitlementActive,
			RequestLimit:   &requestLimit,
			RequestUsed:    25,
			AmountLimitUSD: &amountLimit,
			AmountUsedUSD:  12.5,
			TokenLimit:     &tokenLimit,
			TokenUsed:      456,
		},
	})

	require.NotNil(t, out.PackageEntitlement)
	require.Equal(t, int64(9), out.PackageEntitlement.ID)
	require.Equal(t, int64(331), out.PackageEntitlement.PaymentOrderID)
	require.Equal(t, expiresAt, out.PackageEntitlement.ExpiresAt)
	require.Equal(t, service.PackageEntitlementActive, out.PackageEntitlement.Status)
	require.Equal(t, requestLimit, *out.PackageEntitlement.RequestLimit)
	require.Equal(t, int64(25), out.PackageEntitlement.RequestUsed)
	require.Equal(t, amountLimit, *out.PackageEntitlement.AmountLimitUSD)
	require.Equal(t, 12.5, out.PackageEntitlement.AmountUsedUSD)
	require.Equal(t, tokenLimit, *out.PackageEntitlement.TokenLimit)
	require.Equal(t, int64(456), out.PackageEntitlement.TokenUsed)
}
