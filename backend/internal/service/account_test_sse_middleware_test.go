//go:build unit

package service_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type timingProbeRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r *timingProbeRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}
func (*timingProbeRepo) UpdateExtra(context.Context, int64, map[string]any) error { return nil }
func (*timingProbeRepo) ClearError(context.Context, int64) error                  { return nil }
func (*timingProbeRepo) SetError(context.Context, int64, string) error            { return nil }
func (*timingProbeRepo) SetRateLimited(context.Context, int64, time.Time) error   { return nil }

type timingProbeUpstream struct{ target *url.URL }

func (u *timingProbeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	forward := req.Clone(req.Context())
	target := *u.target
	target.Path = req.URL.Path
	forward.URL = &target
	forward.Host = ""
	return http.DefaultClient.Do(forward)
}
func (u *timingProbeUpstream) DoWithTLS(req *http.Request, p string, a int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, p, a, c)
}

// Use the real global middleware and public service entry, not a timing-writer
// imitation. Bound the subprocess so a recursive writer deadlock produces a
// stack trace and fails without wedging every subsequent repository gate.
func TestAccountTestServerTimingHTTP(t *testing.T) {
	if os.Getenv("SUB2API_TIMING_PROBE_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAccountTestServerTimingHTTP$", "-test.v", "-test.timeout=30s")
		cmd.Env = append(os.Environ(), "SUB2API_TIMING_PROBE_CHILD=1")
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		require.NoError(t, ctx.Err(), "bounded middleware probe exceeded deadline")
		require.NoError(t, err, "real ServerTiming integration failed")
		return
	}
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"complete", "disconnect", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			entered := make(chan struct{})
			upstreamDone := make(chan struct{})
			release := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(upstreamDone)
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				close(entered)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"before\"}\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-release:
				}
				_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"after\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
			}))
			defer upstream.Close()
			target, err := url.Parse(upstream.URL)
			require.NoError(t, err)
			repo := &timingProbeRepo{account: &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "fixture-only", "base_url": "https://fixture.example"}, Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: true}}}
			s := service.NewAccountTestService(repo, nil, nil, nil, nil, &timingProbeUpstream{target}, &config.Config{}, nil)
			result := make(chan error, 1)
			returned := make(chan struct{})
			engine := gin.New()
			engine.Use(middleware.ServerTiming(true))
			engine.GET("/api/v1/admin/accounts/1/test", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserRole), "admin")
				result <- s.TestAccountConnection(c, 1, "gpt-test", "fixture", service.AccountTestModeDefault)
			})
			downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(returned); engine.ServeHTTP(w, r) }))
			defer downstream.Close()
			deadline := 15 * time.Second
			if mode == "timeout" {
				deadline = 2 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, downstream.URL+"/api/v1/admin/accounts/1/test", nil)
			require.NoError(t, err)
			client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{ResponseHeaderTimeout: 2 * time.Second}}
			defer client.CloseIdleConnections()
			resp, err := client.Do(req)
			require.NoError(t, err, "first headers must not deadlock under enabled ServerTiming")
			defer resp.Body.Close()
			require.NotEmpty(t, resp.Header.Get("Server-Timing"), "timing must stay enabled for authorized admin")
			scanner := bufio.NewScanner(resp.Body)
			require.True(t, scanner.Scan())
			require.Equal(t, ": keepalive", scanner.Text(), "first bytes must be flushed before probe output")
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream was not reached after first heartbeat")
			}
			body := ": keepalive\n"
			beats := 1
			seenContent := false
			if mode == "complete" {
				released := false
				for scanner.Scan() {
					line := scanner.Text()
					body += line + "\n"
					if strings.Contains(line, `"text":"before"`) {
						seenContent = true
					}
					if line == ": keepalive" {
						beats++
						if seenContent && !released {
							close(release)
							released = true
						}
					}
				}
				require.NoError(t, scanner.Err())
				require.True(t, released, "heartbeat must continue after real content with timing finalized")
				require.GreaterOrEqual(t, beats, 2)
				require.Equal(t, 1, strings.Count(body, `"type":"test_complete"`))
				require.Contains(t, body, `"text":"after"`)
			} else if mode == "disconnect" {
				_ = resp.Body.Close()
				cancel()
			} else {
				// A transport timeout after headers must release both real HTTP handlers.
				for scanner.Scan() {
				}
				require.Error(t, scanner.Err())
				require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
			}
			select {
			case err := <-result:
				if mode == "complete" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("probe did not release after completion/cancellation")
			}
			for _, done := range []<-chan struct{}{returned, upstreamDone} {
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("middleware or upstream handler leaked")
				}
			}
		})
	}
}
