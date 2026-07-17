package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildImageStudioAssetDerivativeUsesRealDimensionsAndBoundedThumbnail(t *testing.T) {
	source := encodeImageStudioTestPNG(t, 800, 400)

	derivative, err := buildImageStudioAssetDerivative(source)

	require.NoError(t, err)
	require.Equal(t, 800, derivative.Width)
	require.Equal(t, 400, derivative.Height)
	require.Equal(t, "2:1", derivative.AspectRatio)
	require.Equal(t, "image/png", derivative.ThumbnailContentType)
	thumbnail, _, err := image.Decode(bytes.NewReader(derivative.ThumbnailData))
	require.NoError(t, err)
	require.Equal(t, 512, thumbnail.Bounds().Dx())
	require.Equal(t, 256, thumbnail.Bounds().Dy())
}

func TestBuildImageStudioAssetDerivativeRejectsInvalidOrUnsafeDimensions(t *testing.T) {
	_, err := buildImageStudioAssetDerivative([]byte("not an image"))
	require.Error(t, err)

	headerOnly := encodeImageStudioTestPNG(t, 1, 1)
	headerOnly[16] = 0x7f
	headerOnly[17] = 0xff
	headerOnly[18] = 0xff
	headerOnly[19] = 0xff
	_, err = buildImageStudioAssetDerivative(headerOnly)
	require.Error(t, err)
}

type imageStudioArchiveRepoStub struct {
	ImageStudioRepository
	job *ImageStudioJob
	err error
}

func (s *imageStudioArchiveRepoStub) GetJob(_ context.Context, userID int64, jobID string) (*ImageStudioJob, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.job == nil || s.job.UserID != userID || s.job.ID != jobID {
		return nil, ErrImageStudioJobNotFound
	}
	copyJob := *s.job
	copyJob.Assets = append([]ImageStudioAsset(nil), s.job.Assets...)
	return &copyJob, nil
}

func TestImageStudioDownloadJobArchiveIncludesLocalAndExternalOriginalAssets(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	firstKey, err := store.Save(10, "asset-1", "image/png", []byte("first-original"))
	require.NoError(t, err)
	secondKey, err := store.Save(10, "asset-2", "image/webp", []byte("second-original"))
	require.NoError(t, err)
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:        "job-archive",
		UserID:    10,
		Status:    ImageStudioJobStatusPartial,
		CreatedAt: time.Now().UTC(),
		Assets: []ImageStudioAsset{
			{ID: "asset-1", SortOrder: 0, StorageKey: firstKey, ContentType: "image/png"},
			{ID: "asset-external", SortOrder: 1, URL: "https://example.invalid/image.png"},
			{ID: "asset-2", SortOrder: 2, StorageKey: secondKey, ContentType: "image/webp"},
		},
	}}
	svc := &ImageStudioService{repo: repo, assetStore: store}
	originalFetch := fetchImageStudioArchiveRemote
	fetchImageStudioArchiveRemote = func(_ context.Context, rawURL string) ([]byte, string, error) {
		require.Equal(t, "https://example.invalid/image.png", rawURL)
		return []byte("external-original"), "image/png", nil
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })

	data, filename, err := svc.OpenJobArchive(context.Background(), 10, "job-archive")

	require.NoError(t, err)
	require.Equal(t, "image-studio-job-arch.zip", filename)
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, reader.File, 3)
	require.Equal(t, "image-01.png", reader.File[0].Name)
	require.Equal(t, "image-02.png", reader.File[1].Name)
	require.Equal(t, "image-03.webp", reader.File[2].Name)
	for i, expected := range [][]byte{
		[]byte("first-original"),
		[]byte("external-original"),
		[]byte("second-original"),
	} {
		rc, err := reader.File[i].Open()
		require.NoError(t, err)
		got, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		require.Equal(t, expected, got)
	}
}

func TestImageStudioDownloadJobArchiveFetchesExternalAssetsWithBoundedConcurrencyAndStableOrder(t *testing.T) {
	assets := []ImageStudioAsset{
		{ID: "asset-4", SortOrder: 4, URL: "https://example.invalid/4.png"},
		{ID: "asset-2", SortOrder: 2, URL: "https://example.invalid/2.png"},
		{ID: "asset-0", SortOrder: 0, URL: "https://example.invalid/0.png"},
		{ID: "asset-3", SortOrder: 3, URL: "https://example.invalid/3.png"},
		{ID: "asset-1", SortOrder: 1, URL: "https://example.invalid/1.png"},
	}
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:     "job-bounded-fetch",
		UserID: 10,
		Status: ImageStudioJobStatusCompleted,
		Assets: assets,
	}}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}
	originalFetch := fetchImageStudioArchiveRemote
	firstRelease := make(chan struct{})
	started := make(chan string, len(assets))
	var active atomic.Int32
	var maxActive atomic.Int32
	fetchImageStudioArchiveRemote = func(ctx context.Context, rawURL string) ([]byte, string, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		started <- rawURL
		if rawURL == "https://example.invalid/0.png" {
			select {
			case <-firstRelease:
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		}
		return []byte(rawURL), "image/png", nil
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })

	type archiveResult struct {
		data []byte
		err  error
	}
	result := make(chan archiveResult, 1)
	go func() {
		data, _, err := svc.OpenJobArchive(context.Background(), 10, "job-bounded-fetch")
		result <- archiveResult{data: data, err: err}
	}()

	for range len(assets) {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("archive did not fetch external assets concurrently")
		}
	}
	require.LessOrEqual(t, maxActive.Load(), int32(2))
	require.Eventually(t, func() bool {
		return active.Load() == 1
	}, time.Second, time.Millisecond)
	close(firstRelease)

	var got archiveResult
	select {
	case got = <-result:
	case <-time.After(time.Second):
		t.Fatal("archive did not finish after all fetches completed")
	}
	require.NoError(t, got.err)
	require.Zero(t, active.Load())

	reader, err := zip.NewReader(bytes.NewReader(got.data), int64(len(got.data)))
	require.NoError(t, err)
	require.Len(t, reader.File, len(assets))
	expectedURLs := []string{
		"https://example.invalid/0.png",
		"https://example.invalid/1.png",
		"https://example.invalid/2.png",
		"https://example.invalid/3.png",
		"https://example.invalid/4.png",
	}
	for index, file := range reader.File {
		require.Equal(t, fmt.Sprintf("image-%02d.png", index+1), file.Name)
		rc, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		require.Equal(t, []byte(expectedURLs[index]), data)
	}
}

func TestImageStudioDownloadJobArchiveCancelsRemainingFetchesOnError(t *testing.T) {
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:     "job-fetch-error",
		UserID: 10,
		Status: ImageStudioJobStatusCompleted,
		Assets: []ImageStudioAsset{
			{ID: "asset-fail", SortOrder: 0, URL: "https://example.invalid/fail.png"},
			{ID: "asset-slow", SortOrder: 1, URL: "https://example.invalid/slow.png"},
			{ID: "asset-pending-1", SortOrder: 2, URL: "https://example.invalid/pending-1.png"},
			{ID: "asset-pending-2", SortOrder: 3, URL: "https://example.invalid/pending-2.png"},
		},
	}}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}
	originalFetch := fetchImageStudioArchiveRemote
	slowStarted := make(chan struct{})
	slowCancelled := make(chan struct{})
	var active atomic.Int32
	var started atomic.Int32
	fetchImageStudioArchiveRemote = func(ctx context.Context, rawURL string) ([]byte, string, error) {
		started.Add(1)
		active.Add(1)
		defer active.Add(-1)
		switch rawURL {
		case "https://example.invalid/fail.png":
			select {
			case <-slowStarted:
				return nil, "", errors.New("fetch failed")
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		case "https://example.invalid/slow.png":
			close(slowStarted)
			<-ctx.Done()
			close(slowCancelled)
			return nil, "", ctx.Err()
		default:
			return []byte("unexpected"), "image/png", nil
		}
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, err := svc.OpenJobArchive(ctx, 10, "job-fetch-error")

	require.ErrorIs(t, err, ErrImageStudioArchiveUnavailable)
	select {
	case <-slowCancelled:
	default:
		t.Fatal("in-flight fetch was not cancelled before archive returned")
	}
	require.Equal(t, int32(2), started.Load())
	require.Zero(t, active.Load())
}

func TestImageStudioDownloadJobArchiveRejectsTotalSizeOverflow(t *testing.T) {
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:     "job-too-large",
		UserID: 10,
		Status: ImageStudioJobStatusCompleted,
		Assets: []ImageStudioAsset{{
			ID:        "asset-external",
			SortOrder: 0,
			URL:       "https://example.invalid/large.png",
		}},
	}}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}
	originalFetch := fetchImageStudioArchiveRemote
	fetchImageStudioArchiveRemote = func(context.Context, string) ([]byte, string, error) {
		return make([]byte, imageStudioArchiveMaxBytes+1), "image/png", nil
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })

	_, _, err := svc.OpenJobArchive(context.Background(), 10, "job-too-large")

	require.ErrorIs(t, err, ErrImageStudioArchiveUnavailable)
}

func TestImageStudioDownloadJobArchiveHonorsRequestTimeout(t *testing.T) {
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:     "job-timeout",
		UserID: 10,
		Status: ImageStudioJobStatusCompleted,
		Assets: []ImageStudioAsset{{
			ID:        "asset-external",
			SortOrder: 0,
			URL:       "https://example.invalid/slow.png",
		}},
	}}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}
	originalFetch := fetchImageStudioArchiveRemote
	fetchImageStudioArchiveRemote = func(ctx context.Context, _ string) ([]byte, string, error) {
		<-ctx.Done()
		return nil, "", ctx.Err()
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, _, err := svc.OpenJobArchive(ctx, 10, "job-timeout")

	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrImageStudioArchiveUnavailable))
}

func TestImageStudioDownloadJobArchiveLimitsConcurrentRequests(t *testing.T) {
	repo := &imageStudioArchiveRepoStub{job: &ImageStudioJob{
		ID:     "job-concurrency",
		UserID: 10,
		Status: ImageStudioJobStatusCompleted,
		Assets: []ImageStudioAsset{{
			ID:        "asset-external",
			SortOrder: 0,
			URL:       "https://example.invalid/slow.png",
		}},
	}}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}
	originalFetch := fetchImageStudioArchiveRemote
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	fetchImageStudioArchiveRemote = func(ctx context.Context, _ string) ([]byte, string, error) {
		started <- struct{}{}
		select {
		case <-release:
			return []byte("external-original"), "image/png", nil
		case <-ctx.Done():
			return nil, "", ctx.Err()
		}
	}
	t.Cleanup(func() { fetchImageStudioArchiveRemote = originalFetch })

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, _, err := svc.OpenJobArchive(context.Background(), 10, "job-concurrency")
			errs <- err
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("archive request did not acquire a concurrency slot")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, _, err := svc.OpenJobArchive(ctx, 10, "job-concurrency")

	require.ErrorIs(t, err, ErrImageStudioArchiveUnavailable)
	select {
	case <-started:
		t.Fatal("third archive request fetched an asset without a concurrency slot")
	default:
	}
	close(release)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
}

func encodeImageStudioTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}
