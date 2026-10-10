SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Additive, forward-only. No history is inferred, deleted, or billed here.
-- Deliberately no FK to mutable user/key/account/usage rows: audit attribution
-- must survive their deletion and must not lock existing high-traffic tables.
CREATE TABLE IF NOT EXISTS gateway_ledger_instances (
    id UUID PRIMARY KEY,
    lease_until TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS gateway_requests (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    kind VARCHAR(24) NOT NULL CHECK (kind IN ('http', 'ws_session', 'ws_turn', 'async_execution')),
    turn_no INTEGER CHECK (turn_no > 0),
    route VARCHAR(192) NOT NULL,
    method VARCHAR(16) NOT NULL,
    user_id BIGINT,
    api_key_id BIGINT,
    instance_id UUID NOT NULL REFERENCES gateway_ledger_instances(id) ON DELETE RESTRICT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ended_at TIMESTAMPTZ,
    execution_state VARCHAR(16) NOT NULL DEFAULT 'inflight'
        CHECK (execution_state IN ('inflight','succeeded','failed','cancelled','timeout','interrupted')),
    usage_state VARCHAR(20) NOT NULL DEFAULT 'not_applicable'
        CHECK (usage_state IN ('not_applicable','pending','known','usage_unknown')),
    settlement_state VARCHAR(24) NOT NULL DEFAULT 'not_required'
        CHECK (settlement_state IN ('not_required','settlement_pending','settled')),
    error_code VARCHAR(48) NOT NULL DEFAULT ''
        CHECK (error_code IN ('','unauthorized','forbidden','rate_limited','invalid_request','upstream_error',
            'internal_error','cancelled','timeout','interrupted','request_ledger_unavailable','stream_incomplete')),
    http_status INTEGER CHECK (http_status BETWEEN 100 AND 599),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    output_observed BOOLEAN NOT NULL DEFAULT FALSE,
    reconcile_checked_at TIMESTAMPTZ,
    CHECK ((execution_state = 'inflight') = (ended_at IS NULL)),
    CHECK (api_key_id IS NULL OR user_id IS NOT NULL),
    UNIQUE (parent_id, turn_no)
);

CREATE TABLE IF NOT EXISTS gateway_request_attempts (
    request_id UUID NOT NULL REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    attempt_no INTEGER NOT NULL CHECK (attempt_no > 0),
    phase VARCHAR(16) NOT NULL DEFAULT 'request' CHECK (phase IN ('request','ws_connect','ws_control','ws_input','ws_observed','external_search','auxiliary')),
    upstream_kind VARCHAR(24) NOT NULL DEFAULT 'account' CHECK (upstream_kind IN ('account','external_search')),
    account_id BIGINT,
    credential_account_id BIGINT,
    CHECK ((upstream_kind='account' AND account_id IS NOT NULL AND credential_account_id IS NOT NULL AND account_id>0 AND credential_account_id>0)
        OR (upstream_kind='external_search' AND account_id IS NULL AND credential_account_id IS NULL)),
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ended_at TIMESTAMPTZ,
    execution_state VARCHAR(16) NOT NULL DEFAULT 'inflight'
        CHECK (execution_state IN ('inflight','succeeded','failed','cancelled','timeout','interrupted')),
    http_status INTEGER CHECK (http_status BETWEEN 100 AND 599),
    error_code VARCHAR(48) NOT NULL DEFAULT ''
        CHECK (error_code IN ('','upstream_error','cancelled','timeout','interrupted','stream_incomplete','request_ledger_unavailable')),
    usage_state VARCHAR(20) NOT NULL DEFAULT 'not_applicable' CHECK (usage_state IN ('not_applicable','pending','known','usage_unknown')),
    output_observed BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (request_id, attempt_no),
    CHECK ((execution_state = 'inflight') = (ended_at IS NULL))
);

CREATE TABLE IF NOT EXISTS gateway_request_billing_links (
    request_id UUID NOT NULL REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    billing_request_id VARCHAR(256) NOT NULL,
    api_key_id BIGINT NOT NULL,
    request_fingerprint VARCHAR(128) NOT NULL,
    subscription_id BIGINT,
    package_entitlement_id BIGINT,
    settlement_verified BOOLEAN NOT NULL DEFAULT FALSE,
    settlement_applied BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (request_id, billing_request_id, api_key_id)
);

-- These tables are new/empty. No existing table scan or concurrent index build.
CREATE INDEX IF NOT EXISTS gateway_requests_recent_idx ON gateway_requests (started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS gateway_requests_user_recent_idx ON gateway_requests (user_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS gateway_requests_key_recent_idx ON gateway_requests (api_key_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS gateway_requests_state_recent_idx ON gateway_requests (execution_state, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS gateway_requests_inflight_idx ON gateway_requests (instance_id) WHERE execution_state='inflight';
CREATE INDEX IF NOT EXISTS gateway_request_attempts_account_idx ON gateway_request_attempts (account_id, request_id);
CREATE INDEX IF NOT EXISTS gateway_request_attempts_credential_idx ON gateway_request_attempts (credential_account_id, request_id);
CREATE INDEX IF NOT EXISTS gateway_request_billing_lookup_idx ON gateway_request_billing_links (billing_request_id, api_key_id);

-- Task references are opaque IDs, never envelopes, prompts, URLs or credentials.
-- A replay can reference an existing task without replacing its first owner.
CREATE TABLE IF NOT EXISTS gateway_request_tasks (
    task_kind VARCHAR(24) NOT NULL CHECK (task_kind IN ('image_task','image_studio','image_batch','mobile_video','live_call')),
    task_ref VARCHAR(160) NOT NULL,
    request_id UUID NOT NULL REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (task_kind,task_ref,request_id)
);
CREATE INDEX IF NOT EXISTS gateway_request_tasks_request_idx ON gateway_request_tasks(request_id);

CREATE INDEX IF NOT EXISTS gateway_requests_pending_reconcile_idx ON gateway_requests(reconcile_checked_at NULLS FIRST,id) WHERE settlement_state='settlement_pending';
CREATE INDEX IF NOT EXISTS gateway_requests_usage_recent_idx ON gateway_requests(usage_state,started_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS gateway_requests_settlement_recent_idx ON gateway_requests(settlement_state,started_at DESC,id DESC);
