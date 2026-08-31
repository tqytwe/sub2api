package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// OpsHourlyAggregationJobName is also the durable completion watermark key
	// consumed by the public-status snapshot worker.
	OpsHourlyAggregationJobName = "ops_preaggregation_hourly"
	opsAggDailyJobName          = "ops_preaggregation_daily"

	opsAggHourlyInterval = 10 * time.Minute
	opsAggDailyInterval  = 1 * time.Hour

	// Keep in sync with ops retention target (vNext default 30d).
	opsAggBackfillWindow = 1 * time.Hour
	// A missing durable watermark is the first-run case for the public status
	// contract. We must materialize the entire availability window before
	// publishing any completion point: a two-hour bootstrap would otherwise
	// label an incomplete 30-day aggregate as fresh.
	opsAggHourlyBootstrapWindow = 30 * 24 * time.Hour

	// Recompute overlap to absorb late-arriving rows near boundaries.
	opsAggHourlyOverlap = 2 * time.Hour
	opsAggDailyOverlap  = 48 * time.Hour
	// A stale completion watermark is resumed in bounded slices. The overlap is
	// included before each slice so late rows near the prior completion boundary
	// are materialized before that boundary moves forward again.
	opsAggHourlyCatchupWindow = 6 * time.Hour

	opsAggHourlyChunk = 24 * time.Hour
	opsAggDailyChunk  = 7 * 24 * time.Hour

	// Delay around boundaries (e.g. 10:00..10:05) to avoid aggregating buckets
	// that may still receive late inserts.
	opsAggSafeDelay = 5 * time.Minute

	opsAggMaxQueryTimeout = 5 * time.Second
	opsAggHourlyTimeout   = 5 * time.Minute
	opsAggDailyTimeout    = 2 * time.Minute

	opsAggHourlyLeaderLockKey = "ops:aggregation:hourly:leader"
	opsAggDailyLeaderLockKey  = "ops:aggregation:daily:leader"

	opsAggHourlyLeaderLockTTL = 15 * time.Minute
	opsAggDailyLeaderLockTTL  = 10 * time.Minute
)

// OpsAggregationService periodically backfills ops_metrics_hourly / ops_metrics_daily
// for stable long-window dashboard queries.
//
// It is safe to run in multi-replica deployments when Redis is available (leader lock).
type OpsAggregationService struct {
	opsRepo     OpsRepository
	settingRepo SettingRepository
	cfg         *config.Config

	db          *sql.DB
	redisClient *redis.Client
	instanceID  string

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once

	hourlyMu sync.Mutex
	dailyMu  sync.Mutex

	skipLogMu sync.Mutex
	skipLogAt time.Time
}

func NewOpsAggregationService(
	opsRepo OpsRepository,
	settingRepo SettingRepository,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsAggregationService {
	return &OpsAggregationService{
		opsRepo:     opsRepo,
		settingRepo: settingRepo,
		cfg:         cfg,
		db:          db,
		redisClient: redisClient,
		instanceID:  uuid.NewString(),
	}
}

func (s *OpsAggregationService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		if s.stopCh == nil {
			s.stopCh = make(chan struct{})
		}
		go s.hourlyLoop()
		go s.dailyLoop()
	})
}

func (s *OpsAggregationService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
		}
	})
}

func (s *OpsAggregationService) hourlyLoop() {
	// First run immediately.
	s.aggregateHourly()

	ticker := time.NewTicker(opsAggHourlyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.aggregateHourly()
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpsAggregationService) dailyLoop() {
	// First run immediately.
	s.aggregateDaily()

	ticker := time.NewTicker(opsAggDailyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.aggregateDaily()
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpsAggregationService) aggregateHourly() {
	if s == nil || s.opsRepo == nil {
		return
	}
	if s.cfg != nil {
		if !s.cfg.Ops.Enabled {
			return
		}
		if !s.cfg.Ops.Aggregation.Enabled {
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsAggHourlyTimeout)
	defer cancel()

	if !s.isMonitoringEnabled(ctx) {
		return
	}

	release, ok := s.tryAcquireLeaderLock(ctx, opsAggHourlyLeaderLockKey, opsAggHourlyLeaderLockTTL, "[OpsAggregation][hourly]")
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	s.hourlyMu.Lock()
	defer s.hourlyMu.Unlock()

	startedAt := time.Now().UTC()
	runAt := startedAt

	// The durable watermark, not a last non-empty metrics bucket, is the only
	// safe resume point. Reading it must fail closed: an incomplete prior run can
	// have materialized early chunks without being entitled to advance the public
	// completion boundary.
	ctxWatermark, cancelWatermark := context.WithTimeout(context.Background(), opsAggMaxQueryTimeout)
	completedThrough, hasWatermark, err := s.opsRepo.GetHourlyAggregationWatermark(ctxWatermark)
	cancelWatermark()
	if err != nil {
		logger.LegacyPrintf("service.ops_aggregation", "[OpsAggregation][hourly] failed to read completion watermark: %v", err)
		return
	}
	if hasWatermark && !isWholeUTCHour(completedThrough) {
		logger.LegacyPrintf("service.ops_aggregation", "[OpsAggregation][hourly] invalid completion watermark: %s", completedThrough.Format(time.RFC3339))
		return
	}

	start, end := hourlyAggregationWindow(startedAt, completedThrough, hasWatermark)
	if !start.Before(end) {
		return
	}

	aggErr := s.processHourlyWindow(ctx, start, end)

	finishedAt := time.Now().UTC()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	dur := durationMs

	if aggErr != nil {
		msg := truncateString(aggErr.Error(), 2048)
		errAt := finishedAt
		hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer hbCancel()
		_ = s.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
			JobName:        OpsHourlyAggregationJobName,
			LastRunAt:      &runAt,
			LastErrorAt:    &errAt,
			LastError:      &msg,
			LastDurationMs: &dur,
		})
		return
	}

	successAt := finishedAt
	hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer hbCancel()
	result := truncateString(fmt.Sprintf("window=%s..%s", start.Format(time.RFC3339), end.Format(time.RFC3339)), 2048)
	_ = s.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
		JobName:        OpsHourlyAggregationJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &successAt,
		LastDurationMs: &dur,
		LastResult:     &result,
	})
}

// hourlyAggregationWindow returns only full UTC source hours. The first
// successful run covers the complete public-status availability horizon before
// it establishes a durable watermark. On a healthy cadence it replays the
// late-arrival overlap; a stale watermark resumes in bounded slices instead
// of allowing a partial previous run to be skipped.
func hourlyAggregationWindow(now, completedThrough time.Time, hasWatermark bool) (time.Time, time.Time) {
	end := utcFloorToHour(now.UTC().Add(-opsAggSafeDelay))
	lookback := opsAggBackfillWindow
	if opsAggHourlyOverlap > lookback {
		lookback = opsAggHourlyOverlap
	}
	if !hasWatermark {
		return end.Add(-opsAggHourlyBootstrapWindow), end
	}

	completedThrough = completedThrough.UTC()
	if completedThrough.Before(end) {
		catchupEnd := minTime(end, completedThrough.Add(opsAggHourlyCatchupWindow))
		return completedThrough.Add(-opsAggHourlyOverlap), catchupEnd
	}

	return end.Add(-lookback), end
}

func isWholeUTCHour(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	utc := value.UTC()
	return utc.Equal(utc.Truncate(time.Hour))
}

// processHourlyWindow materializes every requested source chunk and advances
// the durable completion watermark only after all chunks have succeeded. An
// empty or already-covered range is still a successful source-window check;
// advancing the watermark in that case is required so snapshots do not stall
// during quiet traffic periods.
func (s *OpsAggregationService) processHourlyWindow(ctx context.Context, start, end time.Time) error {
	if s == nil || s.opsRepo == nil {
		return fmt.Errorf("hourly aggregation repository is unavailable")
	}
	start = start.UTC()
	end = end.UTC()
	if !isWholeUTCHour(start) || !isWholeUTCHour(end) {
		return fmt.Errorf("hourly aggregation windows must use whole UTC hours")
	}
	if start.After(end) {
		return fmt.Errorf("hourly aggregation window end precedes start")
	}
	if start.Equal(end) {
		return s.finalizeHourlyAggregation(ctx, end, nil)
	}

	var aggregateErr error
	for cursor := start; cursor.Before(end); cursor = cursor.Add(opsAggHourlyChunk) {
		chunkEnd := minTime(cursor.Add(opsAggHourlyChunk), end)
		if err := s.opsRepo.UpsertHourlyMetrics(ctx, cursor, chunkEnd); err != nil {
			aggregateErr = err
			logger.LegacyPrintf("service.ops_aggregation", "[OpsAggregation][hourly] upsert failed (%s..%s): %v", cursor.Format(time.RFC3339), chunkEnd.Format(time.RFC3339), err)
			break
		}
	}
	return s.finalizeHourlyAggregation(ctx, end, aggregateErr)
}

// finalizeHourlyAggregation publishes a durable completion point only after
// every aggregation chunk has succeeded. Treat a watermark write failure as a
// failed aggregation run: without it, a public snapshot could freeze a partial
// hour behind its immutable insert trigger.
func (s *OpsAggregationService) finalizeHourlyAggregation(ctx context.Context, completedThrough time.Time, aggregateErr error) error {
	if aggregateErr != nil {
		return aggregateErr
	}
	if s == nil || s.opsRepo == nil {
		return fmt.Errorf("hourly aggregation watermark repository is unavailable")
	}
	if !isWholeUTCHour(completedThrough) {
		return fmt.Errorf("hourly aggregation watermark must use a whole UTC hour")
	}
	if err := s.opsRepo.AdvanceHourlyAggregationWatermark(ctx, completedThrough.UTC()); err != nil {
		return fmt.Errorf("advance hourly aggregation watermark: %w", err)
	}
	return nil
}

func (s *OpsAggregationService) aggregateDaily() {
	if s == nil || s.opsRepo == nil {
		return
	}
	if s.cfg != nil {
		if !s.cfg.Ops.Enabled {
			return
		}
		if !s.cfg.Ops.Aggregation.Enabled {
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsAggDailyTimeout)
	defer cancel()

	if !s.isMonitoringEnabled(ctx) {
		return
	}

	release, ok := s.tryAcquireLeaderLock(ctx, opsAggDailyLeaderLockKey, opsAggDailyLeaderLockTTL, "[OpsAggregation][daily]")
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	s.dailyMu.Lock()
	defer s.dailyMu.Unlock()

	startedAt := time.Now().UTC()
	runAt := startedAt

	end := utcFloorToDay(time.Now().UTC())
	start := end.Add(-opsAggBackfillWindow)

	{
		ctxMax, cancelMax := context.WithTimeout(context.Background(), opsAggMaxQueryTimeout)
		latest, ok, err := s.opsRepo.GetLatestDailyBucketDate(ctxMax)
		cancelMax()
		if err != nil {
			logger.LegacyPrintf("service.ops_aggregation", "[OpsAggregation][daily] failed to read latest bucket: %v", err)
		} else if ok {
			candidate := latest.Add(-opsAggDailyOverlap)
			if candidate.After(start) {
				start = candidate
			}
		}
	}

	start = utcFloorToDay(start)
	if !start.Before(end) {
		return
	}

	var aggErr error
	for cursor := start; cursor.Before(end); cursor = cursor.Add(opsAggDailyChunk) {
		chunkEnd := minTime(cursor.Add(opsAggDailyChunk), end)
		if err := s.opsRepo.UpsertDailyMetrics(ctx, cursor, chunkEnd); err != nil {
			aggErr = err
			logger.LegacyPrintf("service.ops_aggregation", "[OpsAggregation][daily] upsert failed (%s..%s): %v", cursor.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), err)
			break
		}
	}

	finishedAt := time.Now().UTC()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	dur := durationMs

	if aggErr != nil {
		msg := truncateString(aggErr.Error(), 2048)
		errAt := finishedAt
		hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer hbCancel()
		_ = s.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
			JobName:        opsAggDailyJobName,
			LastRunAt:      &runAt,
			LastErrorAt:    &errAt,
			LastError:      &msg,
			LastDurationMs: &dur,
		})
		return
	}

	successAt := finishedAt
	hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer hbCancel()
	result := truncateString(fmt.Sprintf("window=%s..%s", start.Format(time.RFC3339), end.Format(time.RFC3339)), 2048)
	_ = s.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsAggDailyJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &successAt,
		LastDurationMs: &dur,
		LastResult:     &result,
	})
}

func (s *OpsAggregationService) isMonitoringEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}
	if s.cfg != nil && !s.cfg.Ops.Enabled {
		return false
	}
	if s.settingRepo == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpsMonitoringEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return true
		}
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return false
	default:
		return true
	}
}

var opsAggReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (s *OpsAggregationService) tryAcquireLeaderLock(ctx context.Context, key string, ttl time.Duration, logPrefix string) (func(), bool) {
	if s == nil {
		return nil, false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Prefer Redis leader lock when available (multi-instance), but avoid stampeding
	// the DB when Redis is flaky by falling back to a DB advisory lock.
	if s.redisClient != nil {
		ok, err := s.redisClient.SetNX(ctx, key, s.instanceID, ttl).Result()
		if err == nil {
			if !ok {
				s.maybeLogSkip(logPrefix)
				return nil, false
			}
			release := func() {
				ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_, _ = opsAggReleaseScript.Run(ctx2, s.redisClient, []string{key}, s.instanceID).Result()
			}
			return release, true
		}
		// Redis error: fall through to DB advisory lock.
	}

	release, ok := tryAcquireDBAdvisoryLock(ctx, s.db, hashAdvisoryLockID(key))
	if !ok {
		s.maybeLogSkip(logPrefix)
		return nil, false
	}
	return release, true
}

func (s *OpsAggregationService) maybeLogSkip(prefix string) {
	s.skipLogMu.Lock()
	defer s.skipLogMu.Unlock()

	now := time.Now()
	if !s.skipLogAt.IsZero() && now.Sub(s.skipLogAt) < time.Minute {
		return
	}
	s.skipLogAt = now
	if prefix == "" {
		prefix = "[OpsAggregation]"
	}
	logger.LegacyPrintf("service.ops_aggregation", "%s leader lock held by another instance; skipping", prefix)
}

func utcFloorToHour(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

func utcFloorToDay(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
