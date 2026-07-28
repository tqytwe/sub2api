package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestHasIssuableCouponRewardEntryRequiresLiveEntryAndTemplate(t *testing.T) {
	t.Run("returns true for an active entry with an issuable template", func(t *testing.T) {
		db, mock := newCouponRepositoryMock(t)
		now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
		expectPublishedCouponRewardPool(mock, 17, service.CouponRewardActivityBlindbox, now)
		mock.ExpectQuery(`(?s)SELECT e\.id.*FROM coupon_reward_pool_entries e.*WHERE e\.pool_version_id = \$1`).
			WithArgs(int64(17)).
			WillReturnRows(sqlmock.NewRows(couponRewardPoolEntryMockColumns()).AddRow(
				int64(31), int64(17), int64(9), "Recharge 1 off", 10000, true,
				nil, nil, nil, int64(0), nil, 0,
			))
		mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1`).
			WithArgs(int64(9)).
			WillReturnRows(couponRewardTemplateMockRows(9, service.CouponTemplateStatusActive, now))

		repo := &couponRepository{db: db}
		ready, err := repo.HasIssuableCouponRewardEntry(context.Background(), service.CouponRewardActivityBlindbox, now)

		require.NoError(t, err)
		require.True(t, ready)
	})

	t.Run("does not report a published pool ready when every entry is exhausted", func(t *testing.T) {
		db, mock := newCouponRepositoryMock(t)
		now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
		stockCap := int64(1)
		expectPublishedCouponRewardPool(mock, 18, service.CouponRewardActivityQuiz, now)
		mock.ExpectQuery(`(?s)SELECT e\.id.*FROM coupon_reward_pool_entries e.*WHERE e\.pool_version_id = \$1`).
			WithArgs(int64(18)).
			WillReturnRows(sqlmock.NewRows(couponRewardPoolEntryMockColumns()).AddRow(
				int64(32), int64(18), int64(10), "Exhausted coupon", 10000, true,
				nil, nil, stockCap, stockCap, nil, 0,
			))

		repo := &couponRepository{db: db}
		ready, err := repo.HasIssuableCouponRewardEntry(context.Background(), service.CouponRewardActivityQuiz, now)

		require.NoError(t, err)
		require.False(t, ready)
	})
}

func newCouponRepositoryMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return db, mock
}

func expectPublishedCouponRewardPool(mock sqlmock.Sqlmock, id int64, activity service.CouponRewardActivity, now time.Time) {
	mock.ExpectQuery(`(?s)SELECT id FROM coupon_reward_pool_versions.*activity = \$1.*status = 'published'`).
		WithArgs(activity).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_reward_pool_versions WHERE id = \$1`).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows(couponRewardPoolMockColumns()).AddRow(
			id, activity, "pool-v1", service.CouponRewardPoolStatusPublished, 6000, 0, 4000, []byte(`{}`),
			int64(9), nil, nil, now, now, now,
		))
}

func couponRewardPoolMockColumns() []string {
	return []string{
		"id", "activity", "version", "status", "coupon_weight_bp", "redeem_code_weight_bp", "balance_weight_bp", "reward_config",
		"fallback_template_id", "created_by", "updated_by", "published_at", "created_at", "updated_at",
	}
}

func couponRewardPoolEntryMockColumns() []string {
	return []string{
		"id", "pool_version_id", "template_id", "template_name", "weight_bp", "enabled",
		"starts_at", "ends_at", "stock_cap", "issued_count", "per_user_issue_limit", "sort_order",
	}
}

func couponRewardTemplateMockRows(id int64, status service.CouponTemplateStatus, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "template_key", "version", "name", "description", "status",
		"benefit_type", "benefit_value", "max_discount_amount", "currency",
		"applicable_scopes", "minimum_order_amount", "eligible_plan_ids",
		"validity_mode", "validity_days", "fixed_expires_at", "valid_from",
		"total_issue_limit", "issued_count", "rules", "created_by", "updated_by",
		"created_at", "updated_at",
	}).AddRow(
		id, "pool-ready", 1, "Recharge 1 off", "", status,
		service.CouponBenefitTypeFixedAmount, 1.0, nil, "CNY",
		[]byte(`["balance"]`), 0.0, []byte(`[]`),
		service.CouponValidityModeRelativeDays, 1, nil, nil,
		nil, int64(0), []byte(`{}`), nil, nil,
		now, now,
	)
}
