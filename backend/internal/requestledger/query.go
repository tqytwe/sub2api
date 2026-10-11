package requestledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Viewer struct {
	UserID int64
	Admin  bool
}
type Filter struct {
	Page, PageSize                                         int
	UserID, KeyID, AccountID                               int64
	PrivateID, ExecutionState, UsageState, SettlementState string
	Start, End                                             *time.Time
}
type Record struct {
	ID              string     `json:"id"`
	ParentID        *string    `json:"parent_id"`
	Kind            string     `json:"kind"`
	TurnNo          *int       `json:"turn_no"`
	Route           string     `json:"route"`
	Method          string     `json:"method"`
	UserID          *int64     `json:"user_id"`
	APIKeyID        *int64     `json:"api_key_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	ExecutionState  string     `json:"execution_state"`
	UsageState      string     `json:"usage_state"`
	SettlementState string     `json:"settlement_state"`
	ErrorCode       string     `json:"error_code"`
	HTTPStatus      *int       `json:"http_status"`
	AttemptCount    int        `json:"attempt_count"`
	OutputObserved  bool       `json:"output_observed"`
}
type Page struct {
	Items    []Record `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}
type AttemptRecord struct {
	AttemptID           *string    `json:"attempt_id"`
	UpstreamRequestID   *string    `json:"upstream_request_id,omitempty"`
	UpstreamResponseID  *string    `json:"upstream_response_id,omitempty"`
	IdentifierConflict  *bool      `json:"identifier_conflict,omitempty"`
	UsageState          string     `json:"usage_state"`
	OutputObserved      bool       `json:"output_observed"`
	UpstreamKind        string     `json:"upstream_kind"`
	Phase               string     `json:"phase"`
	Number              int        `json:"attempt_no"`
	AccountID           *int64     `json:"account_id,omitempty"`
	CredentialAccountID *int64     `json:"credential_account_id,omitempty"`
	StartedAt           time.Time  `json:"started_at"`
	EndedAt             *time.Time `json:"ended_at"`
	ExecutionState      string     `json:"execution_state"`
	HTTPStatus          *int       `json:"http_status"`
	ErrorCode           string     `json:"error_code"`
}

func whereFor(viewer Viewer, f Filter) (string, []any, error) {
	if !viewer.Admin && viewer.UserID <= 0 {
		return "", nil, errors.New("ledger authentication required")
	}
	parts := []string{"TRUE"}
	args := []any{}
	add := func(expression string, value any) {
		args = append(args, value)
		parts = append(parts, fmt.Sprintf(expression, len(args)))
	}
	if !viewer.Admin {
		add("r.user_id=$%d", viewer.UserID)
	} else if f.UserID > 0 {
		add("r.user_id=$%d", f.UserID)
	}
	if f.KeyID > 0 {
		add("r.api_key_id=$%d", f.KeyID)
	}
	if f.AccountID > 0 && viewer.Admin {
		args = append(args, f.AccountID)
		n := len(args)
		parts = append(parts, fmt.Sprintf("EXISTS(SELECT 1 FROM gateway_request_attempts a WHERE a.request_id=r.id AND (a.account_id=$%d OR a.credential_account_id=$%d))", n, n))
	}
	if f.PrivateID != "" {
		add("r.id=$%d::uuid", f.PrivateID)
	}
	if f.ExecutionState != "" {
		add("r.execution_state=$%d", f.ExecutionState)
	}
	if f.UsageState != "" {
		add("r.usage_state=$%d", f.UsageState)
	}
	if f.SettlementState != "" {
		add("r.settlement_state=$%d", f.SettlementState)
	}
	if f.Start != nil {
		add("r.started_at>=$%d", *f.Start)
	}
	if f.End != nil {
		add("r.started_at<$%d", *f.End)
	}
	return strings.Join(parts, " AND "), args, nil
}

func (l *Ledger) List(ctx context.Context, viewer Viewer, f Filter) (*Page, error) {
	where, args, err := whereFor(viewer, f)
	if err != nil {
		return nil, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Page > 10000 {
		return nil, errors.New("ledger page out of range")
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	page := &Page{Items: []Record{}, Page: f.Page, PageSize: f.PageSize}
	if err = l.db.QueryRowContext(ctx, "SELECT count(*) FROM gateway_requests r WHERE "+where, args...).Scan(&page.Total); err != nil {
		return nil, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := l.db.QueryContext(ctx, fmt.Sprintf("SELECT row_to_json(r) FROM gateway_requests r WHERE %s ORDER BY r.started_at DESC,r.id DESC LIMIT $%d OFFSET $%d", where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var data []byte
		var r Record
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &r); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, r)
	}
	return page, rows.Err()
}

func (l *Ledger) Get(ctx context.Context, viewer Viewer, id string) (*Record, error) {
	where, args, err := whereFor(viewer, Filter{PrivateID: id})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	var data []byte
	if err = l.db.QueryRowContext(ctx, "SELECT row_to_json(r) FROM gateway_requests r WHERE "+where, args...).Scan(&data); err != nil {
		return nil, err
	}
	var record Record
	if err = json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (l *Ledger) Attempts(ctx context.Context, viewer Viewer, id string) ([]AttemptRecord, error) {
	attempts, _, err := l.AttemptsWithEvidence(ctx, viewer, id)
	return attempts, err
}

// AttemptsWithEvidence keeps optional diagnostics independent from the original
// ledger rows. A diagnostic read failure never hides execution or billing.
func (l *Ledger) AttemptsWithEvidence(ctx context.Context, viewer Viewer, id string) ([]AttemptRecord, bool, error) {
	page, err := l.AttemptsPage(ctx, viewer, id, "", MaxDetailPageSize)
	if err != nil {
		return nil, false, err
	}
	if page.HasMore {
		return nil, false, ErrDetailPaginationRequired
	}
	return page.Items, page.EvidenceUnavailable, nil
}

func (l *Ledger) coreAttemptsPage(ctx context.Context, viewer Viewer, id string, after int64, limit int) ([]AttemptRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	rows, err := l.db.QueryContext(ctx, `SELECT to_jsonb(a) FROM gateway_request_attempts a WHERE a.request_id=$1 AND a.attempt_no>$2 ORDER BY a.attempt_no LIMIT $3`, id, after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []AttemptRecord{}
	for rows.Next() {
		var data []byte
		var a AttemptRecord
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if !viewer.Admin {
			a.AccountID = nil
			a.CredentialAccountID = nil
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (l *Ledger) attemptEvidencePage(ctx context.Context, id string, numbers []int) (map[int]AttemptRecord, error) {
	ctx, cancel := optionalEvidenceContext(ctx)
	defer cancel()
	rows, err := l.db.QueryContext(ctx, `SELECT attempt_no,attempt_id,upstream_request_id,upstream_response_id,identifier_conflict FROM gateway_request_attempt_evidence WHERE request_id=$1 AND attempt_no=ANY($2)`, id, pq.Array(numbers))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make(map[int]AttemptRecord)
	for rows.Next() {
		var a AttemptRecord
		if err = rows.Scan(&a.Number, &a.AttemptID, &a.UpstreamRequestID, &a.UpstreamResponseID, &a.IdentifierConflict); err != nil {
			return nil, err
		}
		result[a.Number] = a
	}
	return result, rows.Err()
}

type BillingReference struct {
	WalletTransactionID  *int64   `json:"wallet_transaction_id"`
	UsageLogID           *int64   `json:"usage_log_id"`
	SubscriptionID       *int64   `json:"subscription_id"`
	PackageEntitlementID *int64   `json:"package_entitlement_id"`
	Applied              bool     `json:"applied"`
	Settled              bool     `json:"settled"`
	BilledCost           *float64 `json:"billed_cost"`
	// BatchID means shared task settlement, not a charge applied by this request.
	BatchID *string `json:"batch_id,omitempty"`
}

const billingReferencesSelect = `SELECT u.id,b.subscription_id,b.package_entitlement_id,b.settlement_verified,b.settlement_applied,
 (SELECT bt.id FROM balance_transactions bt JOIN gateway_requests owner ON owner.id=b.request_id
 WHERE bt.user_id=owner.user_id AND (
 (b.batch_id IS NULL AND bt.source_type='usage_charge' AND bt.source_id=b.billing_request_id
 AND bt.idempotency_key='usage_billing:'||b.api_key_id::text||':'||b.billing_request_id)
 OR (b.batch_id IS NOT NULL AND bt.source_type='image_balance_capture' AND bt.source_id=b.batch_id
 AND bt.idempotency_key='image_balance_capture:'||b.api_key_id::text||':'||b.billing_request_id))),
 CASE WHEN b.settlement_verified AND u.billing_settled THEN u.billed_cost ELSE NULL END,b.batch_id
 FROM usage_evidence b LEFT JOIN usage_logs u ON u.request_id=b.billing_request_id AND u.api_key_id=b.api_key_id
 AND u.billing_request_fingerprint=b.request_fingerprint WHERE b.request_id=$1`

func scanBillingReference(row interface{ Scan(...any) error }, r *BillingReference) error {
	return row.Scan(&r.UsageLogID, &r.SubscriptionID, &r.PackageEntitlementID, &r.Settled, &r.Applied, &r.WalletTransactionID, &r.BilledCost, &r.BatchID)
}

func (l *Ledger) BillingReferences(ctx context.Context, viewer Viewer, id string) ([]BillingReference, error) {
	page, err := l.BillingPage(ctx, viewer, id, "", MaxDetailPageSize)
	if err != nil {
		return nil, err
	}
	if page.HasMore {
		return nil, ErrDetailPaginationRequired
	}
	return page.Items, nil
}

// LinkedUsage is a read of an existing usage row, never a copied/reconstructed log.
type LinkedUsage struct {
	ID           int64     `json:"id"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	ImageCount   *int64    `json:"image_count"`
	BillingMode  *string   `json:"billing_mode"`
	BatchID      *string   `json:"batch_id"`
	CreatedAt    time.Time `json:"created_at"`
	BilledCost   *float64  `json:"billed_cost"`
}

func (l *Ledger) Usage(ctx context.Context, viewer Viewer, requestID string, usageID int64) (*LinkedUsage, error) {
	if _, err := l.Get(ctx, viewer, requestID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	var u LinkedUsage
	err := l.db.QueryRowContext(ctx, usageEvidence+`SELECT u.id,u.model,u.input_tokens,u.output_tokens,u.image_count,u.billing_mode,b.batch_id,u.created_at,
 CASE WHEN b.settlement_verified AND u.billing_settled THEN u.billed_cost ELSE NULL END
 FROM usage_evidence b JOIN gateway_requests r ON r.id=b.request_id
 JOIN usage_logs u ON u.request_id=b.billing_request_id AND u.api_key_id=b.api_key_id AND u.billing_request_fingerprint=b.request_fingerprint
 WHERE b.request_id=$1 AND u.id=$2`, requestID, usageID).Scan(&u.ID, &u.Model, &u.InputTokens, &u.OutputTokens, &u.ImageCount, &u.BillingMode, &u.BatchID, &u.CreatedAt, &u.BilledCost)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Optional metadata cannot consume the full HTTP request budget after core reads.
func optionalEvidenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	budget := 100 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline) / 4; remaining < budget {
			budget = remaining
		}
	}
	return context.WithTimeout(ctx, budget)
}
