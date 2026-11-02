//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV3ProberWaitPlanAcquiresAndReleasesSlot(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	prober := &ChannelMonitorV3Prober{concurrency: NewConcurrencyService(cache)}
	selectionReleased := 0
	route := &channelMonitorV3ProbeRoute{
		wait:    &AccountWaitPlan{AccountID: 71, MaxConcurrency: 1, Timeout: time.Second},
		release: func() { selectionReleased++ },
	}

	release, err := prober.holdProbeSlot(context.Background(), route)
	require.NoError(t, err)
	require.Equal(t, 1, selectionReleased, "release the scheduler selection after acquiring the account slot")
	require.Empty(t, cache.releasedAccountIDs, "the slot remains held until the probe finishes")

	release()
	release()
	require.Equal(t, []int64{71}, cache.releasedAccountIDs, "slot release is idempotent")
}

func TestChannelMonitorV3ProberKeepsSchedulerAcquiredSlot(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	prober := &ChannelMonitorV3Prober{concurrency: NewConcurrencyService(cache)}
	selectionReleased := 0
	release, err := prober.holdProbeSlot(context.Background(), &channelMonitorV3ProbeRoute{
		acquired: true,
		release:  func() { selectionReleased++ },
	})
	require.NoError(t, err)
	require.Zero(t, selectionReleased)
	require.Empty(t, cache.releasedAccountIDs)

	release()
	release()
	require.Equal(t, 1, selectionReleased, "the scheduler-acquired slot is released exactly once")
	require.Empty(t, cache.releasedAccountIDs, "an already acquired slot must not be acquired again")
}

func TestChannelMonitorV3ProberReturnsSlotCacheError(t *testing.T) {
	wantErr := errors.New("cache unavailable")
	cache := &stubConcurrencyCacheForTest{acquireErr: wantErr}
	prober := &ChannelMonitorV3Prober{concurrency: NewConcurrencyService(cache)}
	selectionReleased := 0
	route := &channelMonitorV3ProbeRoute{
		wait:    &AccountWaitPlan{AccountID: 71, MaxConcurrency: 1, Timeout: time.Second},
		release: func() { selectionReleased++ },
	}

	release, err := prober.holdProbeSlot(context.Background(), route)
	require.Nil(t, release)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, selectionReleased)
}

func TestChannelMonitorV3ProberLeavesClaimUnfinishedWhenShutdownCancelsSlotWait(t *testing.T) {
	runner := &channelMonitorV3ProbeRunnerFake{}
	prober, repo := channelMonitorV3ProbeFixture(runner)
	ctx, cancel := context.WithCancel(context.Background())
	prober.concurrency = NewConcurrencyService(&stubConcurrencyCacheForTest{acquireResult: false})
	prober.router = &channelMonitorV3ProbeRouterFake{
		routes: []channelMonitorV3ProbeRouteFake{{
			account: &Account{ID: 71, Platform: PlatformOpenAI},
			model:   "gpt-5.5",
			wait:    &AccountWaitPlan{AccountID: 71, MaxConcurrency: 1, Timeout: time.Minute},
		}},
		onRoute: cancel,
	}

	prober.RunDue(ctx, prober.now())

	require.Zero(t, runner.called)
	require.Empty(t, repo.finished, "shutdown during slot wait must leave the probe claim unfinished")
}
