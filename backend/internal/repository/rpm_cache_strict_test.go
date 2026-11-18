package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRPMCacheStrictAcquireConcurrentCapAndMinuteReset(t *testing.T) {
	server := miniredis.RunT(t)
	now := time.Unix(1_800_000_010, 0)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &RPMCacheImpl{rdb: client}
	const limit = 12
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 80 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, count, resetAt, err := cache.TryAcquireRPM(context.Background(), 42, limit)
			if err != nil {
				t.Errorf("acquire failed: %v", err)
				return
			}
			if count > limit {
				t.Errorf("count %d exceeds limit %d", count, limit)
			}
			if !resetAt.Equal(now.Truncate(time.Minute).Add(time.Minute)) {
				t.Errorf("unexpected reset time %s", resetAt)
			}
			if allowed {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, limit, accepted.Load())
	count, err := cache.GetRPM(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, limit, count)

	server.SetTime(now.Add(time.Minute))
	allowed, count, _, err := cache.TryAcquireRPM(context.Background(), 42, limit)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, count)
}
