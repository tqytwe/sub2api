-- Durable, read-only mirror of the managed Canvas image prompt directory.
-- The mirror is separate from the administrator-managed PromptLibrary so an
-- upstream refresh can never bypass its review/publish lifecycle.

CREATE TABLE IF NOT EXISTS canvas_prompt_mirror_items (
    id BIGSERIAL PRIMARY KEY,
    source_key VARCHAR(128) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    title TEXT NOT NULL,
    prompt_text TEXT NOT NULL,
    cover_url TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    preview TEXT NOT NULL DEFAULT '',
    category VARCHAR(255) NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_hash CHAR(64) NOT NULL,
    source_created_at TIMESTAMPTZ,
    source_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT canvas_prompt_mirror_items_source_id_unique UNIQUE (source_key, external_id)
);

CREATE INDEX IF NOT EXISTS idx_canvas_prompt_mirror_items_active
    ON canvas_prompt_mirror_items (source_key, deleted_at, updated_at DESC, external_id);
CREATE INDEX IF NOT EXISTS idx_canvas_prompt_mirror_items_delta
    ON canvas_prompt_mirror_items (source_key, updated_at DESC, external_id);

CREATE TABLE IF NOT EXISTS canvas_prompt_mirror_state (
    source_key VARCHAR(128) PRIMARY KEY,
    revision CHAR(64) NOT NULL,
    total BIGINT NOT NULL DEFAULT 0,
    categories JSONB NOT NULL DEFAULT '[]'::jsonb,
    categories_hash CHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    categories_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT canvas_prompt_mirror_state_total_check CHECK (total >= 0)
);
