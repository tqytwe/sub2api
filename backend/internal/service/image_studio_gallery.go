package service

import (
	"context"
	"strings"
)

func (s *ImageStudioService) ListJobsPage(
	ctx context.Context,
	userID int64,
	page, pageSize int,
) ([]ImageStudioJob, int, error) {
	if !s.IsEnabled(ctx) {
		return nil, 0, ErrImageStudioDisabled
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	jobs, total, err := s.repo.ListJobsPage(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for i := range jobs {
		s.enrichJobAssets(&jobs[i])
		for j := range jobs[i].Assets {
			asset := &jobs[i].Assets[j]
			if strings.TrimSpace(asset.ThumbnailStorageKey) != "" {
				asset.ThumbnailURL = "/api/v1/image-studio/assets/" + asset.ID + "/thumbnail"
			}
		}
	}
	return jobs, total, nil
}

func (s *ImageStudioService) OpenAssetThumbnail(
	ctx context.Context,
	userID int64,
	assetID string,
) ([]byte, string, error) {
	asset, err := s.repo.GetAsset(ctx, userID, assetID)
	if err != nil {
		return nil, "", err
	}
	if s.assetStore == nil || strings.TrimSpace(asset.ThumbnailStorageKey) == "" {
		return nil, "", ErrImageStudioAssetNotFound
	}
	data, err := s.assetStore.Read(asset.ThumbnailStorageKey)
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(asset.ThumbnailContentType)
	if contentType == "" {
		contentType = "image/png"
	}
	return data, contentType, nil
}
