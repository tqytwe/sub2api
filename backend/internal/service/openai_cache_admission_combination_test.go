//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A cache-compatible HTTP 400 does not authorize a second send after the
// selected account becomes ineligible or its original route binding changes.
func TestAdmissionCacheContentRetryRechecksLatestAccount(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, change := range []string{"disabled", "route", "model"} {
			t.Run(fmt.Sprintf("passthrough=%v/%s", passthrough, change), func(t *testing.T) {
				selected := turnAdmissionAccount()
				selected.Extra = map[string]any{"openai_passthrough": passthrough}
				repo := &turnAdmissionRepo{account: selected}
				firstBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(`{"error":{"code":"invalid_parameter","param":"input[0].content[0].prompt_cache_breakpoint","message":"input[0].content[0].prompt_cache_breakpoint is not supported on this model"}}`)}
				upstream := &passthroughAdmissionUpstream{
					httpUpstreamRecorder: &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: firstBody}, passthroughAdmissionResponse()}},
					afterFirst: func() {
						switch change {
						case "disabled":
							selected.Schedulable = false
						case "route":
							selected.Credentials["base_url"] = "http://changed.example"
						case "model":
							selected.Credentials["model_mapping"] = map[string]any{"gpt-5.4": "gpt-5.5"}
						}
					},
				}
				svc := newTurnAdmissionGateway(repo, false)
				svc.httpUpstream = upstream
				body := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello","prompt_cache_breakpoint":true}]}],"stream":false}`)
				c := adaptiveProtocolTestContext("/v1/responses", body)
				result, err := svc.Forward(context.Background(), c, selected, body)
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, result)
				require.Len(t, upstream.requests, 1)
				require.True(t, firstBody.closed)
				var failover *UpstreamFailoverError
				require.False(t, errors.As(err, &failover))
			})
		}
	}
}
