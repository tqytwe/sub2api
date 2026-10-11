package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

const WSEndSnapshotVersion = "ws_attempt_end_v1"
const WSEndCoverageVersion = "openai_ws_finish_hooks_v1"
const wsEndSnapshotMaxBytes = 4096

// WSEndObservation accepts existing, attributed finish results only. It never
// changes the business error, reads protocol bodies, or infers a billing result.
// JoinedUnfinished is reserved for the authoritative post-join relay exit.
type WSEndObservation struct {
	Stage, StageSource, Reason, ReasonSource string
	TerminalEvent                            string
	JoinedUnfinished                         bool
	ResultObserved, PositiveUnitsObserved    bool
	WroteDownstream                          *bool
	WroteDownstreamScope                     string
}

type WSEndTerminalEvidence struct {
	State  string `json:"state"`
	Event  string `json:"event,omitempty"`
	Source string `json:"source"`
	Scope  string `json:"scope"`
}

type wsEndSnapshot struct {
	Version               string                `json:"version"`
	CoverageVersion       string                `json:"coverage_version"`
	AttemptID             string                `json:"attempt_id"`
	Stage                 string                `json:"stage"`
	StageSource           string                `json:"stage_source"`
	Reason                string                `json:"reason"`
	ReasonSource          string                `json:"reason_source"`
	Terminal              WSEndTerminalEvidence `json:"terminal"`
	UsagePresence         string                `json:"usage_presence"`
	ResultObserved        bool                  `json:"result_observed"`
	PositiveUnitsObserved bool                  `json:"positive_units_observed"`
	WroteDownstream       *bool                 `json:"wrote_downstream"`
	WroteDownstreamScope  string                `json:"wrote_downstream_scope"`
	CapturedAtUnixMS      int64                 `json:"captured_at_unix_ms"`
}

func wsEndStage(value string) string {
	switch value {
	case "dial", "write_upstream", "read_upstream", "read_client", "write_client", "upstream_message", "idle_timeout", "relay_init", "turn_complete", "cleanup",
		"client_disconnected", "read_event", "policy_violation", "message_too_big", "prewarm_read_event", "prewarm_policy_violation", "prewarm_message_too_big", "write_request", "read", "read_timeout", "read_closed", "prewarm_write", "prewarm_read", "prewarm_read_timeout", "prewarm_read_closed",
		"invalid_event_json", "missing_final_response", "streaming_not_supported", "error_event", "previous_response_not_found", "invalid_encrypted_content", "rate_limit", "prewarm_error_event", "prewarm_ambiguous_response", "prewarm_ambiguous_terminal":
		return value
	default:
		return "unknown"
	}
}

func wsEndSource(value string) string {
	switch value {
	case "dial", "frame_write", "relay_exit", "relay_turn_complete", "cleanup", "ingress_turn_error", "fallback_error", "finish_cause", "context_cause", "ws_forward_result", "relay_joined_unfinished":
		return value
	default:
		return "unknown"
	}
}

func wsEndReason(value string) string {
	switch value {
	case "none", "cancelled", "deadline_exceeded", "timeout", "eof", "unexpected_eof", "websocket_close", "transport_error", "session_preempted", "ingress_lease_lost", "upstream_failed", "upstream_incomplete", "upstream_cancelled", "unfinished_turn", "failover":
		return value
	default:
		return "unknown"
	}
}

func wsEndTerminal(value string) string {
	switch value {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled":
		return value
	default:
		return ""
	}
}

func newWSEndSnapshot(attemptID string, observation WSEndObservation, now time.Time) wsEndSnapshot {
	if !observation.ResultObserved {
		observation.TerminalEvent, observation.PositiveUnitsObserved = "", false
	}
	snapshot := wsEndSnapshot{
		Version: WSEndSnapshotVersion, CoverageVersion: WSEndCoverageVersion, AttemptID: attemptID,
		Stage: wsEndStage(observation.Stage), StageSource: wsEndSource(observation.StageSource),
		Reason: wsEndReason(observation.Reason), ReasonSource: wsEndSource(observation.ReasonSource),
		Terminal:      WSEndTerminalEvidence{State: "unknown", Source: "unknown", Scope: "normalized_ws_forward_result"},
		UsagePresence: "unknown", ResultObserved: observation.ResultObserved, PositiveUnitsObserved: observation.PositiveUnitsObserved,
		WroteDownstreamScope: "unknown", CapturedAtUnixMS: now.UnixMilli(),
	}
	if terminal := wsEndTerminal(observation.TerminalEvent); terminal != "" {
		snapshot.Terminal = WSEndTerminalEvidence{State: "observed", Event: terminal, Source: "ws_forward_result", Scope: "normalized_ws_forward_result"}
	} else if observation.JoinedUnfinished {
		snapshot.Terminal = WSEndTerminalEvidence{State: "not_observed_in_scope", Source: "relay_joined_unfinished", Scope: "normalized_ws_forward_result"}
	}
	if observation.WroteDownstream != nil && (observation.WroteDownstreamScope == "attempt" || observation.WroteDownstreamScope == "relay_session") {
		value := *observation.WroteDownstream
		snapshot.WroteDownstream, snapshot.WroteDownstreamScope = &value, observation.WroteDownstreamScope
	}
	return snapshot
}

// The queue owns only a database handle, private identity and already encoded
// whitelist scalars. No Attempt, request context, error, headers or body survives
// the producer call. Queue loss/process exit remains explicitly unknown on read.
type wsEndWrite struct {
	db                            *sql.DB
	requestID, attemptID, encoded string
	attemptNo                     int
}

var wsEndWrites = struct {
	once  sync.Once
	queue chan wsEndWrite
}{queue: make(chan wsEndWrite, 256)}

func (r Receipt) ObserveWSEnd(observation WSEndObservation) {
	if r.attempt == nil || r.handle == nil || r.handle.ledger == nil || r.handle.ledger.db == nil {
		return
	}
	encoded, err := json.Marshal(newWSEndSnapshot(r.attempt.ID, observation, time.Now()))
	if err != nil || len(encoded) > wsEndSnapshotMaxBytes {
		slog.Error("request_ledger_ws_end_snapshot_invalid")
		return
	}
	wsEndWrites.once.Do(func() {
		go func() {
			for write := range wsEndWrites.queue {
				write.persist()
			}
		}()
	})
	select {
	case wsEndWrites.queue <- wsEndWrite{db: r.handle.ledger.db, requestID: r.handle.ID, attemptID: r.attempt.ID, attemptNo: r.attempt.Number, encoded: string(encoded)}:
	default:
		// A dropped job remains not_recorded/unknown. Never block the producer on logging.
	}
}

func (write wsEndWrite) persist() {
	stats := write.db.Stats()
	if stats.MaxOpenConnections > 0 && stats.InUse >= stats.MaxOpenConnections-1 {
		slog.Error("request_ledger_ws_end_pool_busy")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	// Plain MVCC ownership read: never lock the identifier writer's row. Only
	// this optional table's insert/conflict may wait, bounded by the deadline.
	result, err := write.db.ExecContext(ctx, `INSERT INTO gateway_request_ws_end_evidence(request_id,attempt_no,attempt_id,snapshot)
 SELECT request_id,attempt_no,attempt_id,$4::jsonb FROM gateway_request_attempt_evidence
 WHERE request_id=$1 AND attempt_no=$2 AND attempt_id=$3::uuid
 ON CONFLICT (request_id,attempt_no) DO NOTHING`, write.requestID, write.attemptNo, write.attemptID, write.encoded)
	if err != nil {
		slog.Error("request_ledger_ws_end_write_failed")
		return
	}
	if _, err := result.RowsAffected(); err != nil {
		slog.Error("request_ledger_ws_end_write_failed")
	}
}
