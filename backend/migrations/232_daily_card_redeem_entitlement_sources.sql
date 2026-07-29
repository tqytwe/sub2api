-- Daily-card entitlements are the authoritative quota ledger. Payment orders
-- remain idempotent by payment_order_id; redeem/admin/backfill grants use an
-- explicit source key so subscription redeem codes can issue real daily cards.

ALTER TABLE subscription_entitlements
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(32) NOT NULL DEFAULT 'payment_order';
ALTER TABLE subscription_entitlements
    ADD COLUMN IF NOT EXISTS source_id VARCHAR(128) NOT NULL DEFAULT '';

UPDATE subscription_entitlements
SET source_type = 'payment_order',
    source_id = payment_order_id::text,
    updated_at = NOW()
WHERE payment_order_id IS NOT NULL
  AND (source_type IS DISTINCT FROM 'payment_order' OR source_id = '');

ALTER TABLE subscription_entitlements
    ALTER COLUMN payment_order_id DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlements_source
    ON subscription_entitlements(source_type, source_id)
    WHERE source_id <> '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'subscription_entitlements_source_check'
    ) THEN
        ALTER TABLE subscription_entitlements
            ADD CONSTRAINT subscription_entitlements_source_check
            CHECK (
                source_type IN ('payment_order', 'redeem_code', 'admin_grant', 'backfill')
                AND (
                    (source_type = 'payment_order' AND payment_order_id IS NOT NULL)
                    OR (source_type <> 'payment_order' AND source_id <> '')
                )
            ) NOT VALID;
    END IF;
END $$;

-- Historical safety net: only active one-time daily-card subscriptions whose
-- current subscription window has no entitlement ledger receive a backfilled
-- card. This does not rewrite balances, usage logs, or ordinary recurring
-- subscription windows.
WITH canonical_daily_plan AS (
    SELECT DISTINCT ON (sp.group_id)
        sp.id AS plan_id,
        sp.group_id,
        sp.quota_limit_usd,
        sp.duration_hours
    FROM subscription_plans AS sp
    WHERE sp.quota_mode = 'one_time'
      AND sp.quota_limit_usd > 0
      AND sp.duration_hours > 0
    ORDER BY sp.group_id, sp.for_sale DESC, sp.sort_order ASC, sp.id ASC
), missing_daily_cards AS (
    SELECT
        us.id AS subscription_id,
        us.user_id,
        us.group_id,
        plan.plan_id,
        plan.quota_limit_usd,
        plan.duration_hours,
        us.starts_at,
        us.expires_at
    FROM user_subscriptions AS us
    JOIN canonical_daily_plan AS plan ON plan.group_id = us.group_id
    WHERE us.status = 'active'
      AND us.deleted_at IS NULL
      AND us.expires_at > NOW()
      AND NOT EXISTS (
          SELECT 1
          FROM subscription_entitlements AS existing
          WHERE existing.user_id = us.user_id
            AND existing.group_id = us.group_id
            AND (
                existing.status IN ('active', 'pending')
                OR existing.created_at >= us.starts_at
                OR (
                    existing.starts_at IS NOT NULL
                    AND COALESCE(existing.expires_at, existing.ended_at, existing.starts_at) > us.starts_at
                    AND existing.starts_at < us.expires_at
                )
            )
      )
)
INSERT INTO subscription_entitlements (
    user_id, group_id, plan_id, payment_order_id, source_type, source_id,
    quota_mode, quota_limit_usd, quota_used_usd, quota_reserved_usd,
    duration_hours, status, starts_at, expires_at, activated_at,
    exhausted_at, ended_at, created_at, updated_at
)
SELECT
    user_id,
    group_id,
    plan_id,
    NULL,
    'backfill',
    'user_subscription:' || subscription_id::text,
    'one_time',
    quota_limit_usd,
    0,
    0,
    duration_hours,
    'active',
    starts_at,
    expires_at,
    starts_at,
    NULL,
    NULL,
    starts_at,
    NOW()
FROM missing_daily_cards
ON CONFLICT DO NOTHING;
