package service

import (
	"context"
	"time"
)

func (s *PlayService) GrantRechargeBoost(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return nil
	}
	rt := s.GetRuntime(ctx)
	if !rt.RechargeBoostEnabled {
		return nil
	}
	hours := rt.RechargeBoostDurationHours
	if hours <= 0 {
		hours = 24
	}
	expiresAt := s.serverNow().Add(time.Duration(hours) * time.Hour)
	return s.repo.UpsertRechargeBoost(ctx, userID, expiresAt)
}

func (s *PlayService) getRechargeBoostStatus(ctx context.Context, userID int64, rt PlayRuntime) (PlayRechargeBoostStatus, error) {
	out := PlayRechargeBoostStatus{
		CheckinMultiplier:  1,
		BlindboxExtraOpens: 0,
		ArenaMultiplier:    1,
	}
	if userID <= 0 || !rt.RechargeBoostEnabled {
		return out, nil
	}
	expiresAt, err := s.repo.GetActiveRechargeBoost(ctx, userID, s.serverNow())
	if err != nil || expiresAt == nil {
		return out, err
	}
	out.Active = true
	out.ExpiresAt = *expiresAt
	out.CheckinMultiplier = rt.RechargeBoostCheckinMult
	if out.CheckinMultiplier <= 0 {
		out.CheckinMultiplier = 2
	}
	out.BlindboxExtraOpens = rt.RechargeBoostBlindboxExtra
	if out.BlindboxExtraOpens < 0 {
		out.BlindboxExtraOpens = 0
	}
	out.ArenaMultiplier = rt.RechargeBoostArenaMult
	if out.ArenaMultiplier <= 0 {
		out.ArenaMultiplier = 1.5
	}
	return out, nil
}

func applyArenaScoreMultiplier(tokenSum int64, mult float64) int64 {
	if tokenSum <= 0 || mult <= 1 {
		return tokenSum
	}
	return int64(float64(tokenSum) * mult)
}
