package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
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

type OpenAIImageResultCleanupStore interface {
	ListExpired(ctx context.Context, before time.Time, limit int) ([]*OpenAIImageResultRecord, error)
	Delete(ctx context.Context, id string) error
}

type OpenAIImageResultService struct {
	store           OpenAIImageResultStore
	cleanupStore    OpenAIImageResultCleanupStore
	storage         ImageStorage
	reader          ImageAssetReader
	uploader        *ImageResultUploader
	prefix          string
	ttl             time.Duration
	cleanupInterval time.Duration
	cleanupBatch    int
	now             func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
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
	cleanupStore, _ := store.(OpenAIImageResultCleanupStore)
	return &OpenAIImageResultService{
		store:           store,
		cleanupStore:    cleanupStore,
		storage:         storage,
		reader:          reader,
		uploader:        NewImageResultUploader(storage, "", 0, nil),
		prefix:          strings.TrimSpace(prefix),
		ttl:             ttl,
		cleanupInterval: time.Minute,
		cleanupBatch:    100,
		now:             time.Now,
	}
}

func (s *OpenAIImageResultService) Enabled() bool {
	return s != nil && s.store != nil && s.storage != nil && s.reader != nil
}

func (s *OpenAIImageResultService) ConfigureCleanup(interval time.Duration, batchSize int) {
	if s == nil {
		return
	}
	if interval > 0 {
		s.cleanupInterval = interval
	}
	if batchSize > 0 {
		s.cleanupBatch = batchSize
	}
}

func (s *OpenAIImageResultService) Start() {
	if s == nil || !s.Enabled() || s.cleanupStore == nil {
		return
	}
	if _, ok := s.storage.(ImageAssetDeleter); !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.runCleanup(ctx, s.done)
}

func (s *OpenAIImageResultService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *OpenAIImageResultService) CleanupExpiredOnce(ctx context.Context) (int, error) {
	if s == nil || s.cleanupStore == nil {
		return 0, nil
	}
	if _, ok := s.storage.(ImageAssetDeleter); !ok {
		return 0, ErrOpenAIImageResultStorageUnavailable
	}
	limit := s.cleanupBatch
	if limit <= 0 {
		limit = 100
	}
	records, err := s.cleanupStore.ListExpired(ctx, s.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	var joined error
	for _, record := range records {
		if record == nil || strings.TrimSpace(record.ID) == "" {
			continue
		}
		keys := make([]string, 0, len(record.Assets))
		for _, asset := range record.Assets {
			if key := strings.TrimSpace(asset.Key); key != "" {
				keys = append(keys, key)
			}
		}
		if err := deleteImageAssets(ctx, s.storage, keys); err != nil {
			joined = errors.Join(joined, fmt.Errorf("delete image result %s assets: %w", record.ID, err))
			continue
		}
		if err := s.cleanupStore.Delete(ctx, record.ID); err != nil {
			joined = errors.Join(joined, fmt.Errorf("delete image result %s metadata: %w", record.ID, err))
			continue
		}
		cleaned++
	}
	return cleaned, joined
}

func (s *OpenAIImageResultService) runCleanup(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	run := func() {
		if _, err := s.CleanupExpiredOnce(ctx); err != nil && ctx.Err() == nil {
			logger.L().Warn("openai_image_result.cleanup_failed", zap.Error(err))
		}
	}
	run()
	interval := s.cleanupInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
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
	uploader := s.uploader
	if uploader == nil {
		uploader = NewImageResultUploader(s.storage, "", 0, nil)
	}
	assets := make([]OpenAIImageResultAsset, 0, len(items))
	committed := false
	defer func() {
		if committed {
			return
		}
		keys := make([]string, 0, len(assets))
		for _, asset := range assets {
			keys = append(keys, asset.Key)
		}
		_ = deleteImageAssets(context.WithoutCancel(ctx), s.storage, keys)
	}()
	baseURL := "/images/results/"
	firstActualSize := ""
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
		if actualSize := detectOpenAIImageBytesSize(data); actualSize != "" {
			item["size"], _ = json.Marshal(actualSize)
			if firstActualSize == "" {
				firstActualSize = actualSize
			}
		}
		assets = append(assets, OpenAIImageResultAsset{Key: key, ContentType: contentType})
		urlRaw, _ := json.Marshal(fmt.Sprintf("%s%s/%d", baseURL, resultID, index))
		expiresRaw, _ := json.Marshal(expiresAt)
		item["url"] = urlRaw
		item["expires_at"] = expiresRaw
		delete(item, "b64_json")
		items[index] = item
	}

	dataRaw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	response["data"] = dataRaw
	response["result_id"], _ = json.Marshal(resultID)
	response["expires_at"], _ = json.Marshal(expiresAt)
	if firstActualSize != "" {
		response["size"], _ = json.Marshal(firstActualSize)
	}
	rewritten, err := json.Marshal(response)
	if err != nil {
		return nil, err
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
	committed = true
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
