-- Daily-card request replay protection is intentionally separate from billing.
-- client_request_id is correlation/replay input only; settlement_request_id is
-- the private key used by usage billing and entitlement settlement.
CREATE TABLE IF NOT EXISTS daily_card_request_replays (
    id BIGSERIAL PRIMARY KEY,
    entitlement_id BIGINT NOT NULL REFERENCES subscription_entitlements(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    client_request_id VARCHAR(200) NOT NULL,
    settlement_request_id VARCHAR(200) NOT NULL,
    request_fingerprint VARCHAR(128) NOT NULL,
    request_path VARCHAR(512) NOT NULL,
    state VARCHAR(32) NOT NULL,
    upstream_account_id BIGINT,
    upstream_request_id VARCHAR(255),
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    reconciled_at TIMESTAMPTZ,
    reconciled_by BIGINT,
    reconciliation_evidence TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_card_request_replays_state_check
        CHECK (state IN ('forwarding', 'completed', 'retryable', 'pending_confirmation')),
    CONSTRAINT daily_card_request_replays_user_group_client_unique
        UNIQUE (user_id, group_id, client_request_id),
    CONSTRAINT daily_card_request_replays_settlement_unique
        UNIQUE (settlement_request_id)
);

CREATE INDEX IF NOT EXISTS idx_daily_card_request_replays_pending
    ON daily_card_request_replays(user_id, group_id, state)
    WHERE state = 'pending_confirmation';

-- Only captured historical holds with corroborating billing evidence become
-- completed replays. Reserved/released holds are never terminal replay state.
INSERT INTO daily_card_request_replays (
    entitlement_id, user_id, group_id, client_request_id, settlement_request_id,
    request_fingerprint, request_path, state, completed_at, created_at, updated_at
)
SELECT hold.entitlement_id, entitlement.user_id, entitlement.group_id,
       substring(hold.request_id FROM 8),
       hold.request_id,
       hold.request_fingerprint,
       '',
       CASE WHEN EXISTS (
            SELECT 1 FROM usage_billing_dedup dedup WHERE dedup.request_id = hold.request_id
       ) OR EXISTS (
            SELECT 1 FROM usage_logs log WHERE log.request_id = hold.request_id
       ) THEN 'completed' ELSE 'pending_confirmation' END,
       CASE WHEN EXISTS (
            SELECT 1 FROM usage_billing_dedup dedup WHERE dedup.request_id = hold.request_id
       ) OR EXISTS (
            SELECT 1 FROM usage_logs log WHERE log.request_id = hold.request_id
       ) THEN COALESCE(hold.captured_at, hold.updated_at) ELSE NULL END,
       hold.created_at,
       hold.updated_at
FROM subscription_entitlement_holds hold
JOIN subscription_entitlements entitlement ON entitlement.id = hold.entitlement_id
WHERE hold.status = 'captured'
  AND hold.request_id LIKE 'client:%'
ON CONFLICT (user_id, group_id, client_request_id) DO NOTHING;

-- Retire the zero-dollar reservation mechanism without altering used quota.
UPDATE subscription_entitlement_holds
SET status = 'released', released_at = COALESCE(released_at, NOW()), updated_at = NOW()
WHERE status = 'reserved';

UPDATE subscription_entitlements
SET quota_reserved_usd = 0, updated_at = NOW()
WHERE quota_reserved_usd <> 0;
