//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All matrix rows run on the application's complete migration schema. Fixture
// edits below model legacy evidence only in this disposable database.
func TestSubscriptionBindingCompatibility(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	var migrations int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations))
	t.Logf("full_migrations count=%d", migrations)
	for _, archive := range []bool{false, true} {
		for _, change := range []string{"exact", "ordinary_exact", "entitlement_only_exact", "other_fingerprint", "other_subscription", "other_entitlement", "drop_subscription", "drop_entitlement", "drop_both", "legacy_null_subscription", "legacy_null_entitlement", "legacy_both_null", "both_null_exact", "no_link_with_ids", "no_link_subscription_only", "no_link_entitlement_only", "no_link_without_ids"} {
			t.Run(fmt.Sprintf("archive_%t/%s", archive, change), func(t *testing.T) {
				f := newCounterFixture(t, db, client, true, 0.5, false, false)
				f.send(t)
				require.NoError(t, f.settlementErrors[0])
				cmd := f.billing.snapshot()[0].command
				ctx := f.retainedCtx
				intent := requestledger.BillingIntent{RequestID: cmd.RequestID, APIKeyID: cmd.APIKeyID, UserID: cmd.UserID, Fingerprint: cmd.RequestFingerprint, SubscriptionID: cmd.SubscriptionID, PackageEntitlementID: cmd.PackageEntitlementID}
				if archive {
					archiveBindingProof(t, db, f.key.ID)
				}
				otherSub, otherEnt := *cmd.SubscriptionID+1000000, *cmd.PackageEntitlementID+1000000
				switch change {
				case "other_fingerprint":
					intent.Fingerprint = "different-proof"
				case "other_subscription":
					intent.SubscriptionID = &otherSub
				case "other_entitlement":
					intent.PackageEntitlementID = &otherEnt
				case "drop_subscription":
					intent.SubscriptionID = nil
				case "drop_entitlement":
					intent.PackageEntitlementID = nil
				case "drop_both":
					intent.SubscriptionID, intent.PackageEntitlementID = nil, nil
				case "legacy_null_subscription", "entitlement_only_exact":
					_, err := db.Exec(`UPDATE gateway_request_billing_links SET subscription_id=NULL WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					if change == "entitlement_only_exact" {
						intent.SubscriptionID = nil
					}
				case "legacy_null_entitlement", "ordinary_exact":
					_, err := db.Exec(`UPDATE gateway_request_billing_links SET package_entitlement_id=NULL WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					if change == "ordinary_exact" {
						intent.PackageEntitlementID = nil
					}
				case "legacy_both_null", "both_null_exact":
					_, err := db.Exec(`UPDATE gateway_request_billing_links SET subscription_id=NULL,package_entitlement_id=NULL WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					if change == "both_null_exact" {
						intent.SubscriptionID, intent.PackageEntitlementID = nil, nil
					}
				case "no_link_with_ids", "no_link_subscription_only", "no_link_entitlement_only", "no_link_without_ids":
					_, err := db.Exec(`DELETE FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					if change == "no_link_without_ids" {
						intent.SubscriptionID, intent.PackageEntitlementID = nil, nil
					}
					if change == "no_link_subscription_only" {
						intent.PackageEntitlementID = nil
					}
					if change == "no_link_entitlement_only" {
						intent.SubscriptionID = nil
					}
				}
				before := f.readCounters(t)
				proofBefore := bindingProofSnapshot(t, db, f.key.ID)
				linksBefore := bindingLinkSnapshot(t, db, f.key.ID)
				allow := change == "exact" || change == "ordinary_exact" || change == "entitlement_only_exact" || change == "both_null_exact" || change == "no_link_without_ids"
				// Verify has no insertion path: even a NULL-only intent requires a
				// pre-existing link. Diagnose it before Prepare can create one.
				tx, err := db.BeginTx(ctx, nil)
				require.NoError(t, err)
				verifyErr := requestledger.VerifyBillingTx(ctx, tx, intent, false)
				require.NoError(t, tx.Rollback())
				if allow && change != "no_link_without_ids" {
					assert.NoError(t, verifyErr)
				} else {
					assert.ErrorIs(t, verifyErr, requestledger.ErrUnavailable, "Verify must preserve frozen IDs, including NULL")
				}
				prepareErr := requestledger.PrepareBilling(ctx, intent)
				if allow {
					assert.NoError(t, prepareErr)
				} else {
					assert.ErrorIs(t, prepareErr, requestledger.ErrUnavailable, "Prepare must not infer missing historical binding")
				}
				if change != "no_link_without_ids" {
					assert.Equal(t, linksBefore, bindingLinkSnapshot(t, db, f.key.ID), "no historical link mutation")
				} else {
					var sub, ent sql.NullInt64
					var verified bool
					require.NoError(t, db.QueryRow(`SELECT subscription_id,package_entitlement_id,settlement_verified FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&sub, &ent, &verified))
					assert.False(t, sub.Valid)
					assert.False(t, ent.Valid)
					assert.False(t, verified)
				}
				assert.Equal(t, proofBefore, bindingProofSnapshot(t, db, f.key.ID), "old live/archive proof bytes and timestamps remain unchanged")
				assertCounters(t, before, f.readCounters(t))
				if change == "exact" {
					result, err := f.billing.UsageBillingRepository.Apply(ctx, &cmd)
					require.NoError(t, err)
					require.False(t, result.Applied)
					require.True(t, result.SettlementVerified)
					assertCounters(t, before, f.readCounters(t))
				}
				t.Logf("binding_compat archive=%t scenario=%s prepare_error=%v verify_error=%v counters_unchanged=true legacy_proof_unchanged=true", archive, change, prepareErr, verifyErr)
			})
		}
	}
	t.Run("no_ledger_context_unchanged", func(t *testing.T) {
		require.NoError(t, requestledger.PrepareBilling(context.Background(), requestledger.BillingIntent{}))
		require.NoError(t, requestledger.VerifyBillingTx(context.Background(), nil, requestledger.BillingIntent{}, true))
	})
	t.Run("concurrent_prepare_different_ids_freezes_one", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		ledger := requestledger.New(db)
		h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
		require.NoError(t, err)
		ctx := requestledger.WithHandle(context.Background(), h)
		require.NoError(t, requestledger.BindIdentity(ctx, f.user.ID, f.key.ID))
		t.Cleanup(func() { _ = h.Finish(ctx, "failed", 500, "synthetic") })
		a, b := f.sub.PackageEntitlement.ID, f.sub.PackageEntitlement.ID+1000000
		base := requestledger.BillingIntent{RequestID: "concurrent:" + h.ID, APIKeyID: f.key.ID, UserID: f.user.ID, Fingerprint: "legacy-unchanged-proof", SubscriptionID: &f.sub.ID}
		ready, start := sync.WaitGroup{}, make(chan struct{})
		ready.Add(2)
		errs := make(chan error, 2)
		for _, id := range []int64{a, b} {
			go func(id int64) {
				intent := base
				intent.PackageEntitlementID = &id
				ready.Done()
				<-start
				errs <- requestledger.PrepareBilling(ctx, intent)
			}(id)
		}
		ready.Wait()
		close(start)
		first, second := <-errs, <-errs
		require.NotEqual(t, first == nil, second == nil, "only one different-ID Prepare may establish the binding")
		var linked int64
		require.NoError(t, db.QueryRow(`SELECT package_entitlement_id FROM gateway_request_billing_links WHERE request_id=$1`, h.ID).Scan(&linked))
		require.Contains(t, []int64{a, b}, linked)
		base.PackageEntitlementID = &linked
		require.NoError(t, requestledger.PrepareBilling(ctx, base), "the exact winning intent remains idempotent")
	})
}

func archiveBindingProof(t *testing.T, db *sql.DB, key int64) {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(`INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) SELECT request_id,api_key_id,request_fingerprint,created_at FROM usage_billing_dedup WHERE api_key_id=$1`, key)
	require.NoError(t, err)
	_, err = tx.Exec(`DELETE FROM usage_billing_dedup WHERE api_key_id=$1`, key)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func bindingProofSnapshot(t *testing.T, db *sql.DB, key int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.QueryRow(`SELECT COALESCE(jsonb_agg(row_to_json(p) ORDER BY p.source,p.request_id),'[]'::jsonb)::text FROM (SELECT 'live' AS source,request_id,api_key_id,request_fingerprint,created_at FROM usage_billing_dedup WHERE api_key_id=$1 UNION ALL SELECT 'archive',request_id,api_key_id,request_fingerprint,created_at FROM usage_billing_dedup_archive WHERE api_key_id=$1) p`, key).Scan(&value))
	return value
}

func bindingLinkSnapshot(t *testing.T, db *sql.DB, key int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.QueryRow(`SELECT COALESCE(jsonb_agg(row_to_json(b) ORDER BY b.request_id),'[]'::jsonb)::text FROM gateway_request_billing_links b WHERE api_key_id=$1`, key).Scan(&value))
	return value
}
