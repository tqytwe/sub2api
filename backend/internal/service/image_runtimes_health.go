package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

type RuntimeRunning interface {
	Running() bool
}

type RuntimeEnabled interface {
	IsEnabled(ctx context.Context) bool
}

type ImageRuntimeBacklog struct {
	Ready   int64 `json:"ready"`
	Delayed int64 `json:"delayed"`
	Active  int64 `json:"active"`
}

type ImageRuntimeTaskHealth struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageRuntimeErrorHealth struct {
	Code      string    `json:"code,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageRuntimeComponentHealth struct {
	Enabled              bool                     `json:"enabled"`
	Ready                bool                     `json:"ready"`
	Storage              string                   `json:"storage"`
	StorageReady         bool                     `json:"storage_ready"`
	Queue                string                   `json:"queue"`
	QueueEnabled         bool                     `json:"queue_enabled"`
	RedisReady           bool                     `json:"redis_ready"`
	WorkerRunning        bool                     `json:"worker_running"`
	Backlog              ImageRuntimeBacklog      `json:"backlog"`
	OldestTask           *ImageRuntimeTaskHealth  `json:"oldest_task,omitempty"`
	RecentError          *ImageRuntimeErrorHealth `json:"recent_error,omitempty"`
	StaleBalanceHolds    int64                    `json:"stale_balance_holds,omitempty"`
	SettlementRetries    int64                    `json:"settlement_retries,omitempty"`
	ProviderFailures     int64                    `json:"provider_failures,omitempty"`
	ResultCleanupPending int64                    `json:"result_cleanup_pending,omitempty"`
}

type ImageRuntimesHealth struct {
	GatewayAsync ImageRuntimeComponentHealth `json:"gateway_async"`
	Batch        ImageRuntimeComponentHealth `json:"batch"`
	ImageStudio  ImageRuntimeComponentHealth `json:"image_studio"`
	CheckedAt    time.Time                   `json:"checked_at"`
}

type ImageRuntimesHealthService struct {
	db                 *sql.DB
	redis              *redis.Client
	cfg                *config.Config
	batch              *BatchImageRuntimeState
	imageTask          *ImageTaskService
	imageStudio        RuntimeRunning
	imageStudioFeature RuntimeEnabled
}

func NewImageRuntimesHealthService(db *sql.DB, redisClient *redis.Client, cfg *config.Config, batch *BatchImageRuntimeState, imageTask *ImageTaskService) *ImageRuntimesHealthService {
	return &ImageRuntimesHealthService{
		db:        db,
		redis:     redisClient,
		cfg:       cfg,
		batch:     batch,
		imageTask: imageTask,
	}
}

func (s *ImageRuntimesHealthService) SetImageStudioRuntime(runtime RuntimeRunning) {
	if s != nil {
		s.imageStudio = runtime
		if feature, ok := runtime.(RuntimeEnabled); ok {
			s.imageStudioFeature = feature
		}
	}
}

func (s *ImageRuntimesHealthService) SetImageStudioFeature(feature RuntimeEnabled) {
	if s != nil {
		s.imageStudioFeature = feature
	}
}

func (s *ImageRuntimesHealthService) GetImageRuntimesHealth(ctx context.Context) (*ImageRuntimesHealth, error) {
	if s == nil || s.cfg == nil {
		return nil, errors.New("image runtimes health service is unavailable")
	}
	health := &ImageRuntimesHealth{CheckedAt: time.Now().UTC()}
	health.GatewayAsync = s.gatewayAsyncHealth(ctx)
	health.Batch = s.batchHealth(ctx)
	health.ImageStudio = s.imageStudioHealth(ctx)
	return health, nil
}

func (s *ImageRuntimesHealthService) gatewayAsyncHealth(ctx context.Context) ImageRuntimeComponentHealth {
	active := s.cfg.ImageStorage.Active() && s.imageTask != nil && s.imageTask.Enabled()
	redisReady := s.redis != nil && s.redis.Ping(ctx).Err() == nil
	return ImageRuntimeComponentHealth{
		Enabled:       active,
		Ready:         active && redisReady,
		Storage:       s.cfg.ImageStorage.BackendOrDefault(),
		StorageReady:  s.cfg.ImageStorage.Active(),
		Queue:         "legacy_in_process",
		QueueEnabled:  false,
		RedisReady:    redisReady,
		WorkerRunning: false,
	}
}

func (s *ImageRuntimesHealthService) batchHealth(ctx context.Context) ImageRuntimeComponentHealth {
	runtime := s.batch.Snapshot(ctx)
	health := ImageRuntimeComponentHealth{
		Enabled:       runtime.Enabled,
		Ready:         runtime.Ready,
		Storage:       "provider_managed",
		StorageReady:  true,
		Queue:         "redis",
		QueueEnabled:  runtime.QueueEnabled,
		RedisReady:    runtime.RedisReady,
		WorkerRunning: runtime.WorkerRunning,
		Backlog: ImageRuntimeBacklog{
			Ready:   runtime.Queue.Ready,
			Delayed: runtime.Queue.Delayed,
			Active:  runtime.Queue.Active,
		},
	}
	if runtime.LastError != "" && runtime.LastErrorAt != nil {
		health.RecentError = &ImageRuntimeErrorHealth{
			Message:   runtime.LastError,
			CreatedAt: *runtime.LastErrorAt,
		}
	}
	s.loadBatchDatabaseHealth(ctx, &health)
	return health
}

func (s *ImageRuntimesHealthService) imageStudioHealth(ctx context.Context) ImageRuntimeComponentHealth {
	dbReady := s.db != nil && s.db.PingContext(ctx) == nil
	workerRunning := s.imageStudio != nil && s.imageStudio.Running()
	enabled := s.imageStudioFeature != nil && s.imageStudioFeature.IsEnabled(ctx)
	health := ImageRuntimeComponentHealth{
		Enabled:       enabled,
		Ready:         enabled && dbReady && workerRunning,
		Storage:       "postgresql_private_assets",
		StorageReady:  dbReady,
		Queue:         "postgresql_leases",
		QueueEnabled:  true,
		WorkerRunning: workerRunning,
	}
	s.loadImageStudioDatabaseHealth(ctx, &health)
	return health
}

func (s *ImageRuntimesHealthService) loadBatchDatabaseHealth(ctx context.Context, health *ImageRuntimeComponentHealth) {
	if s.db == nil || health == nil {
		return
	}
	var oldestID, oldestStatus sql.NullString
	var oldestCreatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT batch_id, status, created_at
		FROM batch_image_jobs
		WHERE status NOT IN ('completed', 'failed', 'cancelled', 'output_deleted')
		ORDER BY created_at ASC
		LIMIT 1`).Scan(&oldestID, &oldestStatus, &oldestCreatedAt)
	if err == nil && oldestID.Valid && oldestCreatedAt.Valid {
		health.OldestTask = &ImageRuntimeTaskHealth{
			ID:        oldestID.String,
			Status:    oldestStatus.String,
			CreatedAt: oldestCreatedAt.Time.UTC(),
		}
	}

	staleAfter := 10 * time.Minute
	if s.cfg.BatchImage.StaleActiveAfterSeconds > 0 {
		staleAfter = time.Duration(s.cfg.BatchImage.StaleActiveAfterSeconds) * time.Second
	}
	_ = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (
				WHERE hold_amount IS NOT NULL
				  AND settled_at IS NULL
				  AND updated_at < $1
			),
			COUNT(*) FILTER (
				WHERE status = 'settling' AND retry_count > 0
			),
			COUNT(*) FILTER (
				WHERE status = 'failed'
				  AND COALESCE(last_error_code, '') LIKE 'PROVIDER_%'
			),
			COUNT(*) FILTER (
				WHERE output_deleted_at IS NULL
				  AND output_expires_at IS NOT NULL
				  AND output_expires_at <= NOW()
			)
		FROM batch_image_jobs`,
		time.Now().UTC().Add(-staleAfter),
	).Scan(
		&health.StaleBalanceHolds,
		&health.SettlementRetries,
		&health.ProviderFailures,
		&health.ResultCleanupPending,
	)

	var code, message sql.NullString
	var createdAt sql.NullTime
	err = s.db.QueryRowContext(ctx, `
		SELECT last_error_code, last_error_message, updated_at
		FROM batch_image_jobs
		WHERE last_error_message IS NOT NULL AND btrim(last_error_message) <> ''
		ORDER BY updated_at DESC
		LIMIT 1`).Scan(&code, &message, &createdAt)
	if err == nil && message.Valid && createdAt.Valid {
		health.RecentError = &ImageRuntimeErrorHealth{
			Code:      code.String,
			Message:   sanitizeBatchImagePublicMessage(message.String),
			CreatedAt: createdAt.Time.UTC(),
		}
	}
}

func (s *ImageRuntimesHealthService) loadImageStudioDatabaseHealth(ctx context.Context, health *ImageRuntimeComponentHealth) {
	if s.db == nil || health == nil {
		return
	}
	var oldestID, oldestStatus sql.NullString
	var oldestCreatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id::text, status, created_at
		FROM image_studio_jobs
		WHERE status IN ('pending', 'running')
		ORDER BY created_at ASC
		LIMIT 1`).Scan(&oldestID, &oldestStatus, &oldestCreatedAt)
	if err == nil && oldestID.Valid && oldestCreatedAt.Valid {
		health.OldestTask = &ImageRuntimeTaskHealth{
			ID:        oldestID.String,
			Status:    oldestStatus.String,
			CreatedAt: oldestCreatedAt.Time.UTC(),
		}
	}
	_ = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'running')
		FROM image_studio_jobs`).Scan(&health.Backlog.Ready, &health.Backlog.Active)

	var message sql.NullString
	var createdAt sql.NullTime
	err = s.db.QueryRowContext(ctx, `
		SELECT error_message, COALESCE(finished_at, created_at)
		FROM image_studio_jobs
		WHERE error_message IS NOT NULL AND btrim(error_message) <> ''
		ORDER BY COALESCE(finished_at, created_at) DESC
		LIMIT 1`).Scan(&message, &createdAt)
	if err == nil && message.Valid && createdAt.Valid {
		health.RecentError = &ImageRuntimeErrorHealth{
			Message:   sanitizeBatchImagePublicMessage(message.String),
			CreatedAt: createdAt.Time.UTC(),
		}
	}
}
