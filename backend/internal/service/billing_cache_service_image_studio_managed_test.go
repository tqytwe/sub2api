//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageStudioManagedEligibilityCacheStub struct {
	BillingCache

	balanceCalls      atomic.Int64
	subscriptionCalls atomic.Int64
	quotaEntry        *UserPlatformQuotaCacheEntry
	rateLimitEntry    *APIKeyRateLimitCacheData
}

type imageStudioManagedEligibilityQuotaRepoStub struct {
	UserPlatformQuotaRepository
}

func (s *imageStudioManagedEligibilityCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	s.balanceCalls.Add(1)
	return 0, nil
}

func (s *imageStudioManagedEligibilityCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	s.subscriptionCalls.Add(1)
	return &SubscriptionCacheData{Status: "expired"}, nil
}

func (s *imageStudioManagedEligibilityCacheStub) GetUserPlatformQuotaCache(context.Context, int64, string) (*UserPlatformQuotaCacheEntry, bool, error) {
	if s.quotaEntry == nil {
		return nil, false, nil
	}
	return s.quotaEntry, true, nil
}

func (s *imageStudioManagedEligibilityCacheStub) SetUserPlatformQuotaCache(context.Context, int64, string, *UserPlatformQuotaCacheEntry, time.Duration) error {
	return nil
}

func (s *imageStudioManagedEligibilityCacheStub) GetAPIKeyRateLimit(context.Context, int64) (*APIKeyRateLimitCacheData, error) {
	return s.rateLimitEntry, nil
}

func (s *imageStudioManagedEligibilityCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error {
	return nil
}

func TestCheckImageStudioManagedEligibility_SkipsBalanceAndSubscription(t *testing.T) {
	cache := &imageStudioManagedEligibilityCacheStub{}
	svc := NewBillingCacheService(
		cache,
		nil,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
		&imageStudioManagedEligibilityQuotaRepoStub{},
	)
	t.Cleanup(svc.Stop)

	err := svc.CheckImageStudioManagedEligibility(
		context.Background(),
		&User{ID: 41},
		nil,
		&Group{ID: 42, SubscriptionType: "subscription"},
		&UserSubscription{Status: "expired"},
		"",
	)

	require.NoError(t, err)
	require.Zero(t, cache.balanceCalls.Load())
	require.Zero(t, cache.subscriptionCalls.Load())
}

func TestCheckImageStudioManagedEligibility_EnforcesUserPlatformQuota(t *testing.T) {
	zero := 0.0
	now := time.Now()
	cache := &imageStudioManagedEligibilityCacheStub{
		quotaEntry: &UserPlatformQuotaCacheEntry{
			DailyLimitUSD:    &zero,
			DailyWindowStart: &now,
			SchemaVersion:    UserPlatformQuotaCacheSchemaV1,
		},
	}
	svc := NewBillingCacheService(
		cache,
		nil,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
		&imageStudioManagedEligibilityQuotaRepoStub{},
	)
	t.Cleanup(svc.Stop)

	err := svc.CheckImageStudioManagedEligibility(
		context.Background(),
		&User{ID: 51},
		nil,
		nil,
		nil,
		PlatformOpenAI,
	)

	require.ErrorIs(t, err, ErrUserPlatformDailyQuotaExhausted)
	require.Zero(t, cache.balanceCalls.Load())
}

func TestCheckImageStudioManagedEligibility_EnforcesAPIKeyRateLimits(t *testing.T) {
	now := time.Now()
	cache := &imageStudioManagedEligibilityCacheStub{
		rateLimitEntry: &APIKeyRateLimitCacheData{
			Usage5h:  1,
			Window5h: now.Unix(),
		},
	}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckImageStudioManagedEligibility(
		context.Background(),
		&User{ID: 61},
		&APIKey{ID: 62, RateLimit5h: 1},
		nil,
		nil,
		"",
	)

	require.ErrorIs(t, err, ErrAPIKeyRateLimit5hExceeded)
	require.Zero(t, cache.balanceCalls.Load())
}

func TestCheckImageStudioManagedEligibility_EnforcesRPMHierarchy(t *testing.T) {
	rpmCache := &userRPMCacheStub{userGroupCounts: []int{2}}
	svc := NewBillingCacheService(nil, nil, nil, nil, rpmCache, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckImageStudioManagedEligibility(
		context.Background(),
		&User{ID: 71},
		nil,
		&Group{ID: 72, RPMLimit: 1},
		nil,
		"",
	)

	require.ErrorIs(t, err, ErrGroupRPMExceeded)
	require.EqualValues(t, 1, rpmCache.userGroupCalls)
}

func TestCheckImageStudioManagedEligibility_EnforcesCircuitBreaker(t *testing.T) {
	cfg := &config.Config{}
	cfg.Billing.CircuitBreaker.Enabled = true
	svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	svc.circuitBreaker.state = billingCircuitOpen
	svc.circuitBreaker.openedAt = time.Now()

	err := svc.CheckImageStudioManagedEligibility(
		context.Background(),
		&User{ID: 81},
		nil,
		nil,
		nil,
		"",
	)

	require.ErrorIs(t, err, ErrBillingServiceUnavailable)
}
