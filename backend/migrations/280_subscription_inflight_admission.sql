SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Future server admissions only. No historical inference, rebilling or backfill.
CREATE SEQUENCE gateway_subscription_order_seq;

CREATE TABLE gateway_subscription_receipts (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL,
    attempt_no INTEGER NOT NULL,
    attempt_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version = 1),
    scope VARCHAR(48) NOT NULL CHECK (scope = 'responses_http_text_v1'),
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    package_entitlement_id BIGINT,
    account_id BIGINT NOT NULL,
    billing_request_id VARCHAR(256) NOT NULL,
    request_payload_hash VARCHAR(128) NOT NULL,
    admission_order BIGINT NOT NULL UNIQUE,
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    subscription_expires_at TIMESTAMPTZ NOT NULL,
    entitlement_expires_at TIMESTAMPTZ,
    pricing_payload TEXT NOT NULL CHECK (jsonb_typeof(pricing_payload::jsonb) = 'object'),
    pricing_hash CHAR(64) NOT NULL,
    UNIQUE(request_id, attempt_no),
    FOREIGN KEY(request_id, attempt_no) REFERENCES gateway_request_attempts(request_id, attempt_no) ON DELETE RESTRICT,
    FOREIGN KEY(attempt_id) REFERENCES gateway_request_attempt_evidence(attempt_id) ON DELETE RESTRICT
);
CREATE INDEX gateway_subscription_receipts_billing_idx ON gateway_subscription_receipts(api_key_id, billing_request_id);

CREATE TABLE gateway_subscription_events (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    event_order BIGINT NOT NULL UNIQUE,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('revoke','restore','extend')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    previous_status VARCHAR(24) NOT NULL,
    previous_expires_at TIMESTAMPTZ NOT NULL,
    previous_window_quota_available BOOLEAN NOT NULL,
    entitlement_states JSONB NOT NULL CHECK (jsonb_typeof(entitlement_states) = 'array')
);
CREATE INDEX gateway_subscription_events_order_idx ON gateway_subscription_events(subscription_id, event_order);

CREATE TABLE gateway_subscription_outbox (
    receipt_id UUID PRIMARY KEY REFERENCES gateway_subscription_receipts(id) ON DELETE RESTRICT,
    request_id UUID NOT NULL REFERENCES gateway_requests(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL,
    billing_request_id VARCHAR(256) NOT NULL,
    version INTEGER NOT NULL CHECK (version = 1),
    command_payload TEXT,
    command_hash CHAR(64),
    usage_payload TEXT NOT NULL CHECK (jsonb_typeof(usage_payload::jsonb) = 'object'),
    log_payload TEXT,
    payload_hash CHAR(64) NOT NULL,
    pending_reason VARCHAR(48) NOT NULL DEFAULT '',
    observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    completed_at TIMESTAMPTZ,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE(api_key_id, billing_request_id),
    UNIQUE(request_id),
    CHECK ((command_payload IS NULL) = (command_hash IS NULL)),
    CHECK (command_payload IS NULL OR jsonb_typeof(command_payload::jsonb) = 'object'),
    CHECK (log_payload IS NULL OR jsonb_typeof(log_payload::jsonb) = 'object'),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL))
);
CREATE INDEX gateway_subscription_outbox_ready_idx ON gateway_subscription_outbox(next_attempt_at, receipt_id)
    WHERE completed_at IS NULL AND command_payload IS NOT NULL;
