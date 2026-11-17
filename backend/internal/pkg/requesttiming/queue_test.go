package requesttiming

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueueObservationUsesMeasuredFinalForwardBoundaries(t *testing.T) {
	s := Snapshot{TotalMS: 1500, TTFTMode: "semantic", Events: map[string]float64{"first_semantic": 600}, Spans: []Span{
		{Name: "forward_attempt", StartMS: 100, EndMS: 250},
		{Name: "forward_attempt", StartMS: 300, EndMS: 1400},
	}}
	v := DeriveQueueObservation(s)
	require.NotNil(t, v)
	require.Equal(t, "measured", v.Status)
	require.Equal(t, int64(1500), v.ServerDurationMs)
	require.Equal(t, int64(1100), v.ForwardDurationMs)
	require.Equal(t, int64(600), *v.ServerFirstTokenMs)
	require.Equal(t, int64(300), *v.ForwardFirstTokenMs)
	// An earlier failed attempt's output cannot be attributed to the final one.
	s.Events["first_semantic"] = 200
	v = DeriveQueueObservation(s)
	require.NotNil(t, v)
	require.Nil(t, v.ForwardFirstTokenMs)
	require.Nil(t, v.ServerFirstTokenMs)
}

func TestQueueObservationDoesNotInventMissingOrInvalidSamples(t *testing.T) {
	for _, s := range []Snapshot{{}, {TotalMS: 100}, {TotalMS: 100, Truncated: true, Spans: []Span{{Name: "forward_attempt", StartMS: 0, EndMS: 100}}}, {TotalMS: math.NaN()}, {TotalMS: 100, Spans: []Span{{Name: "forward_attempt", StartMS: 50, EndMS: 150}}}} {
		require.Nil(t, DeriveQueueObservation(s))
	}
	s := Snapshot{TotalMS: 100, TTFTMode: "visible", Events: map[string]float64{"first_semantic": 20, "first_visible": 40}, Spans: []Span{{Name: "forward_attempt", StartMS: 10, EndMS: 100}}}
	v := DeriveQueueObservation(s)
	require.NotNil(t, v)
	require.Equal(t, int64(40), *v.ServerFirstTokenMs)
	require.Equal(t, int64(30), *v.ForwardFirstTokenMs)
}
