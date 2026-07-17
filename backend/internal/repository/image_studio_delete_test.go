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
		SELECT id::text, status
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`).
		WithArgs("job-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("job-1", service.ImageStudioJobStatusCompleted))
	mock.ExpectQuery(`
		SELECT keys.storage_key
		FROM image_studio_assets a
		CROSS JOIN LATERAL (
			VALUES (a.storage_key), (a.thumbnail_storage_key)
		) AS keys(storage_key)
		WHERE a.job_id = $1::uuid
		  AND keys.storage_key IS NOT NULL
		  AND keys.storage_key <> ''`).
		WithArgs("job-1").
		WillReturnRows(sqlmock.NewRows([]string{"storage_key"}).
			AddRow("42/asset-1.png").
			AddRow("42/asset-1-thumbnail.png").
			AddRow("42/asset-2.png"))
	mock.ExpectExec(`
		INSERT INTO image_studio_object_deletions (user_id, job_id, storage_key)
		SELECT $1, $2::uuid, pending.storage_key
		FROM (
			SELECT keys.storage_key
			FROM image_studio_assets a
			CROSS JOIN LATERAL (
				VALUES (a.storage_key), (a.thumbnail_storage_key)
			) AS keys(storage_key)
			WHERE a.job_id = $2::uuid
			  AND keys.storage_key IS NOT NULL
			  AND keys.storage_key <> ''
			UNION
			SELECT storage_key
			FROM image_studio_job_references
			WHERE job_id = $2::uuid
			  AND storage_key <> ''
		) pending
		ON CONFLICT (job_id, storage_key) DO NOTHING`).
		WithArgs(int64(42), "job-1").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`
		DELETE FROM image_studio_jobs WHERE id = $1::uuid AND user_id = $2`).
		WithArgs("job-1", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	keys, err := repo.DeleteJobWithStorageKeys(context.Background(), 42, "job-1")

	require.NoError(t, err)
	require.Equal(t, []string{"42/asset-1.png", "42/asset-1-thumbnail.png", "42/asset-2.png"}, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioDeleteJobWithStorageKeysRollsBackForWrongOwner(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`
		SELECT id::text, status
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`).
		WithArgs("job-1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))
	mock.ExpectRollback()

	keys, err := repo.DeleteJobWithStorageKeys(context.Background(), 7, "job-1")

	require.ErrorIs(t, err, service.ErrImageStudioJobNotFound)
	require.Nil(t, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioDeleteJobWithStorageKeysRejectsRunningJob(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`
		SELECT id::text, status
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`).
		WithArgs("job-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("job-1", service.ImageStudioJobStatusRunning))
	mock.ExpectRollback()

	keys, err := repo.DeleteJobWithStorageKeys(context.Background(), 42, "job-1")

	require.ErrorIs(t, err, service.ErrImageStudioJobRunning)
	require.Nil(t, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}
