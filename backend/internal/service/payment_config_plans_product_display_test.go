//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newPaymentConfigPlansTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCreatePlanStoresProductDisplayFields(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, nil, nil)

	plan, err := svc.CreatePlan(context.Background(), CreatePlanRequest{
		GroupID:            1,
		Name:               "Pro Monthly",
		Description:        "Short storefront copy",
		Price:              19.99,
		Currency:           "USD",
		ValidityDays:       30,
		ValidityUnit:       "days",
		Features:           "Priority models\nHigher quota",
		ProductName:        "GPT Pro Workbench",
		CoverImageURL:      "/assets/plans/pro.webp",
		DetailDescription:  "Line one\nLine two",
		StorefrontPlatform: "openai",
		StorefrontCategory: "pro",
		StorefrontFeatured: true,
		StorefrontBadge:    "热销",
		ForSale:            true,
		SortOrder:          10,
	})
	require.NoError(t, err)

	require.Equal(t, "GPT Pro Workbench", plan.ProductName)
	require.Equal(t, "/assets/plans/pro.webp", plan.CoverImageURL)
	require.Equal(t, "Line one\nLine two", plan.DetailDescription)
	require.Equal(t, "openai", plan.StorefrontPlatform)
	require.Equal(t, "pro", plan.StorefrontCategory)
	require.True(t, plan.StorefrontFeatured)
	require.Equal(t, "热销", plan.StorefrontBadge)
}

func TestUpdatePlanPatchesProductDisplayFields(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, nil, nil)

	plan, err := svc.CreatePlan(context.Background(), CreatePlanRequest{
		GroupID:      1,
		Name:         "Starter",
		Description:  "Short copy",
		Price:        9.99,
		ValidityDays: 30,
		ValidityUnit: "days",
		ForSale:      true,
	})
	require.NoError(t, err)

	productName := "Starter Product"
	coverImageURL := "data:image/png;base64,QUJD"
	detailDescription := "Detailed storefront copy"
	storefrontPlatform := "anthropic"
	storefrontCategory := "daily"
	storefrontFeatured := true
	storefrontBadge := "日卡"
	updated, err := svc.UpdatePlan(context.Background(), plan.ID, UpdatePlanRequest{
		ProductName:        &productName,
		CoverImageURL:      &coverImageURL,
		DetailDescription:  &detailDescription,
		StorefrontPlatform: &storefrontPlatform,
		StorefrontCategory: &storefrontCategory,
		StorefrontFeatured: &storefrontFeatured,
		StorefrontBadge:    &storefrontBadge,
	})
	require.NoError(t, err)

	require.Equal(t, productName, updated.ProductName)
	require.Equal(t, coverImageURL, updated.CoverImageURL)
	require.Equal(t, detailDescription, updated.DetailDescription)
	require.Equal(t, storefrontPlatform, updated.StorefrontPlatform)
	require.Equal(t, storefrontCategory, updated.StorefrontCategory)
	require.True(t, updated.StorefrontFeatured)
	require.Equal(t, storefrontBadge, updated.StorefrontBadge)
}

func TestCreatePlanInfersStorefrontCategory(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, nil, nil)

	plan, err := svc.CreatePlan(context.Background(), CreatePlanRequest{
		GroupID:      1,
		Name:         "OpenAI 日卡",
		Description:  "Short copy",
		Price:        2.99,
		ValidityDays: 1,
		ValidityUnit: "days",
		ForSale:      true,
	})
	require.NoError(t, err)

	require.Equal(t, "daily", plan.StorefrontCategory)
}
