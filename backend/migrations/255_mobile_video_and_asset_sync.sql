-- Domestic mobile video jobs and incremental local material synchronization.
-- This migration is deliberately additive and keeps existing task history.

ALTER TABLE mobile_tasks
    DROP CONSTRAINT IF EXISTS mobile_tasks_kind_check;

ALTER TABLE mobile_tasks
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE mobile_tasks
    ADD CONSTRAINT mobile_tasks_kind_check
    CHECK (kind IN ('chat', 'image', 'video', 'file'));

CREATE INDEX IF NOT EXISTS idx_mobile_tasks_deleted_at
    ON mobile_tasks (deleted_at);

ALTER TABLE mobile_assets
    DROP CONSTRAINT IF EXISTS mobile_assets_source_check;

ALTER TABLE mobile_assets
    ADD CONSTRAINT mobile_assets_source_check
    CHECK (source IN ('upload', 'share', 'image_result', 'video_result', 'chat_export', 'voice'));

CREATE INDEX IF NOT EXISTS idx_mobile_assets_user_updated
    ON mobile_assets (user_id, updated_at DESC, id DESC);

-- Private execution payload for mobile video tasks.  The public mobile_tasks
-- projection intentionally contains no prompt, references, provider request
-- id, credentials, or upstream URLs.  This row is owned by the task and is
-- reclaimed by the worker lease after a process restart.
CREATE TABLE IF NOT EXISTS mobile_video_jobs (
    task_id UUID PRIMARY KEY REFERENCES mobile_tasks(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL,
    model VARCHAR(255) NOT NULL,
    prompt TEXT NOT NULL,
    resolution VARCHAR(32) NOT NULL,
    ratio VARCHAR(32),
    duration_seconds INTEGER NOT NULL,
    generate_audio BOOLEAN NOT NULL DEFAULT FALSE,
    watermark BOOLEAN NOT NULL DEFAULT FALSE,
    reference_asset_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider VARCHAR(32) NOT NULL,
    provider_request_id VARCHAR(255),
    provider_status VARCHAR(32),
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner VARCHAR(128),
    lease_expires_at TIMESTAMPTZ,
    artifact_storage_key VARCHAR(1024),
    artifact_url TEXT,
    artifact_content_type VARCHAR(255),
    artifact_byte_size BIGINT,
    last_error JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_video_jobs_state_check CHECK (state IN ('queued', 'submitting', 'polling', 'completed', 'failed', 'cancelled')),
    CONSTRAINT mobile_video_jobs_duration_check CHECK (duration_seconds > 0),
    CONSTRAINT mobile_video_jobs_attempt_check CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_claim
    ON mobile_video_jobs (state, next_poll_at, lease_expires_at, created_at);
CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_user_created
    ON mobile_video_jobs (user_id, created_at DESC);
