package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPublicHomeStatsRepositoryUsesExactOverallQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 7, 16, 2, 30, 0, 0, time.UTC)
	throughLocation := time.FixedZone("UTC+8", 8*60*60)
	through := time.Date(2026, 7, 16, 10, 0, 0, 0, throughLocation)
	mock.ExpectQuery(publicHomeStatsQuery).
		WithArgs(now.Add(-30*24*time.Hour), now.Add(-24*time.Hour), now).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests",
			"success_30d",
			"error_sla_30d",
			"ttft_weighted_sum_24h",
			"ttft_samples_24h",
			"ops_data_through",
		}).AddRow(int64(321), int64(98), int64(2), float64(4250), int64(10), through))

	repo := NewPublicHomeStatsRepository(db)
	got, err := repo.GetPublicHomeStats(t.Context(), now)
	require.NoError(t, err)
	require.Equal(t, int64(321), got.TotalRequests)
	require.Equal(t, int64(98), got.Success30d)
	require.Equal(t, int64(2), got.ErrorSLA30d)
	require.Equal(t, float64(4250), got.TTFTWeightedSum24h)
	require.Equal(t, int64(10), got.TTFTSamples24h)
	require.Equal(t, through.UTC(), got.OpsDataThrough.UTC())
	require.Equal(t, time.UTC, got.OpsDataThrough.Location())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicHomeStatsQueryReportsLastBucketEndWithoutPassingSnapshotTime(t *testing.T) {
	require.Contains(t, publicHomeStatsQuery, "INTERVAL '1 hour'")
	require.Contains(t, publicHomeStatsQuery, "LEAST(")
	require.Contains(t, publicHomeStatsQuery, "THEN NULL")
}

func TestPublicHomeStatsRepositoryKeepsMissingOpsWatermarkNull(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 7, 16, 2, 30, 0, 0, time.UTC)
	mock.ExpectQuery(publicHomeStatsQuery).
		WithArgs(now.Add(-30*24*time.Hour), now.Add(-24*time.Hour), now).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests",
			"success_30d",
			"error_sla_30d",
			"ttft_weighted_sum_24h",
			"ttft_samples_24h",
			"ops_data_through",
		}).AddRow(int64(0), int64(0), int64(0), float64(0), int64(0), nil))

	repo := NewPublicHomeStatsRepository(db)
	got, err := repo.GetPublicHomeStats(t.Context(), now)
	require.NoError(t, err)
	require.Nil(t, got.OpsDataThrough)
	require.NoError(t, mock.ExpectationsWereMet())
}
