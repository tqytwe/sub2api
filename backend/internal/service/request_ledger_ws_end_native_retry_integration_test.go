//go:build integration

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerWSEndRealNativeRetryHTTPWSPostgres(t *testing.T) {
	for _, closeKind := range []string{"1013", "eof"} {
		t.Run(closeKind, func(t *testing.T) {
			db := ledgertest.New(t)
			ledger := requestledger.New(db)
			var received atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				require.NoError(t, err)
				defer func() { _ = conn.CloseNow() }()
				_, _, err = conn.Read(r.Context())
				require.NoError(t, err)
				received.Add(1)
				var durable int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts a JOIN gateway_requests q ON q.id=a.request_id WHERE a.phase='request' AND a.execution_state='inflight' AND a.account_id=234 AND a.credential_account_id=234 AND q.user_id=101 AND q.api_key_id=201 AND q.kind='ws_turn'`).Scan(&durable))
				require.Positive(t, durable, "actual upstream must observe precommitted attribution")
				if closeKind == "1013" {
					_ = conn.Close(coderws.StatusTryAgainLater, "synthetic busy")
				} else {
					_ = conn.CloseNow()
				}
			}))
			defer upstream.Close()
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			cfg.Gateway.OpenAIWS.Enabled = true
			cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
			cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
			cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
			cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
			cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
			pool := newOpenAIWSConnPool(cfg)
			defer pool.Close()
			pool.setClientDialerForTest(requestLedgerLoopbackWSDialer{target: "ws" + strings.TrimPrefix(upstream.URL, "http")})
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
			account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-only", "base_url": upstream.URL}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
			r := gin.New()
			r.Use(requestledger.Middleware(ledger))
			done := make(chan error, 1)
			r.GET("/v1/responses", func(c *gin.Context) {
				require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
				conn, err := coderws.Accept(c.Writer, c.Request, nil)
				require.NoError(t, err)
				defer func() { _ = conn.CloseNow() }()
				_, first, err := ReadOpenAIWSClientMessage(c.Request.Context(), conn, 3*time.Second, coderws.StatusPolicyViolation, "synthetic timeout")
				require.NoError(t, err)
				bindings := requestledger.NewTurnBindings(c.Request.Context())
				hooks := &OpenAIWSIngressHooks{
					BeforeRequest: func(turn int, _ []byte, _ string) error { bindings.Bind(turn, c.Request.Context()); return nil },
					AfterTurn: func(turn int, _ *OpenAIForwardResult, cause error) {
						require.NoError(t, requestledger.FinishTurn(bindings.Context(turn, c.Request.Context()), requestledger.Outcome(0, cause), cause))
					},
				}
				done <- svc.ProxyResponsesWebSocketFromClient(c.Request.Context(), c, conn, account, "fixture-only", first, hooks)
			})
			server := httptest.NewServer(r)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer func() { _ = client.CloseNow() }()
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":[]}`)))
			_, _, err = client.Read(ctx)
			require.Error(t, err)
			select {
			case err := <-done:
				require.Error(t, err)
			case <-ctx.Done():
				t.Fatal("gateway did not finish after the synthetic upstream disconnect")
			}
			require.Positive(t, received.Load())
			var turnID, parentID, execution, usage, settlement string
			require.NoError(t, db.QueryRow(`SELECT id,parent_id,execution_state,usage_state,settlement_state FROM gateway_requests WHERE kind='ws_turn' AND user_id=101 AND api_key_id=201`).Scan(&turnID, &parentID, &execution, &usage, &settlement))
			require.NotEmpty(t, parentID)
			require.NotEqual(t, parentID, turnID)
			require.NotEqual(t, "inflight", execution)
			require.NotEqual(t, "succeeded", execution)
			require.Equal(t, "usage_unknown", usage)
			require.Equal(t, "settlement_pending", settlement)
			var connectionAttempts, sendAttempts, billings int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1 AND phase='ws_connect' AND account_id=234 AND credential_account_id=234`, turnID).Scan(&connectionAttempts))
			require.Positive(t, connectionAttempts)
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1 AND phase='request' AND account_id=234 AND usage_state='usage_unknown' AND execution_state<>'inflight'`, turnID).Scan(&sendAttempts))
			require.GreaterOrEqual(t, sendAttempts, int(received.Load()), "precommitted send intent can outlive a failed network write")
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links WHERE request_id=$1`, turnID).Scan(&billings))
			require.Zero(t, billings, "unknown usage must not fabricate a settlement")
			// Query every precommitted handshake/model attempt through the independent reader.
			page, err := ledger.WSEndEvidence(context.Background(), requestledger.Viewer{Admin: true}, turnID, 0)
			require.NoError(t, err)
			for _, item := range page.Attempts {
				snapshot := readWSEnd(t, db, *item.AttemptID)
				require.Equal(t, "unknown", snapshot["usage_presence"])
				require.Equal(t, false, snapshot["positive_units_observed"])
				if item.Phase == "ws_connect" {
					require.Equal(t, "dial", snapshot["stage"])
				} else {
					require.Equal(t, "read_upstream", snapshot["stage"])
					require.Equal(t, "ingress_turn_error", snapshot["stage_source"])
					require.Equal(t, "unknown", snapshot["terminal"].(map[string]any)["state"], "non-joined result cannot assert terminal absence")
				}
			}
			require.EqualValues(t, 2, received.Load(), "the existing one-retry policy remains unchanged")
			beforeRecovery := received.Load()
			require.NoError(t, ledger.Recover(context.Background()))
			require.NoError(t, ledger.Recover(context.Background()))
			require.Equal(t, beforeRecovery, received.Load(), "ledger recovery must not replay upstream work")
		})
	}
}
