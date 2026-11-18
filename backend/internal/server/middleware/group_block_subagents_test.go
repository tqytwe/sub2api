package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupSubagentsPolicyIsIndependentOfClientGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		for _, official := range []bool{false, true} {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{BlockSubagents: enabled}})
			})
			r.Use(GroupCodexCLIOnly(nil), GroupBlockSubagents())
			body := `{"model":"gpt-test","client_metadata":{"x-openai-subagent":"worker"}}`
			calls := 0
			r.POST("/v1/responses", func(c *gin.Context) {
				calls++
				actual, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(actual))
				c.Status(http.StatusNoContent)
			})
			w := httptest.NewRecorder()
			req := codexGroupRequest(http.MethodPost, "/v1/responses", body, official)
			req.Header.Set("X-Sub2API-Internal", "true")
			r.ServeHTTP(w, req)
			if enabled {
				require.Equal(t, http.StatusForbidden, w.Code)
				require.Contains(t, w.Body.String(), "subagents_blocked")
				require.Zero(t, calls)
			} else {
				require.Equal(t, http.StatusNoContent, w.Code)
				require.Equal(t, 1, calls)
			}
		}
	}
}

func TestGroupSubagentsWSChecksEveryOriginalFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{BlockSubagents: true}})
	})
	r.Use(GroupBlockSubagents())
	r.GET("/v1/responses", func(c *gin.Context) {
		// Composite routing and upstream-added headers cannot change the policy
		// or turn an ordinary main thread into a subagent.
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{}})
		c.Request.Header.Set("X-OpenAI-Subagent", "upstream-generated")
		require.False(t, CheckGroupSubagentFrame(c, []byte(`{"type":"response.create","model":"gpt-test"}`)))
		for turn := 0; turn < 2; turn++ {
			require.True(t, CheckGroupSubagentFrame(c, []byte(`{"type":"response.create","response":{"client_metadata":{"x-openai-subagent":"worker"}}}`)))
		}
		c.Status(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, codexGroupRequest(http.MethodGet, "/v1/responses", "", true))
	require.Equal(t, http.StatusNoContent, w.Code)
}
