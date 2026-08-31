package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type publicStatusSummaryRepoStub struct {
	raw PublicStatusSummaryRaw
	err error
}

func (s *publicStatusSummaryRepoStub) GetPublicStatusSummary(context.Context) (PublicStatusSummaryRaw, error) {
	return s.raw, s.err
}

func (s *publicStatusSummaryRepoStub) ListPublicStatusSnapshotWindowEnds(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}

func (s *publicStatusSummaryRepoStub) IsPublicStatusSnapshotWindowReady(context.Context, time.Time) (bool, error) {
	return false, nil
}

func (s *publicStatusSummaryRepoStub) RefreshPublicStatusSnapshot(context.Context, time.Time) error {
	return nil
}

func TestPublicStatusSummaryUsesDataThroughAndRealSampleCounts(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	through := now.Add(-90 * time.Minute)
	availability := 99.5
	p50 := 420.0
	p95 := 900.0
	svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{raw: PublicStatusSummaryRaw{
		HasSnapshot:             true,
		TotalRequests:           901,
		AvailabilityPct:         &availability,
		AvailabilitySampleCount: 800,
		TTFTP50Ms:               &p50,
		TTFTP95Ms:               &p95,
		TTFTSampleCount:         120,
		WindowEnd:               now.Truncate(time.Hour),
		DataThrough:             &through,
		ComputedAt:              now.Add(-5 * time.Minute),
	}})
	svc.now = func() time.Time { return now }

	got, err := svc.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(901), got.TotalRequests)
	require.Equal(t, int64(800), got.Availability.SampleCount)
	require.Equal(t, int64(120), got.TTFT.SampleCount)
	require.Equal(t, &p50, got.TTFT.P50Ms)
	require.Equal(t, &p95, got.TTFT.P95Ms)
	require.Equal(t, now.Truncate(time.Hour).Add(-30*24*time.Hour), *got.Availability.WindowStart)
	require.Equal(t, now.Truncate(time.Hour), *got.Availability.WindowEnd)
	require.Equal(t, now.Truncate(time.Hour).Add(-24*time.Hour), *got.TTFT.WindowStart)
	require.Equal(t, now.Truncate(time.Hour), *got.TTFT.WindowEnd)
	require.Equal(t, PublicStatusFreshnessFresh, got.Freshness)
}

func TestPublicStatusSummaryMarksDelayedAndUnavailableFromBusinessWatermark(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	availability := 99.5
	p50 := 420.0
	p95 := 900.0
	base := PublicStatusSummaryRaw{
		HasSnapshot:             true,
		AvailabilityPct:         &availability,
		AvailabilitySampleCount: 80,
		TTFTP50Ms:               &p50,
		TTFTP95Ms:               &p95,
		TTFTSampleCount:         12,
		WindowEnd:               now.Truncate(time.Hour),
		ComputedAt:              now,
	}

	for _, tt := range []struct {
		name      string
		through   time.Time
		freshness PublicStatusFreshness
	}{
		{name: "delayed", through: now.Add(-publicStatusDataDelay - time.Second), freshness: PublicStatusFreshnessDelayed},
		{name: "unavailable after six hours", through: now.Add(-publicStatusUnavailable - time.Second), freshness: PublicStatusFreshnessUnavailable},
		{name: "unavailable when future", through: now.Add(publicStatusFutureLimit + time.Second), freshness: PublicStatusFreshnessUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := base
			raw.DataThrough = &tt.through
			svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{raw: raw})
			svc.now = func() time.Time { return now }
			got, err := svc.Get(t.Context())
			require.NoError(t, err)
			require.Equal(t, tt.freshness, got.Freshness)
		})
	}
}

func TestPublicStatusSummaryFreshnessIgnoresSnapshotComputeClock(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	through := now.Add(-2 * time.Hour)
	availability := 99.5
	p50 := 420.0
	p95 := 900.0
	svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{raw: PublicStatusSummaryRaw{
		HasSnapshot:             true,
		AvailabilityPct:         &availability,
		AvailabilitySampleCount: 10,
		TTFTP50Ms:               &p50,
		TTFTP95Ms:               &p95,
		TTFTSampleCount:         10,
		WindowEnd:               now.Truncate(time.Hour),
		DataThrough:             &through,
		// A recently computed row can still be delayed when the business
		// watermark is behind; computed_at must never make it look fresh.
		ComputedAt: now.Add(-time.Minute),
	}})
	svc.now = func() time.Time { return now }

	got, err := svc.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, PublicStatusFreshnessDelayed, got.Freshness)
}

func TestPublicStatusSummaryIsUnavailableWhenSnapshotOrSamplesAreMissing(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	through := now.Add(-time.Hour)
	for _, raw := range []PublicStatusSummaryRaw{
		{},
		{HasSnapshot: true, AvailabilitySampleCount: 1, DataThrough: &through},
		{HasSnapshot: true, AvailabilitySampleCount: 1, TTFTSampleCount: 1},
	} {
		svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{raw: raw})
		svc.now = func() time.Time { return now }
		got, err := svc.Get(t.Context())
		require.NoError(t, err)
		require.Equal(t, PublicStatusFreshnessUnavailable, got.Freshness)
	}
}

func TestPublicStatusSummaryDoesNotInventMeasurementWindows(t *testing.T) {
	now := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	through := now.Add(-time.Hour)
	svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{raw: PublicStatusSummaryRaw{
		HasSnapshot:             true,
		AvailabilitySampleCount: 1,
		TTFTSampleCount:         1,
		DataThrough:             &through,
		// A missing snapshot boundary must be observable as absent; the API may
		// not manufacture a start/end interval from the request clock.
		WindowEnd: time.Time{},
	}})
	svc.now = func() time.Time { return now }

	got, err := svc.Get(t.Context())
	require.NoError(t, err)
	require.Nil(t, got.Availability.WindowStart)
	require.Nil(t, got.Availability.WindowEnd)
	require.Nil(t, got.TTFT.WindowStart)
	require.Nil(t, got.TTFT.WindowEnd)
	require.Equal(t, PublicStatusFreshnessUnavailable, got.Freshness)
}

func TestPublicStatusSummaryPropagatesRepositoryFailures(t *testing.T) {
	svc := NewPublicStatusSummaryService(&publicStatusSummaryRepoStub{err: errors.New("database unavailable")})
	_, err := svc.Get(t.Context())
	require.ErrorContains(t, err, "database unavailable")
}
