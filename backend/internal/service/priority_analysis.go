package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type PriorityAnalysisQuery struct {
	Model   string `json:"model"`
	GroupID *int64 `json:"group_id,omitempty"`
	Minutes int    `json:"minutes"`
}

type PriorityAnalysis struct {
	Snapshot *PrioritySchedulingSnapshot `json:"snapshot"`
	Latency  *PriorityLatencySeries      `json:"latency,omitempty"`
}

var ErrPriorityAnalysisScope = errors.New("invalid priority analysis scope")

// PriorityAnalysis is a read-only diagnostic path. It uses the same account
// performance reader as scheduling and never changes routing or sticky state.
func (s *OpenAIGatewayService) PriorityAnalysis(ctx context.Context, q PriorityAnalysisQuery) (*PriorityAnalysis, error) {
	q.Model = strings.TrimSpace(q.Model)
	if q.Model == "" || len(q.Model) > 200 || q.Minutes < 5 || q.Minutes > 1440 || (q.GroupID != nil && *q.GroupID <= 0) {
		return nil, ErrPriorityAnalysisScope
	}
	if s == nil || s.accountRepo == nil || s.usageLogRepo == nil || s.settingService == nil {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, priorityAnalysisTimeout)
	defer cancel()
	work, release, err := s.priorityAnalysis.prepare(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	cfg, err := s.settingService.GetPrioritySchedulingConfig(work)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(work, q.GroupID, []string{PlatformOpenAI}, false)
	if err != nil {
		return nil, err
	}
	routeModel := q.Model
	if q.GroupID != nil && s.channelService != nil {
		lookup, err := s.priorityAnalysisChannel(work, *q.GroupID)
		if err != nil {
			return nil, err
		}
		if lookup != nil {
			routeModel = resolveMapping(lookup, *q.GroupID, q.Model).MappedModel
		}
	}
	selected := make([]Account, 0, len(accounts))
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if account.IsModelSupported(routeModel) && account.IsModelAllowedInGroup(q.GroupID, routeModel) {
			selected = append(selected, account)
			ids = append(ids, account.ID)
		}
	}
	reader, ok := s.usageLogRepo.(PrioritySchedulingSignalReader)
	if !ok {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	if len(ids) > priorityHistoryMaxAccounts {
		return nil, fmt.Errorf("priority analysis exceeds %d accounts", priorityHistoryMaxAccounts)
	}
	now := time.Now()
	signals, err := reader.ReadPrioritySchedulingSignals(work, PrioritySchedulingQuery{AccountIDs: ids, Model: q.Model, GroupID: q.GroupID, Since: now.Add(-time.Duration(q.Minutes) * time.Minute), QualitySince: now.Add(-time.Duration(cfg.QualityMaxAgeHours) * time.Hour), LatencyMeasurement: cfg.LatencyMeasurement, LatencyPercentile: cfg.LatencyPercentile, TargetTTFTMs: cfg.TargetTTFTMs, UseAccountPerformance: true})
	if err != nil {
		return nil, err
	}
	snapshot := &PrioritySchedulingSnapshot{At: now, Model: q.Model, GroupID: q.GroupID, Mode: cfg.Mode, HistoryReady: true, HistoryStatus: "ready", Candidates: make([]PrioritySchedulingScore, 0, len(selected))}
	for _, account := range selected {
		item := openAIAccountCandidateScore{account: &account}
		snapshot.Candidates = append(snapshot.Candidates, scorePriorityCandidateAt(cfg, item, signals[account.ID], now))
	}
	sortPrioritySnapshot(snapshot)
	result := &PriorityAnalysis{Snapshot: snapshot}
	if latencyReader, ok := s.usageLogRepo.(PriorityLatencySeriesReader); ok && len(ids) > 0 {
		latencyRows, err := latencyReader.ReadPriorityLatencySeries(work, PriorityLatencySeriesQuery{
			AccountIDs: ids, Model: q.Model, GroupID: q.GroupID,
			Since: now.Add(-time.Duration(q.Minutes) * time.Minute), BucketMinutes: 5, SlowTTFTMs: cfg.TargetTTFTMs,
		})
		if err != nil {
			return nil, err
		}
		names := make(map[int64]string, len(selected))
		for _, account := range selected {
			names[account.ID] = account.Name
		}
		result.Latency = buildPriorityLatencySeries(now, q.Model, q.GroupID, 5, ids, names, latencyRows)
	}
	return result, nil
}

func (s *OpenAIGatewayService) PriorityAnalysisModels(ctx context.Context, groupID *int64) ([]string, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, priorityAnalysisTimeout)
	defer cancel()
	ctx, release, err := s.priorityAnalysis.prepare(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, groupID, []string{PlatformOpenAI}, false)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, name := range defaultModelsListCandidateIDs(PlatformOpenAI) {
		set[name] = struct{}{}
	}
	for _, account := range accounts {
		for _, name := range groupAllowedConcreteModels(&account, groupID) {
			set[name] = struct{}{}
		}
	}
	if groupID != nil && s.channelService != nil {
		lookup, err := s.priorityAnalysisChannel(ctx, *groupID)
		if err != nil {
			return nil, err
		}
		if lookup != nil {
			for name := range lookup.channel.ModelMapping[PlatformOpenAI] {
				set[name] = struct{}{}
			}
		}
	}
	for _, account := range accounts {
		for model := range account.GetModelMapping() {
			if model = strings.TrimSpace(model); model != "" && !strings.ContainsAny(model, "*?") {
				set[model] = struct{}{}
			}
		}
	}
	if cfg := s.prioritySchedulingRuntimeConfig(); len(cfg.Models) > 0 {
		for _, model := range cfg.Models {
			if model = strings.TrimSpace(model); model != "" {
				set[model] = struct{}{}
			}
		}
	}
	models := make([]string, 0, len(set))
	for model := range set {
		if len(model) > 200 || strings.ContainsAny(model, "*?") {
			continue
		}
		models = append(models, model)
	}
	sort.Strings(models)
	return models, nil
}

func (s *OpenAIGatewayService) priorityAnalysisChannel(ctx context.Context, groupID int64) (*channelLookup, error) {
	if s.channelService == nil {
		return nil, nil
	}
	if s.channelService.repo == nil {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	repo := s.channelService.repo
	id, err := repo.GetChannelIDByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, nil
	}
	channel, err := repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("priority analysis channel missing")
	}
	if !channel.IsActive() {
		return nil, nil
	}
	platforms, err := repo.GetGroupPlatforms(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	platform := platforms[groupID]
	if platform == "" {
		return nil, errors.New("priority analysis group platform missing")
	}
	cache := populateChannelCache([]Channel{*channel}, platforms)
	owned := cache.channelByGroupID[groupID]
	if owned == nil {
		return nil, errors.New("priority analysis channel binding changed")
	}
	return &channelLookup{cache: cache, channel: owned, platform: PlatformOpenAI}, nil
}
