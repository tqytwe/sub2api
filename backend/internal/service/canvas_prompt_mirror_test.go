package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type canvasPromptMirrorTestSource struct {
	pages map[int]canvasPromptMirrorSourcePage
}

func (s canvasPromptMirrorTestSource) FetchPage(_ context.Context, page, _ int) (canvasPromptMirrorSourcePage, error) {
	result, ok := s.pages[page]
	if !ok {
		return canvasPromptMirrorSourcePage{}, errors.New("unexpected page")
	}
	return result, nil
}

type canvasPromptMirrorTestRepo struct {
	snapshot *CanvasPromptMirrorSnapshot
}

func (r *canvasPromptMirrorTestRepo) Refresh(_ context.Context, snapshot CanvasPromptMirrorSnapshot) (*CanvasPromptMirrorManifest, error) {
	r.snapshot = &snapshot
	return &CanvasPromptMirrorManifest{
		MediaType: "image", Revision: snapshot.Revision, Total: int64(len(snapshot.Items)),
		Categories: snapshot.Categories, UpdatedAt: snapshot.RefreshedAt,
	}, nil
}

func (*canvasPromptMirrorTestRepo) Manifest(context.Context) (*CanvasPromptMirrorManifest, error) {
	return nil, nil
}
func (*canvasPromptMirrorTestRepo) Catalog(context.Context, int, int) (*CanvasPromptMirrorCatalog, error) {
	return nil, nil
}
func (*canvasPromptMirrorTestRepo) Delta(context.Context, time.Time) (*CanvasPromptMirrorDelta, error) {
	return nil, nil
}
func (*canvasPromptMirrorTestRepo) Get(context.Context, string) (*CanvasPromptMirrorCatalogItem, error) {
	return nil, nil
}

func canvasMirrorSourcePage(total int64, items ...canvasPromptMirrorSourceItem) canvasPromptMirrorSourcePage {
	return canvasPromptMirrorSourcePage{Code: 0, Data: struct {
		Items []canvasPromptMirrorSourceItem `json:"items"`
		Total int64                          `json:"total"`
	}{Items: items, Total: total}}
}

func TestCanvasPromptMirrorRefreshAcceptsMarkdownPreview(t *testing.T) {
	repo := &canvasPromptMirrorTestRepo{}
	source := canvasPromptMirrorTestSource{pages: map[int]canvasPromptMirrorSourcePage{
		1: canvasMirrorSourcePage(2, canvasPromptMirrorSourceItem{
			ID: "one", Title: "One", Prompt: "prompt one", Category: "portrait",
			Preview:   "![](https://raw.githubusercontent.com/example/one.jpg)",
			CoverURL:  "https://raw.githubusercontent.com/example/one.jpg",
			GithubURL: "https://github.com/example/prompts", CreatedAt: "2026-08-20T00:00:00Z", UpdatedAt: "2026-08-20T00:00:00Z",
		}),
		2: canvasMirrorSourcePage(2, canvasPromptMirrorSourceItem{
			ID: "two", Title: "Two", Prompt: "prompt two", Category: "landscape",
			CoverURL: "https://pbs.twimg.com/media/two.jpg", UpdatedAt: "2026-08-20T00:00:01Z",
		}),
	}}
	service := NewCanvasPromptMirrorServiceWithSource(repo, source)
	service.now = func() time.Time { return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC) }

	manifest, err := service.Refresh(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), manifest.Total)
	require.Equal(t, []string{"landscape", "portrait"}, manifest.Categories)
	require.NotEmpty(t, manifest.Revision)
	require.Len(t, repo.snapshot.Items, 2)
	require.Equal(t, "![](https://raw.githubusercontent.com/example/one.jpg)", repo.snapshot.Items[0].Preview)
}

func TestCanvasPromptMirrorRefreshRejectsChangingTotal(t *testing.T) {
	source := canvasPromptMirrorTestSource{pages: map[int]canvasPromptMirrorSourcePage{
		1: canvasMirrorSourcePage(2, canvasPromptMirrorSourceItem{ID: "one", Title: "One", Prompt: "prompt"}),
		2: canvasMirrorSourcePage(3, canvasPromptMirrorSourceItem{ID: "two", Title: "Two", Prompt: "prompt"}),
	}}
	service := NewCanvasPromptMirrorServiceWithSource(&canvasPromptMirrorTestRepo{}, source)
	_, err := service.Refresh(context.Background())
	require.ErrorIs(t, err, ErrCanvasPromptMirrorInvalidPage)
}

func TestCanvasPromptMirrorWorkerSkipsPeerLeadership(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	acquired, err := cache.TryAcquireLeaderLock(context.Background(), canvasPromptMirrorLeaderLockKey, "peer", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	repo := &canvasPromptMirrorTestRepo{}
	worker := NewCanvasPromptMirrorWorker(
		NewCanvasPromptMirrorServiceWithSource(repo, canvasPromptMirrorTestSource{}),
		time.Hour,
		time.Second,
	).SetLeaderLock(cache, nil)
	require.NoError(t, worker.RunOnce(context.Background()))
	require.Nil(t, repo.snapshot)
}
