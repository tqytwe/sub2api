package repository

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryAdvancesHourlyAggregationWatermarkMonotonically(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	completedThrough := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	mock.ExpectExec(advanceHourlyAggregationWatermarkQuery).
		WithArgs(service.OpsHourlyAggregationJobName, completedThrough).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewOpsRepository(db)
	require.NoError(t, repo.AdvanceHourlyAggregationWatermark(t.Context(), completedThrough))
	require.NoError(t, mock.ExpectationsWereMet())

	queryWithoutWhitespace := regexp.MustCompile(`\s+`).ReplaceAllString(advanceHourlyAggregationWatermarkQuery, "")
	require.Contains(t, queryWithoutWhitespace, "GREATEST(ops_aggregation_watermarks.completed_through,EXCLUDED.completed_through)")
	require.Contains(t, advanceHourlyAggregationWatermarkQuery, "ON CONFLICT (job_name) DO UPDATE")
}

func TestOpsRepositoryRejectsPartialHourAggregationWatermark(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewOpsRepository(db)
	err = repo.AdvanceHourlyAggregationWatermark(t.Context(), time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC))
	require.ErrorContains(t, err, "whole UTC hour")
}

func TestOpsRepositoryReadsHourlyAggregationWatermark(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	completedThrough := time.Date(2026, 8, 29, 14, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	mock.ExpectQuery(getHourlyAggregationWatermarkQuery).
		WithArgs(service.OpsHourlyAggregationJobName).
		WillReturnRows(sqlmock.NewRows([]string{"completed_through"}).AddRow(completedThrough))

	repo := NewOpsRepository(db)
	got, found, err := repo.GetHourlyAggregationWatermark(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, completedThrough.UTC(), got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryReportsMissingHourlyAggregationWatermark(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(getHourlyAggregationWatermarkQuery).
		WithArgs(service.OpsHourlyAggregationJobName).
		WillReturnError(sql.ErrNoRows)

	repo := NewOpsRepository(db)
	got, found, err := repo.GetHourlyAggregationWatermark(t.Context())
	require.NoError(t, err)
	require.False(t, found)
	require.True(t, got.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}
