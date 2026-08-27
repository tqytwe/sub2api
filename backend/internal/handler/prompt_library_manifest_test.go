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

type promptCatalogManifestRepoStub struct {
	manifest   service.PromptCatalogManifest
	calls      int
	t          *testing.T
	listRows   []service.Prompt
	listFilter service.PromptListFilter
}

func (s *promptCatalogManifestRepoStub) GetPrompt(context.Context, int64, *int64, bool) (*service.Prompt, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) SavePrompt(context.Context, *service.Prompt, int64) (*service.Prompt, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) ListPromptSources(context.Context, int64) ([]service.PromptSource, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) ListPromptReviews(context.Context, int64, int) ([]service.PromptReviewRecord, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) SetPromptStatus(context.Context, int64, int, service.PromptStatus, int64) (*service.Prompt, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) RollbackPrompt(context.Context, int64, int, int64) (*service.Prompt, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) ListPrompts(_ context.Context, filter service.PromptListFilter, _ *int64, _ bool) ([]service.Prompt, *pagination.PaginationResult, error) {
	s.listFilter = filter
	return s.listRows, &pagination.PaginationResult{Total: int64(len(s.listRows)), Page: 1, PageSize: 100, Pages: 1}, nil
}

func (s *promptCatalogManifestRepoStub) ListCategories(context.Context, bool) ([]service.PromptCategory, error) {
	return nil, nil
}

func (*promptCatalogManifestRepoStub) SetFavorite(context.Context, int64, int64, bool) (bool, error) {
	return false, nil
}

func (*promptCatalogManifestRepoStub) UsePrompt(context.Context, int64, int64) (*service.Prompt, error) {
	return nil, nil
}

func (s *promptCatalogManifestRepoStub) GetPublicCatalogManifest(_ context.Context, filter service.PromptListFilter) (*service.PromptCatalogManifest, error) {
	s.calls++
	require.Equal(s.t, "video", filter.CatalogKind)
	copy := s.manifest
	return &copy, nil
}

// Keep the assertion helper on the stub so the test verifies the catalog-kind
// boundary without exposing a second repository implementation in production.
func (s *promptCatalogManifestRepoStub) setTesting(t *testing.T) { s.t = t }

func TestPromptCatalogManifestReturnsVersionAndHonorsETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogManifestRepoStub{
		manifest: service.PromptCatalogManifest{
			MediaType: "video",
			Total:     1042,
			UpdatedAt: time.Date(2026, 8, 20, 4, 5, 6, 0, time.UTC),
			Revision:  "prompt-catalog-revision",
		},
	}
	repo.setTesting(t)
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))

	first := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(first)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/prompts/manifest?media_type=video", nil)
	h.Manifest(ctx)
	require.Equal(t, http.StatusOK, first.Code)
	require.NotEmpty(t, first.Header().Get("ETag"))
	require.Contains(t, first.Body.String(), `"media_type":"video"`)
	require.Contains(t, first.Body.String(), `"total":1042`)
	require.Contains(t, first.Body.String(), `"revision":"prompt-catalog-revision"`)

	second := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/manifest?media_type=video", nil)
	request.Header.Set("If-None-Match", first.Header().Get("ETag"))
	ctx, _ = gin.CreateTestContext(second)
	ctx.Request = request
	h.Manifest(ctx)
	require.Equal(t, http.StatusNotModified, second.Code)
	require.Equal(t, 2, repo.calls)
}

func TestPromptCatalogManifestRejectsUnsupportedMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogManifestRepoStub{}
	repo.setTesting(t)
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/prompts/manifest?media_type=reference", nil)
	h.Manifest(ctx)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, repo.calls)
}

func TestPromptLibraryListStaysCompactWhenCatalogContentIsRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &promptCatalogManifestRepoStub{
		listRows: []service.Prompt{{
			ID:               88,
			Status:           service.PromptStatusPublished,
			TitleZH:          "本地缓存目录",
			DescriptionZH:    "一次取回正文",
			PromptText:       "完整提示词正文",
			PublishedVersion: 2,
		}},
	}
	h := NewPromptLibraryHandler(service.NewPromptLibraryService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/prompts?media_type=image&page=1&page_size=100&include_content=1", nil)
	h.List(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, repo.listFilter.IncludeContent)
	require.NotContains(t, recorder.Body.String(), `"prompt_text"`)
}
