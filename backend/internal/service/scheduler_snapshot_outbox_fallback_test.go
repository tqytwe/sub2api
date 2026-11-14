package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fallbackOutboxCache struct {
	*outboxCleanupCache
	calls []int64
}

func (c *fallbackOutboxCache) UpdateLastUsed(_ context.Context, updates map[int64]time.Time) error {
	for id := range updates {
		c.calls = append(c.calls, id)
		if id == 2 {
			return errors.New("synthetic cache outage")
		}
	}
	return nil
}

// Repositories predating the retry interface must retain the failed event,
// checkpoint the completed prefix and avoid replaying it on every poll.
func TestSchedulerOutboxWithoutRetryRepositoryCommitsPrefix(t *testing.T) {
	cache := &fallbackOutboxCache{outboxCleanupCache: &outboxCleanupCache{}}
	repo := &outboxCleanupRepo{events: []SchedulerOutboxEvent{
		{ID: 1, EventType: SchedulerOutboxEventAccountLastUsed, Payload: map[string]any{"last_used": map[string]any{"1": float64(123)}}},
		{ID: 2, EventType: SchedulerOutboxEventAccountLastUsed, Payload: map[string]any{"last_used": map[string]any{"2": float64(123)}}},
		{ID: 3, EventType: SchedulerOutboxEventAccountLastUsed, Payload: map[string]any{"last_used": map[string]any{"3": float64(123)}}},
	}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)
	svc.pollOutbox()
	require.Equal(t, int64(1), cache.watermark)
	require.Equal(t, []int64{1, 2}, cache.calls)
	svc.pollOutbox()
	require.Equal(t, []int64{1, 2}, cache.calls, "backoff must also apply without durable retry support")
}
