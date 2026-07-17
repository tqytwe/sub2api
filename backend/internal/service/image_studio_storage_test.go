package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestNormalizeImageStudioPayloadsSniffsRealImageContentType(t *testing.T) {
	pngData := encodeImageStudioStorageTestImage(t, "png")
	jpegData := encodeImageStudioStorageTestImage(t, "jpeg")
	webpData, err := base64.StdEncoding.DecodeString(
		"UklGRjwAAABXRUJQVlA4IDAAAADQAQCdASoCAAIAAkA4JaQAA3AA/v89WAAAAA==",
	)
	require.NoError(t, err)

	images, err := NormalizeImageStudioPayloads(context.Background(), []ImageStudioImagePayload{
		{Data: pngData, ContentType: "image/jpeg"},
		{Data: jpegData, ContentType: "image/png"},
		{Data: webpData},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"image/png", "image/jpeg", "image/webp"}, []string{
		images[0].ContentType,
		images[1].ContentType,
		images[2].ContentType,
	})
}

func TestNormalizeImageStudioPayloadsRejectsNonImageBytes(t *testing.T) {
	_, err := NormalizeImageStudioPayloads(context.Background(), []ImageStudioImagePayload{{
		Data:        []byte("not an image"),
		ContentType: "image/png",
	}})

	require.Error(t, err)
}

func TestFetchImageStudioRemoteURLRejectsPrivateAndInsecureTargets(t *testing.T) {
	testCases := []string{
		"http://127.0.0.1:1/private.png",
		"https://localhost/private.png",
		"http://example.com/image.png",
	}
	for _, rawURL := range testCases {
		t.Run(rawURL, func(t *testing.T) {
			_, _, err := FetchImageStudioRemoteURL(context.Background(), rawURL)
			require.Error(t, err)
			require.Contains(t, err.Error(), "image studio remote url is not allowed")
		})
	}
}

func TestCloneImageStudioHTTPTransportFallsBackForCustomRoundTripper(t *testing.T) {
	transport := cloneImageStudioHTTPTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, nil
	}))

	require.NotNil(t, transport)
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
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	_ = store
}

func TestImageStudioAssetStoreUsesPrivateFilePermissions(t *testing.T) {
	dir := t.TempDir()
	store := NewImageStudioAssetStore(dir)
	key, err := store.Save(42, "asset-private", "image/png", []byte("private"))
	require.NoError(t, err)

	userDir, err := os.Stat(filepath.Join(dir, imageStudioStorageDir, "42"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), userDir.Mode().Perm())
	file, err := os.Stat(filepath.Join(dir, imageStudioStorageDir, filepath.FromSlash(key)))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), file.Mode().Perm())
}

func TestImageStudioAssetStoreListsOnlyObjectsOlderThanCutoff(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	oldKey, err := store.Save(42, "old-object", "image/png", []byte("old"))
	require.NoError(t, err)
	newKey, err := store.Save(42, "new-object", "image/png", []byte("new"))
	require.NoError(t, err)
	oldPath, err := store.resolve(oldKey)
	require.NoError(t, err)
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, oldTime, oldTime))

	keys, err := store.ListStorageKeysBefore(time.Now().Add(-time.Hour), 100)

	require.NoError(t, err)
	require.Equal(t, []string{oldKey}, keys)
	require.NotContains(t, keys, newKey)
}

func encodeImageStudioStorageTestImage(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 240, G: 20, B: 30, A: 255})
	var out bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	default:
		err = png.Encode(&out, img)
	}
	require.NoError(t, err)
	return out.Bytes()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
