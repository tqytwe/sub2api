package requestledger

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

const MaxDetailPageSize = 100

var (
	ErrDetailPaginationRequired = errors.New("request detail requires pagination")
	ErrDetailCursor             = errors.New("invalid request detail cursor")
)

type DetailPage[T any] struct {
	Items               []T    `json:"items"`
	PageSize            int    `json:"page_size"`
	HasMore             bool   `json:"has_more"`
	NextCursor          string `json:"next_cursor"`
	EvidenceUnavailable bool   `json:"attempt_evidence_unavailable,omitempty"`
	Consistency         string `json:"consistency"`
	RefreshHint         string `json:"refresh_hint"`
}

// A position identifies a scanned source, including sources that did not pass
// the existing evidence checks. It is never an authorization credential.
type detailPosition struct {
	Source    string `json:"s,omitempty"`
	Key       string `json:"k,omitempty"`
	Number    int64  `json:"n,omitempty"`
	Operation string `json:"o,omitempty"`
}

type detailCursor struct {
	Version   int            `json:"v"`
	RequestID string         `json:"r"`
	Section   string         `json:"c"`
	UserID    int64          `json:"u"`
	Admin     bool           `json:"a"`
	Position  detailPosition `json:"p"`
}

func parseDetailCursor(viewer Viewer, id, section, raw string, size int) (detailPosition, error) {
	if size < 1 || size > MaxDetailPageSize || len(raw) > 2048 {
		return detailPosition{}, ErrDetailCursor
	}
	if raw == "" {
		return detailPosition{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return detailPosition{}, ErrDetailCursor
	}
	var c detailCursor
	if json.Unmarshal(data, &c) != nil || c.Version != 1 || c.RequestID != id || c.Section != section || c.UserID != viewer.UserID || c.Admin != viewer.Admin {
		return detailPosition{}, ErrDetailCursor
	}
	p := c.Position
	if p.Number < 0 || len(p.Key) > 256 || strings.IndexByte(p.Key, 0) >= 0 {
		return detailPosition{}, ErrDetailCursor
	}
	switch section {
	case "attempts":
		if p.Number < 1 || p.Number > 2147483647 || p.Source != "" || p.Key != "" || p.Operation != "" {
			return detailPosition{}, ErrDetailCursor
		}
	case "diagnostics":
		if p.Number > 2147483647 || p.Source != "" || p.Key == "" || p.Operation != "" {
			return detailPosition{}, ErrDetailCursor
		}
		if id, err := strconv.ParseInt(p.Key, 10, 64); err != nil || id <= 0 {
			return detailPosition{}, ErrDetailCursor
		}
	case "billing":
		if (p.Source != "link" && p.Source != "task") || p.Key == "" || p.Operation != "" || (p.Source == "link" && p.Number < 1) || (p.Source == "task" && (p.Number != 0 || len(p.Key) > 160)) {
			return detailPosition{}, ErrDetailCursor
		}
	case "wallet":
		if (p.Source != "image_batch" && p.Source != "image_studio" && p.Source != "mobile_video") || p.Key == "" || len(p.Key) > 160 || p.Number < 1 || p.Number > 3 || p.Operation != "" {
			return detailPosition{}, ErrDetailCursor
		}
	default:
		return detailPosition{}, ErrDetailCursor
	}
	return p, nil
}

func (l *Ledger) DiagnosticsPage(ctx context.Context, viewer Viewer, id, cursor string, size int) (*DetailPage[DiagnosticReference], error) {
	if !viewer.Admin {
		return nil, errors.New("administrator access required")
	}
	if _, err := l.Get(ctx, viewer, id); err != nil {
		return nil, err
	}
	position, err := parseDetailCursor(viewer, id, "diagnostics", cursor, size)
	if err != nil {
		return nil, err
	}
	opsID, _ := strconv.ParseInt(position.Key, 10, 64)
	ctx, cancel := optionalEvidenceContext(ctx)
	defer cancel()
	rows, err := l.db.QueryContext(ctx, `SELECT l.attempt_no,l.ops_error_log_id,o.id IS NOT NULL
 FROM gateway_request_ops_links l JOIN gateway_requests r ON r.id=l.request_id
 LEFT JOIN ops_error_logs o ON o.id=l.ops_error_log_id AND o.user_id IS NOT DISTINCT FROM r.user_id
 AND o.api_key_id IS NOT DISTINCT FROM r.api_key_id WHERE l.request_id=$1
 AND (l.attempt_no,l.ops_error_log_id)>($2,$3) ORDER BY l.attempt_no,l.ops_error_log_id LIMIT $4`, id, position.Number, opsID, size+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []DiagnosticReference{}
	positions := []detailPosition{}
	for rows.Next() {
		var d DiagnosticReference
		if err = rows.Scan(&d.AttemptNo, &d.OpsErrorLogID, &d.Available); err != nil {
			return nil, err
		}
		result = append(result, d)
		positions = append(positions, detailPosition{Number: int64(d.AttemptNo), Key: strconv.FormatInt(d.OpsErrorLogID, 10)})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	page := newDetailPage[DiagnosticReference](viewer, id, "diagnostics", size, positions)
	if len(result) > size {
		result = result[:size]
	}
	page.Items = result
	return page, nil
}

func newDetailPage[T any](viewer Viewer, id, section string, size int, candidates []detailPosition) *DetailPage[T] {
	p := &DetailPage[T]{Items: []T{}, PageSize: size, HasMore: len(candidates) > size,
		Consistency: "live", RefreshHint: "new_records_late_evidence_and_state_changes"}
	if p.HasMore {
		data, _ := json.Marshal(detailCursor{Version: 1, RequestID: id, Section: section, UserID: viewer.UserID, Admin: viewer.Admin, Position: candidates[size-1]})
		p.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return p
}

func (l *Ledger) AttemptsPage(ctx context.Context, viewer Viewer, id, cursor string, size int) (*DetailPage[AttemptRecord], error) {
	if _, err := l.Get(ctx, viewer, id); err != nil {
		return nil, err
	}
	position, err := parseDetailCursor(viewer, id, "attempts", cursor, size)
	if err != nil {
		return nil, err
	}
	result, err := l.coreAttemptsPage(ctx, viewer, id, position.Number, size+1)
	if err != nil {
		return nil, err
	}
	positions := make([]detailPosition, len(result))
	for i, a := range result {
		positions[i].Number = int64(a.Number)
	}
	page := newDetailPage[AttemptRecord](viewer, id, "attempts", size, positions)
	if len(result) > size {
		result = result[:size]
	}
	page.Items = result
	numbers := make([]int, len(result))
	for i, a := range result {
		numbers[i] = a.Number
	}
	evidence, err := l.attemptEvidencePage(ctx, id, numbers)
	if err != nil {
		page.EvidenceUnavailable = true
		return page, nil
	}
	for i := range result {
		e := evidence[result[i].Number]
		result[i].AttemptID = e.AttemptID
		if viewer.Admin {
			result[i].UpstreamRequestID = e.UpstreamRequestID
			result[i].UpstreamResponseID = e.UpstreamResponseID
			result[i].IdentifierConflict = e.IdentifierConflict
		}
	}
	return page, nil
}

const billingTaskPageCandidates = `SELECT task_ref FROM gateway_request_tasks
 WHERE request_id=$1 AND task_kind='image_batch' AND task_ref>$2 ORDER BY task_ref LIMIT $3`

func (l *Ledger) billingCandidates(ctx context.Context, id string, after detailPosition, size int) ([]detailPosition, error) {
	result := []detailPosition{}
	if after.Source != "task" {
		rows, err := l.db.QueryContext(ctx, `SELECT billing_request_id,api_key_id FROM gateway_request_billing_links
 WHERE request_id=$1 AND (billing_request_id,api_key_id)>($2,$3)
 ORDER BY billing_request_id,api_key_id LIMIT $4`, id, after.Key, after.Number, size+1)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			p := detailPosition{Source: "link"}
			if err = rows.Scan(&p.Key, &p.Number); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result = append(result, p)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil || len(result) > size {
			return result, err
		}
		after = detailPosition{}
	}
	rows, err := l.db.QueryContext(ctx, billingTaskPageCandidates, id, after.Key, size+1-len(result))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		p := detailPosition{Source: "task"}
		if err = rows.Scan(&p.Key); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

const walletTaskPageCandidates = `WITH tasks AS MATERIALIZED (
 SELECT task_kind,task_ref FROM gateway_request_tasks WHERE request_id=$1
 AND task_kind IN ('image_batch','image_studio','mobile_video') AND (task_kind,task_ref)>=($2,$3)
 ORDER BY task_kind,task_ref LIMIT $5)
 SELECT task_kind,task_ref,operation FROM tasks CROSS JOIN (VALUES (1),(2),(3)) AS op(operation)
 WHERE (task_kind,task_ref,operation)>($2,$3,$4) ORDER BY task_kind,task_ref,operation LIMIT $5`

func (l *Ledger) walletCandidates(ctx context.Context, id string, after detailPosition, size int) ([]detailPosition, error) {
	// Limit tasks before expanding their three possible immutable operations.
	// The inclusive task boundary retains unscanned operations of the last task.
	rows, err := l.db.QueryContext(ctx, walletTaskPageCandidates, id, after.Source, after.Key, after.Number, size+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []detailPosition{}
	for rows.Next() {
		var p detailPosition
		if err = rows.Scan(&p.Source, &p.Key, &p.Number); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (l *Ledger) BillingPage(ctx context.Context, viewer Viewer, id, cursor string, size int) (*DetailPage[BillingReference], error) {
	if _, err := l.Get(ctx, viewer, id); err != nil {
		return nil, err
	}
	position, err := parseDetailCursor(viewer, id, "billing", cursor, size)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	candidates, err := l.billingCandidates(ctx, id, position, size)
	if err != nil {
		return nil, err
	}
	page := newDetailPage[BillingReference](viewer, id, "billing", size, candidates)
	if len(candidates) > size {
		candidates = candidates[:size]
	}
	if len(candidates) == 0 {
		return page, nil
	}
	links, keys, tasks := []string{}, []int64{}, []string{}
	for _, c := range candidates {
		if c.Source == "link" {
			links = append(links, c.Key)
			keys = append(keys, c.Number)
		} else {
			tasks = append(tasks, c.Key)
		}
	}
	// Source restriction precedes proof evaluation. All certification predicates
	// come from the same retained batch evidence as the existing usage drill.
	evidence := `WITH selected_links AS (SELECT * FROM unnest($2::text[],$3::bigint[]) AS s(request_id,key_id)),
 selected_tasks AS (SELECT unnest($4::text[]) AS task_ref), usage_evidence AS (
 SELECT b.request_id,b.billing_request_id,b.api_key_id,b.request_fingerprint,b.subscription_id,b.package_entitlement_id,
 b.settlement_verified,b.settlement_applied,NULL::text AS batch_id,b.created_at
 FROM selected_links s JOIN gateway_request_billing_links b ON b.billing_request_id=s.request_id AND b.api_key_id=s.key_id WHERE b.request_id=$1
 UNION ALL SELECT t.request_id,u.request_id,t.api_key_id,u.billing_request_fingerprint,NULL::bigint,NULL::bigint,TRUE,FALSE,t.task_ref,t.created_at ` +
		strings.Replace(batchCaptureUsageEvidence, "FROM gateway_request_tasks t", "FROM selected_tasks s JOIN gateway_request_tasks t ON t.task_ref=s.task_ref", 1) + `
 AND t.request_id=$1 AND NOT EXISTS(SELECT 1 FROM gateway_request_billing_links b
 WHERE b.request_id=t.request_id AND b.billing_request_id=u.request_id AND b.api_key_id=u.api_key_id)) `
	rows, err := l.db.QueryContext(ctx, evidence+billingReferencesSelect+` ORDER BY b.batch_id IS NOT NULL,b.billing_request_id,b.api_key_id`, id, pq.Array(links), pq.Array(keys), pq.Array(tasks))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var r BillingReference
		if err = scanBillingReference(rows, &r); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, r)
	}
	return page, rows.Err()
}

func (l *Ledger) WalletPage(ctx context.Context, viewer Viewer, id, cursor string, size int) (*DetailPage[WalletReference], error) {
	if _, err := l.Get(ctx, viewer, id); err != nil {
		return nil, err
	}
	position, err := parseDetailCursor(viewer, id, "wallet", cursor, size)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	candidates, err := l.walletCandidates(ctx, id, position, size)
	if err != nil {
		return nil, err
	}
	page := newDetailPage[WalletReference](viewer, id, "wallet", size, candidates)
	if len(candidates) > size {
		candidates = candidates[:size]
	}
	if len(candidates) == 0 {
		return page, nil
	}
	kinds, refs, operations := []string{}, []string{}, []string{}
	for _, c := range candidates {
		kinds = append(kinds, c.Source)
		refs = append(refs, c.Key)
		prefix := "image_balance_"
		if c.Source == "mobile_video" {
			prefix = "video_balance_"
		}
		operations = append(operations, prefix+[]string{"hold", "capture", "release"}[c.Number-1])
	}
	evidence := strings.Replace(taskWalletEvidence, "FROM gateway_request_tasks t", "FROM selected s JOIN gateway_request_tasks t ON t.task_kind=s.kind AND t.task_ref=s.ref", 1)
	rows, err := l.db.QueryContext(ctx, `WITH selected AS (SELECT * FROM unnest($2::text[],$3::text[],$4::text[]) WITH ORDINALITY AS s(kind,ref,operation,ordinal))
 SELECT wallet.id,wallet.source_type FROM gateway_requests r CROSS JOIN LATERAL
 (SELECT bt.id,bt.source_type,s.ordinal `+evidence+` AND bt.source_type=s.operation) wallet WHERE r.id=$1 ORDER BY wallet.ordinal`, id, pq.Array(kinds), pq.Array(refs), pq.Array(operations))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ref WalletReference
		if err = rows.Scan(&ref.ID, &ref.Operation); err != nil {
			return nil, err
		}
		page.Items = append(page.Items, ref)
	}
	return page, rows.Err()
}
