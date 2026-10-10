//go:build integration

package service

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestLedgerPassthroughWaitsForAuthoritativeTurn(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx := context.Background()
	session, err := l.Begin(ctx, "/v1/responses", "GET", "ws_session")
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	turn, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	upstream := newStagedPassthroughConn()
	conn := &requestLedgerWSFrameConn{inner: upstream, account: &Account{ID: 234}}
	require.NoError(t, conn.WriteFrame(ctx, coderws.MessageText, []byte(`{"type":"response.create"}`)))
	// A raw terminal may belong to an old turn. Only the relay can decide
	// whether it is authoritative for the currently admitted turn.
	upstream.Send(`{"type":"response.completed","response":{"id":"old-turn"}}`)
	_, _, err = conn.ReadFrame(ctx)
	require.NoError(t, err)
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, turn.ID).Scan(&state))
	require.Equal(t, "inflight", state)
	second, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	require.NoError(t, conn.WriteFrame(ctx, coderws.MessageText, []byte(`{"type":"response.create"}`)))
	conn.finishTurn(ctx, 1, &OpenAIForwardResult{UpstreamTerminalEvent: "response.failed"}, nil)
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, turn.ID).Scan(&state))
	require.Equal(t, "failed", state)
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, second.ID).Scan(&state))
	require.Equal(t, "inflight", state, "a delayed authoritative first-turn result must not finish the second attempt")
	conn.finishTurn(ctx, 2, &OpenAIForwardResult{UpstreamTerminalEvent: "response.completed"}, nil)
	require.Empty(t, conn.attempts)
}

func TestRequestLedgerWSLateUsageDoesNotMarkTheNextAttemptKnown(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx := context.Background()
	session, err := l.Begin(ctx, "/v1/responses", "GET", "ws_session")
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	turn, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, turn)
	account := &Account{ID: 234}
	first, err := beginLedgerWSAttempt(ctx, account)
	require.NoError(t, err)
	finishLedgerWSResult(ctx, first, &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 7, OutputTokens: 3}}, errors.New("partial response disconnected"))
	_, err = beginLedgerWSAttempt(ctx, account)
	require.NoError(t, err)
	// A detached billing task for the first attempt may run after retry begins.
	require.NoError(t, requestledger.ObserveUsage(ctx, account.ID))
	var firstState, secondState string
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, turn.ID).Scan(&firstState))
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=2`, turn.ID).Scan(&secondState))
	require.Equal(t, []string{"known", "pending"}, []string{firstState, secondState})
}

func TestRequestLedgerNativeWebsocketTwoTurnsWithRealPostgres(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	// The second authoritative terminal deliberately has no response ID or usage.
	// PR346 must bind it to the second turn without reusing the first turn's evidence.
	upstream := &openAIWSCaptureConn{events: [][]byte{[]byte(`{"type":"response.completed","response":{"id":"fixture-first","usage":{"input_tokens":7,"output_tokens":3}}}`), []byte(`{"type":"response.completed","response":{}}`)}}
	pool := newOpenAIWSConnPool(cfg)
	defer pool.Close()
	pool.setClientDialerForTest(&openAIWSQueueDialer{conns: []openAIWSClientConn{upstream}})
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
	account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-only"}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
	r := gin.New()
	r.Use(requestledger.Middleware(ledger))
	done := make(chan error, 1)
	r.GET("/v1/responses", func(c *gin.Context) {
		require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
		conn, err := coderws.Accept(c.Writer, c.Request, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		_, first, err := ReadOpenAIWSClientMessage(c.Request.Context(), conn, 3*time.Second, coderws.StatusPolicyViolation, "fixture timeout")
		require.NoError(t, err)
		bindings := requestledger.NewTurnBindings(c.Request.Context())
		hooks := &OpenAIWSIngressHooks{BeforeRequest: func(turn int, payload []byte, originalModel string) error {
			bindings.Bind(turn, c.Request.Context())
			return nil
		}, AfterTurn: func(turn int, result *OpenAIForwardResult, turnErr error) {
			turnCtx := bindings.Context(turn, c.Request.Context())
			if result != nil && result.HasObservedUsage() {
				require.NoError(t, requestledger.ObserveUsage(turnCtx))
			}
			require.NoError(t, requestledger.FinishTurn(turnCtx, requestledger.Outcome(0, turnErr), turnErr))
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
	for range 2 {
		require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":[]}`)))
		_, _, err = client.Read(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("websocket did not finish")
	}
	var session string
	require.NoError(t, db.QueryRow(`SELECT id FROM gateway_requests WHERE kind='ws_session'`).Scan(&session))
	var turns, attempts int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE parent_id=$1 AND kind='ws_turn'`, session).Scan(&turns))
	require.Equal(t, 2, turns)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE phase='request'`).Scan(&attempts))
	require.Equal(t, 2, attempts)
	var firstUsage, secondUsage string
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE parent_id=$1 AND turn_no=1`, session).Scan(&firstUsage))
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE parent_id=$1 AND turn_no=2`, session).Scan(&secondUsage))
	require.Equal(t, "known", firstUsage)
	require.Equal(t, "usage_unknown", secondUsage)
}
