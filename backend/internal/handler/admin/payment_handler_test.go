package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestSanitizeAdminPaymentOrderForResponseAddsCurrency(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{
		ID:          1,
		UserID:      2,
		Amount:      100,
		PayAmount:   108,
		FeeRate:     8,
		OutTradeNo:  "sub2_202606250001",
		PaymentType: "stripe",
		OrderType:   "subscription",
		Status:      "COMPLETED",
		ExpiresAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "USD",
		},
	}

	got := sanitizeAdminPaymentOrderForResponse(order)
	if got == nil {
		t.Fatal("expected sanitized order")
	}
	if got.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", got.Currency)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized order: %v", err)
	}
	if strings.Contains(string(body), "provider_snapshot") {
		t.Fatalf("expected provider_snapshot to be omitted, got %s", string(body))
	}
}

func TestAdminSubscriptionPlansForResponseIncludesCompositeGroupInfo(t *testing.T) {
	weekly := 25.0
	now := time.Now()
	plans := []*dbent.SubscriptionPlan{
		{
			ID:           11,
			GroupID:      7,
			Name:         "All models",
			Description:  "Composite access",
			Price:        19.99,
			Currency:     "CNY",
			ValidityDays: 30,
			ValidityUnit: "days",
			Features:     "OpenAI\nClaude\nGemini\nGrok",
			ProductName:  "Sub2API",
			ForSale:      true,
			SortOrder:    1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	groupInfo := map[int64]service.PlanGroupInfo{
		7: {
			Platform:       service.PlatformComposite,
			Name:           "Bucket 2 composite",
			RateMultiplier: 1.5,
			WeeklyLimitUSD: &weekly,
			ModelScopes:    []string{"openai", "claude", "gemini", "grok"},
		},
	}

	got := adminSubscriptionPlansForResponse(plans, groupInfo)

	if len(got) != 1 {
		t.Fatalf("expected one plan, got %d", len(got))
	}
	if got[0].GroupPlatform != service.PlatformComposite {
		t.Fatalf("expected composite group platform, got %q", got[0].GroupPlatform)
	}
	if got[0].GroupName != "Bucket 2 composite" {
		t.Fatalf("expected group name to be included, got %q", got[0].GroupName)
	}
	if got[0].WeeklyLimitUSD == nil || *got[0].WeeklyLimitUSD != weekly {
		t.Fatalf("expected weekly limit to be included, got %#v", got[0].WeeklyLimitUSD)
	}
	if strings.Join(got[0].ModelScopes, ",") != "openai,claude,gemini,grok" {
		t.Fatalf("expected model scopes to be preserved, got %#v", got[0].ModelScopes)
	}
	// 投影必须保留 ent 原始响应的全部套餐字段：currency 丢失曾导致编辑保存时
	// 静默清空套餐货币（PlanEditDialog 回传空串 → SetCurrency("")）。
	if got[0].Currency != "CNY" {
		t.Fatalf("expected currency to be preserved, got %q", got[0].Currency)
	}
	if !got[0].CreatedAt.Equal(now) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("expected created_at/updated_at to be preserved, got %v / %v", got[0].CreatedAt, got[0].UpdatedAt)
	}
}

func TestAdminPaymentPlayBillingConfigCanBeReadAndUpdated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &adminPaymentPlayBillingSettingRepo{values: map[string]string{}}
	handler := NewPaymentHandler(nil, service.NewPaymentConfigService(nil, repo, nil))
	router := gin.New()
	router.GET("/admin/payment/play-billing/config", handler.GetPlayBillingConfig)
	router.PUT("/admin/payment/play-billing/config", handler.UpdatePlayBillingConfig)

	readBefore := httptest.NewRecorder()
	router.ServeHTTP(readBefore, httptest.NewRequest(http.MethodGet, "/admin/payment/play-billing/config", nil))
	if readBefore.Code != http.StatusOK {
		t.Fatalf("expected read status 200, got %d: %s", readBefore.Code, readBefore.Body.String())
	}

	payload := []byte(`{"products":[
		{"product_id":"jisudeng.balance.50","product_type":"inapp","order_type":"balance","amount":50,"pay_amount":7.99,"currency":"usd","enabled":true},
		{"product_id":"jisudeng.plan.pro.30d","product_type":"inapp","order_type":"subscription","plan_id":7,"amount":19.99,"enabled":false}
	]}`)
	update := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/payment/play-billing/config", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(update, req)
	if update.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d: %s", update.Code, update.Body.String())
	}

	var envelope response.Response
	if err := json.Unmarshal(update.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	raw, err := json.Marshal(envelope.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var cfg service.MobilePlayBillingAdminConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.ConfigSource != "settings" || cfg.ProductCount != 2 || cfg.EnabledProductCount != 1 {
		t.Fatalf("unexpected config summary: %#v", cfg)
	}
	if !strings.Contains(repo.values[service.SettingMobilePlayBillingProducts], `"product_id":"jisudeng.balance.50"`) {
		t.Fatalf("expected mapping to be persisted, got %s", repo.values[service.SettingMobilePlayBillingProducts])
	}
}

type adminPaymentPlayBillingSettingRepo struct {
	values map[string]string
}

func (r *adminPaymentPlayBillingSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *adminPaymentPlayBillingSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.values == nil {
		return "", service.ErrSettingNotFound
	}
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *adminPaymentPlayBillingSettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *adminPaymentPlayBillingSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *adminPaymentPlayBillingSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *adminPaymentPlayBillingSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *adminPaymentPlayBillingSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}
