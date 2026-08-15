package repository

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const defaultMobileReleaseStoragePrefix = "images"

type prefixedMobileReleaseStorage struct {
	base   service.MobileAssetStorage
	prefix string
}

var _ service.MobileReleaseStorage = (*prefixedMobileReleaseStorage)(nil)

// ProvideMobileReleaseStorage keeps release objects inside the configured
// image-storage namespace so bucket policies can remain prefix-scoped.
func ProvideMobileReleaseStorage(cfg *config.Config, base service.MobileAssetStorage) service.MobileReleaseStorage {
	prefix := defaultMobileReleaseStoragePrefix
	if cfg != nil {
		configuredPrefix := strings.Trim(strings.TrimSpace(cfg.ImageStorage.Prefix), "/")
		if configuredPrefix != "" {
			prefix = configuredPrefix
		}
	}
	return &prefixedMobileReleaseStorage{base: base, prefix: prefix}
}

func (s *prefixedMobileReleaseStorage) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	return s.base.Save(ctx, s.scopedKey(key), contentType, data)
}

func (s *prefixedMobileReleaseStorage) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return s.base.Open(ctx, s.scopedKey(key))
}

func (s *prefixedMobileReleaseStorage) Delete(ctx context.Context, key string) error {
	return s.base.Delete(ctx, s.scopedKey(key))
}

func (s *prefixedMobileReleaseStorage) scopedKey(key string) string {
	return path.Join(s.prefix, strings.TrimLeft(key, "/"))
}
