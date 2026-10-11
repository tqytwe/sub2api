//go:build integration

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type routingEmptyAccountRepo struct{ service.AccountRepository }

func (routingEmptyAccountRepo) ListSchedulableByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}
func (routingEmptyAccountRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}
func (routingEmptyAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	return nil, nil
}

func TestRequestLedgerRoutingRealTextHandlersHTTP(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	gateway := service.NewOpenAIGatewayService(routingEmptyAccountRepo{}, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	defer billing.Stop()
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	for _, tc := range []struct {
		path, body string
		handler    gin.HandlerFunc
	}{
		{"/v1/responses", `{"model":"gpt-5.1","input":"sentinel-prompt","stream":true}`, h.Responses},
		{"/v1/messages", `{"model":"gpt-5.1","max_tokens":10,"messages":[{"role":"user","content":"sentinel-prompt"}],"stream":true}`, h.Messages},
		{"/v1/chat/completions", `{"model":"gpt-5.1","messages":[{"role":"user","content":"sentinel-prompt"}],"stream":true}`, h.ChatCompletions},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var baseline string
			for _, observed := range []bool{false, true} {
				var id string
				r := gin.New()
				if observed {
					r.Use(requestledger.Middleware(l))
				}
				r.Use(func(c *gin.Context) {
					groupID := int64(3131)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 99, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowMessagesDispatch: true}, User: &service.User{ID: 100}})
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 100, Concurrency: 0})
					if observed {
						id = requestledger.FromContext(c.Request.Context()).ID
						require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 100, 99))
					}
				})
				r.POST(tc.path, tc.handler)
				server := httptest.NewServer(r)
				res, err := http.Post(server.URL+tc.path, "application/json", strings.NewReader(tc.body))
				require.NoError(t, err)
				data, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.NoError(t, res.Body.Close())
				server.Close()
				require.Equal(t, 503, res.StatusCode, string(data))
				if !observed {
					baseline = string(data)
					continue
				}
				require.Equal(t, baseline, string(data))
				e := awaitHandlerRouting(t, l, id)
				require.GreaterOrEqual(t, e.SelectionCount, 1)
				require.False(t, e.SourcePopulationComplete)
				var attempts int
				require.NoError(t, db.QueryRow(`SELECT attempt_count FROM gateway_requests WHERE id=$1`, id).Scan(&attempts))
				require.Zero(t, attempts)
			}
		})
	}
}

type routingQueueFullCache struct {
	service.ConcurrencyCache
	acquires, releases, waits int
}

func (c *routingQueueFullCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	c.acquires++
	return false, nil
}
func (c *routingQueueFullCache) ReleaseAccountSlot(context.Context, int64, string) error {
	c.releases++
	return nil
}
func (c *routingQueueFullCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	c.waits++
	return false, nil
}

func TestRequestLedgerRoutingTerminalProfitAndQueue(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, scenario := range []string{"profit", "queue_full"} {
		t.Run(scenario, func(t *testing.T) {
			var baselineCode int
			var baselineBody string
			var baselineAcquires, baselineReleases, baselineWaits int
			for _, observed := range []bool{false, true} {
				cache := &routingQueueFullCache{}
				gateway := &service.OpenAIGatewayService{}
				h := &OpenAIGatewayHandler{gatewayService: gateway, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatClaude, 0)}
				var id string
				r := gin.New()
				if observed {
					r.Use(requestledger.Middleware(l))
				}
				r.POST("/v1/responses", func(c *gin.Context) {
					ctx := c.Request.Context()
					if observed {
						id = requestledger.FromContext(ctx).ID
						var end func(int64, bool, bool)
						ctx, end = requestledger.BeginRoutingSelection(ctx, true)
						end(77, scenario != "profit", false)
					}
					rate := 0.8
					account := &service.Account{ID: 77, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Concurrency: 2, RateMultiplier: &rate}
					groupID := int64(51)
					if scenario == "profit" {
						group := &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1, SubscriptionType: service.SubscriptionTypeStandard, ProfitControlEnabled: true, ProfitMinMargin: 0.5}
						ctx = context.WithValue(ctx, ctxkey.Group, group)
						ctx, _ = gateway.WithOpenAIRequestPricingContext(ctx, &groupID)
					}
					c.Request = c.Request.WithContext(ctx)
					selection := &service.AccountSelectionResult{Account: account, Acquired: scenario == "profit", ReleaseFunc: func() { cache.releases++ }, WaitPlan: &service.AccountWaitPlan{AccountID: 77, MaxConcurrency: 2, Timeout: time.Second, MaxWaiting: 1}}
					started := false
					release, result := h.acquireOpenAIAccountSlot(c, &groupID, "", selection, false, &started, zap.NewNop(), nil)
					require.Nil(t, release)
					if scenario == "profit" {
						require.Equal(t, openAISlotAcquireProfitVetoed, result)
						c.Status(503)
					} else {
						require.Equal(t, openAISlotAcquireFailed, result)
					}
				})
				server := httptest.NewServer(r)
				res, err := http.Post(server.URL+"/v1/responses", "application/json", strings.NewReader("{}"))
				require.NoError(t, err)
				data, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.NoError(t, res.Body.Close())
				server.Close()
				if !observed {
					baselineCode = res.StatusCode
					baselineBody = string(data)
					baselineAcquires = cache.acquires
					baselineReleases = cache.releases
					baselineWaits = cache.waits
					continue
				}
				require.Equal(t, baselineCode, res.StatusCode)
				require.Equal(t, baselineBody, string(data))
				require.Equal(t, baselineAcquires, cache.acquires)
				require.Equal(t, baselineReleases, cache.releases)
				require.Equal(t, baselineWaits, cache.waits)
				e := awaitHandlerRouting(t, l, id)
				wanted := "wait_queue_full"
				if scenario == "profit" {
					wanted = "profit_threshold"
				}
				found := false
				for _, event := range e.Events {
					if event.Reason == wanted && event.Stage == "admission" {
						found = true
					}
				}
				require.True(t, found)
			}
		})
	}
}
func awaitHandlerRouting(t *testing.T, l *requestledger.Ledger, id string) *requestledger.RoutingEvidence {
	t.Helper()
	var evidence *requestledger.RoutingEvidence
	require.Eventually(t, func() bool {
		var err error
		evidence, err = l.RoutingEvidence(context.Background(), requestledger.Viewer{Admin: true, UserID: 1}, id)
		return err == nil && evidence.Status == "complete"
	}, 2*time.Second, 10*time.Millisecond)
	return evidence
}
