package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

func TestPublicStatusSummaryRepositoryReadsOnlyTheLatestSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	through := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	computed := through.Add(5 * time.Minute)
	mock.ExpectQuery(publicStatusSummaryQuery).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "availability_pct", "availability_sample_count", "ttft_p50_ms", "ttft_p95_ms", "ttft_sample_count", "data_through", "computed_at", "window_end",
		}).AddRow(int64(912), 99.5, int64(900), 420.0, 910.0, int64(123), through, computed, through))

	repo := NewPublicHomeStatsRepository(db)
	got, err := repo.GetPublicStatusSummary(t.Context())
	require.NoError(t, err)
	require.True(t, got.HasSnapshot)
	require.Equal(t, int64(912), got.TotalRequests)
	require.Equal(t, int64(900), got.AvailabilitySampleCount)
	require.Equal(t, int64(123), got.TTFTSampleCount)
	require.Equal(t, through, *got.DataThrough)
	require.Equal(t, through, got.WindowEnd)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicStatusSnapshotWindowListIsBoundedAndReturnsOnlyWindowEnds(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	from := time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC)
	through := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	mock.ExpectQuery(publicStatusSnapshotWindowEndsQuery).
		WithArgs(from, through).
		WillReturnRows(sqlmock.NewRows([]string{"window_end"}).
			AddRow(through).
			AddRow(through.Add(-2 * time.Hour)))

	repo := NewPublicHomeStatsRepository(db)
	got, err := repo.ListPublicStatusSnapshotWindowEnds(t.Context(), from, through)

	require.NoError(t, err)
	require.Equal(t, []time.Time{through, through.Add(-2 * time.Hour)}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicStatusSnapshotWindowListSkipsInvalidBoundsWithoutQuerying(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewPublicHomeStatsRepository(db)
	got, err := repo.ListPublicStatusSnapshotWindowEnds(t.Context(), time.Time{}, time.Now())

	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicStatusSnapshotRefreshUsesExactRawTTFTPercentilesAndImmutableInsert(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	mock.ExpectExec(refreshPublicStatusSnapshotQuery).
		WithArgs(windowEnd, service.OpsHourlyAggregationJobName).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPublicHomeStatsRepository(db)
	require.NoError(t, repo.RefreshPublicStatusSnapshot(t.Context(), windowEnd))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, refreshPublicStatusSnapshotQuery, "percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms)")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms)")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "FROM usage_logs")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "WHERE created_at < (SELECT window_end FROM bounds)")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "ON CONFLICT (window_end) DO NOTHING")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "ops_aggregation_watermarks")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "completed_through >= (SELECT window_end FROM bounds)")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "SELECT completed_through")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "LEAST(w.completed_through, b.window_end)")
	require.NotContains(t, refreshPublicStatusSnapshotQuery, "MAX(bucket_start) + INTERVAL '1 hour'")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "WHERE ready.ready")
	require.NotContains(t, refreshPublicStatusSnapshotQuery, "ttft_p50_ms *")
}

func TestPublicStatusSnapshotDataThroughUsesCompletionWatermarkDuringQuietHours(t *testing.T) {
	// A quiet hour has no overall ops_metrics_hourly row. The durable
	// aggregation watermark still advances, so freshness must follow that
	// completion boundary instead of the last non-empty bucket.
	require.Contains(t, refreshPublicStatusSnapshotQuery, "FROM ops_aggregation_watermarks")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "WHERE job_name = $2")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "LEFT JOIN watermark w ON TRUE")
	require.Contains(t, refreshPublicStatusSnapshotQuery, "WHEN w.completed_through IS NULL THEN NULL")
}

func TestPublicStatusSnapshotReadinessUsesMonotonicHourlyAggregationWatermark(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	windowEnd := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	mock.ExpectQuery(publicStatusSnapshotReadyQuery).
		WithArgs(service.OpsHourlyAggregationJobName, windowEnd).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(true))

	repo := NewPublicHomeStatsRepository(db)
	ready, err := repo.IsPublicStatusSnapshotWindowReady(t.Context(), windowEnd)
	require.NoError(t, err)
	require.True(t, ready)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Contains(t, publicStatusSnapshotReadyQuery, "completed_through >= $2")
	require.Contains(t, publicStatusSnapshotReadyQuery, "job_name = $1")
}
