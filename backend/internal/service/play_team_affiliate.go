package service

import (
	"context"
	"errors"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const PlayRewardSourceTeamAffiliateBonus = "team_affiliate_bonus"

type PlayTeamAffiliateInfo struct {
	Enabled             bool    `json:"enabled"`
	TokenThreshold      int64   `json:"token_threshold"`
	MilestoneReached    bool    `json:"milestone_reached"`
	TokensToMilestone   int64   `json:"tokens_to_milestone,omitempty"`
	CaptainBonus        float64 `json:"captain_bonus,omitempty"`
	CaptainBonusGranted bool    `json:"captain_bonus_granted,omitempty"`
}

func (s *PlayService) enrichTeamAffiliate(
	ctx context.Context,
	teamID int64,
	captainID int64,
	tokenSum int64,
	rt PlayRuntime,
) (*PlayTeamAffiliateInfo, error) {
	if !rt.TeamAffiliateEnabled || !rt.AgentTeamEnabled {
		return nil, nil
	}
	info := &PlayTeamAffiliateInfo{
		Enabled:        true,
		TokenThreshold: rt.TeamAffiliateTokenThreshold,
		CaptainBonus:   rt.TeamAffiliateCaptainBonus,
	}
	if rt.TeamAffiliateTokenThreshold > 0 {
		info.MilestoneReached = tokenSum >= rt.TeamAffiliateTokenThreshold
		if !info.MilestoneReached {
			info.TokensToMilestone = rt.TeamAffiliateTokenThreshold - tokenSum
		}
	}

	if info.MilestoneReached && captainID > 0 && rt.TeamAffiliateCaptainBonus > 0 {
		granted, err := s.tryGrantTeamCaptainAffiliateBonus(ctx, teamID, captainID, tokenSum, rt)
		if err != nil {
			return nil, err
		}
		info.CaptainBonusGranted = granted
	}
	return info, nil
}

func (s *PlayService) tryGrantTeamCaptainAffiliateBonus(
	ctx context.Context,
	teamID, captainID int64,
	tokenSum int64,
	rt PlayRuntime,
) (bool, error) {
	if s.affiliateService == nil || !s.affiliateService.IsEnabled(ctx) {
		return false, nil
	}

	now := s.serverNow()
	monthKey := now.Format("2006-01")
	idempotencyKey := fmt.Sprintf("team_affiliate_bonus:%d:%s", teamID, monthKey)

	recorded, err := s.recordPlayIdempotency(ctx, captainID, PlayRewardSourceTeamAffiliateBonus, idempotencyKey, rt.TeamAffiliateCaptainBonus, map[string]any{
		"team_id":    teamID,
		"token_sum":  tokenSum,
		"month":      monthKey,
		"quota_only": true,
	})
	if err != nil {
		return false, err
	}
	if !recorded {
		return true, nil
	}

	applied, err := s.affiliateService.AccrueBonusQuota(ctx, captainID, rt.TeamAffiliateCaptainBonus)
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *PlayService) recordPlayIdempotency(
	ctx context.Context,
	userID int64,
	source string,
	idempotencyKey string,
	amount float64,
	detail map[string]any,
) (bool, error) {
	if s.entClient == nil {
		return false, fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin play idempotency tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	entry := PlayRewardLedgerEntry{
		UserID:         userID,
		Source:         source,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		Detail:         detail,
	}
	if err := s.repo.InsertRewardLedger(txCtx, entry); err != nil {
		if errors.Is(err, ErrPlayRewardDuplicate) {
			return false, nil
		}
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit play idempotency tx: %w", err)
	}
	return true, nil
}
