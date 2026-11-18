package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPriorityStickyCleanupPreservesOwnersAndOtherGroups(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := &gatewayCache{rdb: client}
	ctx := context.Background()
	for key, id := range map[string]int64{"openai:hash-a": 7, "openai:hash-b": 8, "openai:response:resp": 7, "openai:http-response-owner:resp": 7} {
		require.NoError(t, cache.SetSessionAccountID(ctx, 1, key, id, time.Hour))
	}
	require.NoError(t, cache.SetSessionAccountID(ctx, 2, "openai:hash-c", 7, time.Hour))
	counts, err := cache.CountStickySessionBindings(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, map[int64]int{7: 1, 8: 1}, counts)
	n, err := cache.ClearStickySessionBindings(ctx, 1, map[int64]struct{}{7: {}})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	for _, key := range []string{"openai:hash-b", "openai:response:resp", "openai:http-response-owner:resp"} {
		_, err = cache.GetSessionAccountID(ctx, 1, key)
		require.NoError(t, err)
	}
	_, err = cache.GetSessionAccountID(ctx, 2, "openai:hash-c")
	require.NoError(t, err)
	n, err = cache.ClearStickySessionBindings(ctx, 1, map[int64]struct{}{})
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = cache.ClearStickySessionBindings(ctx, 1, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}
