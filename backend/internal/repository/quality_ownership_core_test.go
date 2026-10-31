package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityOwnershipRestoreOnlyReversesExactOwnedValueAndVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	planID, accountID := int64(7), int64(9)
	version := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	config := []byte(`{"quality":{"action":"disable_scheduling"}}`)
	state := qualityStateJSON(t, version)
	fence := qualityOwnershipFence(planID, accountID, version.Format(time.RFC3339Nano), config, state, qualityOwnershipAccountSnapshot{false, version})

	expectOwnershipRead(mock, planID, accountID, version, config, state, false, version)
	mock.ExpectCommit()
	expectOwnershipRead(mock, planID, accountID, version, config, state, false, version)
	mock.ExpectExec(`UPDATE scheduled_test_plans SET enabled=false`).
		WithArgs(planID, version).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)UPDATE accounts SET schedulable=\$2.*RETURNING updated_at`).
		WithArgs(accountID, true).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(version.Add(time.Second)))
	mock.ExpectExec(`(?s)INSERT INTO scheduler_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM account_quality_states WHERE plan_id=\$1`).WithArgs(planID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &scheduledTestPlanRepository{db: db}
	preview, err := repo.QualityOwnership(context.Background(), planID, nil)
	require.NoError(t, err)
	require.True(t, preview.StatePresent)
	require.True(t, preview.RestoreReady)
	require.False(t, preview.ReconcileReady)
	require.Equal(t, fence, preview.Fence)
	result, err := repo.QualityOwnership(context.Background(), planID, &service.QualityOwnershipRequest{Fence: fence, Confirm: true, Restore: true})
	require.NoError(t, err)
	require.False(t, result.StatePresent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQualityOwnershipReconcileOnlyClearsAlreadyPriorValue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	planID, accountID := int64(7), int64(9)
	version := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	accountVersion := version.Add(time.Minute)
	config := []byte(`{"quality":{"action":"disable_scheduling"}}`)
	state := qualityStateJSON(t, version)
	fence := qualityOwnershipFence(planID, accountID, version.Format(time.RFC3339Nano), config, state, qualityOwnershipAccountSnapshot{true, accountVersion})

	expectOwnershipRead(mock, planID, accountID, version, config, state, true, accountVersion)
	mock.ExpectCommit()
	expectOwnershipRead(mock, planID, accountID, version, config, state, true, accountVersion)
	mock.ExpectExec(`DELETE FROM account_quality_states WHERE plan_id=\$1`).WithArgs(planID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &scheduledTestPlanRepository{db: db}
	preview, err := repo.QualityOwnership(context.Background(), planID, nil)
	require.NoError(t, err)
	require.False(t, preview.RestoreReady)
	require.True(t, preview.ReconcileReady)
	result, err := repo.QualityOwnership(context.Background(), planID, &service.QualityOwnershipRequest{Fence: fence, Confirm: true, Reconcile: true})
	require.NoError(t, err)
	require.False(t, result.StatePresent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQualityOwnershipDriftConflictsAndPreservesSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	planID, accountID := int64(7), int64(9)
	version := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	accountVersion := version.Add(time.Second)
	config := []byte(`{"quality":{"action":"disable_scheduling"}}`)
	state := qualityStateJSON(t, version)
	fence := qualityOwnershipFence(planID, accountID, version.Format(time.RFC3339Nano), config, state, qualityOwnershipAccountSnapshot{false, accountVersion})

	expectOwnershipRead(mock, planID, accountID, version, config, state, false, accountVersion)
	mock.ExpectCommit()
	expectOwnershipRead(mock, planID, accountID, version, config, state, false, accountVersion)
	mock.ExpectRollback()

	repo := &scheduledTestPlanRepository{db: db}
	preview, err := repo.QualityOwnership(context.Background(), planID, nil)
	require.NoError(t, err)
	require.False(t, preview.RestoreReady)
	require.False(t, preview.ReconcileReady)
	_, err = repo.QualityOwnership(context.Background(), planID, &service.QualityOwnershipRequest{Fence: fence, Confirm: true, Restore: true})
	require.ErrorContains(t, err, "ownership was preserved")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQualityOwnershipDeleteRequiresClearedState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	planID, accountID := int64(7), int64(9)
	version := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	config := []byte(`{"quality":{"action":"disable_scheduling"}}`)
	state := qualityStateJSON(t, version)
	fence := qualityOwnershipFence(planID, accountID, version.Format(time.RFC3339Nano), config, state, qualityOwnershipAccountSnapshot{false, version})
	expectOwnershipRead(mock, planID, accountID, version, config, state, false, version)
	mock.ExpectRollback()

	repo := &scheduledTestPlanRepository{db: db}
	_, err = repo.QualityOwnership(context.Background(), planID, &service.QualityOwnershipRequest{Fence: fence, Confirm: true, Delete: true})
	require.ErrorContains(t, err, "restore or reconcile")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
		WithArgs(planID).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM accounts WHERE id=$1 FOR NO KEY UPDATE`)).
		WithArgs(accountID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(accountID))
	mock.ExpectQuery(`(?s)SELECT account_id, updated_at, COALESCE\(pelican_config`).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "updated_at", "pelican_config"}).AddRow(accountID, version, config))
	mock.ExpectQuery(`(?s)SELECT state FROM account_quality_states`).WithArgs(planID).WillReturnError(sql.ErrNoRows)
	clearFence := qualityOwnershipFence(planID, accountID, version.Format(time.RFC3339Nano), config, []byte(nil), nil)
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
		WithArgs(planID).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM accounts WHERE id=$1 FOR NO KEY UPDATE`)).
		WithArgs(accountID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(accountID))
	mock.ExpectQuery(`(?s)SELECT account_id, updated_at, COALESCE\(pelican_config`).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "updated_at", "pelican_config"}).AddRow(accountID, version, config))
	mock.ExpectQuery(`(?s)SELECT state FROM account_quality_states`).WithArgs(planID).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`DELETE FROM scheduled_test_plans WHERE id=\$1`).WithArgs(planID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	preview, err := repo.QualityOwnership(context.Background(), planID, nil)
	require.NoError(t, err)
	require.False(t, preview.StatePresent)
	require.Equal(t, clearFence, preview.Fence)
	_, err = repo.QualityOwnership(context.Background(), planID, &service.QualityOwnershipRequest{Fence: clearFence, Confirm: true, Delete: true})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func qualityStateJSON(t *testing.T, version time.Time) []byte {
	t.Helper()
	state, err := json.Marshal(qualityRuntimeState{
		Action: "disable_scheduling", PreviousSchedulable: true, AppliedSchedulable: false,
		AccountUpdatedAt: version,
	})
	require.NoError(t, err)
	return state
}

func expectOwnershipRead(mock sqlmock.Sqlmock, planID, accountID int64, version time.Time, config, state []byte, schedulable bool, accountVersion time.Time) {
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT account_id FROM scheduled_test_plans WHERE id=$1`)).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM accounts WHERE id=$1 FOR NO KEY UPDATE`)).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(accountID))
	mock.ExpectQuery(`(?s)SELECT account_id, updated_at, COALESCE\(pelican_config`).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "updated_at", "pelican_config"}).AddRow(accountID, version, config))
	mock.ExpectQuery(`(?s)SELECT state FROM account_quality_states`).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(state))
	mock.ExpectQuery(`(?s)SELECT schedulable, updated_at FROM accounts`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"schedulable", "updated_at"}).AddRow(schedulable, accountVersion))
}
