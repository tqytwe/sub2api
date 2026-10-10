package requestledger

import "context"

// AcceptTurn must run when a response.create frame arrives, before policy or
// parsing can reject it. Payload bytes and client event IDs are never retained.
func AcceptTurn(ctx context.Context) (*Handle, error) { return acceptTurn(ctx, false) }

// AcceptObservedTurn also supports server-generated Live turns on an admitted observer.
func AcceptObservedTurn(ctx context.Context) (*Handle, error) { return acceptTurn(ctx, true) }

func acceptTurn(ctx context.Context, observer bool) (*Handle, error) {
	parent := FromContext(ctx)
	if parent == nil || (parent.kind != "ws_session" && (!observer || parent.kind != "async_execution")) {
		return nil, nil
	}
	parent.turnMu.Lock()
	defer parent.turnMu.Unlock()
	parent.mu.Lock()
	number := parent.turnCount + 1
	parent.mu.Unlock()
	h, err := parent.ledger.begin(ctx, parent.route, parent.method, "ws_turn", parent, number)
	if err != nil {
		return nil, err
	}
	h.TurnNumber = number
	parent.mu.Lock()
	parent.turnCount = number
	parent.current = h
	parent.mu.Unlock()
	return h, nil
}

// CurrentContext freezes the turn handle for callbacks/queued usage tasks.
// Keeping the session context alone would attribute late usage to a later turn.
func CurrentContext(ctx context.Context) context.Context {
	h := FromContext(ctx)
	if h == nil || h.kind != "ws_session" {
		return ctx
	}
	h.mu.Lock()
	current := h.current
	h.mu.Unlock()
	if current == nil {
		return ctx
	}
	return WithHandle(ctx, current)
}

func FinishTurn(ctx context.Context, state string, cause error) error {
	h := FromContext(ctx)
	if h == nil {
		return nil
	}
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	_, err := h.ledger.db.ExecContext(writeCtx, `UPDATE gateway_request_attempts SET usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END,execution_state=$2,ended_at=clock_timestamp(),error_code=$3
 WHERE request_id=$1 AND execution_state='inflight'`, h.ID, state, ErrorCode(cause))
	if err != nil {
		return ErrUnavailable
	}
	return h.Finish(ctx, state, 0, ErrorCode(cause))
}

func (h *Handle) closeOpenTurns(ctx context.Context) error {
	if h == nil || h.kind != "ws_session" {
		return nil
	}
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	state := "interrupted"
	if ctx.Err() != nil {
		state = Outcome(0, ctx.Err())
	}
	_, err := h.ledger.db.ExecContext(writeCtx, `WITH finished AS (
 UPDATE gateway_requests SET execution_state=CASE WHEN attempt_count=0 THEN 'failed' ELSE $2 END,
 ended_at=clock_timestamp(),error_code=CASE WHEN attempt_count=0 THEN 'invalid_request' ELSE $2 END,
 usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END
 WHERE parent_id=$1 AND execution_state='inflight' RETURNING id)
 UPDATE gateway_request_attempts a SET usage_state=CASE WHEN a.usage_state='pending' THEN 'usage_unknown' ELSE a.usage_state END,execution_state=$2,ended_at=clock_timestamp(),error_code=$2
 FROM finished WHERE a.request_id=finished.id AND a.execution_state='inflight'`, h.ID, state)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
