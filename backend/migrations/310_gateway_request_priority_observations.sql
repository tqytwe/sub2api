SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Priority scheduling uses only complete, successful, semantic-output attempts.
-- Missing or rejected observations remain unknown; no history is backfilled.
CREATE TABLE IF NOT EXISTS gateway_request_priority_observations (
    attempt_id UUID PRIMARY KEY REFERENCES gateway_request_attempt_evidence(attempt_id) ON DELETE CASCADE,
    schema_version SMALLINT NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    model VARCHAR(200) NOT NULL CHECK (length(model) BETWEEN 1 AND 200),
    inbound_endpoint VARCHAR(192) NOT NULL DEFAULT '' CHECK (length(inbound_endpoint) <= 192),
    group_id BIGINT,
    first_semantic_ms INTEGER NOT NULL CHECK (first_semantic_ms > 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= first_semantic_ms),
    http_status SMALLINT NOT NULL CHECK (http_status BETWEEN 200 AND 299),
    terminal_state VARCHAR(16) NOT NULL CHECK (terminal_state = 'completed'),
    body_complete BOOLEAN NOT NULL CHECK (body_complete),
    truncated BOOLEAN NOT NULL DEFAULT FALSE CHECK (NOT truncated),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS gateway_request_priority_observations_created_idx
    ON gateway_request_priority_observations (created_at, attempt_id);
CREATE INDEX IF NOT EXISTS gateway_request_priority_observations_model_created_idx
    ON gateway_request_priority_observations (model, created_at, attempt_id);
CREATE INDEX IF NOT EXISTS gateway_request_priority_observations_scope_idx
    ON gateway_request_priority_observations (model, inbound_endpoint, group_id, created_at, attempt_id);
