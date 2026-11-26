//go:build unit

package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestLegacyCompatibilityHTTPUsesOrdinaryAccountPoolRetry(t *testing.T) {
	for _, legacyMode := range []bool{false, true} {
		name := "strict owner-bound mode"
		if legacyMode {
			name = "legacy compatibility mode"
		}
		t.Run(name, func(t *testing.T) {
			upstream := &issue715Upstream{}
			handler := newOpenAIResponsesFailoverTestHandler(t, upstream, issue715Accounts()...)
			handler.cfg.Gateway.OpenAILegacyCompatibilityMode = legacyMode

			status, first := issue715Request(t, handler, issue715Body(false, false), 99, "legacy-compatibility-pool", false)
			require.Equal(t, 200, status, first)
			upstream.mu.Lock()
			upstream.mode = "pool_failure"
			upstream.mu.Unlock()

			_, _ = issue715Request(t, handler, issue715Body(true, false), 99, "legacy-compatibility-pool", false)
			if legacyMode {
				require.Equal(t, []int64{1, 1, 2}, upstream.ids,
					"legacy mode should use normal same-account and cross-account retry after an ambiguous result submission")
				require.Equal(t, upstream.bodies[1], upstream.bodies[2],
					"ordinary retry must preserve the submitted request instead of rebuilding or stripping history")
			} else {
				require.Equal(t, []int64{1, 1}, upstream.ids,
					"strict owner-bound mode must not move a tool-result turn to a different account")
			}
		})
	}
}

func TestLegacyCompatibilityWSUsesOrdinaryRecoveryWithoutPublishingOwners(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeHTTPBridge} {
		t.Run(mode, func(t *testing.T) {
			clients := issue805Redis(t)
			u := &issue805Oracle{}
			srv := u.server(t)
			h := issue805WSNode(t, mode, srv.URL, repository.NewGatewayCache(clients()), true, false, true)

			issue715WSSend(t, h.clientConn, issue715WSCreate("", false))
			first := issue715WSRead(t, h.clientConn)
			require.Equal(t, "response.completed", gjson.GetBytes(first, "type").String(), string(first))

			secondBody := issue715WSCreate(gjson.GetBytes(first, "response.id").String(), true)
			secondBody = strings.Replace(secondBody, `"store":true`, `"store":true,"instructions":"changed legacy continuation"`, 1)
			issue715WSSend(t, h.clientConn, secondBody)
			second := issue715WSRead(t, h.clientConn)
			require.Equal(t, "response.completed", gjson.GetBytes(second, "type").String(), string(second))
			u.mu.Lock()
			frames := append([][]byte(nil), u.frames...)
			u.mu.Unlock()
			require.Len(t, frames, 2)
			require.Contains(t, string(frames[1]), "changed legacy continuation")
			require.Eventually(t, func() bool {
				return len(issue805Owners(t, clients())) == 0
			}, time.Second, 10*time.Millisecond, "legacy compatibility must not publish strict owner records")
		})
	}
}
