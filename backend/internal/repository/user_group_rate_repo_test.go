package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSyncGroupDeniedModelsRollsBackWhenUpsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &userGroupRateRepository{sql: db, db: db}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE user_group_rate_multipliers`).
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM user_group_rate_multipliers`).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO user_group_rate_multipliers`).
		WithArgs(int64(7), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("user foreign key violation"))
	mock.ExpectRollback()

	err = repo.SyncGroupDeniedModels(context.Background(), 7, []service.GroupUserDeniedModelsInput{
		{UserID: 999, DeniedModels: []string{"gpt-6-luna"}},
	})
	require.ErrorContains(t, err, "user foreign key violation")
	require.NoError(t, mock.ExpectationsWereMet())
}
