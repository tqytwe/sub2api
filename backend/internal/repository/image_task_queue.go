package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const imageTaskQueuePollInterval = 250 * time.Millisecond

var imageTaskSubmitScript = redis.NewScript(`
if ARGV[4] ~= "" then
  local existing_hash = redis.call("HGET", KEYS[3], "request_hash")
  if existing_hash then
    local existing_task = redis.call("HGET", KEYS[3], "task_id")
    if not existing_task or redis.call("EXISTS", ARGV[5] .. existing_task) == 0 then
      redis.call("DEL", KEYS[3])
      if existing_task then
        redis.call("LREM", KEYS[2], 0, existing_task)
        redis.call("ZREM", KEYS[4], existing_task)
      end
    else
      if existing_hash ~= ARGV[4] then
        return {-1, existing_task}
      end
      return {0, existing_task}
    end
  end
end
redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
redis.call("LPUSH", KEYS[2], ARGV[1])
if ARGV[4] ~= "" then
  redis.call("HSET", KEYS[3], "request_hash", ARGV[4], "task_id", ARGV[1])
  redis.call("PEXPIRE", KEYS[3], ARGV[3])
end
return {1, ARGV[1]}
`)

var imageTaskReserveScript = redis.NewScript(`
local task = redis.call("RPOP", KEYS[1])
if not task then
  return nil
end
redis.call("ZADD", KEYS[2], ARGV[1], task)
return task
`)

var imageTaskRecoverScript = redis.NewScript(`
local tasks = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, ARGV[2])
for _, task in ipairs(tasks) do
  redis.call("ZREM", KEYS[1], task)
  redis.call("LPUSH", KEYS[2], task)
end
return #tasks
`)

var imageTaskQueueHeartbeatScript = redis.NewScript(`
if not redis.call("ZSCORE", KEYS[1], ARGV[1]) then
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[2], ARGV[1])
return 1
`)

var imageTaskReleaseLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

var imageTaskRefreshLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

var imageTaskSaveIfStatusWithLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
  return -2
end
local raw = redis.call("GET", KEYS[2])
if not raw then
  return -1
end
local task = cjson.decode(raw)
if task["status"] ~= ARGV[2] then
  return 0
end
redis.call("SET", KEYS[2], ARGV[3], "PX", ARGV[4])
return 1
`)

var imageTaskAckWithLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call("ZREM", KEYS[2], ARGV[2])
redis.call("DEL", KEYS[1])
return 1
`)

var imageTaskRequeueWithLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call("ZREM", KEYS[2], ARGV[2])
redis.call("LPUSH", KEYS[3], ARGV[2])
redis.call("DEL", KEYS[1])
return 1
`)

type imageTaskQueue struct {
	rdb               *redis.Client
	readyKey          string
	activeKey         string
	idempotencyPrefix string
	lockPrefix        string
}

func NewImageTaskQueue(rdb *redis.Client, cfg *config.Config) service.ImageTaskQueue {
	q := &imageTaskQueue{rdb: rdb}
	if cfg != nil {
		q.readyKey = cfg.ImageAsync.QueueReadyKey
		q.activeKey = cfg.ImageAsync.QueueActiveKey
		q.idempotencyPrefix = cfg.ImageAsync.IdempotencyKeyPrefix
		q.lockPrefix = cfg.ImageAsync.JobLockKeyPrefix
	}
	if q.readyKey == "" {
		q.readyKey = "image_task:queue:ready"
	}
	if q.activeKey == "" {
		q.activeKey = "image_task:queue:active"
	}
	if q.idempotencyPrefix == "" {
		q.idempotencyPrefix = "image_task:idem:"
	}
	if q.lockPrefix == "" {
		q.lockPrefix = "image_task:lock:"
	}
	return q
}

func (q *imageTaskQueue) Submit(ctx context.Context, task *service.ImageTaskRecord, ttl time.Duration, idempotencyKey string) (string, bool, error) {
	if q == nil || q.rdb == nil || task == nil || !validImageTaskID(task.ID) || ttl <= 0 {
		return "", false, service.ErrImageTaskUnavailable
	}
	body, err := json.Marshal(task)
	if err != nil {
		return "", false, err
	}
	idemRedisKey := q.idempotencyPrefix + strings.TrimSpace(idempotencyKey)
	if strings.TrimSpace(idempotencyKey) == "" {
		idemRedisKey = q.idempotencyPrefix + "_unused"
	}
	raw, err := imageTaskSubmitScript.Run(
		ctx,
		q.rdb,
		[]string{imageTaskKey(task.ID), q.readyKey, idemRedisKey, q.activeKey},
		task.ID,
		body,
		ttl.Milliseconds(),
		func() string {
			if strings.TrimSpace(idempotencyKey) == "" {
				return ""
			}
			return task.RequestHash
		}(),
		imageTaskKeyPrefix,
	).Result()
	if err != nil {
		return "", false, err
	}
	values, ok := raw.([]any)
	if !ok || len(values) != 2 {
		return "", false, service.ErrImageTaskUnavailable
	}
	status, ok := values[0].(int64)
	if !ok {
		return "", false, service.ErrImageTaskUnavailable
	}
	taskID, _ := values[1].(string)
	switch status {
	case -1:
		return taskID, false, service.ErrImageTaskIdempotency
	case 0:
		return taskID, false, nil
	case 1:
		return taskID, true, nil
	default:
		return "", false, service.ErrImageTaskUnavailable
	}
}

func (q *imageTaskQueue) Reserve(ctx context.Context, blockTimeout time.Duration) (string, error) {
	deadline := time.Now().Add(blockTimeout)
	for {
		raw, err := imageTaskReserveScript.Run(ctx, q.rdb, []string{q.readyKey, q.activeKey}, time.Now().UnixMilli()).Result()
		if err == nil {
			taskID, ok := raw.(string)
			if !ok || !validImageTaskID(taskID) {
				if ok {
					_ = q.rdb.ZRem(ctx, q.activeKey, taskID).Err()
				}
				return "", service.ErrImageTaskUnavailable
			}
			return taskID, nil
		}
		if !errors.Is(err, redis.Nil) {
			return "", err
		}
		if blockTimeout <= 0 || time.Now().After(deadline) {
			return "", service.ErrImageTaskQueueEmpty
		}
		timer := time.NewTimer(minDuration(imageTaskQueuePollInterval, time.Until(deadline)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func (q *imageTaskQueue) Ack(ctx context.Context, taskID string) error {
	if !validImageTaskID(taskID) {
		return service.ErrImageTaskUnavailable
	}
	return q.rdb.ZRem(ctx, q.activeKey, taskID).Err()
}

func (q *imageTaskQueue) Requeue(ctx context.Context, taskID string) error {
	if !validImageTaskID(taskID) {
		return service.ErrImageTaskUnavailable
	}
	pipe := q.rdb.TxPipeline()
	pipe.ZRem(ctx, q.activeKey, taskID)
	pipe.LPush(ctx, q.readyKey, taskID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *imageTaskQueue) Heartbeat(ctx context.Context, taskID string) error {
	if !validImageTaskID(taskID) {
		return service.ErrImageTaskUnavailable
	}
	updated, err := imageTaskQueueHeartbeatScript.Run(
		ctx,
		q.rdb,
		[]string{q.activeKey},
		taskID,
		time.Now().UnixMilli(),
	).Int()
	if err != nil {
		return err
	}
	if updated == 0 {
		return service.ErrImageTaskLeaseLost
	}
	return nil
}

func (q *imageTaskQueue) RecoverStaleActive(ctx context.Context, staleAfter time.Duration, limit int) (int, error) {
	if staleAfter <= 0 {
		return 0, service.ErrImageTaskUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	return imageTaskRecoverScript.Run(
		ctx,
		q.rdb,
		[]string{q.activeKey, q.readyKey},
		time.Now().Add(-staleAfter).UnixMilli(),
		limit,
	).Int()
}

func (q *imageTaskQueue) TryAcquireJobLock(ctx context.Context, taskID string, ttl time.Duration) (service.ImageTaskJobLock, bool, error) {
	if !validImageTaskID(taskID) || ttl <= 0 {
		return nil, false, service.ErrImageTaskUnavailable
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, false, err
	}
	lock := &imageTaskRedisLock{
		rdb:       q.rdb,
		key:       q.lockPrefix + taskID,
		token:     hex.EncodeToString(tokenBytes),
		taskID:    taskID,
		activeKey: q.activeKey,
		readyKey:  q.readyKey,
	}
	ok, err := q.rdb.SetNX(ctx, lock.key, lock.token, ttl).Result()
	if err != nil || !ok {
		return nil, ok, err
	}
	return lock, true, nil
}

func (q *imageTaskQueue) Ping(ctx context.Context) error {
	if q == nil || q.rdb == nil {
		return redis.ErrClosed
	}
	return q.rdb.Ping(ctx).Err()
}

func (q *imageTaskQueue) Stats(ctx context.Context) (service.ImageTaskQueueStats, error) {
	pipe := q.rdb.Pipeline()
	ready := pipe.LLen(ctx, q.readyKey)
	active := pipe.ZCard(ctx, q.activeKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return service.ImageTaskQueueStats{}, err
	}
	stats := service.ImageTaskQueueStats{Ready: ready.Val(), Active: active.Val()}
	candidateIDs := make([]string, 0, 2)
	oldestReadyID, err := q.rdb.LIndex(ctx, q.readyKey, -1).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return service.ImageTaskQueueStats{}, err
	}
	if id := strings.TrimSpace(oldestReadyID); id != "" {
		candidateIDs = append(candidateIDs, id)
	}
	oldestActiveIDs, err := q.rdb.ZRange(ctx, q.activeKey, 0, 0).Result()
	if err != nil {
		return service.ImageTaskQueueStats{}, err
	}
	if ids := oldestActiveIDs; len(ids) > 0 && strings.TrimSpace(ids[0]) != "" {
		candidateIDs = append(candidateIDs, ids[0])
	}
	for _, id := range candidateIDs {
		raw, err := q.rdb.Get(ctx, imageTaskKey(id)).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return service.ImageTaskQueueStats{}, err
		}
		var task service.ImageTaskRecord
		if json.Unmarshal(raw, &task) != nil {
			continue
		}
		createdAt := time.Unix(task.CreatedAt, 0).UTC()
		if stats.OldestTask == nil || createdAt.Before(stats.OldestTask.CreatedAt) {
			stats.OldestTask = &service.ImageTaskRuntimeTask{
				ID:        task.ID,
				Status:    task.Status,
				CreatedAt: createdAt,
			}
		}
	}
	return stats, nil
}

func (q *imageTaskQueue) FailUnrecoverableProcessing(ctx context.Context, before time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	var cursor uint64
	failed := 0
	for failed < limit {
		keys, next, err := q.rdb.Scan(ctx, cursor, imageTaskKeyPrefix+"*", int64(limit-failed)).Result()
		if err != nil {
			return failed, err
		}
		for _, key := range keys {
			raw, err := q.rdb.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}
			var task service.ImageTaskRecord
			if json.Unmarshal(raw, &task) != nil ||
				task.Status != service.ImageTaskStatusProcessing ||
				strings.TrimSpace(task.Request) != "" ||
				time.Unix(task.CreatedAt, 0).After(before) {
				continue
			}
			now := time.Now().UTC().Unix()
			task.Status = service.ImageTaskStatusFailed
			task.HTTPStatus = 500
			task.Error = json.RawMessage(`{"type":"api_error","code":"IMAGE_TASK_RECOVERY_UNAVAILABLE","message":"legacy processing task could not be safely recovered after restart"}`)
			task.Request = ""
			task.CompletedAt = &now
			ttl := time.Until(time.Unix(task.ExpiresAt, 0))
			if ttl <= 0 {
				ttl = time.Hour
			}
			encoded, _ := json.Marshal(&task)
			saved, err := imageTaskSaveIfStatusScript.Run(
				ctx,
				q.rdb,
				[]string{key},
				service.ImageTaskStatusProcessing,
				encoded,
				ttl.Milliseconds(),
			).Int()
			if err != nil {
				return failed, err
			}
			if saved == 1 {
				failed++
			}
			if failed >= limit {
				break
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return failed, nil
}

type imageTaskRedisLock struct {
	rdb       *redis.Client
	key       string
	token     string
	taskID    string
	activeKey string
	readyKey  string
}

func (l *imageTaskRedisLock) Release(ctx context.Context) error {
	return imageTaskReleaseLockScript.Run(ctx, l.rdb, []string{l.key}, l.token).Err()
}

func (l *imageTaskRedisLock) Refresh(ctx context.Context, ttl time.Duration) error {
	refreshed, err := imageTaskRefreshLockScript.Run(ctx, l.rdb, []string{l.key}, l.token, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if refreshed == 0 {
		return service.ErrImageTaskLeaseLost
	}
	return nil
}

func (l *imageTaskRedisLock) SaveIfStatus(
	ctx context.Context,
	task *service.ImageTaskRecord,
	expectedStatus string,
	ttl time.Duration,
) (bool, error) {
	if l == nil || l.rdb == nil || task == nil || task.ID != l.taskID || ttl <= 0 {
		return false, service.ErrImageTaskLeaseLost
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		return false, err
	}
	result, err := imageTaskSaveIfStatusWithLockScript.Run(
		ctx,
		l.rdb,
		[]string{l.key, imageTaskKey(l.taskID)},
		l.token,
		expectedStatus,
		encoded,
		ttl.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	switch result {
	case -2:
		return false, service.ErrImageTaskLeaseLost
	case -1:
		return false, service.ErrImageTaskNotFound
	default:
		return result == 1, nil
	}
}

func (l *imageTaskRedisLock) Ack(ctx context.Context) error {
	applied, err := imageTaskAckWithLockScript.Run(
		ctx,
		l.rdb,
		[]string{l.key, l.activeKey},
		l.token,
		l.taskID,
	).Int()
	if err != nil {
		return err
	}
	if applied == 0 {
		return service.ErrImageTaskLeaseLost
	}
	return nil
}

func (l *imageTaskRedisLock) Requeue(ctx context.Context) error {
	applied, err := imageTaskRequeueWithLockScript.Run(
		ctx,
		l.rdb,
		[]string{l.key, l.activeKey, l.readyKey},
		l.token,
		l.taskID,
	).Int()
	if err != nil {
		return err
	}
	if applied == 0 {
		return service.ErrImageTaskLeaseLost
	}
	return nil
}

func validImageTaskID(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "imgtask_") && len(id) > len("imgtask_")
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
