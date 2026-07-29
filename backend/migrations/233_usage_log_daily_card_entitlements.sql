ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS subscription_entitlement_id BIGINT
    REFERENCES subscription_entitlements(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_subscription_entitlement_id
    ON usage_logs(subscription_entitlement_id);

CREATE INDEX IF NOT EXISTS idx_usage_logs_subscription_entitlement_created
    ON usage_logs(subscription_entitlement_id, created_at)
    WHERE subscription_entitlement_id IS NOT NULL;

WITH candidates AS (
    SELECT
        usage.id AS usage_log_id,
        entitlement.id AS entitlement_id,
        COUNT(*) OVER (PARTITION BY usage.id) AS match_count
    FROM usage_logs AS usage
    JOIN user_subscriptions AS subscription
        ON subscription.id = usage.subscription_id
    JOIN subscription_entitlements AS entitlement
        ON entitlement.user_id = usage.user_id
       AND entitlement.group_id = subscription.group_id
       AND usage.created_at >= COALESCE(entitlement.starts_at, subscription.starts_at)
       AND (
            entitlement.expires_at IS NULL
            OR usage.created_at < entitlement.expires_at
       )
    WHERE usage.subscription_entitlement_id IS NULL
)
UPDATE usage_logs AS usage
SET subscription_entitlement_id = candidates.entitlement_id
FROM candidates
WHERE usage.id = candidates.usage_log_id
  AND candidates.match_count = 1;

WITH entitlement_usage AS (
    SELECT
        usage.subscription_entitlement_id AS entitlement_id,
        SUM(
            CASE
                WHEN usage.billed_cost > 0 THEN usage.billed_cost
                ELSE usage.actual_cost
            END
        ) AS used_usd
    FROM usage_logs AS usage
    WHERE usage.subscription_entitlement_id IS NOT NULL
      AND usage.billing_type = 1
    GROUP BY usage.subscription_entitlement_id
),
reconciled AS (
    SELECT
        entitlement.id,
        entitlement.quota_limit_usd,
        GREATEST(entitlement.quota_used_usd, entitlement_usage.used_usd) AS used_usd
    FROM subscription_entitlements AS entitlement
    JOIN entitlement_usage
        ON entitlement_usage.entitlement_id = entitlement.id
)
UPDATE subscription_entitlements AS entitlement
SET quota_used_usd = LEAST(reconciled.used_usd, entitlement.quota_limit_usd),
    status = CASE
        WHEN entitlement.status = 'active'
         AND LEAST(reconciled.used_usd, entitlement.quota_limit_usd) >= entitlement.quota_limit_usd
            THEN 'exhausted'
        ELSE entitlement.status
    END,
    exhausted_at = CASE
        WHEN entitlement.status = 'active'
         AND LEAST(reconciled.used_usd, entitlement.quota_limit_usd) >= entitlement.quota_limit_usd
            THEN COALESCE(entitlement.exhausted_at, NOW())
        ELSE entitlement.exhausted_at
    END,
    ended_at = CASE
        WHEN entitlement.status = 'active'
         AND LEAST(reconciled.used_usd, entitlement.quota_limit_usd) >= entitlement.quota_limit_usd
            THEN COALESCE(entitlement.ended_at, NOW())
        ELSE entitlement.ended_at
    END,
    updated_at = NOW()
FROM reconciled
WHERE entitlement.id = reconciled.id
  AND reconciled.used_usd > entitlement.quota_used_usd;

UPDATE subscription_entitlement_holds AS hold
SET status = 'captured',
    captured_usd = 0,
    captured_at = COALESCE(hold.captured_at, usage.created_at),
    updated_at = NOW()
FROM usage_logs AS usage
WHERE hold.entitlement_id = usage.subscription_entitlement_id
  AND hold.request_id = usage.request_id
  AND hold.status = 'reserved';
