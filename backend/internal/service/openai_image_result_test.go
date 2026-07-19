package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIImageResultMemoryStore struct {
	mu        sync.Mutex
	records   map[string]*OpenAIImageResultRecord
	saveErr   error
	deleteErr error
	deleted   []string
}

func (s *openAIImageResultMemoryStore) Save(_ context.Context, record *OpenAIImageResultRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	cloned := *record
	cloned.Assets = append([]OpenAIImageResultAsset(nil), record.Assets...)
	s.records[record.ID] = &cloned
	return nil
}

func (s *openAIImageResultMemoryStore) Get(_ context.Context, id string) (*OpenAIImageResultRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[id]
	if record == nil {
		return nil, ErrOpenAIImageResultNotFound
	}
	cloned := *record
	cloned.Assets = append([]OpenAIImageResultAsset(nil), record.Assets...)
	return &cloned, nil
}

func (s *openAIImageResultMemoryStore) ListExpired(_ context.Context, before time.Time, limit int) ([]*OpenAIImageResultRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]*OpenAIImageResultRecord, 0)
	for _, record := range s.records {
		if record.ExpiresAt > before.Unix() {
			continue
		}
		cloned := *record
		cloned.Assets = append([]OpenAIImageResultAsset(nil), record.Assets...)
		records = append(records, &cloned)
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records, nil
}

func (s *openAIImageResultMemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.records, id)
	s.deleted = append(s.deleted, id)
	return nil
}

type openAIImageResultMemoryAssets struct {
	mu          sync.Mutex
	data        map[string][]byte
	contentType map[string]string
	saveCalls   int
	failSaveAt  int
	deleted     []string
}

func (s *openAIImageResultMemoryAssets) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCalls++
	if s.failSaveAt > 0 && s.saveCalls == s.failSaveAt {
		return "", errors.New("asset storage unavailable")
	}
	s.data[key] = append([]byte(nil), data...)
	s.contentType[key] = contentType
	return "https://storage.invalid/" + key, nil
}

func (s *openAIImageResultMemoryAssets) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.data[key]
	if !ok {
		return nil, "", ErrOpenAIImageResultNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), s.contentType[key], nil
}

func (s *openAIImageResultMemoryAssets) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	delete(s.contentType, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func TestOpenAIImageResultServiceRewriteAndEnforceAPIKeyOwnership(t *testing.T) {
	store := &openAIImageResultMemoryStore{records: make(map[string]*OpenAIImageResultRecord)}
	assets := &openAIImageResultMemoryAssets{
		data:        make(map[string][]byte),
		contentType: make(map[string]string),
	}
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	svc := NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	svc.now = func() time.Time { return now }
	encoded := encodeOpenAIImageTestPNG(t, 1254, 1254)
	body, err := json.Marshal(map[string]any{
		"created": 1,
		"data": []map[string]any{{
			"b64_json": encoded,
			"size":     "1024x1024",
		}},
	})
	require.NoError(t, err)

	rewritten, err := svc.Rewrite(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 9},
		"/v1/images/generations",
		body,
	)

	require.NoError(t, err)
	resultURL := gjson.GetBytes(rewritten, "data.0.url").String()
	require.True(t, strings.HasPrefix(resultURL, "/v1/images/results/imgres_"))
	require.True(t, strings.HasSuffix(resultURL, "/0"))
	require.False(t, gjson.GetBytes(rewritten, "data.0.b64_json").Exists())
	require.Equal(t, "1254x1254", gjson.GetBytes(rewritten, "data.0.size").String())
	require.Equal(t, "1254x1254", gjson.GetBytes(rewritten, "size").String())
	require.Equal(t, now.Add(time.Hour).Unix(), gjson.GetBytes(rewritten, "expires_at").Int())
	require.Equal(t, now.Add(time.Hour).Unix(), gjson.GetBytes(rewritten, "data.0.expires_at").Int())

	resultID := strings.Split(strings.TrimPrefix(resultURL, "/v1/images/results/"), "/")[0]
	reader, contentType, err := svc.Open(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 9},
		resultID,
		0,
	)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)
	require.Equal(t, encoded, base64.StdEncoding.EncodeToString(got))

	_, _, err = svc.Open(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 10},
		resultID,
		0,
	)
	require.ErrorIs(t, err, ErrOpenAIImageResultNotFound)

	now = now.Add(2 * time.Hour)
	_, _, err = svc.Open(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 9},
		resultID,
		0,
	)
	require.ErrorIs(t, err, ErrOpenAIImageResultNotFound)
}

func TestOpenAIImageResultServiceRewriteRollsBackStoredAssetsOnPartialUploadFailure(t *testing.T) {
	store := &openAIImageResultMemoryStore{records: make(map[string]*OpenAIImageResultRecord)}
	assets := &openAIImageResultMemoryAssets{
		data:        make(map[string][]byte),
		contentType: make(map[string]string),
		failSaveAt:  2,
	}
	svc := NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	body, err := json.Marshal(map[string]any{
		"data": []map[string]any{
			{"b64_json": encodeOpenAIImageTestPNG(t, 16, 16)},
			{"b64_json": encodeOpenAIImageTestPNG(t, 32, 32)},
		},
	})
	require.NoError(t, err)

	_, err = svc.Rewrite(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 9},
		"/v1/images/generations",
		body,
	)

	require.ErrorContains(t, err, "store image result")
	require.Len(t, assets.deleted, 1)
	require.Empty(t, assets.data)
	require.Empty(t, store.records)
}

func TestOpenAIImageResultServiceRewriteRollsBackStoredAssetsWhenMetadataSaveFails(t *testing.T) {
	store := &openAIImageResultMemoryStore{
		records: make(map[string]*OpenAIImageResultRecord),
		saveErr: errors.New("redis unavailable"),
	}
	assets := &openAIImageResultMemoryAssets{
		data:        make(map[string][]byte),
		contentType: make(map[string]string),
	}
	svc := NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	body, err := json.Marshal(map[string]any{
		"data": []map[string]any{{
			"b64_json": encodeOpenAIImageTestPNG(t, 16, 16),
		}},
	})
	require.NoError(t, err)

	_, err = svc.Rewrite(
		context.Background(),
		ImageTaskOwner{UserID: 7, APIKeyID: 9},
		"/v1/images/generations",
		body,
	)

	require.ErrorIs(t, err, ErrOpenAIImageResultStorageUnavailable)
	require.Len(t, assets.deleted, 1)
	require.Empty(t, assets.data)
	require.Empty(t, store.records)
}

func TestOpenAIImageResultServiceCleanupExpiredDeletesAssetsAndMetadata(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	store := &openAIImageResultMemoryStore{records: map[string]*OpenAIImageResultRecord{
		"imgres_expired": {
			ID:        "imgres_expired",
			ExpiresAt: now.Add(-time.Minute).Unix(),
			Assets: []OpenAIImageResultAsset{{
				Key:         "images/results/imgres_expired-0.png",
				ContentType: "image/png",
			}},
		},
		"imgres_future": {
			ID:        "imgres_future",
			ExpiresAt: now.Add(time.Hour).Unix(),
			Assets: []OpenAIImageResultAsset{{
				Key:         "images/results/imgres_future-0.png",
				ContentType: "image/png",
			}},
		},
	}}
	assets := &openAIImageResultMemoryAssets{
		data: map[string][]byte{
			"images/results/imgres_expired-0.png": []byte("expired"),
			"images/results/imgres_future-0.png":  []byte("future"),
		},
		contentType: map[string]string{
			"images/results/imgres_expired-0.png": "image/png",
			"images/results/imgres_future-0.png":  "image/png",
		},
	}
	svc := NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	svc.now = func() time.Time { return now }

	cleaned, err := svc.CleanupExpiredOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, cleaned)
	require.Equal(t, []string{"images/results/imgres_expired-0.png"}, assets.deleted)
	require.Equal(t, []string{"imgres_expired"}, store.deleted)
	require.NotContains(t, store.records, "imgres_expired")
	require.Contains(t, store.records, "imgres_future")
	require.Contains(t, assets.data, "images/results/imgres_future-0.png")
}

func TestOpenAIImageResultServiceCleanupExpiredKeepsMetadataWhenAssetDeleteFails(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	store := &openAIImageResultMemoryStore{records: map[string]*OpenAIImageResultRecord{
		"imgres_retry": {
			ID:        "imgres_retry",
			ExpiresAt: now.Add(-time.Minute).Unix(),
			Assets:    []OpenAIImageResultAsset{{Key: "images/results/retry.png"}},
		},
	}}
	assets := &failingImageResultDeleteAssets{
		openAIImageResultMemoryAssets: openAIImageResultMemoryAssets{
			data:        map[string][]byte{"images/results/retry.png": []byte("retry")},
			contentType: map[string]string{"images/results/retry.png": "image/png"},
		},
		deleteErr: errors.New("storage delete failed"),
	}
	svc := NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	svc.now = func() time.Time { return now }

	cleaned, err := svc.CleanupExpiredOnce(context.Background())

	require.Zero(t, cleaned)
	require.ErrorContains(t, err, "storage delete failed")
	require.Contains(t, store.records, "imgres_retry")
	require.Empty(t, store.deleted)
}

type failingImageResultDeleteAssets struct {
	openAIImageResultMemoryAssets
	deleteErr error
}

func (s *failingImageResultDeleteAssets) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	return s.deleteErr
}
