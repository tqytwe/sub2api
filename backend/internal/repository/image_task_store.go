package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const imageTaskKeyPrefix = "image_task:"

var imageTaskHeartbeatScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
  return -1
end
local task = cjson.decode(raw)
if task["status"] ~= ARGV[1] then
  return 0
end
local ttl = redis.call("PTTL", KEYS[1])
task["heartbeat_at"] = tonumber(ARGV[2])
redis.call("SET", KEYS[1], cjson.encode(task))
if ttl > 0 then
  redis.call("PEXPIRE", KEYS[1], ttl)
end
return 1
`)

var imageTaskSaveIfStatusScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
  return -1
end
local task = cjson.decode(raw)
if task["status"] ~= ARGV[1] then
  return 0
end
redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
return 1
`)

type imageTaskStore struct {
	rdb *redis.Client
}

func NewImageTaskStore(rdb *redis.Client) service.ImageTaskStore {
	return &imageTaskStore{rdb: rdb}
}

func (s *imageTaskStore) Save(ctx context.Context, task *service.ImageTaskRecord, ttl time.Duration) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, imageTaskKey(task.ID), data, ttl).Err()
}

func (s *imageTaskStore) SaveIfStatus(
	ctx context.Context,
	task *service.ImageTaskRecord,
	expectedStatus string,
	ttl time.Duration,
) (bool, error) {
	if task == nil || ttl <= 0 {
		return false, service.ErrImageTaskUnavailable
	}
	data, err := json.Marshal(task)
	if err != nil {
		return false, err
	}
	result, err := imageTaskSaveIfStatusScript.Run(
		ctx,
		s.rdb,
		[]string{imageTaskKey(task.ID)},
		expectedStatus,
		data,
		ttl.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	if result < 0 {
		return false, service.ErrImageTaskNotFound
	}
	return result == 1, nil
}

func (s *imageTaskStore) Get(ctx context.Context, id string) (*service.ImageTaskRecord, error) {
	data, err := s.rdb.Get(ctx, imageTaskKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrImageTaskNotFound
		}
		return nil, err
	}
	var task service.ImageTaskRecord
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *imageTaskStore) TouchHeartbeat(ctx context.Context, id string, heartbeatAt time.Time) error {
	result, err := imageTaskHeartbeatScript.Run(
		ctx,
		s.rdb,
		[]string{imageTaskKey(id)},
		service.ImageTaskStatusProcessing,
		heartbeatAt.UTC().Unix(),
	).Int()
	if err != nil {
		return err
	}
	if result < 0 {
		return service.ErrImageTaskNotFound
	}
	return nil
}

func imageTaskKey(id string) string {
	return imageTaskKeyPrefix + strings.TrimSpace(id)
}
