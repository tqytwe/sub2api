//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestModelCatalogRepository_OfficialAndSitePricesRemainSeparate(t *testing.T) {
	ctx := context.Background()
	repo := NewModelCatalogRepository(integrationDB)
	model := "catalog-integration-" + uuid.NewString()
	manualInput := 9.0 / 1_000_000
	manualOutput := 45.0 / 1_000_000
	entry := &service.SiteModelCatalogEntry{
		ModelName:   model,
		Platform:    service.PlatformOpenAI,
		VisibleAuth: true,
		InputPrice:  &manualInput,
		OutputPrice: &manualOutput,
		BillingMode: string(service.BillingModeToken),
		Source:      "manual",
	}
	require.NoError(t, repo.UpsertCatalogEntry(ctx, entry))
	t.Cleanup(func() { _ = repo.DeleteCatalogEntry(context.Background(), entry.ID) })

	officialInput := 5.0 / 1_000_000
	officialOutput := 30.0 / 1_000_000
	officialCacheRead := 0.5 / 1_000_000
	officialCacheWrite := 6.25 / 1_000_000
	updated, err := repo.UpdateCatalogOfficialPrices(
		ctx, model, service.PlatformOpenAI, "aihubmix",
		&officialInput, &officialOutput, &officialCacheRead, &officialCacheWrite, time.Now(),
	)
	require.NoError(t, err)
	require.Equal(t, 1, updated)

	got, err := repo.GetCatalogEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.InDelta(t, officialInput, *got.OfficialInputPrice, 1e-12)
	require.InDelta(t, manualInput, *got.InputPrice, 1e-12, "official refresh must not overwrite a manual site price")
	require.Equal(t, "aihubmix", got.OfficialSource)

	multiplier := 0.8
	updated, err = repo.BatchUpdatePrices(ctx, []int64{entry.ID}, &multiplier, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, updated)

	got, err = repo.GetCatalogEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.InDelta(t, officialInput*multiplier, *got.InputPrice, 1e-12)
	require.InDelta(t, officialOutput*multiplier, *got.OutputPrice, 1e-12)
	require.InDelta(t, officialCacheRead*multiplier, *got.CacheReadPrice, 1e-12)
	require.InDelta(t, officialCacheWrite*multiplier, *got.CacheWritePrice, 1e-12)
	require.InDelta(t, multiplier, *got.PriceMultiplier, 1e-12)
}
