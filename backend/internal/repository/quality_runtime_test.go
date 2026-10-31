package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCommitQualityRunRollsBackAccountAndOutboxWhenResultInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	started := time.Now().UTC().Truncate(time.Microsecond)
	leaseUntil := started.Add(time.Minute)
	accountVersion := started.Add(-time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(9)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, schedulable, updated_at, status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "schedulable", "updated_at", "status"}).AddRow(int64(9), true, accountVersion, service.StatusActive))
	mock.ExpectQuery(`(?s)SELECT account_id, enabled AND updated_at=.*FOR UPDATE`).
		WithArgs(int64(7), started, leaseUntil).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "?column?"}).AddRow(int64(9), true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`)).
		WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)UPDATE accounts SET schedulable=false.*RETURNING updated_at`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(started))
	mock.ExpectExec(`(?s)INSERT INTO scheduler_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO account_quality_states`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_results`).
		WillReturnError(errors.New("result storage unavailable"))
	mock.ExpectRollback()

	plan := &service.ScheduledTestPlan{
		ID: 7, AccountID: 9, UpdatedAt: started, RunningUntil: &leaseUntil,
		MaxResults: 10, PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}},
	}
	result := &service.ScheduledTestResult{PlanID: 7, Status: "failed", QualityRoundID: "round", QualityJudgment: map[string]any{"verdict": "incorrect"}}
	repo := &scheduledTestPlanRepository{db: db}
	action, err := repo.CommitQualityRun(context.Background(), plan, "failed", result, 10)

	require.ErrorContains(t, err, "result storage unavailable")
	require.Empty(t, action)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommitInconclusiveQualityRunDoesNotChangeAccountScheduling(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	started := time.Now().UTC().Truncate(time.Microsecond)
	leaseUntil := started.Add(time.Minute)
	accountVersion := started.Add(-time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(9)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, schedulable, updated_at, status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "schedulable", "updated_at", "status"}).AddRow(int64(9), true, accountVersion, service.StatusActive))
	mock.ExpectQuery(`(?s)SELECT account_id, enabled AND updated_at=.*FOR UPDATE`).
		WithArgs(int64(7), started, leaseUntil).WillReturnRows(sqlmock.NewRows([]string{"account_id", "?column?"}).AddRow(int64(9), true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`)).
		WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	createdAt := started.Add(time.Second)
	mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_results`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "plan_id", "status", "response_text", "error_message", "latency_ms", "started_at", "finished_at", "created_at", "pelican_config", "quality_round_id", "quality_action", "quality_judgment"}).
			AddRow(int64(1), int64(7), "success", "", "", int64(25), started, createdAt, createdAt, []byte(`{"quality":{"action":"disable_scheduling"}}`), "round", "inconclusive", []byte(`{"verdict":"unknown"}`)))
	mock.ExpectExec(`(?s)DELETE FROM scheduled_test_results`).WithArgs(int64(7), 10).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	plan := &service.ScheduledTestPlan{
		ID: 7, AccountID: 9, UpdatedAt: started, RunningUntil: &leaseUntil, MaxResults: 10,
		PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}},
	}
	result := &service.ScheduledTestResult{
		PlanID: 7, Status: "success", ResponseText: "", QualityRoundID: "round",
		QualityJudgment: map[string]any{"verdict": "unknown"},
	}
	action, err := (&scheduledTestPlanRepository{db: db}).CommitQualityRun(context.Background(), plan, "inconclusive", result, 10)

	require.NoError(t, err)
	require.Equal(t, "inconclusive", action)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommitQualityPassedRunRespectsAutoRestorePolicy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		autoRestore bool
		autoRecover bool
		drift       bool
		staleLease  bool
		noState     bool
		outcome     string
		wantAction  string
	}{
		{name: "default preserves owned restriction", wantAction: "no_change"},
		{name: "legacy auto recover cannot enable quality restore", autoRecover: true, wantAction: "no_change"},
		{name: "explicit auto restore reverses owned restriction", autoRestore: true, wantAction: "restored"},
		{name: "explicit auto restore preserves changed account", autoRestore: true, drift: true, wantAction: "restore_conflict"},
		{name: "explicit auto restore rejects stale lease", autoRestore: true, staleLease: true, wantAction: "stale_run"},
		{name: "explicit auto restore requires owned restriction", autoRestore: true, noState: true, wantAction: "no_change"},
		{name: "explicit auto restore does not act on unknown judgment", autoRestore: true, outcome: "inconclusive", wantAction: "inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			started := time.Now().UTC().Truncate(time.Microsecond)
			leaseUntil := started.Add(time.Minute)
			ownedVersion := started.Add(-time.Minute)
			accountVersion := ownedVersion
			if tc.drift {
				accountVersion = accountVersion.Add(time.Second)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
				WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(9)))
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, schedulable, updated_at, status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).
				WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "schedulable", "updated_at", "status"}).AddRow(int64(9), false, accountVersion, service.StatusActive))
			mock.ExpectQuery(`(?s)SELECT account_id, enabled AND updated_at=.*FOR UPDATE`).
				WithArgs(int64(7), started, leaseUntil).WillReturnRows(sqlmock.NewRows([]string{"account_id", "valid"}).AddRow(int64(9), !tc.staleLease))
			if tc.staleLease {
				mock.ExpectRollback()
			} else {
				stateRead := mock.ExpectQuery(regexp.QuoteMeta(`SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`)).WithArgs(int64(7))
				if tc.noState {
					stateRead.WillReturnError(sql.ErrNoRows)
				} else {
					stateRead.WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(qualityStateJSON(t, ownedVersion)))
				}
				if tc.wantAction == "restored" {
					mock.ExpectQuery(`(?s)UPDATE accounts SET schedulable=\$2.*RETURNING updated_at`).
						WithArgs(int64(9), true).WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(started))
					mock.ExpectExec(`(?s)INSERT INTO scheduler_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
					mock.ExpectExec(`DELETE FROM account_quality_states WHERE plan_id=\$1`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_results`).
					WithArgs(int64(7), "success", "", "", int64(25), started, started, sqlmock.AnyArg(), "round", tc.wantAction, sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id", "plan_id", "status", "response_text", "error_message", "latency_ms", "started_at", "finished_at", "created_at", "pelican_config", "quality_round_id", "quality_action", "quality_judgment"}).
						AddRow(int64(1), int64(7), "success", "", "", int64(25), started, started, started, []byte(`{"quality":{"action":"disable_scheduling"}}`), "round", tc.wantAction, []byte(`{"verdict":"correct"}`)))
				mock.ExpectExec(`(?s)DELETE FROM scheduled_test_results`).WithArgs(int64(7), 10).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
			}
			plan := &service.ScheduledTestPlan{
				ID: 7, AccountID: 9, UpdatedAt: started, RunningUntil: &leaseUntil, MaxResults: 10, AutoRecover: tc.autoRecover,
				PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling", AutoRestore: tc.autoRestore}},
			}
			result := &service.ScheduledTestResult{
				PlanID: 7, Status: "success", LatencyMs: 25, StartedAt: started, FinishedAt: started,
				PelicanConfig: plan.PelicanConfig, QualityRoundID: "round", QualityJudgment: map[string]any{"verdict": "correct"},
			}
			outcome := tc.outcome
			if outcome == "" {
				outcome = "passed"
			}
			action, err := (&scheduledTestPlanRepository{db: db}).CommitQualityRun(context.Background(), plan, outcome, result, 10)
			require.NoError(t, err)
			require.Equal(t, tc.wantAction, action)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
