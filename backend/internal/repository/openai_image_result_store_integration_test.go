//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImageResultStoreRealRedisPersistsCleanupMetadataPastAccessTTL(t *testing.T) {
	ctx := context.Background()
	rdb := integrationRedis
	store := NewOpenAIImageResultStore(rdb)
	cleanup, ok := store.(service.OpenAIImageResultCleanupStore)
	require.True(t, ok)
	now := time.Now().UTC()
	resultID := fmt.Sprintf("imgres_integration_cleanup_%d", now.UnixNano())
	record := &service.OpenAIImageResultRecord{
		ID:        resultID,
		UserID:    7,
		APIKeyID:  9,
		CreatedAt: now.Add(-time.Minute).Unix(),
		ExpiresAt: now.Add(-time.Second).Unix(),
		Assets: []service.OpenAIImageResultAsset{{
			Key:         "images/results/" + resultID + "-0.png",
			ContentType: "image/png",
		}},
	}
	t.Cleanup(func() {
		_ = cleanup.Delete(context.Background(), resultID)
	})

	require.NoError(t, store.Save(ctx, record, 100*time.Millisecond))
	time.Sleep(150 * time.Millisecond)
	_, err := store.Get(ctx, record.ID)
	require.ErrorIs(t, err, service.ErrOpenAIImageResultNotFound)

	expired, err := cleanup.ListExpired(ctx, now, 10)
	require.NoError(t, err)
	require.Equal(t, []*service.OpenAIImageResultRecord{record}, expired)
	require.NoError(t, cleanup.Delete(ctx, record.ID))

	expired, err = cleanup.ListExpired(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, expired)
}
