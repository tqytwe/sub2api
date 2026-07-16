package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const imageStudioStorageDir = "image-studio"

// ImageStudioAssetStore persists generated images on local disk.
type ImageStudioAssetStore struct {
	root string
}

func NewImageStudioAssetStore(dataDir string) *ImageStudioAssetStore {
	root := filepath.Join(strings.TrimSpace(dataDir), imageStudioStorageDir)
	_ = os.MkdirAll(root, 0o755)
	return &ImageStudioAssetStore{root: root}
}

func (s *ImageStudioAssetStore) Save(userID int64, assetID, contentType string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty image data")
	}
	ext := extensionForContentType(contentType)
	rel := filepath.Join(fmt.Sprintf("%d", userID), assetID+ext)
	abs := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
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
