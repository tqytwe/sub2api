package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

// The distinct costs deliberately model the target's account-stat pricing,
// standard pricing, and user-billed cost. None may replace another.
type lifetimeStatsRepo struct {
	UsageLogRepository
	today       map[int64]*usagestats.AccountStats
	lifetime    map[int64]*usagestats.AccountStats
	todayErr    error
	lifetimeErr error
	mu          sync.Mutex
	starts      []time.Time
}

func (r *lifetimeStatsRepo) GetAccountTodayStats(_ context.Context, id int64) (*usagestats.AccountStats, error) {
	return r.today[id], r.todayErr
}
func (r *lifetimeStatsRepo) GetAccountWindowStats(_ context.Context, id int64, start time.Time) (*usagestats.AccountStats, error) {
	r.mu.Lock()
	r.starts = append(r.starts, start)
	r.mu.Unlock()
	if start.IsZero() {
		return r.lifetime[id], r.lifetimeErr
	}
	return r.today[id], r.todayErr
}

type lifetimeBatchStatsRepo struct {
	*lifetimeStatsRepo
	batchStarts   []time.Time
	batchIDs      [][]int64
	batchTodayErr error
}

func (r *lifetimeBatchStatsRepo) GetAccountWindowStatsBatch(_ context.Context, ids []int64, start time.Time) (map[int64]*usagestats.AccountStats, error) {
	r.batchStarts = append(r.batchStarts, start)
	r.batchIDs = append(r.batchIDs, append([]int64(nil), ids...))
	if start.IsZero() {
		return r.lifetime, r.lifetimeErr
	}
	return r.today, r.batchTodayErr
}

func newLifetimeStatsRepo() *lifetimeStatsRepo {
	return &lifetimeStatsRepo{
		today:    map[int64]*usagestats.AccountStats{17: {Requests: 2, Tokens: 100, Cost: 3, StandardCost: 5, UserCost: 7}},
		lifetime: map[int64]*usagestats.AccountStats{17: {Requests: 200, Tokens: 10000, Cost: 30, StandardCost: 50, UserCost: 70}},
	}
}
func assertLifetimeStatsJSON(t *testing.T, stats *WindowStats) {
	t.Helper()
	data, err := json.Marshal(stats)
	require.NoError(t, err)
	require.JSONEq(t, `{"requests":2,"tokens":100,"cost":3,"standard_cost":5,"user_cost":7,"lifetime_tokens":10000,"lifetime_cost":30}`, string(data))
}

func TestAccountUsageService_TodayLifetimeSinglePreservesCostSemantics(t *testing.T) {
	repo := newLifetimeStatsRepo()
	// No account repository: historical statistics must not depend on an active
	// account, credentials, groups, or current metadata.
	svc := &AccountUsageService{usageLogRepo: repo}
	stats, err := svc.GetTodayStats(context.Background(), 17)
	require.NoError(t, err)
	assertLifetimeStatsJSON(t, stats)
	require.Equal(t, []time.Time{{}}, repo.starts)
}

func TestAccountUsageService_TodayLifetimeBatchPreservesCostSemantics(t *testing.T) {
	repo := &lifetimeBatchStatsRepo{lifetimeStatsRepo: newLifetimeStatsRepo()}
	svc := &AccountUsageService{usageLogRepo: repo}
	before := timezone.Today()
	stats, err := svc.GetTodayStatsBatch(context.Background(), []int64{17, 17, 0, -1, 18})
	require.NoError(t, err)
	assertLifetimeStatsJSON(t, stats[17])
	assertZeroLifetimeJSON(t, stats[18])
	require.Len(t, stats, 2)
	require.Equal(t, [][]int64{{17, 18}, {17, 18}}, repo.batchIDs)
	require.Len(t, repo.batchStarts, 2)
	require.True(t, repo.batchStarts[0].Equal(before) || repo.batchStarts[0].Equal(timezone.Today()))
	require.True(t, repo.batchStarts[1].IsZero())
	require.Empty(t, repo.starts, "successful batches must avoid per-account queries")
}

func TestAccountUsageService_TodayLifetimeFallback(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_batch_reader", true: "failed_batch"}[batch], func(t *testing.T) {
			repo := newLifetimeStatsRepo()
			var reader UsageLogRepository = repo
			if batch {
				reader = &lifetimeBatchStatsRepo{lifetimeStatsRepo: repo, batchTodayErr: errors.New("batch unavailable")}
			}
			svc := &AccountUsageService{usageLogRepo: reader}
			stats, err := svc.GetTodayStatsBatch(context.Background(), []int64{17, 18})
			require.NoError(t, err)
			assertLifetimeStatsJSON(t, stats[17])
			assertZeroLifetimeJSON(t, stats[18])
			var zero, today int
			for _, start := range repo.starts {
				if start.IsZero() {
					zero++
				} else {
					today++
				}
			}
			require.Equal(t, 2, zero)
			require.Equal(t, 2, today)
		})
	}
}

func TestAccountUsageService_TodayLifetimeFailureIsBestEffort(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			repo := newLifetimeStatsRepo()
			repo.lifetimeErr = errors.New("lifetime unavailable")
			svc := &AccountUsageService{usageLogRepo: repo}
			var stats *WindowStats
			if batch {
				svc.usageLogRepo = &lifetimeBatchStatsRepo{lifetimeStatsRepo: repo}
				result, err := svc.GetTodayStatsBatch(context.Background(), []int64{17})
				require.NoError(t, err)
				stats = result[17]
			} else {
				var err error
				stats, err = svc.GetTodayStats(context.Background(), 17)
				require.NoError(t, err)
			}
			require.Equal(t, &WindowStats{Requests: 2, Tokens: 100, Cost: 3, StandardCost: 5, UserCost: 7}, stats)
		})
	}
}

func TestAccountUsageService_TodayLifetimeTodayError(t *testing.T) {
	repo := newLifetimeStatsRepo()
	repo.todayErr = errors.New("today unavailable")
	svc := &AccountUsageService{usageLogRepo: repo}
	stats, err := svc.GetTodayStats(context.Background(), 17)
	require.ErrorIs(t, err, repo.todayErr)
	require.Nil(t, stats)
	require.Empty(t, repo.starts)
}

func TestAccountUsageService_TodayLifetimeFallbackRetainsHistoryWhenTodayFails(t *testing.T) {
	repo := newLifetimeStatsRepo()
	repo.todayErr = errors.New("today unavailable")
	svc := &AccountUsageService{usageLogRepo: repo}
	result, err := svc.GetTodayStatsBatch(context.Background(), []int64{17})
	require.NoError(t, err)
	data, err := json.Marshal(result[17])
	require.NoError(t, err)
	require.JSONEq(t, `{"requests":0,"tokens":0,"cost":0,"standard_cost":0,"user_cost":0,"lifetime_tokens":10000,"lifetime_cost":30}`, string(data))
}

func TestAccountUsageService_TodayLifetimeEmptyBatchAvoidsQueries(t *testing.T) {
	repo := &lifetimeBatchStatsRepo{lifetimeStatsRepo: newLifetimeStatsRepo()}
	svc := &AccountUsageService{usageLogRepo: repo}
	result, err := svc.GetTodayStatsBatch(context.Background(), []int64{0, -1})
	require.NoError(t, err)
	require.Empty(t, result)
	require.Empty(t, repo.batchStarts)
	require.Empty(t, repo.starts)
}

func TestAccountUsageService_TodayLifetimeZeroSerialization(t *testing.T) {
	for _, mode := range []string{"single", "batch", "fallback"} {
		for _, tokens := range []int64{0, 7000} {
			t.Run(fmt.Sprintf("%s/tokens_%d", mode, tokens), func(t *testing.T) {
				repo := newLifetimeStatsRepo()
				repo.today = nil
				repo.lifetime = map[int64]*usagestats.AccountStats{17: {Tokens: tokens, Cost: 0}}
				svc := &AccountUsageService{usageLogRepo: repo}
				var stats *WindowStats
				if mode == "single" {
					var err error
					stats, err = svc.GetTodayStats(context.Background(), 17)
					require.NoError(t, err)
				} else {
					if mode == "batch" {
						svc.usageLogRepo = &lifetimeBatchStatsRepo{lifetimeStatsRepo: repo}
					}
					result, err := svc.GetTodayStatsBatch(context.Background(), []int64{17, 18})
					require.NoError(t, err)
					stats = result[17]
					assertZeroLifetimeJSON(t, result[18])
				}
				data, err := json.Marshal(stats)
				require.NoError(t, err)
				require.JSONEq(t, fmt.Sprintf(`{"requests":0,"tokens":0,"cost":0,"standard_cost":0,"user_cost":0,"lifetime_tokens":%d,"lifetime_cost":0}`, tokens), string(data))
			})
		}
	}
}

func assertZeroLifetimeJSON(t *testing.T, stats *WindowStats) {
	t.Helper()
	data, err := json.Marshal(stats)
	require.NoError(t, err)
	require.JSONEq(t, `{"requests":0,"tokens":0,"cost":0,"standard_cost":0,"user_cost":0,"lifetime_tokens":0,"lifetime_cost":0}`, string(data))
}
