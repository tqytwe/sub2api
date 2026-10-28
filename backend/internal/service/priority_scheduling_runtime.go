package service

import (
	"context"
	"sync"
	"time"
)

// PrioritySchedulingSignalReader is optional. Implementations may aggregate
// recent usage without changing the broad UsageLogRepository contract.
type PrioritySchedulingSignalReader interface {
	ReadPrioritySchedulingSignals(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error)
}

type PrioritySchedulingQuery struct {
	AccountIDs []int64
	Model      string
	GroupID    *int64
	Since      time.Time
}

type PrioritySchedulingSignal struct {
	Samples         int
	P90TTFTMs       float64
	Revenue         float64
	TheoreticalCost float64
	QualitySamples  int
	QualityPassed   int
}

type PrioritySchedulingScore struct {
	AccountID       int64    `json:"account_id"`
	AccountName     string   `json:"account_name"`
	Priority        int      `json:"priority"`
	Score           float64  `json:"score"`
	Tier            string   `json:"tier"`
	Reasons         []string `json:"reasons"`
	Samples         int      `json:"samples"`
	P90TTFTMs       float64  `json:"p90_ttft_ms"`
	QualitySamples  int      `json:"quality_samples"`
	QualityPassed   int      `json:"quality_passed"`
	QualityStatus   string   `json:"quality_status"`
	LoadPercent     *int     `json:"load_percent,omitempty"`
	EconomicsSource string   `json:"economics_source"`
}

type PrioritySchedulingSnapshot struct {
	At            time.Time                 `json:"at"`
	Model         string                    `json:"model"`
	GroupID       *int64                    `json:"group_id,omitempty"`
	Mode          string                    `json:"mode"`
	HistoryReady  bool                      `json:"history_ready"`
	HistoryStatus string                    `json:"history_status"`
	Candidates    []PrioritySchedulingScore `json:"candidates"`
}

type prioritySignalEntry struct {
	signals    map[int64]PrioritySchedulingSignal
	observedAt time.Time
	expiresAt  time.Time
	refreshing bool
}

type prioritySchedulingState struct {
	mu      sync.Mutex
	entries map[string]*prioritySignalEntry
	latest  *PrioritySchedulingSnapshot
}

func (s *OpenAIGatewayService) prioritySchedulingRuntimeConfig() PrioritySchedulingConfig {
	if s == nil || s.settingService == nil {
		return DefaultPrioritySchedulingConfig()
	}
	return s.settingService.GetPrioritySchedulingRuntimeConfig(context.Background())
}

// GetPrioritySchedulingRuntimeConfig is stale-while-revalidate. A scheduler
// request never waits on the settings database.
func (s *SettingService) GetPrioritySchedulingRuntimeConfig(_ context.Context) PrioritySchedulingConfig {
	s.prioritySchedulingRuntimeMu.Lock()
	if !s.prioritySchedulingRuntimeExpires.IsZero() && time.Now().Before(s.prioritySchedulingRuntimeExpires) {
		cfg := s.prioritySchedulingRuntimeConfig
		s.prioritySchedulingRuntimeMu.Unlock()
		return cfg
	}
	if !s.prioritySchedulingRuntimeLoading {
		s.prioritySchedulingRuntimeLoading = true
		go s.refreshPrioritySchedulingRuntimeConfig()
	}
	cfg := s.prioritySchedulingRuntimeConfig
	if cfg.Mode == "" {
		cfg = DefaultPrioritySchedulingConfig()
	}
	s.prioritySchedulingRuntimeMu.Unlock()
	return cfg
}

func (s *SettingService) refreshPrioritySchedulingRuntimeConfig() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cfg, err := s.GetPrioritySchedulingConfig(ctx)
	s.prioritySchedulingRuntimeMu.Lock()
	defer s.prioritySchedulingRuntimeMu.Unlock()
	s.prioritySchedulingRuntimeLoading = false
	if err != nil {
		s.prioritySchedulingRuntimeConfig = DefaultPrioritySchedulingConfig()
		s.prioritySchedulingRuntimeExpires = time.Now().Add(5 * time.Second)
		return
	}
	s.prioritySchedulingRuntimeConfig = cfg
	s.prioritySchedulingRuntimeExpires = time.Now().Add(5 * time.Second)
}

func (s *OpenAIGatewayService) PrioritySchedulingSnapshot() *PrioritySchedulingSnapshot {
	if s == nil {
		return nil
	}
	s.priorityScheduling.mu.Lock()
	defer s.priorityScheduling.mu.Unlock()
	return s.priorityScheduling.latest
}

func (s *OpenAIGatewayService) prioritySignals(req OpenAIAccountScheduleRequest, cfg PrioritySchedulingConfig, candidates []openAIAccountCandidateScore) (map[int64]PrioritySchedulingSignal, bool) {
	reader, ok := s.usageLogRepo.(PrioritySchedulingSignalReader)
	if !ok || len(candidates) == 0 {
		return nil, false
	}
	ids := make([]int64, 0, len(candidates))
	for _, item := range candidates {
		ids = append(ids, item.account.ID)
	}
	key := req.RequestedModel
	if req.GroupID != nil {
		key += ":" + formatPriorityGroup(*req.GroupID)
	}
	state := &s.priorityScheduling
	state.mu.Lock()
	if state.entries == nil {
		state.entries = make(map[string]*prioritySignalEntry)
	}
	entry := state.entries[key]
	if entry == nil {
		entry = &prioritySignalEntry{}
		state.entries[key] = entry
	}
	if !entry.refreshing && time.Now().After(entry.expiresAt) {
		entry.refreshing = true
		query := PrioritySchedulingQuery{AccountIDs: ids, Model: req.RequestedModel, GroupID: req.GroupID, Since: time.Now().Add(-time.Duration(cfg.WindowMinutes) * time.Minute)}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			signals, err := reader.ReadPrioritySchedulingSignals(ctx, query)
			state.mu.Lock()
			defer state.mu.Unlock()
			entry.refreshing = false
			entry.expiresAt = time.Now().Add(30 * time.Second)
			if err == nil {
				entry.signals = signals
				entry.observedAt = time.Now()
			} else {
				entry.signals = nil
				entry.observedAt = time.Time{}
			}
		}()
	}
	ready := !entry.observedAt.IsZero() && time.Since(entry.observedAt) < time.Minute && entry.signals != nil
	signals := entry.signals
	state.mu.Unlock()
	return signals, ready
}

func formatPriorityGroup(id int64) string {
	// Avoid fmt in the hot path and keep cache keys bounded to numeric input.
	if id == 0 {
		return "0"
	}
	buf := [24]byte{}
	return string(appendInt64(buf[:0], id))
}

func appendInt64(dst []byte, value int64) []byte {
	if value == 0 {
		return append(dst, '0')
	}
	if value < 0 {
		dst = append(dst, '-')
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return append(dst, buf[i:]...)
}

func scorePriorityCandidate(cfg PrioritySchedulingConfig, item openAIAccountCandidateScore, signal PrioritySchedulingSignal) PrioritySchedulingScore {
	out := PrioritySchedulingScore{AccountID: item.account.ID, AccountName: item.account.Name, Priority: item.priority, Tier: "eligible", Reasons: []string{}, Samples: signal.Samples, P90TTFTMs: signal.P90TTFTMs, QualitySamples: signal.QualitySamples, QualityPassed: signal.QualityPassed}
	quality := 0.5
	latency := 0.5
	load := 0.5
	cost := 0.5
	unknown := false
	// This fork has no persisted quality-judgement outcome. Keep the signal
	// explicit so unknown quality cannot be interpreted as a passing result.
	if signal.QualitySamples == 0 {
		unknown = true
		out.QualityStatus = "unavailable"
		out.Reasons = append(out.Reasons, "quality_unknown")
	} else {
		out.QualityStatus = "available"
		quality = clamp01(float64(signal.QualityPassed) / float64(signal.QualitySamples))
		if quality*100 < float64(cfg.MinQualityPercent) {
			out.Tier = "degraded"
			out.Reasons = append(out.Reasons, "quality_below_target")
		}
	}
	if signal.Samples >= cfg.MinSamples && signal.P90TTFTMs > 0 {
		latency = 1 / (1 + signal.P90TTFTMs/float64(cfg.TargetTTFTMs))
		if signal.P90TTFTMs > float64(cfg.TargetTTFTMs) {
			out.Tier = "degraded"
			out.Reasons = append(out.Reasons, "latency_above_target")
		}
	} else {
		unknown = true
		out.Reasons = append(out.Reasons, "latency_insufficient")
	}
	if item.loadKnown {
		load = 1 - clamp01(float64(item.loadInfo.LoadRate)/100)
		percent := item.loadInfo.LoadRate
		out.LoadPercent = &percent
		if percent >= cfg.MaxLoadPercent {
			out.Tier = "degraded"
			out.Reasons = append(out.Reasons, "busy")
		}
	} else {
		unknown = true
		out.Reasons = append(out.Reasons, "load_unknown")
	}
	if signal.Revenue+signal.TheoreticalCost > 0 {
		cost = signal.Revenue / (signal.Revenue + signal.TheoreticalCost)
		out.EconomicsSource = "usage"
	} else if item.account.RateMultiplier != nil {
		cost = 1 / (1 + *item.account.RateMultiplier)
		out.EconomicsSource = "rate"
	} else {
		unknown = true
		out.EconomicsSource = "unknown"
		out.Reasons = append(out.Reasons, "profit_insufficient")
	}
	if unknown && out.Tier == "eligible" {
		out.Tier = "insufficient"
	}
	weights := cfg.weights()
	sum := weights[0] + weights[1] + weights[2] + weights[3]
	if sum > 0 {
		out.Score = 100 * (weights[0]*quality + weights[1]*latency + weights[2]*load + weights[3]*cost) / sum
	}
	return out
}

func (s *defaultOpenAIAccountScheduler) applyPriorityScheduling(req OpenAIAccountScheduleRequest, plan *openAIAccountLoadPlan) {
	cfg := s.service.prioritySchedulingRuntimeConfig()
	snapshot := &PrioritySchedulingSnapshot{At: time.Now(), Model: req.RequestedModel, GroupID: req.GroupID, Mode: cfg.Mode, HistoryStatus: "unavailable", Candidates: []PrioritySchedulingScore{}}
	if req.Platform != PlatformOpenAI || req.RequiredImageCapability != "" || !req.UseUpstreamTokenCost || !cfg.applies(req.GroupID, req.RequestedModel) {
		s.service.publishPrioritySnapshot(snapshot)
		return
	}
	signals, ready := s.service.prioritySignals(req, cfg, plan.candidates)
	snapshot.HistoryReady = ready
	if ready {
		snapshot.HistoryStatus = "ready"
		for i := range plan.candidates {
			item := &plan.candidates[i]
			score := scorePriorityCandidate(cfg, *item, signals[item.account.ID])
			// Tiers are disjoint so score cannot make an unqualified account outrank
			// a candidate meeting the configured history thresholds.
			offset := 0.0
			if score.Tier == "eligible" {
				offset = 200
			} else if score.Tier == "insufficient" {
				offset = 100
			}
			item.score = offset + score.Score
			snapshot.Candidates = append(snapshot.Candidates, score)
		}
		plan.priorityScheduling = true
		plan.includeOverflowFallback = true
	} else {
		snapshot.HistoryStatus = "warming"
	}
	s.service.publishPrioritySnapshot(snapshot)
}

func (s *OpenAIGatewayService) publishPrioritySnapshot(snapshot *PrioritySchedulingSnapshot) {
	if s == nil {
		return
	}
	s.priorityScheduling.mu.Lock()
	s.priorityScheduling.latest = snapshot
	s.priorityScheduling.mu.Unlock()
}
