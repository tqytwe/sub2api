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
}

func TestOpenAIImageResultStoreMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	_, err := NewOpenAIImageResultStore(rdb).Get(context.Background(), "imgres_missing")

	require.ErrorIs(t, err, service.ErrOpenAIImageResultNotFound)
}
