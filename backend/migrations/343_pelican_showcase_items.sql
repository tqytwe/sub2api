-- User-visible Pelican artwork snapshots, retained independently from admin test history.
CREATE TABLE IF NOT EXISTS pelican_showcase_items (
    id               BIGSERIAL PRIMARY KEY,
    group_id         BIGINT      NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    source_result_id BIGINT      NOT NULL,
    model_id         TEXT        NOT NULL DEFAULT '',
    reasoning_effort TEXT        NOT NULL DEFAULT '',
    response_text    TEXT        NOT NULL,
    latency_ms       BIGINT      NOT NULL DEFAULT 0,
    generated_at     TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (group_id, source_result_id)
);

CREATE INDEX IF NOT EXISTS idx_pelican_showcase_items_group_generated
    ON pelican_showcase_items (group_id, generated_at DESC, id DESC);

COMMENT ON TABLE pelican_showcase_items IS
    'Pelican user showcase snapshots, retained separately from admin test history';
COMMENT ON COLUMN pelican_showcase_items.source_result_id IS
    'Source group-test result ID used for deduplication; no foreign key so showcase retention is independent';
