package service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	playGrowthRunnerInterval    = 5 * time.Minute
	teamRewardSettlementLockKey = "play:team_reward:settlement"
	teamRewardSettlementLockTTL = 15 * time.Minute
)

// PlayGrowthRunner settles expired daily arena periods and purges expired image studio jobs.
type PlayGrowthRunner struct {
	playService *PlayService
	imageStudio *ImageStudioService
	lockCache   LeaderLockCache
	db          *sql.DB
	ownerID     string
	mu          sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewPlayGrowthRunner(
	playService *PlayService,
	imageStudio *ImageStudioService,
	lockCache LeaderLockCache,
	db *sql.DB,
) *PlayGrowthRunner {
	return &PlayGrowthRunner{
		playService: playService,
		imageStudio: imageStudio,
		lockCache:   lockCache,
		db:          db,
		ownerID:     uuid.NewString(),
	}
}

func (r *PlayGrowthRunner) Start() {
	if r == nil || r.playService == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(playGrowthRunnerInterval)
		defer ticker.Stop()
		r.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.runOnce(ctx)
			}
		}
	}()
}

func (r *PlayGrowthRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *PlayGrowthRunner) runOnce(ctx context.Context) {
	if r.playService != nil {
		now := time.Now().In(timezone.Location())
		if n, err := r.playService.SettleExpiredDailyArenaPeriods(ctx, now); err != nil {
			logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] settle daily periods: %v", err)
		} else if n > 0 {
			logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] settled %d daily arena periods", n)
		}
		release, ok := tryAcquireSingletonLeaderLock(
			ctx,
			r.lockCache,
			r.db,
			teamRewardSettlementLockKey,
			r.ownerID,
			teamRewardSettlementLockTTL,
		)
		if ok {
			if n, err := r.playService.SettleDueTeamRewardMonths(ctx, now); err != nil {
				logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] settle team rewards: %v", err)
			} else if n > 0 {
				logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] settled %d team reward months", n)
			}
			release()
		}
	}
	if r.imageStudio != nil {
		if n, err := r.imageStudio.PurgeExpiredJobs(ctx, time.Now()); err != nil {
			logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] purge image studio jobs: %v", err)
		} else if n > 0 {
			logger.LegacyPrintf("play.growth_runner", "[PlayGrowthRunner] purged %d expired image studio jobs", n)
		}
	}
}

func ProvidePlayGrowthRunner(
	playService *PlayService,
	imageStudio *ImageStudioService,
	lockCache LeaderLockCache,
	db *sql.DB,
) *PlayGrowthRunner {
	svc := NewPlayGrowthRunner(playService, imageStudio, lockCache, db)
	svc.Start()
	return svc
}
