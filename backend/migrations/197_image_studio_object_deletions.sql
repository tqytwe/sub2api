-- Durable outbox for private Image Studio objects whose metadata is being removed.

CREATE TABLE IF NOT EXISTS image_studio_object_deletions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    job_id      UUID,
    storage_key TEXT NOT NULL,
    attempts    INT NOT NULL DEFAULT 0,
    last_error  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_studio_object_deletions_storage_key_chk
        CHECK (btrim(storage_key) <> ''),
    CONSTRAINT image_studio_object_deletions_attempts_chk
        CHECK (attempts >= 0),
    CONSTRAINT image_studio_object_deletions_job_key_uniq
        UNIQUE (job_id, storage_key)
);

CREATE INDEX IF NOT EXISTS idx_image_studio_object_deletions_pending
    ON image_studio_object_deletions(updated_at, id);

COMMENT ON TABLE image_studio_object_deletions IS
    'Deletion outbox retained after job metadata is removed so private objects can be retried';
