package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type strictSessionTestCache struct {
	*schedulerTestGatewayCache
	claimWinner int64
}

func (c *strictSessionTestCache) CompareAndSwapSessionAccountID(_ context.Context, _ int64, swap SessionAccountBindingSwap) (bool, error) {
	if c.claimWinner > 0 {
		c.sessionBindings[swap.SessionHash] = c.claimWinner
		return false, nil
	}
	if current := c.sessionBindings[swap.SessionHash]; current != 0 && current != swap.ExpectedAccountID && current != swap.AccountID {
		return false, nil
	}
	c.sessionBindings[swap.SessionHash] = swap.AccountID
	return true, nil
}

func TestQwenStrictSessionWaitsAndRefusesAnotherAccount(t *testing.T) {
	ctx := context.Background()
	groupID := int64(7)
	owner := Account{ID: 12, Type: AccountTypeAPIKey, Platform: PlatformQwen, Status: StatusActive, Schedulable: true, Concurrency: 1,
		GroupIDs: []int64{groupID}, Credentials: map[string]any{qwenStrictSessionCredential: true, qwenSessionCacheCredential: true}}
	peer := owner
	peer.ID++
	cache := &strictSessionTestCache{schedulerTestGatewayCache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai-strict:qwen-session": owner.ID}}}
	svc := &OpenAIGatewayService{cfg: deepSeekChatFallbackTestConfig(), accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{owner, peer}}, cache: cache,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{owner.ID: false, peer.ID: true}})}
	scheduler := newDefaultOpenAIAccountScheduler(svc, nil)
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, SessionHash: "qwen-session", Platform: PlatformQwen, RequestedModel: DefaultQwenTestModel, RequiredTransport: OpenAIUpstreamTransportAny}
	selection, _, err := scheduler.Select(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, owner.ID, selection.Account.ID)
	req.ExcludedIDs = map[int64]struct{}{owner.ID: {}}
	selection, _, err = scheduler.Select(ctx, req)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)
	delete(req.ExcludedIDs, owner.ID)
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, groupID, "resp_qwen", owner.ID, time.Minute))
	req.PreviousResponseID = "resp_qwen"
	req.SessionHash = ""
	selection, _, err = scheduler.Select(ctx, req)
	require.NoError(t, err)
	require.Equal(t, owner.ID, selection.Account.ID)
}

func TestQwenConcurrentFirstTurnDoesNotSendToLosingAccount(t *testing.T) {
	ctx := context.Background()
	groupID := int64(15)
	account := Account{ID: 17, Type: AccountTypeAPIKey, Platform: PlatformQwen, Status: StatusActive, Schedulable: true, Concurrency: 1,
		GroupIDs: []int64{groupID}}
	cache := &strictSessionTestCache{schedulerTestGatewayCache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{}}, claimWinner: 18}
	var released []int64
	svc := &OpenAIGatewayService{cfg: deepSeekChatFallbackTestConfig(), accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: cache,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{account.ID: true}, releasedIDs: &released})}
	scheduler := newDefaultOpenAIAccountScheduler(svc, nil)
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, SessionHash: "race", Platform: PlatformQwen, RequestedModel: DefaultQwenTestModel, RequiredTransport: OpenAIUpstreamTransportAny}
	selection, _, err := scheduler.Select(ctx, req)
	require.ErrorContains(t, err, "binding changed during selection")
	require.Nil(t, selection)
	require.Contains(t, released, account.ID)
	require.Equal(t, int64(18), cache.sessionBindings["openai-strict:race"])
}
