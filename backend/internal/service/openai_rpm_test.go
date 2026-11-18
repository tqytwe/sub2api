package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type openAIRPMTestCache struct {
	counts map[int64]int
	err    error
}

func (c *openAIRPMTestCache) IncrementRPM(_ context.Context, accountID int64) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	c.counts[accountID]++
	return c.counts[accountID], nil
}

func (c *openAIRPMTestCache) GetRPM(_ context.Context, accountID int64) (int, error) {
	return c.counts[accountID], c.err
}

func (c *openAIRPMTestCache) GetRPMBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = c.counts[accountID]
	}
	return result, c.err
}

func (c *openAIRPMTestCache) TryAcquireRPM(_ context.Context, accountID int64, limit int) (bool, int, time.Time, error) {
	if c.err != nil {
		return false, c.counts[accountID], time.Time{}, c.err
	}
	if c.counts[accountID] >= limit {
		return false, c.counts[accountID], time.Now().Truncate(time.Minute).Add(time.Minute), nil
	}
	c.counts[accountID]++
	return true, c.counts[accountID], time.Now().Truncate(time.Minute).Add(time.Minute), nil
}

func TestOpenAIRPMStrictAcquireScopeAndSharedShadowQuota(t *testing.T) {
	ctx := context.Background()
	parentID := int64(7)
	cache := &openAIRPMTestCache{counts: map[int64]int{}}
	svc := &OpenAIGatewayService{rpmCache: cache}

	for _, account := range []*Account{
		nil,
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"base_rpm": 1}},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 1}},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	} {
		allowed, _, err := svc.TryAcquireOpenAIOAuthRPM(ctx, account)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	parent := &Account{ID: parentID, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 1}}
	shadow := &Account{ID: 8, ParentAccountID: &parentID, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 1}}
	allowed, state, err := svc.TryAcquireOpenAIOAuthRPM(ctx, shadow)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, parentID, shadow.RPMAccountID())
	require.Equal(t, 1, state.Current)

	allowed, state, err = svc.TryAcquireOpenAIOAuthRPM(ctx, parent)
	require.ErrorIs(t, err, ErrOpenAIRPMExhausted)
	require.False(t, allowed)
	require.Equal(t, 1, state.Current)
}

func TestOpenAIRPMFailsClosedAndReservationOnlyCoversFirstSend(t *testing.T) {
	ctx := context.Background()
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 2}}
	_, _, err := (&OpenAIGatewayService{}).TryAcquireOpenAIOAuthRPM(ctx, account)
	require.ErrorIs(t, err, ErrOpenAIRPMUnavailable)

	cache := &openAIRPMTestCache{counts: map[int64]int{}}
	svc := &OpenAIGatewayService{rpmCache: cache, httpUpstream: &rpmCountingUpstream{}}
	allowed, state, err := svc.TryAcquireOpenAIOAuthRPM(ctx, account)
	require.NoError(t, err)
	require.True(t, allowed)
	reserved := WithOpenAIRPMReservation(ctx, account, state)
	for range 2 {
		req, reqErr := http.NewRequestWithContext(reserved, http.MethodPost, "https://example.invalid/v1/responses", nil)
		require.NoError(t, reqErr)
		resp, sendErr := svc.doOpenAIUpstream(req, "", account)
		require.NoError(t, sendErr)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	}
	req, reqErr := http.NewRequestWithContext(reserved, http.MethodPost, "https://example.invalid/v1/responses", nil)
	require.NoError(t, reqErr)
	_, err = svc.doOpenAIUpstream(req, "", account)
	require.ErrorIs(t, err, ErrOpenAIRPMExhausted)
	require.Equal(t, 2, cache.counts[account.ID])
	require.ErrorIs(t, svc.handleOpenAIUpstreamTransportError(reserved, nil, account, err, true), ErrOpenAIRPMExhausted)
}

func TestOpenAIRPMUnavailableCacheFailureIsDistinct(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 3}}
	svc := &OpenAIGatewayService{rpmCache: &openAIRPMTestCache{counts: map[int64]int{}, err: errors.New("redis down")}}
	allowed, _, err := svc.TryAcquireOpenAIOAuthRPM(context.Background(), account)
	require.False(t, allowed)
	require.ErrorIs(t, err, ErrOpenAIRPMUnavailable)
}

func TestOpenAIRPMSchedulabilityKeepsHeadroomSoftAndCapStrict(t *testing.T) {
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"base_rpm": 10}}
	cache := &openAIRPMTestCache{counts: map[int64]int{account.ID: 8}}
	svc := &OpenAIGatewayService{rpmCache: cache}

	allowed, state, err := svc.OpenAIRPMSchedulable(context.Background(), account, true)
	require.NoError(t, err)
	require.False(t, allowed, "movable traffic should yield at the 80% headroom mark")
	require.Equal(t, 8, state.Current)

	allowed, _, err = svc.OpenAIRPMSchedulable(context.Background(), account, false)
	require.NoError(t, err)
	require.True(t, allowed, "sticky traffic may use remaining requests under the hard cap")

	cache.counts[account.ID] = 10
	allowed, state, err = svc.OpenAIRPMSchedulable(context.Background(), account, false)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 10, state.Current)
}

func TestOpenAIRPMSchedulerYieldsMovableStickyButKeepsStickyHeadroom(t *testing.T) {
	groupID := int64(9)
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10, GroupIDs: []int64{groupID}, Extra: map[string]any{"base_rpm": 10}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10, GroupIDs: []int64{groupID}, Extra: map[string]any{"base_rpm": 10}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:rpm_session": 1}}
	rpm := &openAIRPMTestCache{counts: map[int64]int{1: 8, 2: 1}}
	svc := &OpenAIGatewayService{
		cfg:                &config.Config{},
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		rpmCache:           rpm,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selectAccount := func(movable bool) (*AccountSelectionResult, error) {
		selection, _, err := svc.SelectAccountWithSchedulerForCapability(
			context.Background(), &groupID, "", "rpm_session", "gpt-5.1", nil,
			OpenAIUpstreamTransportAny, "", false, movable, false,
		)
		if err != nil {
			return nil, err
		}
		require.NotNil(t, selection)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return selection, nil
	}

	selection, err := selectAccount(true)
	require.NoError(t, err)
	require.EqualValues(t, 2, selection.Account.ID, "movable traffic should yield to the account with more headroom")

	rpm.counts[2] = 9
	_, err = selectAccount(true)
	require.ErrorIs(t, err, ErrNoAvailableAccounts, "movable traffic must not fall back to accounts at or above 80%")

	// The movable selection above replaces its sticky binding. Use the original
	// account for this independent hard-cap case.
	cache.sessionBindings["openai:rpm_session"] = 1
	selection, err = selectAccount(false)
	require.NoError(t, err)
	require.EqualValues(t, 1, selection.Account.ID, "sticky traffic may use remaining RPM below the hard cap")
}

type rpmCountingUpstream struct{ calls int }

func (u *rpmCountingUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: http.NoBody}, nil
}

func (u *rpmCountingUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}
