//go:build integration

package requestledger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresHealthyOwnerRecoversFailedFinalization(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	requestCtx := WithHandle(ctx, h)
	require.NoError(t, BindIdentity(requestCtx, 101, 201))
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"output":"synthetic"}`))
	}))
	defer upstream.Close()
	req, err := http.NewRequestWithContext(requestCtx, "POST", upstream.URL, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	_, err = db.Exec(`ALTER TABLE gateway_requests ADD CONSTRAINT synthetic_finish_fault CHECK (execution_state='inflight') NOT VALID`)
	require.NoError(t, err)
	require.ErrorIs(t, h.Finish(requestCtx, "succeeded", 200, ""), ErrUnavailable)
	_, err = db.Exec(`ALTER TABLE gateway_requests DROP CONSTRAINT synthetic_finish_fault`)
	require.NoError(t, err)
	active, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	other, err := New(db).Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	var live bool
	require.NoError(t, db.QueryRow(`SELECT lease_until>clock_timestamp() FROM gateway_ledger_instances WHERE id=$1`, h.owner).Scan(&live))
	require.True(t, live)
	for range 2 {
		require.NoError(t, l.Recover(ctx))
	}
	var execution, usage, settlement string
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,settlement_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&execution, &usage, &settlement))
	require.Equal(t, "interrupted", execution)
	require.Equal(t, "usage_unknown", usage)
	require.Equal(t, "settlement_pending", settlement)
	for _, id := range []string{active.ID, other.ID} {
		require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, id).Scan(&execution))
		require.Equal(t, "inflight", execution)
	}
	require.EqualValues(t, 1, calls.Load(), "recovery must never repeat upstream work")
	var links int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links`).Scan(&links))
	require.Zero(t, links)
}

func TestLedgerPostgresFalseUpgradeDoesNotChangeHTTPMetering(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "http", true: "sse"}[stream], func(t *testing.T) {
			var id string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"synthetic\"}\n\n"))
				} else {
					w.Header().Set("Content-Length", "1000")
					_, _ = w.Write([]byte(`{"partial":true}`))
				}
			}))
			defer upstream.Close()
			router := gin.New()
			router.Use(Middleware(l))
			router.POST("/v1/responses", func(c *gin.Context) {
				id = FromContext(c.Request.Context()).ID
				require.NoError(t, BindIdentity(c.Request.Context(), 101, 201))
				req, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstream.URL, nil)
				require.NoError(t, err)
				resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
				require.NoError(t, err)
				_, _ = io.Copy(c.Writer, resp.Body)
				_ = resp.Body.Close()
			})
			server := httptest.NewServer(router)
			defer server.Close()
			req, err := http.NewRequest("POST", server.URL+"/v1/responses", strings.NewReader(`{}`))
			require.NoError(t, err)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			resp, err := server.Client().Do(req)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			var kind, usage, settlement string
			require.NoError(t, db.QueryRow(`SELECT kind,usage_state,settlement_state FROM gateway_requests WHERE id=$1`, id).Scan(&kind, &usage, &settlement))
			require.Equal(t, "http", kind)
			require.Equal(t, "usage_unknown", usage)
			require.Equal(t, "settlement_pending", settlement)
		})
	}
}

func TestLedgerPostgresFrozenUsageDoesNotMoveToLaterAttempt(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	h, err := l.Begin(context.Background(), "/v1/images/generations", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	first, err := BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	frozen := FreezeContext(ctx, context.Background())
	second, err := BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	require.NoError(t, ObserveUsage(frozen, 234))
	var firstState, secondState string
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=$2`, h.ID, first.Number).Scan(&firstState))
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=$2`, h.ID, second.Number).Scan(&secondState))
	require.Equal(t, "known", firstState)
	require.Equal(t, "pending", secondState)
}

// This is a bounded local correctness/load probe, not a production throughput claim.
func TestLedgerPostgresBoundedConcurrentAdmissionAndRecovery(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	const workers = 12
	const perWorker = 4
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, workers+1)
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := l.Recover(ctx); err != nil {
				errs <- err
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	var producers sync.WaitGroup
	start := time.Now()
	for range workers {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for range perWorker {
				h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
				if err != nil {
					errs <- err
					return
				}
				hc := WithHandle(ctx, h)
				if err = BindIdentity(hc, 101, 201); err != nil {
					errs <- err
					return
				}
				a, err := BeginAttempt(hc, 234, 22)
				if err != nil {
					errs <- err
					return
				}
				if err = a.ObserveUsage(hc); err != nil {
					errs <- err
					return
				}
				if err = a.Finish(hc, 200, nil); err != nil {
					errs <- err
					return
				}
				if err = h.Finish(hc, "succeeded", 200, ""); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	producers.Wait()
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count, success int
	require.NoError(t, db.QueryRow(`SELECT count(*),count(*) FILTER(WHERE execution_state='succeeded' AND usage_state='known') FROM gateway_requests`).Scan(&count, &success))
	require.Equal(t, workers*perWorker, count)
	require.Equal(t, count, success)
	t.Logf("local only: %d requests, %d concurrent workers, elapsed=%s", count, workers, time.Since(start))
}

func TestLedgerPostgresBodylessResponsesKeepProtocolSuccess(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	for _, tc := range []struct {
		method string
		status int
	}{{"GET", 204}, {"GET", 304}, {"HEAD", 200}} {
		t.Run(fmt.Sprintf("%s_%d", tc.method, tc.status), func(t *testing.T) {
			h, err := ledger.Begin(context.Background(), "/v1/models", tc.method, "http")
			require.NoError(t, err)
			ctx := WithHandle(context.Background(), h)
			require.NoError(t, BindIdentity(ctx, 101, 201))
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) }))
			defer upstream.Close()
			req, err := http.NewRequestWithContext(ctx, tc.method, upstream.URL, nil)
			require.NoError(t, err)
			resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close()) // Real models 304 branch deliberately does not read a body.
			require.NoError(t, h.Finish(ctx, "succeeded", tc.status, ""))
			var state, usage string
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state, &usage))
			require.Equal(t, "succeeded", state)
			require.Equal(t, "not_applicable", usage)
		})
	}
}
