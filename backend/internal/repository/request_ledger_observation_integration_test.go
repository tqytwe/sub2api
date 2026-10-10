//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerObservationFailureKeepsOriginalSettlement(t *testing.T) {
	for _, platform := range []string{"openai", "anthropic"} {
		t.Run(platform, func(t *testing.T) {
			ctx := context.Background()
			client := testEntClient(t)
			user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ledger-observation-%s@example.invalid", uuid.NewString()), PasswordHash: "disabled-fixture", Balance: 100})
			key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "disabled-fixture-" + uuid.NewString(), Name: "fixture"})
			account := mustCreateAccount(t, client, &service.Account{Name: "ledger-observation-" + uuid.NewString(), Platform: platform, Type: service.AccountTypeAPIKey})
			l := requestledger.New(integrationDB)
			h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx = requestledger.WithHandle(ctx, h)
			require.NoError(t, requestledger.BindIdentity(ctx, user.ID, key.ID))
			attempt, err := requestledger.BeginAttempt(ctx, account.ID, account.ID)
			require.NoError(t, err)
			require.NoError(t, attempt.Finish(ctx, 200, nil))
			// Only the audit attempt row is locked. Original wallet/usage/dedup tables
			// remain writable; the append must time out without swallowing settlement.
			blocker, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback() }()
			_, err = blocker.Exec(`SELECT 1 FROM gateway_request_attempts WHERE request_id=$1 FOR UPDATE`, h.ID)
			require.NoError(t, err)
			billing := NewUsageBillingRepositoryWithLedger(client, integrationDB, service.NewBalanceLedgerService(integrationDB, nil, nil))
			usage := NewUsageLogRepository(client, integrationDB)
			cfg := &config.Config{}
			cfg.Default.RateMultiplier = 1
			rid := uuid.NewString()
			var record func() error
			if platform == "openai" {
				s := service.NewOpenAIGatewayService(nil, usage, billing, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
				record = func() error {
					return s.RecordUsage(ctx, &service.OpenAIRecordUsageInput{Result: &service.OpenAIForwardResult{RequestID: rid, Model: "gpt-5.1", Usage: service.OpenAIUsage{InputTokens: 1000, OutputTokens: 300}, Duration: time.Second}, User: user, APIKey: key, Account: account})
				}
			} else {
				s := service.NewGatewayService(nil, nil, usage, billing, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				record = func() error {
					return s.RecordUsage(ctx, &service.RecordUsageInput{Result: &service.ForwardResult{RequestID: rid, Model: "claude-sonnet-4-5", Usage: service.ClaudeUsage{InputTokens: 1000, OutputTokens: 300}, Duration: time.Second}, User: user, APIKey: key, Account: account})
				}
			}
			require.NoError(t, record(), "audit append failure must not bypass the existing billing and usage path")
			require.NoError(t, blocker.Rollback())
			var count int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, rid, key.ID).Scan(&count))
			require.Equal(t, 1, count)
			var cost, balance float64
			var settled bool
			require.Eventually(t, func() bool {
				return integrationDB.QueryRow(`SELECT billed_cost,billing_settled FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, rid, key.ID).Scan(&cost, &settled) == nil
			}, 5*time.Second, 20*time.Millisecond)
			require.True(t, settled)
			require.Positive(t, cost)
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
			require.InDelta(t, 100-cost, balance, 0.000001)
			require.NoError(t, record())
			require.NoError(t, l.Recover(ctx))
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			var again float64
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&again))
			require.InDelta(t, balance, again, 0.000001)
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM balance_transactions WHERE user_id=$1 AND source_type='usage_charge' AND source_id=$2`, user.ID, rid).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestRequestLedgerPendingUsageSnapshotNeverShowsSettledZero(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "ledger-amount-" + uuid.NewString() + "@example.invalid", PasswordHash: "disabled-fixture", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "disabled-fixture-" + uuid.NewString(), Name: "fixture"})
	account := mustCreateAccount(t, client, &service.Account{Name: "ledger-amount-" + uuid.NewString(), Platform: "openai", Type: service.AccountTypeAPIKey})
	ledger := requestledger.New(integrationDB)
	h, err := ledger.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, h)
	require.NoError(t, requestledger.BindIdentity(ctx, user.ID, key.ID))
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, AccountType: account.Type, Model: "gpt-5.1", InputTokens: 7, OutputTokens: 3, BilledCost: 1.25, BalanceCost: 1.25}
	cmd.Normalize()
	logs := NewUsageLogRepository(client, integrationDB)
	stale := &service.UsageLog{RequestID: cmd.RequestID, UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, Model: cmd.Model, InputTokens: 7, OutputTokens: 3, BillingRequestFingerprint: cmd.RequestFingerprint, BillingSettled: false}
	_, err = logs.Create(ctx, stale)
	require.NoError(t, err)
	repo := NewUsageBillingRepositoryWithLedger(client, integrationDB, service.NewBalanceLedgerService(integrationDB, nil, nil))
	proof, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, proof.SettlementVerified)
	_, err = integrationDB.Exec(`ALTER TABLE usage_logs ADD CONSTRAINT synthetic_promotion_fault CHECK (NOT billing_settled) NOT VALID`)
	require.NoError(t, err)
	defer func() {
		_, _ = integrationDB.Exec(`ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS synthetic_promotion_fault`)
	}()
	promoted := *stale
	promoted.BillingSettled = proof.SettlementVerified
	promoted.BilledCost = 1.25
	promoted.ActualCost = 1.25
	_, err = logs.Create(ctx, &promoted)
	require.Error(t, err, "fault must reject the actual usage promotion")
	refs, err := ledger.BillingReferences(ctx, requestledger.Viewer{UserID: user.ID}, h.ID)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.True(t, refs[0].Settled)
	require.Nil(t, refs[0].BilledCost, "a failed zero snapshot is not a verified zero debit")
	linked, err := ledger.Usage(ctx, requestledger.Viewer{UserID: user.ID}, h.ID, stale.ID)
	require.NoError(t, err)
	require.Nil(t, linked.BilledCost)
	var balance float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, .000001)
	_, err = integrationDB.Exec(`ALTER TABLE usage_logs DROP CONSTRAINT synthetic_promotion_fault`)
	require.NoError(t, err)
	_, err = logs.Create(ctx, &promoted)
	require.NoError(t, err)
	linked, err = ledger.Usage(ctx, requestledger.Viewer{UserID: user.ID}, h.ID, stale.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.BilledCost)
	require.InDelta(t, 1.25, *linked.BilledCost, .000001)
	proof, err = repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, proof.Applied)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, .000001)
}
