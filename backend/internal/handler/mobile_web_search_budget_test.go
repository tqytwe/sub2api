package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisMobileWebSearchBudgetEnforcesLimitsWithoutPartialConsumption(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Date(2026, time.August, 4, 9, 30, 15, 0, time.UTC)
	budget := &redisMobileWebSearchBudget{
		client: client,
		config: mobileWebSearchBudgetConfig{
			UserRPM:     1,
			UserDaily:   5,
			GlobalRPM:   10,
			GlobalDaily: 20,
		},
		now: func() time.Time { return now },
	}

	retryAfter, err := budget.Reserve(context.Background(), 7)
	require.NoError(t, err)
	require.Zero(t, retryAfter)

	retryAfter, err = budget.Reserve(context.Background(), 7)
	require.Error(t, err)
	var exceeded *mobileWebSearchBudgetExceededError
	require.True(t, errors.As(err, &exceeded))
	require.Greater(t, retryAfter, time.Duration(0))

	// A user-bucket rejection must not consume another global call.
	globalMinuteKey := "{mobile-web-search}:global:minute:202608040930"
	value, getErr := mini.Get(globalMinuteKey)
	require.NoError(t, getErr)
	require.Equal(t, "1", value)
}

func TestRedisMobileWebSearchBudgetFailsClosedWhenRedisCannotBeReached(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		MaxRetries:   0,
		DialTimeout:  20 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	budget := newRedisMobileWebSearchBudget(client)
	_, err := budget.Reserve(context.Background(), 7)
	require.ErrorIs(t, err, errMobileWebSearchBudgetUnavailable)
}
