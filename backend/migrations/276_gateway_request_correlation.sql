SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Forward-only evidence for future attempts. Old writers/rows remain valid;
-- no inference, backfill, billing operation, or change to historical SQL.
CREATE TABLE IF NOT EXISTS gateway_request_attempt_evidence (
    request_id UUID NOT NULL,
    attempt_no INTEGER NOT NULL,
    attempt_id UUID NOT NULL UNIQUE,
    upstream_request_id VARCHAR(128),
    upstream_response_id VARCHAR(128),
    identifier_conflict BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (request_id, attempt_no),
    FOREIGN KEY (request_id, attempt_no) REFERENCES gateway_request_attempts(request_id, attempt_no) ON DELETE RESTRICT,
    CHECK (upstream_request_id IS NULL OR upstream_request_id ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$'),
    CHECK (upstream_response_id IS NULL OR upstream_response_id ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$')
);

-- attempt_no=0 is an explicit request-only diagnostic, never a guessed attempt.
-- No FK to ops_error_logs: Ops retention must not delete durable audit evidence.
CREATE TABLE IF NOT EXISTS gateway_request_ops_links (
    request_id UUID NOT NULL REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    attempt_no INTEGER NOT NULL CHECK (attempt_no >= 0),
    ops_error_log_id BIGINT NOT NULL CHECK (ops_error_log_id > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (request_id, attempt_no, ops_error_log_id)
);
CREATE INDEX IF NOT EXISTS gateway_request_ops_links_ops_idx ON gateway_request_ops_links(ops_error_log_id);
