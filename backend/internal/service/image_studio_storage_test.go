package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioAssetStoreSaveReadDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewImageStudioAssetStore(dir)
	data := []byte("fake-png")
	key, err := store.Save(42, "asset-1", "image/png", data)
	require.NoError(t, err)
	require.Contains(t, key, "42/asset-1.png")

	got, err := store.Read(key)
	require.NoError(t, err)
	require.Equal(t, data, got)

	require.NoError(t, store.Delete(key))
	_, err = store.Read(key)
	require.Error(t, err)
}

func TestImageStudioAssetStoreRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	store := NewImageStudioAssetStore(dir)
	_, err := store.Read("../secret.png")
	require.Error(t, err)
}

func TestDecodeImageStudioDataURL(t *testing.T) {
	raw := "data:image/png;base64,aGVsbG8="
	data, ct, err := DecodeImageStudioDataURL(raw)
	require.NoError(t, err)
	require.Equal(t, "image/png", ct)
	require.Equal(t, []byte("hello"), data)
}

func TestExtensionForContentType(t *testing.T) {
	require.Equal(t, ".jpg", extensionForContentType("image/jpeg"))
	require.Equal(t, ".webp", extensionForContentType("image/webp"))
	require.Equal(t, ".png", extensionForContentType("image/png"))
}

func TestImageStudioAssetStoreRootCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	store := NewImageStudioAssetStore(dir)
	info, err := os.Stat(filepath.Join(dir, imageStudioStorageDir))
	require.NoError(t, err)
	require.True(t, info.IsDir())
	_ = store
}
