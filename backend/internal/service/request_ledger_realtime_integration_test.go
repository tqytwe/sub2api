//go:build integration

package service

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequestLedgerRealtimeCapacityRejectionStillPersistsTurn(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	for _, observed := range []bool{false, true} {
		t.Run(fmt.Sprintf("observed_%t", observed), func(t *testing.T) {
			ctx := context.Background()
			session, err := l.Begin(ctx, "/v1/realtime", "GET", "ws_session")
			require.NoError(t, err)
			ctx = requestledger.WithHandle(ctx, session)
			require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
			audit := newRealtimeLedgerAudit(ctx, &Account{ID: 234})
			// Occupy only the bounded in-memory registry; no synthetic turn is sent.
			for i := 0; i < 128; i++ {
				audit.active[fmt.Sprint(i)] = &realtimeLedgerTurn{}
			}
			if observed {
				err = audit.Observe([]byte(`{"type":"response.created","response":{"id":"overflow"}}`))
			} else {
				err = audit.BeforeWrite([]byte(`{"type":"response.create"}`))
			}
			require.ErrorIs(t, err, requestledger.ErrUnavailable)
			var state, usage string
			var attempts, turn int
			var output bool
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,attempt_count,turn_no,output_observed FROM gateway_requests WHERE parent_id=$1 AND user_id=101 AND api_key_id=201`, session.ID).Scan(&state, &usage, &attempts, &turn, &output))
			require.Equal(t, "failed", state)
			require.Equal(t, 1, turn)
			if observed {
				require.Equal(t, 1, attempts)
				require.Equal(t, "usage_unknown", usage)
				require.True(t, output)
			} else {
				require.Zero(t, attempts)
				require.Equal(t, "not_applicable", usage)
				require.False(t, output)
			}
			require.Len(t, audit.active, 128, "capacity must remain bounded")
		})
	}
}

func TestRequestLedgerRealtimeExplicitAndAutomaticTurns(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx := context.Background()
	session, err := l.Begin(ctx, "/v1/realtime", "GET", "ws_session")
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	audit := newRealtimeLedgerAudit(ctx, &Account{ID: 234})
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, audit.Observe([]byte(`{"type":"response.created","response":{"id":"first"}}`)))
	require.NoError(t, audit.Observe([]byte(`{"type":"response.done","response":{"id":"first","status":"completed","usage":{"input_tokens":0,"output_tokens":0}}}`)))
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"input_audio_buffer.append","audio":"fixture-only"}`)))
	require.NoError(t, audit.Observe([]byte(`{"type":"response.created","response":{"id":"auto"}}`)))
	require.NoError(t, audit.Observe([]byte(`{"type":"response.done","response":{"id":"auto","status":"failed"}}`)))
	audit.Close(context.Canceled)
	var state, usage string
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE parent_id=$1 AND turn_no=1`, session.ID).Scan(&state, &usage))
	require.Equal(t, "succeeded", state)
	require.Equal(t, "known", usage)
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE parent_id=$1 AND turn_no=2`, session.ID).Scan(&state, &usage))
	require.Equal(t, "failed", state)
	require.Equal(t, "usage_unknown", usage)
	_, err = db.Exec(`ALTER TABLE gateway_request_attempts RENAME TO ledger_fixture_attempts_offline`)
	require.NoError(t, err)
	require.ErrorIs(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)), requestledger.ErrUnavailable)
}

func TestRequestLedgerRealtimeLateTerminalCannotConsumeNextTurn(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	session, err := l.Begin(context.Background(), "/v1/realtime", "GET", "ws_session")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	audit := newRealtimeLedgerAudit(ctx, &Account{ID: 234})
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)))
	first := []byte(`{"type":"response.done","response":{"id":"resp_first","status":"completed"}}`)
	require.NoError(t, audit.Observe(first))
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)))
	secondID := audit.pending[0].handle.ID
	require.NoError(t, audit.Observe(first))
	require.Len(t, audit.pending, 1, "late terminal must not consume new turn")
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, secondID).Scan(&state))
	require.Equal(t, "inflight", state)
	require.NoError(t, audit.Observe([]byte(`{"type":"response.done","response":{"id":"resp_second","status":"failed"}}`)))
	var responseID string
	require.NoError(t, db.QueryRow(`SELECT upstream_response_id FROM gateway_request_attempt_evidence WHERE request_id=$1`, secondID).Scan(&responseID))
	require.Equal(t, "resp_second", responseID)
	audit.Close(nil)
}

func TestRequestLedgerRealtimeCapacityStillCompletesAdmittedUsage(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	session, err := l.Begin(context.Background(), "/v1/realtime", "GET", "ws_session")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	audit := newRealtimeLedgerAudit(ctx, &Account{ID: 234})
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)))
	require.NoError(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)))
	for i := 0; i < 4095; i++ {
		audit.completed[fmt.Sprintf("old_%d", i)] = struct{}{}
	}
	require.NoError(t, audit.Observe([]byte(`{"type":"response.created","response":{"id":"first"}}`)))
	for _, id := range []string{"first", "second"} {
		require.NoError(t, audit.Observe([]byte(fmt.Sprintf(`{"type":"response.done","response":{"id":"%s","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}}`, id))))
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE parent_id=$1 AND execution_state='succeeded' AND usage_state='known'`, session.ID).Scan(&count))
	require.Equal(t, 2, count)
	require.ErrorIs(t, audit.BeforeWrite([]byte(`{"type":"response.create"}`)), requestledger.ErrUnavailable)
	require.NoError(t, audit.Observe([]byte(`{"type":"response.done","response":{"id":"first","status":"completed"}}`)))
}
