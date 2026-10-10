package requestledger

import "strings"

// Classification is audit metadata only. Existing billing policy is unchanged.
// Retrieval, control and quota calls still have durable request/attempt rows.
func meteredRoute(route, method, kind string) bool {
	if kind == "ws_turn" || kind == "async_execution" {
		return true
	}
	if kind == "ws_session" {
		return false
	}
	if method != "POST" {
		return false
	}
	for _, suffix := range []string{"/count_tokens", "/input_tokens", "/estimate", "/cancel", "/ack", "/references"} {
		if strings.HasSuffix(route, suffix) {
			return false
		}
	}
	return !strings.Contains(route, "/custom-voices")
}
