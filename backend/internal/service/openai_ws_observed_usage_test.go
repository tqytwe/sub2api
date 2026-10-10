package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Every upstream is an in-memory fake; the only socket is the loopback test client.
func TestWSObservedUsageSurvivesInterruptedTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{OpenAIWSIngressModeCtxPool, OpenAIWSIngressModePassthrough} {
		for _, failure := range []string{"read", "cancel", "preempt", "lease_lost", "write_client", "no_usage", "completed"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(context.Canceled)
				upstream := newStagedPassthroughConn()
				cfg := passthroughLifecycleConfig()
				cfg.Gateway.OpenAIFirstOutputTimeoutSeconds = 0
				cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
				if failure == "write_client" {
					cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 1
				}
				cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
				cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
				svc := newPassthroughLifecycleService(cfg, upstream)
				account := passthroughLifecycleAccount()
				account.Extra["openai_apikey_responses_websockets_v2_mode"] = mode
				if mode == OpenAIWSIngressModeCtxPool {
					pool := newOpenAIWSConnPool(cfg)
					pool.setClientDialerForTest(&openAIWSSingleConnDialer{conn: upstream})
					svc.openaiWSPool = pool
					t.Cleanup(pool.Close)
				}
				type settlement struct {
					turn   int
					result *OpenAIForwardResult
					err    error
				}
				var mu sync.Mutex
				var settlements []settlement
				server, done := startPassthroughLifecycleServerWithHooks(t, ctx, svc, account, func(*gin.Context) *OpenAIWSIngressHooks {
					return &OpenAIWSIngressHooks{AfterTurn: func(turn int, result *OpenAIForwardResult, err error) {
						mu.Lock()
						defer mu.Unlock()
						settlements = append(settlements, settlement{turn, result, err})
					}}
				})
				defer server.Close()
				clientCtx, stopClient := context.WithTimeout(context.Background(), 8*time.Second)
				defer stopClient()
				client, _, err := coderws.Dial(clientCtx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
				require.NoError(t, err)
				defer func() { _ = client.CloseNow() }()
				create := []byte(`{"type":"response.create","model":"gpt-5.1","input":[]}`)
				require.NoError(t, client.Write(clientCtx, coderws.MessageText, create))
				upstream.Send(`{"type":"response.completed","response":{"id":"resp_done","usage":{"input_tokens":3,"output_tokens":2}}}`)
				_, _, err = client.Read(clientCtx)
				require.NoError(t, err)
				if failure != "completed" {
					require.NoError(t, client.Write(clientCtx, coderws.MessageText, create))
					frame := `{"type":"response.in_progress","response":{"id":"resp_partial","model":"gpt-5.1","usage":{"input_tokens":9,"output_tokens":4,"input_tokens_details":{"cached_tokens":2}}}}`
					if failure == "no_usage" {
						frame = `{"type":"response.output_text.delta","response_id":"resp_partial","delta":"observed output"}`
					}
					upstream.Send(frame)
					_, _, err = client.Read(clientCtx)
					require.NoError(t, err)
				}
				switch failure {
				case "cancel":
					cancel(context.Canceled)
				case "preempt":
					cancel(errOpenAIWSSessionPreempted)
				case "lease_lost":
					cancel(ErrOpenAIWSIngressLeaseLost)
				case "write_client":
					// Stop reading and exhaust the loopback socket send buffer. This
					// deterministically exercises the real downstream write deadline.
					upstream.Send(`{"type":"response.output_text.delta","response_id":"resp_partial","delta":"` + strings.Repeat("x", 16<<20) + `"}`)
				default:
					upstream.Fail(io.EOF)
				}
				// Read the close handshake while the server finishes settlement.
				if failure != "write_client" {
					_, _, _ = client.Read(clientCtx)
				}
				select {
				case <-done:
				case <-clientCtx.Done():
					t.Fatal("relay did not stop")
				}
				mu.Lock()
				defer mu.Unlock()
				require.NotEmpty(t, settlements)
				require.NotNil(t, settlements[0].result)
				require.Equal(t, 3, settlements[0].result.Usage.InputTokens)
				if failure == "completed" {
					for _, s := range settlements[1:] {
						require.Nil(t, s.result, "completed turn must not settle twice")
					}
					return
				}
				require.Len(t, settlements, 2, "each turn is finalized once, including cancellation")
				partial := settlements[1]
				require.Equal(t, 2, partial.turn)
				require.Error(t, partial.err)
				require.NotNil(t, partial.result, "observed current-turn evidence must survive relay failure")
				require.Equal(t, "resp_partial", partial.result.RequestID)
				if failure == "no_usage" {
					require.False(t, partial.result.HasObservedUsage())
					return
				}
				require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 4, CacheReadInputTokens: 2}, partial.result.Usage)
				require.Equal(t, "gpt-5.1", partial.result.Model)
				if failure == "preempt" {
					require.True(t, errors.Is(partial.err, errOpenAIWSSessionPreempted))
				}
			})
		}
	}
}
