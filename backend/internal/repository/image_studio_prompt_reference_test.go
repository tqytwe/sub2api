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
	groupID := int64(7)
	createdAt := time.Now()
	job := &service.ImageStudioJob{
		ID:                  "00000000-0000-0000-0000-000000000001",
		UserID:              42,
		TemplateID:          "prompt-library",
		PromptID:            &promptID,
		PromptVersion:       &promptVersion,
		PromptHash:          "sha256",
		Model:               "gpt-image-2",
		Quality:             "high",
		Size:                "1024x1024",
		Count:               1,
		Status:              service.ImageStudioJobStatusPending,
		EstimatedCost:       0.1,
		APIKeyID:            &apiKeyID,
		GroupID:             &groupID,
		Platform:            "openai",
		CapabilityProfileID: "gpt-image",
		CapabilityRevision:  "2026-07-20",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO image_studio_jobs
			(id, user_id, template_id, prompt_id, prompt_version, prompt_hash, model, quality, size, count, status, estimated_cost, api_key_id, group_id, platform, capability_profile_id, capability_revision, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING created_at`)).
		WithArgs(
			job.ID, job.UserID, job.TemplateID, promptID, promptVersion, job.PromptHash,
			job.Model, job.Quality, job.Size, job.Count, job.Status, job.EstimatedCost, apiKeyID,
			groupID, job.Platform, job.CapabilityProfileID, job.CapabilityRevision, nil,
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
