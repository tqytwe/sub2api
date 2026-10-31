package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const (
	priorityHistoryMaxScopes   = 32
	priorityHistoryMaxAccounts = 2000
	priorityHistoryMaxActive   = 2
	priorityHistoryColdTTL     = 5 * time.Second
	priorityHistoryRetry       = 5 * time.Second
	priorityHistoryMaxAge      = 2 * time.Minute
	priorityHistoryLaunchRate  = time.Second
)

// ErrPriorityLatencyUnavailable is a retryable local admission failure. It
// never represents an upstream account failure and must not affect health or
// billing state.
var ErrPriorityLatencyUnavailable = errors.New("latency scheduling temporarily unavailable")

type prioritySelectionRequestKey struct{}

// PrioritySchedulingSignalReader is optional. Implementations may aggregate
// recent usage without changing the broad UsageLogRepository contract.
type PrioritySchedulingSignalReader interface {
	ReadPrioritySchedulingSignals(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error)
}

type PrioritySchedulingQuery struct {
	AccountIDs         []int64
	Model              string
	InboundEndpoint    string
	GroupID            *int64
	Since              time.Time
	QualitySince       time.Time
	LatencyMeasurement string
	LatencyPercentile  int
	TargetTTFTMs       int
}

type PrioritySchedulingSignal struct {
	Samples            int
	P90TTFTMs          float64
	LatestBadLatencyAt *time.Time
	Revenue            float64
	TheoreticalCost    float64
	QualitySamples     int
	QualityPassed      int
}

type PrioritySchedulingScore struct {
	AccountID           int64      `json:"account_id"`
	AccountName         string     `json:"account_name"`
	Priority            int        `json:"priority"`
	Score               float64    `json:"score"`
	Tier                string     `json:"tier"`
	LatencyStopped      bool       `json:"latency_stopped"`
	Reasons             []string   `json:"reasons"`
	Samples             int        `json:"samples"`
	P90TTFTMs           float64    `json:"p90_ttft_ms"`
	LatencyPenaltyUntil *time.Time `json:"latency_penalty_until,omitempty"`
	LatencyRecovery     bool       `json:"latency_recovery,omitempty"`
	QualitySamples      int        `json:"quality_samples"`
	QualityPassed       int        `json:"quality_passed"`
	ErrorRate           float64    `json:"error_rate"`
	QualityStatus       string     `json:"quality_status"`
	LoadPercent         *int       `json:"load_percent,omitempty"`
	EconomicsSource     string     `json:"economics_source"`
}

type PrioritySchedulingSnapshot struct {
	At              time.Time                 `json:"at"`
	Model           string                    `json:"model"`
	InboundEndpoint string                    `json:"inbound_endpoint,omitempty"`
	GroupID         *int64                    `json:"group_id,omitempty"`
	Mode            string                    `json:"mode"`
	HistoryReady    bool                      `json:"history_ready"`
	HistoryStatus   string                    `json:"history_status"`
	Candidates      []PrioritySchedulingScore `json:"candidates"`
}

type prioritySignalEntry struct {
	signals    map[int64]PrioritySchedulingSignal
	accountIDs []int64
	observedAt time.Time
	expiresAt  time.Time
	freshUntil time.Time
	lastUsedAt time.Time
	refreshing bool
}

type prioritySchedulingState struct {
	mu              sync.Mutex
	entries         map[string]*prioritySignalEntry
	latest          *PrioritySchedulingSnapshot
	launches        [2]time.Time
	launchNext      int
	activeRefreshes int
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

func prioritySchedulingScoringCandidates(cfg PrioritySchedulingConfig, plan *openAIAccountLoadPlan) []openAIAccountCandidateScore {
	if plan == nil {
		return nil
	}
	if cfg.LatencyPenaltyMode == PriorityLatencyStop && len(plan.allCandidates) > 0 {
		return plan.allCandidates
	}
	return plan.candidates
}

func (s *OpenAIGatewayService) prioritySignals(req OpenAIAccountScheduleRequest, cfg PrioritySchedulingConfig, candidates []openAIAccountCandidateScore) (map[int64]PrioritySchedulingSignal, bool) {
	reader, ok := s.usageLogRepo.(PrioritySchedulingSignalReader)
	if !ok || len(candidates) == 0 {
		return nil, false
	}
	ids := make([]int64, 0, len(candidates))
	for _, item := range candidates {
		if item.account != nil {
			ids = append(ids, item.account.ID)
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	ids = compactPriorityAccountIDs(ids)
	if len(ids) > priorityHistoryMaxAccounts {
		return nil, false
	}
	model := strings.TrimSpace(req.HistoryModel)
	if model == "" {
		model = strings.TrimSpace(req.RequestedModel)
	}
	key := prioritySignalCacheKey(req, cfg, ids)
	state := &s.priorityScheduling
	state.mu.Lock()
	if state.entries == nil {
		state.entries = make(map[string]*prioritySignalEntry)
	}
	entry := state.entries[key]
	if entry == nil {
		if len(state.entries) >= priorityHistoryMaxScopes {
			oldestKey := ""
			var oldest time.Time
			for candidateKey, candidate := range state.entries {
				if candidate.refreshing {
					continue
				}
				if oldestKey == "" || candidate.lastUsedAt.Before(oldest) {
					oldestKey, oldest = candidateKey, candidate.lastUsedAt
				}
			}
			if oldestKey == "" {
				state.mu.Unlock()
				return nil, false
			}
			delete(state.entries, oldestKey)
		}
		entry = &prioritySignalEntry{}
		state.entries[key] = entry
	}
	now := time.Now()
	entry.lastUsedAt = now
	queryMatches := equalPriorityAccountIDs(entry.accountIDs, ids)
	if !entry.refreshing && (!queryMatches || !now.Before(entry.expiresAt)) && state.canStartRefresh(now) {
		entry.refreshing = true
		state.activeRefreshes++
		state.launches[state.launchNext] = now
		state.launchNext = (state.launchNext + 1) % len(state.launches)
		queryGroup := clonePriorityGroupID(req.GroupID)
		query := PrioritySchedulingQuery{AccountIDs: append([]int64(nil), ids...), Model: model, InboundEndpoint: strings.TrimSpace(req.InboundEndpoint), GroupID: queryGroup, Since: now.Add(-time.Duration(cfg.WindowMinutes) * time.Minute), QualitySince: now.Add(-time.Duration(cfg.QualityMaxAgeHours) * time.Hour), LatencyMeasurement: cfg.LatencyMeasurement, LatencyPercentile: cfg.LatencyPercentile, TargetTTFTMs: cfg.TargetTTFTMs}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			signals, err := reader.ReadPrioritySchedulingSignals(ctx, query)
			completedAt := time.Now()
			state.mu.Lock()
			defer state.mu.Unlock()
			entry.refreshing = false
			state.activeRefreshes--
			entry.expiresAt = completedAt.Add(priorityHistoryRetry)
			if err == nil {
				entry.signals = signals
				if entry.signals == nil {
					entry.signals = make(map[int64]PrioritySchedulingSignal)
				}
				entry.accountIDs = append(entry.accountIDs[:0], ids...)
				entry.observedAt = completedAt
				cold := false
				for _, id := range ids {
					if signal, exists := entry.signals[id]; !exists || signal.Samples < cfg.MinSamples {
						cold = true
						break
					}
				}
				if cold {
					entry.expiresAt = completedAt.Add(priorityHistoryColdTTL)
				} else {
					entry.expiresAt = completedAt.Add(30 * time.Second)
				}
				entry.freshUntil = entry.expiresAt
			}
		}()
	}
	ready := queryMatches && !entry.observedAt.IsZero() && now.Sub(entry.observedAt) <= priorityHistoryMaxAge && entry.signals != nil
	if cfg.LatencyPenaltyMode == PriorityLatencyStop {
		freshUntil := entry.freshUntil
		if freshUntil.IsZero() {
			// Preserve entries seeded by older callers/tests that only populated
			// expiresAt; new refreshes always set both clocks explicitly.
			freshUntil = entry.expiresAt
		}
		ready = ready && now.Before(freshUntil)
	}
	signals := entry.signals
	state.mu.Unlock()
	return signals, ready
}

func (s *prioritySchedulingState) canStartRefresh(now time.Time) bool {
	if s.activeRefreshes >= priorityHistoryMaxActive {
		return false
	}
	startsInWindow := 0
	for _, launchedAt := range s.launches {
		if !launchedAt.IsZero() && now.Sub(launchedAt) < priorityHistoryLaunchRate {
			startsInWindow++
		}
	}
	return startsInWindow < len(s.launches)
}

func clonePriorityGroupID(groupID *int64) *int64 {
	if groupID == nil {
		return nil
	}
	cloned := *groupID
	return &cloned
}

func prioritySignalCacheKey(req OpenAIAccountScheduleRequest, cfg PrioritySchedulingConfig, accountIDs []int64) string {
	var key strings.Builder
	writePart := func(value string) {
		_, _ = key.WriteString(strconv.Itoa(len(value)))
		_ = key.WriteByte(':')
		_, _ = key.WriteString(value)
		_ = key.WriteByte('|')
	}
	model := strings.TrimSpace(req.HistoryModel)
	if model == "" {
		model = strings.TrimSpace(req.RequestedModel)
	}
	writePart(model)
	writePart(strings.TrimSpace(req.InboundEndpoint))
	if req.GroupID == nil {
		writePart("")
	} else {
		writePart(strconv.FormatInt(*req.GroupID, 10))
	}
	writePart(strconv.Itoa(cfg.WindowMinutes))
	writePart(strconv.Itoa(cfg.QualityMaxAgeHours))
	writePart(strconv.Itoa(cfg.TargetTTFTMs))
	writePart(strings.TrimSpace(cfg.LatencyMeasurement))
	writePart(strconv.Itoa(cfg.LatencyPercentile))
	for _, accountID := range accountIDs {
		writePart(strconv.FormatInt(accountID, 10))
	}
	return key.String()
}

func compactPriorityAccountIDs(ids []int64) []int64 {
	if len(ids) < 2 {
		return ids
	}
	unique := ids[:1]
	for _, id := range ids[1:] {
		if id != unique[len(unique)-1] {
			unique = append(unique, id)
		}
	}
	return unique
}

func equalPriorityAccountIDs(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func shouldApplyPriorityLatencyStop(req OpenAIAccountScheduleRequest, cfg PrioritySchedulingConfig) bool {
	return cfg.LatencyPenaltyMode == PriorityLatencyStop && req.Platform == PlatformOpenAI &&
		req.RequiredImageCapability == "" && req.UseUpstreamTokenCost && cfg.applies(req.GroupID, req.RequestedModel)
}

func (s *OpenAIGatewayService) priorityLatencyStopForStickyAccount(req OpenAIAccountScheduleRequest, cfg PrioritySchedulingConfig, account *Account) bool {
	if account == nil || !shouldApplyPriorityLatencyStop(req, cfg) {
		return false
	}
	candidate := openAIAccountCandidateScore{account: account}
	signals, ready := s.prioritySignals(req, cfg, []openAIAccountCandidateScore{candidate})
	if !ready {
		return false
	}
	return priorityLatencyStopped(cfg, scorePriorityCandidate(cfg, candidate, signals[account.ID]))
}

func priorityHardStopApplies(cfg PrioritySchedulingConfig, req OpenAIAccountScheduleRequest) bool {
	return cfg.LatencyPenaltyMode == PriorityLatencyStop &&
		req.Platform == PlatformOpenAI &&
		req.RequiredImageCapability == "" &&
		req.UseUpstreamTokenCost &&
		cfg.applies(req.GroupID, req.RequestedModel)
}

func priorityLatencyAdmissionError(known bool) error {
	reason := "latency_history_unavailable"
	if known {
		reason = "latency_penalty"
	}
	return &OpenAITurnAdmissionError{Reason: reason, cause: ErrPriorityLatencyUnavailable}
}

// priorityLatencyAdmission is the final local check immediately before an
// upstream request is sent. Candidate filtering is an optimisation; this
// check remains authoritative after slot acquisition and DB refresh.
func (s *OpenAIGatewayService) priorityLatencyAdmission(req OpenAIAccountScheduleRequest, account *Account) error {
	return s.priorityLatencyAdmissionWithConfig(req, account, s.prioritySchedulingRuntimeConfig())
}

func (s *OpenAIGatewayService) priorityLatencyAdmissionWithConfig(req OpenAIAccountScheduleRequest, account *Account, cfg PrioritySchedulingConfig) error {
	if account == nil || !priorityHardStopApplies(cfg, req) {
		return nil
	}
	signals, ready := s.prioritySignals(req, cfg, []openAIAccountCandidateScore{{account: account}})
	if !ready {
		return priorityLatencyAdmissionError(false)
	}
	signal, known := signals[account.ID]
	if !known {
		return priorityLatencyAdmissionError(false)
	}
	score := scorePriorityCandidate(cfg, openAIAccountCandidateScore{account: account}, signal)
	if priorityLatencyStopped(cfg, score) {
		return priorityLatencyAdmissionError(true)
	}
	return nil
}

// admitPriorityWSSend runs at each physical send boundary, including retries.
// It has no turn lifecycle, billing, account-health or slot-acquisition effects.
// The public model scopes observations; only channel routing scopes the policy
// (account-level aliases in the final payload must not change that scope).
func (s *OpenAIGatewayService) admitPriorityWSSend(ctx context.Context, c *gin.Context, account *Account, publicModel string, payload []byte) error {
	cfg := s.prioritySchedulingRuntimeConfig()
	if account == nil || account.Platform != PlatformOpenAI || !cfg.Enabled || cfg.LatencyPenaltyMode != PriorityLatencyStop {
		return nil
	}
	req, selected := prioritySelectionRequestFromContext(ctx)
	if !selected {
		req.GroupID = priorityGroupIDFromContext(c)
		if group, ok := priorityObservationGroupFromContext(ctx); ok {
			req.GroupID = &group
		}
		req.InboundEndpoint, _ = ctx.Value(ctxkey.InboundEndpoint).(string)
		if req.InboundEndpoint == "" {
			req.InboundEndpoint = priorityInboundEndpoint(c)
		}
	}
	publicModel = strings.TrimSpace(publicModel)
	routeModel := publicModel
	if resolved, ok := ResolvedUpstreamModelFromContext(ctx); ok {
		// Composite resolution stays connection-scoped. The handler already
		// rejects a public model switch that would require a new resolution.
		routeModel = resolved
	}
	mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, req.GroupID, routeModel)
	req.Platform = account.Platform
	req.RequestedModel = mapping.MappedModel
	req.HistoryModel = publicModel
	req.RequiredImageCapability = ""
	req.UseUpstreamTokenCost = !IsExplicitImageGenerationIntent("/v1/responses", publicModel, payload)
	if err := s.priorityLatencyAdmissionWithConfig(req, account, cfg); err != nil {
		return NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "latency scheduling temporarily unavailable; please retry", err)
	}
	return nil
}

func prioritySelectionRequestFromContext(ctx context.Context) (OpenAIAccountScheduleRequest, bool) {
	if ctx == nil {
		return OpenAIAccountScheduleRequest{}, false
	}
	req, ok := ctx.Value(prioritySelectionRequestKey{}).(OpenAIAccountScheduleRequest)
	return req, ok
}

func ContextWithSelectionPriorityRequest(ctx context.Context, selection *AccountSelectionResult) context.Context {
	if selection == nil || selection.priorityRequest == nil {
		return ctx
	}
	return context.WithValue(ctx, prioritySelectionRequestKey{}, *selection.priorityRequest)
}

func attachSelectionPriorityRequest(ctx context.Context, selection *AccountSelectionResult) *AccountSelectionResult {
	if selection == nil {
		return nil
	}
	if req, ok := prioritySelectionRequestFromContext(ctx); ok {
		copy := req
		selection.priorityRequest = &copy
	}
	return selection
}

func addExcludedPriorityAccount(ids map[int64]struct{}, accountID int64) map[int64]struct{} {
	cloned := cloneExcludedAccountIDs(ids)
	if cloned == nil {
		cloned = make(map[int64]struct{}, 1)
	}
	cloned[accountID] = struct{}{}
	return cloned
}

func scorePriorityCandidate(cfg PrioritySchedulingConfig, item openAIAccountCandidateScore, signal PrioritySchedulingSignal) PrioritySchedulingScore {
	return scorePriorityCandidateAt(cfg, item, signal, time.Now())
}

func scorePriorityCandidateAt(cfg PrioritySchedulingConfig, item openAIAccountCandidateScore, signal PrioritySchedulingSignal, now time.Time) PrioritySchedulingScore {
	out := PrioritySchedulingScore{AccountID: item.account.ID, AccountName: item.account.Name, Priority: item.priority, Tier: "eligible", Reasons: []string{}, Samples: signal.Samples, P90TTFTMs: signal.P90TTFTMs, QualitySamples: signal.QualitySamples, QualityPassed: signal.QualityPassed, ErrorRate: clamp01(item.errorRate)}
	quality := 0.5
	latency := 0.5
	load := 0.5
	cost := 0.5
	errorRate := 1 - clamp01(item.errorRate)
	unknown := false
	// Only complete, recent, conclusive quality rounds supply this signal.
	// Missing history remains neutral and cannot imply a passing result.
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
			if cfg.LatencyPenaltyMinutes > 0 && signal.LatestBadLatencyAt != nil {
				until := signal.LatestBadLatencyAt.Add(time.Duration(cfg.LatencyPenaltyMinutes) * time.Minute)
				out.LatencyPenaltyUntil = &until
				out.LatencyRecovery = !now.Before(until)
			}
			if out.LatencyRecovery {
				unknown = true
				out.Reasons = append(out.Reasons, "latency_recovery")
			} else {
				out.Tier = "degraded"
				out.Reasons = append(out.Reasons, "latency_above_target")
			}
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
	if item.errorRate > 0 {
		out.Reasons = append(out.Reasons, "error_rate_observed")
		if item.errorRate >= 0.5 {
			out.Tier = "degraded"
			out.Reasons = append(out.Reasons, "error_rate_high")
		}
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
	sum := weights[0] + weights[1] + weights[2] + weights[3] + weights[4]
	if sum > 0 {
		out.Score = 100 * (weights[0]*quality + weights[1]*latency + weights[2]*load + weights[3]*cost + weights[4]*errorRate) / sum
	}
	return out
}

func (s *defaultOpenAIAccountScheduler) applyPriorityScheduling(req OpenAIAccountScheduleRequest, plan *openAIAccountLoadPlan) {
	cfg := s.service.prioritySchedulingRuntimeConfig()
	model := strings.TrimSpace(req.HistoryModel)
	if model == "" {
		model = strings.TrimSpace(req.RequestedModel)
	}
	snapshot := &PrioritySchedulingSnapshot{At: time.Now(), Model: model, InboundEndpoint: strings.TrimSpace(req.InboundEndpoint), GroupID: req.GroupID, Mode: cfg.Mode, HistoryStatus: "unavailable", Candidates: []PrioritySchedulingScore{}}
	if req.Platform != PlatformOpenAI || req.RequiredImageCapability != "" || !req.UseUpstreamTokenCost || !cfg.applies(req.GroupID, req.RequestedModel) {
		s.service.publishPrioritySnapshot(snapshot)
		return
	}
	scoringOverflowCandidates := cfg.LatencyPenaltyMode == PriorityLatencyStop && len(plan.allCandidates) > 0
	// Stop is a hard admission rule, so it must also cover accounts outside the
	// ranked Top-K pool that can later re-enter through overflow fallback.
	scoringCandidates := prioritySchedulingScoringCandidates(cfg, plan)
	signals, ready := s.service.prioritySignals(req, cfg, scoringCandidates)
	snapshot.HistoryReady = ready
	if ready {
		snapshot.HistoryStatus = "ready"
		for i := range scoringCandidates {
			item := &scoringCandidates[i]
			score := scorePriorityCandidateAt(cfg, *item, signals[item.account.ID], snapshot.At)
			if priorityLatencyStopped(cfg, score) {
				score.LatencyStopped = true
			}
			// Tiers are disjoint so score cannot make an unqualified account outrank
			// a candidate meeting the configured history thresholds.
			offset := 0.0
			switch score.Tier {
			case "eligible":
				offset = 200
			case "insufficient":
				offset = 100
			}
			item.score = offset + score.Score
			snapshot.Candidates = append(snapshot.Candidates, score)
		}
		if scoringOverflowCandidates {
			scoresByAccount := make(map[int64]float64, len(scoringCandidates))
			for _, item := range scoringCandidates {
				if item.account != nil {
					scoresByAccount[item.account.ID] = item.score
				}
			}
			for i := range plan.candidates {
				if plan.candidates[i].account != nil {
					plan.candidates[i].score = scoresByAccount[plan.candidates[i].account.ID]
				}
			}
		}
		filterPriorityLatencyStopped(plan, snapshot.Candidates)
		plan.priorityScheduling = true
		plan.includeOverflowFallback = true
	} else {
		snapshot.HistoryStatus = "warming"
	}
	s.service.publishPrioritySnapshot(snapshot)
}

func hasPriorityReason(reasons []string, target string) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}

func priorityLatencyStopped(cfg PrioritySchedulingConfig, score PrioritySchedulingScore) bool {
	return cfg.LatencyPenaltyMode == PriorityLatencyStop && hasPriorityReason(score.Reasons, "latency_above_target")
}

// Stop mode is a hard exclusion only when the configured history produced a
// measured above-target latency result. Missing or insufficient history stays
// eligible and is represented as unknown in the administrator snapshot.
func filterPriorityLatencyStopped(plan *openAIAccountLoadPlan, scores []PrioritySchedulingScore) {
	if plan == nil || len(scores) == 0 {
		return
	}
	stopped := make(map[int64]struct{})
	for _, score := range scores {
		if score.LatencyStopped {
			stopped[score.AccountID] = struct{}{}
		}
	}
	if len(stopped) == 0 {
		return
	}
	plan.priorityLatencyStopped = stopped
	filter := func(items []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		kept := make([]openAIAccountCandidateScore, 0, len(items))
		for _, item := range items {
			if item.account == nil {
				continue
			}
			if _, isStopped := stopped[item.account.ID]; !isStopped {
				kept = append(kept, item)
			}
		}
		return kept
	}
	plan.allCandidates = filter(plan.allCandidates)
	plan.candidates = filter(plan.candidates)
	plan.staleSnapshotCompactRetry = filter(plan.staleSnapshotCompactRetry)
	plan.candidateCount = len(plan.candidates)
	if plan.topK > plan.candidateCount {
		plan.topK = plan.candidateCount
	}
}

func (s *OpenAIGatewayService) publishPrioritySnapshot(snapshot *PrioritySchedulingSnapshot) {
	if s == nil {
		return
	}
	s.priorityScheduling.mu.Lock()
	s.priorityScheduling.latest = snapshot
	s.priorityScheduling.mu.Unlock()
}
