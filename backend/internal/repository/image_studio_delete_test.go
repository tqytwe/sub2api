package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageStudioDeleteJobWithStorageKeysCommitsOwnedJobAtomically(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`
		SELECT id::text
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`).
		WithArgs("job-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("job-1"))
	mock.ExpectQuery(`
		SELECT COALESCE(storage_key, '')
		FROM image_studio_assets
		WHERE job_id = $1::uuid AND storage_key IS NOT NULL AND storage_key <> ''`).
		WithArgs("job-1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_key"}).
			AddRow("42/asset-1.png").
			AddRow("42/asset-2.png"))
	mock.ExpectExec(`
		DELETE FROM image_studio_jobs WHERE id = $1::uuid AND user_id = $2`).
		WithArgs("job-1", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	keys, err := repo.DeleteJobWithStorageKeys(context.Background(), 42, "job-1")

	require.NoError(t, err)
	require.Equal(t, []string{"42/asset-1.png", "42/asset-2.png"}, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioDeleteJobWithStorageKeysRollsBackForWrongOwner(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`
		SELECT id::text
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`).
		WithArgs("job-1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	keys, err := repo.DeleteJobWithStorageKeys(context.Background(), 7, "job-1")

	require.ErrorIs(t, err, service.ErrImageStudioJobNotFound)
	require.Nil(t, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}
