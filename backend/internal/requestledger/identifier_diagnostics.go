package requestledger

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

const IdentifierSnapshotVersion = "identifier_snapshot_v1"
const identifierSnapshotMaxBytes = 4096

// These internal value objects retain only bounded identifier metadata. Sources
// describe capture sites, not inferred causes; timestamps are local observations.
type identifierObservation struct {
	Namespace string
	Value     string
	Source    string
	Field     string
	EventType string
}

type IdentifierValueEvidence struct {
	Value            string `json:"value"`
	Source           string `json:"source"`
	Field            string `json:"field"`
	EventType        string `json:"event_type"`
	ObservedAtUnixMS int64  `json:"observed_at_unix_ms"`
}

type IdentifierNamespaceEvidence struct {
	First          *IdentifierValueEvidence `json:"first,omitempty"`
	FirstDifferent *IdentifierValueEvidence `json:"first_different,omitempty"`
}

type identifierSnapshot struct {
	Version   string                      `json:"version"`
	AttemptID string                      `json:"attempt_id"`
	Request   IdentifierNamespaceEvidence `json:"request"`
	Response  IdentifierNamespaceEvidence `json:"response"`
}

func (s *identifierSnapshot) observe(in identifierObservation, observedAt int64) bool {
	var namespace *IdentifierNamespaceEvidence
	switch in.Namespace {
	case "request":
		namespace = &s.Request
	case "response":
		namespace = &s.Response
	default:
		return false
	}
	value := UpstreamIdentifier(in.Value)
	if value == "" || (namespace.First != nil && (namespace.First.Value == value || namespace.FirstDifferent != nil)) {
		return false
	}
	evidence := &IdentifierValueEvidence{Value: value, Source: identifierSource(in.Source),
		Field: identifierField(in.Field), EventType: identifierEventType(in.EventType), ObservedAtUnixMS: observedAt}
	if namespace.First == nil {
		namespace.First = evidence
	} else {
		namespace.FirstDifferent = evidence
	}
	return true
}

func identifierSource(value string) string {
	switch value {
	case "http_header", "http_sse", "http_json", "ops", "legacy_observer":
		return value
	default:
		return "unknown"
	}
}

func identifierField(value string) string {
	switch value {
	case "x-request-id", "request-id", "x-goog-request-id", "xai-request-id", "x-amzn-requestid",
		"response.id", "response_id", "responseId", "message.id", "id", "upstream_request_id", "upstream_response_id":
		return value
	default:
		return "unknown"
	}
}

func identifierEventType(value string) string {
	switch value {
	case "none", "response.created", "response.in_progress", "response.completed", "response.done",
		"response.failed", "response.incomplete", "response.cancelled", "response.output_item.added",
		"response.output_item.done", "response.content_part.added", "response.content_part.done",
		"response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done",
		"response.function_call_arguments.delta", "response.function_call_arguments.done",
		"message", "message_start", "message_delta", "message_stop", "content_block_start",
		"content_block_delta", "content_block_stop", "error", "ops.http_error", "ops.failover",
		"ops.retry_exhausted_failover", "ops.response_error", "ops.request_error", "ops.stream_failed":
		return value
	default:
		return "unknown"
	}
}

func requestIdentifierObservation(headers http.Header) identifierObservation {
	// Preserve the original header preference and validation exactly.
	for _, name := range []string{"x-request-id", "request-id", "x-goog-request-id", "xai-request-id", "x-amzn-requestid"} {
		if id := UpstreamIdentifier(headers.Get(name)); id != "" {
			return identifierObservation{Namespace: "request", Value: id, Source: "http_header", Field: name, EventType: "none"}
		}
	}
	return identifierObservation{}
}

func responseIdentifierObservation(data []byte, source string) identifierObservation {
	if len(data) > 64*1024 || !gjson.ValidBytes(data) {
		return identifierObservation{}
	}
	paths := []string{"response.id", "response_id", "responseId", "message.id"}
	object, kind := gjson.GetBytes(data, "object").String(), gjson.GetBytes(data, "type").String()
	if object == "response" || object == "chat.completion" || object == "chat.completion.chunk" || kind == "message" {
		paths = append(paths, "id")
	}
	eventType := "none"
	// SSE provenance describes payload.type only. Independent event: lines are
	// not captured here, so a missing payload type is unknown, not absent.
	if source == "http_sse" {
		eventType = "unknown"
	}
	if gjson.GetBytes(data, "type").Exists() {
		eventType = identifierEventType(kind)
	}
	for _, path := range paths {
		value := gjson.GetBytes(data, path)
		if value.Type == gjson.String {
			if id := UpstreamIdentifier(value.String()); id != "" {
				return identifierObservation{Namespace: "response", Value: id, Source: source, Field: path, EventType: eventType}
			}
		}
	}
	return identifierObservation{}
}

// ObserveOpsIdentifiers retains the precise receipt even when a later retry or
// auxiliary transfer became current. kind is reduced to a closed Ops enum.
func (r Receipt) ObserveOpsIdentifiers(_ context.Context, requestID, responseID, kind string) {
	eventType := "unknown"
	switch kind {
	case "http_error", "failover", "retry_exhausted_failover", "response_error", "request_error", "stream_failed":
		eventType = "ops." + kind
	}
	r.attempt.observeIdentifierMetadata(
		identifierObservation{Namespace: "request", Value: requestID, Source: "ops", Field: "upstream_request_id", EventType: eventType},
		identifierObservation{Namespace: "response", Value: responseID, Source: "ops", Field: "upstream_response_id", EventType: eventType},
	)
}

func (a *Attempt) observeIdentifierMetadata(observations ...identifierObservation) {
	if a == nil {
		return
	}
	a.evidenceMu.Lock()
	defer a.evidenceMu.Unlock()
	if a.evidenceUnavailable {
		return
	}
	changed := false
	for _, observation := range observations {
		value := UpstreamIdentifier(observation.Value)
		if value == "" {
			continue
		}
		var first *string
		switch observation.Namespace {
		case "request":
			first = &a.upstreamRequestID
		case "response":
			first = &a.upstreamResponseID
		default:
			continue
		}
		if *first == "" {
			*first = value
			changed = true
		} else if *first != value && !a.identifierConflict {
			a.identifierConflict = true
			changed = true
		}
		if a.identifierDiagnostics.observe(observation, time.Now().UnixMilli()) {
			a.identifierDiagnostics.Version = IdentifierSnapshotVersion
			a.identifierDiagnostics.AttemptID = a.ID
			changed = true
		}
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
