package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestTrendRangeMatchesAggregateBuckets(t *testing.T) {
	utcStart := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		serverTZ    string
		start       time.Time
		end         time.Time
		granularity string
		want        bool
	}{
		{name: "UTC day", serverTZ: "UTC", start: utcStart, end: utcStart.Add(24 * time.Hour), granularity: "day", want: true},
		{name: "UTC hour", serverTZ: "UTC", start: utcStart.Add(time.Hour), end: utcStart.Add(3 * time.Hour), granularity: "hour", want: true},
		{name: "offset representation of UTC bucket", serverTZ: "UTC", start: utcStart.In(time.FixedZone("plus-one", 3600)), end: utcStart.Add(24 * time.Hour).In(time.FixedZone("plus-one", 3600)), granularity: "day", want: true},
		{name: "non UTC server", serverTZ: "Asia/Shanghai", start: utcStart, end: utcStart.Add(24 * time.Hour), granularity: "day", want: false},
		{name: "partial bucket", serverTZ: "UTC", start: utcStart.Add(time.Second), end: utcStart.Add(24 * time.Hour), granularity: "day", want: false},
		{name: "unsupported bucket", serverTZ: "UTC", start: utcStart, end: utcStart.Add(7 * 24 * time.Hour), granularity: "week", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, trendRangeMatchesAggregateBuckets(tt.serverTZ, tt.start, tt.end, tt.granularity))
		})
	}
}

func TestGetUsageTrendFromDailyAggregatesPassesUTCDateArguments(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2024, 3, 10, 1, 0, 0, 0, time.FixedZone("plus-one", 3600))
	end := start.Add(24 * time.Hour)
	columns := []string{"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost"}
	mock.ExpectQuery("FROM usage_dashboard_daily").
		WithArgs(start.UTC(), end.UTC()).
		WillReturnRows(sqlmock.NewRows(columns))

	trend, err := repo.getUsageTrendFromAggregates(context.Background(), start, end, "day")
	require.NoError(t, err)
	require.Empty(t, trend)
	require.NoError(t, mock.ExpectationsWereMet())
}
