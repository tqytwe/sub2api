//go:build integration

package requestledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerDetailPageCandidatesUseBoundedIndex(t *testing.T) {
	db := ledgertest.New(t)
	h, err := New(db).Begin(context.Background(), "/v1/images/batches", "POST", "http")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO gateway_request_tasks(request_id,task_kind,task_ref,user_id,api_key_id)
 SELECT $1,'image_batch','fixture_'||lpad(n::text,6,'0'),101,201 FROM generate_series(1,20000) n`, h.ID)
	require.NoError(t, err)
	// Interleave other owners at the same task keys. A one-request fixture can
	// legitimately use the old task-first primary key and cannot test isolation
	// from unrelated source population during seek.
	_, err = db.Exec(`INSERT INTO gateway_requests(id,instance_id,kind,route,method)
 SELECT gen_random_uuid(),instance_id,'http','/v1/images/batches','POST' FROM gateway_requests CROSS JOIN generate_series(1,10) WHERE id=$1`, h.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO gateway_request_tasks(request_id,task_kind,task_ref,user_id,api_key_id)
 SELECT r.id,'image_batch','fixture_'||lpad(n::text,6,'0'),102,202 FROM gateway_requests r CROSS JOIN generate_series(1,20000) n WHERE r.id<>$1`, h.ID)
	require.NoError(t, err)
	_, source, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "migrations", "278_gateway_request_task_page_index_notx.sql"))
	require.NoError(t, err)
	for range 2 {
		_, err = db.Exec(string(raw))
		require.NoError(t, err)
	}
	_, err = db.Exec(`ANALYZE gateway_request_tasks`)
	require.NoError(t, err)
	for _, test := range []struct {
		name, query string
		args        []any
		maxRows     float64
	}{
		{"billing", billingTaskPageCandidates, []any{h.ID, "fixture_010000", 21}, 21},
		{"wallet", walletTaskPageCandidates, []any{h.ID, "image_batch", "fixture_010000", int64(2), 21}, 63},
	} {
		t.Run(test.name, func(t *testing.T) {
			var plan string
			require.NoError(t, db.QueryRow(`EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) `+test.query, test.args...).Scan(&plan))
			require.Contains(t, plan, "gateway_request_tasks_page_idx")
			var parsed []map[string]any
			require.NoError(t, json.Unmarshal([]byte(plan), &parsed))
			var inspect func(map[string]any)
			inspect = func(node map[string]any) {
				if rows, ok := node["Actual Rows"].(float64); ok {
					require.LessOrEqual(t, rows, test.maxRows, plan)
				}
				if removed, ok := node["Rows Removed by Filter"].(float64); ok {
					require.LessOrEqual(t, removed, test.maxRows, plan)
				}
				if plans, ok := node["Plans"].([]any); ok {
					for _, child := range plans {
						inspect(child.(map[string]any))
					}
				}
			}
			inspect(parsed[0]["Plan"].(map[string]any))
			t.Log(plan)
		})
	}
}
