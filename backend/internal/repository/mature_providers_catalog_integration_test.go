//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMatureProviderCatalogRevisionAndIsolation(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelRepository(integrationDB).(service.MatureProviderRepository)
	name := "Provider " + uuid.NewString()
	in := service.MatureProviderInput{Name: name, SiteType: "newapi", BaseURL: "https://example.com/"}
	p, err := repo.CreateMatureProvider(ctx, in)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM mature_providers WHERE id=$1`, p.ID)
		require.NoError(t, err)
	})
	require.Equal(t, int64(1), p.Revision)
	require.Equal(t, "https://example.com", p.BaseURL)
	require.Equal(t, "unknown", p.Status)
	page, err := repo.ListMatureProviders(ctx, name, "unknown", 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.Items, 1)
	in.Revision = &p.Revision
	in.Notes = "Updated notes"
	updated, err := repo.UpdateMatureProvider(ctx, p.ID, in)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Revision)
	require.Equal(t, in.Notes, updated.Notes)
	_, err = repo.UpdateMatureProvider(ctx, p.ID, in)
	require.ErrorIs(t, err, service.ErrMatureProviderConflict)
	require.ErrorIs(t, repo.DeleteMatureProvider(ctx, p.ID, p.Revision), service.ErrMatureProviderConflict)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM upstream_channels WHERE lower(name)=lower($1)`, name).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, repo.DeleteMatureProvider(ctx, p.ID, updated.Revision))
	_, err = repo.GetMatureProvider(ctx, p.ID)
	require.ErrorIs(t, err, service.ErrMatureProviderNotFound)
}
