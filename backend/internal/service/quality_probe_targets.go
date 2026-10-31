package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const qualityProbeMaxModels = 50
const qualityProbeMaxSamples = 100
const qualityProbeMaxWorkers = 8

func qualityProbeModelIDs(ids []string) []string {
	var models []string
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	return models
}

// QualityProbeModels returns the full rule target set. Empty legacy model IDs
// retain the account test provider's default model resolution.
func QualityProbeModels(plan *ScheduledTestPlan) []string {
	if plan.PelicanConfig != nil {
		if models := qualityProbeModelIDs(plan.PelicanConfig.ModelIDs); len(models) > 0 {
			return models
		}
	}
	return []string{strings.TrimSpace(plan.ModelID)}
}

// A missing count has always represented one probe in the Fork's stored rules.
func QualityProbeParallelCount(cfg *PelicanTestConfig) int {
	if cfg == nil || cfg.ParallelCount == 0 {
		return 1
	}
	return cfg.ParallelCount
}

func QualityProbeExpectedCount(cfg *PelicanTestConfig) int {
	models := 1
	if cfg != nil {
		if n := len(qualityProbeModelIDs(cfg.ModelIDs)); n > 0 {
			models = n
		}
	}
	return models * QualityProbeParallelCount(cfg)
}

func validateQualityProbeTargets(cfg *PelicanTestConfig) error {
	models := qualityProbeModelIDs(cfg.ModelIDs)
	if len(models) > qualityProbeMaxModels {
		return infraerrors.BadRequest("QUALITY_MODELS_TOO_MANY", "quality rules support at most 50 models")
	}
	for _, model := range models {
		if len(model) > 100 {
			return infraerrors.BadRequest("QUALITY_MODEL_INVALID", "model identifiers must not exceed 100 bytes")
		}
	}
	if QualityProbeExpectedCount(cfg) > qualityProbeMaxSamples {
		return infraerrors.BadRequest("QUALITY_SAMPLES_TOO_MANY", "quality rounds support at most 100 samples")
	}
	return nil
}

func prepareQualityPlan(plan *ScheduledTestPlan) error {
	if err := validateQualityConfig(plan.PelicanConfig); err != nil {
		return err
	}
	if plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil {
		return nil
	}
	cfg := cloneQualityPelicanConfig(plan.PelicanConfig)
	cfg.ModelIDs = qualityProbeModelIDs(cfg.ModelIDs)
	if len(cfg.ModelIDs) > 0 {
		plan.ModelID = cfg.ModelIDs[0]
	}
	plan.PelicanConfig = cfg
	return nil
}
