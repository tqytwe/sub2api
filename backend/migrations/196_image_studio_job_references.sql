-- 196_image_studio_job_references.sql
-- Durable private copies of edit references owned by an accepted Image Studio job.

CREATE TABLE IF NOT EXISTS image_studio_job_references (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       UUID NOT NULL REFERENCES image_studio_jobs(id) ON DELETE CASCADE,
    storage_key  TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size    BIGINT NOT NULL,
    sort_order   INT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_studio_job_references_byte_size_chk CHECK (byte_size > 0),
    CONSTRAINT image_studio_job_references_storage_key_chk CHECK (btrim(storage_key) <> ''),
    CONSTRAINT image_studio_job_references_content_type_chk CHECK (
        content_type IN ('image/png', 'image/jpeg', 'image/webp')
    ),
    CONSTRAINT image_studio_job_references_sort_order_chk CHECK (sort_order >= 0),
    CONSTRAINT image_studio_job_references_job_sort_uniq UNIQUE (job_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_image_studio_job_references_job
    ON image_studio_job_references(job_id, sort_order);

COMMENT ON TABLE image_studio_job_references IS
    'Private reference objects copied for an accepted edit job; lifetime follows the job';
