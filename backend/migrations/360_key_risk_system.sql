-- Per-key source risk episodes and owner-scoped acknowledgement state.
-- This migration is additive and intentionally has no historical backfill.
CREATE TABLE key_risk_control (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  config jsonb NOT NULL DEFAULT '{}',
  last_scan_at timestamptz,
  last_error text NOT NULL DEFAULT ''
);
-- Preserve the Fork production behavior: the administrator opts in before the
-- first historical scan or any notification. Later saves keep the upstream rules.
INSERT INTO key_risk_control(singleton, config) VALUES (true, '{"enabled":false}'::jsonb);

CREATE TABLE key_risk_events (
  id bigserial PRIMARY KEY,
  rule text NOT NULL,
  subject text NOT NULL,
  evidence jsonb NOT NULL,
  state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ignored','resolved')),
  first_seen timestamptz NOT NULL,
  last_seen timestamptz NOT NULL,
  actor_id bigint,
  updated_at timestamptz NOT NULL DEFAULT now(),
  generation bigint NOT NULL DEFAULT 1,
  UNIQUE (rule, subject)
);
CREATE INDEX key_risk_events_latest ON key_risk_events(last_seen DESC, id DESC);

CREATE TABLE key_risk_outbox (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('age','risk')),
  bucket bigint NOT NULL,
  state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','sending','sent','failed','uncertain','skipped')),
  manual boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, kind, bucket)
);
CREATE INDEX key_risk_outbox_pending ON key_risk_outbox(available_at) WHERE state='pending';

CREATE TABLE key_risk_outbox_items (
  outbox_id bigint NOT NULL REFERENCES key_risk_outbox(id) ON DELETE CASCADE,
  event_id bigint NOT NULL,
  generation bigint NOT NULL,
  key_id bigint NOT NULL,
  evidence jsonb NOT NULL,
  PRIMARY KEY (outbox_id, event_id, generation, key_id)
);

CREATE TABLE key_risk_recipients (
  event_id bigint NOT NULL REFERENCES key_risk_events(id) ON DELETE CASCADE,
  generation bigint NOT NULL,
  user_id bigint NOT NULL,
  key_ids bigint[] NOT NULL,
  acknowledged_at timestamptz,
  acknowledgement jsonb,
  PRIMARY KEY (event_id, generation, user_id)
);
CREATE INDEX key_risk_recipients_user ON key_risk_recipients(user_id, event_id DESC);

CREATE TABLE key_risk_deliveries (
  outbox_id bigint NOT NULL,
  event_id bigint NOT NULL,
  generation bigint NOT NULL,
  user_id bigint NOT NULL,
  key_ids bigint[] NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now(),
  message_version integer NOT NULL DEFAULT 1,
  state text NOT NULL DEFAULT 'sending' CHECK (state IN ('sending','sent','uncertain')),
  PRIMARY KEY (outbox_id, event_id, generation, user_id),
  FOREIGN KEY (event_id, generation, user_id)
    REFERENCES key_risk_recipients(event_id, generation, user_id) ON DELETE CASCADE
);
CREATE INDEX key_risk_deliveries_recipient ON key_risk_deliveries(event_id, generation, user_id);
CREATE INDEX key_risk_deliveries_user_time ON key_risk_deliveries(user_id, attempted_at DESC);
