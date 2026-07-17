package service

import (
	"context"
	"sync"
	"time"
)

const (
	imageStudioUploadConcurrency = 2
	imageStudioUploadRate        = 20
	imageStudioUploadWindow      = time.Minute
	imageStudioUploadLease       = 10 * time.Minute
)

type ImageStudioUploadLimitRepository interface {
	AcquireImageStudioUploadSlot(
		ctx context.Context,
		userID int64,
		now time.Time,
		leaseDuration time.Duration,
		concurrency int,
		rate int,
		window time.Duration,
	) (token string, acquired bool, err error)
	ReleaseImageStudioUploadSlot(ctx context.Context, userID int64, token string, now time.Time) error
}

func (s *ImageStudioService) AcquireReferenceUpload(
	ctx context.Context,
	userID int64,
	now time.Time,
) (func(), error) {
	repo, ok := s.repo.(ImageStudioUploadLimitRepository)
	if !ok || userID <= 0 {
		return nil, ErrImageStudioReferenceRateLimit
	}
	token, acquired, err := repo.AcquireImageStudioUploadSlot(
		ctx,
		userID,
		now,
		imageStudioUploadLease,
		imageStudioUploadConcurrency,
		imageStudioUploadRate,
		imageStudioUploadWindow,
	)
	if err != nil {
		return nil, ErrImageStudioReferenceRateLimit.WithCause(err)
	}
	if !acquired {
		return nil, ErrImageStudioReferenceRateLimit
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = repo.ReleaseImageStudioUploadSlot(releaseCtx, userID, token, time.Now().UTC())
		})
	}, nil
}
