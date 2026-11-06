package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

type PlayCheckinTransactionRepository interface {
	LockCheckinUser(context.Context, int64) error
	HasCheckinMilestoneReward(context.Context, int64, time.Time) (bool, error)
}

// Normal check-in and makeup read the same date sequence under a per-user
// transaction lock. The unique date constraint remains the final grant fence.
func (s *PlayService) checkinTransaction(ctx context.Context, userID int64, run func(context.Context) (*PlayCheckinResult, error)) (*PlayCheckinResult, error, bool) {
	repo, ok := s.repo.(PlayCheckinTransactionRepository)
	if !ok || dbent.TxFromContext(ctx) != nil {
		return nil, nil, false
	}
	var result *PlayCheckinResult
	err := s.withPlayTx(ctx, func(txCtx context.Context) error {
		if err := repo.LockCheckinUser(txCtx, userID); err != nil {
			return err
		}
		var err error
		result, err = run(txCtx)
		return err
	})
	return result, err, true
}

func (s *PlayService) unpaidCheckinMilestone(ctx context.Context, userID int64, date time.Time, bonus float64) (float64, error) {
	if bonus <= 0 {
		return bonus, nil
	}
	if repo, ok := s.repo.(PlayCheckinTransactionRepository); ok {
		paid, err := repo.HasCheckinMilestoneReward(ctx, userID, date)
		if err != nil {
			return 0, err
		}
		if paid {
			return 0, nil
		}
	}
	return bonus, nil
}
