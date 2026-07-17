package repository

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// LocalImageStorage stores async image task outputs on the instance data volume.
type LocalImageStorage struct {
	root      string
	urlPrefix string
}

var _ service.ImageStorage = (*LocalImageStorage)(nil)
var _ service.ImageAssetReader = (*LocalImageStorage)(nil)

func NewLocalImageStorage(root, urlPrefix string) (*LocalImageStorage, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("local image storage root is required")
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local image storage root: %w", err)
	}
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create local image storage root: %w", err)
	}
	urlPrefix = strings.TrimSpace(urlPrefix)
	if urlPrefix == "" {
		urlPrefix = "/v1/images/task-assets/"
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	return &LocalImageStorage{root: cleanRoot, urlPrefix: urlPrefix}, nil
}

func (s *LocalImageStorage) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	cleanKey, err := cleanLocalImageKey(key)
	if err != nil {
		return "", err
	}
	target := filepath.Join(s.root, filepath.FromSlash(cleanKey))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create local image dir: %w", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", fmt.Errorf("write local image: %w", err)
	}
	return s.urlPrefix + escapeLocalImageKey(cleanKey), nil
}

func (s *LocalImageStorage) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	cleanKey, err := cleanLocalImageKey(key)
	if err != nil {
		return nil, "", err
	}
	target := filepath.Join(s.root, filepath.FromSlash(cleanKey))
	file, err := os.Open(target)
	if err != nil {
		return nil, "", err
	}
	contentType := mime.TypeByExtension(filepath.Ext(target))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return file, contentType, nil
}

func cleanLocalImageKey(raw string) (string, error) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	for _, part := range strings.Split(raw, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid local image key")
		}
	}
	cleaned := strings.TrimLeft(path.Clean("/"+raw), "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("invalid local image key")
	}
	return cleaned, nil
}

func escapeLocalImageKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
