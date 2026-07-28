package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPublishCouponRewardPoolRechecksFallbackAfterTemplateLock(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	limit := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_reward_pool_versions WHERE id = \$1 FOR UPDATE`).
		WithArgs(int64(71)).
		WillReturnRows(sqlmock.NewRows(couponRewardPoolMockColumns()).AddRow(
			int64(71), service.CouponRewardActivityBlindbox, "blindbox-safe-v1", service.CouponRewardPoolStatusDraft,
			6000, 4000, int64(9), nil, nil, nil, now, now,
		))
	mock.ExpectQuery(`(?s)SELECT e\.id.*FROM coupon_reward_pool_entries e.*WHERE e\.pool_version_id = \$1.*FOR UPDATE OF e`).
		WithArgs(int64(71)).
		WillReturnRows(sqlmock.NewRows(couponRewardPoolEntryMockColumns()).
			AddRow(int64(31), int64(71), int64(9), "Fallback", 1, true, nil, nil, nil, int64(0), nil, 0).
			AddRow(int64(32), int64(71), int64(10), "Ordinary", 10000, true, nil, nil, nil, int64(0), nil, 1))
	mock.ExpectQuery(`(?s)SELECT id FROM coupon_reward_pool_versions.*activity = \$1.*status = 'published'.*FOR UPDATE`).
		WithArgs(service.CouponRewardActivityBlindbox).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(9)).
		WillReturnRows(couponRewardTemplateRowsWithLimit(9, service.CouponTemplateStatusActive, now, &limit))
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(10)).
		WillReturnRows(couponRewardTemplateMockRows(10, service.CouponTemplateStatusActive, now))
	mock.ExpectRollback()

	repo := &couponRepository{db: db}
	_, err := repo.PublishCouponRewardPool(context.Background(), 71, 17, now)
	require.ErrorContains(t, err, "fallback template cannot set a total issue limit")
}

func TestUpdateCouponTemplateRechecksPublishedFallbackUnderTemplateLock(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	limit := int64(100)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(9)).
		WillReturnRows(couponRewardTemplateMockRows(9, service.CouponTemplateStatusActive, now))
	mock.ExpectQuery(`(?s)SELECT EXISTS\(.*FROM coupon_reward_pool_versions.*status = 'published'.*fallback_template_id = \$1`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	repo := &couponRepository{db: db}
	_, err := repo.UpdateCouponTemplate(context.Background(), service.CouponTemplate{
		ID:               9,
		Key:              "fallback-coupon",
		Version:          1,
		Name:             "Fallback coupon",
		Status:           service.CouponTemplateStatusActive,
		BenefitType:      service.CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []service.CouponScope{service.CouponScopeBalance},
		ValidityMode:     service.CouponValidityModeRelativeDays,
		ValidityDays:     3,
		TotalIssueLimit:  &limit,
	})
	require.ErrorContains(t, err, "published coupon pool fallback templates must remain active and unbounded")
}

func TestUpdateCouponTemplateRechecksIssuedTermsUnderTemplateLock(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(9)).
		WillReturnRows(couponRewardTemplateRowsWithLimitAndIssuedCount(9, service.CouponTemplateStatusActive, now, nil, 1))
	mock.ExpectRollback()

	repo := &couponRepository{db: db}
	_, err := repo.UpdateCouponTemplate(context.Background(), service.CouponTemplate{
		ID:               9,
		Key:              "fallback-coupon",
		Version:          2,
		Name:             "Fallback coupon",
		Status:           service.CouponTemplateStatusActive,
		BenefitType:      service.CouponBenefitTypeFixedAmount,
		BenefitValue:     2,
		Currency:         "CNY",
		ApplicableScopes: []service.CouponScope{service.CouponScopeBalance},
		ValidityMode:     service.CouponValidityModeRelativeDays,
		ValidityDays:     3,
	})
	require.ErrorContains(t, err, "issued coupon terms cannot be changed")
}

func TestUpdateCouponTemplateRechecksIssueLimitUnderTemplateLock(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	limit := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(9)).
		WillReturnRows(couponRewardTemplateRowsWithLimitAndIssuedCount(9, service.CouponTemplateStatusActive, now, nil, 2))
	mock.ExpectRollback()

	repo := &couponRepository{db: db}
	_, err := repo.UpdateCouponTemplate(context.Background(), service.CouponTemplate{
		ID:               9,
		Key:              "fallback-coupon",
		Version:          1,
		Name:             "Fallback coupon",
		Status:           service.CouponTemplateStatusActive,
		BenefitType:      service.CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []service.CouponScope{service.CouponScopeBalance},
		ValidityMode:     service.CouponValidityModeRelativeDays,
		ValidityDays:     3,
		TotalIssueLimit:  &limit,
	})
	require.ErrorContains(t, err, "coupon issue limit cannot be lower than already issued coupons")
}

func couponRewardTemplateRowsWithLimit(id int64, status service.CouponTemplateStatus, now time.Time, limit *int64) *sqlmock.Rows {
	return couponRewardTemplateRowsWithLimitAndIssuedCount(id, status, now, limit, 0)
}

func couponRewardTemplateRowsWithLimitAndIssuedCount(id int64, status service.CouponTemplateStatus, now time.Time, limit *int64, issuedCount int64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "template_key", "version", "name", "description", "status",
		"benefit_type", "benefit_value", "max_discount_amount", "currency",
		"applicable_scopes", "minimum_order_amount", "eligible_plan_ids",
		"validity_mode", "validity_days", "fixed_expires_at", "valid_from",
		"total_issue_limit", "issued_count", "rules", "created_by", "updated_by",
		"created_at", "updated_at",
	}).AddRow(
		id, "fallback-coupon", 1, "Fallback coupon", "", status,
		service.CouponBenefitTypeFixedAmount, 1.0, nil, "CNY",
		[]byte(`["balance"]`), 0.0, []byte(`[]`),
		service.CouponValidityModeRelativeDays, 3, nil, nil,
		limit, issuedCount, []byte(`{}`), nil, nil,
		now, now,
	)
}
