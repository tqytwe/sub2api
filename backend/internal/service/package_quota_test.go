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

type packageQuotaListRepo struct {
	userSubRepoNoop
	subscriptions []UserSubscription
}

func (r *packageQuotaListRepo) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.subscriptions...), nil
}

func (r *packageQuotaListRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.subscriptions...), nil
}

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
			"id", "payment_order_id", "source_type", "granted_by", "plan_id", "user_id", "group_id", "starts_at", "expires_at", "status", "exhausted_reason",
			"request_limit", "request_used", "amount_limit_usd", "amount_used_usd", "token_limit", "token_used",
		}).
			AddRow(1, 331, PackageEntitlementSourcePayment, nil, 21, 451, 62, now.Add(-time.Hour), now.AddDate(0, 0, 5), "active", nil, requestLimit, requestLimit, nil, 0, nil, 0).
			AddRow(2, 333, PackageEntitlementSourcePayment, nil, 21, 451, 62, now, now.AddDate(0, 0, 35), "active", nil, nil, 0, nil, 0, nil, 0))

	svc := &SubscriptionService{entClient: client, now: func() time.Time { return now }}
	subs := []UserSubscription{{UserID: 451, GroupID: 62}}

	require.NoError(t, svc.attachPackageEntitlements(context.Background(), subs))
	require.NotNil(t, subs[0].PackageEntitlement)
	require.Equal(t, int64(2), subs[0].PackageEntitlement.ID)
	require.Equal(t, PackageEntitlementActive, subs[0].PackageEntitlement.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSubscriptionListsAttachPackageEntitlements(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 9, 27, 17, 0, 0, 0, time.UTC)
	requestLimit := int64(30000)
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "payment_order_id", "source_type", "granted_by", "plan_id", "user_id", "group_id", "starts_at", "expires_at", "status", "exhausted_reason",
			"request_limit", "request_used", "amount_limit_usd", "amount_used_usd", "token_limit", "token_used",
		}).AddRow(14, 787, PackageEntitlementSourcePayment, nil, 22, 451, 63, now.Add(-time.Hour), now.AddDate(0, 0, 30), "active", nil, requestLimit, 2022, nil, 0, nil, 0)
	}
	for range 2 {
		mock.ExpectQuery(regexp.QuoteMeta("WITH selected_subscriptions (user_id, group_id) AS")).
			WithArgs(int64(451), int64(63)).
			WillReturnRows(rows())
	}
	repo := &packageQuotaListRepo{subscriptions: []UserSubscription{{
		ID: 137, UserID: 451, GroupID: 63,
		Status: SubscriptionStatusActive, ExpiresAt: now.AddDate(0, 0, 30),
	}}}
	svc := NewSubscriptionService(nil, repo, nil, client, nil)
	svc.now = func() time.Time { return now }
	t.Cleanup(svc.Stop)

	all, err := svc.ListUserSubscriptions(context.Background(), 451)
	require.NoError(t, err)
	require.NotNil(t, all[0].PackageEntitlement)
	require.Equal(t, int64(2022), all[0].PackageEntitlement.RequestUsed)

	active, err := svc.ListActiveUserSubscriptions(context.Background(), 451)
	require.NoError(t, err)
	require.NotNil(t, active[0].PackageEntitlement)
	require.Equal(t, int64(2022), active[0].PackageEntitlement.RequestUsed)
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

func TestSubscriptionPlanSnapshotIsSharedAndImmutable(t *testing.T) {
	requestLimit := int64(30000)
	amountLimit := 2000.0
	tokenLimit := int64(2_200_000_000)
	plan := &dbent.SubscriptionPlan{
		ID:             22,
		GroupID:        63,
		ValidityDays:   1,
		ValidityUnit:   "month",
		RequestLimit:   &requestLimit,
		AmountLimitUsd: &amountLimit,
		TokenLimit:     &tokenLimit,
	}

	snapshot := subscriptionPlanSnapshot(plan)
	require.Equal(t, int64(22), snapshot["plan_id"])
	require.Equal(t, int64(63), snapshot["group_id"])
	require.Equal(t, 30, snapshot["validity_days"])
	require.Equal(t, requestLimit, snapshot["request_limit"])
	require.Equal(t, amountLimit, snapshot["amount_limit_usd"])
	require.Equal(t, tokenLimit, snapshot["token_limit"])

	requestLimit = 1
	amountLimit = 1
	tokenLimit = 1
	require.Equal(t, int64(30000), snapshot["request_limit"])
	require.Equal(t, 2000.0, snapshot["amount_limit_usd"])
	require.Equal(t, int64(2_200_000_000), snapshot["token_limit"])
}

func TestPackageEntitlementExpiryExtendsFromExistingPackageEnd(t *testing.T) {
	startsAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	existingExpiry := startsAt.AddDate(0, 0, 30)

	require.Equal(t, existingExpiry.AddDate(0, 0, 30), packageEntitlementExpiry(existingExpiry, 30))
	require.Equal(t, startsAt.AddDate(0, 0, 30), packageEntitlementExpiry(startsAt, 30))
}
