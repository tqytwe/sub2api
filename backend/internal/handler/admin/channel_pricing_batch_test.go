package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type batchPricingRepository struct {
	service.ChannelRepository
	called bool
	items  []service.ChannelPricingBatchItem
}

func (r *batchPricingRepository) ListAll(context.Context) ([]service.Channel, error) { return nil, nil }

func (r *batchPricingRepository) BatchReplaceModelPricing(_ context.Context, items []service.ChannelPricingBatchItem) error {
	r.called = true
	r.items = items
	return nil
}

func TestChannelPricingBatchHTTPValidatesBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{
		`{}`,
		`{"items":[]}`,
		`{"items":[{"channel_id":0,"model_pricing":[]}]}`,
		`{"items":[{"channel_id":7}]}`,
		`{"items":[{"channel_id":7,"model_pricing":null}]}`,
		`{"items":[{"channel_id":7,"model_pricing":[{"platform":"openai","models":[],"billing_mode":"token"}]}]}`,
		`{"items":[{"channel_id":7,"model_pricing":[{"platform":"openai","models":["gpt-x"],"billing_mode":"unsupported"}]}]}`,
		`{"items":[{"channel_id":7,"model_pricing":[]},{"channel_id":7,"model_pricing":[]}]}`,
		`{"items":[{"channel_id":7,"model_pricing":[{"platform":"openai","models":["gpt-x"],"billing_mode":"token","input_price":-1}]}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			repo := &batchPricingRepository{}
			router := gin.New()
			handler := NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil)
			router.POST("/channels/pricing/batch", handler.BatchUpdatePricing)
			request := httptest.NewRequest("POST", "/channels/pricing/batch", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 400, response.Code)
			require.False(t, repo.called)
		})
	}
}

func TestChannelPricingBatchHTTPValidRequestReachesService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &batchPricingRepository{}
	router := gin.New()
	handler := NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil)
	router.POST("/channels/pricing/batch", handler.BatchUpdatePricing)
	request := httptest.NewRequest("POST", "/channels/pricing/batch", strings.NewReader(`{"items":[{"channel_id":7,"model_pricing":[{"platform":"openai","models":["gpt-x"],"billing_mode":"token","input_price":0.25}]}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.True(t, repo.called)
	require.Len(t, repo.items, 1)
	require.EqualValues(t, 7, repo.items[0].ChannelID)
	require.Equal(t, []string{"gpt-x"}, repo.items[0].Pricing[0].Models)
}

func TestChannelPricingBatchHTTPClearsExplicitEmptyPrices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &batchPricingRepository{}
	router := gin.New()
	handler := NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil)
	router.POST("/channels/pricing/batch", handler.BatchUpdatePricing)
	request := httptest.NewRequest("POST", "/channels/pricing/batch", strings.NewReader(`{"items":[{"channel_id":7,"model_pricing":[]}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.True(t, repo.called)
	require.NotNil(t, repo.items[0].Pricing)
	require.Empty(t, repo.items[0].Pricing)
}
