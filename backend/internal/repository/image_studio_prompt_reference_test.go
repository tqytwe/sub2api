package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageStudioRepositoryInsertJobPersistsPromptReferenceAndGenerationSpec(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &imageStudioRepository{sql: db, db: db}
	promptID := int64(123)
	promptVersion := 4
	apiKeyID := int64(9)
	createdAt := time.Now()
	job := &service.ImageStudioJob{
		ID:            "00000000-0000-0000-0000-000000000001",
		UserID:        42,
		TemplateID:    "prompt-library",
		PromptID:      &promptID,
		PromptVersion: &promptVersion,
		PromptHash:    "sha256",
		Model:         "gpt-image-2",
		Quality:       "high",
		Size:          "1024x1024",
		Count:         1,
		Status:        service.ImageStudioJobStatusPending,
		EstimatedCost: 0.1,
		APIKeyID:      &apiKeyID,
	}

	mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO image_studio_jobs
			(id, user_id, template_id, prompt_id, prompt_version, prompt_hash, model, quality, size, count, status, estimated_cost, api_key_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at`)).
		WithArgs(
			job.ID, job.UserID, job.TemplateID, promptID, promptVersion, job.PromptHash,
			job.Model, job.Quality, job.Size, job.Count, job.Status, job.EstimatedCost, apiKeyID, nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(createdAt))

	require.NoError(t, repo.InsertJob(context.Background(), job))
	require.Equal(t, createdAt, job.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageStudioJobPromptReferenceStaysOptional(t *testing.T) {
	var promptID sql.NullInt64
	var promptVersion sql.NullInt64

	require.False(t, promptID.Valid)
	require.False(t, promptVersion.Valid)
}
