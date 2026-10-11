package requestledger

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIdentifierSnapshotFirstDifferentPerNamespace(t *testing.T) {
	snapshot := identifierSnapshot{Version: IdentifierSnapshotVersion, AttemptID: "00000000-0000-4000-8000-000000000001"}
	now := time.Now().UnixMilli()
	request := identifierObservation{Namespace: "request", Value: "req_first", Source: "http_header", Field: "x-request-id", EventType: "none"}
	require.True(t, snapshot.observe(request, now))
	duplicate := request
	duplicate.Source, duplicate.Field = "ops", "upstream_request_id"
	require.False(t, snapshot.observe(duplicate, now+1), "same value must not rewrite first provenance")
	different := duplicate
	different.Value, different.EventType = "req_second", "ops.failover"
	require.True(t, snapshot.observe(different, now+2))
	different.Value = "req_third"
	require.False(t, snapshot.observe(different, now+3), "third value must not replace the first different value")
	response := identifierObservation{Namespace: "response", Value: "resp_first", Source: "http_sse", Field: "response.id", EventType: "response.created"}
	require.True(t, snapshot.observe(response, now+4))
	response.Value, response.EventType = "resp_second", "response.failed"
	require.True(t, snapshot.observe(response, now+5), "second namespace changes even after the conflict flag was already true")
	require.Equal(t, "req_first", snapshot.Request.First.Value)
	require.Equal(t, "http_header", snapshot.Request.First.Source)
	require.Equal(t, now, snapshot.Request.First.ObservedAtUnixMS)
	require.Equal(t, "req_second", snapshot.Request.FirstDifferent.Value)
	require.Equal(t, "resp_second", snapshot.Response.FirstDifferent.Value)
	require.Equal(t, "response.failed", snapshot.Response.FirstDifferent.EventType)
}

func TestIdentifierSnapshotBoundsAndWhitelist(t *testing.T) {
	var snapshot identifierSnapshot
	for _, value := range []string{"sk-secret-sentinel", "Bearer-secret-sentinel", strings.Repeat("x", 129), "body secret with spaces"} {
		require.False(t, snapshot.observe(identifierObservation{Namespace: "response", Value: value}, 1))
	}
	require.False(t, snapshot.observe(identifierObservation{Namespace: "credential-sentinel", Value: "valid_id"}, 1))
	require.True(t, snapshot.observe(identifierObservation{Namespace: "response", Value: "resp_valid", Source: "secret-source-sentinel", Field: "secret-field-sentinel", EventType: "secret-event-sentinel"}, 1))
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "sentinel")
	require.Equal(t, "unknown", snapshot.Response.First.Source)
	require.Equal(t, "unknown", snapshot.Response.First.Field)
	require.Equal(t, "unknown", snapshot.Response.First.EventType)
	require.LessOrEqual(t, len(encoded), identifierSnapshotMaxBytes)
}

func TestIdentifierSnapshotSSEPayloadTypeBoundary(t *testing.T) {
	data := []byte("event: response.failed\ndata: {\"response\":{\"id\":\"resp_event_only\"}}\n\n" +
		"data: {\"response\":{\"id\":\"resp_no_type\"}}\n\n" +
		"event: secret-event-sentinel\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_payload\"}}\n\n" +
		"event: error\ndata: {\"response\":{\"id\":\"resp_error_only\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_after_error\"}}\n\n")
	for _, size := range []int{1, 7, len(data)} {
		var observations []identifierObservation
		evidence := streamEvidence{observeIdentifier: func(observation identifierObservation) {
			observations = append(observations, observation)
		}}
		for pos := 0; pos < len(data); pos += size {
			evidence.observe(data[pos:min(pos+size, len(data))])
		}
		require.Len(t, observations, 5)
		for i, want := range []string{"unknown", "unknown", "response.created", "unknown", "response.completed"} {
			require.Equal(t, want, observations[i].EventType, "payload.type only; event lines must not leak across frames")
		}
		require.Equal(t, "failed", evidence.terminal, "existing event:error terminal remains authoritative")
		encoded, err := json.Marshal(observations)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "secret-event-sentinel")
	}
}
