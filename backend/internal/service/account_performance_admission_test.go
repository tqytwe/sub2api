package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountPerformancePauseEnforcedAtGatewaySlotBoundary(t *testing.T) {
	concurrency := NewConcurrencyService(nil)
	concurrency.performancePauses = performancePauseReaderStub{paused: map[int64]bool{7: true}}
	for _, limit := range []int{0, 8} {
		gateway := &GatewayService{concurrencyService: concurrency}
		result, err := gateway.tryAcquireAccountSlot(context.Background(), 7, limit)
		require.NoError(t, err)
		require.False(t, result.Acquired)
		openai := &OpenAIGatewayService{concurrencyService: concurrency}
		result, err = openai.tryAcquireAccountSlot(context.Background(), 7, limit)
		require.NoError(t, err)
		require.False(t, result.Acquired)
	}
}

type performancePluginRepo struct {
	performancePauseReaderStub
	account *Account
}

func (r performancePluginRepo) ListByPlatform(context.Context, string) ([]Account, error) {
	return []Account{*r.account}, nil
}

func (r performancePluginRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func TestAccountPerformancePauseGovernsPluginDirectoryAndCredentialResolution(t *testing.T) {
	account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "synthetic"}}
	for _, readError := range []error{nil, errors.New("pause authority unavailable")} {
		repo := performancePluginRepo{account: account, performancePauseReaderStub: performancePauseReaderStub{paused: map[int64]bool{7: true}, err: readError}}
		gateway := &OpenAIGatewayService{accountRepo: repo}
		scope := newPluginAccountScope(pluginAccountScopeEntry{Platform: PlatformOpenAI, AccountType: AccountTypeOAuth})
		infos, err := gateway.ListPluginAccounts(context.Background(), scope, "", "")
		if readError != nil {
			require.ErrorIs(t, err, readError)
			require.Nil(t, infos)
		} else {
			require.NoError(t, err)
			require.Len(t, infos, 1)
			require.False(t, infos[0].Schedulable)
		}
		identity, err := gateway.ResolvePluginOutboundIdentity(context.Background(), scope, 7)
		if readError != nil {
			require.ErrorIs(t, err, readError)
		} else {
			require.NoError(t, err)
		}
		require.Nil(t, identity, "paused account credentials must not be minted for plugin inference")
		require.True(t, account.Schedulable)
		require.False(t, account.PerformancePaused, "runtime state must not mutate repository/cached snapshots")
	}
}

func TestAccountPerformancePauseRejectsNewTurnOnAlreadyHeldConnection(t *testing.T) {
	repo := performancePauseReaderStub{paused: map[int64]bool{7: true}}
	gateway := &OpenAIGatewayService{accountRepo: repo}
	// A compatible provider normally reuses the supplied selection. A held
	// WebSocket must nevertheless consult the live hold before another turn.
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		selected := &Account{ID: 7, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
		admitted, err := gateway.AdmitOpenAITurn(context.Background(), nil, selected, "test-model")
		require.Nil(t, admitted)
		require.True(t, IsOpenAITurnAdmissionError(err))
		require.ErrorContains(t, err, "performance_paused")
		require.True(t, selected.Schedulable, "the independent hold must not rewrite manual settings")
	}
}
