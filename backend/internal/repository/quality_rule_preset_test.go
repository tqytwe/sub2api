package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityRulePresetRepositoryCreateAndList(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	planRepo := NewScheduledTestPlanRepository(db)
	presetRepo, ok := planRepo.(service.QualityRulePresetRepository)
	require.True(t, ok)
	createdAt := time.Date(2026, 10, 31, 3, 0, 0, 0, time.UTC)
	config := &service.PelicanTestConfig{
		QuestionKind: "text", Prompt: "question",
		Quality: &service.QualityPolicy{Action: "observe_only", ExpectedAnswer: "42", Judge: &service.QualityJudgeConfig{Kind: "expected"}},
	}
	encoded, err := json.Marshal(config)
	require.NoError(t, err)
	columns := []string{"id", "name", "model_id", "cron_expression", "enabled", "max_results", "pelican_config", "created_at", "updated_at"}
	row := func(id int64) *sqlmock.Rows {
		return sqlmock.NewRows(columns).AddRow(id, "Reusable", "test-model", "*/30 * * * *", true, 50, encoded, createdAt, createdAt)
	}

	mock.ExpectQuery(`(?s)INSERT INTO quality_rule_presets.*RETURNING id, name, model_id, cron_expression, enabled, max_results, pelican_config, created_at, updated_at`).
		WithArgs("Reusable", "test-model", "*/30 * * * *", true, 50, sqlmock.AnyArg()).WillReturnRows(row(7))
	created, err := presetRepo.CreateQualityPreset(context.Background(), &service.QualityRulePreset{
		Name: "Reusable", ModelID: "test-model", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 50, PelicanConfig: config,
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), created.ID)
	require.Equal(t, "42", created.PelicanConfig.Quality.ExpectedAnswer)

	mock.ExpectQuery(`SELECT id, name, model_id, cron_expression, enabled, max_results, pelican_config, created_at, updated_at FROM quality_rule_presets ORDER BY id DESC`).
		WillReturnRows(row(7))
	listed, err := presetRepo.ListQualityPresets(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, created.ID, listed[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
