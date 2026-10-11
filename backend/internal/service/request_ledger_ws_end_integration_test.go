//go:build integration

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
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

func readWSEnd(t *testing.T, db *sql.DB, id string) map[string]any {
	t.Helper()
	var raw string
	require.Eventually(t, func() bool {
		return db.QueryRow(`SELECT snapshot::text FROM gateway_request_ws_end_evidence WHERE attempt_id=$1`, id).Scan(&raw) == nil && raw != "{}"
	}, 3*time.Second, 10*time.Millisecond)
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &value))
	require.Equal(t, "unknown", value["usage_presence"])
	require.NotContains(t, raw, "secret-sentinel")
	return value
}

func TestRequestLedgerWSEndExactTurnAndFirstWrite(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	ctx, h, _ := ledgerFollowupContext(t, ledger, "/v1/responses", "http")
	first, err := beginLedgerWSAttempt(ctx, &Account{ID: 234})
	require.NoError(t, err)
	second, err := beginLedgerWSAttempt(ctx, &Account{ID: 234})
	require.NoError(t, err)
	conn := &requestLedgerWSFrameConn{turn: 2, attempts: map[int]*requestledger.Attempt{1: first, 2: second}}
	cause := wrapOpenAIWSIngressTurnError("read_upstream", io.ErrUnexpectedEOF, false)
	conn.finishTurn(ctx, 1, &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 7}}, cause)
	a := readWSEnd(t, db, first.ID)
	require.Equal(t, "read_upstream", a["stage"])
	require.Equal(t, "ingress_turn_error", a["stage_source"])
	require.Equal(t, "unexpected_eof", a["reason"])
	require.Equal(t, true, a["positive_units_observed"])
	require.Equal(t, "unknown", a["terminal"].(map[string]any)["state"])
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=2`, h.ID).Scan(&state))
	require.Equal(t, "inflight", state)
	// A stale callback cannot select the active turn or replace its first end.
	conn.finishTurn(ctx, 1, &OpenAIForwardResult{UpstreamTerminalEvent: "response.completed"}, nil)
	conn.finishTurn(ctx, 2, &OpenAIForwardResult{UpstreamTerminalEvent: "response.failed"}, nil)
	b := readWSEnd(t, db, second.ID)
	require.Equal(t, "upstream_failed", b["reason"])
	require.Equal(t, "observed", b["terminal"].(map[string]any)["state"])
	require.Equal(t, false, b["positive_units_observed"])
	require.Equal(t, a, readWSEnd(t, db, first.ID))
	require.True(t, isOpenAIWSIngressTurnRetryable(cause), "observer must not mutate retry classification")
	upstream := newStagedPassthroughConn()
	_ = upstream.Close()
	writeConn := &requestLedgerWSFrameConn{inner: upstream, account: &Account{ID: 234}}
	err = writeConn.WriteFrame(ctx, coderws.MessageText, []byte(`{"type":"response.create"}`))
	require.Error(t, err)
	require.Empty(t, writeConn.attempts)
	write := readWSEnd(t, db, writeConn.lastFinished.ID)
	require.Equal(t, "write_upstream", write["stage"])
	require.Equal(t, "frame_write", write["stage_source"])
	require.Equal(t, "unknown", write["terminal"].(map[string]any)["state"])
	active, err := beginLedgerWSAttempt(ctx, &Account{ID: 234})
	require.NoError(t, err)
	controlConn := &requestLedgerWSFrameConn{inner: upstream, account: &Account{ID: 234}, turn: 1, attempts: map[int]*requestledger.Attempt{1: active}}
	require.Error(t, controlConn.WriteFrame(ctx, coderws.MessageText, []byte(`{"type":"input_audio_buffer.append"}`)))
	control := readWSEnd(t, db, active.ID)
	require.Equal(t, "write_upstream", control["stage"], "a control-frame write failure is not generic cleanup")
	require.Equal(t, "frame_write", control["stage_source"])
}

func TestRequestLedgerWSEndResultAndTypedCauseContract(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx, _, _ := ledgerFollowupContext(t, l, "/v1/responses", "http")
	for _, tc := range []struct {
		name     string
		result   *OpenAIForwardResult
		cause    error
		reason   string
		positive bool
	}{
		{"cancel", nil, context.Canceled, "cancelled", false},
		{"deadline", nil, context.DeadlineExceeded, "deadline_exceeded", false},
		{"lease", nil, ErrOpenAIWSIngressLeaseLost, "ingress_lease_lost", false},
		{"preempt", nil, errOpenAIWSSessionPreempted, "session_preempted", false},
		{"unknown", nil, errors.New("error-secret-sentinel"), "unknown", false},
		{"empty", &OpenAIForwardResult{}, nil, "none", false},
		{"zero", &OpenAIForwardResult{Usage: OpenAIUsage{}}, nil, "none", false},
		{"details_only", &OpenAIForwardResult{Usage: OpenAIUsage{CacheReadInputTokens: 5, ImageOutputTokens: 7}}, nil, "none", false},
		{"positive", &OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 1}}, nil, "none", true},
		{"image", &OpenAIForwardResult{ImageCount: 1}, nil, "none", true},
		{"failed", &OpenAIForwardResult{UpstreamTerminalEvent: "response.failed"}, nil, "upstream_failed", false},
		{"incomplete", &OpenAIForwardResult{UpstreamTerminalEvent: "response.incomplete"}, nil, "upstream_incomplete", false},
		{"cancelled", &OpenAIForwardResult{UpstreamTerminalEvent: "response.cancelled"}, nil, "upstream_cancelled", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := beginLedgerWSAttempt(ctx, &Account{ID: 234})
			require.NoError(t, err)
			finishLedgerWSResult(ctx, a, tc.result, tc.cause)
			s := readWSEnd(t, db, a.ID)
			require.Equal(t, tc.reason, s["reason"])
			require.Equal(t, tc.positive, s["positive_units_observed"])
			require.Equal(t, tc.result != nil, s["result_observed"])
		})
	}
}

func TestRequestLedgerWSEndRealPassthroughHTTPWSPostgres(t *testing.T) {
	for _, scenario := range []string{"eof", "cancel", "terminal", "two_turns"} {
		t.Run(scenario, func(t *testing.T) {
			db := ledgertest.New(t)
			ledger := requestledger.New(db)
			control, cancelControl := context.WithCancel(context.Background())
			defer cancelControl()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var sends, dials atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dials.Add(1)
				conn, err := coderws.Accept(w, r, nil)
				require.NoError(t, err)
				defer conn.CloseNow()
				rounds := 1
				if scenario == "two_turns" {
					rounds = 2
				}
				for turn := 1; turn <= rounds; turn++ {
					_, _, err = conn.Read(ctx)
					require.NoError(t, err)
					sends.Add(1)
					if scenario == "eof" {
						return
					}
					if scenario == "cancel" {
						require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.created","response":{"id":"resp_cancel"}}`)))
						cancelControl()
						return
					}
					if scenario == "two_turns" && turn == 2 {
						// The relay must discard this old terminal before assigning second-turn evidence.
						require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_first","usage":{"input_tokens":8}}}`)))
						return
					}
					require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_first","usage":{"input_tokens":8},"output":"body-secret-sentinel"}}`)))
				}
				_, _, _ = conn.Read(ctx)
			}))
			defer upstream.Close()
			cfg := passthroughLifecycleConfig()
			cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 3
			pool := newOpenAIWSConnPool(cfg)
			defer pool.Close()
			pool.setClientDialerForTest(requestLedgerLoopbackWSDialer{target: "ws" + strings.TrimPrefix(upstream.URL, "http")})
			svc := &OpenAIGatewayService{cfg: cfg, cache: &stubGatewayCache{}, toolCorrector: NewCodexToolCorrector(), openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), openaiWSPool: pool, openaiWSPassthroughDialer: requestLedgerLoopbackWSDialer{target: "ws" + strings.TrimPrefix(upstream.URL, "http")}}
			account := passthroughLifecycleAccount()
			account.ID = 234
			account.Credentials = map[string]any{"api_key": "fixture-only", "base_url": upstream.URL}
			r := gin.New()
			r.Use(requestledger.Middleware(ledger))
			done := make(chan error, 1)
			r.GET("/v1/responses", func(c *gin.Context) {
				require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
				active := requestledger.WithHandle(control, requestledger.FromContext(c.Request.Context()))
				c.Request = c.Request.WithContext(active)
				conn, err := coderws.Accept(c.Writer, c.Request, nil)
				require.NoError(t, err)
				defer conn.CloseNow()
				_, first, err := ReadOpenAIWSClientMessage(active, conn, 3*time.Second, coderws.StatusPolicyViolation, "fixture")
				require.NoError(t, err)
				bindings := requestledger.NewTurnBindings(active)
				hooks := &OpenAIWSIngressHooks{BeforeRequest: func(turn int, _ []byte, _ string) error { bindings.Bind(turn, active); return nil }, AfterTurn: func(turn int, _ *OpenAIForwardResult, cause error) {
					require.NoError(t, requestledger.FinishTurn(bindings.Context(turn, active), requestledger.Outcome(0, cause), cause))
				}}
				done <- svc.ProxyResponsesWebSocketFromClient(active, c, conn, account, "fixture-only", first, hooks)
			})
			server := httptest.NewServer(r)
			defer server.Close()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer client.CloseNow()
			create := []byte(`{"type":"response.create","model":"gpt-5.1","input":[]}`)
			require.NoError(t, client.Write(ctx, coderws.MessageText, create))
			if scenario == "terminal" || scenario == "two_turns" {
				_, _, err = client.Read(ctx)
				require.NoError(t, err)
				if scenario == "two_turns" {
					require.NoError(t, client.Write(ctx, coderws.MessageText, create))
					_, _, err = client.Read(ctx)
					require.Error(t, err)
				} else {
					_ = client.Close(coderws.StatusNormalClosure, "done")
				}
			} else {
				for {
					_, _, err = client.Read(ctx)
					if err != nil {
						break
					}
				}
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("relay did not finish")
			}
			want := int32(1)
			if scenario == "two_turns" {
				want = 2
			}
			require.Equal(t, want, sends.Load())
			require.EqualValues(t, 1, dials.Load())
			rows, err := db.Query(`SELECT a.phase,e.attempt_id::text,a.execution_state FROM gateway_request_attempts a JOIN gateway_request_attempt_evidence e USING(request_id,attempt_no) ORDER BY a.started_at,a.attempt_no`)
			require.NoError(t, err)
			type row struct{ phase, id, state string }
			var all []row
			for rows.Next() {
				var v row
				require.NoError(t, rows.Scan(&v.phase, &v.id, &v.state))
				all = append(all, v)
			}
			require.NoError(t, rows.Close())
			modelNo := 0
			for _, v := range all {
				s := readWSEnd(t, db, v.id)
				if v.phase == "ws_connect" {
					require.Equal(t, "dial", s["stage"])
					require.Equal(t, "unknown", s["terminal"].(map[string]any)["state"])
					continue
				}
				modelNo++
				if scenario == "terminal" || scenario == "two_turns" && modelNo == 1 {
					require.Equal(t, "observed", s["terminal"].(map[string]any)["state"])
					require.Equal(t, "relay_turn_complete", s["stage_source"])
					require.Nil(t, s["wrote_downstream"])
					require.Equal(t, "succeeded", v.state)
				} else {
					require.Equal(t, "not_observed_in_scope", s["terminal"].(map[string]any)["state"])
					if scenario != "cancel" {
						require.Equal(t, "read_upstream", s["stage"])
					}
					if scenario == "cancel" {
						require.Equal(t, "cancelled", s["reason"])
						require.Equal(t, "context_cause", s["reason_source"])
					}
					require.Equal(t, false, s["positive_units_observed"])
					require.NotEqual(t, "succeeded", v.state)
					if scenario == "two_turns" {
						require.Equal(t, true, s["wrote_downstream"])
						require.Equal(t, "relay_session", s["wrote_downstream_scope"])
					}
				}
			}
			require.Equal(t, int(want), modelNo)
			var billings int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links`).Scan(&billings))
			require.Zero(t, billings)
		})
	}
}
