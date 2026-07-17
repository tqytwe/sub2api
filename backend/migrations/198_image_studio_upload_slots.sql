-- Shared per-user upload concurrency leases and rolling-minute attempt accounting.

CREATE TABLE IF NOT EXISTS image_studio_upload_slots (
    id               UUID PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    started_at       TIMESTAMPTZ NOT NULL,
    lease_expires_at TIMESTAMPTZ NOT NULL,
    released_at      TIMESTAMPTZ,
    CONSTRAINT image_studio_upload_slots_lease_chk
        CHECK (lease_expires_at > started_at),
    CONSTRAINT image_studio_upload_slots_release_chk
        CHECK (released_at IS NULL OR released_at >= started_at)
);

CREATE INDEX IF NOT EXISTS idx_image_studio_upload_slots_user_started
    ON image_studio_upload_slots(user_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_image_studio_upload_slots_active
    ON image_studio_upload_slots(user_id, lease_expires_at)
    WHERE released_at IS NULL;

COMMENT ON TABLE image_studio_upload_slots IS
    'Shared Image Studio reference-upload leases and rolling rate-limit attempts';
