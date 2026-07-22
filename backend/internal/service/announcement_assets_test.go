package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type announcementAssetStorageStub struct {
	key         string
	contentType string
	data        []byte
}

func (s *announcementAssetStorageStub) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.key = key
	s.contentType = contentType
	s.data = data
	return "https://storage.example.com/" + key, nil
}

func (s *announcementAssetStorageStub) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.key = key
	return io.NopCloser(strings.NewReader("image")), "image/png", nil
}

func TestAnnouncementAssetServiceUploadStoresImageAndReturnsPlatformMarkdown(t *testing.T) {
	storage := &announcementAssetStorageStub{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := NewAnnouncementAssetServiceWithResolver(func() (*ImageResultUploader, bool) {
		return uploader, true
	})

	asset, err := svc.Upload(context.Background(), "客服海报.png", "image/png", []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	})

	require.NoError(t, err)
	require.Equal(t, "image/png", asset.ContentType)
	require.True(t, strings.HasPrefix(asset.URL, "/api/v1/announcement-assets/announcements/"))
	require.Contains(t, asset.Markdown, "![客服海报](/api/v1/announcement-assets/announcements/")
	require.True(t, strings.HasPrefix(storage.key, "images/announcements/"))
	require.Equal(t, "image/png", storage.contentType)
}

func TestAnnouncementAssetServiceUploadRejectsNonImagesAndOversizedFiles(t *testing.T) {
	svc := NewAnnouncementAssetServiceWithResolver(func() (*ImageResultUploader, bool) {
		return NewImageResultUploader(&announcementAssetStorageStub{}, "images/", 0, nil), true
	})

	_, err := svc.Upload(context.Background(), "note.txt", "text/plain", []byte("not an image"))
	require.ErrorIs(t, err, ErrAnnouncementAssetInvalidImage)

	_, err = svc.Upload(context.Background(), "huge.png", "image/png", make([]byte, AnnouncementAssetMaxBytes+1))
	require.ErrorIs(t, err, ErrAnnouncementAssetTooLarge)
}

func TestAnnouncementAssetServiceOpenRestrictsToAnnouncementPrefix(t *testing.T) {
	storage := &announcementAssetStorageStub{}
	svc := NewAnnouncementAssetServiceWithResolver(func() (*ImageResultUploader, bool) {
		return NewImageResultUploader(storage, "images/", 0, nil), true
	})

	reader, contentType, err := svc.Open(context.Background(), "/announcements/banner.png")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	require.Equal(t, "image/png", contentType)
	require.Equal(t, "images/announcements/banner.png", storage.key)

	_, _, err = svc.Open(context.Background(), "/../secret.png")
	require.Error(t, err)
}
