//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIImagesManagedEligibilityCache struct {
	service.BillingCache
}

type openAIImagesManagedEligibilityQuotaRepo struct {
	service.UserPlatformQuotaRepository
}

func (openAIImagesManagedEligibilityCache) GetUserBalance(context.Context, int64) (float64, error) {
	return 0, nil
}

func (openAIImagesManagedEligibilityCache) GetUserPlatformQuotaCache(context.Context, int64, string) (*service.UserPlatformQuotaCacheEntry, bool, error) {
	zero := 0.0
	now := time.Now()
	return &service.UserPlatformQuotaCacheEntry{
		DailyLimitUSD:    &zero,
		DailyWindowStart: &now,
		SchemaVersion:    service.UserPlatformQuotaCacheSchemaV1,
	}, true, nil
}

func (openAIImagesManagedEligibilityCache) SetUserPlatformQuotaCache(context.Context, int64, string, *service.UserPlatformQuotaCacheEntry, time.Duration) error {
	return nil
}

func TestOpenAIGatewayHandlerImages_ManagedBillingStillEnforcesPlatformQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	billingService := service.NewBillingCacheService(
		openAIImagesManagedEligibilityCache{},
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		&openAIImagesManagedEligibilityQuotaRepo{},
	)
	t.Cleanup(billingService.Stop)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw","size":"1024x1024"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req = req.WithContext(service.WithImageStudioManagedBilling(req.Context()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	groupID := int64(111)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      222,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
		},
		User: &service.User{ID: 333},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 333, Concurrency: 0})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: billingService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}

	h.Images(c)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_exceeded", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Contains(t, gjson.GetBytes(rec.Body.Bytes(), "error.message").String(), "Daily usage quota")
}

func TestOpenAIGatewayHandlerGrokImages_ManagedBillingStillEnforcesPlatformQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	billingService := service.NewBillingCacheService(
		openAIImagesManagedEligibilityCache{},
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		&openAIImagesManagedEligibilityQuotaRepo{},
	)
	t.Cleanup(billingService.Stop)

	body := []byte(`{"model":"grok-imagine-image-quality","prompt":"draw","aspect_ratio":"1:1","resolution":"1k"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req = req.WithContext(service.WithImageStudioManagedBilling(req.Context()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	groupID := int64(112)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      223,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformGrok,
			AllowImageGeneration: true,
		},
		User: &service.User{ID: 334},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 334, Concurrency: 0})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: billingService,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}

	h.GrokImages(c)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_exceeded", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Contains(t, gjson.GetBytes(rec.Body.Bytes(), "error.message").String(), "Daily usage quota")
}
