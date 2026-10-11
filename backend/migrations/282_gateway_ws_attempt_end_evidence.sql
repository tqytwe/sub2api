SET LOCAL lock_timeout = '50ms';
SET LOCAL statement_timeout = '500ms';

-- The FK's parent lock must not queue behind live admission/finish. Isolate
-- optional WS writes from the original identifier evidence row and its locks.
LOCK TABLE gateway_request_attempts IN ACCESS EXCLUSIVE MODE NOWAIT;
CREATE TABLE IF NOT EXISTS gateway_request_ws_end_evidence (
    request_id UUID NOT NULL,
    attempt_no INTEGER NOT NULL,
    attempt_id UUID NOT NULL,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (request_id, attempt_no),
    FOREIGN KEY (request_id, attempt_no)
        REFERENCES gateway_request_attempts (request_id, attempt_no) ON DELETE CASCADE,
    CONSTRAINT gateway_ws_end_diagnostics_bounds CHECK (
        jsonb_typeof(snapshot) = 'object' AND octet_length(snapshot::text) <= 4096
    )
);
