package requestledger

import (
	"context"
	"database/sql"
	"errors"
)

// BindTask commits before enqueue/creation. Failed enqueue may leave a reference
// to a task that never existed; it still truthfully records the submission intent.
func BindTask(ctx context.Context, kind, ref string, userID, keyID int64) error {
	h := FromContext(ctx)
	if h == nil {
		return nil
	}
	if len(ref) == 0 || len(ref) > 160 {
		return ErrUnavailable
	}
	if err := BindIdentity(ctx, userID, keyID); err != nil {
		return err
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	_, err := h.ledger.db.ExecContext(ctx, `INSERT INTO gateway_request_tasks(task_kind,task_ref,request_id,user_id,api_key_id)
 VALUES($1,$2,$3,$4,NULLIF($5,0)) ON CONFLICT DO NOTHING`, kind, ref, h.ID, userID, keyID)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// BeginTask records this worker execution, including after a restart. Missing
// pre-rollout submissions remain missing: an execution may have no parent.
func (l *Ledger) BeginTask(ctx context.Context, kind, ref string, userID, keyID int64, metered bool) (*Handle, error) {
	if l == nil {
		return nil, nil
	} // only legacy test fixtures omit the wired ledger
	if userID <= 0 || keyID < 0 || len(ref) == 0 || len(ref) > 160 {
		return nil, ErrUnavailable
	}
	readCtx, cancel := detachedWrite(ctx)
	defer cancel()
	var id string
	var owner int64
	var key sql.NullInt64
	err := l.db.QueryRowContext(readCtx, `SELECT t.request_id,t.user_id,t.api_key_id FROM gateway_request_tasks t
 JOIN gateway_requests r ON r.id=t.request_id WHERE t.task_kind=$1 AND t.task_ref=$2
 ORDER BY (r.kind='async_execution'), t.created_at,t.request_id LIMIT 1`, kind, ref).Scan(&id, &owner, &key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnavailable
	}
	var parent *Handle
	if err == nil {
		if owner != userID || key.Int64 != keyID {
			return nil, ErrUnavailable
		}
		parent = &Handle{ID: id, userID: userID, keyID: keyID}
	}
	h, err := l.begin(ctx, "async/"+kind, "WORKER", "async_execution", parent, 0)
	if err != nil {
		return nil, err
	}
	childCtx := WithHandle(ctx, h)
	if err = BindTask(childCtx, kind, ref, userID, keyID); err != nil {
		_ = h.Finish(ctx, "failed", 0, "request_ledger_unavailable")
		return nil, err
	}
	h.mu.Lock()
	h.metered = metered
	h.mu.Unlock()
	return h, nil
}

// ResumeTask restores attribution for a late settlement only. Missing historical
// admission remains missing; this does not create a request or a new billing key.
func (l *Ledger) ResumeTask(ctx context.Context, kind, ref string, userID, keyID int64) (context.Context, error) {
	if l == nil {
		return ctx, nil
	}
	readCtx, cancel := detachedWrite(ctx)
	defer cancel()
	var id string
	var owner, key int64
	err := l.db.QueryRowContext(readCtx, `SELECT t.request_id,t.user_id,COALESCE(t.api_key_id,0) FROM gateway_request_tasks t
 JOIN gateway_requests r ON r.id=t.request_id WHERE t.task_kind=$1 AND t.task_ref=$2
 ORDER BY (r.kind='async_execution'),t.created_at,t.request_id LIMIT 1`, kind, ref).Scan(&id, &owner, &key)
	if errors.Is(err, sql.ErrNoRows) {
		return ctx, nil
	}
	if err != nil || owner != userID || key != keyID {
		return ctx, ErrUnavailable
	}
	return WithHandle(ctx, &Handle{ledger: l, ID: id, userID: owner, keyID: key}), nil
}
