package service

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
	"time"
)

type PlayUsageTokenBreakdown struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
	Total         int64 `json:"total"`
}

type PlayArenaUserReport struct {
	CurrentFarm               *PlayQuestToday         `json:"current_farm,omitempty"`
	UserID                    int64                   `json:"user_id"`
	DayStart                  time.Time               `json:"day_start"`
	DayEnd                    time.Time               `json:"day_end"`
	MonthStart                time.Time               `json:"month_start"`
	MonthEnd                  time.Time               `json:"month_end"`
	AsOf                      time.Time               `json:"as_of"`
	Daily                     PlayUsageTokenBreakdown `json:"daily"`
	Monthly                   PlayUsageTokenBreakdown `json:"monthly"`
	MonthlyActualTokenEnergy  int64                   `json:"monthly_actual_token_energy"`
	MonthlyLegacyTokenEnergy  int64                   `json:"monthly_legacy_token_energy"`
	QualificationGrowthEnergy int64                   `json:"qualification_growth_energy"`
}

type PlayArenaAccountingRepository interface {
	GetArenaUserUsageReport(context.Context, int64, time.Time, time.Time, time.Time, time.Time, time.Time) (*PlayArenaUserReport, error)
}

func (s *PlayService) GetAdminArenaUserReport(ctx context.Context, userID int64, date string) (*PlayArenaUserReport, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("PLAY_ARENA_USER_INVALID", "invalid user id")
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	now := s.serverNow().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if date != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", date, loc)
		if parseErr != nil || parsed.After(day) {
			return nil, infraerrors.BadRequest("PLAY_ARENA_DATE_INVALID", "invalid report date")
		}
		day = parsed
	}
	month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, loc)
	repo, ok := s.repo.(PlayArenaAccountingRepository)
	if !ok {
		return nil, fmt.Errorf("arena accounting repository unavailable")
	}
	out, err := repo.GetArenaUserUsageReport(ctx, userID, day, day.AddDate(0, 0, 1), month, month.AddDate(0, 1, 0), now)
	if err != nil {
		return nil, err
	}
	out.UserID, out.DayStart, out.DayEnd = userID, day, day.AddDate(0, 0, 1)
	out.MonthStart, out.MonthEnd, out.AsOf = month, month.AddDate(0, 1, 0), now
	out.Daily.Total = out.Daily.Input + out.Daily.Output + out.Daily.CacheCreation + out.Daily.CacheRead
	out.Monthly.Total = out.Monthly.Input + out.Monthly.Output + out.Monthly.CacheCreation + out.Monthly.CacheRead
	out.MonthlyActualTokenEnergy = out.Monthly.Total / 10000
	out.MonthlyLegacyTokenEnergy = (out.Monthly.Total - out.Monthly.CacheRead) / 10000
	// Historical task settings were not snapshotted. Only today's energy can
	// be compared with the current user page without inventing old rules.
	if day.Format("2006-01-02") == now.Format("2006-01-02") {
		out.CurrentFarm, err = s.getQuestsToday(ctx, userID, now, false)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

type PlayArenaAccountingChange struct {
	UserID    int64   `json:"user_id"`
	OldTokens int64   `json:"old_tokens"`
	NewTokens int64   `json:"new_tokens"`
	OldRank   int     `json:"old_rank"`
	NewRank   int     `json:"new_rank"`
	OldReward float64 `json:"old_reward"`
	NewReward float64 `json:"new_reward"`
}
type PlayArenaAccountingPreview struct {
	RewardPolicySource string `json:"reward_policy_source"`
	dailyPolicy        *PlayArenaDailyPolicy
	PeriodID           int64                       `json:"period_id"`
	Version            int                         `json:"version"`
	Status             string                      `json:"status"`
	StartAt            time.Time                   `json:"start_at"`
	EndAt              time.Time                   `json:"end_at"`
	HasPayout          bool                        `json:"has_payout"`
	CanActivate        bool                        `json:"can_activate"`
	Digest             string                      `json:"digest"`
	TotalUsers         int                         `json:"total_users"`
	Changes            []PlayArenaAccountingChange `json:"changes"`
	Truncated          bool                        `json:"truncated"`
}
type PlayArenaAccountingActivation struct {
	PeriodID       int64  `json:"period_id"`
	ExpectedDigest string `json:"expected_digest"`
	ActorUserID    int64  `json:"-"`
	Reason         string `json:"reason"`
}
type PlayArenaAccountingVersionRepository interface {
	PreviewArenaAccounting(context.Context, int64) (*PlayArenaAccountingPreview, error)
	ActivateArenaAccounting(context.Context, PlayArenaAccountingActivation) error
}

func (s *PlayService) PreviewArenaAccounting(ctx context.Context, id int64) (*PlayArenaAccountingPreview, error) {
	repo, ok := s.repo.(PlayArenaAccountingVersionRepository)
	if !ok {
		return nil, fmt.Errorf("arena accounting version repository unavailable")
	}
	if id <= 0 {
		return nil, ErrPlayArenaNoPeriod
	}
	out, err := repo.PreviewArenaAccounting(ctx, id)
	if err != nil {
		return nil, err
	}
	out.CanActivate = out.Version == 1 && out.Status == "active" && !out.HasPayout && s.serverNow().Before(out.EndAt)
	period, err := s.repo.GetArenaPeriodByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var tiers []PlayArenaSettlementTier
	if period != nil && period.PeriodType == "daily" {
		policy, policyErr := s.dailyArenaPolicyForPeriod(ctx, period)
		if policyErr != nil {
			return nil, policyErr
		}
		// A legacy day still uses live rules. Include them in confirmation so a
		// configuration change invalidates the preview before freezing the policy.
		raw, marshalErr := json.Marshal(policy)
		if marshalErr != nil {
			return nil, marshalErr
		}
		out.Digest = fmt.Sprintf("%x", md5.Sum(append([]byte(out.Digest+":"), raw...)))
		out.dailyPolicy = &policy
		for i := range out.Changes {
			out.Changes[i].OldReward = dailyArenaRewardForRank(out.Changes[i].OldRank, policy)
			out.Changes[i].NewReward = dailyArenaRewardForRank(out.Changes[i].NewRank, policy)
		}
		return out, nil
	} else {
		tiers, err = s.arenaRewardTiersForPeriod(ctx, period)
		if err != nil {
			return nil, err
		}
	}
	for i := range out.Changes {
		out.Changes[i].OldReward = arenaRewardForRank(out.Changes[i].OldRank, tiers)
		out.Changes[i].NewReward = arenaRewardForRank(out.Changes[i].NewRank, tiers)
	}
	return out, nil
}
func (s *PlayService) ActivateArenaAccounting(ctx context.Context, input PlayArenaAccountingActivation) (*PlayArenaAccountingPreview, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.ActorUserID <= 0 || len([]rune(input.Reason)) < 10 || len([]rune(input.Reason)) > 500 || len(input.ExpectedDigest) != 32 {
		return nil, infraerrors.BadRequest("PLAY_ARENA_ACTIVATION_INVALID", "actor, preview digest and a reason of 10–500 characters are required")
	}
	repo, ok := s.repo.(PlayArenaAccountingVersionRepository)
	if !ok {
		return nil, fmt.Errorf("arena accounting version repository unavailable")
	}
	err := s.withPlayTx(ctx, func(txCtx context.Context) error {
		locker, ok := s.repo.(PlayArenaSettlementLocker)
		if !ok {
			return fmt.Errorf("arena settlement lock unavailable")
		}
		if err := locker.LockArenaSettlement(txCtx, input.PeriodID); err != nil {
			return err
		}
		preview, err := s.PreviewArenaAccounting(txCtx, input.PeriodID)
		if err != nil {
			return err
		}
		if !preview.CanActivate || preview.Digest != input.ExpectedDigest {
			return infraerrors.Conflict("PLAY_ARENA_PREVIEW_STALE", "period or usage changed; refresh the preview")
		}
		if policyRepo, ok := s.repo.(PlayArenaDailyPolicyRepository); ok && preview.dailyPolicy != nil {
			if err := policyRepo.FreezeDailyArenaPolicy(txCtx, input.PeriodID, *preview.dailyPolicy); err != nil {
				return err
			}
		}
		return repo.ActivateArenaAccounting(txCtx, input)
	})
	if err != nil {
		return nil, err
	}
	return s.PreviewArenaAccounting(ctx, input.PeriodID)
}
