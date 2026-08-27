-- Durable private execution state for mobile video tasks. This migration is
-- additive: existing task history, groups, price rules and account mappings
-- are not rewritten.

-- Replacing the task-kind check takes an ACCESS EXCLUSIVE lock. Bound the
-- wait so a busy production table fails this deployment attempt instead of
-- holding mobile task traffic indefinitely.
SET LOCAL lock_timeout = '5s';

ALTER TABLE mobile_tasks
    DROP CONSTRAINT IF EXISTS mobile_tasks_kind_check;

ALTER TABLE mobile_tasks
    ADD CONSTRAINT mobile_tasks_kind_check
    CHECK (kind IN ('chat', 'image', 'video', 'file'));

-- Prompt text, upstream task IDs, reference assets and the pinned managed
-- video key live only here. The public mobile_tasks projection never exposes
-- any of these fields.
CREATE TABLE IF NOT EXISTS mobile_video_jobs (
    task_id UUID PRIMARY KEY REFERENCES mobile_tasks(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL,
    execution_api_key_id BIGINT NOT NULL,
    adapter VARCHAR(100) NOT NULL,
    model VARCHAR(255) NOT NULL,
    prompt TEXT NOT NULL,
    resolution VARCHAR(64) NOT NULL,
    ratio VARCHAR(32),
    duration_seconds INTEGER NOT NULL,
    generate_audio BOOLEAN NOT NULL DEFAULT FALSE,
    watermark BOOLEAN NOT NULL DEFAULT FALSE,
    reference_asset_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider_request_id VARCHAR(255),
    provider_status VARCHAR(64),
    state VARCHAR(32) NOT NULL DEFAULT 'queued',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner VARCHAR(128),
    lease_expires_at TIMESTAMPTZ,
    artifact_storage_key VARCHAR(1024),
    artifact_content_type VARCHAR(255),
    artifact_byte_size BIGINT,
    last_error JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_video_jobs_state_check
        CHECK (state IN ('queued', 'submitting', 'submission_unknown', 'polling', 'completed', 'failed', 'cancelled')),
    CONSTRAINT mobile_video_jobs_duration_check CHECK (duration_seconds > 0),
    CONSTRAINT mobile_video_jobs_attempt_check CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_claim
    ON mobile_video_jobs (state, next_poll_at, lease_expires_at, created_at);

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_user_created
    ON mobile_video_jobs (user_id, created_at DESC);
