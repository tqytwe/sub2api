package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStudioUploadLimitRepoStub struct {
	ImageStudioRepository
	token        string
	acquired     bool
	acquireErr   error
	releaseCalls int
}

func (s *imageStudioUploadLimitRepoStub) AcquireImageStudioUploadSlot(
	context.Context,
	int64,
	time.Time,
	time.Duration,
	int,
	int,
	time.Duration,
) (string, bool, error) {
	return s.token, s.acquired, s.acquireErr
}

func (s *imageStudioUploadLimitRepoStub) ReleaseImageStudioUploadSlot(
	context.Context,
	int64,
	string,
	time.Time,
) error {
	s.releaseCalls++
	return nil
}

func TestImageStudioAcquireReferenceUploadUsesPersistentSlot(t *testing.T) {
	repo := &imageStudioUploadLimitRepoStub{token: "slot-1", acquired: true}
	svc := &ImageStudioService{repo: repo}

	release, err := svc.AcquireReferenceUpload(context.Background(), 42, time.Now().UTC())

	require.NoError(t, err)
	require.NotNil(t, release)
	release()
	release()
	require.Equal(t, 1, repo.releaseCalls)
}

func TestImageStudioAcquireReferenceUploadFailsClosed(t *testing.T) {
	for name, repo := range map[string]*imageStudioUploadLimitRepoStub{
		"limited": {acquired: false},
		"storage error": {
			acquireErr: errors.New("database unavailable"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &ImageStudioService{repo: repo}
			release, err := svc.AcquireReferenceUpload(context.Background(), 42, time.Now().UTC())
			require.Nil(t, release)
			require.ErrorIs(t, err, ErrImageStudioReferenceRateLimit)
		})
	}
}
