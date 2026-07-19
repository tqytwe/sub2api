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
		GroupID:           1,
		Name:              "Pro Monthly",
		Description:       "Short storefront copy",
		Price:             19.99,
		Currency:          "USD",
		ValidityDays:      30,
		ValidityUnit:      "days",
		Features:          "Priority models\nHigher quota",
		ProductName:       "GPT Pro Workbench",
		CoverImageURL:     "/assets/plans/pro.webp",
		DetailDescription: "Line one\nLine two",
		ForSale:           true,
		SortOrder:         10,
	})
	require.NoError(t, err)

	require.Equal(t, "GPT Pro Workbench", plan.ProductName)
	require.Equal(t, "/assets/plans/pro.webp", plan.CoverImageURL)
	require.Equal(t, "Line one\nLine two", plan.DetailDescription)
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
	updated, err := svc.UpdatePlan(context.Background(), plan.ID, UpdatePlanRequest{
		ProductName:       &productName,
		CoverImageURL:     &coverImageURL,
		DetailDescription: &detailDescription,
	})
	require.NoError(t, err)

	require.Equal(t, productName, updated.ProductName)
	require.Equal(t, coverImageURL, updated.CoverImageURL)
	require.Equal(t, detailDescription, updated.DetailDescription)
}
