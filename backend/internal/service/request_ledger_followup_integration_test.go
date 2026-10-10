//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"github.com/gin-gonic/gin"
	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func ledgerFollowupContext(t *testing.T, l *requestledger.Ledger, route, kind string) (context.Context, *requestledger.Handle, *gin.Context) {
	t.Helper()
	h, err := l.Begin(context.Background(), route, "POST", kind)
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	ctx = requestledger.WithAccount(ctx, 234, 22)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", route, nil).WithContext(ctx)
	return ctx, h, c
}

func ledgerFollowupState(t *testing.T, db *sql.DB, h *requestledger.Handle, want string) {
	t.Helper()
	var parent, attempt string
	require.NoError(t, db.QueryRow(`SELECT r.execution_state,a.execution_state FROM gateway_requests r JOIN gateway_request_attempts a ON a.request_id=r.id WHERE r.id=$1`, h.ID).Scan(&parent, &attempt))
	require.Equal(t, want, attempt, "attempt must match the actual business terminal")
	require.Equal(t, want, parent, "parent must preserve the terminal result")
}

func TestRequestLedgerNativeAndPassthroughTerminalStatus(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	for _, path := range []string{"native", "passthrough"} {
		for _, terminal := range []struct{ kind, status, want string }{
			{"response.incomplete", "incomplete", "failed"},
			{"response.done", "failed", "failed"}, {"response.done", "incomplete", "failed"},
			{"response.done", "cancelled", "cancelled"}, {"response.done", "canceled", "cancelled"},
			{"response.completed", "completed", "succeeded"},
		} {
			t.Run(path+"/"+terminal.kind+"/"+terminal.status, func(t *testing.T) {
				ctx, h, c := ledgerFollowupContext(t, l, "/v1/responses", "http")
				payload := fmt.Sprintf("data: {\"type\":%q,\"response\":{\"id\":\"synthetic-terminal\",\"status\":%q,\"output\":[],\"usage\":{\"input_tokens\":7,\"output_tokens\":3}}}\n\n", terminal.kind, terminal.status)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, payload)
				}))
				defer server.Close()
				req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
				require.NoError(t, err)
				resp, err := (ledgerReviewHTTPUpstream{}).Do(req, "", 234, 1)
				require.NoError(t, err)
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
				if path == "native" {
					_, _ = svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				} else {
					_, _ = svc.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				}
				require.NoError(t, resp.Body.Close())
				require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
				ledgerFollowupState(t, db, h, terminal.want)
			})
		}
	}
}

func TestRequestLedgerBridgeAndOAuthImagesTerminal(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	for _, name := range []string{"bridge_bare_error_completed", "bridge_large_completed", "images_nonstream_large", "images_stream_large"} {
		t.Run(name, func(t *testing.T) {
			ctx, h, c := ledgerFollowupContext(t, l, "/v1/images/generations", "http")
			payload := `data: {"type":"response.completed","response":{"id":"synthetic-terminal","status":"completed","output":[],"usage":{"input_tokens":7,"output_tokens":3}}}` + "\n\n"
			switch name {
			case "bridge_bare_error_completed":
				payload = "data: {\"type\":\"error\",\"error\":{\"code\":\"transient\",\"message\":\"synthetic\"}}\n\n" + payload
			case "bridge_large_completed":
				payload = strings.Replace(payload, `"output":[]`, `"output":[{"type":"message","content":[{"type":"output_text","text":"`+strings.Repeat("x", 128*1024)+`"}]}]`, 1)
			default:
				payload = strings.Replace(payload, `"output":[]`, `"output":[{"type":"image_generation_call","id":"synthetic-image","status":"completed","result":"`+strings.Repeat("YQ==", 32*1024)+`","output_format":"png","size":"1024x1024"}]`, 1)
			}
			var sends int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends++
				var durable int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&durable))
				require.Equal(t, 1, durable)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, payload)
				w.(http.Flusher).Flush()
				if name != "images_nonstream_large" {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: ledgerReviewImageUpstream{target: server.URL}}
			account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "disabled-fixture"}}
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			if strings.HasPrefix(name, "bridge") {
				body := []byte(`{"type":"response.create","model":"gpt-5.1","stream":true,"input":"synthetic"}`)
				result, err := svc.proxyOpenAIWSHTTPBridgeTurn(ctx, c, account, "disabled-fixture", body, len(body), "gpt-5.1", "", "", "", "", 1, func([]byte) error { return nil })
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 7, result.Usage.InputTokens)
			} else {
				ctx = withOpenAIImagesForceResponses(ctx)
				c.Request = c.Request.WithContext(ctx)
				result, err := svc.forwardOpenAIImagesOAuth(ctx, c, account, &OpenAIImagesRequest{Endpoint: openAIImagesGenerationsEndpoint, Model: "gpt-image-2", Prompt: "synthetic", ResponseFormat: "b64_json", N: 1, Stream: name == "images_stream_large"}, "")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, result.ImageCount)
			}
			require.Equal(t, 1, sends)
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			ledgerFollowupState(t, db, h, "succeeded")
		})
	}
}

func TestRequestLedgerLiveStoppedTurnRecoversHealthyOwner(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			ctx, h, _ := ledgerFollowupContext(t, l, "/v1/live", "async_execution")
			audit := newRealtimeLedgerAudit(ctx, &Account{ID: 234})
			require.NoError(t, audit.Observe([]byte(`{"type":"response.created","response":{"id":"synthetic-live"}}`)))
			_, err := db.Exec(`ALTER TABLE gateway_request_attempts ADD CONSTRAINT synthetic_live_finish_fault CHECK (execution_state='inflight') NOT VALID`)
			require.NoError(t, err)
			if terminal {
				require.Error(t, audit.Observe([]byte(`{"type":"response.done","response":{"id":"synthetic-live","status":"completed"}}`)))
			}
			audit.Close(nil)
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			_, err = db.Exec(`ALTER TABLE gateway_request_attempts DROP CONSTRAINT synthetic_live_finish_fault`)
			require.NoError(t, err)
			require.NoError(t, l.Recover(ctx))
			var state, usage string
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE parent_id=$1`, h.ID).Scan(&state, &usage))
			require.Equal(t, "interrupted", state)
			require.Equal(t, "usage_unknown", usage)
		})
	}
}

type ledgerModelsPlugin struct {
	pluginv1.TransportPluginClient
	stream *ledgerModelsPluginStream
}

func (p *ledgerModelsPlugin) Forward(context.Context, ...grpc.CallOption) (pluginv1.TransportPlugin_ForwardClient, error) {
	return p.stream, nil
}

type ledgerModelsPluginStream struct {
	pluginv1.TransportPlugin_ForwardClient
	t     *testing.T
	db    *sql.DB
	id    string
	index int
}

func (s *ledgerModelsPluginStream) Send(f *pluginv1.ForwardRequest) error {
	if start := f.GetStart(); start != nil {
		require.EqualValues(s.t, 22, start.AccountId, "plugin still receives the credential account")
		var scheduled, mother int64
		require.NoError(s.t, s.db.QueryRow(`SELECT account_id,credential_account_id FROM gateway_request_attempts WHERE request_id=$1`, s.id).Scan(&scheduled, &mother))
		require.EqualValues(s.t, 234, scheduled)
		require.EqualValues(s.t, 22, mother)
	}
	return nil
}
func (s *ledgerModelsPluginStream) CloseSend() error { return nil }
func (s *ledgerModelsPluginStream) Recv() (*pluginv1.ForwardResponse, error) {
	s.index++
	switch s.index {
	case 1:
		return &pluginv1.ForwardResponse{Frame: &pluginv1.ForwardResponse_Start{Start: &pluginv1.ForwardResponseStart{StatusCode: 200}}}, nil
	case 2:
		return &pluginv1.ForwardResponse{Frame: &pluginv1.ForwardResponse_BodyChunk{BodyChunk: []byte(`{"models":[]}`)}}, nil
	case 3:
		return &pluginv1.ForwardResponse{Frame: &pluginv1.ForwardResponse_End{End: &pluginv1.ForwardResponseEnd{}}}, nil
	default:
		return nil, io.EOF
	}
}
func TestRequestLedgerModelsPluginPreservesScheduledAccount(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx, h, _ := ledgerFollowupContext(t, l, "/v1/models", "http")
	stream := &ledgerModelsPluginStream{t: t, db: db, id: h.ID}
	manager := &PluginManager{}
	manager.route.Store(&pluginRoute{rolloutPercent: 100, runtime: &pluginRuntime{client: &hcplugin.Client{}, api: &ledgerModelsPlugin{stream: stream}}})
	svc := &OpenAIGatewayService{pluginManager: manager}
	_, err := svc.fetchOpenAIModelsUpstream(ctx, openAIModelsRequest{url: "https://models.example.invalid", headers: http.Header{}, accountID: 234, credentialAccountID: 22, credentialAccount: &Account{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeOAuth}}, "")
	require.NoError(t, err)
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
}
