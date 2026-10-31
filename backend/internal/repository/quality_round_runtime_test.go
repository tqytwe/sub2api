package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func qualityRoundLockedFixture(t *testing.T, action string) (*scheduledTestPlanRepository, sqlmock.Sqlmock, *service.ScheduledTestPlan) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	started := time.Now().UTC().Truncate(time.Microsecond)
	until := started.Add(time.Minute)
	plan := &service.ScheduledTestPlan{ID: 7, AccountID: 9, UpdatedAt: started, RunningUntil: &until, MaxResults: 1,
		PelicanConfig: &service.PelicanTestConfig{ModelIDs: []string{"first", "second"}, Quality: &service.QualityPolicy{Action: action}}}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).WithArgs(plan.ID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(plan.AccountID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, schedulable, updated_at, status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`)).WithArgs(plan.AccountID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "schedulable", "updated_at", "status"}).AddRow(plan.AccountID, true, started.Add(-time.Minute), service.StatusActive))
	mock.ExpectQuery(`(?s)SELECT account_id, enabled AND updated_at=.*FOR UPDATE`).WithArgs(plan.ID, started, until).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "valid"}).AddRow(plan.AccountID, true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`)).WithArgs(plan.ID).WillReturnError(sql.ErrNoRows)
	return &scheduledTestPlanRepository{db: db}, mock, plan
}

func qualityRoundSample(plan *service.ScheduledTestPlan, model string) *service.ScheduledTestResult {
	config := *plan.PelicanConfig
	config.ModelID = model
	return &service.ScheduledTestResult{PlanID: plan.ID, Status: "success", PelicanConfig: &config,
		QualityRoundID: "same-round", QualityJudgment: map[string]any{"verdict": "correct"}, StartedAt: plan.UpdatedAt, FinishedAt: plan.UpdatedAt}
}

func expectQualityRoundInsert(t *testing.T, mock sqlmock.Sqlmock, result *service.ScheduledTestResult, id int64, action string) {
	t.Helper()
	config, err := json.Marshal(result.PelicanConfig)
	require.NoError(t, err)
	judgment, err := json.Marshal(result.QualityJudgment)
	require.NoError(t, err)
	mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_results`).
		WithArgs(result.PlanID, result.Status, "", result.ErrorMessage, int64(0), result.StartedAt, result.FinishedAt, config, result.QualityRoundID, action, judgment).
		WillReturnRows(sqlmock.NewRows([]string{"id", "plan_id", "status", "response_text", "error_message", "latency_ms", "started_at", "finished_at", "created_at", "pelican_config", "quality_round_id", "quality_action", "quality_judgment"}).
			AddRow(id, result.PlanID, result.Status, "", result.ErrorMessage, int64(0), result.StartedAt, result.FinishedAt, result.FinishedAt, config, result.QualityRoundID, action, judgment))
}

func TestCommitQualityRoundPersistsAllModelsWithOneActionAndTransaction(t *testing.T) {
	repo, mock, plan := qualityRoundLockedFixture(t, "observe_only")
	samples := []*service.ScheduledTestResult{qualityRoundSample(plan, "first"), qualityRoundSample(plan, "second")}
	expectQualityRoundInsert(t, mock, samples[0], 1, "observed")
	expectQualityRoundInsert(t, mock, samples[1], 2, "observed")
	mock.ExpectExec(`(?s)DELETE FROM scheduled_test_results`).WithArgs(plan.ID, 1).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	action, err := repo.CommitQualityRound(context.Background(), plan, "passed", samples, plan.MaxResults)
	require.NoError(t, err)
	require.Equal(t, "observed", action)
	for _, sample := range samples {
		require.Equal(t, "observed", sample.QualityAction)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommitQualityRoundRollsBackActionAndEarlierSamplesOnLaterInsertFailure(t *testing.T) {
	repo, mock, plan := qualityRoundLockedFixture(t, "disable_scheduling")
	samples := []*service.ScheduledTestResult{qualityRoundSample(plan, "first"), qualityRoundSample(plan, "second")}
	samples[1].Status = "failed"
	samples[1].ErrorMessage = "answer_mismatch"
	samples[1].QualityJudgment = map[string]any{"verdict": "incorrect"}
	mock.ExpectQuery(`(?s)UPDATE accounts SET schedulable=false.*RETURNING updated_at`).WithArgs(plan.AccountID).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(plan.UpdatedAt))
	mock.ExpectExec(`(?s)INSERT INTO scheduler_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO account_quality_states`).WillReturnResult(sqlmock.NewResult(0, 1))
	expectQualityRoundInsert(t, mock, samples[0], 1, "scheduling_disabled")
	mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_results`).WillReturnError(errors.New("second sample unavailable"))
	mock.ExpectRollback()
	action, err := repo.CommitQualityRound(context.Background(), plan, "failed", samples, plan.MaxResults)
	require.ErrorContains(t, err, "second sample unavailable")
	require.Empty(t, action)
	require.NoError(t, mock.ExpectationsWereMet())
}
