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

func TestOpenAIImageResultStoreRoundTripAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewOpenAIImageResultStore(rdb)
	record := &service.OpenAIImageResultRecord{
		ID:        "imgres_123",
		UserID:    7,
		APIKeyID:  9,
		CreatedAt: 100,
		ExpiresAt: 200,
		Assets: []service.OpenAIImageResultAsset{{
			Key:         "images/results/imgres_123-0.png",
			ContentType: "image/png",
		}},
	}

	require.NoError(t, store.Save(context.Background(), record, time.Hour))
	got, err := store.Get(context.Background(), record.ID)
	require.NoError(t, err)
	require.Equal(t, record, got)
	require.Equal(t, time.Hour, mr.TTL(openAIImageResultKey(record.ID)))
	exists, err := rdb.HExists(context.Background(), openAIImageResultCleanupRecordsKey, record.ID).Result()
	require.NoError(t, err)
	require.True(t, exists)
	score, err := rdb.ZScore(context.Background(), openAIImageResultCleanupIndexKey, record.ID).Result()
	require.NoError(t, err)
	require.Equal(t, float64(record.ExpiresAt), score)
}

func TestOpenAIImageResultStoreMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	_, err := NewOpenAIImageResultStore(rdb).Get(context.Background(), "imgres_missing")

	require.ErrorIs(t, err, service.ErrOpenAIImageResultNotFound)
}

func TestOpenAIImageResultStoreListsAndDeletesExpiredCleanupRecords(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewOpenAIImageResultStore(rdb)
	cleanup, ok := store.(service.OpenAIImageResultCleanupStore)
	require.True(t, ok)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	expired := &service.OpenAIImageResultRecord{
		ID:        "imgres_expired",
		ExpiresAt: now.Add(-time.Minute).Unix(),
		Assets:    []service.OpenAIImageResultAsset{{Key: "images/results/expired.png"}},
	}
	future := &service.OpenAIImageResultRecord{
		ID:        "imgres_future",
		ExpiresAt: now.Add(time.Hour).Unix(),
		Assets:    []service.OpenAIImageResultAsset{{Key: "images/results/future.png"}},
	}
	require.NoError(t, store.Save(context.Background(), expired, time.Hour))
	require.NoError(t, store.Save(context.Background(), future, time.Hour))

	records, err := cleanup.ListExpired(context.Background(), now, 10)
	require.NoError(t, err)
	require.Equal(t, []*service.OpenAIImageResultRecord{expired}, records)

	require.NoError(t, cleanup.Delete(context.Background(), expired.ID))
	exists, err := rdb.HExists(context.Background(), openAIImageResultCleanupRecordsKey, expired.ID).Result()
	require.NoError(t, err)
	require.False(t, exists)
	_, err = rdb.ZScore(context.Background(), openAIImageResultCleanupIndexKey, expired.ID).Result()
	require.Error(t, err)
	_, err = store.Get(context.Background(), expired.ID)
	require.ErrorIs(t, err, service.ErrOpenAIImageResultNotFound)
}
