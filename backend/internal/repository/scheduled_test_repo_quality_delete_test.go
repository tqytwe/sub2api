package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGenericScheduledPlanDeleteRequiresQualityOwnershipEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`DELETE FROM scheduled_test_plans WHERE id = \$1 AND pelican_config->'quality' IS NULL AND NOT EXISTS`).
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT pelican_config, EXISTS .* FROM scheduled_test_plans WHERE id=\$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"pelican_config", "owned_state"}).AddRow(nil, true))

	repo := &scheduledTestPlanRepository{db: db}
	err = repo.Delete(context.Background(), 7)
	require.ErrorContains(t, err, "owned quality state")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVersionedPlanUpdateCannotDropRuleWhileOwnedStateExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)UPDATE scheduled_test_plans.*NOT EXISTS \(SELECT 1 FROM account_quality_states`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "model_id", "cron_expression", "enabled", "max_results", "auto_recover", "last_run_at", "next_run_at", "created_at", "updated_at", "pelican_config", "running_until"}))
	plan := &service.ScheduledTestPlan{ID: 7, PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}}}
	repo := &scheduledTestPlanRepository{db: db}
	_, err = repo.UpdateWithVersion(context.Background(), plan, time.Now())
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQualityPlansExcludesDeletedAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)FROM scheduled_test_plans p.*JOIN accounts a.*pelican_config->'quality' IS NOT NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "model_id", "cron_expression", "enabled", "max_results", "auto_recover", "last_run_at", "next_run_at", "created_at", "updated_at", "pelican_config", "running_until"}))

	repo := &scheduledTestPlanRepository{db: db}
	plans, err := repo.ListQualityPlans(context.Background())
	require.NoError(t, err)
	require.Empty(t, plans)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTriggerQualityRequiresEnabledIdleRule(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectExec(`(?s)UPDATE scheduled_test_plans.*pelican_config->'quality' IS NOT NULL`).
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))

	repo := &scheduledTestPlanRepository{db: db}
	err = repo.TriggerQuality(context.Background(), 7)
	require.ErrorContains(t, err, "quality rule must be enabled and idle")
	require.NoError(t, mock.ExpectationsWereMet())
}
