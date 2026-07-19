package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAIImageResultKeyPrefix = "openai_image_result:"

const (
	openAIImageResultCleanupIndexKey   = "openai_image_result:cleanup:index"
	openAIImageResultCleanupRecordsKey = "openai_image_result:cleanup:records"
)

type openAIImageResultStore struct {
	rdb *redis.Client
}

func NewOpenAIImageResultStore(rdb *redis.Client) service.OpenAIImageResultStore {
	return &openAIImageResultStore{rdb: rdb}
}

var _ service.OpenAIImageResultCleanupStore = (*openAIImageResultStore)(nil)

func (s *openAIImageResultStore) Save(
	ctx context.Context,
	record *service.OpenAIImageResultRecord,
	ttl time.Duration,
) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, openAIImageResultKey(record.ID), data, ttl)
	pipe.HSet(ctx, openAIImageResultCleanupRecordsKey, record.ID, data)
	pipe.ZAdd(ctx, openAIImageResultCleanupIndexKey, redis.Z{
		Score:  float64(record.ExpiresAt),
		Member: record.ID,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (s *openAIImageResultStore) Get(ctx context.Context, id string) (*service.OpenAIImageResultRecord, error) {
	data, err := s.rdb.Get(ctx, openAIImageResultKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrOpenAIImageResultNotFound
		}
		return nil, err
	}
	var record service.OpenAIImageResultRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *openAIImageResultStore) ListExpired(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]*service.OpenAIImageResultRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	ids, err := s.rdb.ZRangeByScore(ctx, openAIImageResultCleanupIndexKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(before.UTC().Unix(), 10),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	values, err := s.rdb.HMGet(ctx, openAIImageResultCleanupRecordsKey, ids...).Result()
	if err != nil {
		return nil, err
	}
	records := make([]*service.OpenAIImageResultRecord, 0, len(ids))
	staleIDs := make([]any, 0)
	for index, value := range values {
		raw, ok := value.(string)
		if !ok || strings.TrimSpace(raw) == "" {
			staleIDs = append(staleIDs, ids[index])
			continue
		}
		var record service.OpenAIImageResultRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			return nil, err
		}
		records = append(records, &record)
	}
	if len(staleIDs) > 0 {
		if err := s.rdb.ZRem(ctx, openAIImageResultCleanupIndexKey, staleIDs...).Err(); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (s *openAIImageResultStore) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, openAIImageResultKey(id))
	pipe.HDel(ctx, openAIImageResultCleanupRecordsKey, id)
	pipe.ZRem(ctx, openAIImageResultCleanupIndexKey, id)
	_, err := pipe.Exec(ctx)
	return err
}

func openAIImageResultKey(id string) string {
	return openAIImageResultKeyPrefix + strings.TrimSpace(id)
}
