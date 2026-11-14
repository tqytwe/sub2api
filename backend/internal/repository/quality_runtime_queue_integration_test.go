//go:build integration

package repository

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/qualityqueue"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// integrationRedis belongs to the dedicated test fixture. Two independently
// pooled clients exercise the same cross-replica Lua scripts on real Redis.
func TestQualityRuntimeRedisConcurrentSignalsAndConditionalAcknowledgment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first := qualityqueue.NewRedis(integrationRedis)
	second := qualityqueue.NewRedis(integrationRedis)
	t.Cleanup(func() { _ = first.Close(); _ = second.Close() })
	id := time.Now().UnixNano()
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		require.NoError(t, integrationRedis.ZRem(cleanupCtx, qualityqueue.PendingKey, strconv.FormatInt(id, 10)).Err())
	})
	at := time.Now().Truncate(time.Millisecond)
	require.NoError(t, first.Enqueue(ctx, id, at))
	old := qualityqueue.Signal{AccountID: id, ObservedAt: at}
	var wg sync.WaitGroup
	errs := make(chan error, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			queue := first
			if index%2 != 0 {
				queue = second
			}
			errs <- queue.Enqueue(ctx, id, at)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	latest, err := first.Score(ctx, id)
	require.NoError(t, err)
	require.Equal(t, at.Add(30*time.Millisecond), latest, "same-millisecond observations retain distinct monotonically increasing generations")
	require.NoError(t, second.Acknowledge(ctx, old))
	retained, err := second.Score(ctx, id)
	require.NoError(t, err)
	require.Equal(t, latest, retained, "an old worker cannot delete signals from either replica")
	pending, err := first.Pending(ctx, at.Add(-time.Minute))
	require.NoError(t, err)
	matched := 0
	for _, signal := range pending {
		if signal.AccountID == id {
			matched++
			require.Equal(t, latest, signal.ObservedAt)
		}
	}
	require.Equal(t, 1, matched, "all repeated signals coalesce into one pending account")
	require.NoError(t, second.Acknowledge(ctx, qualityqueue.Signal{AccountID: id, ObservedAt: latest}))
	_, err = integrationRedis.ZScore(ctx, qualityqueue.PendingKey, strconv.FormatInt(id, 10)).Result()
	require.ErrorIs(t, err, redis.Nil, "acknowledging the exact generation removes only that account signal")
}
