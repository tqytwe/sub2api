package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIImageResultMemoryStore struct {
	mu      sync.Mutex
	records map[string]*OpenAIImageResultRecord
}

func (s *openAIImageResultMemoryStore) Save(_ context.Context, record *OpenAIImageResultRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

type openAIImageResultMemoryAssets struct {
	mu          sync.Mutex
	data        map[string][]byte
	contentType map[string]string
}

func (s *openAIImageResultMemoryAssets) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
			"size":     "1254x1254",
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
