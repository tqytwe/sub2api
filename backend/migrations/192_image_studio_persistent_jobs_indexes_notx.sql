-- Online indexes for durable Image Studio jobs and items.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_jobs_claim
    ON image_studio_jobs(status, lease_expires_at, created_at)
    WHERE status IN ('pending', 'running');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_jobs_user_active
    ON image_studio_jobs(user_id, status, created_at)
    WHERE status IN ('pending', 'running');

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_image_studio_jobs_user_idempotency
    ON image_studio_jobs(user_id, idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_items_job_status
    ON image_studio_items(job_id, status, sort_order);
