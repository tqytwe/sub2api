package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const PlayRewardSourceTeamSharedReward = "team_shared_reward"

func (s *PlayService) GetTeamRewardSettings(ctx context.Context) PlayTeamRewardSettings {
	rt := s.GetRuntime(ctx)
	return PlayTeamRewardSettings{
		Enabled:    rt.TeamSharedRewardEnabled,
		Tiers:      append([]TeamRewardTier(nil), rt.TeamSharedRewardTiers...),
		Cap:        rt.TeamSharedRewardCap,
		StartMonth: rt.TeamSharedRewardStartMonth,
	}
}

func (s *PlayService) UpdateTeamRewardSettings(
	ctx context.Context,
	settings PlayTeamRewardSettings,
) (PlayTeamRewardSettings, error) {
	cfg := TeamRewardConfig{
		Enabled: settings.Enabled,
		Tiers:   append([]TeamRewardTier(nil), settings.Tiers...),
		Cap:     settings.Cap,
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		return PlayTeamRewardSettings{}, infraerrors.BadRequest(
			"PLAY_TEAM_REWARD_INVALID_SETTINGS",
			err.Error(),
		)
	}
	startMonth, diagnostic := parseTeamRewardStartMonth(settings.StartMonth)
	if diagnostic != nil || startMonth == "" {
		return PlayTeamRewardSettings{}, infraerrors.BadRequest(
			"PLAY_TEAM_REWARD_INVALID_SETTINGS",
			"team reward start month must use YYYY-MM",
		)
	}
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("team reward settings repository missing")
	}
	tiersJSON, err := json.Marshal(cfg.Tiers)
	if err != nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("marshal team reward tiers: %w", err)
	}
	if err := s.settingService.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyPlayTeamSharedRewardEnabled:    strconv.FormatBool(cfg.Enabled),
		SettingKeyPlayTeamSharedRewardTiers:      string(tiersJSON),
		SettingKeyPlayTeamSharedRewardCap:        cfg.Cap.StringFixed(teamRewardAmountScale),
		SettingKeyPlayTeamSharedRewardStartMonth: startMonth,
	}); err != nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("update team reward settings: %w", err)
	}
	if s.settingService.onUpdate != nil {
		s.settingService.onUpdate()
	}
	settings.StartMonth = startMonth
	settings.Tiers = cfg.Tiers
	settings.Cap = cfg.Cap
	return settings, nil
}

func (s *PlayService) SettleTeamRewardMonth(
	ctx context.Context,
	teamID int64,
	month time.Time,
) (*PlayTeamSettlement, error) {
	cfg, _, enabled, err := s.teamRewardConfigForCompetitionMonth(ctx, month, s.currentTeamRewardConfig(ctx))
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, nil
	}
	return s.settleTeamRewardMonth(ctx, teamID, month, cfg)
}

func (s *PlayService) settleTeamRewardMonth(
	ctx context.Context,
	teamID int64,
	month time.Time,
	cfg TeamRewardConfig,
) (*PlayTeamSettlement, error) {
	if teamID <= 0 {
		return nil, fmt.Errorf("team reward settlement team ID must be positive")
	}
	if !cfg.Enabled {
		return nil, nil
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		return nil, fmt.Errorf("team reward settlement config: %w", err)
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load team reward timezone: %w", err)
	}
	localMonth := month.In(location)
	windowStart := time.Date(localMonth.Year(), localMonth.Month(), 1, 0, 0, 0, 0, location)
	windowEnd := windowStart.AddDate(0, 1, 0)
	periodStart := time.Date(localMonth.Year(), localMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

	var snapshot *PlayTeamSettlement
	err = s.repo.WithTeamRewardSnapshotLock(ctx, teamID, func(lockCtx context.Context) error {
		existing, loadErr := s.repo.GetTeamRewardSettlementByTeamPeriod(lockCtx, teamID, periodStart)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			snapshot = existing
			return nil
		}

		contributions, loadErr := s.repo.ListTeamRewardContributions(lockCtx, teamID, windowStart, windowEnd)
		if loadErr != nil {
			return loadErr
		}
		contributions = normalizeTeamContributions(contributions)
		teamSpend := sumTeamContributions(contributions).Round(teamRewardAmountScale)
		pool := resolveTeamRewardPool(teamSpend, cfg)
		if !pool.IsPositive() {
			return nil
		}

		threshold, rate := reachedTeamRewardTier(teamSpend, cfg.Tiers)
		rewardByUser, allocationErr := allocateTeamReward(pool, contributions)
		if allocationErr != nil {
			return allocationErr
		}
		allocations := make([]PlayTeamRewardAllocation, 0, len(contributions))
		periodKey := periodStart.Format("2006-01")
		for _, contribution := range contributions {
			reward := rewardByUser[contribution.UserID].Round(teamRewardAmountScale)
			if !reward.IsPositive() {
				continue
			}
			allocations = append(allocations, PlayTeamRewardAllocation{
				UserID:         contribution.UserID,
				Contribution:   contribution.Amount.Round(teamRewardAmountScale),
				Ratio:          contribution.Amount.Div(teamSpend).Round(teamRewardAmountScale),
				RewardAmount:   reward,
				PayoutStatus:   PlayTeamRewardAllocationStatusPending,
				IdempotencyKey: fmt.Sprintf("team_reward:%d:%s:%d", teamID, periodKey, contribution.UserID),
			})
		}
		if len(allocations) == 0 {
			return nil
		}

		settlement := PlayTeamSettlement{
			TeamID:           teamID,
			PeriodStart:      periodStart,
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			TeamSpend:        teamSpend,
			ReachedThreshold: threshold.Round(teamRewardAmountScale),
			RewardRate:       rate.Round(teamRewardAmountScale),
			PoolAmount:       pool.Round(teamRewardAmountScale),
			CapAmount:        cfg.Cap.Round(teamRewardAmountScale),
			Status:           PlayTeamSettlementStatusPending,
		}
		created, _, createErr := s.repo.CreateTeamRewardSnapshot(lockCtx, settlement, allocations)
		if createErr != nil {
			return createErr
		}
		snapshot = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *PlayService) PayoutTeamRewardSettlement(
	ctx context.Context,
	settlementID int64,
) (*PlayTeamSettlement, error) {
	settlement, err := s.repo.GetTeamRewardSettlement(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	if settlement == nil {
		return nil, fmt.Errorf("team reward settlement %d not found", settlementID)
	}
	if settlement.Status == PlayTeamSettlementStatusCompleted {
		return settlement, nil
	}
	if err := s.repo.MarkTeamRewardSettlementProcessing(ctx, settlementID); err != nil {
		return nil, err
	}

	allocations, err := s.repo.ListUnpaidTeamRewardAllocations(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	var payoutErr error
	for _, allocation := range allocations {
		if allocation.UserID <= 0 || !allocation.RewardAmount.IsPositive() {
			continue
		}
		claimed, err := s.repo.ClaimTeamRewardAllocation(ctx, allocation.ID)
		if err != nil {
			payoutErr = errors.Join(payoutErr, err)
			continue
		}
		if !claimed {
			continue
		}
		amount, _ := allocation.RewardAmount.Float64()
		err = s.grantBalance(
			ctx,
			allocation.UserID,
			amount,
			PlayRewardSourceTeamSharedReward,
			allocation.IdempotencyKey,
			map[string]any{
				"team_id":       settlement.TeamID,
				"settlement_id": settlement.ID,
				"period":        settlement.PeriodStart.Format("2006-01"),
			},
			func(txCtx context.Context) error {
				return s.repo.MarkTeamRewardAllocationPaid(txCtx, allocation.ID)
			},
		)
		if err != nil {
			if markErr := s.repo.MarkTeamRewardAllocationFailed(ctx, allocation.ID, err.Error()); markErr != nil {
				err = errors.Join(err, markErr)
			}
			payoutErr = errors.Join(payoutErr, err)
		}
	}

	refreshed, refreshErr := s.repo.RefreshTeamRewardSettlementStatus(ctx, settlementID)
	if refreshErr != nil {
		return nil, errors.Join(payoutErr, refreshErr)
	}
	return refreshed, payoutErr
}

func (s *PlayService) SettleDueTeamRewardMonths(ctx context.Context, now time.Time) (int, error) {
	settings := s.GetTeamRewardSettings(ctx)
	startMonth, diagnostic := parseTeamRewardStartMonth(settings.StartMonth)
	if diagnostic != nil || startMonth == "" {
		return 0, nil
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return 0, fmt.Errorf("load team reward timezone: %w", err)
	}
	currentMonth := time.Date(now.In(location).Year(), now.In(location).Month(), 1, 0, 0, 0, 0, location)
	firstPeriod, err := time.ParseInLocation("2006-01", startMonth, location)
	if err != nil {
		return 0, fmt.Errorf("parse team reward start month: %w", err)
	}
	if currentMonth.Before(firstPeriod) {
		return 0, nil
	}
	cfg := TeamRewardConfig{
		Enabled: settings.Enabled,
		Tiers:   append([]TeamRewardTier(nil), settings.Tiers...),
		Cap:     settings.Cap,
	}
	if cfg.Enabled {
		if _, _, _, err := s.teamRewardConfigForCompetitionMonth(ctx, currentMonth, cfg); err != nil {
			return 0, err
		}
	}

	settled := 0
	var settleErr error
	for period := firstPeriod; period.Before(currentMonth); period = period.AddDate(0, 1, 0) {
		periodCfg, rules, enabled, periodErr := s.teamRewardConfigForCompetitionMonth(ctx, period, cfg)
		if periodErr != nil {
			settleErr = errors.Join(settleErr, periodErr)
			continue
		}
		if !enabled {
			continue
		}

		_, windowStart, windowEnd, boundsErr := teamCompetitionPeriodWindow(period)
		if boundsErr != nil {
			settleErr = errors.Join(settleErr, boundsErr)
			continue
		}
		teamIDs, listErr := s.repo.ListTeamIDsForRewardMonth(ctx, windowStart, windowEnd)
		if listErr != nil {
			settleErr = errors.Join(settleErr, listErr)
			continue
		}

		for _, teamID := range teamIDs {
			snapshot, snapshotErr := s.settleTeamRewardMonth(ctx, teamID, period, periodCfg)
			if snapshotErr != nil {
				periodErr = errors.Join(periodErr, snapshotErr)
				continue
			}
			if snapshot == nil {
				continue
			}
			payout, payoutErr := s.PayoutTeamRewardSettlement(ctx, snapshot.ID)
			if payoutErr != nil {
				periodErr = errors.Join(periodErr, payoutErr)
				continue
			}
			if payout == nil || payout.Status != PlayTeamSettlementStatusCompleted {
				periodErr = errors.Join(periodErr, fmt.Errorf("team reward settlement %d is not complete", snapshot.ID))
				continue
			}
			settled++
		}

		if periodErr == nil {
			if err := s.snapshotTeamCompetitionSeason(ctx, period, rules); err != nil {
				periodErr = errors.Join(periodErr, err)
			}
		}
		settleErr = errors.Join(settleErr, periodErr)
	}
	return settled, settleErr
}

func (s *PlayService) snapshotTeamCompetitionSeason(ctx context.Context, period time.Time, rules map[string]any) error {
	repo, ok := s.repo.(PlayTeamCompetitionSeasonRepository)
	if !ok {
		return nil
	}
	periodStart, windowStart, windowEnd, err := teamCompetitionPeriodWindow(period)
	if err != nil {
		return err
	}
	_, err = repo.CreateTeamCompetitionSeasonSnapshot(ctx, periodStart, windowStart, windowEnd, rules)
	if err != nil {
		return fmt.Errorf("snapshot team competition season: %w", err)
	}
	return nil
}

// PrepareCurrentTeamCompetitionSeason freezes the current Shanghai-month rules
// before any team traffic reads or settles them. It intentionally records a
// disabled reward config too: enabling or changing policy later in the month
// must not rewrite the season that has already started.
func (s *PlayService) PrepareCurrentTeamCompetitionSeason(ctx context.Context, now time.Time) error {
	repo, ok := s.repo.(PlayTeamCompetitionSeasonRepository)
	if !ok {
		return nil
	}
	cfg := s.currentTeamRewardConfig(ctx)
	if err := validateTeamRewardConfig(cfg); err != nil {
		return fmt.Errorf("prepare team competition season rules: %w", err)
	}
	periodStart, windowStart, windowEnd, err := teamCompetitionPeriodWindow(now)
	if err != nil {
		return err
	}
	if _, err := repo.EnsureTeamCompetitionSeason(
		ctx,
		periodStart,
		windowStart,
		windowEnd,
		teamCompetitionSeasonRules(cfg),
	); err != nil {
		return fmt.Errorf("prepare current team competition season: %w", err)
	}
	return nil
}

type teamCompetitionFrozenRewardRules struct {
	Enabled bool             `json:"enabled"`
	Cap     decimal.Decimal  `json:"cap"`
	Tiers   []TeamRewardTier `json:"tiers"`
}

const teamRewardSettlementAlertAfter = 30 * time.Minute

// FindStalledTeamRewardSettlements is a read-only health probe. Payout leases
// and retries recover work independently; this method makes a long-running
// settlement visible to the production alert pipeline without altering it.
func (s *PlayService) FindStalledTeamRewardSettlements(ctx context.Context, now time.Time) ([]PlayTeamSettlement, error) {
	repo, ok := s.repo.(PlayTeamRewardHealthRepository)
	if !ok {
		return []PlayTeamSettlement{}, nil
	}
	return repo.ListStalledTeamRewardSettlements(ctx, now.Add(-teamRewardSettlementAlertAfter), 100)
}

func teamCompetitionPeriodWindow(period time.Time) (time.Time, time.Time, time.Time, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("load team competition season timezone: %w", err)
	}
	local := period.In(location)
	windowStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	windowEnd := windowStart.AddDate(0, 1, 0)
	periodStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)
	return periodStart, windowStart, windowEnd, nil
}

func teamCompetitionSeasonRules(cfg TeamRewardConfig) map[string]any {
	frozen := teamCompetitionFrozenRewardRules{
		Enabled: cfg.Enabled,
		Cap:     cfg.Cap.Round(teamRewardAmountScale),
		Tiers:   append([]TeamRewardTier(nil), cfg.Tiers...),
	}
	return map[string]any{
		"scoring_version":        "team-competition-v2",
		"timezone":               "Asia/Shanghai",
		"actual_cost_positive":   true,
		"late_join_cutoff_day":   25,
		"member_capacity":        PlayTeamMaxMembers,
		"team_reward":            frozen,
		"reward_cap":             frozen.Cap.StringFixed(teamRewardAmountScale),
		"reward_tiers":           frozen.Tiers,
		"payout_idempotency_key": "team_reward:{team_id}:{period}:{user_id}",
	}
}

func (s *PlayService) teamRewardConfigForCompetitionMonth(
	ctx context.Context,
	period time.Time,
	fallback TeamRewardConfig,
) (TeamRewardConfig, map[string]any, bool, error) {
	fallback = copyTeamRewardConfig(fallback)
	repo, ok := s.repo.(PlayTeamCompetitionSeasonRepository)
	if !ok {
		return fallback, teamCompetitionSeasonRules(fallback), fallback.Enabled, nil
	}
	periodStart, windowStart, windowEnd, err := teamCompetitionPeriodWindow(period)
	if err != nil {
		return TeamRewardConfig{}, nil, false, err
	}
	season, err := repo.GetTeamCompetitionSeason(ctx, periodStart)
	if err != nil {
		return TeamRewardConfig{}, nil, false, fmt.Errorf("load team competition season: %w", err)
	}
	if season == nil {
		if !fallback.Enabled {
			return fallback, nil, false, nil
		}
		season, err = repo.EnsureTeamCompetitionSeason(ctx, periodStart, windowStart, windowEnd, teamCompetitionSeasonRules(fallback))
		if err != nil {
			return TeamRewardConfig{}, nil, false, fmt.Errorf("freeze team competition season rules: %w", err)
		}
	}
	frozen, found, err := teamCompetitionFrozenRewardConfig(season)
	if err != nil {
		return TeamRewardConfig{}, nil, false, err
	}
	if found {
		return frozen, season.Rules, frozen.Enabled, nil
	}
	if season.Status != "legacy" {
		return TeamRewardConfig{}, nil, false, fmt.Errorf("team competition season %s is missing frozen reward rules", season.Month)
	}
	return fallback, season.Rules, fallback.Enabled, nil
}

// currentCompetitionRewardConfig reads only an active Shanghai-month freeze.
// Reads never create a season; the settlement runner owns that write so normal
// request traffic cannot move an effective-date boundary. If a production
// season record is unavailable or malformed, fail closed rather than showing a
// newly edited configuration as the current month's expected payout.
func (s *PlayService) currentCompetitionRewardConfig(ctx context.Context) TeamRewardConfig {
	fallback := s.currentTeamRewardConfig(ctx)
	repo, ok := s.repo.(PlayTeamCompetitionSeasonRepository)
	if !ok {
		return fallback
	}
	periodStart, _, _, err := teamCompetitionPeriodWindow(s.serverNow())
	if err != nil {
		return TeamRewardConfig{}
	}
	season, err := repo.GetTeamCompetitionSeason(ctx, periodStart)
	if err != nil || season == nil {
		return TeamRewardConfig{}
	}
	frozen, found, err := teamCompetitionFrozenRewardConfig(season)
	if err != nil || !found {
		return TeamRewardConfig{}
	}
	return frozen
}

func copyTeamRewardConfig(cfg TeamRewardConfig) TeamRewardConfig {
	cfg.Tiers = append([]TeamRewardTier(nil), cfg.Tiers...)
	return cfg
}

func teamCompetitionFrozenRewardConfig(season *PlayTeamSeason) (TeamRewardConfig, bool, error) {
	if season == nil || len(season.Rules) == 0 {
		return TeamRewardConfig{}, false, nil
	}
	raw, found := season.Rules["team_reward"]
	if !found {
		cap, hasCap := season.Rules["reward_cap"]
		tiers, hasTiers := season.Rules["reward_tiers"]
		if !hasCap || !hasTiers {
			return TeamRewardConfig{}, false, nil
		}
		raw = map[string]any{
			"enabled": true,
			"cap":     cap,
			"tiers":   tiers,
		}
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return TeamRewardConfig{}, false, fmt.Errorf("encode frozen team reward rules: %w", err)
	}
	var frozen teamCompetitionFrozenRewardRules
	if err := json.Unmarshal(encoded, &frozen); err != nil {
		return TeamRewardConfig{}, false, fmt.Errorf("decode frozen team reward rules: %w", err)
	}
	cfg := TeamRewardConfig{
		Enabled: frozen.Enabled,
		Cap:     frozen.Cap,
		Tiers:   append([]TeamRewardTier(nil), frozen.Tiers...),
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		return TeamRewardConfig{}, false, fmt.Errorf("invalid frozen team reward rules: %w", err)
	}
	return cfg, true, nil
}

func (s *PlayService) ListUserTeamRewardSettlements(
	ctx context.Context,
	userID int64,
	limit int,
) ([]PlayUserTeamSettlementRecord, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	return s.repo.ListUserTeamRewardSettlements(ctx, userID, limit)
}

func (s *PlayService) ListAdminTeamRewardSettlements(
	ctx context.Context,
	limit int,
) ([]PlayTeamSettlementRecord, error) {
	settlements, err := s.repo.ListTeamRewardSettlements(ctx, limit)
	if err != nil {
		return nil, err
	}
	return s.attachTeamRewardAllocations(ctx, settlements)
}

func (s *PlayService) ListPublicTeamRewardWinners(ctx context.Context, limit int) ([]PlayTeamRewardPublicWinner, error) {
	repo, ok := s.repo.(PlayPublicTeamRewardsRepository)
	if !ok {
		return []PlayTeamRewardPublicWinner{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return repo.ListPublicTeamRewardWinners(ctx, limit)
}

func (s *PlayService) listTeamRewardSettlementRecords(
	ctx context.Context,
	teamID int64,
	limit int,
) ([]PlayTeamSettlementRecord, error) {
	settlements, err := s.repo.ListTeamRewardSettlementsByTeam(ctx, teamID, limit)
	if err != nil {
		return nil, err
	}
	return s.attachTeamRewardAllocations(ctx, settlements)
}

func (s *PlayService) attachTeamRewardAllocations(
	ctx context.Context,
	settlements []PlayTeamSettlement,
) ([]PlayTeamSettlementRecord, error) {
	records := make([]PlayTeamSettlementRecord, 0, len(settlements))
	for _, settlement := range settlements {
		allocations, err := s.repo.ListTeamRewardAllocations(ctx, settlement.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, PlayTeamSettlementRecord{
			Settlement:  settlement,
			Allocations: allocations,
		})
	}
	return records, nil
}

func normalizeTeamContributions(contributions []TeamContribution) []TeamContribution {
	amountByUser := make(map[int64]decimal.Decimal, len(contributions))
	for _, contribution := range contributions {
		if contribution.UserID <= 0 || !contribution.Amount.IsPositive() {
			continue
		}
		amountByUser[contribution.UserID] = amountByUser[contribution.UserID].Add(contribution.Amount)
	}
	out := make([]TeamContribution, 0, len(amountByUser))
	for userID, amount := range amountByUser {
		out = append(out, TeamContribution{UserID: userID, Amount: amount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func sumTeamContributions(contributions []TeamContribution) decimal.Decimal {
	total := decimal.Zero
	for _, contribution := range contributions {
		total = total.Add(contribution.Amount)
	}
	return total
}

func reachedTeamRewardTier(
	teamSpend decimal.Decimal,
	tiers []TeamRewardTier,
) (decimal.Decimal, decimal.Decimal) {
	threshold := decimal.Zero
	rate := decimal.Zero
	for _, tier := range tiers {
		if teamSpend.LessThan(tier.Threshold) {
			break
		}
		threshold = tier.Threshold
		rate = tier.Rate
	}
	return threshold, rate
}
