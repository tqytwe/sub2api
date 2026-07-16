package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type publicStatsRepoStub struct {
	raw PublicHomeStatsRaw
	err error
}

func (s *publicStatsRepoStub) GetPublicHomeStats(context.Context, time.Time) (PublicHomeStatsRaw, error) {
	return s.raw, s.err
}

func TestPublicHomeStatsUsesWeightedTTFTAndSLACounts(t *testing.T) {
	repo := &publicStatsRepoStub{raw: PublicHomeStatsRaw{
		TotalRequests:      120,
		Success30d:         99,
		ErrorSLA30d:        1,
		TTFTWeightedSum24h: 6000,
		TTFTSamples24h:     10,
	}}
	svc := NewPublicHomeStatsService(repo)
	computedAt := time.Date(2026, 7, 16, 2, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return computedAt }

	got, err := svc.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(120), got.TotalRequests)
	require.InEpsilon(t, 99.0, *got.AvailabilityPct, 1e-9)
	require.InEpsilon(t, 600.0, *got.AvgTTFTMs, 1e-9)
	require.Equal(t, computedAt, got.ComputedAt)
}

func TestPublicHomeStatsReturnsNullWithoutDenominators(t *testing.T) {
	through := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	repo := &publicStatsRepoStub{raw: PublicHomeStatsRaw{
		TotalRequests:  7,
		OpsDataThrough: &through,
	}}
	svc := NewPublicHomeStatsService(repo)

	got, err := svc.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(7), got.TotalRequests)
	require.Nil(t, got.AvailabilityPct)
	require.Nil(t, got.AvgTTFTMs)
	require.Equal(t, &through, got.OpsDataThrough)
}

func TestPublicHomeStatsPropagatesRepositoryFailure(t *testing.T) {
	svc := NewPublicHomeStatsService(&publicStatsRepoStub{err: errors.New("database unavailable")})

	_, err := svc.Get(t.Context())
	require.ErrorContains(t, err, "database unavailable")
}
