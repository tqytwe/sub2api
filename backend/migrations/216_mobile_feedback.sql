-- Mobile app feedback collection for JisudengChat Android.

CREATE TABLE IF NOT EXISTS mobile_feedback (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'other',
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'new',
    app_version TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT 'android',
    device_model TEXT NOT NULL DEFAULT '',
    android_version TEXT NOT NULL DEFAULT '',
    system_version TEXT NOT NULL DEFAULT '',
    group_name TEXT NOT NULL DEFAULT '',
    group_id BIGINT,
    backend_url TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    crash_log TEXT NOT NULL DEFAULT '',
    device_info JSONB NOT NULL DEFAULT '{}'::jsonb,
    screenshots JSONB NOT NULL DEFAULT '[]'::jsonb,
    admin_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_feedback_status_check CHECK (status IN ('new', 'viewed', 'handled', 'deferred', 'ignored')),
    CONSTRAINT mobile_feedback_category_check CHECK (category IN ('bug', 'experience', 'image', 'chat', 'payment', 'account', 'request', 'other'))
);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_created_at
    ON mobile_feedback (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_status_created
    ON mobile_feedback (status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_user_created
    ON mobile_feedback (user_id, created_at DESC, id DESC);
