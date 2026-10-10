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
	streamTerminal atomic.Int32
	handle         *Handle
	Number         int
	accountID      int64
}

// CurrentAttempt captures the precise model attempt before a parser or detached
// usage callback runs. Auxiliary transfers must never replace this attribution.
func CurrentAttempt(ctx context.Context) *Attempt {
	if ctx == nil {
		return nil
	}
	if frozen, ok := ctx.Value(attemptKey{}).(*Attempt); ok {
		return frozen
	}
	h := FromContext(CurrentContext(ctx))
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.currentAttempt
}

type attemptKey struct{}

// FreezeContext copies only durable identity, never request bodies or secrets.
func FreezeContext(parent, base context.Context) context.Context {
	if parent == nil {
		return base
	}
	if base == nil {
		base = context.Background()
	}
	parent = CurrentContext(parent)
	base = WithHandle(base, FromContext(parent))
	return context.WithValue(base, attemptKey{}, CurrentAttempt(parent))
}

// ObserveStreamTerminal accepts the existing business parser's authoritative
// terminal classification. It can supersede a provisional bare error, without
// reparsing or retaining output. Persistence happens when the body is closed.
func (a *Attempt) ObserveStreamTerminal(state string) {
	if a == nil {
		return
	}
	switch state {
	case "succeeded":
		a.streamTerminal.Store(1)
	case "failed":
		a.streamTerminal.Store(2)
	case "cancelled":
		a.streamTerminal.Store(3)
	}
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
	a := &Attempt{handle: h, accountID: accountID}
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
	return TransportDecoded(base, accountID, nil)
}

// TransportDecoded keeps the admission gate inside each actual RoundTrip, but
// observes decoded bytes when the upstream client performs explicit decoding.
func TransportDecoded(base http.RoundTripper, accountID int64, decode func(*http.Response)) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &auditTransport{base: base, accountID: accountID, decode: decode}
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
	decode    func(*http.Response)
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
		phase := "request"
		if h := FromContext(CurrentContext(req.Context())); h != nil && h.metered && (req.Method == http.MethodGet || req.Method == http.MethodHead) {
			phase = "auxiliary"
		}
		a, err = beginAttempt(req.Context(), scheduled, credential, phase, true)
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
	if t.decode != nil {
		t.decode(resp)
	}
	if a != nil {
		resp.Body = newAttemptBody(req.Context(), resp, a, req.Method)
	}
	return resp, nil
}

func newAttemptBody(ctx context.Context, resp *http.Response, attempt *Attempt, method string) *attemptBody {
	body := &attemptBody{ReadCloser: resp.Body, attempt: attempt, ctx: ctx, status: resp.StatusCode,
		streaming: strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") && resp.StatusCode < 400}
	// HEAD, 204 and 304 complete at their headers. Callers correctly skip Read.
	if method == http.MethodHead || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified || resp.Body == http.NoBody {
		body.streaming = false
		body.readErr = io.EOF
	}
	return body
}

type attemptBody struct {
	io.ReadCloser
	attempt   *Attempt
	ctx       context.Context
	status    int
	once      sync.Once
	streaming bool
	mu        sync.Mutex
	evidence  streamEvidence
	readErr   error
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
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.streaming {
		b.evidence.observe(p[:n])
	}
	if err != nil {
		b.readErr = err
	}
	return n, err
}
func (b *attemptBody) Close() error {
	err := b.ReadCloser.Close()
	b.mu.Lock()
	defer b.mu.Unlock()
	cause := b.readErr
	if b.streaming {
		terminal := b.evidence.terminal
		switch b.attempt.streamTerminal.Load() {
		case 1:
			terminal = "succeeded"
		case 2:
			terminal = "failed"
		case 3:
			terminal = "cancelled"
		}
		// A parser may stop at a terminal before transport EOF. Cleanup cancellation
		// does not undo that evidence. Without a terminal, an early close is uncertain.
		switch terminal {
		case "succeeded":
			cause = nil
		case "failed":
			cause = errTerminalFailure
		case "cancelled":
			cause = context.Canceled
		default:
			if cause == nil || cause == io.EOF {
				cause = errStreamIncomplete
			}
		}
	} else if cause == io.EOF {
		cause = nil
	} else if cause == nil {
		cause = b.ctx.Err()
		if cause == nil {
			cause = io.ErrUnexpectedEOF
		}
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
	method := ""
	if resp.Request != nil {
		method = resp.Request.Method
	}
	resp.Body = newAttemptBody(ctx, resp, a, method)
}
