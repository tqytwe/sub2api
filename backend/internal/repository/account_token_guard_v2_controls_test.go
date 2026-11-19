package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAccountTokenGuardV2SwitchUpdatesOnlySuppliedFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &accountTokenGuardV2Repository{db: db}
	disabled := false
	mock.ExpectExec("(?s)UPDATE account_token_guard_v2_accounts.*COALESCE.*WHERE account_id").WithArgs(int64(42), false, nil).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateSwitches(context.Background(), 42, &disabled, nil))
	mock.ExpectExec("UPDATE account_token_guard_v2_accounts").WithArgs(int64(42), nil, false).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateSwitches(context.Background(), 42, nil, &disabled))
	mock.ExpectExec("UPDATE account_token_guard_v2_accounts").WithArgs(int64(404), nil, false).WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.UpdateSwitches(context.Background(), 404, nil, &disabled))
	require.NoError(t, mock.ExpectationsWereMet())
}
