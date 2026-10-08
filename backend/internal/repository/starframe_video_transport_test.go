//go:build unit

package repository

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestStarframeTransportKeepsAllEndpointsOnOriginalOrigin(t *testing.T) {
	for _, endpoint := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/v1/videos"},
		{"status", http.MethodGet, "/v1/videos/task-1"},
		{"content", http.MethodGet, "/v1/videos/task-1/content"},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			var hosts []string
			var identities []string
			transport := &grokAccessDeniedFallbackTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				hosts = append(hosts, req.URL.Hostname())
				identities = append(identities, req.Header.Get("X-XAI-Token-Auth"))
				status := http.StatusForbidden
				if req.URL.Hostname() != grokCLIProxyHost {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"Access denied"}`)), Request: req}, nil
			})}
			ctx := service.WithStarframeVideoRequest(t.Context())
			ctx = service.WithHTTPUpstreamRedirectsDisabled(ctx)
			// Empty GET bodies still have GetBody, as in the gateway's bytes reader.
			req, err := http.NewRequestWithContext(ctx, endpoint.method, "https://"+grokCLIProxyHost+endpoint.path, bytes.NewReader([]byte{}))
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer test-only")
			applyGrokCLIProxyHeaders(req)
			resp, err := transport.RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, []string{grokCLIProxyHost}, hosts)
			require.Equal(t, []string{""}, identities)
			require.Equal(t, http.StatusForbidden, resp.StatusCode)
			require.Empty(t, req.Header.Get("x-grok-client-version"))
		})
	}
}
