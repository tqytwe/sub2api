package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPromptCatalogManifestUsesPublishedMediaRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	updatedAt := time.Date(2026, 8, 20, 6, 7, 8, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH selected.*LEFT JOIN prompt_media.*LOWER\(BTRIM\(v\.purpose\)\) = 'video'`).
		WithArgs("video").
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "total", "revision"}).
			AddRow(updatedAt, int64(1001), "catalog-revision"))

	manifest, err := NewPromptLibraryRepository(db).GetPublicCatalogManifest(
		context.Background(),
		service.PromptListFilter{CatalogKind: "video"},
	)
	require.NoError(t, err)
	require.Equal(t, "video", manifest.MediaType)
	require.Equal(t, int64(1001), manifest.Total)
	require.Equal(t, updatedAt, manifest.UpdatedAt)
	require.Equal(t, "catalog-revision", manifest.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPromptCatalogManifestSupportsEmptyCatalog(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	updatedAt := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH selected`).
		WithArgs("image").
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "total", "revision"}).
			AddRow(updatedAt, int64(0), "empty-catalog-revision"))

	manifest, err := NewPromptLibraryRepository(db).GetPublicCatalogManifest(
		context.Background(),
		service.PromptListFilter{CatalogKind: "image"},
	)
	require.NoError(t, err)
	require.Equal(t, int64(0), manifest.Total)
	require.Equal(t, "empty-catalog-revision", manifest.Revision)
	require.Equal(t, updatedAt, manifest.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPromptCatalogManifestRejectsUnsupportedMediaTypeBeforeQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = NewPromptLibraryRepository(db).GetPublicCatalogManifest(
		context.Background(),
		service.PromptListFilter{CatalogKind: "reference"},
	)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
