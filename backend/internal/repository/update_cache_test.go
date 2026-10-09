package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUpdateCacheSeparatesLegacyAndSourceIdentity(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "update:latest", "legacy Wei-Shaw release", time.Hour).Err())
	cache := NewUpdateCache(client)
	_, err := cache.GetUpdateInfo(ctx)
	require.ErrorIs(t, err, redis.Nil)
	require.NoError(t, cache.SetUpdateInfo(ctx, "ranxi discovery only", time.Minute))
	value, err := server.Get("update:latest:" + service.UpdateSourceIdentity)
	require.NoError(t, err)
	require.Equal(t, "ranxi discovery only", value)
}
