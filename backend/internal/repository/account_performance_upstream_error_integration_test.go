//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountPerformancePostgresUpstreamErrorSavePauseAuditAndRelease(t *testing.T) {
	ctx := context.Background()
	id := performanceCommittedAccount(t)
	repo := &accountPerformanceRepository{db: integrationDB}
	now := time.Now().UTC().Truncate(time.Second)
	original := upstreamstats.Default
	upstreamstats.Default = upstreamstats.NewStore(20, now.Add(-time.Minute))
	t.Cleanup(func() { upstreamstats.Default = original })
	upstreamstats.Default.Record(id, 503, now)
	upstreamstats.Default.Record(id, 200, now)
	max := 25.0
	cfg := service.PerformanceConfig{Enabled: true, UpstreamError: service.PerformanceBounds{Max: &max}}
	require.NoError(t, repo.Save(ctx, id, cfg, 0, 0))
	metrics, err := repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	m := metrics[id]
	require.True(t, m.Config.Enabled)
	require.NotNil(t, m.Config.UpstreamError.Max)
	require.Equal(t, max, *m.Config.UpstreamError.Max)
	require.Equal(t, int64(1), m.Revision)
	require.False(t, m.Paused)
	require.Equal(t, int64(2), m.UpstreamError.Total)
	require.Equal(t, int64(1), m.UpstreamError.Failures)
	require.False(t, m.UpstreamError.WindowComplete, "a new process still evaluates its real observed samples")
	require.NoError(t, repo.Evaluate(ctx, id, now))
	metrics, err = repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	m = metrics[id]
	require.True(t, m.Paused)
	require.Equal(t, int64(1), m.PauseGeneration)
	require.Equal(t, "upstream_error", m.Reason)
	events, err := repo.Events(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "pause", events[0].Action)
	var evidence service.AccountPerformance
	require.NoError(t, json.Unmarshal(events[0].Evidence, &evidence))
	require.Equal(t, m.UpstreamError.Percent, evidence.UpstreamError.Percent)
	require.Equal(t, m.UpstreamError.Total, evidence.UpstreamError.Total)
	require.Equal(t, m.UpstreamError.Failures, evidence.UpstreamError.Failures)
	require.Equal(t, m.UpstreamError.Source, evidence.UpstreamError.Source)
	require.Equal(t, m.UpstreamError.WindowComplete, evidence.UpstreamError.WindowComplete)
	require.True(t, m.UpstreamError.ObservedSince.Equal(evidence.UpstreamError.ObservedSince))
	require.True(t, m.WindowStart.Equal(evidence.WindowStart))
	require.True(t, m.WindowEnd.Equal(evidence.WindowEnd))
	// A healthy later window must not clear the latched performance pause.
	require.NoError(t, repo.Evaluate(ctx, id, now.Add(25*time.Hour)))
	metrics, err = repo.Read(ctx, []int64{id}, now.Add(25*time.Hour))
	require.NoError(t, err)
	require.True(t, metrics[id].Paused)
	require.NoError(t, repo.Release(ctx, id, 1, 1, 0))
	metrics, err = repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	m = metrics[id]
	require.False(t, m.Paused)
	require.False(t, m.Config.Enabled, "manual release keeps the existing disable-on-release default")
	require.Equal(t, int64(2), m.Revision)
	require.Equal(t, max, *m.Config.UpstreamError.Max, "release retains configured bounds")
	require.NoError(t, repo.Evaluate(ctx, id, now))
	events, err = repo.Events(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 3, "disabled evaluation must not immediately pause the account again")
	require.Equal(t, "release", events[0].Action)
	// An explicit re-enable with a positive minimum treats an observed 0% as
	// a real value, unlike the absence of any HTTP responses after restart.
	upstreamstats.Default = upstreamstats.NewStore(20, now.Add(-time.Minute))
	upstreamstats.Default.Record(id, 200, now)
	min := 1.0
	cfg = service.PerformanceConfig{Enabled: true, UpstreamError: service.PerformanceBounds{Min: &min}}
	require.NoError(t, repo.Save(ctx, id, cfg, 2, 0))
	require.NoError(t, repo.Evaluate(ctx, id, now))
	metrics, err = repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	require.True(t, metrics[id].Paused)
	require.Equal(t, int64(2), metrics[id].PauseGeneration)
	require.Equal(t, int64(1), metrics[id].UpstreamError.Total)
	require.NotNil(t, metrics[id].UpstreamError.Percent)
	require.Zero(t, *metrics[id].UpstreamError.Percent)
	events, err = repo.Events(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 5)
	require.Equal(t, "pause", events[0].Action)
}

func TestAccountPerformancePostgresUpstreamErrorLegacyConfig(t *testing.T) {
	ctx := context.Background()
	id := performanceCommittedAccount(t)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_performance_monitors(account_id,config) VALUES($1,'{"enabled":true,"tps":{}}')`, id)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	original := upstreamstats.Default
	upstreamstats.Default = upstreamstats.NewStore(20, now.Add(-time.Hour))
	t.Cleanup(func() { upstreamstats.Default = original })
	upstreamstats.Default.Record(id, 503, now)
	repo := &accountPerformanceRepository{db: integrationDB}
	require.NoError(t, repo.Evaluate(ctx, id, now))
	metrics, err := repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	require.Nil(t, metrics[id].Config.UpstreamError.Min)
	require.Nil(t, metrics[id].Config.UpstreamError.Max)
	require.False(t, metrics[id].Paused, "legacy data never enables a new threshold implicitly")
	events, err := repo.Events(ctx, id)
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestAccountPerformancePostgresCASAndReleasePreserveManualPause(t *testing.T) {
	ctx := context.Background()
	id := performanceCommittedAccount(t)
	repo := &accountPerformanceRepository{db: integrationDB}
	now := time.Now().UTC().Truncate(time.Second)
	original := upstreamstats.Default
	upstreamstats.Default = upstreamstats.NewStore(20, now.Add(-time.Minute))
	t.Cleanup(func() { upstreamstats.Default = original })
	minTPS, maxTTFT, maxDegraded, maxErrors := 10.0, 1500.0, 20.0, 25.0
	cfg := service.PerformanceConfig{Enabled: true, TPS: service.PerformanceBounds{Min: &minTPS}, TTFT: service.PerformanceBounds{Max: &maxTTFT}, Degradation: service.PerformanceBounds{Max: &maxDegraded}, UpstreamError: service.PerformanceBounds{Max: &maxErrors}}
	require.NoError(t, repo.Save(ctx, id, cfg, 0, 0))
	require.ErrorIs(t, repo.Save(ctx, id, service.PerformanceConfig{}, 0, 0), service.ErrPerformanceConflict)
	require.NoError(t, repo.Evaluate(ctx, id, now), "absent timing, quality and HTTP samples must not manufacture a threshold breach")
	metrics, err := repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	require.False(t, metrics[id].Paused)
	require.Equal(t, int64(1), metrics[id].Revision)
	upstreamstats.Default.Record(id, 503, now)
	require.NoError(t, repo.Evaluate(ctx, id, now))
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET schedulable=false WHERE id=$1`, id)
	require.NoError(t, err)
	require.ErrorIs(t, repo.Release(ctx, id, 1, 0, 0), service.ErrPerformanceConflict, "stale pause ownership cannot release the hold")
	require.ErrorIs(t, repo.Release(ctx, id, 0, 1, 0), service.ErrPerformanceConflict, "stale configuration cannot release the hold")
	require.NoError(t, repo.Release(ctx, id, 1, 1, 0))
	var schedulable bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1`, id).Scan(&schedulable))
	require.False(t, schedulable, "release belongs only to the performance hold; manual pause remains")
	metrics, err = repo.Read(ctx, []int64{id}, now)
	require.NoError(t, err)
	require.False(t, metrics[id].Paused)
	require.False(t, metrics[id].Config.Enabled)
	require.Equal(t, int64(2), metrics[id].Revision)
	events, err := repo.Events(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 3, "CAS conflicts must not append audit events")
}
