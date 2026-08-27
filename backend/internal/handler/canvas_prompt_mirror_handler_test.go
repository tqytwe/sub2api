package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type canvasPromptMirrorHandlerRepo struct {
	manifest service.CanvasPromptMirrorManifest
	catalog  service.CanvasPromptMirrorCatalog
	delta    service.CanvasPromptMirrorDelta
	item     *service.CanvasPromptMirrorCatalogItem
}

func (*canvasPromptMirrorHandlerRepo) Refresh(context.Context, service.CanvasPromptMirrorSnapshot) (*service.CanvasPromptMirrorManifest, error) {
	return nil, nil
}

func (r *canvasPromptMirrorHandlerRepo) Manifest(context.Context) (*service.CanvasPromptMirrorManifest, error) {
	value := r.manifest
	return &value, nil
}

func (r *canvasPromptMirrorHandlerRepo) Catalog(context.Context, int, int) (*service.CanvasPromptMirrorCatalog, error) {
	value := r.catalog
	return &value, nil
}

func (r *canvasPromptMirrorHandlerRepo) Delta(context.Context, time.Time) (*service.CanvasPromptMirrorDelta, error) {
	value := r.delta
	return &value, nil
}

func (r *canvasPromptMirrorHandlerRepo) Get(context.Context, string) (*service.CanvasPromptMirrorCatalogItem, error) {
	if r.item == nil {
		return nil, nil
	}
	value := *r.item
	return &value, nil
}

func newCanvasPromptMirrorHandlerForTest() *CanvasPromptMirrorHandler {
	updated := time.Date(2026, 8, 20, 4, 5, 6, 0, time.UTC)
	item := service.CanvasPromptMirrorCatalogItem{
		ID: "canvas-1", Title: "城市短片", Prompt: "夜景城市延时摄影，镜头缓慢推进",
		PromptText: "夜景城市延时摄影，镜头缓慢推进", Category: "video animation",
		Categories: []string{"video animation", "motion"}, Version: 1, UpdatedAt: updated,
		CoverURL:       "/api/v1/mobile/canvas-prompts/canvas-1/cover",
		CoverSourceURL: "https://raw.githubusercontent.com/example/repo/main/cover.jpg",
		Media:          []service.PromptMedia{{MediaType: "image", URL: "/api/v1/mobile/canvas-prompts/canvas-1/cover"}},
	}
	repo := &canvasPromptMirrorHandlerRepo{
		manifest: service.CanvasPromptMirrorManifest{MediaType: "image", Revision: "canvas-r1", Total: 1, Categories: []string{"video animation", "motion"}, UpdatedAt: updated},
		catalog:  service.CanvasPromptMirrorCatalog{Items: []service.CanvasPromptMirrorCatalogItem{item}, Total: 1, Page: 1, PageSize: 100, Pages: 1, Revision: "canvas-r1", Categories: []string{"video animation", "motion"}},
		delta:    service.CanvasPromptMirrorDelta{Cursor: updated.Format(time.RFC3339Nano), Revision: "canvas-r1", Version: "canvas-r1", ETag: "canvas-r1", Items: []service.CanvasPromptMirrorCatalogItem{item}, DeletedIDs: []string{}},
		item:     &item,
	}
	return NewCanvasPromptMirrorHandler(service.NewCanvasPromptMirrorService(repo))
}

func performCanvasPromptMirrorRequest(h gin.HandlerFunc, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	if parts := strings.Split(strings.Trim(target, "/"), "/"); len(parts) >= 4 && parts[0] == "mobile" && parts[1] == "canvas-prompts" {
		ctx.Params = gin.Params{{Key: "id", Value: parts[2]}}
	}
	h(ctx)
	return recorder
}

func TestCanvasPromptMirrorHandlerCatalogDeliversPromptBodyCoverAndCategories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newCanvasPromptMirrorHandlerForTest()

	catalog := performCanvasPromptMirrorRequest(h.Catalog, http.MethodGet, "/mobile/canvas-prompts/catalog?media_type=video&page=1&page_size=100", nil)
	require.Equal(t, http.StatusOK, catalog.Code)
	require.Contains(t, catalog.Body.String(), `"prompt_text":"夜景城市延时摄影，镜头缓慢推进"`)
	require.Contains(t, catalog.Body.String(), `"cover_url":"/api/v1/mobile/canvas-prompts/canvas-1/cover"`)
	require.Contains(t, catalog.Body.String(), `"categories":["video animation","motion"]`)

	manifest := performCanvasPromptMirrorRequest(h.Manifest, http.MethodGet, "/mobile/canvas-prompts/manifest?media_type=video", nil)
	require.Equal(t, http.StatusOK, manifest.Code)
	require.Equal(t, `"canvas-r1"`, manifest.Header().Get("ETag"))

	notModified := performCanvasPromptMirrorRequest(h.Manifest, http.MethodGet, "/mobile/canvas-prompts/manifest?media_type=video", map[string]string{"If-None-Match": manifest.Header().Get("ETag")})
	require.Equal(t, http.StatusNotModified, notModified.Code)

	delta := performCanvasPromptMirrorRequest(h.Delta, http.MethodGet, "/mobile/canvas-prompts/catalog/delta?media_type=video&since=2026-08-20T00:00:00Z", nil)
	require.Equal(t, http.StatusOK, delta.Code)
	require.Contains(t, delta.Body.String(), `"cursor":"2026-08-20T04:05:06Z"`)
	require.Contains(t, delta.Body.String(), `"prompt_text":"夜景城市延时摄影，镜头缓慢推进"`)
}

type canvasPromptMirrorCoverTransport struct{}

func (canvasPromptMirrorCoverTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
		Body:       io.NopCloser(strings.NewReader("cover-bytes")),
		Request:    request,
	}, nil
}

func TestCanvasPromptMirrorHandlerProxiesAllowedCoverAsImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newCanvasPromptMirrorHandlerForTest()
	h.httpClient = &http.Client{Transport: canvasPromptMirrorCoverTransport{}}
	recorder := performCanvasPromptMirrorRequest(h.Cover, http.MethodGet, "/mobile/canvas-prompts/canvas-1/cover", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/jpeg", recorder.Header().Get("Content-Type"))
	require.Equal(t, "cover-bytes", recorder.Body.String())
}
