-- Reconcile cards that continued to display legacy parent usage after the
-- entitlement rollout. The migration is idempotent and never restores quota.

LOCK TABLE subscription_entitlements IN SHARE ROW EXCLUSIVE MODE;

UPDATE subscription_entitlements
SET quota_reserved_usd = 0,
    updated_at = NOW()
WHERE quota_reserved_usd <> 0
  AND quota_mode = 'one_time';

UPDATE subscription_entitlements
SET status = 'expired',
    quota_reserved_usd = 0,
    ended_at = expires_at,
    updated_at = NOW()
WHERE status = 'active'
  AND expires_at <= NOW();

WITH reconciled AS (
    SELECT
        entitlement.id,
        LEAST(
            entitlement.quota_limit_usd,
            GREATEST(
                entitlement.quota_used_usd,
                CASE
                    WHEN NOT EXISTS (
                        SELECT 1
                        FROM subscription_entitlements AS prior
                        WHERE prior.user_id = entitlement.user_id
                          AND prior.group_id = entitlement.group_id
                          AND (prior.created_at, prior.id) < (entitlement.created_at, entitlement.id)
                    ) THEN COALESCE(subscription.daily_usage_usd, 0)
                    ELSE 0
                END,
                COALESCE(log_usage.used_usd, 0)
            )
        ) AS used_usd,
        COALESCE(log_usage.last_usage_at, NOW()) AS terminal_at
    FROM subscription_entitlements AS entitlement
    LEFT JOIN LATERAL (
        SELECT active_subscription.id, active_subscription.daily_usage_usd
        FROM user_subscriptions AS active_subscription
        WHERE active_subscription.user_id = entitlement.user_id
          AND active_subscription.group_id = entitlement.group_id
          AND active_subscription.deleted_at IS NULL
        ORDER BY active_subscription.updated_at DESC, active_subscription.id DESC
        LIMIT 1
    ) AS subscription ON TRUE
    LEFT JOIN LATERAL (
        SELECT
            SUM(CASE WHEN usage.billed_cost > 0 THEN usage.billed_cost ELSE usage.actual_cost END) AS used_usd,
            MAX(usage.created_at) AS last_usage_at
        FROM usage_logs AS usage
        WHERE usage.subscription_id = subscription.id
          AND usage.created_at >= entitlement.starts_at
          AND usage.created_at < LEAST(entitlement.expires_at, NOW())
    ) AS log_usage ON TRUE
    WHERE entitlement.status = 'active'
      AND entitlement.expires_at > NOW()
), exhausted AS (
    UPDATE subscription_entitlements AS entitlement
    SET quota_used_usd = reconciled.used_usd,
        status = CASE
            WHEN reconciled.used_usd >= entitlement.quota_limit_usd THEN 'exhausted'
            ELSE entitlement.status
        END,
        exhausted_at = CASE
            WHEN reconciled.used_usd >= entitlement.quota_limit_usd
                THEN COALESCE(entitlement.exhausted_at, reconciled.terminal_at)
            ELSE entitlement.exhausted_at
        END,
        ended_at = CASE
            WHEN reconciled.used_usd >= entitlement.quota_limit_usd
                THEN COALESCE(entitlement.ended_at, reconciled.terminal_at)
            ELSE entitlement.ended_at
        END,
        updated_at = NOW()
    FROM reconciled
    WHERE entitlement.id = reconciled.id
      AND reconciled.used_usd >= entitlement.quota_used_usd
    RETURNING entitlement.user_id, entitlement.group_id
), next_pending AS (
    SELECT DISTINCT ON (pending.user_id, pending.group_id)
        pending.id,
        pending.user_id,
        pending.group_id,
        pending.duration_hours
    FROM subscription_entitlements AS pending
    WHERE pending.status = 'pending'
      AND NOT EXISTS (
          SELECT 1
          FROM subscription_entitlements AS active
          WHERE active.user_id = pending.user_id
            AND active.group_id = pending.group_id
            AND active.status = 'active'
      )
    ORDER BY pending.user_id, pending.group_id, pending.created_at, pending.id
)
UPDATE subscription_entitlements AS entitlement
SET status = 'active',
    starts_at = NOW(),
    activated_at = NOW(),
    expires_at = NOW() + make_interval(hours => next_pending.duration_hours),
    updated_at = NOW()
FROM next_pending
WHERE entitlement.id = next_pending.id
  AND entitlement.status = 'pending';
