//go:build integration

package routes

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerRoutingActualUnsupportedDispatch(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformGemini} {
		for _, path := range []string{"/v1/messages", "/v1/responses", "/responses", "/backend-api/codex/responses", "/v1/chat/completions", "/chat/completions"} {
			t.Run(platform+path, func(t *testing.T) {
				r := gin.New()
				r.Use(requestledger.Middleware(l))
				auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
					groupID := int64(123)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 201, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: platform}})
					// Omit the subject so the real provider handler rejects locally,
					// after dispatch but before attempting any upstream operation.
					c.Next()
				})
				RegisterGatewayRoutes(r, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil, nil)}, auth, nil, nil, nil, nil, nil, &config.Config{})
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(`{"model":"fixture"}`)))
				require.Equal(t, 500, w.Code)
				var id string
				require.NoError(t, db.QueryRow(`SELECT id FROM gateway_requests ORDER BY started_at DESC LIMIT 1`).Scan(&id))
				e, err := l.RoutingEvidence(context.Background(), requestledger.Viewer{Admin: true, UserID: 1}, id)
				require.NoError(t, err)
				require.Equal(t, "unsupported_coverage", e.Status)
				require.Zero(t, e.SelectionCount)
			})
		}
	}
}
