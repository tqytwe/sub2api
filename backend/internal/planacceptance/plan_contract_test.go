package planacceptance

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type planContract struct {
	client     *dbent.Client
	router     *gin.Engine
	plan       *dbent.SubscriptionPlan
	adminToken string
	userToken  string
}

// Default tests use a real disposable SQLite database. The browser contract also
// runs against PostgreSQL; only this dedicated loopback test DB is accepted.
func newPlanContract(t *testing.T) *planContract {
	t.Helper()
	driver, dsn, dialectName := "sqlite", t.TempDir()+"/plans.db?_pragma=foreign_keys(1)", dialect.SQLite
	if raw := os.Getenv("PLAN_CONTRACT_POSTGRES_DSN"); raw != "" {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Equal(t, "postgres", u.Scheme)
		require.Equal(t, "127.0.0.1", u.Hostname())
		require.Equal(t, "/sub2api_plan_edit_test", u.Path)
		require.NotNil(t, u.User)
		require.Equal(t, "sub2api_test", u.User.Username())
		require.Equal(t, "sslmode=disable", u.RawQuery)
		driver, dsn, dialectName = "postgres", raw, dialect.Postgres
	}
	db, err := sql.Open(driver, dsn)
	require.NoError(t, err)
	controlDB := db
	t.Cleanup(func() { _ = controlDB.Close() })
	if dialectName == dialect.Postgres {
		// A fresh database per test runs the actual repository migration chain,
		// including core migrations 273/274; never substitute an Ent-only schema.
		database := "plan_contract_" + hex.EncodeToString(randomContractBytes(t))
		_, err = db.Exec("CREATE DATABASE " + database)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = controlDB.Exec("DROP DATABASE " + database + " WITH (FORCE)") })
		u, _ := url.Parse(dsn)
		u.Path = "/" + database
		isolated, openErr := sql.Open(driver, u.String())
		require.NoError(t, openErr)
		db = isolated
		t.Cleanup(func() { _ = isolated.Close() })
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialectName, db)))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if dialectName == dialect.Postgres {
		require.NoError(t, timezone.Init("UTC"))
		require.NoError(t, repository.ApplyMigrations(ctx, db))
	} else {
		require.NoError(t, client.Schema.Create(ctx))
		// This auth dependency is SQL-managed rather than part of Ent's schema.
		_, err = db.Exec(`CREATE TABLE user_avatars (user_id BIGINT PRIMARY KEY, storage_provider TEXT, storage_key TEXT, url TEXT, content_type TEXT, byte_size BIGINT, sha256 TEXT)`)
		require.NoError(t, err)
	}
	g, err := client.Group.Create().SetName("Contract group").SetPlatform("openai").SetSubscriptionType("subscription").
		SetPeakRateEnabled(true).SetPeakStart("18:00").SetPeakEnd("22:00").SetPeakRateMultiplier(2).Save(ctx)
	require.NoError(t, err)
	plan, err := client.SubscriptionPlan.Create().SetGroupID(g.ID).
		SetName("Isolated contract plan").SetDescription("Original description").
		SetPrice(19.99).SetOriginalPrice(29.99).SetCurrency("USD").
		SetValidityDays(2).SetValidityUnit("months").
		SetRequestLimit(123).SetAmountLimitUsd(45.67).SetTokenLimit(89012).
		SetFeatures(" First feature \nSecond feature ").SetProductName(" Contract product ").
		SetCoverImageURL("/contract-cover.svg").SetDetailDescription(" Detail line one\nLine two ").
		SetStorefrontPlatform("image").SetStorefrontCategory("enterprise").
		SetStorefrontFeatured(true).SetStorefrontBadge("Contract badge").
		SetForSale(true).SetSortOrder(17).Save(ctx)
	require.NoError(t, err)
	// Read the DB's timestamp precision (PostgreSQL uses microseconds) as the baseline.
	plan, err = client.SubscriptionPlan.Get(ctx, plan.ID)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.JWT.Secret = hex.EncodeToString(randomContractBytes(t))
	cfg.JWT.ExpireHour = 1
	userRepo := repository.NewUserRepository(client, db)
	auth := service.NewAuthService(client, userRepo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	tokens := map[string]string{}
	for _, role := range []string{service.RoleAdmin, service.RoleUser} {
		u, createErr := client.User.Create().SetEmail(role + "@plan-contract.invalid").SetPasswordHash("unused").SetRole(role).SetStatus(service.StatusActive).Save(ctx)
		require.NoError(t, createErr)
		user, getErr := userRepo.GetByID(ctx, u.ID)
		require.NoError(t, getErr)
		tokens[role], err = auth.GenerateToken(ctx, user)
		require.NoError(t, err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminRoutes := router.Group("/api/v1/admin")
	adminRoutes.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(auth, service.NewUserService(userRepo, nil, nil, nil), nil, nil)))
	h := admin.NewPaymentHandler(nil, service.NewPaymentConfigService(client, repository.NewSettingRepository(client), nil))
	adminRoutes.GET("/payment/plans", h.ListPlans)
	adminRoutes.PUT("/payment/plans/:id", h.UpdatePlan)
	adminRoutes.GET("/payment/config", h.GetConfig)
	adminRoutes.GET("/payment/storefront", h.GetStorefrontConfig)
	adminRoutes.GET("/payment/api-onboarding", h.GetAPIOnboardingConfig)
	adminRoutes.GET("/groups/all", func(c *gin.Context) {
		groups, queryErr := client.Group.Query().All(c.Request.Context())
		if queryErr != nil {
			response.ErrorFrom(c, queryErr)
			return
		}
		response.Success(c, groups)
	})
	return &planContract{client, router, plan, tokens[service.RoleAdmin], tokens[service.RoleUser]}
}

func randomContractBytes(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func (f *planContract) request(t *testing.T, method, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	path := "/api/v1/admin/payment/plans"
	if method == http.MethodPut {
		path += fmt.Sprintf("/%d", f.plan.ID)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f *planContract) list(t *testing.T) map[string]any {
	t.Helper()
	w := f.request(t, http.MethodGet, f.adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, 1)
	return envelope.Data[0]
}

func contractPlanJSON(t *testing.T, p *dbent.SubscriptionPlan) map[string]any {
	t.Helper()
	b, err := json.Marshal(p)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(b, &result))
	return result
}

func TestAdminPlanHTTPContract(t *testing.T) {
	f := newPlanContract(t)
	original := contractPlanJSON(t, f.plan)
	t.Run("list_preserves_every_persisted_field", func(t *testing.T) {
		row := f.list(t)
		for key, want := range original {
			assert.Equal(t, want, row[key], key)
		}
		assert.Equal(t, true, row["peak_rate_enabled"])
		assert.Equal(t, "18:00", row["peak_start"])
		assert.Equal(t, "22:00", row["peak_end"])
		assert.Equal(t, float64(2), row["peak_rate_multiplier"])
	})
	t.Run("legacy_full_edit_roundtrip_preserves_configuration", func(t *testing.T) {
		row := f.list(t)
		// Reproduce the shipped dialog's fallbacks against actual HTTP output.
		for _, key := range []string{"cover_image_url", "detail_description", "storefront_badge"} {
			if row[key] == nil {
				row[key] = ""
			}
		}
		if row["storefront_platform"] == nil {
			row["storefront_platform"] = "openai"
		}
		if row["storefront_category"] == nil {
			row["storefront_category"] = "pro"
		}
		if row["storefront_featured"] == nil {
			row["storefront_featured"] = false
		}
		row["description"] = "Edited description"
		w := f.request(t, http.MethodPut, f.adminToken, row)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		persisted, err := f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
		require.NoError(t, err)
		after := contractPlanJSON(t, persisted)
		listed := f.list(t)
		for key, want := range original {
			if key == "updated_at" {
				continue
			}
			if key == "description" {
				want = "Edited description"
			}
			assert.Equal(t, want, after[key], "database: %s", key)
			assert.Equal(t, want, listed[key], "GET after PUT: %s", key)
		}
	})
}

func TestAdminPlanProjectionCoversModelColumns(t *testing.T) {
	// Guard against another manually added projection silently omitting a model
	// column, including future fields whose zero value is omitted by Ent JSON.
	projection := reflect.TypeOf(admin.AdminSubscriptionPlanResult{})
	fields := map[string]bool{}
	for i := 0; i < projection.NumField(); i++ {
		fields[strings.Split(projection.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	for _, column := range subscriptionplan.Columns {
		assert.True(t, fields[column], "missing projection column: %s", column)
	}
}

func TestAdminPlanHTTPPatchAndPermissions(t *testing.T) {
	f := newPlanContract(t)
	t.Run("omission_preserves_and_explicit_values_clear", func(t *testing.T) {
		before := contractPlanJSON(t, f.plan)
		w := f.request(t, http.MethodPut, f.adminToken, map[string]any{"description": "Only description"})
		require.Equal(t, http.StatusOK, w.Code)
		p, err := f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
		require.NoError(t, err)
		after := contractPlanJSON(t, p)
		for key, value := range before {
			if key != "description" && key != "updated_at" {
				require.Equal(t, value, after[key], key)
			}
		}
		w = f.request(t, http.MethodPut, f.adminToken, map[string]any{
			"cover_image_url": "", "detail_description": "", "storefront_platform": "", "storefront_category": "", "storefront_badge": "",
			"storefront_featured": false, "for_sale": false, "sort_order": 0, "original_price": 0,
			"clear_request_limit": true, "clear_amount_limit_usd": true, "clear_token_limit": true,
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		p, err = f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
		require.NoError(t, err)
		require.Empty(t, p.CoverImageURL)
		require.Empty(t, p.DetailDescription)
		require.Empty(t, p.StorefrontPlatform)
		require.Empty(t, p.StorefrontCategory)
		require.Empty(t, p.StorefrontBadge)
		require.False(t, p.StorefrontFeatured)
		require.False(t, p.ForSale)
		require.Zero(t, p.SortOrder)
		require.NotNil(t, p.OriginalPrice)
		require.Zero(t, *p.OriginalPrice)
		require.Nil(t, p.RequestLimit)
		require.Nil(t, p.AmountLimitUsd)
		require.Nil(t, p.TokenLimit)
		row := f.list(t)
		for _, key := range []string{"cover_image_url", "detail_description", "storefront_platform", "storefront_category", "storefront_badge"} {
			require.Equal(t, "", row[key], key)
		}
		require.Equal(t, false, row["storefront_featured"])
	})
	t.Run("invalid_or_unauthorized_requests_do_not_write", func(t *testing.T) {
		p, err := f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
		require.NoError(t, err)
		before := contractPlanJSON(t, p)
		for _, payload := range []map[string]any{
			{"name": ""}, {"price": -1}, {"price": 0}, {"price": "bad"}, {"validity_days": 0}, {"group_id": 0},
			{"currency": "invalid"}, {"original_price": -1}, {"request_limit": 0}, {"token_limit": -1},
			{"amount_limit_usd": -1}, {"request_limit": 5, "clear_request_limit": true},
		} {
			w := f.request(t, http.MethodPut, f.adminToken, payload)
			require.Equal(t, http.StatusBadRequest, w.Code, "%v: %s", payload, w.Body.String())
		}
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			require.Equal(t, http.StatusUnauthorized, f.request(t, method, "", map[string]any{"description": "unauthorized"}).Code)
			require.Equal(t, http.StatusForbidden, f.request(t, method, f.userToken, map[string]any{"description": "forbidden"}).Code)
		}
		p, err = f.client.SubscriptionPlan.Get(context.Background(), f.plan.ID)
		require.NoError(t, err)
		require.Equal(t, before, contractPlanJSON(t, p))
	})
}
