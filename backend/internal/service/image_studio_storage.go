package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const imageStudioStorageDir = "image-studio"

// ImageStudioAssetStore persists generated images on local disk.
type ImageStudioAssetStore struct {
	root string
}

func NewImageStudioAssetStore(dataDir string) *ImageStudioAssetStore {
	root := filepath.Join(strings.TrimSpace(dataDir), imageStudioStorageDir)
	if err := os.MkdirAll(root, 0o700); err == nil {
		_ = os.Chmod(root, 0o700)
	}
	return &ImageStudioAssetStore{root: root}
}

func (s *ImageStudioAssetStore) StorageHealth(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(s.root) == "" {
		return errors.New("image studio asset storage is unavailable")
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return err
	}
	probe, err := os.CreateTemp(s.root, ".image-studio-health-*")
	if err != nil {
		return err
	}
	probePath := probe.Name()
	defer func() {
		_ = probe.Close()
		_ = os.Remove(probePath)
	}()
	if err := probe.Chmod(0o600); err != nil {
		return err
	}
	if _, err := probe.Write([]byte("ok")); err != nil {
		return err
	}
	if err := probe.Sync(); err != nil {
		return err
	}
	return probe.Close()
}

func (s *ImageStudioAssetStore) Save(userID int64, assetID, contentType string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty image data")
	}
	ext := extensionForContentType(contentType)
	rel := filepath.Join(fmt.Sprintf("%d", userID), assetID+ext)
	abs := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Dir(abs), 0o700); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(abs), ".image-studio-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		return "", err
	}
	if err := temp.Sync(); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, abs); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (s *ImageStudioAssetStore) Read(storageKey string) ([]byte, error) {
	abs, err := s.resolve(storageKey)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func (s *ImageStudioAssetStore) Delete(storageKey string) error {
	if strings.TrimSpace(storageKey) == "" {
		return nil
	}
	abs, err := s.resolve(storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *ImageStudioAssetStore) ListStorageKeysBefore(before time.Time, limit int) ([]string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	keys := make([]string, 0, limit)
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || len(keys) >= limit {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(before) {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	return keys, err
}

func (s *ImageStudioAssetStore) resolve(storageKey string) (string, error) {
	key := filepath.Clean(filepath.FromSlash(strings.TrimSpace(storageKey)))
	if key == "." || strings.HasPrefix(key, "..") {
		return "", fmt.Errorf("invalid storage key")
	}
	abs := filepath.Join(s.root, key)
	cleanRoot := filepath.Clean(s.root)
	if !strings.HasPrefix(abs, cleanRoot+string(os.PathSeparator)) && abs != cleanRoot {
		return "", fmt.Errorf("invalid storage key path")
	}
	return abs, nil
}
