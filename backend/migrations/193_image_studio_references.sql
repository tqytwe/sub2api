-- 193_image_studio_references.sql
-- Private, short-lived reference images uploaded before an Image Studio edit job.

CREATE TABLE IF NOT EXISTS image_studio_references (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key       TEXT NOT NULL,
    original_filename TEXT NOT NULL DEFAULT '',
    content_type      TEXT NOT NULL,
    byte_size         BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT image_studio_references_byte_size_chk CHECK (byte_size > 0),
    CONSTRAINT image_studio_references_storage_key_chk CHECK (btrim(storage_key) <> ''),
    CONSTRAINT image_studio_references_content_type_chk CHECK (content_type LIKE 'image/%')
);

CREATE INDEX IF NOT EXISTS idx_image_studio_references_user_created
    ON image_studio_references(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_image_studio_references_expiry
    ON image_studio_references(expires_at);

CREATE INDEX IF NOT EXISTS idx_image_studio_references_user_expiry
    ON image_studio_references(user_id, expires_at)
    INCLUDE (byte_size);

COMMENT ON TABLE image_studio_references IS
    'Private reference image metadata; object bytes remain under the Image Studio private storage root';
