package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCouponRewardEntryForDrawUsesRemainingEligibleWeight(t *testing.T) {
	entries := []service.CouponRewardPoolEntry{
		{ID: 10, TemplateID: 101, WeightBP: 2500, Enabled: true},
		{ID: 20, TemplateID: 102, WeightBP: 7500, Enabled: true},
	}

	assertEntryID := func(draw int, want int64) {
		t.Helper()
		entry := couponRewardEntryForDraw(entries, draw)
		require.NotNil(t, entry)
		require.Equal(t, want, entry.ID)
	}
	assertEntryID(0, 10)
	assertEntryID(2499, 10)
	assertEntryID(2500, 20)
	assertEntryID(9999, 20)
	require.Nil(t, couponRewardEntryForDraw(entries, -1))
	require.Nil(t, couponRewardEntryForDraw(entries, 10_000))

	// Removing an ineligible entry makes the remaining entry cover its own
	// 7,500-point range, rather than routing the unavailable 2,500 points to
	// the fallback coupon.
	eligible := couponRewardEntriesWithoutID(entries, 10)
	require.Equal(t, 7500, couponRewardEntryWeightTotal(eligible))
	entry := couponRewardEntryForDraw(eligible, 7499)
	require.NotNil(t, entry)
	require.Equal(t, int64(20), entry.ID)
	require.Nil(t, couponRewardEntryForDraw(eligible, 7500))
}

func TestCouponRewardOrdinaryEntriesExcludeFallbackBeforeTheDraw(t *testing.T) {
	entries := []service.CouponRewardPoolEntry{
		{ID: 10, TemplateID: 101, WeightBP: 1, Enabled: true},
		{ID: 20, TemplateID: 102, WeightBP: 10_000, Enabled: true},
	}

	ordinary := couponRewardOrdinaryEntries(entries, 101)
	require.Len(t, ordinary, 1)
	require.Equal(t, int64(20), ordinary[0].ID)
	require.Equal(t, 10_000, couponRewardEntryWeightTotal(ordinary))

	entry := couponRewardEntryForDraw(ordinary, 9_999)
	require.NotNil(t, entry)
	require.Equal(t, int64(20), entry.ID)
}

func TestCouponRewardPoolTemplateLocksUseStableOrder(t *testing.T) {
	t.Parallel()

	pool := service.CouponRewardPoolVersion{
		FallbackTemplateID: 7,
		Entries: []service.CouponRewardPoolEntry{
			{TemplateID: 19},
			{TemplateID: 7},
			{TemplateID: 3},
		},
	}

	require.Equal(t, []int64{3, 7, 19}, couponPoolTemplateIDs(pool))
}

func TestCouponRewardGlobalReadinessRejectsUnavailableEntries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	stockCap := int64(1)
	entry := service.CouponRewardPoolEntry{
		TemplateID:  9,
		Enabled:     true,
		StockCap:    &stockCap,
		IssuedCount: 1,
	}
	require.False(t, couponRewardEntryWindowHasCapacity(entry, now))

	entry.IssuedCount = 0
	require.True(t, couponRewardEntryWindowHasCapacity(entry, now))

	template := service.CouponTemplate{
		ID:               9,
		Key:              "ready-coupon",
		Name:             "Ready coupon",
		Status:           service.CouponTemplateStatusActive,
		BenefitType:      service.CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []service.CouponScope{service.CouponScopeBalance},
		ValidityMode:     service.CouponValidityModeRelativeDays,
		ValidityDays:     1,
	}
	require.True(t, couponRewardTemplateIssuable(&template, now))

	template.Status = service.CouponTemplateStatusPaused
	require.False(t, couponRewardTemplateIssuable(&template, now))
	template.Status = service.CouponTemplateStatusActive
	template.ValidityMode = service.CouponValidityModeFixed
	template.ValidityDays = 0
	past := now.Add(-time.Minute)
	template.FixedExpiresAt = &past
	require.False(t, couponRewardTemplateIssuable(&template, now))
}

func TestDrawUsableCouponRewardEntryUsesUnboundedFallbackAfterUserCapsNormalEntries(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	perUserLimit := 1
	entries := []service.CouponRewardPoolEntry{
		{
			ID:                11,
			TemplateID:        101,
			WeightBP:          7000,
			Enabled:           true,
			PerUserIssueLimit: &perUserLimit,
		},
		{
			ID:         22,
			TemplateID: 202,
			WeightBP:   1,
			Enabled:    true,
		},
	}

	// The first entry is capped for this user. The fallback has no user cap,
	// no stock cap, and no activity window, so it is the only issuable result.
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM coupon_reward_draws.*pool_entry_id = \$1 AND user_id = \$2`).
		WithArgs(int64(11), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(202)).
		WillReturnRows(couponRewardTemplateMockRows(202, service.CouponTemplateStatusActive, now))

	entry, template, fallbackUsed, err := drawUsableCouponRewardEntry(
		context.Background(), db, entries, 202, 42, now,
	)

	require.NoError(t, err)
	require.NotNil(t, entry)
	require.NotNil(t, template)
	require.Equal(t, int64(22), entry.ID)
	require.Equal(t, int64(202), template.ID)
	require.True(t, fallbackUsed)
}

func TestDrawUsableCouponRewardEntryDoesNotReadFallbackWhileAnOrdinaryCouponCanIssue(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	entries := []service.CouponRewardPoolEntry{
		{ID: 11, TemplateID: 101, WeightBP: 10_000, Enabled: true},
		{ID: 22, TemplateID: 202, WeightBP: 1, Enabled: true},
	}

	// The fallback is intentionally absent from the expected queries. It may
	// only be examined after every ordinary entry is found unavailable.
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1$`).
		WithArgs(int64(101)).
		WillReturnRows(couponRewardTemplateMockRows(101, service.CouponTemplateStatusActive, now))
	mock.ExpectQuery(`(?s)SELECT .*FROM coupon_templates WHERE id = \$1 FOR UPDATE$`).
		WithArgs(int64(101)).
		WillReturnRows(couponRewardTemplateMockRows(101, service.CouponTemplateStatusActive, now))

	entry, template, fallbackUsed, err := drawUsableCouponRewardEntry(
		context.Background(), db, entries, 202, 42, now,
	)

	require.NoError(t, err)
	require.NotNil(t, entry)
	require.NotNil(t, template)
	require.Equal(t, int64(11), entry.ID)
	require.Equal(t, int64(101), template.ID)
	require.False(t, fallbackUsed)
}

func TestFindCouponRewardIssueByIdempotencyReturnsTheIssuedCouponSnapshot(t *testing.T) {
	db, mock := newCouponRepositoryMock(t)
	issuedAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)FROM coupon_reward_draws d.*JOIN coupon_reward_pool_versions p.*JOIN user_coupons uc.*WHERE d\.idempotency_key = \$1$`).
		WithArgs("blindbox:42:replay-key").
		WillReturnRows(sqlmock.NewRows([]string{
			"pool_version_id", "pool_version", "pool_entry_id", "template_id", "user_coupon_id", "fallback_used",
			"coupon_id", "coupon_template_id", "template_name", "user_id", "coupon_status", "terms_snapshot",
			"source", "source_ref", "issue_batch_id", "coupon_idempotency_key", "issued_at", "valid_from", "expires_at",
			"locked_order_id", "locked_at", "used_order_id", "used_at", "voided_at", "void_reason", "created_at", "updated_at",
		}).AddRow(
			int64(401), "blindbox-coupon-v1", int64(501), int64(601), int64(701), false,
			int64(701), int64(601), "充值满10减1", int64(42), service.UserCouponStatusAvailable, []byte(`{"template_id":601,"name":"充值满10减1","benefit_type":"fixed_amount","benefit_value":1,"currency":"CNY","applicable_scopes":["balance"],"eligible_plan_ids":[],"validity_mode":"relative_days","validity_days":3,"rules":{}}`),
			service.CouponIssueSourceBlindbox, "2026-07-27", nil, "blindbox:42:replay-key", issuedAt, issuedAt, issuedAt.Add(72*time.Hour),
			nil, nil, nil, nil, nil, "", issuedAt, issuedAt,
		))

	repo := &couponRepository{db: db}
	result, err := repo.FindCouponRewardIssueByIdempotency(context.Background(), "blindbox:42:replay-key")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "blindbox-coupon-v1", result.PoolVersion)
	require.Equal(t, int64(701), result.UserCouponID)
	require.Equal(t, int64(42), result.Coupon.UserID)
	require.Equal(t, "充值满10减1", result.Coupon.TemplateName)
	require.Equal(t, issuedAt.Add(72*time.Hour), result.ExpiresAt)
}
