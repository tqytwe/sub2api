package handler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSTurnSettlementClaimAndPrivateIdentity(t *testing.T) {
	var settlement openAIWSTurnSettlement
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if settlement.claim(1) {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, accepted.Load(), "duplicate AfterTurn callbacks must submit once")
	require.True(t, settlement.claim(2))
	parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "connection")
	first := settlement.context(parent, 1).Value(ctxkey.UsageBillingRequestID)
	require.NotEqual(t, "connection", first)
	require.Equal(t, first, settlement.context(parent, 1).Value(ctxkey.UsageBillingRequestID))
	require.NotEqual(t, first, settlement.context(parent, 2).Value(ctxkey.UsageBillingRequestID))
	var another openAIWSTurnSettlement
	require.NotEqual(t, first, another.context(parent, 1).Value(ctxkey.UsageBillingRequestID))
}

func TestOpenAIWSTurnSettlementRetryKeepsLogicalIdentity(t *testing.T) {
	var settlement openAIWSTurnSettlement
	var acceptedTurns, settledTurns []int
	var requestIDs []any
	hooks := &service.OpenAIWSIngressHooks{
		BeforeRequest: func(turn int, _ []byte, _ string) error {
			acceptedTurns = append(acceptedTurns, turn)
			requestIDs = append(requestIDs, settlement.context(context.Background(), turn).Value(ctxkey.UsageBillingRequestID))
			return nil
		},
		AfterTurn: func(turn int, result *service.OpenAIForwardResult, _ error) {
			if result != nil && settlement.claim(turn) {
				settledTurns = append(settledTurns, turn)
			}
		},
	}
	first := settlement.bindAttempt(hooks)
	require.NoError(t, first.BeforeRequest(1, nil, ""))
	first.AfterTurn(1, &service.OpenAIForwardResult{}, nil)
	require.NoError(t, first.BeforeRequest(2, nil, ""))
	first.AfterTurn(2, nil, errors.New("unmetered 429"))
	retry := settlement.bindAttempt(hooks)
	require.NoError(t, retry.BeforeRequest(1, nil, ""))
	retry.AfterTurn(1, &service.OpenAIForwardResult{}, nil)
	first.AfterTurn(1, &service.OpenAIForwardResult{}, nil) // Late duplicate retains its original binding.
	require.NoError(t, retry.BeforeRequest(2, nil, ""))
	retry.AfterTurn(2, &service.OpenAIForwardResult{}, nil)
	require.Equal(t, []int{1, 2, 2, 3}, acceptedTurns)
	require.Equal(t, []int{1, 2, 3}, settledTurns)
	require.Equal(t, requestIDs[1], requestIDs[2], "the same real turn retains its missing-ID billing key across retry")
	require.NotEqual(t, requestIDs[0], requestIDs[1])
	require.NotEqual(t, requestIDs[2], requestIDs[3])
}

func TestOpenAIWSTurnUsageSettlement(t *testing.T) {
	turnErr := errors.New("upstream stream interrupted")
	for _, tc := range []struct {
		name          string
		result        *service.OpenAIForwardResult
		err           error
		cyberRecorded bool
		want          bool
	}{
		{name: "nil_error_result", err: turnErr},
		{name: "nil_success_result"},
		{name: "successful_zero_usage_retains_log", result: &service.OpenAIForwardResult{}, want: true},
		{name: "failed_zero_usage_not_billed", result: &service.OpenAIForwardResult{}, err: turnErr},
		{name: "failed_input_usage", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9}}, err: turnErr, want: true},
		{name: "failed_output_usage", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 2}}, err: turnErr, want: true},
		{name: "failed_cache_write_details_without_aggregate", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{CacheCreationInputTokens: 4}}, err: turnErr},
		{name: "failed_cache_read_details_without_aggregate", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{CacheReadInputTokens: 4}}, err: turnErr},
		{name: "failed_image_retained", result: &service.OpenAIForwardResult{ImageCount: 1}, err: turnErr, want: true},
		{name: "failed_cyber_text_already_recorded", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9}}, err: turnErr, cyberRecorded: true},
		{name: "failed_cyber_image_already_recorded", result: &service.OpenAIForwardResult{ImageCount: 1}, err: turnErr, cyberRecorded: true},
		{name: "successful_cyber_retains_existing_path", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9}}, cyberRecorded: true, want: true},
		{name: "failed_negative_usage_not_billed", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: -1}}, err: turnErr},
		{name: "failed_details_without_aggregate_not_billed", result: &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputAudioTokens: 2, ImageInputTokens: 3}}, err: turnErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldRecordOpenAIWSTurnUsage(&service.Account{Platform: service.PlatformOpenAI}, tc.result, tc.err, tc.cyberRecorded))
		})
	}
}

func TestOpenAIWSTurnPartialTextDoesNotReportForwardSuccess(t *testing.T) {
	turnErr := errors.New("upstream stream interrupted")
	text := &service.OpenAIForwardResult{OpenAIWSMode: true, Usage: service.OpenAIUsage{InputTokens: 9, OutputTokens: 2}}
	// The existing scheduling helper treats an empty terminal as legacy success.
	// Partial text must therefore bypass both scheduler and quota-header updates.
	require.True(t, text.SucceededForScheduling())
	require.False(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformOpenAI}, text, turnErr))
	require.True(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformOpenAI}, text, nil))
	require.True(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformOpenAI}, &service.OpenAIForwardResult{ImageCount: 1}, turnErr), "existing image reporting is unchanged")
}

func TestOpenAIWSTurnUsageSettlementPreservesGrokScope(t *testing.T) {
	turnErr := errors.New("interrupted")
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			account := &service.Account{Platform: platform}
			text := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9, OutputTokens: 2}}
			require.Equal(t, platform == service.PlatformOpenAI, shouldRecordOpenAIWSTurnUsage(account, text, turnErr, false))
			require.True(t, shouldRecordOpenAIWSTurnUsage(account, text, nil, false))
			require.True(t, shouldRecordOpenAIWSTurnUsage(account, &service.OpenAIForwardResult{ImageCount: 1}, turnErr, false))
		})
	}
}

func TestOpenAIWSTurnLocalAdmissionDoesNotReportAccountFailure(t *testing.T) {
	err := errors.Join(errors.New("read bridge stream"), &service.OpenAITurnAdmissionError{Reason: "changed"})
	require.False(t, shouldReportOpenAIWSProxyAccountFailure(&service.Account{Platform: service.PlatformOpenAI}, err))
	require.True(t, shouldReportOpenAIWSProxyAccountFailure(&service.Account{Platform: service.PlatformGrok}, err), "compatible provider health policy stays unchanged")
	require.True(t, shouldReportOpenAIWSProxyAccountFailure(&service.Account{Platform: service.PlatformOpenAI}, errors.New("upstream failed")))
}

func TestOpenAIWSImageLocalAdmissionSkipsAccountObservation(t *testing.T) {
	image := &service.OpenAIForwardResult{ImageCount: 1}
	local := errors.Join(errors.New("read bridge stream"), &service.OpenAITurnAdmissionError{Reason: "changed"})
	require.False(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformOpenAI}, image, local))
	require.True(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformGrok}, image, local), "compatible provider image reporting remains legacy")
	require.True(t, shouldReportOpenAIWSTurnForwardResult(&service.Account{Platform: service.PlatformOpenAI}, image, errors.New("upstream failure")))
}
