package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const IdentifierEvidencePageSize = 128

type IdentifierAttemptEvidence struct {
	AttemptNo          int                         `json:"attempt_no"`
	AttemptID          *string                     `json:"attempt_id"`
	IdentifierConflict *bool                       `json:"identifier_conflict"`
	Status             string                      `json:"status"`
	Version            string                      `json:"version,omitempty"`
	Request            IdentifierNamespaceEvidence `json:"request"`
	Response           IdentifierNamespaceEvidence `json:"response"`
}

type IdentifierEvidencePage struct {
	RequestID        string                      `json:"request_id"`
	Completeness     string                      `json:"completeness"`
	Attempts         []IdentifierAttemptEvidence `json:"attempts"`
	HasMore          bool                        `json:"has_more"`
	NextAfterAttempt *int                        `json:"next_after_attempt"`
}

// IdentifierEvidence is an independent administrator read. A saved snapshot is
// never proof of complete observation: a later update may have been dropped or
// the process may have died. Existing detail readers do not depend on this query.
func (l *Ledger) IdentifierEvidence(ctx context.Context, viewer Viewer, requestID string, afterAttempt int) (*IdentifierEvidencePage, error) {
	if !viewer.Admin {
		return nil, errors.New("ledger administrator access required")
	}
	if afterAttempt < 0 {
		return nil, errors.New("invalid attempt cursor")
	}
	if _, err := l.Get(ctx, viewer, requestID); err != nil {
		return nil, err
	}
	ctx, cancel := optionalEvidenceContext(ctx)
	defer cancel()
	rows, err := l.db.QueryContext(ctx, `WITH page AS (
 SELECT attempt_no FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no>$2 ORDER BY attempt_no LIMIT $4
 ) SELECT a.attempt_no,e.attempt_id,e.identifier_conflict,
 e.upstream_request_id,e.upstream_response_id,
 CASE WHEN e.attempt_id IS NULL THEN NULL
 WHEN octet_length(e.identifier_diagnostics::text)<=$3 THEN e.identifier_diagnostics::text ELSE '' END
 FROM page a LEFT JOIN gateway_request_attempt_evidence e
 ON e.request_id=$1 AND e.attempt_no=a.attempt_no ORDER BY a.attempt_no`, requestID, afterAttempt, identifierSnapshotMaxBytes, IdentifierEvidencePageSize+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	page := &IdentifierEvidencePage{RequestID: requestID, Completeness: "unknown", Attempts: make([]IdentifierAttemptEvidence, 0)}
	for rows.Next() {
		var item IdentifierAttemptEvidence
		var requestID, responseID *string
		var raw sql.NullString
		if err := rows.Scan(&item.AttemptNo, &item.AttemptID, &item.IdentifierConflict, &requestID, &responseID, &raw); err != nil {
			return nil, err
		}
		if len(page.Attempts) == IdentifierEvidencePageSize {
			page.HasMore = true
			last := page.Attempts[len(page.Attempts)-1].AttemptNo
			page.NextAfterAttempt = &last
			break
		}
		item.readSnapshot(raw, requestID, responseID)
		page.Attempts = append(page.Attempts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return page, nil
}

func (item *IdentifierAttemptEvidence) readSnapshot(raw sql.NullString, requestID, responseID *string) {
	item.Status = "not_recorded"
	if !raw.Valid || raw.String == "{}" {
		return
	}
	item.Status = "incomplete"
	var snapshot identifierSnapshot
	if len(raw.String) == 0 || len(raw.String) > identifierSnapshotMaxBytes || json.Unmarshal([]byte(raw.String), &snapshot) != nil {
		return
	}
	if snapshot.Version == "" {
		return
	}
	if snapshot.Version != IdentifierSnapshotVersion {
		item.Status = "unsupported_version"
		return
	}
	if item.AttemptID == nil || item.IdentifierConflict == nil || snapshot.AttemptID != *item.AttemptID ||
		!validIdentifierNamespace(snapshot.Request, requestID) || !validIdentifierNamespace(snapshot.Response, responseID) {
		return
	}
	conflictObserved := snapshot.Request.FirstDifferent != nil || snapshot.Response.FirstDifferent != nil
	if conflictObserved != *item.IdentifierConflict {
		return
	}
	item.Status, item.Version = "persisted_snapshot", IdentifierSnapshotVersion
	item.Request, item.Response = snapshot.Request, snapshot.Response
}

func validIdentifierNamespace(namespace IdentifierNamespaceEvidence, stored *string) bool {
	if namespace.First == nil {
		return namespace.FirstDifferent == nil && stored == nil
	}
	if stored == nil || namespace.First.Value != *stored || !validIdentifierValue(namespace.First) {
		return false
	}
	return namespace.FirstDifferent == nil || (validIdentifierValue(namespace.FirstDifferent) && namespace.FirstDifferent.Value != namespace.First.Value)
}

func validIdentifierValue(value *IdentifierValueEvidence) bool {
	return value != nil && value.Value != "" && UpstreamIdentifier(value.Value) == value.Value &&
		identifierSource(value.Source) == value.Source && identifierField(value.Field) == value.Field &&
		identifierEventType(value.EventType) == value.EventType && value.ObservedAtUnixMS > 0
}
