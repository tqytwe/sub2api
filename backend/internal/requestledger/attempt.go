package requestledger

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

type Attempt struct {
	outputObserved atomic.Bool
	handle         *Handle
	Number         int
}

// BeginAttempt is a synchronous admission gate, not an event queue. The row
// lock allocates sequence numbers across retries and concurrent attempts.
func BeginAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "request", true)
}

func BeginConnectionAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "ws_connect", true)
}

func BeginControlAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "ws_control", true)
}

// Session input is a continuous stream; its durable attempt precedes the first
// frame. Automatic upstream turns are separate observations, not claimed sends.
func BeginSessionInputAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "ws_input", false)
}
func BeginSessionControlAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "ws_control", false)
}
func BeginObservedAttempt(ctx context.Context, accountID, credentialAccountID int64) (*Attempt, error) {
	return beginAttempt(ctx, accountID, credentialAccountID, "ws_observed", false)
}

func beginAttempt(ctx context.Context, accountID, credentialAccountID int64, phase string, resolveTurn bool) (attempt *Attempt, failure error) {
	if resolveTurn {
		ctx = CurrentContext(ctx)
	}
	h := FromContext(ctx)
	if h == nil {
		return nil, nil
	}
	defer func() {
		if failure != nil {
			h.mu.Lock()
			h.admissionFailed = true
			h.mu.Unlock()
		}
	}()
	h.mu.Lock()
	denied := h.admissionFailed
	h.mu.Unlock()
	if denied {
		return nil, ErrUnavailable
	}
	if phase != "external_search" && (accountID <= 0 || credentialAccountID <= 0) {
		return nil, ErrUnavailable
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := h.ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	a := &Attempt{handle: h}
	metered := (h.metered && phase == "request") || phase == "ws_input" || phase == "ws_observed" || phase == "external_search"
	err = tx.QueryRowContext(ctx, `UPDATE gateway_requests r SET attempt_count=attempt_count+1,
 usage_state=CASE WHEN usage_state IN ('not_applicable','known') AND $3 THEN 'pending' ELSE usage_state END,
 settlement_state=CASE WHEN settlement_state='not_required' AND $3 THEN 'settlement_pending' ELSE settlement_state END
 WHERE r.id=$1 AND r.execution_state='inflight' AND r.user_id IS NOT NULL
 AND r.instance_id=$2 AND EXISTS(SELECT 1 FROM gateway_ledger_instances i WHERE i.id=r.instance_id AND i.lease_until>clock_timestamp())
 RETURNING attempt_count`, h.ID, h.owner, metered).Scan(&a.Number)
	if err != nil {
		return nil, ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id,phase,upstream_kind,usage_state) VALUES($1,$2,NULLIF($3,0),NULLIF($4,0),$5::text,CASE WHEN $5::text='external_search' THEN 'external_search' ELSE 'account' END,CASE WHEN $6 THEN 'pending' ELSE 'not_applicable' END)`, h.ID, a.Number, accountID, credentialAccountID, phase, metered)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return nil, ErrUnavailable
	}
	if metered {
		h.mu.Lock()
		h.currentAttempt = a
		h.mu.Unlock()
	}
	return a, nil
}

func (a *Attempt) Finish(ctx context.Context, status int, cause error) error {
	if a == nil {
		return nil
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	code := ErrorCode(cause)
	if code == "" && status >= 400 {
		code = "upstream_error"
	}
	_, err := a.handle.ledger.db.ExecContext(ctx, `UPDATE gateway_request_attempts
 SET execution_state=$3,ended_at=clock_timestamp(),http_status=NULLIF($4,0),error_code=$5,usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END
 WHERE request_id=$1 AND attempt_no=$2 AND execution_state='inflight'`, a.handle.ID, a.Number, Outcome(status, cause), status, code)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

type accountKey struct{}
type accountIdentity struct{ scheduled, credential int64 }

func WithAccount(ctx context.Context, scheduled, credential int64) context.Context {
	return context.WithValue(ctx, accountKey{}, accountIdentity{scheduled, credential})
}

// Transport wraps the actual RoundTripper, inside any compatibility retry
// transport, so a fallback request receives its own precommitted attempt.
func Transport(base http.RoundTripper, accountID int64) http.RoundTripper {
	return &auditTransport{base: base, accountID: accountID}
}

// ExternalSearchTransport covers configured search services which have no
// scheduler account. NULL attribution is explicit, never a fabricated account.
func ExternalSearchTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &auditTransport{base: base, external: true}
}

type auditTransport struct {
	external  bool
	base      http.RoundTripper
	accountID int64
}

func (t *auditTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	scheduled, credential := t.accountID, t.accountID
	if identity, ok := req.Context().Value(accountKey{}).(accountIdentity); ok && (scheduled == 0 || identity.scheduled == scheduled) {
		scheduled, credential = identity.scheduled, identity.credential
	}
	var a *Attempt
	var err error
	if t.external {
		a, err = beginAttempt(req.Context(), 0, 0, "external_search", true)
	} else {
		a, err = BeginAttempt(req.Context(), scheduled, credential)
	}
	if err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		if finishErr := a.Finish(req.Context(), 0, err); finishErr != nil {
			slog.Error("request_ledger_attempt_finish_failed")
		}
		return resp, err
	}
	if a != nil {
		resp.Body = &attemptBody{ReadCloser: resp.Body, attempt: a, ctx: req.Context(), status: resp.StatusCode, streaming: strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") && resp.StatusCode < 400}
	}
	return resp, nil
}

type attemptBody struct {
	io.ReadCloser
	attempt   *Attempt
	ctx       context.Context
	status    int
	once      sync.Once
	streaming bool
	evidence  streamEvidence
}

func (b *attemptBody) finish(err error) {
	b.once.Do(func() {
		if e := b.attempt.Finish(b.ctx, b.status, err); e != nil {
			slog.Error("request_ledger_attempt_finish_failed")
		}
	})
}
func (b *attemptBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 && b.status >= 200 && b.status < 300 {
		b.attempt.ObserveOutput(b.ctx)
	}
	if b.streaming {
		b.evidence.observe(p[:n])
	}
	if err == io.EOF {
		var cause error
		if b.streaming {
			switch b.evidence.terminal {
			case "succeeded":
			case "failed":
				cause = errTerminalFailure
			case "cancelled":
				cause = context.Canceled
			default:
				cause = errStreamIncomplete
			}
		}
		b.finish(cause)
	} else if err != nil {
		b.finish(err)
	}
	return n, err
}
func (b *attemptBody) Close() error {
	err := b.ReadCloser.Close()
	// Closing before EOF does not prove upstream completion, even for HTTP 200.
	cause := b.ctx.Err()
	if cause == nil {
		cause = io.ErrUnexpectedEOF
	}
	b.finish(cause)
	return err
}

// TrackResponse covers plugin transports using the same durable gate as HTTP.
func (a *Attempt) TrackResponse(ctx context.Context, resp *http.Response, err error) {
	if a == nil {
		return
	}
	if err != nil {
		_ = a.Finish(ctx, 0, err)
		return
	}
	if resp == nil || resp.Body == nil {
		_ = a.Finish(ctx, 0, io.ErrUnexpectedEOF)
		return
	}
	resp.Body = &attemptBody{ReadCloser: resp.Body, attempt: a, ctx: ctx, status: resp.StatusCode, streaming: strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") && resp.StatusCode < 400}
}
