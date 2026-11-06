package service

import (
	"context"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type PlayArenaSettlementLocker interface {
	LockArenaSettlement(context.Context, int64) error
	HasArenaRewardLedger(context.Context, int64) (bool, error)
}

// New settlements commit all winners and the final period state together.
// This also serializes accounting changes with settlements across instances.
// A pre-upgrade partial payout lacks a complete frozen winner list: require
// evidence review rather than guessing unpaid winners from a changed ranking.
func (s *PlayService) atomicArenaSettlement(ctx context.Context, id int64, run func(context.Context) (*PlayArenaSettlementResult, error)) (*PlayArenaSettlementResult, error, bool) {
	locker, ok := s.repo.(PlayArenaSettlementLocker)
	if !ok || dbent.TxFromContext(ctx) != nil {
		return nil, nil, false
	}
	var result *PlayArenaSettlementResult
	err := s.withPlayTx(ctx, func(txCtx context.Context) error {
		if err := locker.LockArenaSettlement(txCtx, id); err != nil {
			return err
		}
		exists, err := locker.HasArenaRewardLedger(txCtx, id)
		if err != nil {
			return err
		}
		if exists {
			return infraerrors.Conflict("PLAY_ARENA_LEGACY_PAYOUT_REVIEW_REQUIRED", "existing payout evidence requires reconciliation before resuming this period")
		}
		result, err = run(txCtx)
		return err
	})
	return result, err, true
}
