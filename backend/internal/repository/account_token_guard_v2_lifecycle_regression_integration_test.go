//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountTokenGuardV2LifecycleRegressionExpiredUnreplacedClaimPreservesHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	guard, _, accountID, _ := newBudgetFixture(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, err := integrationDB.ExecContext(ctx, `UPDATE account_token_guard_v2_accounts
		SET probe_state='auth', probe_detail='previous credential rejection', fail_streak=7,
			last_probe_at=$2, last_reauth_at=$3, next_probe_at=$4,
			cooldown_until=$5, blocked_reason='previous automation state',
			lease_until=NOW()-INTERVAL '1 second'
		WHERE account_id=$1`, accountID, at.Add(-time.Hour), at.Add(-30*time.Minute),
		at.Add(-time.Minute), at.Add(time.Minute))
	require.NoError(t, err)
	before, err := guard.GetAccount(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, before)
	require.NotEmpty(t, before.LeaseOwner)
	require.NotNil(t, before.LeaseUntil)

	// No replacement claim is acquired: expiry alone must fence the old owner.
	err = guard.CompleteProbe(ctx, accountID, before.LeaseOwner, service.AccountTokenGuardV2ProbeCompletion{
		ProbeState: service.AccountTokenGuardV2ProbeOK, ProbeDetail: "late successful inspection",
		LastProbeAt: at, NextProbeAt: at.Add(time.Hour),
	})

	require.ErrorIs(t, err, sql.ErrNoRows, "an expired claim cannot publish even before another owner replaces it")
	after, err := guard.GetAccount(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, before, after, "rejected stale completion must leave the entire persisted record unchanged")
}
