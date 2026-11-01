package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// modelBillingRuntimeContextKey carries one immutable policy through all cost
// calculations for a request. A response-model retry or a free-fast reprice
// therefore cannot observe an admin save halfway through settlement.
type modelBillingRuntimeContextKey struct{}

type modelBillingRequestSnapshot struct {
	Config  ModelBillingConfig
	GroupID *int64
}

type modelBillingRuntimeCache struct {
	mu      sync.Mutex
	value   ModelBillingConfig
	valid   bool
	expires time.Time
}

const modelBillingRuntimeCacheTTL = 15 * time.Second

func withModelBillingConfig(ctx context.Context, settings *SettingService, groupID *int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, exists := ctx.Value(modelBillingRuntimeContextKey{}).(modelBillingRequestSnapshot); exists {
		return ctx
	}
	snapshot := modelBillingRequestSnapshot{Config: settings.modelBillingConfigForUsage(ctx)}
	if groupID != nil {
		id := *groupID
		snapshot.GroupID = &id
	}
	return context.WithValue(ctx, modelBillingRuntimeContextKey{}, snapshot)
}

func (s *SettingService) modelBillingConfigForUsage(ctx context.Context) ModelBillingConfig {
	if snapshot, ok := ctx.Value(modelBillingRuntimeContextKey{}).(modelBillingRequestSnapshot); ok {
		return snapshot.Config
	}
	if s == nil || s.settingRepo == nil {
		return ModelBillingConfig{}
	}
	s.modelBillingRuntimeCache.mu.Lock()
	defer s.modelBillingRuntimeCache.mu.Unlock()
	if time.Now().Before(s.modelBillingRuntimeCache.expires) {
		return s.modelBillingRuntimeCache.value
	}
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	snapshot, err := s.GetModelBillingConfig(readCtx)
	if err == nil {
		s.modelBillingRuntimeCache.value = NormalizeModelBillingConfig(snapshot.Config)
		s.modelBillingRuntimeCache.valid = true
	} else if !s.modelBillingRuntimeCache.valid {
		// A cold storage failure leaves the feature disabled. Once a valid value
		// exists, retain it until a later refresh succeeds.
		s.modelBillingRuntimeCache.value = ModelBillingConfig{}
	}
	if err != nil {
		logger.LegacyPrintf("service.billing", "model_billing_config_refresh_failed: retaining last valid policy; cold policy remains disabled")
	}
	s.modelBillingRuntimeCache.expires = time.Now().Add(modelBillingRuntimeCacheTTL)
	return s.modelBillingRuntimeCache.value
}

func modelBillingGroupIDForUsage(ctx context.Context, fallback *int64) *int64 {
	if snapshot, ok := ctx.Value(modelBillingRuntimeContextKey{}).(modelBillingRequestSnapshot); ok {
		return snapshot.GroupID
	}
	return fallback
}

func applyModelBillingMultiplier(cost *CostBreakdown, cfg ModelBillingConfig, model string, groupID *int64) {
	if cost == nil || (cost.BillingMode != "" && cost.BillingMode != string(BillingModeToken)) {
		return
	}
	multiplier := cfg.MultiplierForModel(model, groupID)
	if multiplier == 1 {
		return
	}
	cost.ActualCost *= multiplier
	cost.modelBillingMultiplier = multiplier
}

func costModelBillingMultiplier(cost *CostBreakdown) float64 {
	if cost == nil || cost.modelBillingMultiplier == 0 {
		return 1
	}
	return cost.modelBillingMultiplier
}
