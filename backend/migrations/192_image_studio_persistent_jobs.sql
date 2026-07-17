-- 192_image_studio_persistent_jobs.sql
-- Durable Image Studio jobs with encrypted requests, item progress, leases, and settlement state.
-- The migration runner runs each statement in its own recoverable phase with lock_timeout.
-- Keep every statement idempotent and keep long validation or backfill work separate from DDL.

ALTER TABLE image_studio_jobs
    DROP CONSTRAINT IF EXISTS chk_image_studio_jobs_status_upgrade;

ALTER TABLE image_studio_jobs
    ADD CONSTRAINT chk_image_studio_jobs_status_upgrade
    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled', 'partial'))
    NOT VALID;

ALTER TABLE image_studio_jobs
    VALIDATE CONSTRAINT chk_image_studio_jobs_status_upgrade;

ALTER TABLE image_studio_jobs
    DROP CONSTRAINT IF EXISTS chk_image_studio_jobs_status;

ALTER TABLE image_studio_jobs
    RENAME CONSTRAINT chk_image_studio_jobs_status_upgrade
    TO chk_image_studio_jobs_status;

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS request_payload_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quality TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS hold_amount DECIMAL(20, 8),
    ADD COLUMN IF NOT EXISTS hold_id TEXT,
    ADD COLUMN IF NOT EXISTS success_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fail_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS lease_owner TEXT,
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS idempotency_key_hash TEXT,
    ADD COLUMN IF NOT EXISTS idempotency_fingerprint TEXT;

UPDATE image_studio_jobs
SET status = 'failed',
    error_message = COALESCE(error_message, 'legacy image studio job cannot be resumed after durable worker upgrade'),
    finished_at = COALESCE(finished_at, NOW()),
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE status IN ('pending', 'running')
  AND (request_payload_encrypted IS NULL OR btrim(request_payload_encrypted) = '');

ALTER TABLE image_studio_jobs
    DROP CONSTRAINT IF EXISTS image_studio_jobs_active_payload_chk_upgrade;

ALTER TABLE image_studio_jobs
    ADD CONSTRAINT image_studio_jobs_active_payload_chk_upgrade
    CHECK (
        status NOT IN ('pending', 'running')
        OR (request_payload_encrypted IS NOT NULL AND btrim(request_payload_encrypted) <> '')
    )
    NOT VALID;

ALTER TABLE image_studio_jobs
    VALIDATE CONSTRAINT image_studio_jobs_active_payload_chk_upgrade;

ALTER TABLE image_studio_jobs
    DROP CONSTRAINT IF EXISTS image_studio_jobs_active_payload_chk;

ALTER TABLE image_studio_jobs
    RENAME CONSTRAINT image_studio_jobs_active_payload_chk_upgrade
    TO image_studio_jobs_active_payload_chk;

CREATE TABLE IF NOT EXISTS image_studio_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES image_studio_jobs(id) ON DELETE CASCADE,
    sort_order  INT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    actual_cost DECIMAL(20, 8),
    checkpoint_data BYTEA,
    checkpoint_content_type TEXT,
    checkpoint_actual_cost DECIMAL(20, 8),
    error       TEXT,
    asset_id    UUID REFERENCES image_studio_assets(id) ON DELETE SET NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT chk_image_studio_items_status
        CHECK (status IN ('pending', 'running', 'persisting', 'success', 'failed', 'cancelled')),
    CONSTRAINT uq_image_studio_items_job_sort UNIQUE (job_id, sort_order)
);

COMMENT ON COLUMN image_studio_jobs.request_payload_encrypted IS
    'SecretEncryptor ciphertext containing the gateway request payload, never plaintext prompt';
COMMENT ON COLUMN image_studio_jobs.hold_id IS
    'Idempotent balance hold request ID used to validate capture and release';
COMMENT ON COLUMN image_studio_jobs.idempotency_key_hash IS
    'SHA-256 of the client Idempotency-Key, unique per user when present';
COMMENT ON COLUMN image_studio_jobs.idempotency_fingerprint IS
    'Canonical request fingerprint used to reject key reuse with a different payload';
COMMENT ON TABLE image_studio_items IS
    'Durable per-output Image Studio progress and authoritative actual cost';
