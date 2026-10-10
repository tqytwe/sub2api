//go:build integration

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ledgerReviewHTTPUpstream struct{}

func (ledgerReviewHTTPUpstream) Do(r *http.Request, _ string, account int64, _ int) (*http.Response, error) {
	return (&http.Client{Transport: requestledger.Transport(http.DefaultTransport, account)}).Do(r)
}
func (u ledgerReviewHTTPUpstream) DoWithTLS(r *http.Request, p string, a int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, a, c)
}

func TestRequestLedgerBusinessStreamTerminalBeforeTransportEOF(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	terminal := `data: {"type":"response.completed","response":{"id":"fixture","status":"completed","output":[],"usage":{"input_tokens":7,"output_tokens":3}}}` + "\n\n"
	for _, name := range []string{"cc_fallback", "bare_error_then_completed", "long_terminal", "long_terminal_chat", "long_terminal_messages", "missing_terminal"} {
		t.Run(name, func(t *testing.T) {
			payload := terminal
			switch name {
			case "cc_fallback":
				payload = "data: {\"id\":\"fixture\",\"model\":\"gpt-5.1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"synthetic\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n"
			case "bare_error_then_completed":
				payload = "data: {\"type\":\"error\",\"error\":{\"code\":\"transient\",\"message\":\"synthetic retry\"}}\n\n" + terminal
			case "long_terminal", "long_terminal_chat", "long_terminal_messages":
				payload = strings.Replace(terminal, `"output":[]`, `"output":[{"type":"message","content":[{"type":"output_text","text":"`+strings.Repeat("x", 128*1024)+`"}]}]`, 1)
			case "missing_terminal":
				payload = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"synthetic\"}\n\n"
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, payload)
				w.(http.Flusher).Flush()
				if name != "missing_terminal" {
					<-r.Context().Done()
				}
			}))
			defer upstream.Close()
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			service := &OpenAIGatewayService{cfg: cfg, httpUpstream: ledgerReviewHTTPUpstream{}}
			account := &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"base_url": upstream.URL, "api_key": "disabled-fixture"}}
			router := gin.New()
			router.Use(requestledger.Middleware(ledger))
			var id string
			router.POST("/v1/responses", func(c *gin.Context) {
				id = requestledger.FromContext(c.Request.Context()).ID
				require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
				ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
				defer cancel()
				var err error
				if name == "cc_fallback" {
					account.Type = AccountTypeAPIKey
					_, err = service.forwardResponsesViaRawChatCompletions(ctx, c, account, []byte(`{"model":"gpt-5.1","input":"synthetic","stream":true}`))
				} else {
					req, reqErr := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
					require.NoError(t, reqErr)
					resp, sendErr := service.httpUpstream.Do(req, "", 234, 1)
					require.NoError(t, sendErr)
					defer func() { _ = resp.Body.Close() }()
					switch name {
					case "long_terminal_chat":
						_, err = service.handleChatStreamingResponse(resp, c, account, "gpt-5.1", "gpt-5.1", "gpt-5.1", time.Now(), 0)
					case "long_terminal_messages":
						_, err = service.handleAnthropicStreamingResponse(resp, c, account, "gpt-5.1", "gpt-5.1", "gpt-5.1", time.Now())
					default:
						_, err = service.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
					}
				}
				if name != "missing_terminal" {
					require.NoError(t, err)
				}
			})
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, httptest.NewRequest("POST", "/v1/responses", nil))
			var requestState, attemptState, usage string
			require.NoError(t, db.QueryRow(`SELECT r.execution_state,a.execution_state,r.usage_state FROM gateway_requests r JOIN gateway_request_attempts a ON a.request_id=r.id WHERE r.id=$1`, id).Scan(&requestState, &attemptState, &usage))
			if name == "missing_terminal" {
				require.NotEqual(t, "succeeded", requestState)
				require.NotEqual(t, "succeeded", attemptState)
				require.Equal(t, "usage_unknown", usage)
			} else {
				require.Equal(t, "succeeded", attemptState)
				require.Equal(t, "succeeded", requestState)
			}
		})
	}
}

func TestRequestLedgerOAuthModelsDirectTransportAdmission(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	h, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	var sends int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends++
		var count int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&count))
		require.Equal(t, 1, count)
		_, _ = io.WriteString(w, `{"models":[]}`)
	}))
	defer upstream.Close()
	s := &OpenAIGatewayService{}
	_, err = s.fetchOpenAIModelsUpstream(ctx, openAIModelsRequest{url: upstream.URL, headers: http.Header{}, accountID: 234, credentialAccountID: 22, credentialAccount: &Account{ID: 22}}, "")
	require.NoError(t, err)
	require.Equal(t, 1, sends)
	var account, mother int64
	var usage string
	require.NoError(t, db.QueryRow(`SELECT account_id,credential_account_id,usage_state FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&account, &mother, &usage))
	require.EqualValues(t, 234, account)
	require.EqualValues(t, 22, mother)
	require.Equal(t, "not_applicable", usage)
	_, err = db.Exec(`ALTER TABLE gateway_request_attempts ADD CONSTRAINT synthetic_send_fault CHECK(false) NOT VALID`)
	require.NoError(t, err)
	defer func() { _, _ = db.Exec(`ALTER TABLE gateway_request_attempts DROP CONSTRAINT synthetic_send_fault`) }()
	_, err = s.fetchOpenAIModelsUpstream(ctx, openAIModelsRequest{url: upstream.URL, headers: http.Header{}, accountID: 234, credentialAccountID: 22}, "")
	require.Error(t, err)
	require.Equal(t, 1, sends, "storage failure must reject this send")
}

type ledgerReviewImageUpstream struct{ target string }

func (u ledgerReviewImageUpstream) Do(r *http.Request, _ string, account int64, _ int) (*http.Response, error) {
	copy := r.Clone(r.Context())
	target, err := url.Parse(u.target)
	if err != nil {
		return nil, err
	}
	copy.URL.Scheme = target.Scheme
	copy.URL.Host = target.Host
	return (&http.Client{Transport: requestledger.Transport(http.DefaultTransport, account)}).Do(copy)
}
func (u ledgerReviewImageUpstream) DoWithTLS(r *http.Request, p string, a int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, a, c)
}

func TestRequestLedgerImageBackfillKeepsGenerationAttribution(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	for _, status := range []int{200, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":[{"url":"https://cdn.example.invalid/image.png"}]}`)
					return
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0})
			}))
			defer upstream.Close()
			h, err := ledger.Begin(context.Background(), "/v1/images/generations", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
			u := ledgerReviewImageUpstream{target: upstream.URL}
			s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: u}
			account := &Account{ID: 234, Extra: map[string]any{AccountExtraImagesURLToB64JSON: true}}
			req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
			require.NoError(t, err)
			resp, err := u.Do(req, "", 234, 1)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			out := s.backfillOpenAIImagesB64JSON(ctx, account, nil, body)
			if status == 200 {
				require.Contains(t, string(out), "b64_json")
			} else {
				require.Equal(t, body, out)
			}
			require.NoError(t, requestledger.ObserveUsage(ctx, 234))
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			var state, usage, phase string
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state, &usage))
			require.Equal(t, "succeeded", state)
			require.Equal(t, "known", usage)
			require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&usage))
			require.Equal(t, "known", usage)
			require.NoError(t, db.QueryRow(`SELECT phase,usage_state,execution_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=2`, h.ID).Scan(&phase, &usage, &state))
			require.Equal(t, "auxiliary", phase)
			require.Equal(t, "not_applicable", usage)
			if status == 500 {
				require.Equal(t, "failed", state)
			}
		})
	}
}

func TestRequestLedgerModelsCacheRefreshKeepsDurableChild(t *testing.T) {
	db := ledgertest.New(t)
	for _, stale := range []bool{false, true} {
		t.Run(fmt.Sprintf("stale_%v", stale), func(t *testing.T) {
			ledger := requestledger.New(db)
			h, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
			entered := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var count int
				queryErr := db.QueryRow(`SELECT count(*) FROM gateway_request_attempts a JOIN gateway_requests r ON r.id=a.request_id WHERE r.parent_id=$1 AND r.kind='async_execution' AND r.user_id=101 AND r.api_key_id=201 AND a.account_id=234 AND a.credential_account_id=22`, h.ID).Scan(&count)
				assert.NoError(t, queryErr)
				assert.Equal(t, 1, count, "real cache refresh must have durable child/attempt before send")
				close(entered)
				if stale {
					<-release
				}
				_, _ = io.WriteString(w, `{"models":[]}`)
			}))
			defer upstream.Close()
			request := openAIModelsRequest{url: upstream.URL, headers: http.Header{}, accountID: 234, credentialAccountID: 22, credentialAccount: &Account{ID: 22}}
			s := &OpenAIGatewayService{}
			fetch := s.fetchCodexModelsManifestUpstreamForRequest(request)
			if stale {
				s.openAIModelsCache.set(buildOpenAIModelsCacheKey(request), &OpenAIModelsResponse{Body: []byte(`{"models":[]}`)}, time.Now().Add(-2*openAIModelsCacheTTL))
			}
			_, err = s.fetchCachedOpenAIModels(ctx, request, fetch, "")
			require.NoError(t, err)
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("refresh did not reach loopback upstream")
			}
			// The original HTTP request may have ended while the singleflight refresh continues.
			if stale {
				require.NoError(t, ledger.Recover(context.Background()))
				close(release)
			}
			var child, state, usage string
			require.Eventually(t, func() bool {
				return db.QueryRow(`SELECT id,execution_state,usage_state FROM gateway_requests WHERE parent_id=$1`, h.ID).Scan(&child, &state, &usage) == nil && state == "succeeded"
			}, 5*time.Second, 10*time.Millisecond)
			require.Equal(t, "not_applicable", usage)
			_, err = ledger.Get(context.Background(), requestledger.Viewer{UserID: 102}, child)
			require.Error(t, err)
			// A fresh cache consumer has its own admission but causes no additional send.
			h2, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
			require.NoError(t, err)
			ctx2 := requestledger.WithHandle(context.Background(), h2)
			require.NoError(t, requestledger.BindIdentity(ctx2, 102, 202))
			_, err = s.fetchCachedOpenAIModels(ctx2, request, fetch, "")
			require.NoError(t, err)
			require.NoError(t, h2.Finish(ctx2, "succeeded", 200, ""))
			require.EqualValues(t, 1, calls.Load())
		})
	}
}

func TestRequestLedgerModelsCacheStorageFailureNeverSends(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	h, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	_, err = db.Exec(`ALTER TABLE gateway_requests ADD CONSTRAINT synthetic_query_child_fault CHECK(kind<>'async_execution') NOT VALID`)
	require.NoError(t, err)
	defer func() { _, _ = db.Exec(`ALTER TABLE gateway_requests DROP CONSTRAINT synthetic_query_child_fault`) }()
	var sends atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sends.Add(1); _, _ = io.WriteString(w, `{"models":[]}`) }))
	defer upstream.Close()
	request := openAIModelsRequest{url: upstream.URL, headers: http.Header{}, accountID: 234, credentialAccountID: 22}
	s := &OpenAIGatewayService{}
	_, err = s.fetchCachedOpenAIModels(ctx, request, s.fetchCodexModelsManifestUpstreamForRequest(request), "")
	require.Error(t, err)
	require.Zero(t, sends.Load(), "background cache refresh must not bypass durable admission")
}
