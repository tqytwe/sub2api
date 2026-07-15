package service

import (
	"context"
	"time"
)

type ArenaAggregateRepository interface {
	GetArenaAggregateScore(ctx context.Context, userID int64, periodType string, periodStart time.Time) (score int64, rank int, gap int64, newcomerRank, participants int, err error)
	ListArenaAggregateLeaderboard(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]PlayArenaScoreRow, error)
}

func (s *PlayService) refreshArenaAggregates(ctx context.Context) {
	_ = s.RefreshGrowthWorld(ctx, s.serverNow())
}

func (s *PlayService) GetArenaSummary(ctx context.Context, userID int64, daily bool) (*PlayArenaSummary, error) {
	var current *PlayArenaCurrent
	var err error
	periodType := "monthly"
	if daily {
		periodType = "daily"
		current, err = s.GetDailyArenaCurrent(ctx, userID)
	} else {
		current, err = s.GetArenaCurrent(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	out := &PlayArenaSummary{Enabled: current.Enabled, Period: current.Period}
	if !current.Enabled || current.Period == nil {
		return out, nil
	}
	out.MyScore = current.DisplayTokenSum
	out.MyRank = current.Rank
	out.ScoreMultiplierApplied = current.ArenaScoreMultiplier
	if out.ScoreMultiplierApplied <= 0 {
		out.ScoreMultiplierApplied = 1
	}
	out.TokensToPreviousRank = current.TokensToPrevRank
	if repo, ok := s.repo.(ArenaAggregateRepository); ok {
		_, _, _, newcomerRank, participants, err := repo.GetArenaAggregateScore(ctx, userID, periodType, current.Period.StartAt)
		if err != nil {
			return nil, err
		}
		out.NewcomerRank = newcomerRank
		out.Participants = participants
	}
	return out, nil
}
