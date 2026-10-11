//go:build integration

package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresRoutingStatesBoundsAndLoss(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	defer l.Stop()
	ctx := context.Background()
	viewer := Viewer{Admin: true, UserID: 1}
	begin := func() *Handle {
		h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
		require.NoError(t, err)
		return h
	}
	t.Run("old_writer_and_no_selection", func(t *testing.T) {
		h := begin()
		require.NoError(t, h.Finish(ctx, "failed", 401, "unauthorized"))
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "not_recorded", e.Status)
		_, err = db.Exec(`UPDATE gateway_requests SET routing_coverage_version='' WHERE id=$1`, h.ID)
		require.NoError(t, err)
		e, err = l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "not_recorded", e.Status)
		_, err = l.RoutingEvidence(ctx, Viewer{UserID: 1}, h.ID)
		require.ErrorIs(t, err, ErrRoutingForbidden)
		_, err = l.RoutingEvidence(ctx, viewer, uuid.NewString())
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
	t.Run("truncated_and_secrets", func(t *testing.T) {
		h := begin()
		c, end := BeginRoutingSelection(WithHandle(ctx, h), true)
		for range 300 {
			RoutingVeto(c, "initial_filter", 33, "model_not_supported")
		}
		end(44, false, false)
		require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
		var e *RoutingEvidence
		require.Eventually(t, func() bool {
			var err error
			e, err = l.RoutingEvidence(ctx, viewer, h.ID)
			return err == nil && e.Status == "truncated"
		}, time.Second, 10*time.Millisecond)
		require.Equal(t, 128, len(e.Events))
		require.Equal(t, 174, e.OmittedCount)
		require.False(t, e.SourcePopulationComplete)
		var size int
		require.NoError(t, db.QueryRow(`SELECT octet_length(events::text) FROM gateway_request_routing WHERE request_id=$1`, h.ID).Scan(&size))
		require.LessOrEqual(t, size, 32768)
	})
	t.Run("crash_and_missing_batch", func(t *testing.T) {
		h := begin()
		c, end := BeginRoutingSelection(WithHandle(ctx, h), true)
		RoutingVeto(c, "initial_filter", 33, "excluded")
		end(0, false, true)
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "pending", e.Status)
		l.forget(h.ID, false)
		require.NoError(t, l.Recover(ctx))
		e, err = l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "incomplete", e.Status)
	})
	t.Run("queue_full", func(t *testing.T) {
		blocked := New(db)
		defer blocked.Stop()
		blocked.routingStart.Do(func() {})
		for range cap(blocked.routingQueue) {
			blocked.routingQueue <- routingBatch{}
		}
		h, err := blocked.Begin(ctx, "/v1/messages", "POST", "http")
		require.NoError(t, err)
		_, end := BeginRoutingSelection(WithHandle(ctx, h), true)
		end(44, false, false)
		started := time.Now()
		require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
		require.Less(t, time.Since(started), time.Second)
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "incomplete", e.Status)
	})
	t.Run("unsupported", func(t *testing.T) {
		h, err := l.Begin(ctx, "/v1/images/generations", "POST", "http")
		require.NoError(t, err)
		_, end := BeginRoutingSelection(WithHandle(ctx, h), true)
		end(1, false, false)
		require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "unsupported_coverage", e.Status)
	})
}

func TestLedgerPostgresRoutingWriterLockAndPoolPressure(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	defer l.Stop()
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	events, _ := json.Marshal([]RoutingEvent{{SelectionNo: 1, Stage: "initial_filter", Reason: "excluded", AccountID: 7, At: time.Now()}})
	batch := routingBatch{id: h.ID, data: string(events), count: 1}
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`LOCK TABLE gateway_request_routing IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	// Keep connection capacity available so this exercises the lock deadline,
	// not the intentional saturated-pool fast drop.
	require.NoError(t, db.Ping())
	start := time.Now()
	l.persistRouting(batch)
	require.Less(t, time.Since(start), time.Second)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_routing`).Scan(&count))
	require.Zero(t, count)
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	start = time.Now()
	l.persistRouting(batch)
	require.Less(t, time.Since(start), 50*time.Millisecond)
	require.NoError(t, conn.Close())
	db.SetMaxOpenConns(0)
	c, end := BeginRoutingSelection(WithHandle(ctx, h), true)
	RoutingVeto(c, "initial_filter", 1, "excluded")
	end(0, false, true)
	l.Stop()
	require.NoError(t, h.Finish(ctx, "failed", 503, "internal_error"))
	e, err := l.RoutingEvidence(ctx, Viewer{Admin: true, UserID: 1}, h.ID)
	require.NoError(t, err)
	require.Equal(t, "incomplete", e.Status)
}

func TestLedgerPostgresRoutingFinishBarrierAndFailure(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	defer l.Stop()
	ctx := context.Background()
	viewer := Viewer{Admin: true, UserID: 1}
	begin := func() *Handle {
		h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
		require.NoError(t, err)
		c, end := BeginRoutingSelection(WithHandle(ctx, h), true)
		RoutingVeto(c, "initial_filter", 9, "excluded")
		end(10, false, false)
		return h
	}
	count := func(id string) int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_routing WHERE request_id=$1`, id).Scan(&n))
		return n
	}
	t.Run("ready_waits_for_finish", func(t *testing.T) {
		h := begin()
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()
		_, err = tx.Exec(`SELECT id FROM gateway_requests WHERE id=$1 FOR UPDATE`, h.ID)
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() { done <- h.Finish(ctx, "succeeded", 200, "") }()
		require.Eventually(t, func() bool {
			h.routing.mu.Lock()
			defer h.routing.mu.Unlock()
			return h.routing.ready != nil
		}, time.Second, time.Millisecond)
		require.Never(t, func() bool { return count(h.ID) > 0 }, 100*time.Millisecond, 10*time.Millisecond)
		require.NoError(t, tx.Rollback())
		require.NoError(t, <-done)
		require.Eventually(t, func() bool { return count(h.ID) == 1 }, time.Second, 10*time.Millisecond)
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "complete", e.Status)
	})
	t.Run("failed_finish_with_persisted_batch", func(t *testing.T) {
		h := begin()
		// Reject only finalization, leaving durable admission and the detail table writable.
		_, err := db.Exec(`ALTER TABLE gateway_requests ADD CONSTRAINT fixture_reject_finish CHECK (execution_state <> 'succeeded') NOT VALID`)
		require.NoError(t, err)
		require.ErrorIs(t, h.Finish(ctx, "succeeded", 200, ""), ErrUnavailable)
		require.Eventually(t, func() bool { return count(h.ID) == 1 }, time.Second, 10*time.Millisecond)
		e, err := l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "pending", e.Status)
		_, err = db.Exec(`ALTER TABLE gateway_requests DROP CONSTRAINT fixture_reject_finish`)
		require.NoError(t, err)
		require.NoError(t, l.Recover(ctx))
		e, err = l.RoutingEvidence(ctx, viewer, h.ID)
		require.NoError(t, err)
		require.Equal(t, "incomplete", e.Status)
		require.True(t, e.Incomplete)
	})
}

func TestLedgerPostgresRoutingFinishedBatchLossExpires(t *testing.T) {
	db := ledgerPostgres(t)
	ctx := context.Background()
	for _, cause := range []string{"lock", "pool"} {
		t.Run(cause, func(t *testing.T) {
			l := New(db)
			defer l.Stop()
			// Drive this exact queued worker batch deterministically after Finish.
			l.routingStart.Do(func() {})
			h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
			require.NoError(t, err)
			_, end := BeginRoutingSelection(WithHandle(ctx, h), true)
			end(9, false, false)
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			batch := <-l.routingQueue
			if cause == "lock" {
				tx, err := db.Begin()
				require.NoError(t, err)
				_, err = tx.Exec(`LOCK TABLE gateway_request_routing IN ACCESS EXCLUSIVE MODE`)
				require.NoError(t, err)
				l.persistRouting(batch)
				require.NoError(t, tx.Rollback())
			} else {
				db.SetMaxOpenConns(1)
				conn, err := db.Conn(ctx)
				require.NoError(t, err)
				l.persistRouting(batch)
				require.NoError(t, conn.Close())
				db.SetMaxOpenConns(0)
			}
			e, err := l.RoutingEvidence(ctx, Viewer{Admin: true, UserID: 1}, h.ID)
			require.NoError(t, err)
			require.Equal(t, "pending", e.Status)
			_, err = db.Exec(`UPDATE gateway_requests SET ended_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, h.ID)
			require.NoError(t, err)
			e, err = l.RoutingEvidence(ctx, Viewer{Admin: true, UserID: 1}, h.ID)
			require.NoError(t, err)
			require.Equal(t, "incomplete", e.Status)
			require.True(t, e.Incomplete)
			require.Empty(t, e.Events)
		})
	}
}
