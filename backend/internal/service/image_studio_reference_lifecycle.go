package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	maxImageStudioReferences          = 4
	imageStudioReferenceMaxDimension  = 16384
	imageStudioReferenceMaxPixelCount = 40_000_000
)

type ImageStudioJobReferenceReader interface {
	ListJobReferencesByID(ctx context.Context, jobID string, ids []string) ([]ImageStudioJobReference, error)
}

type ImageStudioJobReferenceStorageRepository interface {
	ListJobReferenceStorageKeysForJob(ctx context.Context, jobID string) ([]string, error)
}

type ImageStudioReferenceCleanupRepository interface {
	ListExpiredReferences(ctx context.Context, before time.Time) ([]ImageStudioReference, error)
	DeleteReference(ctx context.Context, referenceID string) error
}

type ImageStudioReferenceLifecycleRepository interface {
	GetReferenceForDelete(ctx context.Context, userID int64, referenceID string) (*ImageStudioReference, error)
	DeleteReferenceForUser(ctx context.Context, userID int64, referenceID string) error
}

func validateImageStudioReference(contentType string, data []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return "", ErrImageStudioReferenceInvalid
	}
	if cfg.Width > imageStudioReferenceMaxDimension || cfg.Height > imageStudioReferenceMaxDimension ||
		int64(cfg.Width)*int64(cfg.Height) > imageStudioReferenceMaxPixelCount {
		return "", ErrImageStudioReferenceInvalid
	}
	detected := imageStudioReferenceContentType(format)
	declared := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if detected == "" ||
		(declared != "" && declared != "application/octet-stream" && declared != detected) {
		return "", ErrImageStudioReferenceInvalid
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decoded == nil || imageStudioReferenceContentType(decodedFormat) != detected {
		return "", ErrImageStudioReferenceInvalid
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != cfg.Width || bounds.Dy() != cfg.Height {
		return "", ErrImageStudioReferenceInvalid
	}
	return detected, nil
}

func imageStudioReferenceContentType(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}

func (s *ImageStudioService) copyImageStudioJobReferences(
	job *ImageStudioJob,
	uploads []ImageStudioReference,
) ([]ImageStudioJobReference, error) {
	if job == nil || job.ID == "" || job.UserID <= 0 || s.assetStore == nil {
		return nil, ErrImageStudioReferenceInvalid
	}
	out := make([]ImageStudioJobReference, 0, len(uploads))
	for i, upload := range uploads {
		data, err := s.assetStore.Read(upload.StorageKey)
		if err != nil {
			return nil, errors.Join(
				ErrImageStudioReferenceNotFound.WithCause(err),
				s.deleteImageStudioJobReferenceObjects(out),
			)
		}
		contentType, err := validateImageStudioReference(upload.ContentType, data)
		if err != nil {
			return nil, errors.Join(err, s.deleteImageStudioJobReferenceObjects(out))
		}
		referenceID := uuid.NewString()
		storageKey, err := s.assetStore.Save(
			job.UserID,
			fmt.Sprintf("%s-reference-%s", job.ID, referenceID),
			contentType,
			data,
		)
		if err != nil {
			return nil, errors.Join(err, s.deleteImageStudioJobReferenceObjects(out))
		}
		out = append(out, ImageStudioJobReference{
			ID:          referenceID,
			JobID:       job.ID,
			StorageKey:  storageKey,
			ContentType: contentType,
			ByteSize:    int64(len(data)),
			SortOrder:   i,
		})
	}
	return out, nil
}

func (s *ImageStudioService) deleteImageStudioJobReferenceObjects(references []ImageStudioJobReference) error {
	if s.assetStore == nil {
		return nil
	}
	var deleteErr error
	for _, reference := range references {
		deleteErr = errors.Join(deleteErr, s.assetStore.Delete(reference.StorageKey))
	}
	return deleteErr
}

func (s *ImageStudioService) purgeExpiredUploadReferences(ctx context.Context, now time.Time) error {
	repo, ok := s.repo.(ImageStudioReferenceCleanupRepository)
	if !ok || s.assetStore == nil {
		return nil
	}
	references, err := repo.ListExpiredReferences(ctx, now)
	if err != nil {
		return err
	}
	for _, reference := range references {
		if err := s.assetStore.Delete(reference.StorageKey); err != nil {
			return err
		}
		if err := repo.DeleteReference(ctx, reference.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *ImageStudioService) DeleteReference(ctx context.Context, userID int64, referenceID string) error {
	repo, ok := s.repo.(ImageStudioReferenceLifecycleRepository)
	if !ok || s.assetStore == nil || userID <= 0 || strings.TrimSpace(referenceID) == "" {
		return ErrImageStudioReferenceNotFound
	}
	reference, err := repo.GetReferenceForDelete(ctx, userID, referenceID)
	if err != nil {
		return err
	}
	if err := s.assetStore.Delete(reference.StorageKey); err != nil {
		return err
	}
	return repo.DeleteReferenceForUser(ctx, userID, referenceID)
}

func (s *ImageStudioService) listJobReferenceStorageKeys(
	ctx context.Context,
	jobID string,
) ([]string, error) {
	repo, ok := s.repo.(ImageStudioJobReferenceStorageRepository)
	if !ok {
		return nil, nil
	}
	return repo.ListJobReferenceStorageKeysForJob(ctx, jobID)
}
