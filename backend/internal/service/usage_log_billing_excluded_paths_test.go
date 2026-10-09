//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRecoveryPreservesSimpleRateLimitErrorSnapshots(t *testing.T) {
	for _, gateway := range []string{"openai", "generic"} {
		for _, conflict := range []bool{false, true} {
			name := gateway + "/failure"
			billingErr := errors.New("injected simple-mode failure")
			if conflict {
				name = gateway + "/conflict"
				billingErr = ErrUsageBillingRequestConflict
			}
			t.Run(name, func(t *testing.T) {
				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				billing := &openAIRecordUsageBillingRepoStub{err: billingErr}
				userRepo, subRepo := &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}
				key := &APIKey{ID: 100, UserID: 200, RateLimit5h: 10, Group: &Group{
					RateMultiplier: 1, BillingSurchargeOverrideEnabled: true, BillingSurchargeEnabled: true,
					BillingSurchargeMode: BillingSurchargeModePercentOnChargedCost, BillingSurchargeValue: 0.1,
				}}
				user, account := &User{ID: 200}, &Account{ID: 300, Type: AccountTypeAPIKey}
				var err error
				if gateway == "openai" {
					svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, userRepo, subRepo, nil)
					svc.cfg.RunMode, svc.cfg.SimpleModeKeyRateLimitEnabled = config.RunModeSimple, true
					err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: "simple-failure", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 800, OutputTokens: 400}},
						APIKey: key, User: user, Account: account,
					})
				} else {
					svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, userRepo, subRepo)
					svc.cfg.RunMode, svc.cfg.SimpleModeKeyRateLimitEnabled = config.RunModeSimple, true
					err = svc.RecordUsage(context.Background(), &RecordUsageInput{
						Result: &ForwardResult{RequestID: "simple-failure", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 800, OutputTokens: 400}},
						APIKey: key, User: user, Account: account,
					})
				}
				require.ErrorIs(t, err, billingErr)
				require.Equal(t, 1, logs.calls, "excluded simple mode retains its previous failure log, including conflicts")
				require.NotNil(t, logs.lastLog)
				require.Zero(t, logs.lastLog.ActualCost)
				require.Greater(t, billing.lastCmd.BilledCost, 0.0)
				require.Greater(t, billing.lastCmd.BillingSurchargeCost, 0.0)
				require.Equal(t, billing.lastCmd.BilledCost, logs.lastLog.BilledCost, "preserve prior excluded-path snapshot semantics")
				require.Equal(t, billing.lastCmd.BillingSurchargeCost, logs.lastLog.BillingSurchargeCost)
				require.Empty(t, logs.lastLog.BillingRequestFingerprint)
				require.False(t, logs.lastLog.BillingSettled)
				require.Zero(t, billing.lastCmd.BalanceCost)
				require.Zero(t, billing.lastCmd.AccountQuotaCost)
				require.Greater(t, billing.lastCmd.APIKeyRateLimitCost, 0.0)
			})
		}
	}
}
