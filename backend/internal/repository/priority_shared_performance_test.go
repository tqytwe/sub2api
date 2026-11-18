package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPrioritySharedPerformanceReadsMonitorWindowAndScopedFinance(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT value FROM settings").WithArgs(service.SettingKeyAccountPerformanceWindowMinutes).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("30"))
	mock.ExpectQuery("WITH source").WillReturnRows(sqlmock.NewRows(performanceColumns).AddRow(7, 30, 10, 100, 10, 0, 2, "{}", 0, false, 0, nil, "", 75, 10))
	mock.ExpectQuery("SELECT account_id").WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "ttft", "last_bad", "revenue", "cost"}).AddRow(7, 4, 250, nil, 12, 3))
	got, err := (&usageLogRepository{sql: db}).ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{AccountIDs: []int64{7}, UseAccountPerformance: true, Model: "fixture"})
	require.NoError(t, err)
	require.Equal(t, 75.0, *got[7].Performance.AvgCachePercent)
	require.Equal(t, 12.0, got[7].Revenue)
	require.Equal(t, 3.0, got[7].TheoreticalCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriorityLatencySeriesScopesModelGroupAndKeepsUnknownErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	groupID := int64(12)
	since := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("date_bin").WithArgs(pq.Array([]int64{7}), since, sqlmock.AnyArg(), 5, 1000, "fixture-model", groupID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "bucket", "samples", "p50", "p90", "slow", "errors"}).
			AddRow(7, since, 4, 80, 120, 1, nil))
	rows, err := (&usageLogRepository{sql: db}).ReadPriorityLatencySeries(context.Background(), service.PriorityLatencySeriesQuery{
		AccountIDs: []int64{7}, Model: "fixture-model", GroupID: &groupID, Since: since, BucketMinutes: 5, SlowTTFTMs: 1000,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].ErrorSamples)
	require.NoError(t, mock.ExpectationsWereMet())
}
