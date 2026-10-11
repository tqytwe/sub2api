package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrRoutingForbidden = errors.New("routing evidence requires administrator access")

type RoutingEvidence struct {
	RequestID       string `json:"request_id"`
	CoverageVersion string `json:"coverage_version"`
	RoutingSummary
	Events []RoutingEvent `json:"events"`
}

// RoutingEvidence is deliberately separate from Get/Attempts: optional routing
// storage failure must never make the existing request detail unavailable.
func (l *Ledger) RoutingEvidence(ctx context.Context, viewer Viewer, id string) (*RoutingEvidence, error) {
	if !viewer.Admin || viewer.UserID <= 0 {
		return nil, ErrRoutingForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	result := &RoutingEvidence{RequestID: id, Events: []RoutingEvent{}}
	var summary, events []byte
	var ended sql.NullTime
	var state, route, method, kind string
	var expired, pastPending bool
	var stored sql.NullInt64
	err := l.db.QueryRowContext(ctx, `SELECT r.routing_coverage_version,r.routing_summary,r.ended_at,r.execution_state,r.route,r.method,r.kind,
 i.lease_until<clock_timestamp(),COALESCE(r.ended_at<clock_timestamp()-($2 * interval '1 second'),false),d.events,d.event_count
 FROM gateway_requests r JOIN gateway_ledger_instances i ON i.id=r.instance_id
 LEFT JOIN gateway_request_routing d ON d.request_id=r.id WHERE r.id=$1::uuid`, id, int64(routingPendingWindow/time.Second)).Scan(&result.CoverageVersion, &summary, &ended, &state, &route, &method, &kind, &expired, &pastPending, &events, &stored)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(summary, &result.RoutingSummary); err != nil {
		return nil, ErrUnavailable
	}
	if result.CoverageVersion == "" {
		result.Status = "not_recorded"
		return result, nil
	}
	if !routingEligible(route, method, kind) {
		result.Status = "unsupported_coverage"
		return result, nil
	}
	if len(events) > 0 {
		if err = json.Unmarshal(events, &result.Events); err != nil {
			return nil, ErrUnavailable
		}
	}
	if result.Status == "not_recorded" || result.Status == "unsupported_coverage" {
		return result, nil
	}
	// An inflight/crashed request has no authoritative end summary, even if a
	// batch happened to reach storage before its Finish write failed.
	if !ended.Valid || state == "interrupted" && result.SelectionCount == 0 {
		result.Status = "pending"
		if expired || state == "interrupted" {
			result.Status = "incomplete"
			result.Incomplete = true
		}
		return result, nil
	}
	if result.Incomplete || result.Status == "incomplete" {
		result.Status = "incomplete"
		result.Incomplete = true
		return result, nil
	}
	if !stored.Valid || int(stored.Int64) != result.EventCount || len(result.Events) != result.EventCount {
		result.Status = "pending"
		if pastPending {
			result.Status = "incomplete"
			result.Incomplete = true
		}
		return result, nil
	}
	result.Status = "complete"
	if result.OmittedCount > 0 {
		result.Status = "truncated"
	}
	return result, nil
}
