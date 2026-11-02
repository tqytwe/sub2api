package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const CandyProbePrompt = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）
苹果味 桃子味 西瓜味
圆形 7 9 8
五角星形 7 6 4`

var candyAnswerPattern = regexp.MustCompile(`^(?:(?:最终)?(?:答案|结果)(?:为|是)?[:：]?|(?:最少|至少)(?:需要)?(?:取出|抽取|摸出)?|(?:需要|取出|抽取|摸出))?(?:21|二十一)(?:个|颗|粒)?(?:糖果|糖)?(?:即可)?[。.!！]?$|^(?:(?:the)?answer(?:is|:)?|atleast)?21(?:candies)?[.!]?$`)

func CandyAnswerCorrect(output string) bool {
	normalized := strings.Map(func(r rune) rune {
		if strings.ContainsRune(" \t\r\n*_`\"'“”", r) {
			return -1
		}
		if r >= '０' && r <= '９' {
			return '0' + r - '０'
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(output))
	return candyAnswerPattern.MatchString(normalized)
}

const (
	ChannelMonitorV2CandyHistoryDefault = 100
	ChannelMonitorV2CandyHistoryMin     = 15
	ChannelMonitorV2CandyHistoryMax     = 100
	ChannelMonitorV2CandyRetention      = 24 * time.Hour
	channelMonitorV2CandyMaxProbes      = 64
)

type ChannelMonitorV2CandyProbe struct {
	GroupID         int64  `json:"group_id"`
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	IntervalMinutes int    `json:"interval_minutes"`
}

func (p ChannelMonitorV2CandyProbe) ConfigKey() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("candy-v1:%d:%s:%s", p.GroupID, p.Model, p.ReasoningEffort))))
}

type ChannelMonitorV2CandyResult struct {
	ID            int64     `json:"-"`
	GroupID       int64     `json:"-"`
	ConfigKey     string    `json:"-"`
	CheckedAt     time.Time `json:"checked_at"`
	Verdict       string    `json:"verdict"`
	LatencyMs     int64     `json:"latency_ms"`
	AnswerPreview string    `json:"answer_preview,omitempty"`
	Reason        string    `json:"reason,omitempty"`
}

type ChannelMonitorV2CandyHistory struct {
	Model           string                        `json:"model"`
	ReasoningEffort string                        `json:"reasoning_effort"`
	IntervalMinutes int                           `json:"interval_minutes"`
	HistoryLimit    int                           `json:"history_limit"`
	Results         []ChannelMonitorV2CandyResult `json:"results"`
}

type ChannelMonitorV2CandyScope struct {
	GroupID   int64
	ConfigKey string
}

type ChannelMonitorV2CandyRepository interface {
	ValidateCandyGroups(context.Context, []int64) error
	ClaimCandyProbe(ctx context.Context, probe ChannelMonitorV2CandyProbe, slot time.Time, configVersion int) (int64, error)
	FinishCandyProbe(ctx context.Context, result ChannelMonitorV2CandyResult) error
	CandyHistory(ctx context.Context, scopes []ChannelMonitorV2CandyScope, since time.Time, limit int) ([]ChannelMonitorV2CandyResult, error)
	PruneCandyHistory(ctx context.Context, before time.Time) error
}

type ChannelMonitorV2CandyService struct {
	config       ChannelMonitorV2Repository
	repo         ChannelMonitorV2CandyRepository
	accountTests *AccountTestService
	accountRepo  AccountRepository
	settings     channelMonitorRuntimeReader
	mu           sync.Mutex
	now          func() time.Time
}

func NewChannelMonitorV2CandyService(config ChannelMonitorV2Repository, accountTests *AccountTestService, readers ...channelMonitorRuntimeReader) *ChannelMonitorV2CandyService {
	if config == nil || accountTests == nil {
		return nil
	}
	repo, ok := config.(ChannelMonitorV2CandyRepository)
	if !ok {
		return nil
	}
	var settings channelMonitorRuntimeReader
	if len(readers) > 0 {
		settings = readers[0]
	}
	return &ChannelMonitorV2CandyService{config: config, repo: repo, accountTests: accountTests, accountRepo: accountTests.accountRepo, settings: settings, now: time.Now}
}

func (s *ChannelMonitorV2CandyService) RunDue(ctx context.Context, now time.Time) {
	if s == nil || !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()
	if s.settings == nil || !s.settings.GetChannelMonitorRuntime(ctx).PassiveAggregationAllowed() {
		return
	}
	if err := s.repo.PruneCandyHistory(ctx, now); err != nil {
		return
	}
	cfg, err := s.config.GetConfig(ctx)
	if err != nil || cfg == nil || !cfg.Enabled {
		return
	}
	jobs := make(chan ChannelMonitorV2CandyProbe)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for probe := range jobs {
				s.runOne(ctx, *cfg, probe, now)
			}
		}()
	}
	for _, probe := range cfg.CandyProbes {
		if !probe.Enabled || (len(cfg.GroupIDs) > 0 && !containsInt64(cfg.GroupIDs, probe.GroupID)) {
			continue
		}
		select {
		case jobs <- probe:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		}
	}
	close(jobs)
	workers.Wait()
}

func (s *ChannelMonitorV2CandyService) runOne(ctx context.Context, cfg ChannelMonitorV2Config, probe ChannelMonitorV2CandyProbe, now time.Time) {
	if s.accountRepo == nil {
		return
	}
	groupAllowed := false
	for _, p := range cfg.Platforms {
		if p.Enabled {
			groupAllowed = true
		}
	}
	if !groupAllowed {
		return
	}
	slot := now.UTC().Truncate(time.Duration(probe.IntervalMinutes) * time.Minute)
	id, err := s.repo.ClaimCandyProbe(ctx, probe, slot, cfg.Version)
	if err != nil || id == 0 {
		return
	}
	result := ChannelMonitorV2CandyResult{ID: id, GroupID: probe.GroupID, ConfigKey: probe.ConfigKey(), CheckedAt: now, Verdict: "error", Reason: "probe_failed"}
	defer func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.FinishCandyProbe(saveCtx, result)
	}()
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var account *Account
	for _, p := range cfg.Platforms {
		if !p.Enabled {
			continue
		}
		accounts, listErr := s.accountRepo.ListSchedulableByGroupIDAndPlatform(runCtx, probe.GroupID, p.Platform)
		if listErr != nil {
			continue
		}
		for i := range accounts {
			if accounts[i].IsSchedulable() && accounts[i].IsModelSupported(probe.Model) {
				account = &accounts[i]
				break
			}
		}
		if account != nil {
			break
		}
	}
	if account == nil {
		result.Reason = "no_eligible_account"
		return
	}
	started := time.Now()
	sample, err := s.accountTests.RunQualityProbeWithEffortBackground(runCtx, account.ID, probe.Model, CandyProbePrompt, probe.ReasoningEffort)
	result.LatencyMs = time.Since(started).Milliseconds()
	if runCtx.Err() != nil {
		result.Reason = "timeout"
		return
	}
	if err != nil || sample == nil || sample.Status != "success" {
		result.Reason = "probe_failed"
		return
	}
	answer := strings.TrimSpace(sample.ResponseText)
	runes := []rune(answer)
	result.AnswerPreview = string(runes[:min(len(runes), 512)])
	if answer == "" {
		result.Reason = "empty_answer"
		return
	}
	result.Reason = ""
	result.Verdict = "incorrect"
	if CandyAnswerCorrect(answer) {
		result.Verdict = "correct"
	}
}

func normalizeChannelMonitorV2CandyProbes(probes []ChannelMonitorV2CandyProbe) error {
	if len(probes) > channelMonitorV2CandyMaxProbes {
		return fmt.Errorf("%w: at most %d candy probes", ErrChannelMonitorV2InvalidConfig, channelMonitorV2CandyMaxProbes)
	}
	seen := map[string]bool{}
	for i := range probes {
		p := &probes[i]
		if p.GroupID <= 0 {
			return fmt.Errorf("%w: invalid candy group", ErrChannelMonitorV2InvalidConfig)
		}
		p.Model = strings.TrimSpace(p.Model)
		if p.Model == "" || len(p.Model) > 100 {
			return fmt.Errorf("%w: candy model is required", ErrChannelMonitorV2InvalidConfig)
		}
		p.ReasoningEffort = strings.ToLower(strings.TrimSpace(p.ReasoningEffort))
		if p.ReasoningEffort == "" {
			p.ReasoningEffort = "medium"
		}
		switch p.ReasoningEffort {
		case "minimal", "low", "medium", "high", "xhigh":
		default:
			return fmt.Errorf("%w: invalid candy reasoning effort", ErrChannelMonitorV2InvalidConfig)
		}
		key := fmt.Sprintf("%d\x00%s\x00%s", p.GroupID, p.Model, p.ReasoningEffort)
		if seen[key] {
			return fmt.Errorf("%w: duplicate candy probe", ErrChannelMonitorV2InvalidConfig)
		}
		seen[key] = true
		if p.IntervalMinutes == 0 {
			p.IntervalMinutes = 1
		}
		if p.IntervalMinutes < 1 || p.IntervalMinutes > 1440 {
			return fmt.Errorf("%w: candy interval must be 1-1440 minutes", ErrChannelMonitorV2InvalidConfig)
		}
	}
	return nil
}

func AttachCandyHistory(rows []ChannelMonitorV2MatrixRow, probes []ChannelMonitorV2CandyProbe, results []ChannelMonitorV2CandyResult, historyLimit int, admin bool) {
	if historyLimit < ChannelMonitorV2CandyHistoryMin || historyLimit > ChannelMonitorV2CandyHistoryMax {
		historyLimit = ChannelMonitorV2CandyHistoryDefault
	}
	for i := range rows {
		row := &rows[i]
		if row.GroupID == nil {
			continue
		}
		for _, probe := range probes {
			if !probe.Enabled || probe.GroupID != *row.GroupID {
				continue
			}
			history := ChannelMonitorV2CandyHistory{Model: probe.Model, ReasoningEffort: probe.ReasoningEffort, IntervalMinutes: probe.IntervalMinutes, HistoryLimit: historyLimit, Results: []ChannelMonitorV2CandyResult{}}
			for _, result := range results {
				if result.GroupID == probe.GroupID && result.ConfigKey == probe.ConfigKey() {
					history.Results = append(history.Results, result)
				}
			}
			if len(history.Results) > historyLimit {
				history.Results = history.Results[:historyLimit]
			}
			if !admin {
				for j := range history.Results {
					history.Results[j].AnswerPreview = ""
					history.Results[j].Reason = ""
				}
			}
			row.CandyChecks = append(row.CandyChecks, history)
		}
		if len(row.CandyChecks) > 0 {
			legacy := row.CandyChecks[0]
			row.Candy = &legacy
		}
	}
}
