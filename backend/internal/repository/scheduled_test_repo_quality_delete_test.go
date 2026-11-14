package repository

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT account_id FROM scheduled_test_plans`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(9))
	mock.ExpectQuery(`SELECT id FROM accounts`).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	version := time.Now()
	mock.ExpectQuery(`SELECT account_id,updated_at FROM scheduled_test_plans`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"account_id", "updated_at"}).AddRow(9, version))
	mock.ExpectQuery(`SELECT state FROM account_quality_states`).WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)UPDATE scheduled_test_plans.*NOT EXISTS \(SELECT 1 FROM account_quality_states`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "model_id", "cron_expression", "enabled", "max_results", "auto_recover", "last_run_at", "next_run_at", "created_at", "updated_at", "pelican_config", "running_until"}))
	mock.ExpectRollback()
	plan := &service.ScheduledTestPlan{ID: 7, PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}}}
	repo := &scheduledTestPlanRepository{db: db}
	_, err = repo.UpdateWithVersion(context.Background(), plan, version)
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
	var appError *infraerrors.ApplicationError
	require.ErrorAs(t, err, &appError)
	require.EqualValues(t, http.StatusConflict, appError.Code)
	require.Equal(t, "QUALITY_PLAN_NOT_RUNNABLE", appError.Reason)
	require.Contains(t, appError.Message, "enabled, idle")
	require.Contains(t, appError.Message, "retest rest period")
	require.NoError(t, mock.ExpectationsWereMet())
}
