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

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Built frontend + real HTTP/PG only. Synthetic identity and existing settlement
// repositories; never provider generation or production credentials/data.
func TestRequestLedgerDetailPagingBrowserFixture(t *testing.T) {
	if os.Getenv("LEDGER_DETAIL_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in browser fixture")
	}
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	_, err := f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_billing_links(request_id,billing_request_id,api_key_id,request_fingerprint,subscription_id)
 SELECT $1,'fixture_link_'||lpad(n::text,4,'0'),201,'fixture_unverified',n FROM generate_series(1,45) n`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_tasks(request_id,task_kind,task_ref,user_id,api_key_id)
 SELECT $1,'image_batch','000_empty_'||lpad(n::text,4,'0'),101,201 FROM generate_series(1,41) n`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id,execution_state,ended_at,http_status,usage_state,error_code)
 SELECT $1,n,234,22,'failed',clock_timestamp(),200,'usage_unknown','upstream_error' FROM generate_series(1,25) n`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_attempt_evidence(request_id,attempt_no,attempt_id,upstream_request_id,upstream_response_id)
 SELECT request_id,attempt_no,gen_random_uuid(),'fixture_req_'||attempt_no,'fixture_resp_'||attempt_no FROM gateway_request_attempts WHERE request_id=$1`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE gateway_requests SET attempt_count=25,usage_state='usage_unknown',settlement_state='settlement_pending' WHERE id=$1`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_ops_links(request_id,attempt_no,ops_error_log_id) SELECT $1,n,n FROM generate_series(1,5) n`, s.RequestID)
	require.NoError(t, err)
	other, err := f.ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	require.NoError(t, requestledger.BindIdentity(requestledger.WithHandle(context.Background(), other), 101, 201))
	var usageID int64
	require.NoError(t, f.db.QueryRow(`SELECT id FROM usage_logs WHERE request_id='batch_image_capture:'||$1 AND api_key_id=201`, s.ID).Scan(&usageID))
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
	fmt.Printf("DETAIL_BROWSER_READY url=%s request=%s batch=%s usage=%d other=%s\n", f.server.URL, s.RequestID, s.ID, usageID, other.ID)
	stop := time.NewTimer(20 * time.Minute)
	defer stop.Stop()
	for {
		select {
		case <-stop.C:
			return
		case <-time.After(time.Second):
			if _, err := os.Stat("/tmp/ledger-detail-browser-stop"); err == nil {
				return
			}
		}
	}
}
