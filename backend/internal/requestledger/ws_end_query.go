package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const WSEndEvidencePageSize = 128

type WSEndAttemptEvidence struct {
	AttemptNo int            `json:"attempt_no"`
	AttemptID *string        `json:"attempt_id"`
	Phase     string         `json:"phase"`
	Status    string         `json:"status"`
	Snapshot  *wsEndSnapshot `json:"snapshot"`
}

type WSEndEvidencePage struct {
	RequestID        string                 `json:"request_id"`
	Completeness     string                 `json:"completeness"`
	Attempts         []WSEndAttemptEvidence `json:"attempts"`
	HasMore          bool                   `json:"has_more"`
	NextAfterAttempt *int                   `json:"next_after_attempt"`
}

// WSEndEvidence is optional and independent of the request detail response.
// Empty evidence cannot distinguish old writers, pending work, dropped work or
// an uncovered protocol. Even a saved finish is not full protocol/billing proof.
func (l *Ledger) WSEndEvidence(ctx context.Context, viewer Viewer, requestID string, afterAttempt int) (*WSEndEvidencePage, error) {
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
 SELECT attempt_no,phase FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no>$2 ORDER BY attempt_no LIMIT $4
 ) SELECT a.attempt_no,a.phase,e.attempt_id,
 CASE WHEN w.attempt_id IS NULL THEN NULL
 WHEN w.attempt_id IS DISTINCT FROM e.attempt_id THEN ''
 WHEN octet_length(w.snapshot::text)<=$3 THEN w.snapshot::text ELSE '' END
 FROM page a LEFT JOIN gateway_request_attempt_evidence e
 ON e.request_id=$1 AND e.attempt_no=a.attempt_no
 LEFT JOIN gateway_request_ws_end_evidence w ON w.request_id=$1 AND w.attempt_no=a.attempt_no
 ORDER BY a.attempt_no`, requestID, afterAttempt, wsEndSnapshotMaxBytes, WSEndEvidencePageSize+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	page := &WSEndEvidencePage{RequestID: requestID, Completeness: "unknown", Attempts: make([]WSEndAttemptEvidence, 0)}
	for rows.Next() {
		var item WSEndAttemptEvidence
		var raw sql.NullString
		if err := rows.Scan(&item.AttemptNo, &item.Phase, &item.AttemptID, &raw); err != nil {
			return nil, err
		}
		if len(page.Attempts) == WSEndEvidencePageSize {
			page.HasMore = true
			last := page.Attempts[len(page.Attempts)-1].AttemptNo
			page.NextAfterAttempt = &last
			break
		}
		item.readSnapshot(raw)
		page.Attempts = append(page.Attempts, item)
	}
	return page, rows.Err()
}

func (item *WSEndAttemptEvidence) readSnapshot(raw sql.NullString) {
	item.Status, item.Snapshot = "not_recorded", nil
	if !raw.Valid || raw.String == "{}" {
		return
	}
	item.Status = "incomplete"
	var snapshot wsEndSnapshot
	if len(raw.String) == 0 || len(raw.String) > wsEndSnapshotMaxBytes || json.Unmarshal([]byte(raw.String), &snapshot) != nil || snapshot.Version == "" {
		return
	}
	if snapshot.Version != WSEndSnapshotVersion || (snapshot.CoverageVersion != "" && snapshot.CoverageVersion != WSEndCoverageVersion) {
		item.Status = "unsupported_version"
		return
	}
	if item.AttemptID == nil || snapshot.AttemptID != *item.AttemptID || !validWSEndSnapshot(snapshot) {
		return
	}
	item.Status, item.Snapshot = "persisted_snapshot", &snapshot
}

func validWSEndSnapshot(s wsEndSnapshot) bool {
	if !s.ResultObserved && (s.PositiveUnitsObserved || s.Terminal.State == "observed") {
		return false
	}
	if s.CoverageVersion != WSEndCoverageVersion || s.CapturedAtUnixMS <= 0 || s.UsagePresence != "unknown" || s.Terminal.Scope != "normalized_ws_forward_result" ||
		wsEndStage(s.Stage) != s.Stage || wsEndSource(s.StageSource) != s.StageSource ||
		wsEndReason(s.Reason) != s.Reason || wsEndSource(s.ReasonSource) != s.ReasonSource {
		return false
	}
	if s.WroteDownstream == nil {
		if s.WroteDownstreamScope != "unknown" {
			return false
		}
	} else if s.WroteDownstreamScope != "attempt" && s.WroteDownstreamScope != "relay_session" {
		return false
	}
	switch s.Terminal.State {
	case "observed":
		return s.Terminal.Event != "" && wsEndTerminal(s.Terminal.Event) == s.Terminal.Event && s.Terminal.Source == "ws_forward_result"
	case "not_observed_in_scope":
		return s.Terminal.Event == "" && s.Terminal.Source == "relay_joined_unfinished"
	case "unknown":
		return s.Terminal.Event == "" && s.Terminal.Source == "unknown"
	default:
		return false
	}
}
