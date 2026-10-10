package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequestLedgerContextComposesWithLogicalTurnRetryBinding(t *testing.T) {
	current := requestledger.WithHandle(context.Background(), &requestledger.Handle{ID: "first"})
	bindings := requestledger.NewTurnBindings(current)
	var settlement openAIWSTurnSettlement
	var privateIDs, billingIDs []string
	hooks := &service.OpenAIWSIngressHooks{
		BeforeRequest: func(turn int, _ []byte, _ string) error {
			bindings.Bind(turn, current)
			return nil
		},
		AfterTurn: func(turn int, _ *service.OpenAIForwardResult, _ error) {
			ledgerCtx := bindings.Context(turn, current)
			ctx := settlement.context(ledgerCtx, turn)
			privateIDs = append(privateIDs, requestledger.FromContext(ctx).ID)
			id, _ := ctx.Value(ctxkey.UsageBillingRequestID).(string)
			billingIDs = append(billingIDs, id)
		},
	}
	first := settlement.bindAttempt(hooks)
	require.NoError(t, first.BeforeRequest(1, nil, ""))
	first.AfterTurn(1, nil, nil)
	current = requestledger.WithHandle(context.Background(), &requestledger.Handle{ID: "second"})
	require.NoError(t, first.BeforeRequest(2, nil, ""))
	first.AfterTurn(2, nil, nil)
	retry := settlement.bindAttempt(hooks)
	require.NoError(t, retry.BeforeRequest(1, nil, ""))
	retry.AfterTurn(1, nil, nil)
	current = requestledger.WithHandle(context.Background(), &requestledger.Handle{ID: "third"})
	require.NoError(t, retry.BeforeRequest(2, nil, ""))
	retry.AfterTurn(2, nil, nil)
	first.AfterTurn(2, nil, nil) // Late callback must retain the second turn's identities.
	require.Equal(t, []string{"first", "second", "second", "third", "second"}, privateIDs)
	require.NotEmpty(t, billingIDs[0])
	require.NotEqual(t, billingIDs[0], billingIDs[1])
	require.Equal(t, billingIDs[1], billingIDs[2])
	require.NotEqual(t, billingIDs[2], billingIDs[3])
	require.Equal(t, billingIDs[1], billingIDs[4])
}

func TestRequestLedgerPrivateIdentitySurvivesDetachedUsage(t *testing.T) {
	h := &requestledger.Handle{ID: "fixture-private-id"}
	parent, cancel := context.WithCancel(requestledger.WithHandle(context.Background(), h))
	cancel()
	for _, submit := range []func(context.Context, service.UsageRecordTask){(&GatewayHandler{}).submitUsageRecordTask, (&OpenAIGatewayHandler{}).submitUsageRecordTask} {
		submit(parent, func(ctx context.Context) {
			require.NoError(t, ctx.Err())
			require.Same(t, h, requestledger.FromContext(ctx))
		})
	}
}
