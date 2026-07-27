package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type couponServiceUserCouponRepo struct {
	CouponRepository
	coupon *UserCoupon
}

func (r *couponServiceUserCouponRepo) GetUserCoupon(context.Context, int64) (*UserCoupon, error) {
	return r.coupon, nil
}

type couponServiceRewardReadinessRepo struct {
	CouponRepository
	ready    bool
	err      error
	activity CouponRewardActivity
	at       time.Time
}

func (r *couponServiceRewardReadinessRepo) HasIssuableCouponRewardEntry(_ context.Context, activity CouponRewardActivity, at time.Time) (bool, error) {
	r.activity = activity
	r.at = at
	return r.ready, r.err
}

type couponServiceListRepo struct {
	CouponRepository
	expireCalls int
	expireUser  int64
	expireAt    time.Time
}

func (r *couponServiceListRepo) ExpireAvailableUserCoupons(_ context.Context, userID int64, at time.Time) error {
	r.expireCalls++
	r.expireUser = userID
	r.expireAt = at
	return nil
}

func (r *couponServiceListRepo) ListUserCoupons(_ context.Context, _ UserCouponListFilter) ([]UserCoupon, int64, error) {
	return []UserCoupon{{ID: 41, Status: UserCouponStatusExpired}}, 1, nil
}

type couponServiceRewardPoolRepo struct {
	CouponRepository
	pool      *CouponRewardPoolVersion
	templates map[int64]*CouponTemplate
}

func (r *couponServiceRewardPoolRepo) GetPublishedCouponRewardPool(_ context.Context, _ CouponRewardActivity) (*CouponRewardPoolVersion, error) {
	return r.pool, nil
}

func (r *couponServiceRewardPoolRepo) GetCouponTemplate(_ context.Context, id int64) (*CouponTemplate, error) {
	return r.templates[id], nil
}

type couponServicePublishPoolRepo struct {
	CouponRepository
	pool      *CouponRewardPoolVersion
	templates map[int64]*CouponTemplate
	published bool
}

func (r *couponServicePublishPoolRepo) GetCouponRewardPool(_ context.Context, _ int64) (*CouponRewardPoolVersion, error) {
	return r.pool, nil
}

func (r *couponServicePublishPoolRepo) GetCouponTemplate(_ context.Context, id int64) (*CouponTemplate, error) {
	return r.templates[id], nil
}

func (r *couponServicePublishPoolRepo) PublishCouponRewardPool(_ context.Context, _ int64, _ int64, _ time.Time) (*CouponRewardPoolVersion, error) {
	r.published = true
	return r.pool, nil
}

type couponServiceFallbackTemplateRepo struct {
	CouponRepository
	template *CouponTemplate
	pools    []CouponRewardPoolVersion
	updated  bool
}

func (r *couponServiceFallbackTemplateRepo) GetCouponTemplate(_ context.Context, _ int64) (*CouponTemplate, error) {
	return r.template, nil
}

func (r *couponServiceFallbackTemplateRepo) ListCouponRewardPools(_ context.Context, _ CouponRewardActivity) ([]CouponRewardPoolVersion, error) {
	return r.pools, nil
}

func (r *couponServiceFallbackTemplateRepo) UpdateCouponTemplate(_ context.Context, template CouponTemplate) (*CouponTemplate, error) {
	r.updated = true
	r.template = &template
	return r.template, nil
}

type couponServiceDeleteRewardPoolRepo struct {
	CouponRepository
	pool       *CouponRewardPoolVersion
	deletedIDs []int64
}

type couponServiceRewardReplayRepo struct {
	CouponRepository
	result *CouponRewardIssueResult
}

func (r *couponServiceRewardReplayRepo) FindCouponRewardIssueByIdempotency(context.Context, string) (*CouponRewardIssueResult, error) {
	return r.result, nil
}

func (r *couponServiceDeleteRewardPoolRepo) GetCouponRewardPool(_ context.Context, _ int64) (*CouponRewardPoolVersion, error) {
	return r.pool, nil
}

func (r *couponServiceDeleteRewardPoolRepo) DeleteCouponRewardPool(_ context.Context, id int64) error {
	r.deletedIDs = append(r.deletedIDs, id)
	return nil
}

func TestValidateCouponTemplateRequiresUsableTerms(t *testing.T) {
	t.Parallel()

	validFrom := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		template CouponTemplate
		wantErr  string
	}{
		{
			name: "accepts a relative recharge coupon",
			template: CouponTemplate{
				Key:                "recharge-10-off-1",
				Name:               "Recharge 10 off 1",
				Status:             CouponTemplateStatusActive,
				BenefitType:        CouponBenefitTypeFixedAmount,
				BenefitValue:       1,
				Currency:           "CNY",
				ApplicableScopes:   []CouponScope{CouponScopeBalance},
				MinimumOrderAmount: 10,
				ValidityMode:       CouponValidityModeRelativeDays,
				ValidityDays:       3,
				ValidFrom:          &validFrom,
			},
		},
		{
			name: "rejects a template without an order scope",
			template: CouponTemplate{
				Key:          "missing-scope",
				Name:         "Missing scope",
				Status:       CouponTemplateStatusDraft,
				BenefitType:  CouponBenefitTypeFixedAmount,
				BenefitValue: 1,
				Currency:     "CNY",
				ValidityMode: CouponValidityModeRelativeDays,
				ValidityDays: 1,
			},
			wantErr: "applicable scopes",
		},
		{
			name: "rejects percentage above one hundred",
			template: CouponTemplate{
				Key:              "too-much-percent",
				Name:             "Too much percent",
				Status:           CouponTemplateStatusDraft,
				BenefitType:      CouponBenefitTypePercentage,
				BenefitValue:     100.01,
				Currency:         "CNY",
				ApplicableScopes: []CouponScope{CouponScopeSubscription},
				ValidityMode:     CouponValidityModeRelativeDays,
				ValidityDays:     1,
			},
			wantErr: "within (0, 100]",
		},
		{
			name: "rejects relative validity without days",
			template: CouponTemplate{
				Key:              "relative-without-days",
				Name:             "Relative without days",
				Status:           CouponTemplateStatusDraft,
				BenefitType:      CouponBenefitTypeFixedAmount,
				BenefitValue:     1,
				Currency:         "CNY",
				ApplicableScopes: []CouponScope{CouponScopeBalance},
				ValidityMode:     CouponValidityModeRelativeDays,
			},
			wantErr: "validity days",
		},
		{
			name: "rejects fixed validity with relative days",
			template: CouponTemplate{
				Key:              "fixed-with-days",
				Name:             "Fixed with days",
				Status:           CouponTemplateStatusDraft,
				BenefitType:      CouponBenefitTypeFixedAmount,
				BenefitValue:     1,
				Currency:         "CNY",
				ApplicableScopes: []CouponScope{CouponScopeBalance},
				ValidityMode:     CouponValidityModeFixed,
				ValidityDays:     1,
				FixedExpiresAt:   couponTestTimePtr(validFrom.Add(24 * time.Hour)),
			},
			wantErr: "only allowed for relative_days",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCouponTemplate(tc.template)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestDeleteRewardPoolOnlyDeletesDraftVersions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     CouponRewardPoolStatus
		wantReason string
		wantDelete bool
	}{
		{name: "draft", status: CouponRewardPoolStatusDraft, wantDelete: true},
		{name: "published", status: CouponRewardPoolStatusPublished, wantReason: "COUPON_POOL_DELETE_REJECTED"},
		{name: "retired", status: CouponRewardPoolStatusRetired, wantReason: "COUPON_POOL_DELETE_REJECTED"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &couponServiceDeleteRewardPoolRepo{pool: &CouponRewardPoolVersion{ID: 71, Status: tc.status}}
			svc := NewCouponService(repo)

			err := svc.DeleteRewardPool(context.Background(), 71)
			if tc.wantReason != "" {
				require.Equal(t, tc.wantReason, infraerrors.Reason(err))
			} else {
				require.NoError(t, err)
			}
			if tc.wantDelete {
				require.Equal(t, []int64{71}, repo.deletedIDs)
			} else {
				require.Empty(t, repo.deletedIDs)
			}
		})
	}
}

func TestCouponRewardReplayReadReturnsOnlyTheOwnersIssuedCoupon(t *testing.T) {
	issuedAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &couponServiceRewardReplayRepo{result: &CouponRewardIssueResult{
		UserCouponID: 701,
		Coupon: UserCoupon{
			ID:        701,
			UserID:    42,
			ValidFrom: issuedAt,
			ExpiresAt: issuedAt.Add(time.Hour),
		},
	}}
	svc := NewCouponService(repo)

	result, err := svc.GetCouponRewardIssueByIdempotency(context.Background(), 42, "blindbox:42:hash")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(701), result.UserCouponID)

	result, err = svc.GetCouponRewardIssueByIdempotency(context.Background(), 43, "blindbox:42:hash")
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestValidateCouponRewardPoolEnforcesOuterAndInnerWeights(t *testing.T) {
	t.Parallel()

	valid := CouponRewardPoolVersion{
		Activity:           CouponRewardActivityBlindbox,
		Version:            "blindbox-20260727-v1",
		Status:             CouponRewardPoolStatusDraft,
		CouponWeightBP:     6000,
		BalanceWeightBP:    4000,
		FallbackTemplateID: 9,
		Entries: []CouponRewardPoolEntry{
			{TemplateID: 9, WeightBP: 1, Enabled: true},
			{TemplateID: 10, WeightBP: 6000, Enabled: true},
			{TemplateID: 11, WeightBP: 4000, Enabled: true},
		},
	}
	require.NoError(t, ValidateCouponRewardPool(valid))

	wrongOuter := valid
	wrongOuter.CouponWeightBP = 7000
	wrongOuter.BalanceWeightBP = 3000
	require.ErrorContains(t, ValidateCouponRewardPool(wrongOuter), "blindbox coupon weight")

	wrongInner := valid
	wrongInner.Entries = []CouponRewardPoolEntry{
		{TemplateID: 9, WeightBP: 1, Enabled: true},
		{TemplateID: 10, WeightBP: 9999, Enabled: true},
	}
	require.ErrorContains(t, ValidateCouponRewardPool(wrongInner), "entry weights")

	wrongFallbackWeight := valid
	wrongFallbackWeight.Entries = append([]CouponRewardPoolEntry(nil), valid.Entries...)
	wrongFallbackWeight.Entries[0].WeightBP = 2
	require.ErrorContains(t, ValidateCouponRewardPool(wrongFallbackWeight), "fallback entry weight")
}

func TestValidateCouponRewardPoolRequiresAnUnboundedFallbackEntry(t *testing.T) {
	t.Parallel()

	stockCap := int64(100)
	perUserLimit := 1
	startsAt := time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC)
	base := CouponRewardPoolVersion{
		Activity:           CouponRewardActivityBlindbox,
		Version:            "blindbox-fallback-v1",
		Status:             CouponRewardPoolStatusDraft,
		CouponWeightBP:     6000,
		BalanceWeightBP:    4000,
		FallbackTemplateID: 9,
		Entries: []CouponRewardPoolEntry{
			{TemplateID: 9, WeightBP: 1, Enabled: true},
			{TemplateID: 10, WeightBP: 10000, Enabled: true, PerUserIssueLimit: &perUserLimit},
		},
	}

	for _, tc := range []struct {
		name   string
		mutate func(*CouponRewardPoolVersion)
		want   string
	}{
		{
			name: "per user limit",
			mutate: func(pool *CouponRewardPoolVersion) {
				pool.Entries[0].PerUserIssueLimit = &perUserLimit
			},
			want: "fallback entry cannot set a per-user issue limit",
		},
		{
			name: "stock cap",
			mutate: func(pool *CouponRewardPoolVersion) {
				pool.Entries[0].StockCap = &stockCap
			},
			want: "fallback entry cannot set a stock cap",
		},
		{
			name: "start window",
			mutate: func(pool *CouponRewardPoolVersion) {
				pool.Entries[0].StartsAt = &startsAt
			},
			want: "fallback entry cannot set an active window",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := base
			pool.Entries = append([]CouponRewardPoolEntry(nil), base.Entries...)
			tc.mutate(&pool)
			require.ErrorContains(t, ValidateCouponRewardPool(pool), tc.want)
		})
	}
}

func TestPublishCouponRewardPoolRequiresAnUnboundedFallbackTemplate(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	limit := int64(100)
	fixedExpiry := now.Add(24 * time.Hour)
	baseTemplate := func(id int64) *CouponTemplate {
		return &CouponTemplate{
			ID:               id,
			Key:              "fallback-template",
			Name:             "Fallback coupon",
			Status:           CouponTemplateStatusActive,
			BenefitType:      CouponBenefitTypeFixedAmount,
			BenefitValue:     1,
			Currency:         "CNY",
			ApplicableScopes: []CouponScope{CouponScopeBalance},
			ValidityMode:     CouponValidityModeRelativeDays,
			ValidityDays:     3,
		}
	}
	pool := &CouponRewardPoolVersion{
		ID:                 71,
		Activity:           CouponRewardActivityBlindbox,
		Version:            "blindbox-safe-fallback-v1",
		Status:             CouponRewardPoolStatusDraft,
		CouponWeightBP:     6000,
		BalanceWeightBP:    4000,
		FallbackTemplateID: 9,
		Entries: []CouponRewardPoolEntry{
			{TemplateID: 9, WeightBP: 1, Enabled: true},
			{TemplateID: 10, WeightBP: 10000, Enabled: true, PerUserIssueLimit: couponTestIntPtr(1)},
		},
	}

	for _, tc := range []struct {
		name   string
		mutate func(*CouponTemplate)
		want   string
	}{
		{
			name: "total issue limit",
			mutate: func(template *CouponTemplate) {
				template.TotalIssueLimit = &limit
			},
			want: "fallback template cannot set a total issue limit",
		},
		{
			name: "fixed expiry",
			mutate: func(template *CouponTemplate) {
				template.ValidityMode = CouponValidityModeFixed
				template.ValidityDays = 0
				template.FixedExpiresAt = &fixedExpiry
			},
			want: "fallback template cannot use fixed expiry",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fallback := baseTemplate(9)
			tc.mutate(fallback)
			repo := &couponServicePublishPoolRepo{
				pool: pool,
				templates: map[int64]*CouponTemplate{
					9:  fallback,
					10: baseTemplate(10),
				},
			}
			svc := NewCouponService(repo)
			svc.now = func() time.Time { return now }

			_, err := svc.PublishRewardPool(context.Background(), pool.ID, 17)
			require.ErrorContains(t, err, tc.want)
			require.False(t, repo.published)
		})
	}
}

func TestUpdateTemplateCannotConstrainPublishedFallback(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	limit := int64(100)
	existing := &CouponTemplate{
		ID:               9,
		Key:              "fallback-template",
		Version:          1,
		Name:             "Fallback coupon",
		Status:           CouponTemplateStatusActive,
		BenefitType:      CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []CouponScope{CouponScopeBalance},
		ValidityMode:     CouponValidityModeRelativeDays,
		ValidityDays:     3,
	}
	repo := &couponServiceFallbackTemplateRepo{
		template: existing,
		pools: []CouponRewardPoolVersion{{
			ID:                 71,
			Activity:           CouponRewardActivityBlindbox,
			Status:             CouponRewardPoolStatusPublished,
			FallbackTemplateID: existing.ID,
		}},
	}
	svc := NewCouponService(repo)
	svc.now = func() time.Time { return now }

	_, err := svc.UpdateTemplate(context.Background(), existing.ID, CouponTemplateInput{
		Key:                existing.Key,
		Name:               existing.Name,
		Status:             CouponTemplateStatusActive,
		BenefitType:        existing.BenefitType,
		BenefitValue:       existing.BenefitValue,
		Currency:           existing.Currency,
		ApplicableScopes:   existing.ApplicableScopes,
		ValidityMode:       existing.ValidityMode,
		ValidityDays:       existing.ValidityDays,
		TotalIssueLimit:    &limit,
		MinimumOrderAmount: existing.MinimumOrderAmount,
	}, 17)
	require.ErrorContains(t, err, "fallback template cannot set a total issue limit")
	require.False(t, repo.updated)
}

func TestCouponTemplateFinancialTermsIncludeAbsoluteValidityControls(t *testing.T) {
	t.Parallel()

	validFrom := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	fixedExpiry := validFrom.Add(72 * time.Hour)
	base := CouponTemplate{
		Key:              "fixed-validity",
		Name:             "Fixed validity",
		Status:           CouponTemplateStatusActive,
		BenefitType:      CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []CouponScope{CouponScopeBalance},
		ValidityMode:     CouponValidityModeFixed,
		FixedExpiresAt:   &fixedExpiry,
		ValidFrom:        &validFrom,
	}

	changedExpiry := base
	differentExpiry := fixedExpiry.Add(time.Hour)
	changedExpiry.FixedExpiresAt = &differentExpiry
	require.False(t, CouponTemplateFinancialTermsEqual(base, changedExpiry))

	changedValidFrom := base
	differentValidFrom := validFrom.Add(time.Hour)
	changedValidFrom.ValidFrom = &differentValidFrom
	require.False(t, CouponTemplateFinancialTermsEqual(base, changedValidFrom))

	equivalentUTC := base
	localExpiry := fixedExpiry.In(time.FixedZone("test", 8*60*60))
	equivalentUTC.FixedExpiresAt = &localExpiry
	require.True(t, CouponTemplateFinancialTermsEqual(base, equivalentUTC))
}

func TestQuoteCouponUsesSnapshotAndNeverMakesNegativePayable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	coupon := UserCoupon{
		ID:        42,
		UserID:    7,
		Status:    UserCouponStatusAvailable,
		ValidFrom: now.Add(-time.Hour),
		ExpiresAt: expiresAt,
		TermsSnapshot: CouponTermsSnapshot{
			Name:               "30 percent up to 5",
			BenefitType:        CouponBenefitTypePercentage,
			BenefitValue:       30,
			MaxDiscountAmount:  couponTestFloat64Ptr(5),
			ApplicableScopes:   []CouponScope{CouponScopeSubscription},
			MinimumOrderAmount: 10,
			EligiblePlanIDs:    []int64{88},
			Currency:           "CNY",
		},
	}

	quote, err := QuoteUserCoupon(coupon, CouponOrderContext{
		UserID:      7,
		Scope:       CouponScopeSubscription,
		OrderAmount: 30,
		PlanID:      88,
		At:          now,
	})
	require.NoError(t, err)
	require.Equal(t, 5.0, quote.DiscountAmount)
	require.Equal(t, 25.0, quote.PayableAmount)

	coupon.TermsSnapshot = CouponTermsSnapshot{
		BenefitType:      CouponBenefitTypeFixedAmount,
		BenefitValue:     100,
		ApplicableScopes: []CouponScope{CouponScopeBalance},
		Currency:         "CNY",
	}
	quote, err = QuoteUserCoupon(coupon, CouponOrderContext{
		UserID:      7,
		Scope:       CouponScopeBalance,
		OrderAmount: 10,
		At:          now,
	})
	require.NoError(t, err)
	require.Equal(t, 10.0, quote.DiscountAmount)
	require.Zero(t, quote.PayableAmount)
}

func TestQuoteCouponUsesPaymentCurrencyPrecision(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	coupon := UserCoupon{
		ID:        43,
		UserID:    7,
		Status:    UserCouponStatusAvailable,
		ValidFrom: now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
		TermsSnapshot: CouponTermsSnapshot{
			BenefitType:      CouponBenefitTypeFixedAmount,
			BenefitValue:     0.123,
			ApplicableScopes: []CouponScope{CouponScopeBalance},
			Currency:         "KWD",
		},
	}

	quote, err := QuoteUserCoupon(coupon, CouponOrderContext{
		UserID:      7,
		Scope:       CouponScopeBalance,
		OrderAmount: 1.234,
		Currency:    "KWD",
		At:          now,
	})

	require.NoError(t, err)
	require.Equal(t, 1.234, quote.OriginalAmount)
	require.Equal(t, 0.123, quote.DiscountAmount)
	require.Equal(t, 1.111, quote.PayableAmount)
}

func TestGetUserCouponForUserHidesOtherUsersCoupons(t *testing.T) {
	t.Parallel()

	repo := &couponServiceUserCouponRepo{coupon: &UserCoupon{ID: 52, UserID: 7}}
	svc := NewCouponService(repo)

	coupon, err := svc.GetUserCouponForUser(context.Background(), 52, 7)
	require.NoError(t, err)
	require.Equal(t, int64(52), coupon.ID)

	_, err = svc.GetUserCouponForUser(context.Background(), 52, 8)
	require.Equal(t, "COUPON_NOT_FOUND", infraerrors.Reason(err))

	repo.coupon = nil
	_, err = svc.GetUserCouponForUser(context.Background(), 52, 7)
	require.Equal(t, "COUPON_NOT_FOUND", infraerrors.Reason(err))
}

func TestCouponRewardPoolReadyUsesLiveIssuanceCheck(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	repo := &couponServiceRewardReadinessRepo{ready: false}
	svc := NewCouponService(repo)
	svc.now = func() time.Time { return now }

	ready, err := svc.CouponRewardPoolReady(context.Background(), CouponRewardActivityBlindbox)
	require.NoError(t, err)
	require.False(t, ready)
	require.Equal(t, CouponRewardActivityBlindbox, repo.activity)
	require.Equal(t, now, repo.at)

	repo.ready = true
	ready, err = svc.CouponRewardPoolReady(context.Background(), CouponRewardActivityQuiz)
	require.NoError(t, err)
	require.True(t, ready)

	_, err = svc.CouponRewardPoolReady(context.Background(), CouponRewardActivity("unknown"))
	require.Equal(t, "INVALID_COUPON_POOL_ACTIVITY", infraerrors.Reason(err))
}

func TestListUserCouponsExpiresAvailableWalletItemsBeforeReturningThem(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	repo := &couponServiceListRepo{}
	svc := NewCouponService(repo)
	svc.now = func() time.Time { return now }

	rows, page, err := svc.ListUserCoupons(context.Background(), UserCouponListFilter{
		UserID:   7,
		Status:   UserCouponStatusAvailable,
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 1, repo.expireCalls)
	require.Equal(t, int64(7), repo.expireUser)
	require.Equal(t, now, repo.expireAt)
}

func TestGetPublishedRewardPoolRejectsPublishedButExhaustedEntries(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	stockCap := int64(1)
	template := &CouponTemplate{
		ID:               9,
		Key:              "blindbox-stocked",
		Name:             "Blindbox stocked coupon",
		Status:           CouponTemplateStatusActive,
		BenefitType:      CouponBenefitTypeFixedAmount,
		BenefitValue:     1,
		Currency:         "CNY",
		ApplicableScopes: []CouponScope{CouponScopeBalance},
		ValidityMode:     CouponValidityModeRelativeDays,
		ValidityDays:     3,
	}
	repo := &couponServiceRewardPoolRepo{
		pool: &CouponRewardPoolVersion{
			Activity:           CouponRewardActivityBlindbox,
			Version:            "blindbox-stocked-v1",
			Status:             CouponRewardPoolStatusPublished,
			CouponWeightBP:     6000,
			BalanceWeightBP:    4000,
			FallbackTemplateID: 9,
			Entries: []CouponRewardPoolEntry{{
				ID:          31,
				TemplateID:  9,
				WeightBP:    10_000,
				Enabled:     true,
				StockCap:    &stockCap,
				IssuedCount: 1,
			}},
		},
		templates: map[int64]*CouponTemplate{9: template},
	}
	svc := NewCouponService(repo)
	svc.now = func() time.Time { return now }

	pool, err := svc.GetPublishedRewardPool(context.Background(), CouponRewardActivityBlindbox)
	require.Nil(t, pool)
	require.ErrorIs(t, err, ErrCouponRewardPoolUnavailable)
}

func couponTestFloat64Ptr(v float64) *float64 { return &v }

func couponTestIntPtr(v int) *int { return &v }

func couponTestTimePtr(v time.Time) *time.Time { return &v }
