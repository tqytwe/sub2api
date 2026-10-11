//go:build integration

package acceptance

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Pause only after the real transaction commits; then disconnect the actual HTTP
// client. The detached usage writer must retain proof even if job finalization
// needs a later worker retry. No provider is configured or contacted.
func TestBatchImageHTTPDisconnectAfterCapture(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	before := f.balance(t)
	gate := &batchCaptureDisconnectGate{UsageBillingRepository: f.billing, captured: make(chan struct{})}
	settler := *f.settler
	settler.BillingRepo = gate
	finished := make(chan error, 1)
	f.router.POST("/v1/batch-fixture/settle", func(c *gin.Context) {
		require.NoError(t, requestledger.BindTask(c.Request.Context(), "image_batch", s.ID, 101, 201))
		_, err := settler.Settle(c.Request.Context(), s.ID)
		finished <- err
		c.Status(200)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", f.server.URL+"/v1/batch-fixture/settle", nil)
	require.NoError(t, err)
	clientDone := make(chan error, 1)
	go func() {
		response, err := f.server.Client().Do(req)
		if response != nil {
			_ = response.Body.Close()
		}
		clientDone <- err
	}()
	select {
	case <-gate.captured:
	case <-time.After(10 * time.Second):
		t.Fatal("capture did not commit")
	}
	cancel()
	select {
	case err := <-clientDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("HTTP client did not disconnect")
	}
	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("settlement did not finish after disconnect")
	}
	f.requireCapture(t, s.ID, before+4)
	ref := f.reference(t, s.RequestID)
	require.NotNil(t, ref.UsageLogID)
	_, err = f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	f.requireCapture(t, s.ID, before+4)
	require.Equal(t, ref, f.reference(t, s.RequestID))
}

type batchCaptureDisconnectGate struct {
	service.UsageBillingRepository
	captured chan struct{}
}

func (g *batchCaptureDisconnectGate) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	result, err := g.UsageBillingRepository.CaptureBatchImageBalance(ctx, cmd)
	if err != nil {
		return result, err
	}
	close(g.captured)
	<-ctx.Done()
	return result, nil
}

func TestBatchImageHTTPUnknownUsageAndNullableImageCount(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "?queued=1")
	ctx := context.Background()
	h, err := f.ledger.BeginTask(ctx, "image_batch", s.ID, 101, 201, true)
	require.NoError(t, err)
	hc := requestledger.WithHandle(ctx, h)
	attempt, err := requestledger.BeginAttempt(hc, 234, 234)
	require.NoError(t, err)
	require.NoError(t, attempt.Finish(hc, 502, fmt.Errorf("synthetic missing usage")))
	require.NoError(t, h.Finish(ctx, "failed", 502, "upstream_error"))
	public := &service.BatchImagePublicService{Repo: f.jobs, BillingRepo: f.billing}
	_, err = public.Cancel(ctx, service.BatchImageOwner{UserID: 101, APIKeyID: 201}, s.ID)
	require.NoError(t, err)
	require.NoError(t, f.ledger.ReconcileTaskBilling(ctx))
	status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+h.ID)
	require.Equal(t, 200, status, body)
	require.Contains(t, body, `"usage_state":"usage_unknown"`)
	require.Contains(t, body, `"settlement_state":"settled"`)
	require.Contains(t, body, `"billing":[]`)
	require.NotContains(t, body, `"billed_cost":0`)
	// Existing token-mode usage can legally have NULL image_count (migration029).
	id, usageID := seedBilledRequest(t, f.router, f.db, 101)
	_, err = f.db.Exec(`UPDATE usage_logs SET image_count=NULL WHERE id=$1`, usageID)
	require.NoError(t, err)
	status, body = batchImageHTTPRead(t, f.server, "101", fmt.Sprintf("/api/v1/requests/%s/usage/%d", id, usageID))
	require.Equal(t, 200, status, body)
	require.Contains(t, body, `"image_count":null`)
	require.Contains(t, body, `"input_tokens":7`)
	require.Contains(t, body, `"billed_cost":1.25`)
}

// Same-route authentication failures have no task and therefore no settlement
// evidence. Rotate at most 1000 candidates per call without changing their state.
func TestBatchImageHTTPReconciliationBoundsRequestsWithoutTask(t *testing.T) {
	f := newBatchImageFixture(t)
	ctx := context.Background()
	h, err := f.ledger.Begin(ctx, "/v1/models", "GET", "http")
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_requests(id,instance_id,kind,route,method,execution_state,usage_state,http_status,ended_at)
 SELECT gen_random_uuid(),instance_id,'http','/v1/images/batches','POST','failed','not_applicable',401,clock_timestamp() FROM gateway_requests CROSS JOIN generate_series(1,2000) WHERE id=$1`, h.ID)
	require.NoError(t, err)
	for _, expected := range []int{1000, 2000} {
		require.NoError(t, f.ledger.ReconcileTaskBilling(ctx))
		var checked, unchanged int
		require.NoError(t, f.db.QueryRow(`SELECT count(*) FILTER (WHERE reconcile_checked_at IS NOT NULL),count(*) FILTER (WHERE settlement_state='not_required' AND execution_state='failed' AND usage_state='not_applicable' AND user_id IS NULL AND api_key_id IS NULL) FROM gateway_requests WHERE route='/v1/images/batches'`).Scan(&checked, &unchanged))
		require.Equal(t, expected, checked)
		require.Equal(t, 2000, unchanged)
	}
}
