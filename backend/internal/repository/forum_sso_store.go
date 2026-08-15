package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type forumSSORedisStore struct {
	client *redis.Client
}

func NewForumSSOStore(client *redis.Client) service.ForumSSOStore {
	if client == nil {
		return nil
	}
	return &forumSSORedisStore{client: client}
}

func (s *forumSSORedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *forumSSORedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrForumSSOStoreKeyNotFound
	}
	return raw, err
}

func (s *forumSSORedisStore) GetDel(ctx context.Context, key string) ([]byte, error) {
	raw, err := s.client.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrForumSSOStoreKeyNotFound
	}
	return raw, err
}

func (s *forumSSORedisStore) Del(ctx context.Context, keys ...string) error {
	return s.client.Del(ctx, keys...).Err()
}

func (s *forumSSORedisStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, key, ttl).Err()
}

func (s *forumSSORedisStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return s.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

func (s *forumSSORedisStore) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return s.client.ZRange(ctx, key, start, stop).Result()
}

func (s *forumSSORedisStore) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return s.client.ZRevRange(ctx, key, start, stop).Result()
}

func (s *forumSSORedisStore) ZRangeByScore(ctx context.Context, key, min, max string) ([]string, error) {
	return s.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: min, Max: max}).Result()
}

func (s *forumSSORedisStore) ZRem(ctx context.Context, key string, members ...string) error {
	args := make([]any, len(members))
	for i, member := range members {
		args[i] = member
	}
	return s.client.ZRem(ctx, key, args...).Err()
}
