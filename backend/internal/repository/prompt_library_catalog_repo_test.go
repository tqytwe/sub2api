package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestListPromptsVideoCatalogUsesPurposeAndAllowsImageCover(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM prompts.*LOWER\(BTRIM\(v\.purpose\)\) = 'video'`).
		WithArgs("video").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	updatedAt := time.Date(2026, 8, 20, 7, 8, 9, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT p\.id, p\.status.*LOWER\(BTRIM\(v\.purpose\)\) = 'video'`).
		WithArgs("video", 100, 0).
		WillReturnRows(promptCatalogPromptRows(updatedAt))

	mock.ExpectQuery(`SELECT pm\.prompt_id, pm\.id, pm\.media_type`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "id", "media_type", "url", "alt_zh", "sort_order"}).
			AddRow(int64(42), int64(100), "image", "https://cdn.example/video-cover.webp", "封面", 0))
	mock.ExpectQuery(`SELECT link\.prompt_id, link\.category_id`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "category_id"}).AddRow(int64(42), int64(7)).AddRow(int64(42), int64(11)))

	rows, page, err := NewPromptLibraryRepository(db).ListPrompts(
		context.Background(),
		service.PromptListFilter{
			CatalogKind:    "video",
			IncludeContent: true,
			Pagination:     pagination.PaginationParams{Page: 1, PageSize: 100},
		},
		nil,
		true,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, []int64{7, 11}, rows[0].CategoryIDs)
	require.Equal(t, "城市夜景", rows[0].PromptText)
	require.Equal(t, int64(1), page.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPromptsNormalPageRetainsPerPromptMediaLoading(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM prompts`).
		WithArgs("image").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	updatedAt := time.Date(2026, 8, 20, 7, 8, 10, 0, time.UTC)
	mock.ExpectQuery(`SELECT p\.id, p\.status`).
		WithArgs("image", 20, 0).
		WillReturnRows(promptCatalogPromptRows(updatedAt))
	mock.ExpectQuery(`SELECT id, media_type, url, alt_zh, sort_order`).
		WithArgs(int64(42), 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "media_type", "url", "alt_zh", "sort_order"}).
			AddRow(int64(100), "image", "https://cdn.example/image.webp", "封面", 0))

	rows, _, err := NewPromptLibraryRepository(db).ListPrompts(
		context.Background(),
		service.PromptListFilter{MediaType: "image"},
		nil,
		true,
	)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Media, 1)
	require.Empty(t, rows[0].CategoryIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHydratePromptCatalogRelationsBatchesLargeDelta(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	prompts := make([]service.Prompt, promptCatalogRelationBatchSize+1)
	for index := range prompts {
		prompts[index] = service.Prompt{
			ID:               int64(index + 1),
			PublishedVersion: 1,
		}
	}
	for batch := 0; batch < 2; batch++ {
		mock.ExpectQuery(`SELECT pm\.prompt_id, pm\.id, pm\.media_type`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "id", "media_type", "url", "alt_zh", "sort_order"}))
		mock.ExpectQuery(`SELECT link\.prompt_id, link\.category_id`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "category_id"}))
	}

	err = NewPromptLibraryRepository(db).hydratePromptCatalogRelations(context.Background(), prompts, true)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func promptCatalogPromptRows(updatedAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "status", "brand_type", "provenance_type", "authorization_status",
		"source_evidence_verified", "title_zh", "description_zh", "purpose", "style",
		"subject", "featured", "current_version", "published_version", "prompt_text",
		"variables", "models", "sizes", "reference_requirement", "reference_instructions",
		"requires_reference", "public_attribution_note", "use_count", "favorite_count",
		"favorited", "published_at", "created_at", "updated_at",
	}).AddRow(
		int64(42), service.PromptStatusPublished, service.PromptBrandCurated,
		service.PromptProvenanceInternal, service.PromptAuthorizationCurated, true,
		"视频提示词", "测试目录", "video", "cinematic", "city", true,
		3, 3, "城市夜景", []byte(`{}`), "{video-model}", "{16:9}",
		service.PromptReferenceNone, "", false, "", int64(0), int64(0), false,
		nil, updatedAt, updatedAt,
	)
}
