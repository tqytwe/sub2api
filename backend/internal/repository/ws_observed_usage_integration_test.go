//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type wsObservedUsageFrames struct {
	frames   [][]byte
	upstream bool
}

func (c *wsObservedUsageFrames) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	if len(c.frames) > 0 {
		frame := c.frames[0]
		c.frames = c.frames[1:]
		return coderws.MessageText, frame, nil
	}
	if c.upstream {
		return coderws.MessageText, nil, io.EOF
	}
	<-ctx.Done()
	return coderws.MessageText, nil, ctx.Err()
}
func (*wsObservedUsageFrames) WriteFrame(context.Context, coderws.MessageType, []byte) error {
	return nil
}
func (*wsObservedUsageFrames) Close() error { return nil }

type wsObservedUsageBillingFailure struct {
	service.UsageBillingRepository
	fail atomic.Bool
}

func (r *wsObservedUsageBillingFailure) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if r.fail.Swap(false) {
		return nil, errors.New("synthetic settlement failure")
	}
	return r.UsageBillingRepository.Apply(ctx, cmd)
}

func TestWSObservedUsagePostgresExactlyOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "ws-usage-" + uuid.NewString() + "@example.com", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "test-" + uuid.NewString(), Name: "ws-usage"})
	account := mustCreateAccount(t, client, &service.Account{Name: "ws-usage-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI})
	logs := NewUsageLogRepository(client, integrationDB)
	billing := &wsObservedUsageBillingFailure{UsageBillingRepository: NewUsageBillingRepositoryWithLedger(client, integrationDB, service.NewBalanceLedgerService(integrationDB, nil, nil))}
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	gateway := service.NewOpenAIGatewayService(nil, logs, billing, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	id := uuid.NewString()
	upstream := &wsObservedUsageFrames{upstream: true, frames: [][]byte{
		[]byte(fmt.Sprintf(`{"type":"response.completed","response":{"id":"%s-done","usage":{"input_tokens":9,"output_tokens":4}}}`, id)),
		[]byte(fmt.Sprintf(`{"type":"response.in_progress","response":{"id":"%s-partial","usage":{"input_tokens":18,"output_tokens":8}}}`, id)),
	}}
	var complete []openaiwsv2.RelayTurnResult
	result, _ := openaiwsv2.Relay(ctx, &wsObservedUsageFrames{}, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), openaiwsv2.RelayOptions{OnTurnComplete: func(turn openaiwsv2.RelayTurnResult) { complete = append(complete, turn) }})
	require.Len(t, complete, 1)
	require.NotNil(t, result.UnfinishedTurn)
	input := func(turn openaiwsv2.RelayTurnResult) *service.OpenAIRecordUsageInput {
		return &service.OpenAIRecordUsageInput{Result: &service.OpenAIForwardResult{RequestID: turn.RequestID, Model: "gpt-5.1", OpenAIWSMode: true, Usage: service.OpenAIUsage{InputTokens: turn.Usage.InputTokens, OutputTokens: turn.Usage.OutputTokens}}, User: user, APIKey: key, Account: account, RequestPayloadHash: "synthetic-ws-turn"}
	}
	require.NoError(t, gateway.RecordUsage(ctx, input(complete[0])))
	billing.fail.Store(true)
	require.Error(t, gateway.RecordUsage(ctx, input(*result.UnfinishedTurn)))
	var partialInput, partialOutput int
	var settled bool
	var partialBilled float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT input_tokens, output_tokens, billing_settled, billed_cost FROM usage_logs WHERE api_key_id=$1 AND request_id=$2", key.ID, result.UnfinishedTurn.RequestID).Scan(&partialInput, &partialOutput, &settled, &partialBilled))
	require.Equal(t, 18, partialInput)
	require.Equal(t, 8, partialOutput)
	require.False(t, settled)
	require.Zero(t, partialBilled)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- gateway.RecordUsage(ctx, input(*result.UnfinishedTurn)) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var rows, dedup, debits int
	var inputs, outputs int
	var billed, balance, ledgerDelta float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*), SUM(input_tokens), SUM(output_tokens), SUM(billed_cost), BOOL_AND(billing_settled) FROM usage_logs WHERE user_id=$1 AND api_key_id=$2 AND account_id=$3", user.ID, key.ID, account.ID).Scan(&rows, &inputs, &outputs, &billed, &settled))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id=$1", key.ID).Scan(&dedup))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*), SUM(balance_delta) FROM balance_transactions WHERE user_id=$1 AND source_type='usage_charge'", user.ID).Scan(&debits, &ledgerDelta))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", user.ID).Scan(&balance))
	require.Equal(t, 2, rows)
	require.Equal(t, 2, dedup)
	require.Equal(t, 2, debits)
	require.Equal(t, 27, inputs)
	require.Equal(t, 12, outputs)
	require.True(t, settled)
	require.Greater(t, billed, 0.0)
	require.InDelta(t, 100-billed, balance, 1e-9)
	require.InDelta(t, -billed, ledgerDelta, 1e-9)
}
