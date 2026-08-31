package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type opsAggregationWatermarkRepoStub struct {
	OpsRepository
	advanced     []time.Time
	advanceErr   error
	upserts      []opsAggregationWindow
	failUpsertAt int
}

type opsAggregationWindow struct {
	start time.Time
	end   time.Time
}

func (s *opsAggregationWatermarkRepoStub) AdvanceHourlyAggregationWatermark(_ context.Context, completedThrough time.Time) error {
	s.advanced = append(s.advanced, completedThrough.UTC())
	return s.advanceErr
}

func (s *opsAggregationWatermarkRepoStub) UpsertHourlyMetrics(_ context.Context, start, end time.Time) error {
	s.upserts = append(s.upserts, opsAggregationWindow{start: start.UTC(), end: end.UTC()})
	if s.failUpsertAt > 0 && len(s.upserts) == s.failUpsertAt {
		return errors.New("hourly chunk failed")
	}
	return nil
}

func TestOpsAggregationFinalizesPublicStatusWatermarkOnlyAfterSuccessfulHourlyAggregation(t *testing.T) {
	completedThrough := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &opsAggregationWatermarkRepoStub{}
	svc := &OpsAggregationService{opsRepo: repo}
	aggregationErr := errors.New("hourly chunk failed")

	require.ErrorIs(t, svc.finalizeHourlyAggregation(context.Background(), completedThrough, aggregationErr), aggregationErr)
	require.Empty(t, repo.advanced)

	require.NoError(t, svc.finalizeHourlyAggregation(context.Background(), completedThrough, nil))
	require.Equal(t, []time.Time{completedThrough}, repo.advanced)
}

func TestOpsAggregationFinalizationFailsClosedWhenWatermarkCannotAdvance(t *testing.T) {
	completedThrough := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &opsAggregationWatermarkRepoStub{advanceErr: errors.New("watermark write failed")}
	svc := &OpsAggregationService{opsRepo: repo}

	err := svc.finalizeHourlyAggregation(context.Background(), completedThrough, nil)
	require.ErrorContains(t, err, "watermark write failed")
	require.Equal(t, []time.Time{completedThrough}, repo.advanced)
}

func TestOpsAggregationHourlyWindowAdvancesWatermarkWhenThereIsNoPendingWork(t *testing.T) {
	completedThrough := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &opsAggregationWatermarkRepoStub{}
	svc := &OpsAggregationService{opsRepo: repo}

	require.NoError(t, svc.processHourlyWindow(context.Background(), completedThrough, completedThrough))
	require.Equal(t, []time.Time{completedThrough}, repo.advanced)
}

func TestOpsAggregationHourlyWindowRejectsPartialHourBoundaries(t *testing.T) {
	start := time.Date(2026, 8, 29, 13, 30, 0, 0, time.UTC)
	end := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	repo := &opsAggregationWatermarkRepoStub{}
	svc := &OpsAggregationService{opsRepo: repo}

	err := svc.processHourlyWindow(context.Background(), start, end)
	require.ErrorContains(t, err, "whole UTC hours")
	require.Empty(t, repo.upserts)
	require.Empty(t, repo.advanced)
}

func TestOpsAggregationHourlyWindowAdvancesAfterSuccessfulNoRowChunks(t *testing.T) {
	start := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	repo := &opsAggregationWatermarkRepoStub{}
	svc := &OpsAggregationService{opsRepo: repo}

	require.NoError(t, svc.processHourlyWindow(context.Background(), start, end))
	require.Equal(t, []opsAggregationWindow{{start: start, end: end}}, repo.upserts)
	require.Equal(t, []time.Time{end}, repo.advanced)
}

func TestOpsAggregationHourlyWindowDoesNotAdvanceAfterLaterChunkFailure(t *testing.T) {
	start := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	repo := &opsAggregationWatermarkRepoStub{failUpsertAt: 2}
	svc := &OpsAggregationService{opsRepo: repo}

	err := svc.processHourlyWindow(context.Background(), start, end)
	require.ErrorContains(t, err, "hourly chunk failed")
	require.Equal(t, []opsAggregationWindow{
		{start: start, end: start.Add(opsAggHourlyChunk)},
		{start: start.Add(opsAggHourlyChunk), end: end},
	}, repo.upserts)
	require.Empty(t, repo.advanced)
}

func TestHourlyAggregationWindowBootstrapsCompleteAvailabilityAndBoundsCatchup(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 3, 0, 0, time.UTC)

	start, end := hourlyAggregationWindow(now, time.Time{}, false)
	require.Equal(t, time.Date(2026, 7, 30, 13, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC), end)
	require.Equal(t, opsAggHourlyBootstrapWindow, end.Sub(start))

	watermark := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	start, end = hourlyAggregationWindow(now, watermark, true)
	require.Equal(t, time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC), end)

	staleWatermark := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	start, end = hourlyAggregationWindow(now, staleWatermark, true)
	require.Equal(t, staleWatermark.Add(-opsAggHourlyOverlap), start)
	require.Equal(t, staleWatermark.Add(opsAggHourlyCatchupWindow), end)
}
