package requestledger

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWSEndSnapshotWhitelistAndUsageSemantics(t *testing.T) {
	secret := "sk-fixture-secret-must-not-persist"
	s := newWSEndSnapshot("00000000-0000-4000-8000-000000000001", WSEndObservation{
		Stage: secret, StageSource: secret, Reason: secret, ReasonSource: secret,
		TerminalEvent: secret, WroteDownstreamScope: secret,
	}, time.UnixMilli(1234))
	require.Equal(t, "unknown", s.Stage)
	require.Equal(t, "unknown", s.Reason)
	require.Equal(t, "unknown", s.Terminal.State)
	require.Equal(t, "unknown", s.UsagePresence)
	require.False(t, s.PositiveUnitsObserved)
	encoded, err := json.Marshal(s)
	require.NoError(t, err)
	require.Less(t, len(encoded), wsEndSnapshotMaxBytes)
	require.NotContains(t, string(encoded), secret)
	require.NotContains(t, string(encoded), "billing")
}

func TestWSEndSnapshotTerminalAuthorityAndImmutableScalars(t *testing.T) {
	wrote := false
	input := WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit", Reason: "transport_error", ReasonSource: "finish_cause",
		WroteDownstream: &wrote, WroteDownstreamScope: "relay_session", JoinedUnfinished: true, ResultObserved: true}
	s := newWSEndSnapshot("00000000-0000-4000-8000-000000000001", input, time.UnixMilli(1234))
	wrote = true
	require.NotNil(t, s.WroteDownstream)
	require.False(t, *s.WroteDownstream)
	require.Equal(t, "not_observed_in_scope", s.Terminal.State)
	require.Equal(t, "relay_joined_unfinished", s.Terminal.Source)
	for _, terminal := range []string{"response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled"} {
		input.TerminalEvent = terminal
		s = newWSEndSnapshot(s.AttemptID, input, time.UnixMilli(1234))
		require.Equal(t, "observed", s.Terminal.State)
		require.Equal(t, terminal, s.Terminal.Event)
		require.Equal(t, "ws_forward_result", s.Terminal.Source)
	}
	input.JoinedUnfinished, input.TerminalEvent = false, ""
	s = newWSEndSnapshot(s.AttemptID, input, time.UnixMilli(1234))
	require.Equal(t, "unknown", s.Terminal.State)
	require.Equal(t, "unknown", s.Terminal.Source)
	input.PositiveUnitsObserved = true
	s = newWSEndSnapshot(s.AttemptID, input, time.UnixMilli(1234))
	require.True(t, s.PositiveUnitsObserved)
	require.Equal(t, "unknown", s.UsagePresence)
	require.False(t, strings.Contains(s.Stage, "sk-"))
}

func TestWSEndSnapshotRequiresResultForPositiveOrTerminal(t *testing.T) {
	s := newWSEndSnapshot("00000000-0000-4000-8000-000000000001", WSEndObservation{TerminalEvent: "response.failed", PositiveUnitsObserved: true}, time.UnixMilli(1234))
	require.False(t, s.PositiveUnitsObserved)
	require.Equal(t, "unknown", s.Terminal.State)
}
