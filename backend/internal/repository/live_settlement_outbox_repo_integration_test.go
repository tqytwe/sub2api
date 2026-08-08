//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLiveSettlementOutboxRepositoryClaimParsesQualifiedReturningColumns(t *testing.T) {
	ctx := context.Background()
	repo := NewLiveSettlementOutboxRepository(integrationDB)
	record := liveSettlementTestRecord()
	record.CallHash = "integration-" + uuid.NewString()

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx,
			"DELETE FROM live_usage_settlement_outbox WHERE call_hash = $1", record.CallHash,
		)
	})

	require.NoError(t, repo.Enqueue(ctx, record))
	require.NoError(t, repo.Activate(ctx, record.CallHash))

	jobs, err := repo.Claim(ctx, "integration-worker", 32, 30*time.Second)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, record.CallHash, jobs[0].CallHash)
}
