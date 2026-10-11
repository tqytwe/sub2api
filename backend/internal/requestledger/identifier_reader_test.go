package requestledger

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentifierSnapshotReaderRejectsUntrustedMetadata(t *testing.T) {
	id, request, conflict := "00000000-0000-4000-8000-000000000001", "req_first", false
	for _, tc := range []struct {
		name, status string
		change       func(*identifierSnapshot)
	}{
		{"valid", "persisted_snapshot", func(*identifierSnapshot) {}},
		{"different attempt", "incomplete", func(s *identifierSnapshot) { s.AttemptID = "other" }},
		{"unknown version", "unsupported_version", func(s *identifierSnapshot) { s.Version = "future" }},
		{"empty version", "incomplete", func(s *identifierSnapshot) { s.Version = "" }},
		{"untrusted source", "incomplete", func(s *identifierSnapshot) { s.Request.First.Source = "secret-source-sentinel" }},
		{"untrusted event", "incomplete", func(s *identifierSnapshot) { s.Request.First.EventType = "secret-event-sentinel" }},
		{"untrusted field", "incomplete", func(s *identifierSnapshot) { s.Request.First.Field = "secret-field-sentinel" }},
		{"invalid identifier", "incomplete", func(s *identifierSnapshot) { s.Request.First.Value = "sk-secret-sentinel" }},
		{"old first mismatch", "incomplete", func(s *identifierSnapshot) { s.Request.First.Value = "req_new" }},
		{"missing observation time", "incomplete", func(s *identifierSnapshot) { s.Request.First.ObservedAtUnixMS = 0 }},
		{"missing first", "incomplete", func(s *identifierSnapshot) { s.Request.First = nil }},
		{"snapshot flag mismatch", "incomplete", func(s *identifierSnapshot) {
			v := *s.Request.First
			v.Value = "req_second"
			s.Request.FirstDifferent = &v
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := identifierSnapshot{Version: IdentifierSnapshotVersion, AttemptID: id}
			snapshot.observe(identifierObservation{Namespace: "request", Value: request, Source: "http_header", Field: "x-request-id", EventType: "none"}, 1)
			tc.change(&snapshot)
			raw, err := json.Marshal(snapshot)
			require.NoError(t, err)
			item := IdentifierAttemptEvidence{AttemptID: &id, IdentifierConflict: &conflict}
			item.readSnapshot(sql.NullString{String: string(raw), Valid: true}, &request, nil)
			require.Equal(t, tc.status, item.Status)
			output, err := json.Marshal(item)
			require.NoError(t, err)
			require.NotContains(t, string(output), "sentinel")
		})
	}
	for _, tc := range []struct{ raw, status string }{
		{"", "incomplete"}, {"[]", "incomplete"}, {"null", "incomplete"},
		{`{"version":null}`, "incomplete"}, {`{"version":""}`, "incomplete"},
		{`{"request":{}}`, "incomplete"}, {`{"version":"future"}`, "unsupported_version"},
		{"{}", "not_recorded"}, {strings.Repeat("x", identifierSnapshotMaxBytes+1), "incomplete"},
	} {
		t.Run(tc.raw[:min(len(tc.raw), 32)], func(t *testing.T) {
			item := IdentifierAttemptEvidence{AttemptID: &id, IdentifierConflict: &conflict}
			item.readSnapshot(sql.NullString{String: tc.raw, Valid: true}, &request, nil)
			require.Equal(t, tc.status, item.Status)
		})
	}
}
