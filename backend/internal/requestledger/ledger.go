// Package requestledger persists gateway admission independently of usage and
// billing. It never stores request bodies, credentials or arbitrary errors, and
// never initiates a monetary operation.
package requestledger

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrUnavailable = errors.New("request ledger unavailable")

const writeTimeout = 3 * time.Second

type Ledger struct {
	reconcileTasks bool
	db             *sql.DB
	instance       string
	ownerMu        sync.Mutex
	stop           chan struct{}
	once           sync.Once
	start          sync.Once
}

func New(db *sql.DB) *Ledger {
	return &Ledger{db: db, instance: uuid.NewString(), stop: make(chan struct{})}
}

func NewStarted(db *sql.DB) *Ledger {
	l := New(db)
	l.reconcileTasks = true
	l.Start()
	return l
}

// Start maintains only this process's lease. Recovery does not replay requests
// or billing. Database time, rather than a machine's clock, decides expiry.
func (l *Ledger) Start() {
	l.start.Do(func() {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-l.stop:
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
					_, err := l.db.ExecContext(ctx, `UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()+interval '90 seconds' WHERE id=$1 AND lease_until>clock_timestamp()`, l.owner())
					cancel()
					if err != nil {
						slog.Error("request_ledger_heartbeat_failed")
					}
					if err := l.Recover(context.Background()); err != nil {
						slog.Error("request_ledger_recovery_failed")
					}
					if l.reconcileTasks {
						if err := l.ReconcileTaskBilling(context.Background()); err != nil {
							slog.Error("request_ledger_task_billing_reconciliation_failed")
						}
					}
				}
			}
		}()
	})
}

func (l *Ledger) owner() string {
	l.ownerMu.Lock()
	defer l.ownerMu.Unlock()
	return l.instance
}

func (l *Ledger) rotateExpiredOwner(expired string) string {
	l.ownerMu.Lock()
	defer l.ownerMu.Unlock()
	if l.instance == expired {
		l.instance = uuid.NewString()
	}
	return l.instance
}

func (l *Ledger) Stop() { l.once.Do(func() { close(l.stop) }) }

type Handle struct {
	ledger          *Ledger
	owner           string
	ID              string
	ParentID        string
	mu              sync.Mutex
	userID          int64
	keyID           int64
	route           string
	method          string
	kind            string
	turnCount       int
	turnMu          sync.Mutex
	current         *Handle
	currentAttempt  *Attempt
	TurnNumber      int
	admissionFailed bool
	metered         bool
	outputObserved  bool
}

type contextKey struct{}

func WithHandle(ctx context.Context, h *Handle) context.Context {
	return context.WithValue(ctx, contextKey{}, h)
}
func FromContext(ctx context.Context) *Handle {
	if ctx == nil {
		return nil
	}
	h, _ := ctx.Value(contextKey{}).(*Handle)
	return h
}

// detachedWrite preserves attribution after client cancellation but is bounded.
func detachedWrite(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
}

func (l *Ledger) Begin(ctx context.Context, route, method, kind string) (*Handle, error) {
	return l.begin(ctx, route, method, kind, nil, 0)
}

func (l *Ledger) begin(ctx context.Context, route, method, kind string, parent *Handle, turn int) (*Handle, error) {
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	if l == nil || l.db == nil {
		return nil, ErrUnavailable
	}
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE", "WORKER":
	default:
		method = "OTHER"
	}
	h := &Handle{ledger: l, ID: uuid.NewString(), route: route, method: method, kind: kind, metered: meteredRoute(route, method, kind)}
	var parentID any
	if parent != nil {
		parent.mu.Lock()
		h.userID, h.keyID, h.ParentID = parent.userID, parent.keyID, parent.ID
		parent.mu.Unlock()
		parentID = parent.ID
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	// Each handle freezes its lease generation. Storage recovery may rotate the
	// process's generation, but never revives already expired work.
	owner := l.owner()
	if parent != nil && (parent.kind == "ws_session" || parent.kind == "async_execution") {
		owner = parent.owner
	}
	const leaseSQL = `INSERT INTO gateway_ledger_instances(id,lease_until)
 VALUES($1,clock_timestamp()+interval '90 seconds') ON CONFLICT(id) DO UPDATE
 SET lease_until=EXCLUDED.lease_until WHERE gateway_ledger_instances.lease_until>clock_timestamp()
 RETURNING id`
	var confirmed string
	err = tx.QueryRowContext(ctx, leaseSQL, owner).Scan(&confirmed)
	if errors.Is(err, sql.ErrNoRows) && (parent == nil || (parent.kind != "ws_session" && parent.kind != "async_execution")) {
		owner = l.rotateExpiredOwner(owner)
		err = tx.QueryRowContext(ctx, leaseSQL, owner).Scan(&confirmed)
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	h.owner = owner
	_, err = tx.ExecContext(ctx, `INSERT INTO gateway_requests(id,parent_id,kind,turn_no,route,method,user_id,api_key_id,instance_id)
 VALUES($1,$2,$3,NULLIF($4,0),$5,$6,NULLIF($7,0),NULLIF($8,0),$9)`, h.ID, parentID, kind, turn, route, method, h.userID, h.keyID, h.owner)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return nil, ErrUnavailable
	}
	return h, nil
}

// BindIdentity is called after credential ownership has been verified, before
// quota/routing checks. A caller without a ledger context is not a gateway call.
func BindIdentity(ctx context.Context, userID, keyID int64) error {
	h := FromContext(ctx)
	if h == nil {
		return nil
	}
	if userID <= 0 || keyID < 0 {
		return ErrUnavailable
	}
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	result, err := h.ledger.db.ExecContext(writeCtx, `UPDATE gateway_requests SET user_id=$2,api_key_id=NULLIF($3,0)
 WHERE id=$1 AND execution_state='inflight' AND (user_id IS NULL OR user_id=$2)
 AND (api_key_id IS NULL OR api_key_id=NULLIF($3,0))`, h.ID, userID, keyID)
	if err != nil {
		return ErrUnavailable
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return ErrUnavailable
	}
	h.mu.Lock()
	h.userID, h.keyID = userID, keyID
	h.mu.Unlock()
	return nil
}

func (h *Handle) Finish(ctx context.Context, state string, status int, code string) error {
	if h == nil {
		return nil
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	_, err := h.ledger.db.ExecContext(ctx, `WITH closing AS (
 SELECT id, CASE WHEN $2<>'succeeded' THEN $2
 WHEN EXISTS(SELECT 1 FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.execution_state='inflight') THEN 'interrupted'
 ELSE COALESCE((SELECT execution_state FROM gateway_request_attempts a WHERE a.request_id=r.id ORDER BY attempt_no DESC LIMIT 1),$2) END AS state,
 (SELECT error_code FROM gateway_request_attempts a WHERE a.request_id=r.id ORDER BY attempt_no DESC LIMIT 1) AS last_code
 FROM gateway_requests r WHERE id=$1 AND execution_state='inflight' FOR UPDATE
 ), finished AS (
 UPDATE gateway_requests r SET execution_state=c.state,ended_at=clock_timestamp(),http_status=NULLIF($3,0),
 error_code=CASE WHEN c.state='interrupted' THEN COALESCE(NULLIF(c.last_code,''),'interrupted') WHEN $4='' AND c.state<>'succeeded' THEN COALESCE(NULLIF(c.last_code,''),'upstream_error') ELSE $4 END,
 usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END
 FROM closing c WHERE r.id=c.id RETURNING r.id,r.execution_state)
 UPDATE gateway_request_attempts a SET usage_state=CASE WHEN a.usage_state='pending' THEN 'usage_unknown' ELSE a.usage_state END,execution_state=f.execution_state,ended_at=clock_timestamp(),
 error_code=CASE WHEN f.execution_state IN ('cancelled','timeout','interrupted') THEN f.execution_state ELSE 'upstream_error' END
 FROM finished f WHERE a.request_id=f.id AND a.execution_state='inflight'`, h.ID, state, status, code)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// Recover only marks stale work uncertain. It cannot infer whether an external
// request was sent, completed, or billed during a crash window.
func (l *Ledger) Recover(ctx context.Context) error {
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `WITH stale AS (
 SELECT r.id FROM gateway_requests r JOIN gateway_ledger_instances i ON i.id=r.instance_id
 WHERE r.execution_state='inflight' AND i.lease_until<clock_timestamp() LIMIT 1000 FOR UPDATE OF r SKIP LOCKED
 ), finished AS (
 UPDATE gateway_requests r SET execution_state='interrupted',ended_at=clock_timestamp(),error_code='interrupted',
 usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END
 FROM stale WHERE r.id=stale.id RETURNING r.id)
 UPDATE gateway_request_attempts a SET usage_state=CASE WHEN a.usage_state='pending' THEN 'usage_unknown' ELSE a.usage_state END,execution_state='interrupted',ended_at=clock_timestamp(),error_code='interrupted'
 FROM finished WHERE a.request_id=finished.id AND a.execution_state='inflight'`)
	if err != nil {
		return ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return ErrUnavailable
	}
	return nil
}

func Outcome(status int, err error) string {
	if errors.Is(err, errStreamIncomplete) {
		return "interrupted"
	}
	if errors.Is(err, context.DeadlineExceeded) || status == 504 || status == 408 {
		return "timeout"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if err != nil || status >= 400 {
		return "failed"
	}
	return "succeeded"
}

func ErrorCode(err error) string {
	if errors.Is(err, ErrUnavailable) {
		return "request_ledger_unavailable"
	}
	if errors.Is(err, errStreamIncomplete) {
		return "stream_incomplete"
	}
	if err == nil {
		return ""
	}
	switch Outcome(0, err) {
	case "cancelled":
		return "cancelled"
	case "timeout":
		return "timeout"
	default:
		return "upstream_error"
	}
}
