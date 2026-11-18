package service

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type accountOpsRoleSettings struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func (r *accountOpsRoleSettings) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *accountOpsRoleSettings) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}
func (r *accountOpsRoleSettings) setConfig(t *testing.T, c AccountOpsConfig) {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	require.NoError(t, r.Set(context.Background(), accountOpsSettingsKey, string(raw)))
}

type accountOpsRoleAccounts struct {
	AccountRepository
	scans atomic.Int64
	items []Account
}

func (r *accountOpsRoleAccounts) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]Account, error) {
	r.scans.Add(1)
	return r.items, nil
}

type accountOpsRoleEvents struct {
	AccountOpsRepository
	refreshed chan AccountOpsConfig
	recorded  chan AccountOpsEvent
}

func (r *accountOpsRoleEvents) SuppressDisabled(_ context.Context, c AccountOpsConfig) error {
	select {
	case r.refreshed <- c:
	default:
	}
	return nil
}
func (r *accountOpsRoleEvents) Record(_ context.Context, event AccountOpsEvent) error {
	r.recorded <- event
	return nil
}

func TestAccountOpsProviderReconcilesGroupsAndRefreshesEventPolicy(t *testing.T) {
	settings := &accountOpsRoleSettings{values: make(map[string]string)}
	c := defaultAccountOpsConfig()
	c.Enabled, c.WeeklyQuota = true, true
	c.Recipient = "first@example.test"
	settings.setConfig(t, c)
	accounts := &accountOpsRoleAccounts{items: []Account{{
		ID: 42, Name: "test account", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://example.com/v1"},
	}}}
	events := &accountOpsRoleEvents{refreshed: make(chan AccountOpsConfig, 8), recorded: make(chan AccountOpsEvent, 8)}
	svc := ProvideAccountOpsService(settings, events, nil, accounts, &config.Config{}, nil, nil, nil, nil, nil)
	defer svc.Stop()
	require.Eventually(t, func() bool { return svc.currentConfig().GroupsInitialized }, time.Second, time.Millisecond)
	require.EqualValues(t, 2, accounts.scans.Load(), "initial group persistence requires a fresh account snapshot")
	stored, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Len(t, stored.Groups, 1)
	require.Equal(t, []int64{42}, stored.Groups[0].AccountIDs)
	stored.Recipient = "updated@example.test"
	stored.WeeklyQuota, stored.BalanceLow = false, true
	settings.setConfig(t, stored)
	require.True(t, svc.refreshConfig(context.Background(), true))
	require.EqualValues(t, 3, accounts.scans.Load(), "periodic refresh adds only one account scan")
	require.Equal(t, stored.Recipient, svc.currentConfig().Recipient)
	require.False(t, svc.currentConfig().Allows("weekly_quota"))
	svc.queue <- AccountOpsEvent{AccountID: 42, Kind: "balance_low"}
	select {
	case event := <-events.recorded:
		require.Equal(t, "balance_low", event.Kind)
	case <-time.After(time.Second):
		t.Fatal("runtime did not persist the local event under refreshed policy")
	}
}

type accountOpsBlockingEvents struct {
	accountOpsRoleEvents
	entered chan struct{}
	stopped chan struct{}
}

func (r *accountOpsBlockingEvents) Record(ctx context.Context, _ AccountOpsEvent) error {
	close(r.entered)
	<-ctx.Done()
	close(r.stopped)
	return ctx.Err()
}

func TestAccountOpsStopCancelsInFlightPersistenceAndCanRepeat(t *testing.T) {
	settings := &accountOpsRoleSettings{values: make(map[string]string)}
	c := defaultAccountOpsConfig()
	c.Enabled = true
	c.Recipient = "fixture@example.test"
	settings.setConfig(t, c)
	events := &accountOpsBlockingEvents{
		accountOpsRoleEvents: accountOpsRoleEvents{refreshed: make(chan AccountOpsConfig, 1)},
		entered:              make(chan struct{}), stopped: make(chan struct{}),
	}
	svc := NewAccountOpsService(settings, events, nil)
	svc.Start()
	defer svc.Stop()
	select {
	case <-events.refreshed:
	case <-time.After(time.Second):
		t.Fatal("runtime did not load configuration")
	}
	svc.Observe(&Account{ID: 42, Platform: PlatformOpenAI}, 402, nil, []byte(`{"error":{"code":"insufficient_balance"}}`))
	select {
	case <-events.entered:
	case <-time.After(time.Second):
		t.Fatal("runtime did not persist its queued event")
	}
	done := make(chan struct{})
	go func() { svc.Stop(); svc.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown failed to cancel in-flight persistence")
	}
	select {
	case <-events.stopped:
	default:
		t.Fatal("shutdown returned before the worker exited")
	}
}
