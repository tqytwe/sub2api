package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestGetBillingSurchargeReportAppliesSearchAndSortOnlyToDetails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 11, 2, 15, 8, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	today := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("FROM usage_logs")).
		WithArgs(today, today.AddDate(0, 0, 1), start, end).
		WillReturnRows(sqlmock.NewRows(make([]string, 9)).AddRow(1, 2, 3, 4, 5, 6, 7, 8, 9))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM usage_logs ul.*ul.created_at < \$2.*u.email ILIKE \$3.*ul.id::text ILIKE \$3`).
		WithArgs(start, end, "%alice%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT.*FROM usage_logs ul.*u.email ILIKE \$3.*ul.id::text ILIKE \$3.*ORDER BY ul.billing_surcharge_cost asc, ul.id DESC LIMIT \$4 OFFSET \$5`).
		WithArgs(start, end, "%alice%", 25, 25).
		WillReturnRows(sqlmock.NewRows(make([]string, 16)))

	_, err = repo.GetBillingSurchargeReport(context.Background(), start, end, today, usagestats.BillingSurchargeReportParams{
		Pagination: pagination.PaginationParams{Page: 2, PageSize: 25, SortBy: "billing_surcharge_cost", SortOrder: "asc"},
		Search:     "alice",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBillingSurchargeSortColumnUsesAllowlist(t *testing.T) {
	tests := map[string]string{
		"created_at":                        "ul.created_at",
		"user":                              "COALESCE(u.email, '')",
		"model":                             "ul.model",
		"actual_cost":                       "ul.actual_cost",
		"billing_surcharge_cost":            "ul.billing_surcharge_cost",
		"billed_cost":                       usageLogChargedCostExprUL,
		"created_at; DROP TABLE usage_logs": "ul.created_at",
	}
	for input, want := range tests {
		require.Equal(t, want, billingSurchargeSortColumn(input))
	}
}
