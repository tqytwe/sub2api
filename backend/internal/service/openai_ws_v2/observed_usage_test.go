package openai_ws_v2

import (
	"context"
	"errors"
	"fmt"
	"testing"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type observedUsageWriteFailure struct{ FrameConn }

func (c observedUsageWriteFailure) WriteFrame(context.Context, coderws.MessageType, []byte) error {
	return errors.New("synthetic write failure")
}

func TestRelayUnfinishedUsageIsSeparateFromCompletedTurns(t *testing.T) {
	for _, failure := range []string{"read", "write", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := FrameConn(newPassthroughTestFrameConn(nil, false))
			if failure == "write" {
				client = observedUsageWriteFailure{client}
			}
			frames := []passthroughTestFrame{
				{msgType: coderws.MessageText, payload: []byte(`{"type":"response.completed","response":{"id":"done","usage":{"input_tokens":3,"output_tokens":2}}}`)},
				{msgType: coderws.MessageText, payload: []byte(`{"type":"response.in_progress","response":{"id":"partial","usage":{"input_tokens":9,"output_tokens":4}}}`)},
			}
			if failure == "write" {
				frames = frames[1:]
			}
			upstream := newPassthroughTestFrameConn(frames, failure != "cancel")
			var turns []RelayTurnResult
			result, _ := Relay(ctx, client, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), RelayOptions{
				OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) },
				AfterClientWrite: func(_ coderws.MessageType, raw []byte, _ error) {
					if failure == "cancel" && string(raw) == string(frames[1].payload) {
						cancel()
					}
				},
			})
			require.NotNil(t, result.UnfinishedTurn)
			require.Equal(t, "partial", result.UnfinishedTurn.RequestID)
			require.Equal(t, Usage{InputTokens: 9, OutputTokens: 4}, result.UnfinishedTurn.Usage)
			if failure == "write" {
				require.Empty(t, turns)
				require.Equal(t, Usage{}, result.Usage)
			} else {
				require.Len(t, turns, 1)
				require.Equal(t, Usage{InputTokens: 3, OutputTokens: 2}, result.Usage)
			}
		})
	}
}

func TestRelayTerminalSettlementIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		want   int
	}{
		{"duplicate", []string{
			`{"type":"response.completed","response":{"id":"resp_one","usage":{"input_tokens":9,"output_tokens":4}}}`,
			`{"type":"response.completed","response":{"id":"resp_one","usage":{"input_tokens":9,"output_tokens":4}}}`,
		}, 1},
		{"without_response_id", []string{
			`{"type":"response.created","response":{}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":9,"output_tokens":4}}}`,
			`{"type":"response.created","response":{}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":9,"output_tokens":4}}}`,
		}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames []passthroughTestFrame
			for _, raw := range tc.frames {
				frames = append(frames, passthroughTestFrame{msgType: coderws.MessageText, payload: []byte(raw)})
			}
			client := newPassthroughTestFrameConn(nil, false)
			upstream := newPassthroughTestFrameConn(frames, true)
			var turns []RelayTurnResult
			result, _ := Relay(context.Background(), client, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), RelayOptions{OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) }})
			require.Len(t, turns, tc.want)
			require.Equal(t, 9*tc.want, result.Usage.InputTokens)
		})
	}
}

func TestRelayBareErrorKeepsAuthoritativeSameResponseTerminal(t *testing.T) {
	for _, terminal := range []string{"response.completed", "response.done", "response.incomplete", "response.failed"} {
		for _, bare := range []struct{ name, frame string }{
			{"inferred_id", `{"type":"error","usage":{"input_tokens":5,"output_tokens":1},"error":{"message":"transient"}}`},
			{"explicit_id", `{"type":"error","response_id":"current","usage":{"input_tokens":5,"output_tokens":1},"error":{"message":"transient"}}`},
			{"no_id_or_usage", `{"type":"error","error":{"message":"transient"}}`},
		} {
			t.Run(terminal+"/"+bare.name, func(t *testing.T) {
				final := fmt.Sprintf(`{"type":%q,"response":{"id":"current","usage":{"input_tokens":9,"output_tokens":4}}}`, terminal)
				rawFrames := []string{
					`{"type":"response.completed","response":{"id":"previous","usage":{"input_tokens":2,"output_tokens":1}}}`,
					`{"type":"response.created","response":{"id":"current"}}`,
					bare.frame,
					`{"type":"rate_limits.updated","rate_limits":[]}`,
					final,
					final, // A real repeated terminal is still deduplicated.
					`{"type":"response.created","response":{"id":"next"}}`,
					`{"type":"response.completed","response":{"id":"next","usage":{"input_tokens":3,"output_tokens":2}}}`,
				}
				frames := make([]passthroughTestFrame, 0, len(rawFrames))
				for _, raw := range rawFrames {
					frames = append(frames, passthroughTestFrame{msgType: coderws.MessageText, payload: []byte(raw)})
				}
				client := newPassthroughTestFrameConn(nil, false)
				upstream := newPassthroughTestFrameConn(frames, true)
				var turns []RelayTurnResult
				result, relayExit := Relay(context.Background(), client, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), RelayOptions{
					OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) },
				})
				require.Nil(t, relayExit)
				require.Len(t, turns, 3)
				require.Equal(t, "previous", turns[0].RequestID)
				require.Equal(t, Usage{InputTokens: 2, OutputTokens: 1}, turns[0].Usage)
				require.Equal(t, "current", turns[1].RequestID)
				require.Equal(t, terminal, turns[1].TerminalEventType)
				require.Equal(t, Usage{InputTokens: 9, OutputTokens: 4}, turns[1].Usage)
				require.Equal(t, "next", turns[2].RequestID)
				require.Equal(t, Usage{InputTokens: 3, OutputTokens: 2}, turns[2].Usage)
				require.Equal(t, Usage{InputTokens: 14, OutputTokens: 7}, result.Usage)
				require.Nil(t, result.UnfinishedTurn)
				writes := client.Writes()
				require.Len(t, writes, len(rawFrames)-1)
				require.Equal(t, final, string(writes[4].payload), "the authoritative terminal must reach the client")
			})
		}
	}
}
