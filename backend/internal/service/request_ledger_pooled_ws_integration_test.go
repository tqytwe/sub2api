//go:build integration

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerPooledHTTPWSRejectsLateTerminal(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	for _, stream := range []bool{false, true} {
		for _, secondID := range []string{"resp_B", "", "created-only", "conflict"} {
			t.Run(fmt.Sprintf("stream=%t/id=%s", stream, secondID), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				var connections atomic.Int32
				stop := make(chan struct{})
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					connectionNo := connections.Add(1)
					conn, err := coderws.Accept(w, r, nil)
					require.NoError(t, err)
					defer conn.CloseNow()
					third := []byte(`{"type":"response.completed","response":{"id":"resp_C","usage":{"input_tokens":21,"output_tokens":8}}}`)
					if connectionNo == 2 {
						_, _, err = conn.Read(ctx)
						require.NoError(t, err)
						require.NoError(t, conn.Write(ctx, coderws.MessageText, third))
						select {
						case <-stop:
						case <-ctx.Done():
						}
						return
					}
					_, _, err = conn.Read(ctx)
					require.NoError(t, err)
					require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"rate_limits.updated","id":"event_aux"}`)))
					first := []byte(`{"type":"response.completed","response":{"id":"resp_A","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}}`)
					require.NoError(t, conn.Write(ctx, coderws.MessageText, first))
					// This read is the synchronization barrier: A has returned its lease,
					// B has acquired the empty connection and actually sent its create.
					_, _, err = conn.Read(ctx)
					require.NoError(t, err)
					require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response_id":"resp_A","response":{"usage":{"input_tokens":7,"output_tokens":3}}}`)))
					require.NoError(t, conn.Write(ctx, coderws.MessageText, first))
					require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.output_text.delta","delta":"current turn"}`)))
					idField := ""
					if secondID == "created-only" || secondID == "conflict" {
						require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(fmt.Sprintf(`{"type":"response.created","response":{"id":%q}}`, secondID))))
					}
					if secondID != "" && secondID != "created-only" {
						terminalID := secondID
						if secondID == "conflict" {
							terminalID = "resp_conflict_terminal"
						}
						idField = fmt.Sprintf(`"id":%q,`, terminalID)
					}
					require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(fmt.Sprintf(`{"type":"response.completed","response":{%s"status":"completed","usage":{"input_tokens":13,"output_tokens":5}}}`, idField))))
					if secondID == "resp_B" {
						_, _, err = conn.Read(ctx)
						require.NoError(t, err)
						require.NoError(t, conn.Write(ctx, coderws.MessageText, first))
						require.NoError(t, conn.Write(ctx, coderws.MessageText, third))
					}
					select {
					case <-stop:
					case <-ctx.Done():
					}
				}))
				defer upstream.Close()
				defer close(stop)
				cfg := newOpenAIWSV2TestConfig()
				cfg.Security.URLAllowlist.AllowInsecureHTTP = true
				cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
				cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
				cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
				cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
				pool := newOpenAIWSConnPool(cfg)
				defer pool.Close()
				pool.setClientDialerForTest(requestLedgerLoopbackWSDialer{target: "ws" + strings.TrimPrefix(upstream.URL, "http")})
				svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
				account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-only"}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
				var ids []string
				var results []*OpenAIForwardResult
				r := gin.New()
				r.Use(requestledger.Middleware(ledger))
				r.POST("/v1/responses", func(c *gin.Context) {
					require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
					ids = append(ids, requestledger.FromContext(c.Request.Context()).ID)
					result, err := svc.Forward(c.Request.Context(), c, account, []byte(fmt.Sprintf(`{"model":"gpt-5.1","stream":%t,"input":[]}`, stream)))
					require.NoError(t, err)
					require.True(t, result.OpenAIWSMode)
					results = append(results, result)
				})
				for range 3 {
					w := httptest.NewRecorder()
					r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil).WithContext(ctx))
					require.Equal(t, 200, w.Code)
				}
				wantConnections := int32(1)
				if secondID != "resp_B" {
					wantConnections = 2
				}
				require.Equal(t, wantConnections, connections.Load(), "anonymous terminals retire only after their admitted turn")
				require.Equal(t, "resp_C", results[2].RequestID)
				require.Equal(t, 21, results[2].Usage.InputTokens)
				require.Equal(t, 8, results[2].Usage.OutputTokens)
				// The legacy business parser may retain an event.id in RequestID;
				// protocol ownership and persisted evidence must never use it.
				require.Equal(t, secondID, results[1].RequestID, "A's terminal must not become B's result")
				require.Equal(t, 13, results[1].Usage.InputTokens)
				require.Equal(t, 5, results[1].Usage.OutputTokens)
				var evidenceID, execution, usage string
				require.Eventually(t, func() bool {
					return db.QueryRow(`SELECT COALESCE(e.upstream_response_id,''),a.execution_state,a.usage_state FROM gateway_request_attempts a JOIN gateway_request_attempt_evidence e USING(request_id,attempt_no) WHERE a.request_id=$1 AND a.phase='request'`, ids[1]).Scan(&evidenceID, &execution, &usage) == nil && evidenceID == secondID
				}, 3*time.Second, 10*time.Millisecond)
				require.Equal(t, secondID, evidenceID)
				require.Equal(t, "succeeded", execution)
				require.Equal(t, "known", usage)
			})
		}
	}
}

func TestRequestLedgerIngressRetirementPreservesUsageBeforeStrictClose(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	for _, mode := range []string{OpenAIWSIngressModeShared, OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			cfg := newOpenAIWSV2TestConfig()
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
			cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
			upstream := &openAIWSCaptureConn{events: [][]byte{
				[]byte(`{"type":"response.created","response":{"id":"resp_strict"}}`),
				[]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":7,"output_tokens":3}}}`),
			}}
			pool := newOpenAIWSConnPool(cfg)
			defer pool.Close()
			dialer := &openAIWSQueueDialer{conns: []openAIWSClientConn{upstream}}
			pool.setClientDialerForTest(dialer)
			svc := &OpenAIGatewayService{cfg: cfg, cache: &stubGatewayCache{}, toolCorrector: NewCodexToolCorrector(), openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), openaiWSPool: pool}
			account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{"openai_apikey_responses_websockets_v2_mode": mode}}
			done := make(chan error, 1)
			settled := make(chan *OpenAIForwardResult, 2)
			r := gin.New()
			r.Use(requestledger.Middleware(ledger))
			r.GET("/v1/responses", func(c *gin.Context) {
				require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
				conn, err := coderws.Accept(c.Writer, c.Request, nil)
				require.NoError(t, err)
				defer conn.CloseNow()
				_, first, err := ReadOpenAIWSClientMessage(c.Request.Context(), conn, 3*time.Second, coderws.StatusPolicyViolation, "fixture timeout")
				require.NoError(t, err)
				bindings := requestledger.NewTurnBindings(c.Request.Context())
				hooks := &OpenAIWSIngressHooks{BeforeRequest: func(turn int, _ []byte, _ string) error { bindings.Bind(turn, c.Request.Context()); return nil }, AfterTurn: func(turn int, result *OpenAIForwardResult, cause error) {
					turnCtx := bindings.Context(turn, c.Request.Context())
					if result != nil && result.HasObservedUsage() {
						require.NoError(t, requestledger.ObserveUsage(turnCtx))
						settled <- result
					}
					require.NoError(t, requestledger.FinishTurn(turnCtx, requestledger.Outcome(0, cause), cause))
				}}
				done <- svc.ProxyResponsesWebSocketFromClient(c.Request.Context(), c, conn, account, "fixture-only", first, hooks)
			})
			server := httptest.NewServer(r)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer client.CloseNow()
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","store":false,"input":[]}`)))
			for range 2 {
				_, _, err = client.Read(ctx)
				require.NoError(t, err)
			}
			select {
			case result := <-settled:
				require.Equal(t, 7, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
			case <-ctx.Done():
				t.Fatal("first usage did not reach its original callback")
			}
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","store":false,"previous_response_id":"resp_strict","input":[{"type":"function_call_output","call_id":"fixture_call","output":"fixture"}]}`)))
			select {
			case err := <-done:
				require.ErrorContains(t, err, "upstream response identity is ambiguous")
			case <-ctx.Done():
				t.Fatal("strict retired turn did not close")
			}
			upstream.mu.Lock()
			writes := len(upstream.writes)
			upstream.mu.Unlock()
			require.Equal(t, 1, writes, "neither shared nor dedicated mode may send the strict continuation on another connection")
			require.Empty(t, settled, "the rejected second turn must not settle the first usage again")
		})
	}
}
