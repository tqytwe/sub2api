//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCounterEdges(t *testing.T, db *sql.DB, client *dbent.Client) {
	t.Run("exhausted_replay_must_remain_idempotent", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		_, err := db.Exec(`UPDATE subscription_package_entitlements SET request_limit=request_used+1 WHERE id=$1`, f.sub.PackageEntitlement.ID)
		require.NoError(t, err)
		before := f.readCounters(t)
		f.send(t)
		require.NoError(t, f.settlementErrors[0])
		after := f.readCounters(t)
		require.Equal(t, before.Requests+1, after.Requests)
		// The exact admitted entitlement is now exhausted. Replay must check the
		// committed fingerprint before eligibility for a NEW charge.
		err = f.gateway.RecordUsage(f.retainedCtx, f.retainedInput)
		assert.NoError(t, err, "already-settled exhausted package callback must replay successfully")
		assertCounters(t, after, f.readCounters(t))
		f.assertDisplays(t, after)
		t.Logf("exhausted replay: error=%v before=%+v after=%+v", err, before, after)
	})
	for _, dimension := range []string{"request", "amount", "token"} {
		t.Run("exhaustion_"+dimension+"_next_request_rejected", func(t *testing.T) {
			f := newCounterFixture(t, db, client, true, 0.5, false, false)
			statement := map[string]string{
				"request": "request_limit=request_used+1",
				"amount":  "amount_limit_usd=amount_used_usd+0.0000000001",
				"token":   "token_limit=token_used+120",
			}[dimension]
			_, err := db.Exec(`UPDATE subscription_package_entitlements SET `+statement+` WHERE id=$1`, f.sub.PackageEntitlement.ID)
			require.NoError(t, err)
			before := f.readCounters(t)
			f.send(t)
			require.NoError(t, f.settlementErrors[0])
			after := f.readCounters(t)
			require.Equal(t, before.Requests+1, after.Requests)
			require.Equal(t, before.Tokens+120, after.Tokens)
			require.Greater(t, after.Amount, before.Amount)
			var status, reason string
			require.NoError(t, db.QueryRow(`SELECT status,exhausted_reason FROM subscription_package_entitlements WHERE id=$1`, f.sub.PackageEntitlement.ID).Scan(&status, &reason))
			require.Equal(t, "exhausted", status)
			require.Equal(t, dimension, reason)
			f.send(t)
			require.Error(t, f.settlementErrors[1])
			assertCounters(t, after, f.readCounters(t))
			var dedup int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID).Scan(&dedup))
			require.Equal(t, 1, dedup)
			f.assertDisplays(t, after)
		})
	}

	for _, edge := range []string{"exhausted", "expired", "wrong_user", "wrong_group"} {
		t.Run(edge+"_rolls_back", func(t *testing.T) {
			f := newCounterFixture(t, db, client, true, 0.5, false, false)
			eid := f.sub.PackageEntitlement.ID
			switch edge {
			case "exhausted":
				_, err := db.Exec(`UPDATE subscription_package_entitlements SET status='exhausted',request_used=request_limit,exhausted_reason='request' WHERE id=$1`, eid)
				require.NoError(t, err)
			case "expired":
				_, err := db.Exec(`UPDATE subscription_package_entitlements SET starts_at=NOW()-INTERVAL '2 days',expires_at=NOW()-INTERVAL '1 day' WHERE id=$1`, eid)
				require.NoError(t, err)
			case "wrong_user":
				other := newCounterFixture(t, db, client, true, 0.5, false, false)
				f.sub.PackageEntitlement = other.sub.PackageEntitlement
			case "wrong_group":
				g, err := client.Group.Create().SetName(fmt.Sprintf("wrong-group-%d", eid)).SetPlatform(service.PlatformOpenAI).Save(context.Background())
				require.NoError(t, err)
				_, err = db.Exec(`UPDATE subscription_package_entitlements SET group_id=$2 WHERE id=$1`, eid, g.ID)
				require.NoError(t, err)
			}
			before := f.readCounters(t)
			f.send(t)
			require.Len(t, f.settlementErrors, 1)
			assert.Error(t, f.settlementErrors[0], "invalid entitlement must not settle")
			assertCounters(t, before, f.readCounters(t))
			var dedup, verified int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID).Scan(&dedup))
			assert.Zero(t, dedup)
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_request_billing_links WHERE api_key_id=$1 AND settlement_verified`, f.key.ID).Scan(&verified))
			assert.Zero(t, verified)
			assert.Eventually(t, func() bool {
				var n int
				err := db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1 AND NOT billing_settled AND actual_cost=0 AND billed_cost=0`, f.key.ID).Scan(&n)
				return err == nil && n == 1
			}, time.Second*3, time.Millisecond*10, "failed usage must be uncharged and unsettled")
			t.Logf("rejected edge=%s error=%v counters_before=%+v counters_after=%+v dedup=%d verified=%d", edge, f.settlementErrors[0], before, f.readCounters(t), dedup, verified)
		})
	}
	for _, packaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("database_failure_rollback_package_%t", packaged), func(t *testing.T) {
			f := newCounterFixture(t, db, client, packaged, 0.5, true, false)
			before := f.readCounters(t)
			// Fail the subscription update AFTER the package update in the real Apply
			// transaction. The constraint targets this fixture alone, and is removed
			// before recovery. Dedup, all six counters and verification must roll back.
			constraint := fmt.Sprintf("counter_fault_%d", f.sub.ID)
			_, err := db.Exec(fmt.Sprintf(`ALTER TABLE user_subscriptions ADD CONSTRAINT %s CHECK(id <> %d OR daily_usage_usd <= %.10f) NOT VALID`, constraint, f.sub.ID, before.Daily))
			require.NoError(t, err)
			t.Cleanup(func() { _, _ = db.Exec(`ALTER TABLE user_subscriptions DROP CONSTRAINT IF EXISTS ` + constraint) })
			f.send(t)
			require.Len(t, f.settlementErrors, 1)
			require.Error(t, f.settlementErrors[0])
			var databaseError *pq.Error
			require.ErrorAs(t, f.settlementErrors[0], &databaseError)
			require.Equal(t, constraint, databaseError.Constraint)
			assertCounters(t, before, f.readCounters(t))
			var dedup, verified int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID).Scan(&dedup))
			require.Zero(t, dedup)
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_request_billing_links WHERE api_key_id=$1 AND settlement_verified`, f.key.ID).Scan(&verified))
			require.Zero(t, verified)
			require.Eventually(t, func() bool {
				var n int
				err := db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1 AND NOT billing_settled AND billed_cost=0`, f.key.ID).Scan(&n)
				return err == nil && n == 1
			}, 5*time.Second, 10*time.Millisecond)
			_, err = db.Exec(`ALTER TABLE user_subscriptions DROP CONSTRAINT ` + constraint)
			require.NoError(t, err)
			require.NoError(t, f.gateway.RecordUsage(f.retainedCtx, f.retainedInput))
			require.NoError(t, f.gateway.RecordUsage(f.retainedCtx, f.retainedInput))
			f.assertSettlement(t, before, 1, 0.5, true, false)
		})
	}
	t.Run("charged_entitlement_is_not_current_display_pointer", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		before := f.readCounters(t)
		oldID := f.sub.PackageEntitlement.ID
		_, err := db.Exec(`UPDATE subscription_package_entitlements SET request_limit=request_used+1 WHERE id=$1`, oldID)
		require.NoError(t, err)
		var renewalID int64
		require.NoError(t, db.QueryRow(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,amount_limit_usd,token_limit) VALUES($1,$2,'admin_grant',$3,NOW(),NOW()+INTERVAL '60 days',100,1000,1000000) RETURNING id`, f.user.ID, f.sub.GroupID, fmt.Sprintf("renewal-%d", oldID)).Scan(&renewalID))
		f.send(t)
		require.NoError(t, f.settlementErrors[0])
		after := f.readCounters(t)
		require.Equal(t, before.Requests+1, after.Requests)
		active, err := f.subscriptions.GetActiveSubscription(context.Background(), f.user.ID, f.sub.GroupID)
		require.NoError(t, err)
		require.Equal(t, renewalID, active.PackageEntitlement.ID)
		var charged int64
		require.NoError(t, db.QueryRow(`SELECT package_entitlement_id FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&charged))
		require.Equal(t, oldID, charged)
		f.sub.PackageEntitlement = active.PackageEntitlement
		shown := f.readCounters(t)
		require.Zero(t, shown.Requests)
		require.Zero(t, shown.Amount)
		require.Zero(t, shown.Tokens)
		f.assertDisplays(t, shown)
		t.Logf("billing_link_entitlement=%d displayed_entitlement=%d old_delta=%+v renewal=%+v", charged, renewalID, after.minus(before), shown)
	})
	t.Run("window_anchors_and_stale_reset_CAS", func(t *testing.T) { runCounterWindows(t, db, client) })
	t.Run("explicit_package_reset_only_current", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		f.send(t)
		require.NoError(t, f.settlementErrors[0])
		// A later renewal has independent already-recorded counters. Reset
		// may touch only the current entitlement, never the renewal chain.
		var laterID int64
		require.NoError(t, db.QueryRow(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,request_used,amount_limit_usd,amount_used_usd,token_limit,token_used) VALUES($1,$2,'admin_grant',$3,NOW(),NOW()+INTERVAL '60 days',100,9,1000,8.5,1000000,777) RETURNING id`, f.user.ID, f.sub.GroupID, fmt.Sprintf("reset-renewal-%d", f.sub.ID)).Scan(&laterID))
		before := f.readCounters(t)
		_, err := f.subscriptions.AdminResetQuota(context.Background(), f.sub.ID, true, true, true)
		require.NoError(t, err)
		after := f.readCounters(t)
		expected := before
		expected.Requests = 0
		expected.Amount = 0
		expected.Tokens = 0
		assertCounters(t, expected, after)
		var later counterSnapshot
		require.NoError(t, db.QueryRow(`SELECT request_used,amount_used_usd,token_used FROM subscription_package_entitlements WHERE id=$1`, laterID).Scan(&later.Requests, &later.Amount, &later.Tokens))
		assertCounters(t, counterSnapshot{Requests: 9, Amount: 8.5, Tokens: 777}, later)
		f.assertDisplays(t, after)
	})
}

func runCounterWindows(t *testing.T, db *sql.DB, client *dbent.Client) {
	f := newCounterFixture(t, db, client, false, 0.5, false, false)
	repo := repository.NewUserSubscriptionRepository(client)
	ctx := context.Background()
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	oldDay := day.Add(-24 * time.Hour).Add(16 * time.Hour)
	oldWeek := day.Add(-7 * 24 * time.Hour)
	oldMonth := day.Add(-30 * 24 * time.Hour)
	_, err := db.Exec(`UPDATE user_subscriptions SET daily_window_start=$2,weekly_window_start=$3,monthly_window_start=$4 WHERE id=$1`, f.sub.ID, oldDay, oldWeek, oldMonth)
	require.NoError(t, err)
	loaded, err := repo.GetByID(ctx, f.sub.ID)
	require.NoError(t, err)
	require.True(t, loaded.NeedsDailyResetAt(day))
	require.False(t, loaded.NeedsDailyResetAt(day.Add(-time.Nanosecond)))
	require.True(t, loaded.NeedsWeeklyResetAt(day))
	require.False(t, loaded.NeedsWeeklyResetAt(day.Add(-time.Nanosecond)))
	require.True(t, loaded.NeedsMonthlyResetAt(day))
	require.False(t, loaded.NeedsMonthlyResetAt(day.Add(-time.Nanosecond)))
	require.NoError(t, f.subscriptions.CheckAndResetWindows(ctx, loaded))
	assertCounters(t, counterSnapshot{}, f.readCounters(t))
	before := f.readCounters(t)
	f.send(t)
	require.NoError(t, f.settlementErrors[0])
	after := f.readCounters(t)
	require.Positive(t, after.Daily)
	require.NoError(t, repo.ResetDailyUsage(ctx, f.sub.ID, &oldDay, day))
	require.NoError(t, repo.ResetWeeklyUsage(ctx, f.sub.ID, &oldWeek, day))
	require.NoError(t, repo.ResetMonthlyUsage(ctx, f.sub.ID, &oldMonth, day))
	assertCounters(t, after, f.readCounters(t))
	f.assertDisplays(t, after)
	loaded, err = repo.GetByID(ctx, f.sub.ID)
	require.NoError(t, err)
	for _, anchor := range []*time.Time{loaded.DailyWindowStart, loaded.WeeklyWindowStart, loaded.MonthlyWindowStart} {
		require.NotNil(t, anchor)
		require.True(t, anchor.Equal(day))
	}
	require.NoError(t, repo.ResetUsageWindows(ctx, f.sub.ID, true, true, true, day, now))
	assertCounters(t, counterSnapshot{}, f.readCounters(t))
	loaded, err = repo.GetByID(ctx, f.sub.ID)
	require.NoError(t, err)
	require.True(t, loaded.DailyWindowStart.Equal(day))
	require.WithinDuration(t, now, *loaded.WeeklyWindowStart, time.Microsecond)
	require.WithinDuration(t, now, *loaded.MonthlyWindowStart, time.Microsecond)
	t.Logf("reset evidence: day=calendar-midnight week=7d month=30d stale_CAS_preserved=%+v initial=%+v", after, before)
}
