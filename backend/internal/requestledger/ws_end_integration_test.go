//go:build integration

package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func wsEndTestAttempt(t *testing.T, db *sql.DB) (*Ledger, *Handle, *Attempt) {
	t.Helper()
	l := New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	a, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	return l, h, a
}
func wsEndTestWrite(t *testing.T, a *Attempt) wsEndWrite {
	t.Helper()
	s := newWSEndSnapshot(a.ID, WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit", Reason: "unexpected_eof", ReasonSource: "finish_cause", JoinedUnfinished: true}, time.Now())
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return wsEndWrite{db: a.handle.ledger.db, requestID: a.handle.ID, attemptID: a.ID, attemptNo: a.Number, encoded: string(b)}
}
func waitWSEnd(t *testing.T, a *Attempt) {
	t.Helper()
	require.Eventually(t, func() bool {
		var raw string
		return a.handle.ledger.db.QueryRow(`SELECT snapshot::text FROM gateway_request_ws_end_evidence WHERE attempt_id=$1`, a.ID).Scan(&raw) == nil && raw != "{}"
	}, 4*time.Second, 10*time.Millisecond)
}

func TestLedgerPostgresWSEndReceiptIsolationAndFirstSnapshot(t *testing.T) {
	db := ledgerPostgres(t)
	l, h, first := wsEndTestAttempt(t, db)
	ctx := WithHandle(context.Background(), h)
	second, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	first.Receipt().ObserveWSEnd(WSEndObservation{Stage: "read_upstream", StageSource: "ingress_turn_error", Reason: "unexpected_eof", ReasonSource: "finish_cause"})
	waitWSEnd(t, first)
	require.NoError(t, first.Finish(ctx, 0, context.Canceled))
	require.NoError(t, second.Finish(ctx, 200, nil))
	// The original receipt remains exact after a later attempt has become current.
	write := wsEndTestWrite(t, first)
	write.attemptID = second.ID
	write.persist()
	second.Receipt().ObserveWSEnd(WSEndObservation{TerminalEvent: "response.completed", ResultObserved: true})
	waitWSEnd(t, second)
	wsEndTestWrite(t, first).persist() // later end cannot overwrite first snapshot
	page, err := l.WSEndEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.Len(t, page.Attempts, 2)
	require.Equal(t, "unknown", page.Completeness)
	require.Equal(t, "ingress_turn_error", page.Attempts[0].Snapshot.StageSource)
	require.Equal(t, "unknown", page.Attempts[0].Snapshot.Terminal.State)
	require.Equal(t, "response.completed", page.Attempts[1].Snapshot.Terminal.Event)
	require.Equal(t, second.ID, *page.Attempts[1].AttemptID)
	_, err = l.WSEndEvidence(ctx, Viewer{UserID: 101}, h.ID, 0)
	require.Error(t, err)
	var a, b string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&a))
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=2`, h.ID).Scan(&b))
	require.Equal(t, "cancelled", a)
	require.Equal(t, "succeeded", b)
}

func TestLedgerPostgresWSEndLockPoolAndOldWriter(t *testing.T) {
	for _, mode := range []string{"table_lock", "identifier_lock", "pool", "old_writer"} {
		t.Run(mode, func(t *testing.T) {
			db := ledgerPostgres(t)
			l, h, a := wsEndTestAttempt(t, db)
			var release func()
			switch mode {
			case "identifier_lock", "table_lock":
				tx, err := db.Begin()
				require.NoError(t, err)
				defer tx.Rollback()
				if mode == "identifier_lock" {
					_, err = tx.Exec(`SELECT 1 FROM gateway_request_attempt_evidence WHERE attempt_id=$1 FOR UPDATE`, a.ID)
				} else {
					_, err = tx.Exec(`LOCK TABLE gateway_request_ws_end_evidence IN ACCESS EXCLUSIVE MODE`)
				}
				require.NoError(t, err)
				release = func() { require.NoError(t, tx.Rollback()) }
			case "pool":
				db.SetMaxOpenConns(2)
				conn, err := db.Conn(context.Background())
				require.NoError(t, err)
				defer conn.Close()
				release = func() { require.NoError(t, conn.Close()) }
			default:
				release = func() {}
			}
			start := time.Now()
			wsEndTestWrite(t, a).persist()
			require.Less(t, time.Since(start), time.Second)
			release()
			page, err := l.WSEndEvidence(context.Background(), Viewer{Admin: true}, h.ID, 0)
			require.NoError(t, err)
			if mode != "old_writer" && mode != "identifier_lock" {
				require.Equal(t, "not_recorded", page.Attempts[0].Status)
				require.Nil(t, page.Attempts[0].Snapshot)
			} else {
				require.Equal(t, "persisted_snapshot", page.Attempts[0].Status)
				// Existing identifier UPDATE and pre-282 attempt finish never touch this table.
				a.ObserveIdentifiers(context.Background(), "req_old", "resp_old")
				waitAttemptIdentifiers(t, a)
				require.NoError(t, a.Finish(context.Background(), 200, nil))
				after, err := l.WSEndEvidence(context.Background(), Viewer{Admin: true}, h.ID, 0)
				require.NoError(t, err)
				require.Equal(t, page, after)
			}
			require.Equal(t, "unknown", page.Completeness)
		})
	}
}

func TestLedgerPostgresWSEndQueueBoundedAndMissing(t *testing.T) {
	db := ledgerPostgres(t)
	l, h, victim := wsEndTestAttempt(t, db)
	ctx := WithHandle(context.Background(), h)
	attempts := make([]*Attempt, 300)
	for i := range attempts {
		a, err := BeginAttempt(ctx, 234, 234)
		require.NoError(t, err)
		attempts[i] = a
	}
	marker, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer blocker.Rollback()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_ws_end_evidence IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	start := time.Now()
	for _, a := range attempts {
		a.Receipt().ObserveWSEnd(WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit"})
	}
	victim.Receipt().ObserveWSEnd(WSEndObservation{Stage: "read_client", StageSource: "relay_exit"})
	require.Less(t, time.Since(start), 200*time.Millisecond)
	require.LessOrEqual(t, len(wsEndWrites.queue), 256)
	require.NoError(t, blocker.Rollback())
	// FIFO marker proves the worker drained all earlier jobs without retrying loss.
	select {
	case wsEndWrites.queue <- wsEndTestWrite(t, marker):
	case <-time.After(4 * time.Second):
		t.Fatal("queue failed to drain")
	}
	waitWSEnd(t, marker)
	page, err := l.WSEndEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.Equal(t, "not_recorded", page.Attempts[0].Status)
	require.Equal(t, "unknown", page.Completeness)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&count))
	require.Equal(t, 302, count)
}

func TestLedgerPostgresWSEndWriterCannotSuppressIdentifiersOrFinish(t *testing.T) {
	db := ledgerPostgres(t)
	_, h, a := wsEndTestAttempt(t, db)
	// The same controlled stall applies to the old shared-column implementation
	// and the isolated table. It must never block or drop legacy evidence.
	var isolated bool
	require.NoError(t, db.QueryRow(`SELECT to_regclass('gateway_request_ws_end_evidence') IS NOT NULL`).Scan(&isolated))
	target := "AFTER UPDATE OF ws_end_diagnostics ON gateway_request_attempt_evidence"
	if isolated {
		target = "AFTER INSERT ON gateway_request_ws_end_evidence"
	}
	_, err := db.Exec(`CREATE FUNCTION fixture_stall_ws_end() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(2); RETURN NEW; END $$;
 CREATE TRIGGER fixture_stall_ws_end ` + target + ` FOR EACH ROW EXECUTE FUNCTION fixture_stall_ws_end()`)
	require.NoError(t, err)
	a.Receipt().ObserveWSEnd(WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit"})
	require.Eventually(t, func() bool {
		var blocked bool
		return db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event='PgSleep' AND query LIKE '%ws_end%')`).Scan(&blocked) == nil && blocked
	}, 200*time.Millisecond, time.Millisecond)
	ctx := WithHandle(context.Background(), h)
	a.ObserveIdentifiers(ctx, "req_first", "resp_first")
	waitAttemptIdentifiers(t, a)
	a.ObserveIdentifiers(ctx, "req_late", "resp_late")
	waitAttemptIdentifiers(t, a)
	started := time.Now()
	require.NoError(t, a.Finish(ctx, 0, context.Canceled))
	require.Less(t, time.Since(started), 200*time.Millisecond)
	var first, response *string
	var conflict bool
	require.NoError(t, db.QueryRow(`SELECT upstream_request_id,upstream_response_id,identifier_conflict FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, a.ID).Scan(&first, &response, &conflict))
	require.NotNil(t, first, "optional WS evidence must not suppress the original identifier writer")
	require.Equal(t, "req_first", *first)
	require.Equal(t, "resp_first", *response)
	require.True(t, conflict, "later original evidence must still be observable")
}
