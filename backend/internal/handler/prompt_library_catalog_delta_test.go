package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type promptCatalogDeltaHandlerRepo struct {
	result service.PromptCatalogDeltaResult
	calls  int
	filter service.PromptListFilter
	since  time.Time
}

func (*promptCatalogDeltaHandlerRepo) GetPrompt(context.Context, int64, *int64, bool) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) SavePrompt(context.Context, *service.Prompt, int64) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) ListPromptSources(context.Context, int64) ([]service.PromptSource, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) ListPromptReviews(context.Context, int64, int) ([]service.PromptReviewRecord, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) SetPromptStatus(context.Context, int64, int, service.PromptStatus, int64) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) RollbackPrompt(context.Context, int64, int, int64) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) ListPrompts(context.Context, service.PromptListFilter, *int64, bool) ([]service.Prompt, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*promptCatalogDeltaHandlerRepo) ListCategories(context.Context, bool) ([]service.PromptCategory, error) {
	return nil, nil
}

func (*promptCatalogDeltaHandlerRepo) SetFavorite(context.Context, int64, int64, bool) (bool, error) {
	return false, nil
}

func (*promptCatalogDeltaHandlerRepo) UsePrompt(context.Context, int64, int64) (*service.Prompt, error) {
	return nil, nil
}

func (r *promptCatalogDeltaHandlerRepo) GetPublicCatalogDelta(
	_ context.Context,
	filter service.PromptListFilter,
	since time.Time,
) (*service.PromptCatalogDeltaResult, error) {
	r.calls++
	r.filter = filter
	r.since = since
	copy := r.result
	return &copy, nil
}

func TestPromptCatalogDeltaReturnsCompleteChangedRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cursor := time.Date(2026, 8, 20, 6, 7, 8, 900, time.UTC)
	repo := &promptCatalogDeltaHandlerRepo{
		result: service.PromptCatalogDeltaResult{
			Cursor:     cursor,
			Version:    "catalog-revision-v2",
			ETag:       `"catalog-revision-v2"`,
			DeletedIDs: []int64{18},
			Prompts: []service.Prompt{{
				ID:               17,
				Status:           service.PromptStatusPublished,
				TitleZH:          "城市镜头",
				DescriptionZH:    "夜景延时",
				PublishedVersion: 4,
				PromptText:       "霓虹夜景延时摄影",
				Variables:        map[string]any{"style": "电影感"},
				CategoryIDs:      []int64{3, 9},
				Media: []service.PromptMedia{{
					ID:        5,
					MediaType: "image",
					URL:       "https://cdn.example.test/17.webp",
				}},
			}},
			CategoriesChanged: true,
			Categories: []service.PromptCategory{{
				ID: 3, Slug: "cinematic", NameZH: "电影感", Enabled: true,
			}},
		},
	}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/prompts/catalog/delta?media_type=video&since=2026-08-20T06:00:00Z",
		nil,
	)

	h.CatalogDelta(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "video", repo.filter.CatalogKind)
	require.Empty(t, repo.filter.MediaType)
	require.Equal(t, service.PromptStatusPublished, repo.filter.Status)
	require.Equal(t, time.Date(2026, 8, 20, 6, 0, 0, 0, time.UTC), repo.since)
	require.Equal(t, `"catalog-revision-v2"`, recorder.Header().Get("ETag"))
	require.Contains(t, recorder.Body.String(), `"cursor":"2026-08-20T06:07:08.0000009Z"`)
	require.Contains(t, recorder.Body.String(), `"version":"catalog-revision-v2"`)
	require.Contains(t, recorder.Body.String(), `"etag":"catalog-revision-v2"`)
	require.Contains(t, recorder.Body.String(), `"prompt_text":"霓虹夜景延时摄影"`)
	require.Contains(t, recorder.Body.String(), `"category_ids":[3,9]`)
	require.Contains(t, recorder.Body.String(), `"deleted_ids":[18]`)
	require.Contains(t, recorder.Body.String(), `"categories":[{`)
}

func TestPromptCatalogDeltaHonorsETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogDeltaHandlerRepo{
		result: service.PromptCatalogDeltaResult{
			Cursor:     time.Date(2026, 8, 20, 6, 7, 8, 0, time.UTC),
			Version:    "catalog-revision-v2",
			ETag:       `"catalog-revision-v2"`,
			Prompts:    []service.Prompt{},
			DeletedIDs: []int64{},
		},
	}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/prompts/catalog/delta?media_type=image&since=2026-08-20T06:00:00Z",
		nil,
	)
	request.Header.Set("If-None-Match", `"catalog-revision-v2"`)
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	h.CatalogDelta(ctx)

	require.Equal(t, http.StatusNotModified, recorder.Code)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, `"catalog-revision-v2"`, recorder.Header().Get("ETag"))
}

func TestPromptCatalogDeltaRejectsInvalidMediaAndCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogDeltaHandlerRepo{}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	for _, path := range []string{
		"/api/v1/prompts/catalog/delta?media_type=audio&since=2026-08-20T06:00:00Z",
		"/api/v1/prompts/catalog/delta?media_type=image&since=invalid",
		"/api/v1/prompts/catalog/delta?media_type=image",
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
		h.CatalogDelta(ctx)
		require.Equal(t, http.StatusBadRequest, recorder.Code, path)
	}
	require.Zero(t, repo.calls)
}
