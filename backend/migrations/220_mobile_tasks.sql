-- Cross-device task history for chat, image and file operations.

CREATE TABLE IF NOT EXISTS mobile_tasks (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(16) NOT NULL,
    operation VARCHAR(100) NOT NULL,
    status VARCHAR(16) NOT NULL,
    progress INTEGER NOT NULL DEFAULT 0,
    parent_task_id UUID,
    retry_of UUID,
    client_request_id VARCHAR(128) NOT NULL,
    resource JSONB,
    artifacts JSONB NOT NULL DEFAULT '[]'::jsonb,
    error JSONB,
    protocol_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT mobile_tasks_kind_check CHECK (kind IN ('chat', 'image', 'file')),
    CONSTRAINT mobile_tasks_status_check CHECK (status IN ('queued', 'running', 'streaming', 'completed', 'partial', 'failed', 'cancelled')),
    CONSTRAINT mobile_tasks_progress_check CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT mobile_tasks_protocol_check CHECK (protocol_version > 0),
    UNIQUE (user_id, client_request_id)
);

CREATE INDEX IF NOT EXISTS idx_mobile_tasks_user_created ON mobile_tasks (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_tasks_user_kind_created ON mobile_tasks (user_id, kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_tasks_user_status_created ON mobile_tasks (user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_tasks_retry_of ON mobile_tasks (retry_of);
CREATE INDEX IF NOT EXISTS idx_mobile_tasks_parent ON mobile_tasks (parent_task_id);
