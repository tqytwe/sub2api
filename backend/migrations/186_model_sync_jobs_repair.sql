-- 186_model_sync_jobs_repair.sql
-- Repair production databases where migration 183 created the catalog but the
-- async model sync job table was omitted or left with an incomplete schema.

CREATE TABLE IF NOT EXISTS model_sync_jobs (
    id            VARCHAR(64) PRIMARY KEY,
    kind          VARCHAR(32) NOT NULL DEFAULT 'pricing_refresh',
    status        VARCHAR(16) NOT NULL,
    result        JSONB,
    error         TEXT,
    started_at    TIMESTAMPTZ NOT NULL,
    completed_at  TIMESTAMPTZ
);

ALTER TABLE model_sync_jobs
    ADD COLUMN IF NOT EXISTS id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS kind VARCHAR(32) NOT NULL DEFAULT 'pricing_refresh',
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'running',
    ADD COLUMN IF NOT EXISTS result JSONB,
    ADD COLUMN IF NOT EXISTS error TEXT,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_model_sync_jobs_started_at
    ON model_sync_jobs (started_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_sync_jobs_id
    ON model_sync_jobs (id);
