package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelCatalogRepositoryListScansMediaCapabilities(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	capabilities := `{"version":"v1","adapter":"sensenova","modalities":["image"],"image":{"operations":["create"]}}`
	mock.ExpectQuery(regexp.QuoteMeta("SELECT ") + `(?s).*media_capabilities.*FROM site_model_catalog.*`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "model_name", "platform", "display_name", "use_case", "sort_order",
			"visible_public", "visible_auth", "featured", "group_ids", "tool_capabilities", "media_capabilities",
			"official_input_price", "official_output_price", "official_cache_read_price", "official_cache_write_price",
			"official_source", "official_updated_at", "official_input_manual", "official_output_manual",
			"official_cache_read_manual", "official_cache_write_manual", "price_multiplier", "input_price",
			"output_price", "cache_read_price", "cache_write_price", "billing_mode", "source",
			"source_updated_at", "created_at", "updated_at",
		}).AddRow(
			int64(7), "sensenova-u1-fast", service.PlatformOpenAI, nil, nil, 0,
			false, true, false, nil, nil, capabilities,
			nil, nil, nil, nil, nil, nil, false, false, false, false,
			nil, nil, nil, nil, nil, "token", "manual", nil, now, now,
		))

	repo := NewModelCatalogRepository(db)
	entries, err := repo.ListCatalog(context.Background(), service.CatalogListFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.JSONEq(t, capabilities, string(entries[0].MediaCapabilities))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelCatalogRepositoryDiscoveryUpsertDoesNotOverwriteMediaCapabilities(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)INSERT INTO site_model_catalog.*ON CONFLICT.*media_capabilities = site_model_catalog\.media_capabilities.*RETURNING id, created_at, updated_at`).
		WithArgs(
			"discovered-image", service.PlatformOpenAI, nil, nil, 0,
			false, true, false, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, false, false, false, false,
			"token", "discovery", nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(11), time.Now(), time.Now()))

	repo := NewModelCatalogRepository(db)
	err = repo.UpsertDiscoveryCatalogEntry(context.Background(), &service.SiteModelCatalogEntry{
		ModelName:   "discovered-image",
		Platform:    service.PlatformOpenAI,
		VisibleAuth: true,
		Source:      "discovery",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
