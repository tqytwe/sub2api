package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// These stateful fakes model the current financial dedup and insert-only log
// contracts; recovery must not solve a stale log by applying another debit.
type recoveryBillingRepo struct {
	UsageBillingRepository
	failNext    bool
	fingerprint string
	debits      int
	lastCmd     *UsageBillingCommand
}

func (r *recoveryBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	cmd.Normalize()
	r.lastCmd = cmd
	if r.failNext {
		r.failNext = false
		return nil, errors.New("injected billing rollback")
	}
	if r.fingerprint != "" {
		if r.fingerprint != cmd.RequestFingerprint {
			return nil, ErrUsageBillingRequestConflict
		}
		return &UsageBillingApplyResult{Applied: false, SettlementVerified: true, SettlementFingerprint: cmd.RequestFingerprint}, nil
	}
	r.fingerprint = cmd.RequestFingerprint
	r.debits++
	return &UsageBillingApplyResult{Applied: true, SettlementVerified: true, SettlementFingerprint: cmd.RequestFingerprint}, nil
}

type recoveryUsageLogRepo struct {
	UsageLogRepository
	row        *UsageLog
	failWrites int
}

func (r *recoveryUsageLogRepo) Create(_ context.Context, log *UsageLog) (bool, error) {
	if r.failWrites > 0 {
		r.failWrites--
		return false, errors.New("injected log write failure")
	}
	if r.row != nil {
		if r.row.BillingRequestFingerprint != "" && r.row.BillingRequestFingerprint == log.BillingRequestFingerprint && !r.row.BillingSettled && log.BillingSettled {
			r.row.ActualCost = log.ActualCost
			r.row.BilledCost = log.BilledCost
			r.row.BillingSurchargeCost = log.BillingSurchargeCost
			r.row.BillingSurchargeMode = log.BillingSurchargeMode
			r.row.BillingSurchargeValue = log.BillingSurchargeValue
			r.row.BillingSettled = true
		}
		return false, nil
	}
	snapshot := *log
	r.row = &snapshot
	return true, nil
}

func newRecoveryUsageFixture(logs *recoveryUsageLogRepo, billing *recoveryBillingRepo) (*OpenAIGatewayService, *OpenAIRecordUsageInput) {
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	accountRate := 1.7
	return svc, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "recovery-request", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 800, OutputTokens: 400}},
		APIKey: &APIKey{ID: 100, UserID: 200, Group: &Group{
			RateMultiplier: 1, BillingSurchargeOverrideEnabled: true, BillingSurchargeEnabled: true,
			BillingSurchargeMode: BillingSurchargeModePercentOnChargedCost, BillingSurchargeValue: 0.1,
		}},
		User: &User{ID: 200}, Account: &Account{ID: 300, Type: AccountTypeAPIKey, RateMultiplier: &accountRate},
		RequestPayloadHash: "original-payload",
	}
}

func TestUsageLogRecoveryFailureThenSuccessThenDuplicate(t *testing.T) {
	logs := &recoveryUsageLogRepo{}
	billing := &recoveryBillingRepo{failNext: true}
	svc, input := newRecoveryUsageFixture(logs, billing)
	require.Error(t, svc.RecordUsage(context.Background(), input))
	require.NotNil(t, logs.row)
	require.Zero(t, logs.row.ActualCost)
	require.Zero(t, logs.row.BilledCost)
	require.Zero(t, logs.row.BillingSurchargeCost)
	require.NotEmpty(t, logs.row.BillingRequestFingerprint)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 1, billing.debits, "recovery must not duplicate the financial transaction")
	require.Greater(t, billing.lastCmd.ActualCost, 0.0)
	require.Equal(t, billing.lastCmd.ActualCost, logs.row.ActualCost, "verified settlement must repair the original failed log")
	require.Equal(t, billing.lastCmd.BilledCost, logs.row.BilledCost)
	require.Greater(t, logs.row.BillingSurchargeCost, 0.0)
	require.Equal(t, 1.7, *logs.row.AccountRateMultiplier)
	billing.failNext = true
	require.Error(t, svc.RecordUsage(context.Background(), input))
	require.Greater(t, logs.row.ActualCost, 0.0, "stale failure cannot downgrade settlement")
	require.True(t, logs.row.BillingSettled)
}

type recoveryUnverifiedRepo struct {
	*recoveryBillingRepo
	wrongFingerprint bool
}

func (r *recoveryUnverifiedRepo) Apply(ctx context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	result, err := r.recoveryBillingRepo.Apply(ctx, cmd)
	if result != nil {
		result.SettlementVerified = r.wrongFingerprint
		result.SettlementFingerprint = "unverified"
	}
	return result, err
}

func TestUsageLogRecoveryRequiresExactVerifiedProof(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		logs, billing := &recoveryUsageLogRepo{}, &recoveryBillingRepo{failNext: true}
		svc, input := newRecoveryUsageFixture(logs, billing)
		require.Error(t, svc.RecordUsage(context.Background(), input))
		svc.usageBillingRepo = &recoveryUnverifiedRepo{recoveryBillingRepo: billing, wrongFingerprint: mismatch}
		require.NoError(t, svc.RecordUsage(context.Background(), input))
		require.Zero(t, logs.row.ActualCost)
		require.False(t, logs.row.BillingSettled)
	}
}

func TestUsageLogRecoveryMetadataIsNotSerialized(t *testing.T) {
	payload, err := json.Marshal(&UsageLog{BillingRequestFingerprint: "internal-fingerprint", BillingSettled: true})
	require.NoError(t, err)
	require.NotContains(t, string(payload), "internal-fingerprint")
	require.NotContains(t, string(payload), "BillingSettled")
}

func TestUsageLogRecoveryZeroCostSettlement(t *testing.T) {
	logs, billing := &recoveryUsageLogRepo{}, &recoveryBillingRepo{failNext: true}
	svc, input := newRecoveryUsageFixture(logs, billing)
	input.Result.Usage = OpenAIUsage{}
	require.Error(t, svc.RecordUsage(context.Background(), input))
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.True(t, logs.row.BillingSettled, "a committed zero-cost command is still a verified settlement")
	require.Zero(t, logs.row.ActualCost)
}

func TestUsageLogRecoveryPreservesExcludedPaths(t *testing.T) {
	for _, mode := range []string{"simple", "legacy", "image_studio"} {
		t.Run(mode, func(t *testing.T) {
			logs, billing := &recoveryUsageLogRepo{}, &recoveryBillingRepo{}
			svc, input := newRecoveryUsageFixture(logs, billing)
			ctx := context.Background()
			switch mode {
			case "simple":
				svc.cfg.RunMode = config.RunModeSimple
			case "legacy":
				svc.usageBillingRepo = nil
			case "image_studio":
				ctx = WithImageStudioManagedBilling(ctx)
			}
			require.NoError(t, svc.RecordUsage(ctx, input))
			require.NotNil(t, logs.row)
			require.Empty(t, logs.row.BillingRequestFingerprint)
			require.False(t, logs.row.BillingSettled)
		})
	}
}

func TestUsageLogRecoveryDebitThenLogFailureThenRetry(t *testing.T) {
	logs := &recoveryUsageLogRepo{failWrites: 1}
	billing := &recoveryBillingRepo{}
	svc, input := newRecoveryUsageFixture(logs, billing)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Nil(t, logs.row)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 1, billing.debits)
	require.NotNil(t, logs.row)
	require.Equal(t, billing.lastCmd.ActualCost, logs.row.ActualCost)
}

func TestUsageLogRecoveryConflictCannotPoisonMissingLog(t *testing.T) {
	logs := &recoveryUsageLogRepo{failWrites: 1}
	billing := &recoveryBillingRepo{}
	svc, input := newRecoveryUsageFixture(logs, billing)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Nil(t, logs.row)
	input.RequestPayloadHash = "different-payload-same-client-id"
	require.ErrorIs(t, svc.RecordUsage(context.Background(), input), ErrUsageBillingRequestConflict)
	require.Nil(t, logs.row, "a rejected fingerprint must not occupy the settled request's missing log")
	require.Equal(t, 1, billing.debits)
}
