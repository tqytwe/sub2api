//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlement"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlementhold"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingExhaustsDailyCardAndActivatesNextExactlyOnce(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	now := time.Now().UTC().Truncate(time.Second)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order1 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	order2 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now.Add(time.Minute))
	cardRepo := NewDailyCardEntitlementRepository(client)
	first, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order1.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	second, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order2.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)

	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now).
		SetExpiresAt(now.Add(48 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID, GroupID: &group.ID,
		Key: "sk-daily-card-" + fmt.Sprintf("%d", time.Now().UnixNano()), Name: "daily-card",
	})

	billingRepo := NewUsageBillingRepository(client, integrationDB)
	requestID := "daily-card-billing-" + fmt.Sprintf("%d", time.Now().UnixNano())
	require.NoError(t, cardRepo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: first.ID, UserID: user.ID, RequestID: requestID,
		RequestFingerprint: "daily-card-billing-fingerprint", ReservedAt: now.Add(2 * time.Minute),
	}))
	cmd := &service.UsageBillingCommand{
		RequestID: requestID,
		APIKeyID:  apiKey.ID, UserID: user.ID, AccountID: 1,
		SubscriptionID: &sub.ID, SubscriptionEntitlementID: &first.ID,
		SubscriptionCost: 10, ActualCost: 10, BilledCost: 10,
	}
	result, err := billingRepo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.DailyCardExhausted)
	require.NotNil(t, result.ActivatedEntitlementID)
	require.Equal(t, second.ID, *result.ActivatedEntitlementID)

	firstEntity, err := client.SubscriptionEntitlement.Get(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusExhausted, firstEntity.Status)
	require.Equal(t, 10.0, firstEntity.QuotaUsedUsd)
	require.Zero(t, firstEntity.QuotaReservedUsd)
	hold, err := client.SubscriptionEntitlementHold.Query().
		Where(subscriptionentitlementhold.RequestIDEQ(requestID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "captured", hold.Status)
	require.Zero(t, hold.ReservedUsd)
	require.Zero(t, hold.CapturedUsd)
	secondEntity, err := client.SubscriptionEntitlement.Get(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusActive, secondEntity.Status)
	require.NotNil(t, secondEntity.StartsAt)

	duplicate, err := billingRepo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
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

func TestUsageBillingSettlesInFlightRequestAgainstOriginalExpiredCard(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	now := time.Now().UTC().Truncate(time.Second)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order1 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	order2 := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now.Add(time.Minute))
	cardRepo := NewDailyCardEntitlementRepository(client)
	first, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order1.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	second, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order2.ID,
		QuotaLimitUSD: 10, DurationHours: 24, IssuedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)

	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now).
		SetExpiresAt(now.Add(48 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID, GroupID: &group.ID,
		Key: "sk-daily-card-expiry-" + fmt.Sprintf("%d", time.Now().UnixNano()), Name: "daily-card-expiry",
	})

	requestID := "daily-card-cross-expiry-" + fmt.Sprintf("%d", time.Now().UnixNano())
	require.NoError(t, cardRepo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: first.ID, UserID: user.ID, RequestID: requestID,
		RequestFingerprint: "daily-card-cross-expiry-fingerprint", ReservedAt: now.Add(24*time.Hour - time.Minute),
	}))

	active, err := cardRepo.ReconcileAndGetActive(ctx, user.ID, group.ID, now.Add(24*time.Hour+time.Minute))
	require.NoError(t, err)
	require.Equal(t, second.ID, active.ID)

	billingRepo := NewUsageBillingRepository(client, integrationDB)
	result, err := billingRepo.Apply(ctx, &service.UsageBillingCommand{
		RequestID: requestID,
		APIKeyID:  apiKey.ID, UserID: user.ID, AccountID: 1,
		SubscriptionID: &sub.ID, SubscriptionEntitlementID: &first.ID,
		SubscriptionCost: 10, ActualCost: 10, BilledCost: 10,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.False(t, result.DailyCardExhausted)
	require.Nil(t, result.ActivatedEntitlementID)

	settledFirst, err := client.SubscriptionEntitlement.Get(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusExpired, settledFirst.Status)
	require.Equal(t, 10.0, settledFirst.QuotaUsedUsd)
	require.Zero(t, settledFirst.QuotaReservedUsd)
	settledSecond, err := client.SubscriptionEntitlement.Get(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusActive, settledSecond.Status)
	require.Zero(t, settledSecond.QuotaUsedUsd)
}

func TestUsageBillingDailyCardOverageRecordsBalanceDebt(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	now := time.Now().UTC().Truncate(time.Second)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	_, err := client.User.UpdateOneID(user.ID).SetBalance(0.25).Save(ctx)
	require.NoError(t, err)
	order := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	cardRepo := NewDailyCardEntitlementRepository(client)
	card, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order.ID,
		QuotaLimitUSD: 1, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	sub, err := client.UserSubscription.Create().SetUserID(user.ID).SetGroupID(group.ID).
		SetStartsAt(now).SetExpiresAt(now.Add(24 * time.Hour)).SetStatus(service.SubscriptionStatusActive).Save(ctx)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID, GroupID: &group.ID,
		Key: "sk-daily-overage-" + fmt.Sprintf("%d", time.Now().UnixNano()), Name: "daily-overage",
	})
	requestID := "daily-overage-" + fmt.Sprintf("%d", time.Now().UnixNano())
	require.NoError(t, cardRepo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: requestID,
		RequestFingerprint: "daily-overage", ReservedAt: now.Add(time.Minute),
	}))

	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	result, err := NewUsageBillingRepositoryWithLedger(client, integrationDB, ledger).Apply(ctx, &service.UsageBillingCommand{
		RequestID: requestID, APIKeyID: apiKey.ID, UserID: user.ID, AccountID: 1,
		SubscriptionID: &sub.ID, SubscriptionEntitlementID: &card.ID,
		SubscriptionCost: 2, ActualCost: 2, BilledCost: 2,
	})

	require.NoError(t, err)
	require.True(t, result.DailyCardExhausted)
	require.True(t, result.BalanceOverdrafted)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, -0.75, balance, 0.000001)
}

func TestUsageBillingDailyCardExpiryWinsWhenSettlementAlsoExhaustsQuota(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	now := time.Now().UTC().Truncate(time.Second)
	user, group, plan := createDailyCardIntegrationCatalog(t, ctx, client)
	order := createDailyCardIntegrationOrder(t, ctx, client, user.ID, group.ID, plan.ID, now)
	cardRepo := NewDailyCardEntitlementRepository(client)
	card, _, err := cardRepo.IssuePaidCard(ctx, service.IssueDailyCardInput{
		UserID: user.ID, GroupID: group.ID, PlanID: plan.ID, PaymentOrderID: order.ID,
		QuotaLimitUSD: 1, DurationHours: 24, IssuedAt: now,
	})
	require.NoError(t, err)
	sub, err := client.UserSubscription.Create().SetUserID(user.ID).SetGroupID(group.ID).
		SetStartsAt(now).SetExpiresAt(now.Add(24 * time.Hour)).SetStatus(service.SubscriptionStatusActive).Save(ctx)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID, GroupID: &group.ID,
		Key: "sk-daily-earlier-of-" + fmt.Sprintf("%d", time.Now().UnixNano()), Name: "daily-earlier-of",
	})
	requestID := "daily-earlier-of-" + fmt.Sprintf("%d", time.Now().UnixNano())
	require.NoError(t, cardRepo.ReserveRequest(ctx, service.DailyCardRequestHoldInput{
		EntitlementID: card.ID, UserID: user.ID, RequestID: requestID,
		RequestFingerprint: "daily-earlier-of", ReservedAt: now.Add(time.Minute),
	}))
	expiredAt := time.Now().UTC().Add(-time.Second)
	_, err = integrationDB.ExecContext(ctx, "UPDATE subscription_entitlements SET expires_at = $1 WHERE id = $2", expiredAt, card.ID)
	require.NoError(t, err)
	beforeSettlement, err := client.SubscriptionEntitlement.Get(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusActive, beforeSettlement.Status)
	require.WithinDuration(t, expiredAt, *beforeSettlement.ExpiresAt, time.Millisecond)

	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, &service.UsageBillingCommand{
		RequestID: requestID, APIKeyID: apiKey.ID, UserID: user.ID, AccountID: 1,
		SubscriptionID: &sub.ID, SubscriptionEntitlementID: &card.ID,
		SubscriptionCost: 1, ActualCost: 1, BilledCost: 1,
	})
	require.NoError(t, err)
	require.False(t, result.DailyCardExhausted)
	settled, err := client.SubscriptionEntitlement.Get(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusExpired, settled.Status)
	require.WithinDuration(t, expiredAt, *settled.EndedAt, time.Millisecond)
}
