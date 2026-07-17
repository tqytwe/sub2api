package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const imageStudioPublicItemsQuery = "image-studio-public-items"

type imageStudioGalleryQueryMatcher struct{}

func (imageStudioGalleryQueryMatcher) Match(expectedSQL, actualSQL string) error {
	if expectedSQL == imageStudioPublicItemsQuery {
		if strings.Contains(actualSQL, "checkpoint_data") {
			return fmt.Errorf("public gallery query must not read checkpoint_data")
		}
		if !strings.Contains(actualSQL, "FROM image_studio_items") ||
			!strings.Contains(actualSQL, "job_id = ANY") {
			return fmt.Errorf("unexpected public item query: %s", actualSQL)
		}
		return nil
	}
	return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
}

func TestImageStudioGetJobUsesLightweightPublicItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(imageStudioGalleryQueryMatcher{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectQuery(`(?s)SELECT id::text.+FROM image_studio_jobs.+WHERE id = \$1::uuid AND user_id = \$2`).
		WithArgs("job-public", int64(42)).
		WillReturnRows(imageStudioJobRows().AddRow(imageStudioJobRow("job-public", 42)...))
	mock.ExpectQuery(`(?s)FROM image_studio_assets.+job_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioAssetRows())
	mock.ExpectQuery(imageStudioPublicItemsQuery).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioPublicItemRows().AddRow(
			"item-public", "job-public", 0, service.ImageStudioItemStatusRunning,
			nil, nil, nil, 1, nil, nil,
		))

	job, err := repo.GetJob(context.Background(), 42, "job-public")

	require.NoError(t, err)
	require.Len(t, job.Items, 1)
	require.Empty(t, job.Items[0].CheckpointData)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioListJobsPageUsesLightweightPublicItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(imageStudioGalleryQueryMatcher{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\)::int.+FROM image_studio_jobs`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT id::text.+FROM image_studio_jobs.+LIMIT \$2 OFFSET \$3`).
		WithArgs(int64(42), 12, 0).
		WillReturnRows(imageStudioJobRows().AddRow(imageStudioJobRow("job-page", 42)...))
	mock.ExpectQuery(`(?s)FROM image_studio_assets.+job_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioAssetRows())
	mock.ExpectQuery(imageStudioPublicItemsQuery).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioPublicItemRows())

	jobs, total, err := repo.ListJobsPage(context.Background(), 42, 1, 12)

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, jobs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioListActiveJobsBatchesJobsAssetsAndPublicItems(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(imageStudioGalleryQueryMatcher{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &imageStudioRepository{sql: db, db: db}

	mock.ExpectQuery(`(?s)SELECT id::text.+FROM image_studio_jobs.+status IN \('pending', 'running'\).+ORDER BY created_at DESC`).
		WithArgs(int64(42)).
		WillReturnRows(imageStudioJobRows().
			AddRow(imageStudioJobRow("job-active-2", 42)...).
			AddRow(imageStudioJobRow("job-active-1", 42)...))
	mock.ExpectQuery(`(?s)FROM image_studio_assets.+job_id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioAssetRows())
	mock.ExpectQuery(imageStudioPublicItemsQuery).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(imageStudioPublicItemRows().
			AddRow("item-active-2", "job-active-2", 0, service.ImageStudioItemStatusRunning, nil, nil, nil, 1, nil, nil).
			AddRow("item-active-1", "job-active-1", 0, service.ImageStudioItemStatusRunning, nil, nil, nil, 1, nil, nil))

	jobs, err := repo.ListActiveJobs(context.Background(), 42)

	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, "job-active-2", jobs[0].ID)
	require.Len(t, jobs[0].Items, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func imageStudioJobRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "user_id", "template_id", "prompt_id", "prompt_version", "prompt_hash", "request_payload_encrypted",
		"model", "quality", "size", "count", "status", "estimated_cost", "actual_cost",
		"api_key_id", "hold_amount", "hold_id", "success_count", "fail_count",
		"error_message", "created_at", "expires_at", "cancel_requested_at", "started_at",
		"finished_at", "heartbeat_at", "lease_owner", "lease_expires_at",
	})
}

func imageStudioJobRow(id string, userID int64) []driver.Value {
	return []driver.Value{
		id, userID, "free-create", nil, nil, "hash", "",
		"gpt-image-1", "standard", "1024x1024", 1, service.ImageStudioJobStatusRunning,
		0.08, nil, nil, nil, "", 0, 0, nil, time.Now().UTC(),
		nil, nil, nil, nil, nil, "", nil,
	}
}

func imageStudioAssetRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"job_id", "id", "url", "sort_order", "storage_key", "content_type", "byte_size",
		"width", "height", "thumbnail_storage_key", "thumbnail_content_type",
		"thumbnail_byte_size",
	})
}

func imageStudioPublicItemRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "job_id", "sort_order", "status", "actual_cost", "error", "asset_id",
		"attempt_count", "started_at", "finished_at",
	})
}
