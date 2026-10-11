package requestledger

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestRoutingCaptureBoundsPrivacyAndSelectionNumbers(t *testing.T) {
	h := &Handle{ID: "00000000-0000-0000-0000-000000000001", route: "/v1/responses", method: "POST", kind: "http"}
	ctx := WithHandle(context.Background(), h)
	first, end := BeginRoutingSelection(ctx, true)
	RoutingVeto(first, "initial_filter", 11, "model_not_supported")
	end(12, false, false)
	second, end := BeginRoutingSelection(ctx, true)
	for range 200 {
		RoutingVeto(second, "db_recheck", 22, "privacy_not_set")
	}
	end(23, true, false)
	summary, events := h.routingSnapshot()
	require.Equal(t, 2, summary.SelectionCount)
	require.Equal(t, 128, len(events))
	require.Positive(t, summary.OmittedCount)
	require.Equal(t, 1, events[0].SelectionNo)
	require.Equal(t, 2, events[3].SelectionNo)
	data, err := json.Marshal(events)
	require.NoError(t, err)
	require.LessOrEqual(t, len(data), 32*1024)
	RoutingVeto(second, "db_recheck", 22, "secret-sentinel-DO-NOT-STORE")
	summary, events = h.routingSnapshot()
	require.True(t, summary.Incomplete)
	data, err = json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(data), "secret-sentinel")
}

func TestRoutingCoverageAndWhitelist(t *testing.T) {
	for _, route := range []string{"/v1/responses", "/responses", "/backend-api/codex/responses", "/v1/responses/*subpath", "/responses/*subpath", "/backend-api/codex/responses/*subpath", "/v1/messages", "/v1/chat/completions", "/chat/completions"} {
		require.True(t, routingEligible(route, "POST", "http"), route)
		require.False(t, routingEligible(route, "GET", "ws_session"), route)
	}
	for _, route := range []string{"/v1/images/generations", "/v1/embeddings", "/v1/messages/count_tokens", "/v1beta/models/:model", "/antigravity/v1/messages", "/responses/" + strings.Repeat("secret", 20)} {
		require.False(t, routingEligible(route, "POST", "http"), route)
	}
	h := &Handle{route: "/v1/messages", method: "POST", kind: "http"}
	ctx, done := BeginRoutingSelection(WithHandle(context.Background(), h), false)
	RoutingVeto(ctx, "initial_filter", 42, "excluded")
	done(42, false, false)
	summary, events := h.routingSnapshot()
	require.Equal(t, "unsupported_coverage", summary.Status)
	require.Empty(t, events)
}

func TestRoutingActualQuotaAndProfitReasons(t *testing.T) {
	h := &Handle{route: "/v1/responses", method: "POST", kind: "http"}
	ctx, end := BeginRoutingSelection(WithHandle(context.Background(), h), true)
	reasons := []string{"quota_auto_pause_5h", "quota_auto_pause_7d", "quota_auto_reset_pending_5h", "quota_auto_reset_pending_7d", "quota_auto_reset_credit_check_5h", "quota_auto_reset_credit_check_7d", "quota_auto_pause_retry_after", "quota_auto_pause_requests", "quota_auto_pause_tokens", "profit_threshold", "profit_invalid_account_rate"}
	for _, reason := range reasons {
		RoutingVeto(ctx, "initial_filter", 1, reason)
	}
	RoutingObservation(ctx, "fallback", 0, "load_read_failed")
	RoutingObservation(ctx, "admission", 1, "wait_counter_failed")
	end(1, true, false)
	summary, _ := h.routingSnapshot()
	require.False(t, summary.Incomplete)
	require.Equal(t, len(reasons), summary.VetoCount)
	require.NotPanics(t, func() { (&Ledger{}).persistRouting(routingBatch{}) }, "a writer panic cannot propagate into admission")
}
