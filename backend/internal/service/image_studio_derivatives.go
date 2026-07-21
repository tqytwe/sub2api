package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	imageStudioThumbnailMaxSide = 512
	imageStudioMaxDimension     = 16384
	imageStudioMaxPixels        = 40_000_000
	imageStudioArchiveMaxAssets = 100
	imageStudioArchiveMaxBytes  = 64 << 20
	imageStudioArchiveTimeout   = 30 * time.Second
	imageStudioArchiveFetchers  = 2
)

var (
	errImageStudioArchiveSizeExceeded = errors.New("image studio archive size limit exceeded")
	imageStudioArchivePermits         = make(chan struct{}, 2)
	fetchImageStudioArchiveRemote     = FetchImageStudioRemoteURL
)

type imageStudioAssetDerivative struct {
	Width                int
	Height               int
	AspectRatio          string
	ThumbnailData        []byte
	ThumbnailContentType string
}

func buildImageStudioAssetDerivative(data []byte) (*imageStudioAssetDerivative, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, errors.New("invalid generated image dimensions")
	}
	if cfg.Width > imageStudioMaxDimension || cfg.Height > imageStudioMaxDimension ||
		int64(cfg.Width)*int64(cfg.Height) > imageStudioMaxPixels {
		return nil, errors.New("generated image dimensions exceed safety limits")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("invalid generated image")
	}
	width, height := boundedImageStudioThumbnailSize(cfg.Width, cfg.Height)
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	var thumbnail bytes.Buffer
	if err := png.Encode(&thumbnail, dst); err != nil {
		return nil, err
	}
	return &imageStudioAssetDerivative{
		Width:                cfg.Width,
		Height:               cfg.Height,
		AspectRatio:          imageStudioAspectRatio(cfg.Width, cfg.Height),
		ThumbnailData:        thumbnail.Bytes(),
		ThumbnailContentType: "image/png",
	}, nil
}

func boundedImageStudioThumbnailSize(width, height int) (int, int) {
	if width <= imageStudioThumbnailMaxSide && height <= imageStudioThumbnailMaxSide {
		return width, height
	}
	if width >= height {
		return imageStudioThumbnailMaxSide, max(1, height*imageStudioThumbnailMaxSide/width)
	}
	return max(1, width*imageStudioThumbnailMaxSide/height), imageStudioThumbnailMaxSide
}

func imageStudioAspectRatio(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	divisor := imageStudioGCD(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor)
}

func imageStudioGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a <= 0 {
		return 1
	}
	return a
}

func (s *ImageStudioService) OpenJobArchive(ctx context.Context, userID int64, jobID string) ([]byte, string, error) {
	if s.repo == nil || s.assetStore == nil {
		return nil, "", ErrImageStudioArchiveUnavailable
	}
	archiveCtx, cancel := context.WithTimeout(ctx, imageStudioArchiveTimeout)
	defer cancel()
	select {
	case imageStudioArchivePermits <- struct{}{}:
		defer func() { <-imageStudioArchivePermits }()
	case <-archiveCtx.Done():
		return nil, "", ErrImageStudioArchiveUnavailable.WithCause(archiveCtx.Err())
	}
	job, err := s.repo.GetJob(archiveCtx, userID, jobID)
	if err != nil {
		return nil, "", err
	}
	if job.Status != ImageStudioJobStatusCompleted && job.Status != ImageStudioJobStatusPartial {
		return nil, "", ErrImageStudioArchiveUnavailable
	}
	assets := append([]ImageStudioAsset(nil), job.Assets...)
	if len(assets) == 0 || len(assets) > imageStudioArchiveMaxAssets {
		return nil, "", ErrImageStudioArchiveUnavailable
	}
	now := time.Now().UTC()
	for i := range assets {
		if imageStudioAssetExpired(&assets[i], now) {
			return nil, "", ErrImageStudioAssetExpired
		}
	}
	sort.SliceStable(assets, func(i, j int) bool { return assets[i].SortOrder < assets[j].SortOrder })
	archiveAssets, err := s.readImageStudioArchiveAssets(archiveCtx, assets)
	if err != nil {
		return nil, "", ErrImageStudioArchiveUnavailable.WithCause(err)
	}
	out := &imageStudioArchiveBuffer{limit: imageStudioArchiveMaxBytes}
	writer := zip.NewWriter(out)
	for index, asset := range assets {
		if err := archiveCtx.Err(); err != nil {
			_ = writer.Close()
			return nil, "", ErrImageStudioArchiveUnavailable.WithCause(err)
		}
		header := &zip.FileHeader{
			Name:   fmt.Sprintf("image-%02d%s", asset.SortOrder+1, extensionForContentType(archiveAssets[index].contentType)),
			Method: zip.Store,
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, "", ErrImageStudioArchiveUnavailable.WithCause(err)
		}
		if _, err := io.Copy(entry, bytes.NewReader(archiveAssets[index].data)); err != nil {
			_ = writer.Close()
			return nil, "", ErrImageStudioArchiveUnavailable.WithCause(err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", ErrImageStudioArchiveUnavailable.WithCause(err)
	}
	shortID := strings.TrimSpace(jobID)
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return out.Bytes(), "image-studio-" + shortID + ".zip", nil
}

type imageStudioArchiveAssetData struct {
	data        []byte
	contentType string
}

func (s *ImageStudioService) readImageStudioArchiveAssets(
	ctx context.Context,
	assets []ImageStudioAsset,
) ([]imageStudioArchiveAssetData, error) {
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]imageStudioArchiveAssetData, len(assets))
	jobs := make(chan int, len(assets))
	for index := range assets {
		jobs <- index
	}
	close(jobs)

	var (
		workers     sync.WaitGroup
		sizeMu      sync.Mutex
		sourceBytes int64
		firstErr    error
		errOnce     sync.Once
	)
	fail := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}
	worker := func() {
		defer workers.Done()
		for {
			select {
			case <-fetchCtx.Done():
				return
			case index, ok := <-jobs:
				if !ok {
					return
				}
				if err := fetchCtx.Err(); err != nil {
					return
				}
				data, contentType, err := s.readImageStudioArchiveAsset(fetchCtx, assets[index])
				if err != nil {
					fail(err)
					return
				}
				sizeMu.Lock()
				if sourceBytes+int64(len(data)) > imageStudioArchiveMaxBytes {
					err = errImageStudioArchiveSizeExceeded
				} else {
					sourceBytes += int64(len(data))
				}
				sizeMu.Unlock()
				if err != nil {
					fail(err)
					return
				}
				results[index] = imageStudioArchiveAssetData{
					data:        data,
					contentType: contentType,
				}
			}
		}
	}

	workerCount := min(imageStudioArchiveFetchers, len(assets))
	workers.Add(workerCount)
	for range workerCount {
		go worker()
	}
	workers.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *ImageStudioService) readImageStudioArchiveAsset(
	ctx context.Context,
	asset ImageStudioAsset,
) ([]byte, string, error) {
	if storageKey := strings.TrimSpace(asset.StorageKey); storageKey != "" {
		if asset.ByteSize > imageStudioArchiveMaxBytes {
			return nil, "", errImageStudioArchiveSizeExceeded
		}
		data, err := s.assetStore.Read(storageKey)
		if err != nil {
			return nil, "", err
		}
		contentType := strings.TrimSpace(asset.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		return data, contentType, nil
	}
	if rawURL := strings.TrimSpace(asset.URL); rawURL != "" {
		return fetchImageStudioArchiveRemote(ctx, rawURL)
	}
	return nil, "", errors.New("image studio archive asset has no source")
}

type imageStudioArchiveBuffer struct {
	bytes.Buffer
	limit int64
}

func (w *imageStudioArchiveBuffer) Write(p []byte) (int, error) {
	if int64(w.Len())+int64(len(p)) > w.limit {
		return 0, errImageStudioArchiveSizeExceeded
	}
	return w.Buffer.Write(p)
}
