package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dailyCardRepoStub struct {
	issuedInput            IssueDailyCardInput
	issued                 *DailyCardEntitlement
	created                bool
	err                    error
	listed                 []DailyCardEntitlement
	reconciled             []int64
	hasRecurringOrderAfter bool
	recurringOrderAfter    time.Time
	oneTimeGroup           bool
	adminReleaseInput      []int64
	adminRestoreInput      []int64
	adminResult            *DailyCardAdminActionResult
}

func (r *dailyCardRepoStub) IssuePaidCard(_ context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error) {
	r.issuedInput = input
	return r.issued, r.created, r.err
}

func (*dailyCardRepoStub) GetActive(context.Context, int64, int64) (*DailyCardEntitlement, error) {
	return nil, ErrDailyCardEntitlementNotFound
}

func (*dailyCardRepoStub) GetByPaymentOrder(context.Context, int64) (*DailyCardEntitlement, error) {
	return nil, ErrDailyCardEntitlementNotFound
}

func (r *dailyCardRepoStub) ReconcileAndGetActive(_ context.Context, _ int64, groupID int64, _ time.Time) (*DailyCardEntitlement, error) {
	r.reconciled = append(r.reconciled, groupID)
	return nil, ErrDailyCardEntitlementNotFound
}

func (r *dailyCardRepoStub) ListByUser(context.Context, int64) ([]DailyCardEntitlement, error) {
	return append([]DailyCardEntitlement(nil), r.listed...), nil
}

func (r *dailyCardRepoStub) HasRecurringOrderAfter(_ context.Context, _, _ int64, after time.Time) (bool, error) {
	r.recurringOrderAfter = after
	return r.hasRecurringOrderAfter, nil
}

func (r *dailyCardRepoStub) IsOneTimeGroup(context.Context, int64) (bool, error) {
	return r.oneTimeGroup, r.err
}

func (*dailyCardRepoStub) ReserveRequest(context.Context, DailyCardRequestHoldInput) error {
	return nil
}

func (*dailyCardRepoStub) ReleaseRequest(context.Context, int64, int64, string, time.Time) error {
	return nil
}

func (r *dailyCardRepoStub) AdminReleaseReservedHolds(_ context.Context, entitlementID, userID, groupID int64, _ time.Time) (*DailyCardAdminActionResult, error) {
	r.adminReleaseInput = []int64{entitlementID, userID, groupID}
	if r.adminResult != nil {
		return r.adminResult, r.err
	}
	return &DailyCardAdminActionResult{}, r.err
}

func (r *dailyCardRepoStub) AdminRestoreQuota(_ context.Context, entitlementID, userID, groupID int64, _ time.Time) (*DailyCardAdminActionResult, error) {
	r.adminRestoreInput = []int64{entitlementID, userID, groupID}
	if r.adminResult != nil {
		return r.adminResult, r.err
	}
	return &DailyCardAdminActionResult{}, r.err
}

func TestDailyCardEntitlementCrossingMidnightDoesNotResetQuota(t *testing.T) {
	start := time.Date(2026, 7, 28, 18, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	card := DailyCardEntitlement{
		Status:        DailyCardStatusActive,
		QuotaLimitUSD: 10,
		QuotaUsedUSD:  9,
		StartsAt:      &start,
		ExpiresAt:     ptrDailyCardTime(start.Add(24 * time.Hour)),
	}

	card.ReconcileAt(time.Date(2026, 7, 29, 0, 1, 0, 0, start.Location()))

	require.Equal(t, DailyCardStatusActive, card.Status)
	require.Equal(t, 9.0, card.QuotaUsedUSD)
}

func TestDailyCardEntitlementExhaustionEndsBeforeTimeExpiry(t *testing.T) {
	start := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	card := DailyCardEntitlement{
		Status:        DailyCardStatusActive,
		QuotaLimitUSD: 10,
		QuotaUsedUSD:  9,
		StartsAt:      &start,
		ExpiresAt:     ptrDailyCardTime(start.Add(24 * time.Hour)),
	}
	settledAt := start.Add(2 * time.Hour)

	card.ApplyCapturedUsage(1, settledAt)

	require.Equal(t, DailyCardStatusExhausted, card.Status)
	require.Equal(t, 10.0, card.QuotaUsedUSD)
	require.Equal(t, &settledAt, card.ExhaustedAt)
	require.Equal(t, &settledAt, card.EndedAt)
}

func TestDailyCardEntitlementTimeExpiryWinsWhenQuotaRemains(t *testing.T) {
	start := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	expiresAt := start.Add(24 * time.Hour)
	card := DailyCardEntitlement{
		Status:        DailyCardStatusActive,
		QuotaLimitUSD: 10,
		QuotaUsedUSD:  3,
		StartsAt:      &start,
		ExpiresAt:     &expiresAt,
	}

	card.ReconcileAt(expiresAt)

	require.Equal(t, DailyCardStatusExpired, card.Status)
	require.Equal(t, &expiresAt, card.EndedAt)
	require.Nil(t, card.ExhaustedAt)
}

func TestDailyCardEntitlementPendingActivationStartsFreshTwentyFourHours(t *testing.T) {
	activateAt := time.Date(2026, 7, 29, 9, 30, 0, 0, time.UTC)
	card := DailyCardEntitlement{
		Status:        DailyCardStatusPending,
		QuotaLimitUSD: 10,
	}

	require.NoError(t, card.Activate(activateAt, 24*time.Hour))

	require.Equal(t, DailyCardStatusActive, card.Status)
	require.Equal(t, &activateAt, card.StartsAt)
	require.Equal(t, activateAt.Add(24*time.Hour), *card.ExpiresAt)
	require.Zero(t, card.QuotaUsedUSD)
}

func TestDailyCardServiceIssuePaidCardPassesImmutableOrderSnapshot(t *testing.T) {
	issuedAt := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	want := &DailyCardEntitlement{ID: 99, PaymentOrderID: 44}
	repo := &dailyCardRepoStub{issued: want, created: true}
	svc := NewDailyCardService(repo)

	got, created, err := svc.IssuePaidCard(context.Background(), IssueDailyCardInput{
		UserID: 1, GroupID: 2, PlanID: 3, PaymentOrderID: 44,
		QuotaLimitUSD: 12.5, DurationHours: 24, IssuedAt: issuedAt,
	})

	require.NoError(t, err)
	require.True(t, created)
	require.Same(t, want, got)
	require.Equal(t, int64(44), repo.issuedInput.PaymentOrderID)
	require.Equal(t, 12.5, repo.issuedInput.QuotaLimitUSD)
	require.Equal(t, issuedAt, repo.issuedInput.IssuedAt)
}

func TestDailyCardServiceIssuePaidCardRejectsMissingQuota(t *testing.T) {
	svc := NewDailyCardService(&dailyCardRepoStub{})

	_, _, err := svc.IssuePaidCard(context.Background(), IssueDailyCardInput{
		UserID: 1, GroupID: 2, PlanID: 3, PaymentOrderID: 44, DurationHours: 24,
	})

	require.ErrorIs(t, err, ErrDailyCardInvalidInput)
}

func TestDailyCardServiceListForUserReconcilesEachGroupOnce(t *testing.T) {
	repo := &dailyCardRepoStub{listed: []DailyCardEntitlement{
		{ID: 1, UserID: 7, GroupID: 10, Status: DailyCardStatusActive},
		{ID: 2, UserID: 7, GroupID: 10, Status: DailyCardStatusPending},
		{ID: 3, UserID: 7, GroupID: 20, Status: DailyCardStatusExpired},
	}}
	svc := NewDailyCardService(repo)

	cards, err := svc.ListForUser(context.Background(), 7, time.Now())

	require.NoError(t, err)
	require.Len(t, cards, 3)
	require.Equal(t, []int64{10, 20}, repo.reconciled)
}

func TestDailyCardServiceFallsBackToLaterRecurringPurchaseAfterCardsEnd(t *testing.T) {
	purchasedAt := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	repo := &dailyCardRepoStub{
		listed: []DailyCardEntitlement{{
			ID: 1, UserID: 7, GroupID: 10, Status: DailyCardStatusExhausted, CreatedAt: purchasedAt,
		}},
		hasRecurringOrderAfter: true,
	}
	svc := NewDailyCardService(repo)

	card, managed, err := svc.ResolveAccess(context.Background(), 7, 10, purchasedAt.Add(time.Hour))

	require.NoError(t, err)
	require.Nil(t, card)
	require.False(t, managed)
	require.Equal(t, purchasedAt, repo.recurringOrderAfter)
}

func TestDailyCardServiceKeepsDailyOnlyGroupClosedAfterCardsEnd(t *testing.T) {
	purchasedAt := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	repo := &dailyCardRepoStub{listed: []DailyCardEntitlement{{
		ID: 1, UserID: 7, GroupID: 10, Status: DailyCardStatusExhausted, CreatedAt: purchasedAt,
	}}}
	svc := NewDailyCardService(repo)

	card, managed, err := svc.ResolveAccess(context.Background(), 7, 10, purchasedAt.Add(time.Hour))

	require.ErrorIs(t, err, ErrDailyCardUnavailable)
	require.Nil(t, card)
	require.True(t, managed)
}

func TestDailyCardServiceClosesManagedGroupWithoutEntitlementHistory(t *testing.T) {
	svc := NewDailyCardService(&dailyCardRepoStub{oneTimeGroup: true})

	card, managed, err := svc.ResolveAccess(context.Background(), 7, 10, time.Now())

	require.ErrorIs(t, err, ErrDailyCardUnavailable)
	require.Nil(t, card)
	require.True(t, managed)
}

func TestAssignSubscriptionRejectsUntrackedDailyCardGrant(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: 10, SubscriptionType: SubscriptionTypeSubscription}}
	svc := NewSubscriptionService(groupRepo, newSubscriptionUserSubRepoStub(), nil, nil, nil)
	svc.SetDailyCardService(NewDailyCardService(&dailyCardRepoStub{oneTimeGroup: true}))

	_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 7, GroupID: 10, ValidityDays: 1,
	})

	require.ErrorIs(t, err, ErrDailyCardPaidOrderRequired)
}

func TestDecorateDailyCardEntitlementsMakesExhaustionEffectiveImmediately(t *testing.T) {
	now := time.Now().UTC()
	repo := &dailyCardRepoStub{listed: []DailyCardEntitlement{{
		ID: 9, UserID: 7, GroupID: 10, Status: DailyCardStatusExhausted,
		QuotaLimitUSD: 16, QuotaUsedUSD: 16, CreatedAt: now.Add(-time.Hour), ExhaustedAt: &now, EndedAt: &now,
	}}}
	svc := &SubscriptionService{dailyCardSvc: NewDailyCardService(repo)}
	subs := []UserSubscription{{
		ID: 1, UserID: 7, GroupID: 10, Status: SubscriptionStatusActive,
		ExpiresAt: now.Add(12 * time.Hour), DailyUsageUSD: 17.28,
	}}

	require.NoError(t, svc.decorateDailyCardEntitlements(context.Background(), 7, subs))
	require.Equal(t, "exhausted", subs[0].Status)
	require.Equal(t, 16.0, subs[0].DailyUsageUSD)
	require.Equal(t, now, subs[0].ExpiresAt)
}

func TestDecorateDailyCardEntitlementsSkipsEndedCardAfterLaterRecurringPurchase(t *testing.T) {
	now := time.Now().UTC()
	purchasedAt := now.Add(-2 * time.Hour)
	repo := &dailyCardRepoStub{
		listed: []DailyCardEntitlement{{
			ID: 9, UserID: 7, GroupID: 10, Status: DailyCardStatusExhausted,
			QuotaLimitUSD: 16, QuotaUsedUSD: 16, CreatedAt: purchasedAt, ExhaustedAt: &now, EndedAt: &now,
		}},
		hasRecurringOrderAfter: true,
	}
	svc := &SubscriptionService{dailyCardSvc: NewDailyCardService(repo)}
	subs := []UserSubscription{{
		ID: 1, UserID: 7, GroupID: 10, Status: SubscriptionStatusActive,
		ExpiresAt: now.Add(30 * 24 * time.Hour), DailyUsageUSD: 3,
	}}

	require.NoError(t, svc.decorateDailyCardEntitlements(context.Background(), 7, subs))
	require.Equal(t, SubscriptionStatusActive, subs[0].Status)
	require.Nil(t, subs[0].DailyCard)
	require.Equal(t, 3.0, subs[0].DailyUsageUSD)
	require.Equal(t, purchasedAt, repo.recurringOrderAfter)
}

func TestListSubscriptionsFiltersByEffectiveDailyCardStatus(t *testing.T) {
	now := time.Now().UTC()
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID: 1, UserID: 7, GroupID: 10, Status: SubscriptionStatusActive,
		StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(23 * time.Hour), CreatedAt: now.Add(-time.Minute),
	})
	subRepo.seed(&UserSubscription{
		ID: 2, UserID: 7, GroupID: 20, Status: SubscriptionStatusActive,
		StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(23 * time.Hour), CreatedAt: now,
	})
	repo := &dailyCardRepoStub{listed: []DailyCardEntitlement{{
		ID: 9, UserID: 7, GroupID: 10, Status: DailyCardStatusExhausted,
		QuotaLimitUSD: 16, QuotaUsedUSD: 16, CreatedAt: now.Add(-time.Hour), EndedAt: &now,
	}}}
	svc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	svc.SetDailyCardService(NewDailyCardService(repo))

	active, activePage, err := svc.List(context.Background(), 1, 20, nil, nil, SubscriptionStatusActive, "", "", "")
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, int64(2), active[0].ID)
	require.Equal(t, int64(1), activePage.Total)

	exhausted, exhaustedPage, err := svc.List(context.Background(), 1, 20, nil, nil, SubscriptionStatusExhausted, "", "", "")
	require.NoError(t, err)
	require.Len(t, exhausted, 1)
	require.Equal(t, int64(1), exhausted[0].ID)
	require.Equal(t, int64(1), exhaustedPage.Total)
}

func ptrDailyCardTime(value time.Time) *time.Time {
	return &value
}
