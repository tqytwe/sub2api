-- 195_image_studio_billing_reconciliation.sql
-- Durable audit and retry state for managed Image Studio usage billing failures.

CREATE TABLE IF NOT EXISTS image_studio_billing_reconciliations (
    id                  BIGSERIAL PRIMARY KEY,
    request_id          TEXT NOT NULL,
    api_key_id          BIGINT NOT NULL,
    user_id             BIGINT NOT NULL,
    actual_cost         DECIMAL(20, 8) NOT NULL,
    command_payload     JSONB NOT NULL,
    command_fingerprint TEXT NOT NULL,
    last_error          TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    attempts            INT NOT NULL DEFAULT 1,
    first_failed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_failed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at         TIMESTAMPTZ,
    CONSTRAINT uq_image_studio_billing_reconciliation_request
        UNIQUE (request_id, api_key_id),
    CONSTRAINT chk_image_studio_billing_reconciliation_cost
        CHECK (actual_cost >= 0),
    CONSTRAINT chk_image_studio_billing_reconciliation_fingerprint
        CHECK (btrim(command_fingerprint) <> ''),
    CONSTRAINT chk_image_studio_billing_reconciliation_payload
        CHECK (jsonb_typeof(command_payload) = 'object'),
    CONSTRAINT chk_image_studio_billing_reconciliation_status
        CHECK (status IN ('pending', 'processing', 'resolved', 'failed')),
    CONSTRAINT chk_image_studio_billing_reconciliation_attempts
        CHECK (attempts > 0)
);

CREATE INDEX IF NOT EXISTS idx_image_studio_billing_reconciliation_pending
    ON image_studio_billing_reconciliations(status, last_failed_at, id)
    WHERE status IN ('pending', 'failed');

COMMENT ON TABLE image_studio_billing_reconciliations IS
    'Durable managed Image Studio billing failures awaiting reconciliation';
COMMENT ON COLUMN image_studio_billing_reconciliations.command_payload IS
    'Sanitized usage billing command sufficient for audited retry';
