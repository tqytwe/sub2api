//go:build integration

package acceptance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Every request, item, monetary operation and usage row below belongs to a
// disposable PostgreSQL fixture. No provider submission or generation is made.
type batchImageFixture struct {
	router  *gin.Engine
	ledger  *requestledger.Ledger
	db      *sql.DB
	billing service.UsageBillingRepository
	jobs    service.BatchImageRepository
	settler *service.BatchImageSettlementService
	server  *httptest.Server
}

type batchImageSubmission struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
}

func newBatchImageFixture(t *testing.T) *batchImageFixture {
	t.Helper()
	router, ledger, db := ledgerFixtureRouter(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	billing := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
	jobs := repository.NewBatchImageRepository(db)
	settler := &service.BatchImageSettlementService{
		Repo: jobs, BillingRepo: billing, UsageLogRepo: repository.NewUsageLogRepository(client, db),
		Pricing: &service.BatchImageModelPricingResolver{}, // Only the frozen price is usable.
	}
	seedBatchImageIdentity(t, db)
	public := handler.NewBatchImageHandler(&service.BatchImagePublicService{Repo: jobs, BillingRepo: billing}, nil, nil)
	owner := func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if ok {
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: subject.UserID + 100, UserID: subject.UserID})
		}
	}
	router.GET("/v1/images/batches/:id/items", owner, public.Items)
	router.POST("/v1/images/batches/:id/cancel", owner, public.Cancel)
	router.POST("/v1/images/batches", func(c *gin.Context) {
		batchID, err := service.NewBatchImageID()
		require.NoError(t, err)
		require.NoError(t, requestledger.BindTask(c.Request.Context(), "image_batch", batchID, 101, 201))
		requestID := requestledger.FromContext(c.Request.Context()).ID
		key, account, hold := int64(201), int64(234), 10.0
		unitPrice, estimate := 2.0, 6.0
		if c.Query("free-proof") == "1" {
			// Repository proof fixture only. The public reserve helper skips zero
			// holds; this is NOT coverage of a free-price public submission flow.
			hold, unitPrice, estimate = 0, 0, 0
		}
		status, success, output, failed := service.BatchImageJobStatusSettling, 2, 3, 1
		if c.Query("queued") == "1" {
			status, success, output, failed = service.BatchImageJobStatusCreated, 0, 0, 0
		}
		if c.Query("zero") == "1" {
			success, output, failed = 0, 0, 3
		}
		job, err := jobs.CreateBatchImageJob(c.Request.Context(), service.CreateBatchImageJobParams{
			BatchID: batchID, UserID: 101, APIKeyID: &key, AccountID: &account,
			Provider: service.BatchImageProviderGeminiAPI, Model: "synthetic-batch-image",
			Status: status, ItemCount: 3, SuccessCount: success,
			OutputImageCount: output, FailCount: failed, EstimatedCost: estimate, HoldAmount: &hold,
			BillableUnitPrice: unitPrice, PricingSnapshotVersion: 1,
			GroupRateMultiplier: 1, AccountRateMultiplier: 1, BatchDiscountMultiplier: 1,
		})
		require.NoError(t, err)
		if output > 0 {
			require.NoError(t, jobs.BulkCreateBatchImageItems(c.Request.Context(), []service.CreateBatchImageItemParams{
				{JobID: batchID, CustomID: "one", Status: service.BatchImageItemStatusSuccess, ImageCount: 2},
				{JobID: batchID, CustomID: "two", Status: service.BatchImageItemStatusSuccess, ImageCount: 1},
				{JobID: batchID, CustomID: "failed", Status: "failed", ImageCount: 0},
			}))
		}
		_, err = billing.ReserveBatchImageBalance(c.Request.Context(), &service.BatchImageBalanceHoldCommand{
			RequestID: service.BatchImageHoldRequestID(batchID), HoldRequestID: service.BatchImageHoldRequestID(batchID),
			UserID: job.UserID, APIKeyID: key, BatchID: batchID, HoldAmount: hold,
		})
		require.NoError(t, err)
		c.JSON(http.StatusAccepted, batchImageSubmission{ID: batchID, RequestID: requestID})
	})
	f := &batchImageFixture{router: router, ledger: ledger, db: db, billing: billing, jobs: jobs, settler: settler}
	f.server = httptest.NewServer(router)
	t.Cleanup(f.server.Close)
	return f
}

func (f *batchImageFixture) submit(t *testing.T, query string) batchImageSubmission {
	t.Helper()
	response, err := f.server.Client().Post(f.server.URL+"/v1/images/batches"+query, "application/json", nil)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusAccepted, response.StatusCode)
	var submission batchImageSubmission
	require.NoError(t, json.NewDecoder(response.Body).Decode(&submission))
	return submission
}

func TestBatchImageHTTPUsageDrillUsesCommittedCapture(t *testing.T) {
	f := newBatchImageFixture(t)
	submission := f.submit(t, "")
	batchID, requestID := submission.ID, submission.RequestID
	settler, ledger, db, server := f.settler, f.ledger, f.db, f.server
	result, err := settler.Settle(context.Background(), batchID)
	require.NoError(t, err)
	require.Equal(t, 6.0, result.ActualCost)
	require.NoError(t, ledger.ReconcileTaskBilling(context.Background()))
	var usageID int64
	var fingerprint string
	var settled bool
	var billed, balance, frozen float64
	require.NoError(t, db.QueryRow(`SELECT id,COALESCE(billing_request_fingerprint,''),billing_settled,billed_cost FROM usage_logs WHERE request_id=$1`, result.RequestID).Scan(&usageID, &fingerprint, &settled, &billed))
	require.NoError(t, db.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=101`).Scan(&balance, &frozen))
	require.Equal(t, 94.0, balance)
	require.Zero(t, frozen)
	status, body := batchImageHTTPRead(t, server, "101", "/api/v1/requests/"+requestID)
	require.Equal(t, 200, status, body)
	var detail struct {
		Data struct {
			Billing []requestledger.BillingReference `json:"billing"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &detail))
	require.Len(t, detail.Data.Billing, 1, "capture committed and usage exists, but no exact drill reference: %s", body)
	require.NotEmpty(t, fingerprint)
	require.True(t, settled)
	require.Equal(t, 6.0, billed)
	require.Equal(t, &usageID, detail.Data.Billing[0].UsageLogID)
	require.Contains(t, body, `"settlement_state":"settled"`)
	require.Contains(t, body, `"image_balance_capture"`)
	status, body = batchImageHTTPRead(t, server, "101", fmt.Sprintf("/api/v1/requests/%s/usage/%d", requestID, usageID))
	require.Equal(t, 200, status, body)
	require.Contains(t, body, `"billed_cost":6`)
	require.Contains(t, body, `"image_count":3`)
	require.Contains(t, body, `"batch_id":"`+batchID+`"`)
	status, body = batchImageHTTPRead(t, server, "101", "/v1/images/batches/"+batchID+"/items")
	require.Equal(t, 200, status, body)
	var items service.BatchImagePublicItemsResponse
	require.NoError(t, json.Unmarshal([]byte(body), &items))
	require.Len(t, items.Data, 3)
	total := 0
	for _, item := range items.Data {
		total += item.ImageCount
	}
	require.Equal(t, 3, total)
}

func seedBatchImageIdentity(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users(id,email,password_hash,balance) VALUES(101,'batch-fixture@example.invalid','disabled-fixture',100);
 INSERT INTO api_keys(id,user_id,key,name) VALUES(201,101,'not-a-credential-batch-fixture','disabled fixture');
 INSERT INTO accounts(id,name,platform,type) VALUES(234,'disabled fixture','gemini','apikey')`)
	require.NoError(t, err)
}

func batchImageHTTPRead(t *testing.T, server *httptest.Server, user, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", server.URL+path, nil)
	require.NoError(t, err)
	req.Header.Set("X-Ledger-Test-User", user)
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(body)
}
