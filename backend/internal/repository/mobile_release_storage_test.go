package repository

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type mobileReleaseStorageBaseFake struct {
	savedKey   string
	openedKey  string
	deletedKey string
}

func (s *mobileReleaseStorageBaseFake) Save(_ context.Context, key, _ string, _ []byte) (string, error) {
	s.savedKey = key
	return "ignored", nil
}

func (s *mobileReleaseStorageBaseFake) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.openedKey = key
	return io.NopCloser(strings.NewReader("artifact")), "application/vnd.android.package-archive", nil
}

func (s *mobileReleaseStorageBaseFake) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	return nil
}

func TestProvideMobileReleaseStorageScopesObjectsUnderConfiguredPrefix(t *testing.T) {
	base := &mobileReleaseStorageBaseFake{}
	cfg := &config.Config{}
	cfg.ImageStorage.Prefix = "tenant-assets/"

	storage := ProvideMobileReleaseStorage(cfg, base)
	key := "mobile-releases/direct/301-release.apk"

	_, err := storage.Save(context.Background(), key, "application/vnd.android.package-archive", []byte("artifact"))
	require.NoError(t, err)
	reader, _, err := storage.Open(context.Background(), key)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.NoError(t, storage.Delete(context.Background(), key))

	require.Equal(t, "tenant-assets/mobile-releases/direct/301-release.apk", base.savedKey)
	require.Equal(t, base.savedKey, base.openedKey)
	require.Equal(t, base.savedKey, base.deletedKey)
}

func TestProvideMobileReleaseStorageUsesDefaultImagePrefix(t *testing.T) {
	for _, configuredPrefix := range []string{"", "/", " /// "} {
		t.Run(configuredPrefix, func(t *testing.T) {
			base := &mobileReleaseStorageBaseFake{}
			cfg := &config.Config{}
			cfg.ImageStorage.Prefix = configuredPrefix
			storage := ProvideMobileReleaseStorage(cfg, base)

			_, err := storage.Save(context.Background(), "mobile-releases/direct/release.apk", "application/vnd.android.package-archive", []byte("artifact"))
			require.NoError(t, err)
			require.Equal(t, "images/mobile-releases/direct/release.apk", base.savedKey)
		})
	}
}
