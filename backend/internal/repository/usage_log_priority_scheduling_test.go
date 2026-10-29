package repository

import (
	"context"
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

	mock.ExpectQuery(regexp.QuoteMeta("SELECT account_id,")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public-model", sqlmock.AnyArg(), "/v1/responses").
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "p90", "revenue", "theoretical_cost"}).
			AddRow(int64(7), 6, 120.0, 3.5, 1.25))

	got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{
		AccountIDs:      []int64{7},
		Model:           "public-model",
		InboundEndpoint: "/v1/responses",
		Since:           time.Now().Add(-time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, service.PrioritySchedulingSignal{Samples: 6, P90TTFTMs: 120, Revenue: 3.5, TheoreticalCost: 1.25}, got[7])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadPrioritySchedulingSignalsUsesAttributedObservations(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	mock.ExpectQuery(regexp.QuoteMeta("FROM gateway_request_priority_observations o")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "public-model", "/v1/responses", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "samples", "p90", "revenue", "theoretical_cost"}).
			AddRow(int64(7), 4, 240.0, 0.0, 0.0))

	got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{
		AccountIDs: []int64{7}, Model: "public-model", InboundEndpoint: "/v1/responses",
		LatencyMeasurement: "attributed_v1", LatencyPercentile: 90, Since: time.Now().Add(-time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, 4, got[7].Samples)
	require.Equal(t, 240.0, got[7].P90TTFTMs)
	require.NoError(t, mock.ExpectationsWereMet())
}
