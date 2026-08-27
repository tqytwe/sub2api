package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPromptCatalogDeltaUsesBoundedRelationQueriesAndOverlapsTiedCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	since := time.Date(2026, 8, 20, 6, 7, 8, 120000, time.UTC)
	boundary := since.Add(-time.Microsecond)
	cursor := time.Date(2026, 8, 20, 7, 8, 9, 120000, time.UTC)

	mock.ExpectQuery(`(?s)SELECT GREATEST`).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(cursor))
	mock.ExpectQuery(`(?s)WITH selected.*LEFT JOIN prompt_media.*LOWER\(BTRIM\(v\.purpose\)\) = 'video'`).
		WithArgs("video").
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "total", "revision"}).
			AddRow(cursor, int64(1), "catalog-revision-v2"))
	mock.ExpectQuery(`(?s)SELECT p\.id, p\.status.*LOWER\(BTRIM\(v\.purpose\)\) = 'video'`).
		WithArgs(boundary, "video").
		WillReturnRows(promptCatalogPromptRows(cursor))
	mock.ExpectQuery(`SELECT pm\.prompt_id, pm\.id, pm\.media_type`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "id", "media_type", "url", "alt_zh", "sort_order"}).
			AddRow(int64(42), int64(100), "image", "https://cdn.example/video-cover.webp", "封面", 0))
	mock.ExpectQuery(`SELECT link\.prompt_id, link\.category_id`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"prompt_id", "category_id"}).
			AddRow(int64(42), int64(7)).AddRow(int64(42), int64(11)))
	mock.ExpectQuery(`(?s)SELECT p\.id\s+FROM prompts p.*prompt_versions historic_version.*LOWER\(BTRIM\(historic_version\.purpose\)\) = 'video'`).
		WithArgs(boundary, "video").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(boundary).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT id, slug, name_zh`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "slug", "name_zh", "description_zh", "dimension", "sort_order", "enabled", "created_at", "updated_at",
		}).AddRow(int64(7), "cinematic", "电影感", "", "style", 0, true, cursor, cursor))

	result, err := NewPromptLibraryRepository(db).GetPublicCatalogDelta(
		context.Background(),
		service.PromptListFilter{CatalogKind: "video"},
		since,
	)

	require.NoError(t, err)
	require.Equal(t, cursor, result.Cursor)
	require.Equal(t, "catalog-revision-v2", result.Version)
	require.Equal(t, `"catalog-revision-v2"`, result.ETag)
	require.Len(t, result.Prompts, 1)
	require.Equal(t, "城市夜景", result.Prompts[0].PromptText)
	require.Equal(t, []int64{7, 11}, result.Prompts[0].CategoryIDs)
	require.Len(t, result.Prompts[0].Media, 1)
	require.Equal(t, []int64{99}, result.DeletedIDs)
	require.True(t, result.CategoriesChanged)
	require.Len(t, result.Categories, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPromptCatalogDeltaRejectsUnsupportedMediaTypeBeforeQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = NewPromptLibraryRepository(db).GetPublicCatalogDelta(
		context.Background(),
		service.PromptListFilter{CatalogKind: "reference"},
		time.Now(),
	)

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
