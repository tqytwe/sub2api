//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reproduce an installation that already recorded main's 281 before receiving
// this branch's lower-numbered 280. The shared harness requires a fresh, dedicated
// loopback database and applies every production migration before this test.
func TestSubscriptionMigration280AfterIdentifier281(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	var beforeCount, emptyRows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&beforeCount))
	var prior281 time.Time
	var priorChecksum string
	require.NoError(t, db.QueryRow(`SELECT applied_at,checksum FROM schema_migrations WHERE filename='281_gateway_identifier_diagnostics.sql'`).Scan(&prior281, &priorChecksum))
	// Only rewind the empty 280 objects of this newly created fixture. Keep all
	// other full migrations, especially 281's actual schema and history, intact.
	require.NoError(t, db.QueryRow(`SELECT (SELECT COUNT(*) FROM gateway_subscription_outbox)+(SELECT COUNT(*) FROM gateway_subscription_receipts)+(SELECT COUNT(*) FROM gateway_subscription_events)`).Scan(&emptyRows))
	require.Zero(t, emptyRows)
	_, err := db.Exec(`DROP TABLE gateway_subscription_outbox,gateway_subscription_receipts,gateway_subscription_events; DROP SEQUENCE gateway_subscription_order_seq; DELETE FROM schema_migrations WHERE filename='280_subscription_inflight_admission.sql'`)
	require.NoError(t, err)
	var missing int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE filename='280_subscription_inflight_admission.sql'`).Scan(&missing))
	require.Zero(t, missing)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	require.NoError(t, repository.ApplyMigrations(context.Background(), db), "complete runner restart must remain idempotent")
	var afterCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&afterCount))
	require.Equal(t, beforeCount, afterCount)
	var after281, added280 time.Time
	var checksum281, checksum280 string
	require.NoError(t, db.QueryRow(`SELECT applied_at,checksum FROM schema_migrations WHERE filename='281_gateway_identifier_diagnostics.sql'`).Scan(&after281, &checksum281))
	require.Equal(t, prior281, after281, "existing 281 must not be reapplied")
	require.Equal(t, priorChecksum, checksum281)
	require.NoError(t, db.QueryRow(`SELECT applied_at,checksum FROM schema_migrations WHERE filename='280_subscription_inflight_admission.sql'`).Scan(&added280, &checksum280))
	raw, err := migrations.FS.ReadFile("280_subscription_inflight_admission.sql")
	require.NoError(t, err)
	want := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
	require.Equal(t, hex.EncodeToString(want[:]), checksum280)
	require.True(t, added280.After(prior281), "missing 280 must run despite already-applied 281")
	t.Logf("full_migrations count=%d lower_filename_280_applied_after_281=true checksum_280=%s checksum_281=%s", afterCount, checksum280, checksum281)
	for _, packaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("package_%t", packaged), func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, packaged)
			before := f.readCounters(t)
			status, body, err := f.request()
			require.NoError(t, err)
			require.Equal(t, 200, status, body)
			f.workers.Stop()
			calls := f.billing.snapshot()
			require.Len(t, calls, 1)
			require.NoError(t, calls[0].err)
			assertCounters(t, inflightCharge(t, f, calls[0].command), f.readCounters(t).minus(before))
			assertInflightDetail(t, f, calls[0].command, true)
			assertAdmissionUsageLink(t, f, true)
			var requestID, responseID, requestSource, responseSource, requestField, responseField string
			var sameAttempt, conflict bool
			require.Eventually(t, func() bool {
				err := db.QueryRow(`SELECT e.upstream_request_id,e.upstream_response_id,e.identifier_conflict,e.identifier_diagnostics->'request'->'first'->>'source',e.identifier_diagnostics->'response'->'first'->>'source',e.identifier_diagnostics->'request'->'first'->>'field',e.identifier_diagnostics->'response'->'first'->>'field',(e.identifier_diagnostics->>'attempt_id')=r.attempt_id::text FROM gateway_subscription_receipts r JOIN gateway_request_attempt_evidence e ON e.attempt_id=r.attempt_id AND e.request_id=r.request_id WHERE r.id=$1`, calls[0].command.SubscriptionReceiptID).Scan(&requestID, &responseID, &conflict, &requestSource, &responseSource, &requestField, &responseField, &sameAttempt)
				return err == nil && requestSource != "" && responseSource != ""
			}, 3*time.Second, 10*time.Millisecond, "actual transport identifiers must coexist with the same settlement receipt")
			assert.Equal(t, "synthetic-upstream-id", requestID)
			assert.Equal(t, "synthetic-response", responseID)
			assert.Equal(t, "http_header", requestSource)
			assert.Equal(t, "http_json", responseSource)
			assert.Equal(t, "x-request-id", requestField)
			assert.Equal(t, "id", responseField)
			assert.True(t, sameAttempt)
			assert.False(t, conflict)
			assert.EqualValues(t, 1, f.sends.Load())
		})
	}
}
