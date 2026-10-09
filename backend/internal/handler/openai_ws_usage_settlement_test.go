package handler

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

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
