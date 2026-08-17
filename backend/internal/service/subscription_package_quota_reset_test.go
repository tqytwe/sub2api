//go:build unit

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

func TestAdminResetQuotaResetsOnlyCurrentPackageEntitlement(t *testing.T) {
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
			AddRow(1, 331, 451, 62, now.Add(-time.Hour), now.AddDate(0, 0, 5), "exhausted", "request", requestLimit, requestLimit, nil, 0, nil, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscription_package_entitlements")).
		WithArgs(int64(1), PackageEntitlementActive).
		WillReturnResult(sqlmock.NewResult(0, 1))

	stub := &resetQuotaUserSubRepoStub{sub: &UserSubscription{ID: 42, UserID: 451, GroupID: 62}}
	svc := NewSubscriptionService(groupRepoNoop{}, stub, nil, client, nil)
	svc.now = func() time.Time { return now }

	result, err := svc.AdminResetQuota(context.Background(), 42, true, true, true)
	require.NoError(t, err)
	require.NotNil(t, result.PackageEntitlement)
	require.Equal(t, int64(1), result.PackageEntitlement.ID)
	require.Equal(t, PackageEntitlementActive, result.PackageEntitlement.Status)
	require.Zero(t, result.PackageEntitlement.RequestUsed)
	require.Zero(t, result.PackageEntitlement.AmountUsedUSD)
	require.Zero(t, result.PackageEntitlement.TokenUsed)
	require.False(t, stub.resetDailyCalled)
	require.False(t, stub.resetWeeklyCalled)
	require.False(t, stub.resetMonthlyCalled)
	require.NoError(t, mock.ExpectationsWereMet())
}
