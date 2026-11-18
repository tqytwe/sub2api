-- Persist threshold episodes independently from request-failure alert coalescing.
CREATE TABLE IF NOT EXISTS account_ops_threshold_monitors (
    group_id TEXT NOT NULL,
    account_id BIGINT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('balance_threshold', 'quota_threshold')),
    criteria TEXT NOT NULL DEFAULT '',
    identity TEXT NOT NULL DEFAULT '',
    episode_id TEXT NOT NULL DEFAULT '',
    adverse BOOLEAN NOT NULL DEFAULT FALSE,
    eligible BOOLEAN NOT NULL DEFAULT FALSE,
    healthy_since TIMESTAMPTZ,
    checked_at TIMESTAMPTZ,
    PRIMARY KEY (group_id, kind)
);
CREATE INDEX IF NOT EXISTS account_ops_threshold_monitors_account ON account_ops_threshold_monitors(account_id, kind);

CREATE TABLE IF NOT EXISTS account_ops_threshold_events (
    id TEXT PRIMARY KEY,
    episode_id TEXT NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('alert', 'recovery')),
    notification_enabled BOOLEAN NOT NULL,
    account_id BIGINT NOT NULL,
    group_id TEXT NOT NULL,
    group_name TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('balance_threshold', 'quota_threshold')),
    account_name TEXT NOT NULL DEFAULT '',
    signal TEXT NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL DEFAULT 0,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    occurrences BIGINT NOT NULL DEFAULT 1,
    state TEXT NOT NULL CHECK (state IN ('pending', 'sending', 'sent', 'failed', 'suppressed', 'resolved')),
    last_sent_at TIMESTAMPTZ,
    next_send_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INTEGER NOT NULL DEFAULT 0,
    lease TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    details JSONB NOT NULL DEFAULT 'null'::jsonb,
    deliveries JSONB NOT NULL DEFAULT '{}'::jsonb,
    identity TEXT NOT NULL,
    criteria TEXT NOT NULL,
    UNIQUE (episode_id, phase)
);
CREATE INDEX IF NOT EXISTS account_ops_threshold_events_due ON account_ops_threshold_events(next_send_at)
    WHERE state IN ('pending', 'failed', 'sending');
CREATE INDEX IF NOT EXISTS account_ops_threshold_events_history ON account_ops_threshold_events(last_seen DESC, id);
CREATE INDEX IF NOT EXISTS account_ops_threshold_events_group ON account_ops_threshold_events(group_id, kind, last_seen DESC);
