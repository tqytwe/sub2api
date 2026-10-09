//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This opt-in package must run through the fresh external test-service wrapper.
// It has no Docker fallback, production configuration fallback, or silent skip.
func openIsolatedBillingDB(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	if os.Getenv("SUB2API_TEST_EXTERNAL_MODE") != "local" {
		t.Fatal("explicit isolated local test mode is required")
	}
	raw := os.Getenv("SUB2API_TEST_POSTGRES_DSN")
	endpoint, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid isolated test database configuration")
	}
	if endpoint.Scheme != "postgres" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() != "55432" || endpoint.Path != "/sub2api_test_migration" || endpoint.User == nil || endpoint.User.Username() != "sub2api_test" || endpoint.RawQuery != "sslmode=disable" || endpoint.Fragment != "" {
		t.Fatal("refusing a database outside the dedicated loopback test service")
	}
	if _, hasPassword := endpoint.User.Password(); hasPassword {
		t.Fatal("isolated wrapper must not supply account credentials")
	}
	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal("cannot open isolated test database")
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(20)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx))
	var database string
	var tableCount int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.Equal(t, "sub2api_test_migration", database)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&tableCount))
	require.Zero(t, tableCount, "acceptance requires a fresh database before migrations")
	require.NoError(t, timezone.Init("UTC"))
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return db, client
}

func TestGatewayIdentityActualPostgresSettlement(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct {
		name            string
		requests        int
		concurrent      bool
		gatewayIdentity bool
	}{
		{"legacy_reused_header_control", 2, false, false},
		{"independent_requests", 2, false, true},
		{"concurrent_independent_requests", 8, true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			u, err := client.User.Create().SetEmail("billing-acceptance-" + uuid.NewString() + "@example.com").SetPasswordHash("test-only-hash").SetRole(service.RoleUser).SetStatus(service.StatusActive).SetBalance(100).SetConcurrency(16).Save(ctx)
			require.NoError(t, err)
			k, err := client.APIKey.Create().SetUserID(u.ID).SetKey("test-only-" + uuid.NewString()).SetName("acceptance").SetStatus(service.StatusActive).Save(ctx)
			require.NoError(t, err)
			a, err := client.Account.Create().SetName("acceptance-" + uuid.NewString()).SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).SetCredentials(map[string]any{}).SetExtra(map[string]any{}).SetConcurrency(16).SetPriority(50).SetStatus(service.StatusActive).SetSchedulable(true).Save(ctx)
			require.NoError(t, err)
			user := &service.User{ID: u.ID, Balance: 100, Status: service.StatusActive}
			key := &service.APIKey{ID: k.ID, UserID: u.ID}
			account := &service.Account{ID: a.ID, Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI}
			cfg := &config.Config{}
			cfg.Default.RateMultiplier = 1
			pricing := service.NewBillingService(cfg, nil)
			cost, err := pricing.CalculateCost("gpt-5.1", service.UsageTokens{InputTokens: 8, OutputTokens: 4}, 1)
			require.NoError(t, err)
			require.Greater(t, cost.ActualCost, 0.0)
			logs := repository.NewUsageLogRepository(client, db)
			ledger := service.NewBalanceLedgerService(db, nil, nil)
			billing := repository.NewUsageBillingRepositoryWithLedger(client, db, ledger)
			gateway := service.NewOpenAIGatewayService(nil, logs, billing, nil, nil, nil, nil, cfg, nil, nil, pricing, nil, &service.BillingCacheService{}, nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(middleware.RequestLogger())
			if scenario.gatewayIdentity {
				router.Use(middleware.GatewayClientRequestID())
			} else {
				router.Use(middleware.ClientRequestID())
			}
			var upstreamID atomic.Int64
			router.POST("/v1/responses", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.String(http.StatusInternalServerError, "body read failed")
					return
				}
				// Distinct accepted upstream results incur distinct expense; callback replay
				// retains the same result and request context without another forward call.
				input := &service.OpenAIRecordUsageInput{
					Result: &service.OpenAIForwardResult{RequestID: fmt.Sprintf("accepted-upstream-%d", upstreamID.Add(1)), Model: "gpt-5.1", Usage: service.OpenAIUsage{InputTokens: 8, OutputTokens: 4}},
					APIKey: key, User: user, Account: account, RequestPayloadHash: service.HashUsageRequestPayload(body),
				}
				for range 2 {
					if err := gateway.RecordUsage(context.WithoutCancel(c.Request.Context()), input); err != nil {
						c.String(http.StatusInternalServerError, "usage settlement failed")
						return
					}
				}
				c.Status(http.StatusOK)
			})
			const clientHeader = "e48b837c-e08b-4521-b956-c495415cfb34"
			const requestHeader = "fbc26f36-ddb8-478a-b28e-f19cc5c97702"
			type response struct {
				status              int
				clientID, requestID string
			}
			responses := make(chan response, scenario.requests)
			send := func() {
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.1","input":"same accepted request body"}`))
				req.Header.Set("X-Client-Request-ID", clientHeader)
				req.Header.Set("X-Request-ID", requestHeader)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				responses <- response{recorder.Code, recorder.Header().Get("X-Client-Request-ID"), recorder.Header().Get("X-Request-ID")}
			}
			var workers sync.WaitGroup
			for range scenario.requests {
				if scenario.concurrent {
					workers.Add(1)
					go func() { defer workers.Done(); send() }()
				} else {
					send()
				}
			}
			workers.Wait()
			close(responses)
			for response := range responses {
				require.Equal(t, http.StatusOK, response.status)
				require.Equal(t, clientHeader, response.clientID)
				require.Equal(t, requestHeader, response.requestID)
			}
			require.EqualValues(t, scenario.requests, upstreamID.Load())
			expected := scenario.requests
			if !scenario.gatewayIdentity {
				expected = 1
			}
			rows, err := db.QueryContext(ctx, "SELECT request_id, actual_cost FROM usage_logs WHERE api_key_id=$1 ORDER BY id", key.ID)
			require.NoError(t, err)
			defer rows.Close()
			ids := make([]string, 0, expected)
			for rows.Next() {
				var id string
				var charged float64
				require.NoError(t, rows.Scan(&id, &charged))
				require.InDelta(t, cost.ActualCost, charged, 1e-9)
				if scenario.gatewayIdentity {
					require.True(t, strings.HasPrefix(id, "gateway:"))
				}
				ids = append(ids, id)
			}
			require.NoError(t, rows.Err())
			require.NoError(t, rows.Close())
			require.Len(t, ids, expected)
			distinct := make(map[string]bool)
			for _, id := range ids {
				require.False(t, distinct[id])
				distinct[id] = true
				var count int
				require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1 AND source_type='usage_charge' AND source_id=$2", user.ID, id).Scan(&count))
				require.Equal(t, 1, count, "each result must have exactly one ledger effect despite callback replay")
			}
			var ledgerCount int
			var balance float64
			require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1 AND source_type='usage_charge'", user.ID).Scan(&ledgerCount))
			require.Equal(t, expected, ledgerCount)
			require.NoError(t, db.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", user.ID).Scan(&balance))
			require.InDelta(t, 100-float64(expected)*cost.ActualCost, balance, 1e-8)
			statsService := service.NewAccountUsageService(nil, logs, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			stats, err := statsService.GetTodayStats(ctx, account.ID)
			require.NoError(t, err)
			require.EqualValues(t, expected, stats.Requests)
			require.EqualValues(t, expected*12, stats.Tokens)
			require.InDelta(t, float64(expected)*cost.TotalCost, stats.Cost, 1e-8)
			require.InDelta(t, float64(expected)*cost.ActualCost, stats.UserCost, 1e-8)
			t.Logf("accepted=%d callbacks=%d usage_rows=%d ledger_effects=%d today_requests=%d today_tokens=%d", scenario.requests, scenario.requests*2, len(ids), ledgerCount, stats.Requests, stats.Tokens)
		})
	}
}
