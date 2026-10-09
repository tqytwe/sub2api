//go:build unit

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Exercise real handlers and JSON serialization, with the existing repository
// fixture supplying deterministic records. Authentication is the contract
// suite's synthetic subject; this does not replace middleware security tests.
func TestUsageTPSHTTPContracts(t *testing.T) {
	deps := newContractDeps(t)
	rows := []service.UsageLog{
		{ID: 1, UserID: 1, RequestID: "request-one", Model: "text-model", OutputTokens: 200, DurationMs: ptr(10000), FirstTokenMs: ptr(9000), Stream: true, ActualCost: 1.25, UpstreamModel: ptr("internal-model"), CreatedAt: deps.now},
		{ID: 2, UserID: 1, RequestID: "request-two", Model: "text-model", OutputTokens: 50, DurationMs: ptr(1000), ActualCost: 0.75, CreatedAt: deps.now},
		{ID: 3, UserID: 1, RequestID: "request-history", Model: "historical-model", OutputTokens: 200, ActualCost: 0.5, CreatedAt: deps.now},
	}
	deps.usageRepo.SetUserLogs(1, rows)
	deps.usageRepo.SetUserLogs(2, []service.UsageLog{{ID: 4, UserID: 2, RequestID: "other-user", OutputTokens: 200, DurationMs: ptr(1000)}})

	for _, tc := range []struct {
		name  string
		path  string
		admin bool
	}{
		{"user", "/api/v1/usage?user_id=2", false}, // Cannot override the authenticated subject.
		{"admin", "/api/v1/admin/usage?user_id=1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readPage := func(query string) (map[string]any, []any) {
				t.Helper()
				status, body := doRequest(t, deps.router, http.MethodGet, tc.path+query, "", nil)
				require.Equal(t, http.StatusOK, status, body)
				var envelope struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, json.Unmarshal([]byte(body), &envelope))
				return envelope.Data, envelope.Data["items"].([]any)
			}
			page, items := readPage("&page=1&page_size=10")
			require.Equal(t, float64(3), page["total"])
			require.Len(t, items, 3)
			for i, wantTPS := range []any{float64(20), float64(50), nil} {
				item := items[i].(map[string]any)
				require.Contains(t, item, "output_tps")
				require.Equal(t, wantTPS, item["output_tps"])
				require.Equal(t, rows[i].RequestID, item["request_id"])
				require.Equal(t, rows[i].ActualCost, item["actual_cost"])
			}
			first := items[0].(map[string]any)
			if tc.admin {
				require.Equal(t, "internal-model", first["upstream_model"])
			} else {
				require.NotContains(t, first, "upstream_model")
				require.NotContains(t, first, "account_stats_cost")
			}

			page, items = readPage("&model=text-model&page=2&page_size=1")
			require.Equal(t, float64(2), page["total"])
			require.Equal(t, float64(2), page["page"])
			require.Equal(t, float64(2), page["pages"])
			require.Len(t, items, 1)
			require.Equal(t, "request-two", items[0].(map[string]any)["request_id"])
			require.Equal(t, float64(50), items[0].(map[string]any)["output_tps"])
		})
	}

	t.Run("detail ownership and JSON", func(t *testing.T) {
		status, body := doRequest(t, deps.router, http.MethodGet, "/api/v1/usage/1", "", nil)
		require.Equal(t, http.StatusOK, status)
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &envelope))
		require.Equal(t, float64(20), envelope.Data["output_tps"])
		require.Equal(t, 1.25, envelope.Data["actual_cost"])
		require.NotContains(t, envelope.Data, "upstream_model")
		status, body = doRequest(t, deps.router, http.MethodGet, "/api/v1/usage/4", "", nil)
		require.Equal(t, http.StatusForbidden, status)
		require.NotContains(t, body, "output_tps")
		require.NotContains(t, body, "other-user")
	})
}
