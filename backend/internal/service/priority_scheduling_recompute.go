package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

var ErrPrioritySchedulingRecomputeUnavailable = errors.New("priority scheduling recompute unavailable")

type PrioritySchedulingDiffRow struct {
	AccountID   int64    `json:"account_id"`
	AccountName string   `json:"account_name"`
	OldRank     int      `json:"old_rank,omitempty"`
	NewRank     int      `json:"new_rank,omitempty"`
	OldScore    float64  `json:"old_score"`
	NewScore    float64  `json:"new_score"`
	OldTier     string   `json:"old_tier,omitempty"`
	NewTier     string   `json:"new_tier,omitempty"`
	OldSamples  int      `json:"old_samples"`
	NewSamples  int      `json:"new_samples"`
	Changed     bool     `json:"changed"`
	Reasons     []string `json:"reasons,omitempty"`
}

type PrioritySchedulingRecomputeResult struct {
	Before *PrioritySchedulingSnapshot `json:"before"`
	After  *PrioritySchedulingSnapshot `json:"after"`
	Diff   []PrioritySchedulingDiffRow `json:"diff"`
}

// RecomputePriorityScheduling refreshes the latest observed candidate pool and
// returns a before/after diagnostic diff. It intentionally does not mutate
// account priority, routing state, or Redis sticky bindings.
func (s *OpenAIGatewayService) RecomputePriorityScheduling(ctx context.Context) (*PrioritySchedulingRecomputeResult, error) {
	if s == nil {
		return nil, ErrPrioritySchedulingRecomputeUnavailable
	}
	s.priorityScheduling.mu.Lock()
	before := clonePrioritySnapshot(s.priorityScheduling.latest)
	req := s.priorityScheduling.latestRequest
	cfg := s.priorityScheduling.latestConfig
	candidates := clonePriorityCandidates(s.priorityScheduling.latestCandidates)
	set := s.priorityScheduling.latestSet
	s.priorityScheduling.mu.Unlock()
	if !set || before == nil || len(candidates) == 0 {
		return nil, ErrPrioritySchedulingRecomputeUnavailable
	}
	ids := make([]int64, 0, len(candidates))
	for _, item := range candidates {
		if item.account != nil {
			ids = append(ids, item.account.ID)
		}
	}
	reader, ok := s.usageLogRepo.(PrioritySchedulingSignalReader)
	if !ok {
		return nil, ErrPrioritySchedulingRecomputeUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	now := time.Now()
	historyModel := strings.TrimSpace(req.HistoryModel)
	if historyModel == "" {
		historyModel = strings.TrimSpace(req.RequestedModel)
	}
	// A diagnostic refresh has no side effects on routing cache entries. A
	// failed or cancelled read leaves both the scheduler and last snapshot intact.
	signals, err := reader.ReadPrioritySchedulingSignals(ctx, PrioritySchedulingQuery{
		AccountIDs: ids, Model: historyModel, InboundEndpoint: req.InboundEndpoint,
		GroupID: req.GroupID, Since: now.Add(-time.Duration(cfg.WindowMinutes) * time.Minute),
		QualitySince:       now.Add(-time.Duration(cfg.QualityMaxAgeHours) * time.Hour),
		LatencyMeasurement: cfg.LatencyMeasurement, LatencyPercentile: cfg.LatencyPercentile, TargetTTFTMs: cfg.TargetTTFTMs,
		UseAccountPerformance: true,
	})
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.HistoryModel)
	if model == "" {
		model = strings.TrimSpace(req.RequestedModel)
	}
	after := &PrioritySchedulingSnapshot{At: time.Now(), Model: model, InboundEndpoint: strings.TrimSpace(req.InboundEndpoint), GroupID: req.GroupID, Mode: cfg.Mode, HistoryReady: true, HistoryStatus: "ready", Candidates: make([]PrioritySchedulingScore, 0, len(candidates))}
	for _, item := range candidates {
		if item.account == nil {
			continue
		}
		after.Candidates = append(after.Candidates, scorePriorityCandidateAt(cfg, item, signals[item.account.ID], after.At))
	}
	sortPrioritySnapshot(after)
	return &PrioritySchedulingRecomputeResult{Before: before, After: after, Diff: diffPrioritySchedulingSnapshots(before, after)}, nil
}

func clonePrioritySnapshot(in *PrioritySchedulingSnapshot) *PrioritySchedulingSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.GroupID = clonePriorityGroupID(in.GroupID)
	out.Candidates = append([]PrioritySchedulingScore(nil), in.Candidates...)
	for i := range out.Candidates {
		row := &out.Candidates[i]
		row.Reasons = append([]string(nil), row.Reasons...)
		row.Performance = cloneAccountPerformance(row.Performance)
		if row.Performance != nil {
			row.AvgTPS, row.AvgTTFTMs, row.CachePercent = row.Performance.AvgTPS, row.Performance.AvgTTFTMs, row.Performance.AvgCachePercent
		}
	}
	return &out
}

func diffPrioritySchedulingSnapshots(before, after *PrioritySchedulingSnapshot) []PrioritySchedulingDiffRow {
	if after == nil {
		return nil
	}
	oldRows := make(map[int64]PrioritySchedulingScore)
	oldRanks := make(map[int64]int)
	if before != nil {
		for i, row := range before.Candidates {
			oldRows[row.AccountID] = row
			oldRanks[row.AccountID] = i + 1
		}
	}
	rows := make([]PrioritySchedulingDiffRow, 0, len(after.Candidates))
	seen := make(map[int64]struct{}, len(after.Candidates))
	for i, row := range after.Candidates {
		seen[row.AccountID] = struct{}{}
		old, ok := oldRows[row.AccountID]
		item := PrioritySchedulingDiffRow{AccountID: row.AccountID, AccountName: row.AccountName, OldRank: oldRanks[row.AccountID], NewRank: i + 1, OldScore: old.Score, NewScore: row.Score, OldTier: old.Tier, NewTier: row.Tier, OldSamples: old.Samples, NewSamples: row.Samples, Changed: !ok || oldRanks[row.AccountID] != i+1 || math.Abs(old.Score-row.Score) > .01 || old.Tier != row.Tier || old.Samples != row.Samples, Reasons: append([]string(nil), row.Reasons...)}
		rows = append(rows, item)
	}
	if before != nil {
		for i, row := range before.Candidates {
			if _, ok := seen[row.AccountID]; ok {
				continue
			}
			rows = append(rows, PrioritySchedulingDiffRow{AccountID: row.AccountID, AccountName: row.AccountName, OldRank: i + 1, OldScore: row.Score, OldTier: row.Tier, OldSamples: row.Samples, Changed: true})
		}
	}
	return rows
}

func sortPrioritySnapshot(snapshot *PrioritySchedulingSnapshot) {
	rank := func(tier string) int {
		switch tier {
		case "eligible":
			return 2
		case "insufficient":
			return 1
		default:
			return 0
		}
	}
	sort.SliceStable(snapshot.Candidates, func(i, j int) bool {
		a, b := snapshot.Candidates[i], snapshot.Candidates[j]
		if rank(a.Tier) != rank(b.Tier) {
			return rank(a.Tier) > rank(b.Tier)
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.AccountID < b.AccountID
	})
}

func clonePriorityRequest(req OpenAIAccountScheduleRequest) OpenAIAccountScheduleRequest {
	req.GroupID = clonePriorityGroupID(req.GroupID)
	req.ExcludedIDs = cloneExcludedAccountIDs(req.ExcludedIDs)
	return req
}
func clonePriorityCandidates(in []openAIAccountCandidateScore) []openAIAccountCandidateScore {
	out := append([]openAIAccountCandidateScore(nil), in...)
	for i := range out {
		if out[i].account != nil {
			a := *out[i].account
			out[i].account = &a
		}
		if out[i].loadInfo != nil {
			load := *out[i].loadInfo
			out[i].loadInfo = &load
		}
	}
	return out
}
