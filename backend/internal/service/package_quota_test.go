package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
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

func TestPackageEntitlementDisplayPriorityUsesCurrentPackage(t *testing.T) {
	now := time.Date(2026, 8, 17, 17, 0, 0, 0, time.UTC)
	requestLimit := int64(10)

	exhausted := &PackageEntitlement{
		ExpiresAt:    now.AddDate(0, 0, 5),
		RequestLimit: &requestLimit,
		RequestUsed:  requestLimit,
	}
	active := &PackageEntitlement{
		ExpiresAt: now.AddDate(0, 0, 35),
	}
	expired := &PackageEntitlement{ExpiresAt: now.Add(-time.Second)}

	exhausted.Status, exhausted.ExhaustedReason = PackageQuotaState(exhausted, now)
	active.Status, active.ExhaustedReason = PackageQuotaState(active, now)
	expired.Status, expired.ExhaustedReason = PackageQuotaState(expired, now)

	require.Less(t, packageEntitlementDisplayPriority(active.Status), packageEntitlementDisplayPriority(exhausted.Status))
	require.Less(t, packageEntitlementDisplayPriority(exhausted.Status), packageEntitlementDisplayPriority(expired.Status))
	// A later active renewal is the package that runtime will consume next, so
	// the management page must not keep displaying the exhausted predecessor.
	require.Equal(t, PackageEntitlementActive, active.Status)
}

func TestAttachPackageEntitlementsUsesOneBatchQueryAndSelectsCurrentPackage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 8, 17, 17, 0, 0, 0, time.UTC)
	requestLimit := int64(10)
	mock.ExpectQuery(regexp.QuoteMeta("WITH selected_subscriptions (user_id, group_id) AS")).
		WithArgs(int64(451), int64(62)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "payment_order_id", "user_id", "group_id", "starts_at", "expires_at", "status", "exhausted_reason",
			"request_limit", "request_used", "amount_limit_usd", "amount_used_usd", "token_limit", "token_used",
		}).
			AddRow(1, 331, 451, 62, now.Add(-time.Hour), now.AddDate(0, 0, 5), "active", nil, requestLimit, requestLimit, nil, 0, nil, 0).
			AddRow(2, 333, 451, 62, now, now.AddDate(0, 0, 35), "active", nil, nil, 0, nil, 0, nil, 0))

	svc := &SubscriptionService{entClient: client, now: func() time.Time { return now }}
	subs := []UserSubscription{{UserID: 451, GroupID: 62}}

	require.NoError(t, svc.attachPackageEntitlements(context.Background(), subs))
	require.NotNil(t, subs[0].PackageEntitlement)
	require.Equal(t, int64(2), subs[0].PackageEntitlement.ID)
	require.Equal(t, PackageEntitlementActive, subs[0].PackageEntitlement.Status)
	require.NoError(t, mock.ExpectationsWereMet())
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
