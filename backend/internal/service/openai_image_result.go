package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOpenAIImageResultNotFound           = errors.New("image result not found")
	ErrOpenAIImageResultStorageUnavailable = errors.New("image result storage is unavailable")
)

type OpenAIImageResultAsset struct {
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
}

type OpenAIImageResultRecord struct {
	ID        string                   `json:"id"`
	UserID    int64                    `json:"user_id"`
	APIKeyID  int64                    `json:"api_key_id"`
	Assets    []OpenAIImageResultAsset `json:"assets"`
	CreatedAt int64                    `json:"created_at"`
	ExpiresAt int64                    `json:"expires_at"`
}

type OpenAIImageResultStore interface {
	Save(ctx context.Context, record *OpenAIImageResultRecord, ttl time.Duration) error
	Get(ctx context.Context, id string) (*OpenAIImageResultRecord, error)
}

type OpenAIImageResultService struct {
	store   OpenAIImageResultStore
	storage ImageStorage
	reader  ImageAssetReader
	prefix  string
	ttl     time.Duration
	now     func() time.Time
}

func NewOpenAIImageResultService(
	store OpenAIImageResultStore,
	storage ImageStorage,
	reader ImageAssetReader,
	prefix string,
	ttl time.Duration,
) *OpenAIImageResultService {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &OpenAIImageResultService{
		store:   store,
		storage: storage,
		reader:  reader,
		prefix:  strings.TrimSpace(prefix),
		ttl:     ttl,
		now:     time.Now,
	}
}

func (s *OpenAIImageResultService) Enabled() bool {
	return s != nil && s.store != nil && s.storage != nil && s.reader != nil
}

func (s *OpenAIImageResultService) Rewrite(
	ctx context.Context,
	owner ImageTaskOwner,
	requestPath string,
	result json.RawMessage,
) (json.RawMessage, error) {
	if !s.Enabled() {
		return nil, ErrOpenAIImageResultStorageUnavailable
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("parse image response: %w", err)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(response["data"], &items); err != nil {
		return nil, fmt.Errorf("parse image response data: %w", err)
	}
	if len(items) == 0 {
		return nil, errors.New("image response data is empty")
	}

	now := s.now().UTC()
	resultID := "imgres_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	expiresAt := now.Add(s.ttl).Unix()
	uploader := NewImageResultUploader(s.storage, "", 0, nil)
	assets := make([]OpenAIImageResultAsset, 0, len(items))
	baseURL := "/images/results/"
	if strings.HasPrefix(strings.TrimSpace(requestPath), "/v1/") {
		baseURL = "/v1/images/results/"
	}

	for index, item := range items {
		data, contentType, err := uploader.fetchImageBytes(ctx, item)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", index, err)
		}
		key := s.resultKey(resultID, index, contentType)
		if _, err := s.storage.Save(ctx, key, contentType, data); err != nil {
			return nil, fmt.Errorf("image %d: store image result: %w", index, err)
		}
		assets = append(assets, OpenAIImageResultAsset{Key: key, ContentType: contentType})
		urlRaw, _ := json.Marshal(fmt.Sprintf("%s%s/%d", baseURL, resultID, index))
		expiresRaw, _ := json.Marshal(expiresAt)
		item["url"] = urlRaw
		item["expires_at"] = expiresRaw
		delete(item, "b64_json")
		items[index] = item
	}

	record := &OpenAIImageResultRecord{
		ID:        resultID,
		UserID:    owner.UserID,
		APIKeyID:  owner.APIKeyID,
		Assets:    assets,
		CreatedAt: now.Unix(),
		ExpiresAt: expiresAt,
	}
	if err := s.store.Save(ctx, record, s.ttl); err != nil {
		return nil, ErrOpenAIImageResultStorageUnavailable
	}
	dataRaw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	response["data"] = dataRaw
	response["result_id"], _ = json.Marshal(resultID)
	response["expires_at"], _ = json.Marshal(expiresAt)
	rewritten, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return rewritten, nil
}

func (s *OpenAIImageResultService) Open(
	ctx context.Context,
	owner ImageTaskOwner,
	resultID string,
	index int,
) (io.ReadCloser, string, error) {
	if !s.Enabled() || index < 0 {
		return nil, "", ErrOpenAIImageResultNotFound
	}
	record, err := s.store.Get(ctx, strings.TrimSpace(resultID))
	if err != nil {
		return nil, "", ErrOpenAIImageResultNotFound
	}
	if record.UserID != owner.UserID || record.APIKeyID != owner.APIKeyID ||
		record.ExpiresAt <= s.now().UTC().Unix() || index >= len(record.Assets) {
		return nil, "", ErrOpenAIImageResultNotFound
	}
	reader, contentType, err := s.reader.Open(ctx, record.Assets[index].Key)
	if err != nil {
		return nil, "", ErrOpenAIImageResultNotFound
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = record.Assets[index].ContentType
	}
	return reader, contentType, nil
}

func (s *OpenAIImageResultService) resultKey(resultID string, index int, contentType string) string {
	prefix := strings.TrimRight(s.prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return fmt.Sprintf("%sresults/%s-%d%s", prefix, resultID, index, extensionForContentType(contentType))
}
