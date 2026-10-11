//go:build integration && billing_acceptance

package billingacceptance

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
)

// This regression deliberately changes an INTERNAL retry input after a real
// failed settlement. It does not claim an external caller can choose an
// entitlement, or that any production request was charged twice.
func TestSubscriptionEntitlementBindingReproduction(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	var migrations int
	var last string
	require.NoError(t, db.QueryRow(`SELECT COUNT(*),MAX(filename) FROM schema_migrations`).Scan(&migrations, &last))
	t.Logf("full_migrations count=%d last=%s", migrations, last)
	for _, scenario := range []struct {
		name    string
		changed bool
	}{
		{"unchanged_entitlement_retry_recovers", false},
		{"retry_must_keep_prepared_entitlement", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCounterFixture(t, db, client, true, 0.5, false, false)
			a := f.sub.PackageEntitlement.ID
			var b int64
			require.NoError(t, db.QueryRow(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,amount_limit_usd,token_limit) VALUES($1,$2,'admin_grant',$3,NOW()-INTERVAL '1 hour',NOW()+INTERVAL '60 days',100,1000,1000000) RETURNING id`, f.user.ID, f.sub.GroupID, fmt.Sprintf("binding-B-%d", a)).Scan(&b))
			before := f.readCounters(t)
			constraint := fmt.Sprintf("binding_fault_%d", f.sub.ID)
			_, err := db.Exec(fmt.Sprintf(`ALTER TABLE user_subscriptions ADD CONSTRAINT %s CHECK(id <> %d OR daily_usage_usd <= %.10f) NOT VALID`, constraint, f.sub.ID, before.Daily))
			require.NoError(t, err)
			t.Cleanup(func() { _, _ = db.Exec(`ALTER TABLE user_subscriptions DROP CONSTRAINT IF EXISTS ` + constraint) })
			f.send(t) // HTTP 200 means the synthetic upstream forwarded successfully.
			require.Len(t, f.settlementErrors, 1)
			var fault *pq.Error
			require.ErrorAs(t, f.settlementErrors[0], &fault)
			require.Equal(t, constraint, fault.Constraint)
			assertCounters(t, before, f.readCounters(t))
			assertCounterProofCounts(t, f, 0, 0)
			var linked int64
			var verified bool
			require.NoError(t, db.QueryRow(`SELECT package_entitlement_id,settlement_verified FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&linked, &verified))
			require.Equal(t, a, linked, "Prepare committed A independently of failed Apply")
			require.False(t, verified)
			_, err = db.Exec(`ALTER TABLE user_subscriptions DROP CONSTRAINT ` + constraint)
			require.NoError(t, err)

			retry := *f.retainedInput
			result := *retry.Result
			retry.Result = &result
			sub := *retry.Subscription
			entitlement := *sub.PackageEntitlement
			retry.Subscription = &sub
			sub.PackageEntitlement = &entitlement
			if scenario.changed {
				entitlement.ID = b
			}
			err = f.gateway.RecordUsage(f.retainedCtx, &retry)
			calls := f.billing.snapshot()
			var bRequests, bTokens, dedup, settledUsage int64
			var bAmount float64
			require.NoError(t, db.QueryRow(`SELECT request_used,amount_used_usd,token_used FROM subscription_package_entitlements WHERE id=$1`, b).Scan(&bRequests, &bAmount, &bTokens))
			require.NoError(t, db.QueryRow(`SELECT package_entitlement_id,settlement_verified FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&linked, &verified))
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID).Scan(&dedup))
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1 AND billing_settled`, f.key.ID).Scan(&settledUsage))
			t.Logf("internal_retry changed=%t upstream_sends=%d prepared_A=%d retry_entitlement=%d link_entitlement=%d verified=%t dedup=%d settled_usage=%d B_requests=%d B_amount=%.10f B_tokens=%d A_delta=%+v error=%v", scenario.changed, f.sends.Load(), a, entitlement.ID, linked, verified, dedup, settledUsage, bRequests, bAmount, bTokens, f.readCounters(t).minus(before), err)
			if len(calls) == 2 {
				t.Logf("legacy_fingerprint_same=%t retry_applied=%t", calls[0].command.RequestFingerprint == calls[1].command.RequestFingerprint, calls[1].result != nil && calls[1].result.Applied)
			}
			if scenario.changed {
				assert.Error(t, err, "a retry must not substitute the entitlement frozen by Prepare")
				assertCounters(t, before, f.readCounters(t))
				assert.Zero(t, bRequests, "B must remain uncharged when link is bound to A")
				assert.Zero(t, bAmount)
				assert.Zero(t, bTokens)
				assertCounterProofCounts(t, f, 0, 0)
			} else {
				require.NoError(t, err)
				require.Len(t, calls, 2)
				require.True(t, calls[1].result.Applied)
				require.True(t, calls[1].result.SettlementVerified)
				assertCounterProofCounts(t, f, 1, 1)
				assert.Zero(t, bRequests)
				require.NoError(t, f.gateway.RecordUsage(f.retainedCtx, &retry))
				f.assertSettlement(t, before, 1, 0.5, false, false)
			}
		})
	}

	for _, archive := range []bool{false, true} {
		t.Run(fmt.Sprintf("direct_verify_must_reject_changed_entitlement_archive_%t", archive), func(t *testing.T) {
			f := newCounterFixture(t, db, client, true, 0.5, false, false)
			before := f.readCounters(t)
			f.send(t)
			require.Len(t, f.settlementErrors, 1)
			require.NoError(t, f.settlementErrors[0])
			f.assertSettlement(t, before, 1, 0.5, false, false)
			settled := f.readCounters(t)
			cmd := f.billing.snapshot()[0].command
			a := *cmd.PackageEntitlementID
			var b int64
			require.NoError(t, db.QueryRow(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,amount_limit_usd,token_limit) VALUES($1,$2,'admin_grant',$3,NOW()-INTERVAL '1 hour',NOW()+INTERVAL '60 days',100,1000,1000000) RETURNING id`, f.user.ID, f.sub.GroupID, fmt.Sprintf("verify-B-%d", a)).Scan(&b))
			if archive {
				// Move only this disposable fixture's genuine dedup proof. No
				// fingerprint, amount, production row, or historical row is edited.
				tx, err := db.BeginTx(f.retainedCtx, nil)
				require.NoError(t, err)
				defer func() { _ = tx.Rollback() }()
				_, err = tx.Exec(`INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) SELECT request_id,api_key_id,request_fingerprint,created_at FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID)
				require.NoError(t, err)
				_, err = tx.Exec(`DELETE FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID)
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
			}
			intent := requestledger.BillingIntent{RequestID: cmd.RequestID, APIKeyID: cmd.APIKeyID, UserID: cmd.UserID, Fingerprint: cmd.RequestFingerprint, SubscriptionID: cmd.SubscriptionID, PackageEntitlementID: cmd.PackageEntitlementID}
			// Directly exercise the verifier, without Apply or Prepare. The
			// exact-A positive control uses the real committed billing proof.
			tx, err := db.BeginTx(f.retainedCtx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			require.NoError(t, requestledger.VerifyBillingTx(f.retainedCtx, tx, intent, false))
			intent.PackageEntitlementID = &b
			err = requestledger.VerifyBillingTx(f.retainedCtx, tx, intent, false)
			t.Logf("direct_verify archive=%t exact_A_control=accepted prepared_A=%d changed_B=%d changed_B_error=%v fingerprint_unchanged=true applied=false", archive, a, b, err)
			assert.Error(t, err, "Verify must reject a different entitlement despite matching live/archive fingerprint proof")
			require.NoError(t, tx.Rollback())
			assertCounters(t, settled, f.readCounters(t))
			var linked, bRequests, bTokens int64
			var bAmount float64
			require.NoError(t, db.QueryRow(`SELECT package_entitlement_id FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&linked))
			require.Equal(t, a, linked)
			require.NoError(t, db.QueryRow(`SELECT request_used,amount_used_usd,token_used FROM subscription_package_entitlements WHERE id=$1`, b).Scan(&bRequests, &bAmount, &bTokens))
			assert.Zero(t, bRequests)
			assert.Zero(t, bAmount)
			assert.Zero(t, bTokens)
			assertCounterProofCounts(t, f, 1, 1)
		})
	}
}
