package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type publicStatusSnapshotWorkerRepoStub struct {
	mu             sync.Mutex
	readiness      map[time.Time]bool
	readinessError error
	refreshes      []time.Time
}

func (s *publicStatusSnapshotWorkerRepoStub) GetPublicStatusSummary(context.Context) (PublicStatusSummaryRaw, error) {
	return PublicStatusSummaryRaw{}, nil
}

func (s *publicStatusSnapshotWorkerRepoStub) ListPublicStatusSnapshotWindowEnds(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}

func (s *publicStatusSnapshotWorkerRepoStub) IsPublicStatusSnapshotWindowReady(_ context.Context, windowEnd time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readinessError != nil {
		return false, s.readinessError
	}
	return s.readiness[windowEnd.UTC().Truncate(time.Hour)], nil
}

func (s *publicStatusSnapshotWorkerRepoStub) RefreshPublicStatusSnapshot(_ context.Context, windowEnd time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshes = append(s.refreshes, windowEnd.UTC().Truncate(time.Hour))
	return nil
}

func TestPublicStatusSnapshotScheduleUsesCompletedHoursAfterSafeDelay(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 3, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC), publicStatusSnapshotWindowEnd(now))
	require.Equal(t, time.Date(2026, 8, 29, 14, 5, 0, 0, time.UTC), nextPublicStatusSnapshotRun(now))

	afterDelay := time.Date(2026, 8, 29, 14, 6, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC), publicStatusSnapshotWindowEnd(afterDelay))
	require.Equal(t, time.Date(2026, 8, 29, 14, 10, 0, 0, time.UTC), nextPublicStatusSnapshotRun(afterDelay))
}

func TestPublicStatusSnapshotCatchupPrioritizesNewestMissingWindowsAndCapsWork(t *testing.T) {
	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	existing := []time.Time{
		windowEnd.Add(-time.Hour),
		windowEnd.Add(-3 * time.Hour),
	}

	got := publicStatusSnapshotRefreshWindowEnds(windowEnd, existing)

	require.Equal(t, []time.Time{
		windowEnd,
		windowEnd.Add(-2 * time.Hour),
		windowEnd.Add(-4 * time.Hour),
		windowEnd.Add(-5 * time.Hour),
	}, got)
	require.Len(t, got, publicStatusSnapshotMaxCatchupWindowsPerRun)
}

func TestPublicStatusSnapshotCatchupDoesNotScanOutsideItsBoundedRecentWindow(t *testing.T) {
	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	oldestIncluded := windowEnd.Add(-(publicStatusSnapshotCatchupLookback - time.Hour))

	got := publicStatusSnapshotRefreshWindowEnds(windowEnd, []time.Time{
		windowEnd,
		windowEnd.Add(-time.Hour),
		windowEnd.Add(-2 * time.Hour),
		windowEnd.Add(-3 * time.Hour),
		windowEnd.Add(-4 * time.Hour),
		windowEnd.Add(-5 * time.Hour),
		oldestIncluded.Add(-time.Hour), // Old snapshots do not expand the query window.
	})

	require.NotContains(t, got, oldestIncluded.Add(-time.Hour))
	for _, end := range got {
		require.False(t, end.Before(oldestIncluded))
		require.False(t, end.After(windowEnd))
	}
}

func TestPublicStatusSnapshotWorkerWaitsForAggregationWatermarkBeforeWriting(t *testing.T) {
	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &publicStatusSnapshotWorkerRepoStub{readiness: map[time.Time]bool{}}
	worker := &PublicStatusSnapshotWorker{repo: repo}

	// Keep only the target in the bounded candidate list. A missing completion
	// watermark must not invoke the immutable snapshot insert.
	for i := 1; i < 6; i++ {
		repoWindow := windowEnd.Add(-time.Duration(i) * time.Hour)
		repo.readiness[repoWindow] = false
	}
	require.NoError(t, worker.refreshPendingWindows(context.Background(), windowEnd))
	require.Empty(t, repo.refreshes)

	// Once the aggregation job publishes its completion watermark, the same
	// candidate is retried and may be inserted.
	repo.readiness[windowEnd] = true
	require.NoError(t, worker.refreshPendingWindows(context.Background(), windowEnd))
	require.Equal(t, []time.Time{windowEnd}, repo.refreshes)
}

func TestPublicStatusSnapshotWorkerFailsClosedWhenWatermarkCannotBeRead(t *testing.T) {
	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &publicStatusSnapshotWorkerRepoStub{
		readiness:      map[time.Time]bool{},
		readinessError: errors.New("watermark unavailable"),
	}
	worker := &PublicStatusSnapshotWorker{repo: repo}

	err := worker.refreshPendingWindows(context.Background(), windowEnd)
	require.ErrorContains(t, err, "watermark unavailable")
	require.Empty(t, repo.refreshes)
}
