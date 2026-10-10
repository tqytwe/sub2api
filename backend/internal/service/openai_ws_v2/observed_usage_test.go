package openai_ws_v2

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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
		for ids := 0; ids < 8; ids++ {
			t.Run(fmt.Sprintf("%s/created_id=%t/error_id=%t/final_id=%t", terminal, ids&1 != 0, ids&2 != 0, ids&4 != 0), func(t *testing.T) {
				idField := func(key string, bit int) string {
					if ids&bit == 0 {
						return ""
					}
					return fmt.Sprintf(`%q:"current",`, key)
				}
				bare := fmt.Sprintf(`{"type":"error",%s"usage":{"input_tokens":5,"output_tokens":1},"error":{"message":"transient"}}`, idField("response_id", 2))
				if ids == 0 {
					bare = `{"type":"error","error":{"message":"transient"}}`
				}
				final := fmt.Sprintf(`{"type":%q,"response":{%s"usage":{"input_tokens":9,"output_tokens":4}}}`, terminal, idField("id", 4))
				rawFrames := []string{
					`{"type":"response.completed","response":{"id":"previous","usage":{"input_tokens":2,"output_tokens":1}}}`,
					fmt.Sprintf(`{"type":"response.created","response":{%s"model":"gpt-5.1"}}`, idField("id", 1)),
					bare,
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
				wantID := "current"
				if ids == 0 {
					wantID = ""
				}
				require.Equal(t, wantID, turns[1].RequestID)
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

func TestRelayBareErrorDoesNotAbsorbDifferentResponseTerminal(t *testing.T) {
	for _, terminal := range []string{"response.completed", "response.done", "response.incomplete", "response.failed"} {
		t.Run(terminal, func(t *testing.T) {
			final := fmt.Sprintf(`{"type":%q,"response":{"id":"other","usage":{"input_tokens":9,"output_tokens":4}}}`, terminal)
			client := newPassthroughTestFrameConn(nil, false)
			upstream := newPassthroughTestFrameConn([]passthroughTestFrame{
				{msgType: coderws.MessageText, payload: []byte(`{"type":"response.created","response":{"id":"current"}}`)},
				{msgType: coderws.MessageText, payload: []byte(`{"type":"error","usage":{"input_tokens":5,"output_tokens":1},"error":{"message":"transient"}}`)},
				{msgType: coderws.MessageText, payload: []byte(final)},
			}, true)
			var turns []RelayTurnResult
			result, relayExit := Relay(context.Background(), client, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), RelayOptions{
				OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) },
			})
			require.Nil(t, relayExit)
			require.Len(t, turns, 2)
			require.Equal(t, "current", turns[0].RequestID)
			require.Equal(t, Usage{InputTokens: 5, OutputTokens: 1}, turns[0].Usage)
			require.Equal(t, "other", turns[1].RequestID)
			require.Equal(t, Usage{InputTokens: 9, OutputTokens: 4}, turns[1].Usage)
			require.Equal(t, Usage{InputTokens: 14, OutputTokens: 5}, result.Usage)
			require.Len(t, client.Writes(), 3)
			require.Equal(t, final, string(client.Writes()[2].payload))
		})
	}
}

func TestRelayBareErrorWithoutIDsKeepsAcceptedNextClientTurnSeparate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := newPassthroughTestFrameConn(nil, false)
	upstream := newPassthroughTestFrameConn([]passthroughTestFrame{
		{msgType: coderws.MessageText, payload: []byte(`{"type":"response.created","response":{}}`)},
		{msgType: coderws.MessageText, payload: []byte(`{"type":"error","usage":{"input_tokens":5,"output_tokens":1},"error":{"message":"first turn failed"}}`)},
	}, false)
	var turns []RelayTurnResult
	var result RelayResult
	var relayExit *RelayExit
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, relayExit = Relay(ctx, client, upstream, []byte(`{"type":"response.create","model":"gpt-5.1"}`), RelayOptions{
			OnTurnComplete: func(turn RelayTurnResult) { turns = append(turns, turn) },
		})
	}()
	require.Eventually(t, func() bool { return len(client.Writes()) == 2 }, time.Second, time.Millisecond)
	client.readCh <- passthroughTestFrame{msgType: coderws.MessageText, payload: []byte(`{"type":"response.create","model":"gpt-5.1"}`)}
	require.Eventually(t, func() bool { return len(upstream.Writes()) == 2 }, time.Second, time.Millisecond)
	upstream.readCh <- passthroughTestFrame{msgType: coderws.MessageText, payload: []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2}}}`)}
	require.NoError(t, upstream.Close())
	<-done
	require.Nil(t, relayExit)
	require.Len(t, turns, 2, "an accepted next response.create is a real turn boundary even without upstream IDs")
	require.Equal(t, Usage{InputTokens: 5, OutputTokens: 1}, turns[0].Usage)
	require.Equal(t, Usage{InputTokens: 3, OutputTokens: 2}, turns[1].Usage)
	require.Equal(t, Usage{InputTokens: 8, OutputTokens: 3}, result.Usage)
	require.Len(t, client.Writes(), 3)
}
