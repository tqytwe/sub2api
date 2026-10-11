//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerOpsBatchUsesCommittedRowIDs(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@example.invalid", PasswordHash: "disabled-fixture"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "disabled-fixture-" + uuid.NewString(), Name: "fixture"})
	ledger := requestledger.New(integrationDB)
	var inputs []*service.OpsInsertErrorLogInput
	var attempts []*requestledger.Attempt
	for range 2 {
		h, err := ledger.Begin(ctx, "/v1/responses", "POST", "http")
		require.NoError(t, err)
		requestCtx := requestledger.WithHandle(ctx, h)
		require.NoError(t, requestledger.BindIdentity(requestCtx, user.ID, key.ID))
		a, err := requestledger.BeginAttempt(requestCtx, 234, 22)
		require.NoError(t, err)
		attempts = append(attempts, a)
		inputs = append(inputs, &service.OpsInsertErrorLogInput{UserID: &user.ID, APIKeyID: &key.ID, ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 502, CreatedAt: time.Now(), LedgerReceipts: []requestledger.Receipt{a.Receipt()}})
	}
	repo := NewOpsRepository(integrationDB)
	count, err := repo.BatchInsertErrorLogs(ctx, []*service.OpsInsertErrorLogInput{inputs[0], nil, inputs[1]})
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	var firstID int64
	for i, a := range attempts {
		correlation := a.Receipt().Correlation()
		var id int64
		require.NoError(t, integrationDB.QueryRow(`SELECT ops_error_log_id FROM gateway_request_ops_links WHERE request_id=$1 AND attempt_no=$2`, correlation.RequestID, correlation.AttemptNo).Scan(&id))
		if i == 0 {
			firstID = id
		} else {
			require.NotEqual(t, firstID, id)
		}
		detail, err := repo.GetErrorLogByID(ctx, id)
		require.NoError(t, err)
		require.Len(t, detail.LedgerReferences, 1)
		require.Equal(t, *correlation, detail.LedgerReferences[0].Correlation)
		require.False(t, detail.LedgerReferencesUnavailable)
	}
}
