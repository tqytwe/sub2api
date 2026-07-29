package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExtendSubscription_DailyCardAdjustsEntitlementExpiry(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        101,
		UserID:    10,
		GroupID:   20,
		StartsAt:  now,
		ExpiresAt: expiresAt,
		Status:    SubscriptionStatusActive,
	})
	cardRepo := &dailyCardRepoStub{
		oneTimeGroup: true,
		listed: []DailyCardEntitlement{{
			ID:        88,
			UserID:    10,
			GroupID:   20,
			Status:    DailyCardStatusActive,
			StartsAt:  &now,
			ExpiresAt: &expiresAt,
			CreatedAt: now,
		}},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, subRepo, nil, nil, nil)
	svc.SetDailyCardService(NewDailyCardService(cardRepo))

	got, err := svc.ExtendSubscription(context.Background(), 101, 1)

	require.NoError(t, err)
	wantExpiresAt := expiresAt.AddDate(0, 0, 1)
	require.Equal(t, []int64{88, 10, 20}, cardRepo.adminAdjustInput)
	require.Equal(t, wantExpiresAt, cardRepo.adminAdjustExpiresAt)
	require.Equal(t, wantExpiresAt, got.ExpiresAt)
	require.NotNil(t, got.DailyCard)
	require.Equal(t, wantExpiresAt, *got.DailyCard.ExpiresAt)
}

func TestExtendSubscription_ExpiredDailyCardRestoresEntitlementActive(t *testing.T) {
	now := time.Now().UTC()
	expiredAt := now.Add(-time.Hour)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        102,
		UserID:    11,
		GroupID:   21,
		StartsAt:  now.Add(-25 * time.Hour),
		ExpiresAt: expiredAt,
		Status:    SubscriptionStatusExpired,
	})
	cardRepo := &dailyCardRepoStub{
		oneTimeGroup: true,
		listed: []DailyCardEntitlement{{
			ID:        89,
			UserID:    11,
			GroupID:   21,
			Status:    DailyCardStatusExpired,
			StartsAt:  ptrDailyCardTime(now.Add(-25 * time.Hour)),
			ExpiresAt: &expiredAt,
			EndedAt:   &expiredAt,
			CreatedAt: now.Add(-25 * time.Hour),
		}},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, subRepo, nil, nil, nil)
	svc.SetDailyCardService(NewDailyCardService(cardRepo))
	before := time.Now().UTC()

	got, err := svc.ExtendSubscription(context.Background(), 102, 1)

	require.NoError(t, err)
	require.Equal(t, []int64{89, 11, 21}, cardRepo.adminAdjustInput)
	require.True(t, cardRepo.adminAdjustExpiresAt.After(before.Add(23*time.Hour)))
	require.Equal(t, SubscriptionStatusActive, got.Status)
	require.NotNil(t, got.DailyCard)
	require.Equal(t, DailyCardStatusActive, got.DailyCard.Status)
	require.Nil(t, got.DailyCard.EndedAt)
}
