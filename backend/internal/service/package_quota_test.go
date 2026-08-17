package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPackageQuotaStateUsesAnyQuotaAsHardStop(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	requestLimit, tokenLimit := int64(10), int64(100)
	amountLimit := 7.0

	base := PackageEntitlement{
		StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
		RequestLimit: &requestLimit, AmountLimitUSD: &amountLimit, TokenLimit: &tokenLimit,
	}
	status, reason := PackageQuotaState(&base, now)
	require.Equal(t, PackageEntitlementActive, status)
	require.Empty(t, reason)

	base.RequestUsed = requestLimit
	status, reason = PackageQuotaState(&base, now)
	require.Equal(t, PackageEntitlementExhausted, status)
	require.Equal(t, PackageExhaustedByRequest, reason)

	base.RequestUsed = 0
	base.AmountUsedUSD = amountLimit
	status, reason = PackageQuotaState(&base, now)
	require.Equal(t, PackageEntitlementExhausted, status)
	require.Equal(t, PackageExhaustedByAmount, reason)

	base.AmountUsedUSD = 0
	base.TokenUsed = tokenLimit
	status, reason = PackageQuotaState(&base, now)
	require.Equal(t, PackageEntitlementExhausted, status)
	require.Equal(t, PackageExhaustedByToken, reason)
}

func TestPackageQuotaStateExpiresAtExactBoundary(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	status, reason := PackageQuotaState(&PackageEntitlement{ExpiresAt: now}, now)
	require.Equal(t, PackageEntitlementExpired, status)
	require.Empty(t, reason)
}

func TestPackageQuotaSnapshotReadsPaymentOrderPointerValues(t *testing.T) {
	requests, tokens := int64(10000), int64(100000000)
	amount := 700.0
	requestLimit, amountLimit, tokenLimit, ok := packageQuotaSnapshot(map[string]any{
		"request_limit":    &requests,
		"amount_limit_usd": &amount,
		"token_limit":      &tokens,
	})
	require.True(t, ok)
	require.Equal(t, requests, *requestLimit)
	require.Equal(t, amount, *amountLimit)
	require.Equal(t, tokens, *tokenLimit)
}

func TestPackageEntitlementExpiryExtendsFromExistingPackageEnd(t *testing.T) {
	startsAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	existingExpiry := startsAt.AddDate(0, 0, 30)

	require.Equal(t, existingExpiry.AddDate(0, 0, 30), packageEntitlementExpiry(existingExpiry, 30))
	require.Equal(t, startsAt.AddDate(0, 0, 30), packageEntitlementExpiry(startsAt, 30))
}
