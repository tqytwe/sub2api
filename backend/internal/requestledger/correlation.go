package requestledger

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// Correlation is public metadata. Receipt is the in-process capability that
// authorizes a link; decoding this metadata cannot manufacture a Receipt.
type Correlation struct {
	RequestID string `json:"request_id"`
	AttemptID string `json:"attempt_id,omitempty"`
	AttemptNo int    `json:"attempt_no"`
}

type Receipt struct {
	handle  *Handle
	attempt *Attempt
}

func RequestReceipt(ctx context.Context) Receipt {
	return Receipt{handle: FromContext(CurrentContext(ctx))}
}
func (a *Attempt) Receipt() Receipt {
	if a == nil {
		return Receipt{}
	}
	return Receipt{handle: a.handle, attempt: a}
}
func (r Receipt) Correlation() *Correlation {
	if r.handle == nil {
		return nil
	}
	c := &Correlation{RequestID: r.handle.ID}
	if r.attempt != nil {
		c.AttemptID, c.AttemptNo = r.attempt.ID, r.attempt.Number
	}
	return c
}

func (r Receipt) TurnNumber() int {
	if r.handle == nil {
		return 0
	}
	return r.handle.TurnNumber
}
func (r Receipt) AccountID() int64 {
	if r.attempt == nil {
		return 0
	}
	return r.attempt.accountID
}
func (r Receipt) ObserveIdentifiers(ctx context.Context, requestID, responseID string) {
	r.attempt.ObserveIdentifiers(ctx, requestID, responseID)
}
func (r Receipt) ObserveOutput(ctx context.Context) { r.attempt.ObserveOutput(ctx) }

// IngressReceipt is the outer HTTP/session identity, used for a diagnostic whose
// capture site did not provide a turn/attempt receipt.
func IngressReceipt(ctx context.Context) Receipt { return Receipt{handle: FromContext(ctx)} }

// The private context key follows the specific response through body wrappers,
// retries and async consumers. It is never placed on the wire.
type responseReceiptKey struct{}

func attachResponseReceipt(ctx context.Context, resp *http.Response, receipt Receipt) {
	if resp == nil || receipt.Correlation() == nil {
		return
	}
	if resp.Request == nil {
		resp.Request = &http.Request{}
	} else {
		ctx = resp.Request.Context()
	}
	resp.Request = resp.Request.WithContext(context.WithValue(ctx, responseReceiptKey{}, receipt))
}
func ResponseReceipt(resp *http.Response) Receipt {
	if resp == nil || resp.Request == nil {
		return Receipt{}
	}
	receipt, _ := resp.Request.Context().Value(responseReceiptKey{}).(Receipt)
	return receipt
}
func ResponseAttempt(resp *http.Response) *Attempt { return ResponseReceipt(resp).attempt }

type attemptError struct {
	cause   error
	receipt Receipt
}

func (e *attemptError) Error() string { return e.cause.Error() }
func (e *attemptError) Unwrap() error { return e.cause }
func (e *attemptError) Timeout() bool { var n net.Error; return errors.As(e.cause, &n) && n.Timeout() }
func (e *attemptError) Temporary() bool {
	var n interface{ Temporary() bool }
	return errors.As(e.cause, &n) && n.Temporary()
}
func (a *Attempt) WrapError(err error) error {
	if a == nil || err == nil {
		return err
	}
	return &attemptError{cause: err, receipt: a.Receipt()}
}
func ErrorReceipt(err error) Receipt {
	var e *attemptError
	if errors.As(err, &e) {
		return e.receipt
	}
	return Receipt{}
}

// Do preserves the exact transport failure across http.Client timeout errors,
// which otherwise replace (rather than wrap) the RoundTripper error. The slot is
// private to one Do call; a later successful/rejected attempt clears it.
func Do(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if req == nil || FromContext(CurrentContext(req.Context())) == nil {
		return send(req)
	}
	slot := &transportFailureSlot{}
	req = req.WithContext(context.WithValue(req.Context(), transportFailureKey{}, slot))
	resp, err := send(req)
	if err != nil && ErrorReceipt(err).Correlation() == nil {
		slot.mu.Lock()
		receipt := slot.receipt
		slot.mu.Unlock()
		if receipt.attempt != nil {
			err = receipt.attempt.WrapError(err)
		}
	}
	return resp, err
}

type transportFailureKey struct{}
type transportFailureSlot struct {
	mu      sync.Mutex
	receipt Receipt
}

func captureTransportFailure(ctx context.Context, receipt Receipt) {
	if slot, ok := ctx.Value(transportFailureKey{}).(*transportFailureSlot); ok {
		slot.mu.Lock()
		slot.receipt = receipt
		slot.mu.Unlock()
	}
}

// UpstreamIdentifier drops malformed/oversized identifiers instead of truncating
// them into an ambiguous match. These are bounded evidence, never dedup keys.
func UpstreamIdentifier(s string) string {
	if len(s) == 0 || len(s) > 128 {
		return ""
	}
	for i, c := range []byte(s) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		if i > 0 && (c == '_' || c == '-' || c == '.' || c == ':') {
			continue
		}
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "sk-") || strings.HasPrefix(lower, "sess-") || strings.HasPrefix(lower, "bearer") || strings.HasPrefix(s, "eyJ") {
		return ""
	}
	return s
}

func UpstreamRequestID(headers http.Header) string {
	for _, name := range []string{"x-request-id", "request-id", "x-goog-request-id", "xai-request-id", "x-amzn-requestid"} {
		if id := UpstreamIdentifier(headers.Get(name)); id != "" {
			return id
		}
	}
	return ""
}

// ResponseIdentifier inspects only known envelope fields, never arbitrary nested
// content. Callers bound the input; oversized bodies simply have no identifier.
func ResponseIdentifier(data []byte) string {
	if len(data) > 64*1024 || !gjson.ValidBytes(data) {
		return ""
	}
	paths := []string{"response.id", "response_id", "responseId", "message.id"}
	// Top-level event IDs (especially WS error.id) are not response IDs.
	object, kind := gjson.GetBytes(data, "object").String(), gjson.GetBytes(data, "type").String()
	if object == "response" || object == "chat.completion" || object == "chat.completion.chunk" || kind == "message" {
		paths = append(paths, "id")
	}
	for _, path := range paths {
		v := gjson.GetBytes(data, path)
		if v.Type == gjson.String {
			if id := UpstreamIdentifier(v.String()); id != "" {
				return id
			}
		}
	}
	return ""
}

// identifierWrites is process-wide and bounded. Only validated metadata and the
// existing Attempt are queued, never a request context, body or credential.
var identifierWrites = struct {
	once  sync.Once
	queue chan *Attempt
}{queue: make(chan *Attempt, 256)}

func enqueueIdentifiers(a *Attempt) bool {
	identifierWrites.once.Do(func() {
		go func() {
			for a := range identifierWrites.queue {
				a.persistIdentifiers()
			}
		}()
	})
	select {
	case identifierWrites.queue <- a:
		return true
	default:
		return false
	}
}

// ObserveIdentifiers only updates bounded in-memory metadata. In particular it
// never waits for PostgreSQL while RoundTrip or a streaming body read is active.
func (a *Attempt) ObserveIdentifiers(_ context.Context, requestID, responseID string) {
	if a == nil {
		return
	}
	requestID, responseID = UpstreamIdentifier(requestID), UpstreamIdentifier(responseID)
	if requestID == "" && responseID == "" {
		return
	}
	a.evidenceMu.Lock()
	defer a.evidenceMu.Unlock()
	if a.evidenceUnavailable {
		return
	}
	conflict := (requestID != "" && a.upstreamRequestID != "" && requestID != a.upstreamRequestID) ||
		(responseID != "" && a.upstreamResponseID != "" && responseID != a.upstreamResponseID)
	changed := false
	if requestID != "" && a.upstreamRequestID == "" {
		a.upstreamRequestID = requestID
		changed = true
	}
	if responseID != "" && a.upstreamResponseID == "" {
		a.upstreamResponseID = responseID
		changed = true
	}
	if conflict && !a.identifierConflict {
		a.identifierConflict = true
		changed = true
	}
	if !changed {
		return
	}
	a.evidenceVersion++
	if a.evidencePending {
		return
	}
	a.evidencePending = true
	if !enqueueIdentifiers(a) {
		a.evidencePending = false
		a.evidenceUnavailable = true
		slog.Error("request_ledger_identifiers_queue_full")
	}
}

func (a *Attempt) persistIdentifiers() {
	for {
		a.evidenceMu.Lock()
		requestID, responseID, conflict, version := a.upstreamRequestID, a.upstreamResponseID, a.identifierConflict, a.evidenceVersion
		a.evidenceMu.Unlock()
		// A locked diagnostic row is a gap, never a reason to occupy a database
		// connection for the full business deadline. No retries of dropped evidence.
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		stats := a.handle.ledger.db.Stats()
		if stats.MaxOpenConnections > 0 && stats.InUse >= stats.MaxOpenConnections-1 {
			cancel()
			a.evidenceMu.Lock()
			a.evidenceUnavailable = true
			a.evidencePending = false
			a.evidenceMu.Unlock()
			slog.Error("request_ledger_identifiers_pool_busy")
			return
		}
		result, err := a.handle.ledger.db.ExecContext(ctx, `UPDATE gateway_request_attempt_evidence SET
 identifier_conflict=identifier_conflict OR $5 OR (upstream_request_id IS NOT NULL AND $3<>'' AND upstream_request_id<>$3)
 OR (upstream_response_id IS NOT NULL AND $4<>'' AND upstream_response_id<>$4),
 upstream_request_id=COALESCE(upstream_request_id,NULLIF($3,'')),
 upstream_response_id=COALESCE(upstream_response_id,NULLIF($4,''))
 WHERE (request_id,attempt_no) IN (SELECT request_id,attempt_no FROM gateway_request_attempt_evidence
 WHERE request_id=$1 AND attempt_no=$2 FOR UPDATE SKIP LOCKED)`, a.handle.ID, a.Number, requestID, responseID, conflict)
		cancel()
		var affected int64
		if err == nil {
			affected, err = result.RowsAffected()
		}
		a.evidenceMu.Lock()
		if err != nil || affected != 1 {
			a.evidenceUnavailable = true
			a.evidencePending = false
			a.evidenceMu.Unlock()
			slog.Error("request_ledger_identifiers_failed")
			return
		}
		if version == a.evidenceVersion {
			a.evidencePending = false
			a.evidenceMu.Unlock()
			return
		}
		a.evidenceMu.Unlock()
	}
}

// LinkOps is called only after the original Ops INSERT commits. Ownership is
// rechecked against the persisted row; no timestamp/client/upstream ID matching.
// A failure is diagnostic only, and may not undo the Ops insert or billing.
func LinkOps(ctx context.Context, db *sql.DB, opsID int64, receipts []Receipt) {
	if db == nil || opsID <= 0 || len(receipts) == 0 {
		return
	}
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	seen := make(map[Correlation]bool)
	for _, receipt := range receipts {
		c := receipt.Correlation()
		if c == nil || seen[*c] {
			continue
		}
		seen[*c] = true
		_, err := db.ExecContext(writeCtx, `INSERT INTO gateway_request_ops_links(request_id,attempt_no,ops_error_log_id)
 SELECT r.id,$2,o.id FROM gateway_requests r JOIN ops_error_logs o ON o.id=$3
 AND o.user_id IS NOT DISTINCT FROM r.user_id AND o.api_key_id IS NOT DISTINCT FROM r.api_key_id
 WHERE r.id=$1 AND ($2=0 OR EXISTS(SELECT 1 FROM gateway_request_attempt_evidence e
 WHERE e.request_id=r.id AND e.attempt_no=$2 AND e.attempt_id::text=$4))
 ON CONFLICT DO NOTHING`, c.RequestID, c.AttemptNo, opsID, c.AttemptID)
		if err != nil {
			slog.Error("request_ledger_ops_link_failed")
			return
		}
	}
}

type OpsReference struct {
	Correlation
	OpsErrorLogID int64 `json:"ops_error_log_id"`
}

type DiagnosticReference struct {
	AttemptNo     int   `json:"attempt_no"`
	OpsErrorLogID int64 `json:"ops_error_log_id"`
	Available     bool  `json:"available"`
}

func (l *Ledger) Diagnostics(ctx context.Context, viewer Viewer, requestID string) ([]DiagnosticReference, error) {
	if !viewer.Admin {
		return nil, nil
	}
	page, err := l.DiagnosticsPage(ctx, viewer, requestID, "", MaxDetailPageSize)
	if err != nil {
		return nil, err
	}
	if page.HasMore {
		return nil, ErrDetailPaginationRequired
	}
	return page.Items, nil
}

func OpsReferences(ctx context.Context, db *sql.DB, opsID int64) ([]OpsReference, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT l.request_id,l.attempt_no,COALESCE(e.attempt_id::text,''),l.ops_error_log_id
 FROM gateway_request_ops_links l LEFT JOIN gateway_request_attempt_evidence e USING(request_id,attempt_no)
 WHERE l.ops_error_log_id=$1 ORDER BY l.request_id,l.attempt_no`, opsID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	refs := []OpsReference{}
	for rows.Next() {
		var ref OpsReference
		if err := rows.Scan(&ref.RequestID, &ref.AttemptNo, &ref.AttemptID, &ref.OpsErrorLogID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
