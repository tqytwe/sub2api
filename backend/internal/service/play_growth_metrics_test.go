package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type failingPublicMetricsRepository struct{}

func (f failingPublicMetricsRepository) GetLatestPublicMetricSnapshot(context.Context) (*PublicMetricSnapshot, error) {
	return nil, errors.New("snapshot storage unavailable")
}

func (f failingPublicMetricsRepository) RefreshPublicMetricSnapshot(context.Context, time.Time) (*PublicMetricSnapshot, error) {
	return nil, errors.New("snapshot refresh unavailable")
}

func (f failingPublicMetricsRepository) ListPublicActivity(context.Context, int, int) ([]PlayPublicActivity, error) {
	return nil, nil
}

func (f failingPublicMetricsRepository) RefreshPlayUsageAggregates(context.Context, time.Time, float64, float64, int64, int64, int, int) error {
	return nil
}

func TestPublicMetricSnapshotServiceReturnsMarkedFallback(t *testing.T) {
	svc := NewPublicMetricSnapshotService(failingPublicMetricsRepository{})

	snapshot, err := svc.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, "estimated", snapshot.Source)
	require.Contains(t, snapshot.SnapshotID, ":unavailable")
	require.Zero(t, snapshot.Requests24h)
	require.Nil(t, snapshot.SuccessRate30d)
}
