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

type txAwarePackageGrantRepo struct {
	UserSubscriptionRepository
	base          *subscriptionUserSubRepoStub
	transactional bool
}

func (r *txAwarePackageGrantRepo) GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	r.transactional = r.transactional || dbent.TxFromContext(ctx) != nil
	return r.base.GetByUserIDAndGroupID(ctx, userID, groupID)
}

func (r *txAwarePackageGrantRepo) Create(ctx context.Context, sub *UserSubscription) error {
	r.transactional = r.transactional || dbent.TxFromContext(ctx) != nil
	return r.base.Create(ctx, sub)
}

func (r *txAwarePackageGrantRepo) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	return r.base.GetByID(ctx, id)
}

func packageEntitlementColumns() []string {
	return []string{
		"id", "payment_order_id", "source_type", "granted_by", "plan_id", "user_id", "group_id", "starts_at", "expires_at",
		"status", "exhausted_reason", "request_limit", "request_used", "amount_limit_usd", "amount_used_usd", "token_limit", "token_used",
	}
}

func TestGrantPackagePlanCreatesSubscriptionAndEntitlementInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	requestLimit := int64(30_000)
	amountLimit := 2_000.0
	tokenLimit := int64(2_200_000_000)
	plan := &dbent.SubscriptionPlan{
		ID: 22, GroupID: 63, ValidityDays: 1, ValidityUnit: "month",
		RequestLimit: &requestLimit, AmountLimitUsd: &amountLimit, TokenLimit: &tokenLimit,
	}
	baseRepo := newSubscriptionUserSubRepoStub()
	repo := &txAwarePackageGrantRepo{UserSubscriptionRepository: baseRepo, base: baseRepo}
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 63, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, client, nil)
	svc.now = func() time.Time { return now }
	t.Cleanup(svc.Stop)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("WHERE grant_key = $1")).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(packageEntitlementColumns()))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT MAX(expires_at)")).
		WithArgs(int64(451), int64(63)).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(nil))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO subscription_package_entitlements")).
		WithArgs(int64(451), int64(63), now, now.AddDate(0, 0, 30), requestLimit, amountLimit, tokenLimit, sqlmock.AnyArg(), int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(14, 1))
	mock.ExpectQuery(regexp.QuoteMeta("WHERE grant_key = $1")).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(packageEntitlementColumns()).AddRow(
			14, nil, PackageEntitlementSourceAdminGrant, 7, 22, 451, 63, now, now.AddDate(0, 0, 30),
			PackageEntitlementActive, nil, requestLimit, 0, amountLimit, 0, tokenLimit, 0,
		))
	mock.ExpectCommit()

	subscription, replayed, err := svc.grantPackagePlan(context.Background(), &PackagePlanGrantInput{
		UserID: 451, PlanID: 22, GrantedBy: 7, Notes: "support grant", IdempotencyKey: "grant-451-plan-22",
	}, plan)
	require.NoError(t, err)
	require.False(t, replayed)
	require.True(t, repo.transactional)
	require.Equal(t, now.AddDate(0, 0, 30), subscription.ExpiresAt)
	require.NotNil(t, subscription.PackageEntitlement)
	require.Equal(t, PackageEntitlementSourceAdminGrant, subscription.PackageEntitlement.SourceType)
	require.Equal(t, requestLimit, *subscription.PackageEntitlement.RequestLimit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantPackagePlanReplayDoesNotExtendSubscriptionAgain(t *testing.T) {
	requestLimit := int64(100)
	plan := &dbent.SubscriptionPlan{ID: 4, GroupID: 8, ValidityDays: 30, ValidityUnit: "day", RequestLimit: &requestLimit}
	grantKey := packageGrantKey(9, "same-operation", 42)
	now := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	baseRepo := newSubscriptionUserSubRepoStub()
	baseRepo.seed(&UserSubscription{ID: 3, UserID: 42, GroupID: 8, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 30), Status: SubscriptionStatusActive})
	svc := NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 8, SubscriptionType: SubscriptionTypeSubscription}}, baseRepo, nil, client, nil)
	svc.now = func() time.Time { return now }
	t.Cleanup(svc.Stop)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("WHERE grant_key = $1")).WithArgs(grantKey).
		WillReturnRows(sqlmock.NewRows(packageEntitlementColumns()).AddRow(
			5, nil, PackageEntitlementSourceAdminGrant, 9, 4, 42, 8, now, now.AddDate(0, 0, 30),
			PackageEntitlementActive, nil, requestLimit, 0, nil, 0, nil, 0,
		))
	mock.ExpectCommit()

	subscription, replayed, err := svc.grantPackagePlan(context.Background(), &PackagePlanGrantInput{
		UserID: 42, PlanID: 4, GrantedBy: 9, IdempotencyKey: "same-operation",
	}, plan)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, now.AddDate(0, 0, 30), subscription.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantPackagePlanRejectsLegacyPlanWithoutPackageQuota(t *testing.T) {
	svc := &SubscriptionService{}
	_, _, err := svc.grantPackagePlan(context.Background(), &PackagePlanGrantInput{
		UserID: 42, PlanID: 4, GrantedBy: 9, IdempotencyKey: "legacy-plan",
	}, &dbent.SubscriptionPlan{ID: 4, GroupID: 8, ValidityDays: 30, ValidityUnit: "day"})
	require.ErrorIs(t, err, ErrPackagePlanHasNoQuota)
}
