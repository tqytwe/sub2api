package planacceptance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opt-in because the browser runner requires Playwright and a frontend build.
// Tokens are generated for disposable DB users and passed only in child env.
func TestAdminPlanBrowserContract(t *testing.T) {
	if os.Getenv("PLAN_BROWSER_CONTRACT") != "1" {
		t.Skip("set PLAN_BROWSER_CONTRACT=1 to run the real browser/HTTP/DB contract")
	}
	t.Run("description_only", func(t *testing.T) { runPlanBrowserContract(t, false) })
	t.Run("explicit_whitespace_clear", func(t *testing.T) { runPlanBrowserContract(t, true) })
}

func runPlanBrowserContract(t *testing.T, clearWhitespace bool) {
	t.Helper()
	f := newPlanContract(t)
	if clearWhitespace {
		var err error
		f.plan, err = f.client.SubscriptionPlan.UpdateOneID(f.plan.ID).
			SetProductName("   ").SetDetailDescription("   ").SetStorefrontBadge("   ").Save(context.Background())
		require.NoError(t, err)
	}
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	assets := t.TempDir()
	// A local fixture image keeps the non-default persisted URL resolvable;
	// no image request leaves the disposable HTTP server.
	f.router.GET("/contract-cover.svg", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="160" height="100"><rect width="160" height="100" fill="#e4e4df"/><text x="20" y="55" fill="#171717">Contract plan</text></svg>`))
	})
	f.router.NoRoute(gin.WrapH(http.FileServer(http.Dir(assets))))
	server := httptest.NewServer(f.router)
	t.Cleanup(server.Close)
	before := contractPlanJSON(t, f.plan)
	cmd := exec.CommandContext(context.Background(), "node", filepath.Join(root, "frontend/e2e/plan-edit/run.mjs"))
	cmd.Env = append(os.Environ(), "PLAN_CONTRACT_URL="+server.URL, "PLAN_CONTRACT_TOKEN="+f.adminToken, "PLAN_CONTRACT_DIST="+assets)
	if clearWhitespace {
		cmd.Env = append(cmd.Env, "PLAN_CONTRACT_CLEAR_WHITESPACE=1")
	}
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	require.NoError(t, err)
	p, err := f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
	require.NoError(t, err)
	after := contractPlanJSON(t, p)
	for key, value := range before {
		if key == "updated_at" {
			continue
		}
		if !clearWhitespace && key == "description" {
			value = "Browser edited description"
		}
		if clearWhitespace && (key == "product_name" || key == "detail_description" || key == "storefront_badge") {
			// Ent omits these empty strings from its JSON representation.
			value = nil
		}
		assert.Equal(t, value, after[key], "database after browser edit: %s", key)
	}
}
