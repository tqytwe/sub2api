package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAIImageResultKeyPrefix = "openai_image_result:"

type openAIImageResultStore struct {
	rdb *redis.Client
}

func NewOpenAIImageResultStore(rdb *redis.Client) service.OpenAIImageResultStore {
	return &openAIImageResultStore{rdb: rdb}
}

func (s *openAIImageResultStore) Save(
	ctx context.Context,
	record *service.OpenAIImageResultRecord,
	ttl time.Duration,
) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, openAIImageResultKey(record.ID), data, ttl).Err()
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

func openAIImageResultKey(id string) string {
	return openAIImageResultKeyPrefix + strings.TrimSpace(id)
}
