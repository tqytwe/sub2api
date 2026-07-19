package repository

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalImageStorageSaveAndOpen(t *testing.T) {
	store, err := NewLocalImageStorage(t.TempDir(), "/v1/images/task-assets/")
	require.NoError(t, err)

	gotURL, err := store.Save(context.Background(), "images/imgtask_abc-0.png", "image/png", []byte("png-bytes"))
	require.NoError(t, err)
	require.Equal(t, "/v1/images/task-assets/images/imgtask_abc-0.png", gotURL)

	reader, contentType, err := store.Open(context.Background(), "images/imgtask_abc-0.png")
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)
	require.Equal(t, []byte("png-bytes"), data)
}

func TestLocalImageStorageRejectsTraversalKeys(t *testing.T) {
	store, err := NewLocalImageStorage(t.TempDir(), "/v1/images/task-assets/")
	require.NoError(t, err)

	_, err = store.Save(context.Background(), "../../secret.png", "image/png", []byte("x"))
	require.Error(t, err)
	_, _, err = store.Open(context.Background(), `..\secret.png`)
	require.Error(t, err)
}

func TestLocalImageStorageDeleteRemovesStoredAsset(t *testing.T) {
	store, err := NewLocalImageStorage(t.TempDir(), "/v1/images/task-assets/")
	require.NoError(t, err)
	key := "images/results/imgres_delete-0.png"
	_, err = store.Save(context.Background(), key, "image/png", []byte("png-bytes"))
	require.NoError(t, err)

	require.NoError(t, store.Delete(context.Background(), key))
	_, _, err = store.Open(context.Background(), key)
	require.Error(t, err)
	require.NoError(t, store.Delete(context.Background(), key), "delete must be idempotent")
}
