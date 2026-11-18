package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeferredGroupPoliciesUseSelectedGroup(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		gin.SetMode(gin.TestMode)
		route := gin.New()
		route.Use(func(c *gin.Context) {
			c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{CodexCLIOnly: !enabled, BlockSubagents: !enabled}})
			SetDeferredAPIKeyRequestGroup(c, func([]string) (*service.APIKey, error) { return nil, nil })
			c.Next()
		})
		route.Use(GroupCodexCLIOnly(nil), GroupBlockSubagents())
		var reached bool
		route.GET("/v1/responses", func(c *gin.Context) {
			reached = true
			c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{CodexCLIOnly: enabled, BlockSubagents: enabled}})
			result := CheckGroupCodexClientFrame(c, []byte(`{"input":"hello"}`))
			require.Equal(t, enabled, result.Enabled)
			require.False(t, result.Matched)
			require.Equal(t, enabled, CheckGroupSubagentFrame(c, []byte(`{"client_metadata":{"x-openai-subagent":"worker"}}`)))
			// Composite routing must not replace the originally admitted policy.
			c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{}})
			require.Equal(t, enabled, CheckGroupCodexClientFrame(c, nil).Enabled)
			require.Equal(t, enabled, CheckGroupSubagentFrame(c, []byte(`{"client_metadata":{"x-openai-subagent":"worker"}}`)))
		})
		response := httptest.NewRecorder()
		route.ServeHTTP(response, httptest.NewRequest("GET", "/v1/responses", nil))
		require.True(t, reached, "WS policy must wait for actual group selection")
	}
}
