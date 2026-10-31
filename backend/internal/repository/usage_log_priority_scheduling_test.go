package repository

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestReadPrioritySchedulingSignalsScopesByInboundEndpoint(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	badAt := time.Now().Add(-time.Minute)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT account_id,")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public-model", sqlmock.AnyArg(), "/v1/responses", 15000).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "p90", "latest_bad", "revenue", "theoretical_cost"}).
			AddRow(int64(7), 6, 120.0, badAt, 3.5, 1.25))

	mock.ExpectQuery("WITH quality_rounds AS").WillReturnRows(sqlmock.NewRows([]string{"account_id", "quality_samples", "quality_passed"}))

	got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{
		AccountIDs:      []int64{7},
		Model:           "public-model",
		InboundEndpoint: "/v1/responses",
		Since:           time.Now().Add(-time.Hour),
		TargetTTFTMs:    15000,
	})
	require.NoError(t, err)
	require.Equal(t, 6, got[7].Samples)
	require.Equal(t, 120.0, got[7].P90TTFTMs)
	require.Equal(t, &badAt, got[7].LatestBadLatencyAt)
	require.Equal(t, 3.5, got[7].Revenue)
	require.Equal(t, 1.25, got[7].TheoreticalCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadPrioritySchedulingSignalsUsesAttributedObservations(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	badAt := time.Now().Add(-time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta("FROM gateway_request_priority_observations o")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public-model", "/v1/responses", sqlmock.AnyArg(), 15000).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "p90", "latest_bad", "revenue", "theoretical_cost"}).
			AddRow(int64(7), 4, 240.0, badAt, 0.0, 0.0))

	mock.ExpectQuery("WITH quality_rounds AS").WillReturnRows(sqlmock.NewRows([]string{"account_id", "quality_samples", "quality_passed"}))

	got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{
		AccountIDs: []int64{7}, Model: "public-model", InboundEndpoint: "/v1/responses",
		LatencyMeasurement: "attributed_v1", LatencyPercentile: 90, Since: time.Now().Add(-time.Hour), TargetTTFTMs: 15000,
	})
	require.NoError(t, err)
	require.Equal(t, 4, got[7].Samples)
	require.Equal(t, 240.0, got[7].P90TTFTMs)
	require.Equal(t, &badAt, got[7].LatestBadLatencyAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPriorityLatencyPercentileUsesConfiguredValue(t *testing.T) {
	tests := []struct {
		configured int
		want       float64
	}{
		{configured: 50, want: 0.50},
		{configured: 70, want: 0.70},
		{configured: 90, want: 0.90},
		{configured: 95, want: 0.95},
		{configured: 0, want: 0.90},
		{configured: 99, want: 0.90},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.configured), func(t *testing.T) {
			require.Equal(t, tt.want, priorityLatencyPercentile(tt.configured))
		})
	}
}

func TestReadPrioritySchedulingSignalsIncludesQualityOnlyAccounts(t *testing.T) {
	for _, measurement := range []string{"legacy_forward", "attributed_v1"} {
		t.Run(measurement, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}
			since := time.Now().Add(-time.Hour)
			qualitySince := since.Add(-23 * time.Hour)
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "p90", "latest_bad", "revenue", "theoretical_cost"}))
			mock.ExpectQuery("WITH quality_rounds AS").
				WithArgs(sqlmock.AnyArg(), "public-model", qualitySince).
				WillReturnRows(sqlmock.NewRows([]string{"account_id", "quality_samples", "quality_passed"}).AddRow(int64(7), 2, 1))
			got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{
				AccountIDs: []int64{7}, Model: "public-model", InboundEndpoint: "/v1/responses", Since: since, QualitySince: qualitySince, LatencyMeasurement: measurement,
			})
			require.NoError(t, err)
			require.Equal(t, 2, got[7].QualitySamples, "complete quality rounds remain available without latency samples")
			require.Equal(t, 1, got[7].QualityPassed)
			require.Zero(t, got[7].Samples)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
