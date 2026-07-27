-- Product work items linked to the existing Play Ops mobile feedback entry.
-- This keeps management in /admin/play/mobile-feedback while allowing feedback
-- to become a triaged bug/feature/UX item with release tracking.

CREATE TABLE IF NOT EXISTS mobile_feedback_work_items (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES mobile_feedback(id) ON DELETE CASCADE,
    type TEXT NOT NULL DEFAULT 'bug',
    priority TEXT NOT NULL DEFAULT 'p2',
    status TEXT NOT NULL DEFAULT 'backlog',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    acceptance_criteria TEXT NOT NULL DEFAULT '',
    target_version TEXT NOT NULL DEFAULT '',
    released_version TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'auto',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_feedback_work_items_type_check CHECK (type IN ('bug', 'feature', 'ux', 'payment', 'account', 'network', 'image', 'chat', 'other')),
    CONSTRAINT mobile_feedback_work_items_priority_check CHECK (priority IN ('p0', 'p1', 'p2', 'p3')),
    CONSTRAINT mobile_feedback_work_items_status_check CHECK (status IN ('backlog', 'accepted', 'in_progress', 'testing', 'released', 'rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mobile_feedback_work_items_feedback_auto
    ON mobile_feedback_work_items (feedback_id, source)
    WHERE source = 'auto';

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_work_items_status_priority
    ON mobile_feedback_work_items (status, priority, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_work_items_feedback
    ON mobile_feedback_work_items (feedback_id, created_at DESC, id DESC);
