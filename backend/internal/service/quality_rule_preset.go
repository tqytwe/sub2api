package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// QualityRulePreset is a reusable quality configuration. It deliberately has
// no account or execution identity; creating one never creates or runs a plan.
type QualityRulePreset struct {
	ID             int64              `json:"id"`
	Name           string             `json:"name"`
	ModelID        string             `json:"model_id"`
	CronExpression string             `json:"cron_expression"`
	Enabled        bool               `json:"enabled"`
	MaxResults     int                `json:"max_results"`
	PelicanConfig  *PelicanTestConfig `json:"pelican_config"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// QualityRulePresetRepository stores only preset records. It must not be used
// to create scheduled plans or dispatch work to the quality runner.
type QualityRulePresetRepository interface {
	ListQualityPresets(context.Context) ([]*QualityRulePreset, error)
	CreateQualityPreset(context.Context, *QualityRulePreset) (*QualityRulePreset, error)
}

func (s *ScheduledTestService) qualityPresetRepo() (QualityRulePresetRepository, error) {
	if s == nil || s.planRepo == nil {
		return nil, infraerrors.New(503, "QUALITY_PRESETS_UNAVAILABLE", "quality rule presets are unavailable")
	}
	repo, ok := s.planRepo.(QualityRulePresetRepository)
	if !ok {
		return nil, infraerrors.New(503, "QUALITY_PRESETS_UNAVAILABLE", "quality rule presets are unavailable")
	}
	return repo, nil
}

func (s *ScheduledTestService) ListQualityPresets(ctx context.Context) ([]*QualityRulePreset, error) {
	repo, err := s.qualityPresetRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListQualityPresets(ctx)
}

// reusableQualityConfig copies only configuration inputs. Result snapshots,
// account state, and rule display names are intentionally not retained.
func reusableQualityConfig(source *PelicanTestConfig) (*PelicanTestConfig, error) {
	if source == nil || source.Quality == nil {
		return nil, infraerrors.BadRequest("QUALITY_PRESET_CONFIG_REQUIRED", "presets require a quality rule configuration")
	}
	config := &PelicanTestConfig{
		Quality:         &QualityPolicy{},
		QuestionKind:    source.QuestionKind,
		TestChannel:     source.TestChannel,
		Prompt:          source.Prompt,
		ReasoningEffort: source.ReasoningEffort,
		ParallelCount:   source.ParallelCount,
		ModelIDs:        append([]string(nil), source.ModelIDs...),
	}
	*config.Quality = *source.Quality
	if source.Quality.Judge != nil {
		judge := *source.Quality.Judge
		config.Quality.Judge = &judge
	}
	if err := validateQualityConfig(config); err != nil {
		return nil, err
	}
	return config, nil
}

// CreateQualityPreset validates the same quality inputs used by a plan, then
// persists a standalone configuration record. It does not call CreatePlan,
// account repositories, or any runner method.
func (s *ScheduledTestService) CreateQualityPreset(ctx context.Context, input *QualityRulePreset) (*QualityRulePreset, error) {
	if input == nil {
		return nil, infraerrors.BadRequest("QUALITY_PRESET_INVALID", "quality preset is required")
	}
	repo, err := s.qualityPresetRepo()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, infraerrors.BadRequest("QUALITY_PRESET_NAME_INVALID", "preset name must be 1-100 characters")
	}
	modelID := strings.TrimSpace(input.ModelID)
	if modelID == "" || utf8.RuneCountInString(modelID) > 100 {
		return nil, infraerrors.BadRequest("QUALITY_PRESET_MODEL_INVALID", "model_id must be 1-100 characters")
	}
	config, err := reusableQualityConfig(input.PelicanConfig)
	if err != nil {
		return nil, err
	}
	config.ModelIDs = qualityProbeModelIDs(config.ModelIDs)
	if len(config.ModelIDs) > 0 {
		modelID = config.ModelIDs[0]
	}
	cronExpression := strings.TrimSpace(input.CronExpression)
	if _, err := computeNextRun(cronExpression, time.Now()); err != nil {
		return nil, infraerrors.BadRequest("QUALITY_PRESET_CRON_INVALID", "invalid cron expression")
	}
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	preset := &QualityRulePreset{
		Name: name, ModelID: modelID, CronExpression: cronExpression,
		Enabled: input.Enabled, MaxResults: maxResults, PelicanConfig: config,
	}
	return repo.CreateQualityPreset(ctx, preset)
}
