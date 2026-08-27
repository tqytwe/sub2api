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

type promptCatalogHandlerRepo struct {
	rows       []service.Prompt
	page       *pagination.PaginationResult
	listCalls  int
	listFilter service.PromptListFilter
	publicOnly bool
	userID     *int64
}

func (*promptCatalogHandlerRepo) GetPrompt(context.Context, int64, *int64, bool) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) SavePrompt(context.Context, *service.Prompt, int64) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) ListPromptSources(context.Context, int64) ([]service.PromptSource, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) ListPromptReviews(context.Context, int64, int) ([]service.PromptReviewRecord, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) SetPromptStatus(context.Context, int64, int, service.PromptStatus, int64) (*service.Prompt, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) RollbackPrompt(context.Context, int64, int, int64) (*service.Prompt, error) {
	return nil, nil
}

func (r *promptCatalogHandlerRepo) ListPrompts(
	_ context.Context,
	filter service.PromptListFilter,
	userID *int64,
	publicOnly bool,
) ([]service.Prompt, *pagination.PaginationResult, error) {
	r.listCalls++
	r.listFilter = filter
	r.userID = userID
	r.publicOnly = publicOnly
	return r.rows, r.page, nil
}

func (*promptCatalogHandlerRepo) ListCategories(context.Context, bool) ([]service.PromptCategory, error) {
	return nil, nil
}

func (*promptCatalogHandlerRepo) SetFavorite(context.Context, int64, int64, bool) (bool, error) {
	return false, nil
}

func (*promptCatalogHandlerRepo) UsePrompt(context.Context, int64, int64) (*service.Prompt, error) {
	return nil, nil
}

func TestPromptCatalogReturnsCompletePromptBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	updatedAt := time.Date(2026, 8, 20, 4, 5, 6, 0, time.UTC)
	repo := &promptCatalogHandlerRepo{
		rows: []service.Prompt{{
			ID:               84,
			Status:           service.PromptStatusPublished,
			TitleZH:          "城市短片",
			DescriptionZH:    "夜景镜头",
			Purpose:          "video",
			PublishedVersion: 7,
			PromptText:       "霓虹雨夜中的城市延时摄影",
			Models:           []string{"video-model"},
			CategoryIDs:      []int64{3, 9},
			Media: []service.PromptMedia{{
				MediaType: "image",
				URL:       "https://cdn.example.test/prompt-84.webp",
			}},
			UpdatedAt: updatedAt,
		}},
		page: &pagination.PaginationResult{Total: 251, Page: 2, PageSize: 100, Pages: 3},
	}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/prompts/catalog?media_type=video&page=2&page_size=100",
		nil,
	)

	h.Catalog(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, repo.listCalls)
	require.True(t, repo.publicOnly)
	require.Nil(t, repo.userID)
	require.Equal(t, service.PromptStatusPublished, repo.listFilter.Status)
	require.Equal(t, "video", repo.listFilter.CatalogKind)
	require.Empty(t, repo.listFilter.MediaType)
	require.Equal(t, 2, repo.listFilter.Pagination.Page)
	require.Equal(t, 100, repo.listFilter.Pagination.PageSize)
	require.True(t, repo.listFilter.IncludeContent)
	require.Contains(t, recorder.Body.String(), `"prompt_text":"霓虹雨夜中的城市延时摄影"`)
	require.Contains(t, recorder.Body.String(), `"category_ids":[3,9]`)
	require.Contains(t, recorder.Body.String(), `"total":251`)
}

func TestPromptCatalogRejectsUnsupportedMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogHandlerRepo{}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/prompts/catalog?media_type=audio", nil)

	h.Catalog(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, repo.listCalls)
}

func TestPromptCatalogBoundsRequestedPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogHandlerRepo{
		page: &pagination.PaginationResult{Page: 1, PageSize: 100, Pages: 1},
	}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/prompts/catalog?media_type=image&page_size=1000",
		nil,
	)

	h.Catalog(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 100, repo.listFilter.Pagination.PageSize)
}
