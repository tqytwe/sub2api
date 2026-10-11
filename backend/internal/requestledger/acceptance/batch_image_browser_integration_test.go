//go:build integration

package acceptance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBatchImageBrowserFixture(t *testing.T) {
	if os.Getenv("LEDGER_BATCH_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in browser fixture")
	}
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	_, err := f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	ref := f.reference(t, s.RequestID)
	f.router.GET("/api/v1/auth/me", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("X-Ledger-Test-User"))
		role := "user"
		if id == 999 {
			role = "admin"
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"id": id, "username": "Synthetic reviewer", "email": "fixture@example.invalid", "role": role, "balance": 94, "status": "active"}})
	})
	f.router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"site_name": "极速蹬", "custom_menu_items": []gin.H{}}})
	})
	dist := os.Getenv("LEDGER_BROWSER_DIST")
	require.True(t, strings.HasPrefix(dist, "/tmp/ledger-"))
	f.router.NoRoute(func(c *gin.Context) {
		p := filepath.Join(dist, filepath.Clean("/"+c.Request.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			c.File(p)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(200, gin.H{"code": 0, "data": gin.H{"items": []gin.H{}, "total": 0}})
			return
		}
		c.File(filepath.Join(dist, "index.html"))
	})
	fmt.Printf("BATCH_BROWSER_READY url=%s request=%s batch=%s usage=%d\n", f.server.URL, s.RequestID, s.ID, *ref.UsageLogID)
	stop := time.NewTimer(20 * time.Minute)
	defer stop.Stop()
	for {
		select {
		case <-stop.C:
			return
		case <-time.After(time.Second):
			if _, err := os.Stat("/tmp/ledger-batch-browser-stop"); err == nil {
				return
			}
		}
	}
}
