//go:build vipcreditintegration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestVipCreditUsageRollbackAndRecovery(t *testing.T) {
	ctx := context.Background()
	db := creditDB(t)
	uid := creditUser(t, db, "0")
	configureCredit(t, db, uid, "10", "2", "4")
	_, err := NewAutoCreditRepository(db).Save(ctx, uid, uid, service.AutoCreditPatch{Version: 1, NotifyEmail: boolPtr(true)})
	require.NoError(t, err)
	var key int64
	require.NoError(t, db.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'staged') RETURNING id`, uid, fmt.Sprintf("sk-vip-staged-%d", uid)).Scan(&key))
	cmd := service.UsageBillingCommand{RequestID: fmt.Sprintf("vip-rollback-%d", uid), UserID: uid, APIKeyID: key, BalanceCost: 0.25, APIKeyQuotaCost: 0.25}
	repo := NewUsageBillingRepositoryWithLedger(nil, db, creditLedger(db))
	name := fmt.Sprintf("vip_credit_failure_%d", uid)
	// Key quota is written after balance and credit. A failure here must undo all effects.
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=%[2]d THEN RAISE EXCEPTION 'injected quota failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER %[1]s BEFORE UPDATE OF quota_used ON api_keys FOR EACH ROW EXECUTE FUNCTION %[1]s()`, name, key))
	require.NoError(t, err)
	cleanup := func() {
		_, e := db.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON api_keys; DROP FUNCTION IF EXISTS %s()`, name, name))
		require.NoError(t, e)
	}
	t.Cleanup(cleanup)
	_, err = repo.Apply(ctx, &cmd)
	require.ErrorContains(t, err, "injected quota failure")
	var balance string
	var count int
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, "0.00000000", balance)
	for _, table := range []string{"auto_credit_ledger", "auto_credit_cache_outbox"} {
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM `+table+` WHERE user_id=$1`, uid).Scan(&count))
		require.Zero(t, count, "money/cache receipt must rollback together")
	}
	cleanup()
	require.Zero(t, emailReceiptCount(t, db, uid), "late usage failure must roll back mail with the ledger")
	result, err := repo.Apply(ctx, &cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.AutoCreditIssued)
	require.Equal(t, 1, emailReceiptCount(t, db, uid))
	result, err = repo.Apply(ctx, &cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, uid).Scan(&balance))
	require.Equal(t, "1.75000000", balance)
	state, err := NewAutoCreditRepository(db).Get(ctx, uid, "all")
	require.NoError(t, err)
	require.Len(t, state.Entries, 1)
	require.EqualValues(t, 1, state.Entries[0].Epoch)
}
