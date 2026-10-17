package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGatewayBillingIdentitySeparatesRepeatedClientHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestLogger(), GatewayClientRequestID())
	var identities []string
	router.POST("/v1/responses", func(c *gin.Context) {
		id, _ := c.Request.Context().Value(ctxkey.UsageBillingRequestID).(string)
		identities = append(identities, id)
		require.Equal(t, "reused-correlation", c.Request.Context().Value(ctxkey.ClientRequestID))
		require.Equal(t, "reused-request", c.Request.Context().Value(ctxkey.RequestID))
		c.Status(http.StatusOK)
	})
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		req.Header.Set("X-Client-Request-ID", "reused-correlation")
		req.Header.Set("X-Request-ID", "reused-request")
		req.Header.Set("X-Usage-Billing-Request-ID", "00000000-0000-4000-8000-000000000123")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "reused-correlation", response.Header().Get("X-Client-Request-ID"))
		require.Empty(t, response.Header().Get("X-Usage-Billing-Request-ID"))
	}
	require.Len(t, identities, 2)
	for _, id := range identities {
		_, err := uuid.Parse(id)
		require.NoError(t, err)
		require.NotEqual(t, "00000000-0000-4000-8000-000000000123", id)
	}
	require.NotEqual(t, identities[0], identities[1])
}

func TestGatewayBillingIdentityIgnoresPreexistingCorrelationAndMintsOnlyOnce(t *testing.T) {
	router := gin.New()
	var before string
	router.Use(GatewayClientRequestID(), func(c *gin.Context) {
		before, _ = c.Request.Context().Value(ctxkey.UsageBillingRequestID).(string)
		c.Next()
	}, GatewayClientRequestID())
	router.GET("/", func(c *gin.Context) {
		require.NotEmpty(t, before)
		require.Equal(t, before, c.Request.Context().Value(ctxkey.UsageBillingRequestID))
		require.Equal(t, "trusted-correlation", c.Request.Context().Value(ctxkey.ClientRequestID))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "trusted-correlation"))
	router.ServeHTTP(httptest.NewRecorder(), req)
}

// This exercises real HTTP middleware and RecordUsage together. The repository
// models the same key/fingerprint uniqueness as the transactional SQL boundary.
type gatewayIdentityBillingRepo struct {
	service.UsageBillingRepository
	fingerprints map[string]string
	effects      int
}

func (r *gatewayIdentityBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	cmd.Normalize()
	if previous, ok := r.fingerprints[cmd.RequestID]; ok {
		if previous != cmd.RequestFingerprint {
			return nil, service.ErrUsageBillingRequestConflict
		}
		return &service.UsageBillingApplyResult{}, nil
	}
	r.fingerprints[cmd.RequestID] = cmd.RequestFingerprint
	r.effects++
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

type gatewayIdentityUsageRepo struct{ service.UsageLogRepository }

func (*gatewayIdentityUsageRepo) Create(context.Context, *service.UsageLog) (bool, error) {
	return true, nil
}

func TestGatewayBillingIdentityHTTPRequestsSettleIndependentlyAndCallbacksDeduplicate(t *testing.T) {
	for _, changedPayload := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed_payload_%t", changedPayload), func(t *testing.T) {
			billing := &gatewayIdentityBillingRepo{fingerprints: map[string]string{}}
			cfg := &config.Config{}
			cfg.Default.RateMultiplier = 1
			svc := service.NewOpenAIGatewayService(nil, &gatewayIdentityUsageRepo{}, billing, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(RequestLogger(), GatewayClientRequestID())
			router.POST("/v1/responses", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				input := &service.OpenAIRecordUsageInput{
					Result: &service.OpenAIForwardResult{RequestID: "upstream-response", Model: "gpt-5.1", Duration: time.Second, Usage: service.OpenAIUsage{InputTokens: 8, OutputTokens: 4}},
					APIKey: &service.APIKey{ID: 100}, User: &service.User{ID: 200}, Account: &service.Account{ID: 300}, RequestPayloadHash: service.HashUsageRequestPayload(body),
				}
				for range 2 {
					require.NoError(t, svc.RecordUsage(c.Request.Context(), input))
				}
				c.Status(http.StatusOK)
			})
			for i := range 2 {
				body := `{"model":"gpt-5.1","input":"one"}`
				if changedPayload && i == 1 {
					body = `{"model":"gpt-5.1","input":"two"}`
				}
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
				req.Header.Set("X-Client-Request-ID", "reused-correlation")
				req.Header.Set("X-Request-ID", "reused-request")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				require.Equal(t, http.StatusOK, response.Code)
			}
			require.Equal(t, 2, billing.effects)
			require.Len(t, billing.fingerprints, 2)
		})
	}
}
