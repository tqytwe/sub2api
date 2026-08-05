//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlement"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlementhold"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDailyCardRepositoryIssuesOneActiveAndQueuesDuplicatesByOrder(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := NewDailyCardEntitlementRepository(client)
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)

	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order1 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	order2 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now.Add(time.Minute))

	first, created, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order1.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, service.DailyCardStatusActive, first.Status)

	second, created, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order2.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, service.DailyCardStatusPending, second.Status)

	duplicate, created, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order2.ID,
		QuotaLimitUSD: 999, DurationHours: 999, IssuedAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, second.ID, duplicate.ID)
	require.Equal(t, 10.0, duplicate.QuotaLimitUSD, "duplicate callback must not mutate the immutable order grant")

	active, err := repo.ReconcileAndGetActive(ctx, user.ID, group.ID, now.Add(24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, second.ID, active.ID)
	require.Equal(t, now.Add(24*time.Hour), *active.StartsAt)
	require.Equal(t, now.Add(48*time.Hour), *active.ExpiresAt)
}

func TestDailyCardRepositoryTracksAdmissionWithoutBlockingParallelRequests(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := NewDailyCardEntitlementRepository(client)
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	card, _, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)

	err = repo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: "local:first",
		RequestFingerprint: "fingerprint-first", ReservedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	hold, err := client.SubscriptionEntitlementHold.Query().
		Where(subscriptionentitlementhold.EntitlementIDEQ(card.ID), subscriptionentitlementhold.RequestIDEQ("local:first")).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, *card.ExpiresAt, hold.ExpiresAt, "admission hold must prove the request started before card expiry")
	require.Zero(t, hold.ReservedUsd)
	held, err := repo.GetActive(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.Zero(t, held.QuotaReservedUSD)
	err = repo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: "local:first",
		RequestFingerprint: "fingerprint-first", ReservedAt: now.Add(90 * time.Second),
	})
	require.NoError(t, err)

	err = repo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: "local:second",
		RequestFingerprint: "fingerprint-second", ReservedAt: now.Add(2 * time.Minute),
	})
	require.NoError(t, err)

	require.NoError(t, repo.ReleaseRequest(ctx, card.ID, user.ID, "local:first", now.Add(3*time.Minute)))
	released, err := repo.GetActive(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.Zero(t, released.QuotaReservedUSD)
}

func TestDailyCardRepositoryAdminReleaseReservedHoldsDoesNotResetUsage(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := NewDailyCardEntitlementRepository(client)
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	card, _, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: "admin-release",
		RequestFingerprint: "admin-release-fingerprint", ReservedAt: now.Add(time.Minute),
	}))
	_, err = client.SubscriptionEntitlement.UpdateOneID(card.ID).
		SetQuotaUsedUsd(6.25).
		SetQuotaReservedUsd(48.75).
		Save(ctx)
	require.NoError(t, err)

	result, err := repo.AdminReleaseReservedHolds(ctx, card.ID, user.ID, group.ID, now.Add(2*time.Minute))

	require.NoError(t, err)
	require.Equal(t, int64(1), result.ReleasedHolds)
	require.NotNil(t, result.Card)
	require.Equal(t, 6.25, result.Card.QuotaUsedUSD)
	require.Zero(t, result.Card.QuotaReservedUSD)
	hold, err := client.SubscriptionEntitlementHold.Query().
		Where(subscriptionentitlementhold.EntitlementIDEQ(card.ID), subscriptionentitlementhold.RequestIDEQ("admin-release")).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "released", hold.Status)
	require.NotNil(t, hold.ReleasedAt)
}

func TestDailyCardRepositoryAdminRestoreQuotaClearsCurrentCardUsage(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := NewDailyCardEntitlementRepository(client)
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	card, _, err := repo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: "admin-restore",
		RequestFingerprint: "admin-restore-fingerprint", ReservedAt: now.Add(time.Minute),
	}))
	_, err = client.SubscriptionEntitlement.UpdateOneID(card.ID).
		SetQuotaUsedUsd(10).
		SetQuotaReservedUsd(2).
		SetStatus(service.DailyCardStatusExhausted).
		SetExhaustedAt(now.Add(time.Hour)).
		SetEndedAt(now.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	result, err := repo.AdminRestoreQuota(ctx, card.ID, user.ID, group.ID, now.Add(2*time.Minute))

	require.NoError(t, err)
	require.Equal(t, int64(1), result.ReleasedHolds)
	require.NotNil(t, result.Card)
	require.Equal(t, service.DailyCardStatusActive, result.Card.Status)
	require.Zero(t, result.Card.QuotaUsedUSD)
	require.Zero(t, result.Card.QuotaReservedUSD)
	require.Nil(t, result.Card.ExhaustedAt)
	require.Nil(t, result.Card.EndedAt)
	activeCount, err := client.SubscriptionEntitlement.Query().
		Where(
			subscriptionentitlement.UserIDEQ(user.ID),
			subscriptionentitlement.GroupIDEQ(group.ID),
			subscriptionentitlement.StatusEQ(service.DailyCardStatusActive),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, activeCount)
}

func TestDailyCardRepositoryRecognizesOnlyLaterRecurringPurchase(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	repo := NewDailyCardEntitlementRepository(client)
	dailyPurchasedAt := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)

	recurringPaidAt := dailyPurchasedAt.Add(time.Hour)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail("daily-card@example.com").
		SetUserName("daily-card").
		SetAmount(99).
		SetPayAmount(99).
		SetRechargeCode("RECURRING-" + suffix).
		SetOutTradeNo("recurring-" + suffix).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-recurring-" + suffix).
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(plan.ID).
		SetSubscriptionGroupID(group.ID).
		SetSubscriptionDays(30).
		SetSubscriptionSnapshot(map[string]any{"quota_mode": service.PlanQuotaModeRecurring}).
		SetStatus(service.OrderStatusCompleted).
		SetPaidAt(recurringPaidAt).
		SetExpiresAt(recurringPaidAt.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	hasLaterRecurring, err := repo.HasRecurringOrderAfter(ctx, user.ID, group.ID, dailyPurchasedAt)
	require.NoError(t, err)
	require.True(t, hasLaterRecurring)
	hasStillLaterRecurring, err := repo.HasRecurringOrderAfter(ctx, user.ID, group.ID, recurringPaidAt)
	require.NoError(t, err)
	require.False(t, hasStillLaterRecurring)
}

func createDailyCardIntegrationCatalog(t *testing.T, ctx context.Context, client *dbent.Client) (*dbent.User, *dbent.Group, *dbent.SubscriptionPlan) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user, err := client.User.Create().
		SetEmail("daily-card-" + suffix + "@example.com").
		SetPasswordHash("hash").
		Save(ctx)
	require.NoError(t, err)
	limit := 10.0
	group, err := client.Group.Create().
		SetName("daily-card-" + suffix).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetDailyLimitUsd(limit).
		Save(ctx)
	require.NoError(t, err)
	duration := 24
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("Daily Card").
		SetPrice(9.9).
		SetValidityDays(1).
		SetValidityUnit("day").
		SetQuotaMode(service.DailyCardQuotaModeOneTime).
		SetQuotaLimitUsd(limit).
		SetDurationHours(duration).
		Save(ctx)
	require.NoError(t, err)
	return user, group, plan
}

func createDailyCardIntegrationOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID, groupID, planID int64, paidAt time.Time) *dbent.PaymentOrder {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	order, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("daily-card@example.com").
		SetUserName("daily-card").
		SetAmount(9.9).
		SetPayAmount(9.9).
		SetRechargeCode("DAILY-" + suffix).
		SetOutTradeNo("daily-" + suffix).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-" + suffix).
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(planID).
		SetSubscriptionGroupID(groupID).
		SetSubscriptionDays(1).
		SetSubscriptionSnapshot(map[string]any{"quota_mode": "one_time", "quota_limit_usd": 10.0, "duration_hours": 24}).
		SetStatus(service.OrderStatusCompleted).
		SetPaidAt(paidAt).
		SetExpiresAt(paidAt.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)
	return order
}
