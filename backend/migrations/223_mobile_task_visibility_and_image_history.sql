-- Add user-facing visibility controls for the unified mobile task protocol.
-- The row is kept for diagnostics/audit, while APP history lists hide it.

ALTER TABLE mobile_tasks
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_mobile_tasks_user_kind_visible_created
    ON mobile_tasks (user_id, kind, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_mobile_tasks_user_status_visible_created
    ON mobile_tasks (user_id, status, created_at DESC)
    WHERE deleted_at IS NULL;
