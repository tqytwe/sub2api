//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2CandyRepositoryHistoryAndRetention(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: "v2-candy-" + uuid.NewString(), Platform: service.PlatformOpenAI})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, group.ID)
		require.NoError(t, err)
	})

	repo := NewChannelMonitorV2Repository(integrationDB).(*channelMonitorV2Repository)
	cfg, err := repo.GetConfig(ctx)
	require.NoError(t, err)
	probe := service.ChannelMonitorV2CandyProbe{GroupID: group.ID, Enabled: true, Model: "gpt-5.5", IntervalMinutes: 5}
	otherProbe := probe
	otherProbe.Model, otherProbe.ReasoningEffort = "gpt-5.5-mini", "low"
	slot := time.Now().UTC().Truncate(5 * time.Minute).Add(-time.Hour)

	first, err := repo.ClaimCandyProbe(ctx, probe, slot, cfg.Version)
	require.NoError(t, err)
	require.Positive(t, first)
	duplicate, err := repo.ClaimCandyProbe(ctx, probe, slot, cfg.Version)
	require.NoError(t, err)
	require.Zero(t, duplicate, "the same model configuration must claim a slot once")
	second, err := repo.ClaimCandyProbe(ctx, otherProbe, slot, cfg.Version)
	require.NoError(t, err)
	require.Positive(t, second, "different model configurations have independent slots")

	require.NoError(t, repo.FinishCandyProbe(ctx, service.ChannelMonitorV2CandyResult{
		ID: first, GroupID: group.ID, ConfigKey: probe.ConfigKey(), Verdict: "correct", LatencyMs: 240,
		AnswerPreview: "21 candies",
	}))
	require.NoError(t, repo.FinishCandyProbe(ctx, service.ChannelMonitorV2CandyResult{
		ID: second, GroupID: group.ID, ConfigKey: otherProbe.ConfigKey(), Verdict: "incorrect", LatencyMs: 320,
		Reason: "answer_mismatch",
	}))

	history, err := repo.CandyHistory(ctx, []service.ChannelMonitorV2CandyScope{
		{GroupID: group.ID, ConfigKey: probe.ConfigKey()},
		{GroupID: group.ID, ConfigKey: otherProbe.ConfigKey()},
	}, time.Now().Add(-time.Hour), 15)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.ElementsMatch(t, []string{probe.ConfigKey(), otherProbe.ConfigKey()}, []string{history[0].ConfigKey, history[1].ConfigKey})
	otherScope, err := repo.CandyHistory(ctx, []service.ChannelMonitorV2CandyScope{{GroupID: group.ID, ConfigKey: "unconfigured"}}, time.Now().Add(-time.Hour), 15)
	require.NoError(t, err)
	require.Empty(t, otherScope, "history is isolated by the saved configuration identity")

	running, err := repo.ClaimCandyProbe(ctx, probe, slot.Add(5*time.Minute), cfg.Version)
	require.NoError(t, err)
	require.Positive(t, running)
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_monitor_v2_candy_results SET checked_at = NOW() - INTERVAL '3 minutes' WHERE id = $1`, running)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE channel_monitor_v2_candy_results SET checked_at = NOW() - INTERVAL '25 hours' WHERE id = $1`, first)
	require.NoError(t, err)

	require.NoError(t, repo.PruneCandyHistory(ctx, time.Now()))
	var runningStatus, oldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_monitor_v2_candy_results WHERE id = $1 AND status = 'finished' AND verdict = 'error' AND reason = 'timeout'`, running).Scan(&runningStatus))
	require.Equal(t, 1, runningStatus, "stale running claims are finished as timeouts")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_monitor_v2_candy_results WHERE id = $1`, first).Scan(&oldCount))
	require.Zero(t, oldCount, "finished history older than 24 hours is pruned")
}
