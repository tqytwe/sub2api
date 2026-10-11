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

func TestBatchImageReconciliationUsesBoundedIndex(t *testing.T) {
	db := ledgertest.New(t)
	l := New(db)
	h, err := l.Begin(context.Background(), "/v1/images/batches", "POST", "http")
	require.NoError(t, err)
	require.NoError(t, BindTask(WithHandle(context.Background(), h), "image_batch", "imgbatch_index_fixture", 101, 201))
	// A large unrelated control population must not turn the periodic lookup
	// into a scan over all not_required requests.
	_, err = db.Exec(`INSERT INTO gateway_requests(id,instance_id,kind,route,method)
 SELECT gen_random_uuid(),instance_id,'http','/v1/models','GET' FROM gateway_requests CROSS JOIN generate_series(1,20000) WHERE id=$1`, h.ID)
	require.NoError(t, err)
	// Rejected requests on the same route have no task. They must still be
	// bounded before proof checks, rather than rescanned forever before LIMIT.
	_, err = db.Exec(`INSERT INTO gateway_requests(id,instance_id,kind,route,method,execution_state,usage_state,http_status,ended_at)
 SELECT gen_random_uuid(),instance_id,'http','/v1/images/batches','POST','failed','not_applicable',401,clock_timestamp() FROM gateway_requests CROSS JOIN generate_series(1,20000) WHERE id=$1`, h.ID)
	require.NoError(t, err)
	_, source, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "migrations", "277_batch_image_ledger_reconcile_index_notx.sql"))
	require.NoError(t, err)
	for range 2 {
		_, err = db.Exec(string(content))
		require.NoError(t, err)
	}
	_, err = db.Exec(`ANALYZE gateway_requests; ANALYZE gateway_request_tasks`)
	require.NoError(t, err)
	var plan string
	require.NoError(t, db.QueryRow(`EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) `+taskBillingCandidates).Scan(&plan))
	require.Contains(t, plan, `gateway_requests_batch_reconcile_idx`, plan)
	t.Log(plan)
	var explanation []struct {
		Plan struct {
			ActualRows float64 `json:"Actual Rows"`
		} `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal([]byte(plan), &explanation))
	require.Len(t, explanation, 1)
	require.Equal(t, float64(1000), explanation[0].Plan.ActualRows, "LIMIT must bound candidates before task verification")
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT indisvalid FROM pg_index WHERE indexrelid='gateway_requests_batch_reconcile_idx'::regclass`).Scan(&valid))
	require.True(t, valid)
}
