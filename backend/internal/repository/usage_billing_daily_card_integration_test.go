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

func TestDailyCardEntitlementSettlementUpdateSQLPreparesInPostgres(t *testing.T) {
	ctx := context.Background()
	statementName := "daily_card_entitlement_settlement_update"
	_, err := integrationDB.ExecContext(ctx, "PREPARE "+statementName+" AS "+dailyCardEntitlementSettlementUpdateSQL)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DEALLOCATE "+statementName)
	})
}

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
	require.Equal(t, 10.0, hold.CapturedUsd)
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
		SubscriptionCost: 4, ActualCost: 4, BilledCost: 4,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.False(t, result.DailyCardExhausted)
	require.Nil(t, result.ActivatedEntitlementID)

	settledFirst, err := client.SubscriptionEntitlement.Get(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusExpired, settledFirst.Status)
	require.Equal(t, 4.0, settledFirst.QuotaUsedUsd)
	require.Zero(t, settledFirst.QuotaReservedUsd)
	settledSecond, err := client.SubscriptionEntitlement.Get(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, service.DailyCardStatusActive, settledSecond.Status)
	require.Zero(t, settledSecond.QuotaUsedUsd)
}
