package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountTokenGuardV2LifecycleRegressionCancelledProbePreservesHistory(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	lastProbeAt := at.Add(-time.Hour)
	lastReauthAt := at.Add(-30 * time.Minute)
	cooldownUntil := at.Add(time.Minute)
	original := AccountTokenGuardV2Record{
		AccountID: 42, Enabled: true, AutoReloginEnabled: false,
		ProbeState: AccountTokenGuardV2ProbeAuth, ProbeDetail: "previous inspection",
		FailStreak: 2, LastProbeAt: &lastProbeAt, LastReauthAt: &lastReauthAt,
		NextProbeAt: at.Add(-time.Minute), CooldownUntil: &cooldownUntil,
		BlockedReason: "previous automation state", ReloginFailStreak: 1, LastCountedTaskID: 8,
	}
	record := original
	repo := &accountTokenGuardV2TestRepo{record: &record}
	reauth, reader, _, _, _, _ := newReauthTestService("acct-1")
	svc := NewAccountTokenGuardV2Service(repo, nil, reader, accountTokenGuardV2TestProber{}, reauth)
	t.Cleanup(svc.cancel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, svc.runProbe(ctx, record), context.Canceled)

	require.Nil(t, repo.completion, "a cancelled inspection must not publish an outcome")
	require.Equal(t, original, *repo.record, "cancellation must preserve prior inspection history, schedule and budget")
}
