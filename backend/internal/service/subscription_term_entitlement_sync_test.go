package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

var packageEntitlementTermColumns = []string{
	"id", "payment_order_id", "source_type", "granted_by", "plan_id", "user_id", "group_id",
	"starts_at", "expires_at", "status", "exhausted_reason", "request_limit", "request_used",
	"amount_limit_usd", "amount_used_usd", "token_limit", "token_used",
}

func newSubscriptionTermEntitlementTestService(t *testing.T, repo UserSubscriptionRepository, now time.Time) (*SubscriptionService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	service := NewSubscriptionService(nil, repo, nil, client, nil)
	service.now = func() time.Time { return now }
	t.Cleanup(service.Stop)
	return service, mock
}

func expectPackageEntitlementsForTermUpdate(mock sqlmock.Sqlmock, userID, groupID int64, rows *sqlmock.Rows) {
	mock.ExpectQuery(`(?s)SELECT id, payment_order_id, source_type, granted_by,.*FROM subscription_package_entitlements.*FOR UPDATE`).
		WithArgs(userID, groupID).
		WillReturnRows(rows)
}

func expectPackageEntitlementExpiryUpdate(mock sqlmock.Sqlmock, id int64, expiresAt time.Time, status, reason string) {
	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscription_package_entitlements")).
		WithArgs(id, expiresAt, status, reason).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestPackageEntitlementExpirySupportsArbitraryPlanDurations(t *testing.T) {
	start := time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC)
	for _, days := range []int{60, 90, 120} {
		t.Run(time.Duration(days*24).String(), func(t *testing.T) {
			require.Equal(t, start.AddDate(0, 0, days), packageEntitlementExpiry(start, days))
		})
	}
	require.Equal(t, MaxExpiresAt, packageEntitlementExpiry(start, MaxValidityDays))
}

func TestExtendSubscriptionAlignsSinglePackageEntitlementWithoutResettingQuota(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	oldExpiry := now.AddDate(0, 0, 30)
	newExpiry := oldExpiry.AddDate(0, 0, 30)
	amountLimit := 2000.0
	repo := &lockingRenewalRepo{current: UserSubscription{
		ID: 137, UserID: 451, GroupID: 63, StartsAt: now, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive,
	}}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 451, 63, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(14, 787, PackageEntitlementSourcePayment, nil, 22, 451, 63, now, oldExpiry, PackageEntitlementActive, "", nil, 0, amountLimit, 140.16, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 14, newExpiry, PackageEntitlementActive, "")
	mock.ExpectCommit()

	updated, err := service.ExtendSubscription(context.Background(), 137, 30)
	require.NoError(t, err)
	require.Equal(t, newExpiry, updated.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExtendSubscriptionOnlyExtendsTerminalQueuedEntitlement(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	firstExpiry := now.AddDate(0, 0, 30)
	oldExpiry := now.AddDate(0, 0, 90)
	newExpiry := now.AddDate(0, 0, 120)
	repo := &lockingRenewalRepo{current: UserSubscription{
		ID: 200, UserID: 88, GroupID: 11, StartsAt: now, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive,
	}}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 88, 11, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(21, 901, PackageEntitlementSourcePayment, nil, 2, 88, 11, now, firstExpiry, PackageEntitlementActive, "", nil, 0, 3000.0, 100.0, nil, 0).
		AddRow(22, 902, PackageEntitlementSourcePayment, nil, 2, 88, 11, now, oldExpiry, PackageEntitlementActive, "", nil, 0, 3000.0, 0.0, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 22, newExpiry, PackageEntitlementActive, "")
	mock.ExpectCommit()

	updated, err := service.ExtendSubscription(context.Background(), 200, 30)
	require.NoError(t, err)
	require.Equal(t, newExpiry, updated.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShortenSubscriptionClampsEveryEntitlementPastNewEnd(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	oldExpiry := now.AddDate(0, 0, 120)
	newExpiry := now.AddDate(0, 0, 20)
	repo := &lockingRenewalRepo{current: UserSubscription{
		ID: 201, UserID: 89, GroupID: 12, StartsAt: now, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive,
	}}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 89, 12, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(31, 911, PackageEntitlementSourcePayment, nil, 3, 89, 12, now, now.AddDate(0, 0, 60), PackageEntitlementActive, "", nil, 0, 3000.0, 100.0, nil, 0).
		AddRow(32, 912, PackageEntitlementSourcePayment, nil, 3, 89, 12, now, oldExpiry, PackageEntitlementActive, "", nil, 0, 3000.0, 0.0, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 31, newExpiry, PackageEntitlementActive, "")
	expectPackageEntitlementExpiryUpdate(mock, 32, newExpiry, PackageEntitlementActive, "")
	mock.ExpectCommit()

	updated, err := service.ExtendSubscription(context.Background(), 201, -100)
	require.NoError(t, err)
	require.Equal(t, newExpiry, updated.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExtendSubscriptionRollsBackWhenPackageEntitlementUpdateFails(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	oldExpiry := now.AddDate(0, 0, 30)
	repo := &transactionalBulkSubscriptionRepo{
		committed: UserSubscription{ID: 137, UserID: 451, GroupID: 63, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive},
		pending:   make(map[*dbent.Tx]*UserSubscription),
		reads:     make(map[*dbent.Tx]int),
	}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 451, 63, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(14, 787, PackageEntitlementSourcePayment, nil, 22, 451, 63, now, oldExpiry, PackageEntitlementActive, "", nil, 0, 2000.0, 140.16, nil, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscription_package_entitlements")).
		WithArgs(int64(14), oldExpiry.AddDate(0, 0, 30), PackageEntitlementActive, "").
		WillReturnError(errors.New("package update failed"))
	mock.ExpectRollback()

	_, err := service.ExtendSubscription(context.Background(), 137, 30)
	require.ErrorContains(t, err, "package update failed")
	require.Equal(t, oldExpiry, repo.committed.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExtendExpiredSubscriptionReactivatesRemainingPackageQuota(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	oldExpiry := now.Add(-time.Hour)
	newExpiry := now.AddDate(0, 0, 60)
	repo := &lockingRenewalRepo{current: UserSubscription{
		ID: 300, UserID: 90, GroupID: 13, StartsAt: now.AddDate(0, 0, -30), ExpiresAt: oldExpiry, Status: SubscriptionStatusExpired,
	}}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 90, 13, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(41, 921, PackageEntitlementSourcePayment, nil, 4, 90, 13, now.AddDate(0, 0, -30), oldExpiry, PackageEntitlementExpired, "", nil, 0, 3000.0, 100.0, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 41, newExpiry, PackageEntitlementActive, "")
	mock.ExpectCommit()

	updated, err := service.ExtendSubscription(context.Background(), 300, 60)
	require.NoError(t, err)
	require.Equal(t, newExpiry, updated.ExpiresAt)
	require.Equal(t, SubscriptionStatusActive, updated.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExtendSubscriptionKeepsExhaustedPackageExhausted(t *testing.T) {
	now := time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC)
	oldExpiry := now.AddDate(0, 0, 60)
	newExpiry := now.AddDate(0, 0, 90)
	requestLimit := int64(1000)
	repo := &lockingRenewalRepo{current: UserSubscription{
		ID: 302, UserID: 92, GroupID: 15, StartsAt: now, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive,
	}}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)

	mock.ExpectBegin()
	expectPackageEntitlementsForTermUpdate(mock, 92, 15, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(42, 922, PackageEntitlementSourcePayment, nil, 6, 92, 15, now, oldExpiry, PackageEntitlementExhausted, PackageExhaustedByRequest, requestLimit, requestLimit, nil, 0, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 42, newExpiry, PackageEntitlementExhausted, PackageExhaustedByRequest)
	mock.ExpectCommit()

	updated, err := service.ExtendSubscription(context.Background(), 302, 30)
	require.NoError(t, err)
	require.Equal(t, newExpiry, updated.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedeemReductionClampsPackageEntitlementWithoutResettingQuota(t *testing.T) {
	now := time.Now()
	oldExpiry := now.AddDate(0, 0, 90)
	newExpiry := oldExpiry.AddDate(0, 0, -30)
	subscription := UserSubscription{
		ID: 301, UserID: 91, GroupID: 14, StartsAt: now, ExpiresAt: oldExpiry, Status: SubscriptionStatusActive,
	}
	repo := &lockingRenewalRepo{stale: subscription, current: subscription}
	service, mock := newSubscriptionTermEntitlementTestService(t, repo, now)
	redeemService := NewRedeemService(nil, nil, service, nil, nil, nil, nil, nil, nil)

	expectPackageEntitlementsForTermUpdate(mock, 91, 14, sqlmock.NewRows(packageEntitlementTermColumns).
		AddRow(51, 931, PackageEntitlementSourcePayment, nil, 5, 91, 14, now, oldExpiry, PackageEntitlementActive, "", nil, 0, 3000.0, 250.0, nil, 0))
	expectPackageEntitlementExpiryUpdate(mock, 51, newExpiry, PackageEntitlementActive, "")

	err := redeemService.reduceOrCancelSubscription(context.Background(), 91, 14, 30, "REFUND-CODE")
	require.NoError(t, err)
	require.WithinDuration(t, newExpiry, repo.current.ExpiresAt, time.Second)
	require.NoError(t, mock.ExpectationsWereMet())
}
