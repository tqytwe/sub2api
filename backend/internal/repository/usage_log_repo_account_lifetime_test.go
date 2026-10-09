//go:build unit

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// Check semantic fragments, rather than formatting or an opaque SQL snapshot.
// Historical statistics must remain independent of live account/group records.
func accountLifetimeQueryMatcher(_ string, actual string) error {
	sql := strings.Join(strings.Fields(strings.ToLower(actual)), " ")
	for _, fragment := range []string{
		"sum(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens)",
		"sum(coalesce(account_stats_cost, total_cost) * coalesce(account_rate_multiplier, 1))",
		"sum(total_cost)",
		"case when coalesce(billed_cost, 0) > 0 then billed_cost else actual_cost + coalesce(billing_surcharge_cost, 0) end",
		"from usage_logs", "created_at >= $2",
	} {
		if !strings.Contains(sql, fragment) {
			return fmt.Errorf("missing account statistics semantics: %s", fragment)
		}
	}
	for _, forbidden := range []string{" join ", "deleted_at", " from accounts", " from groups"} {
		if strings.Contains(sql, forbidden) {
			return fmt.Errorf("historical usage filtered by live records: %s", forbidden)
		}
	}
	return nil
}

func TestAccountLifetimeWindowSQLPreservesTargetAccounting(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(accountLifetimeQueryMatcher)))
			require.NoError(t, err)
			defer db.Close()
			repo := newUsageLogRepositoryWithSQL(nil, db)
			columns := []string{"requests", "tokens", "cost", "standard_cost", "user_cost"}
			if batch {
				columns = append([]string{"account_id"}, columns...)
				mock.ExpectQuery("account lifetime").WithArgs("{17,18}", time.Time{}).
					WillReturnRows(sqlmock.NewRows(columns).AddRow(17, 200, 10000, 30, 50, 70))
				result, err := repo.GetAccountWindowStatsBatch(context.Background(), []int64{17, 18}, time.Time{})
				require.NoError(t, err)
				require.Equal(t, int64(10000), result[17].Tokens)
				require.Equal(t, float64(30), result[17].Cost)
				require.Equal(t, float64(50), result[17].StandardCost)
				require.Equal(t, float64(70), result[17].UserCost)
				require.NotNil(t, result[18])
				require.Zero(t, result[18].Cost)
			} else {
				mock.ExpectQuery("account lifetime").WithArgs(int64(17), time.Time{}).
					WillReturnRows(sqlmock.NewRows(columns).AddRow(200, 10000, 30, 50, 70))
				result, err := repo.GetAccountWindowStats(context.Background(), 17, time.Time{})
				require.NoError(t, err)
				require.Equal(t, int64(10000), result.Tokens)
				require.Equal(t, float64(30), result.Cost)
				require.Equal(t, float64(50), result.StandardCost)
				require.Equal(t, float64(70), result.UserCost)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
