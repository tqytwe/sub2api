package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExpireAvailableUserCouponsPersistsExpiredWalletItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE user_coupons.*SET status = 'expired', updated_at = \$1.*expires_at <= \$1.*user_id = \$2`).
		WithArgs(now, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &couponRepository{db: db}
	err = repo.ExpireAvailableUserCoupons(context.Background(), 7, now)

	require.NoError(t, err)
}

func TestLockUserCouponForOrderRechecksValidityAtUpdate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	validFrom := now.Add(-time.Hour)
	expiresAt := now.Add(time.Hour)
	terms := []byte(`{"template_id":9,"template_version":1,"template_key":"wallet-lock","name":"Wallet lock","benefit_type":"fixed_amount","benefit_value":1,"currency":"CNY","applicable_scopes":["balance"],"minimum_order_amount":0,"eligible_plan_ids":[],"validity_mode":"relative_days","validity_days":1,"rules":{}}`)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM user_coupons uc.*WHERE uc\.id = \$1.*FOR UPDATE OF uc`).
		WithArgs(int64(41)).
		WillReturnRows(couponWalletMockRows(41, 9, 7, service.UserCouponStatusAvailable, terms, now, validFrom, expiresAt, nil, nil))
	mock.ExpectQuery(`(?s)UPDATE user_coupons.*status = 'locked'.*valid_from <= NOW\(\).*expires_at > NOW\(\).*RETURNING`).
		WithArgs(int64(41), int64(88), now).
		WillReturnRows(couponWalletMockRows(41, 9, 7, service.UserCouponStatusLocked, terms, now, validFrom, expiresAt, int64(88), now))
	mock.ExpectExec(`(?s)INSERT INTO coupon_events`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := &couponRepository{db: db}
	result, err := repo.LockUserCouponForOrder(context.Background(), service.CouponLockRequest{
		UserCouponID: 41,
		UserID:       7,
		OrderID:      88,
		OrderContext: service.CouponOrderContext{
			UserID:      7,
			Scope:       service.CouponScopeBalance,
			OrderAmount: 10,
			Currency:    "CNY",
			At:          now,
		},
		LockedAt: now,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, service.UserCouponStatusLocked, result.Coupon.Status)
}

func couponWalletMockRows(
	id, templateID, userID int64,
	status service.UserCouponStatus,
	terms []byte,
	issuedAt, validFrom, expiresAt time.Time,
	lockedOrder any,
	lockedAt any,
) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "template_id", "template_name", "user_id", "status",
		"terms_snapshot", "source", "source_ref", "issue_batch_id",
		"idempotency_key", "issued_at", "valid_from", "expires_at",
		"locked_order_id", "locked_at", "used_order_id", "used_at",
		"voided_at", "void_reason", "created_at", "updated_at",
	}).AddRow(
		id, templateID, "Wallet lock", userID, status,
		terms, service.CouponIssueSourceBlindbox, "blindbox:1", nil,
		"coupon:41", issuedAt, validFrom, expiresAt,
		lockedOrder, lockedAt, nil, nil, nil, "", issuedAt, issuedAt,
	)
}
