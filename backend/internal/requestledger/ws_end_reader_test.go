package requestledger

import (
	"database/sql"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWSEndReaderMissingFutureInvalidAndSecretFields(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	s := newWSEndSnapshot(id, WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit", Reason: "cancelled", ReasonSource: "context_cause", JoinedUnfinished: true}, time.UnixMilli(1234))
	encoded, err := json.Marshal(s)
	require.NoError(t, err)
	for _, tc := range []struct{ name, raw, status string }{
		{"old", "{}", "not_recorded"},
		{"empty", "", "incomplete"},
		{"malformed", "{", "incomplete"},
		{"missing_version", `{"stage":"read_upstream"}`, "incomplete"},
		{"future", `{"version":"future"}`, "unsupported_version"},
		{"good", string(encoded), "persisted_snapshot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := WSEndAttemptEvidence{AttemptID: &id}
			item.readSnapshot(sql.NullString{String: tc.raw, Valid: true})
			require.Equal(t, tc.status, item.Status)
		})
	}
	for _, field := range []string{"stage", "stage_source", "reason", "reason_source", "usage_presence", "coverage_version", "attempt_id"} {
		var raw map[string]any
		require.NoError(t, json.Unmarshal(encoded, &raw))
		raw[field] = "secret-sentinel"
		b, err := json.Marshal(raw)
		require.NoError(t, err)
		item := WSEndAttemptEvidence{AttemptID: &id}
		item.readSnapshot(sql.NullString{String: string(b), Valid: true})
		require.Nil(t, item.Snapshot, field)
	}
	var raw map[string]any
	require.NoError(t, json.Unmarshal(encoded, &raw))
	raw["body"] = "body-secret-sentinel"
	raw["header"] = "header-secret-sentinel"
	b, err := json.Marshal(raw)
	require.NoError(t, err)
	item := WSEndAttemptEvidence{AttemptID: &id}
	item.readSnapshot(sql.NullString{String: string(b), Valid: true})
	require.Equal(t, "persisted_snapshot", item.Status)
	out, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(out), "secret-sentinel")
	item = WSEndAttemptEvidence{}
	item.readSnapshot(sql.NullString{})
	require.Equal(t, "not_recorded", item.Status)
	require.Nil(t, item.AttemptID)
}
