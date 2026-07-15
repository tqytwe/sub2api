package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelCatalogVisibilityRepoStub struct {
	ModelCatalogRepository
	entries []SiteModelCatalogEntry
	err     error
}

func (r *modelCatalogVisibilityRepoStub) ListCatalog(context.Context, CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	return r.entries, r.err
}

func TestModelCatalogService_ListPublicPricingFailsClosed(t *testing.T) {
	t.Run("no guest-visible models", func(t *testing.T) {
		svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{}, nil, nil, nil, nil, nil)
		require.Empty(t, svc.ListPublicPricing(context.Background()))
	})

	t.Run("catalog query error", func(t *testing.T) {
		svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{err: errors.New("database unavailable")}, nil, nil, nil, nil, nil)
		require.Empty(t, svc.ListPublicPricing(context.Background()))
	})
}
