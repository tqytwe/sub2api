-- Restore daily-card accounting to the upstream one-day subscription model.
-- Older installations may still have the fork-only entitlement tables; fresh
-- installations do not, so this migration is intentionally a no-op there.
DO $$
BEGIN
    IF to_regclass('public.subscription_entitlements') IS NULL
       OR to_regclass('public.user_subscriptions') IS NULL THEN
        RETURN;
    END IF;

    -- Keep the authoritative usage already present on the subscription. The
    -- entitlement value is only a compatibility backfill for the one active
    -- one-day subscription and can never reduce recorded usage.
    EXECUTE $sql$
        UPDATE user_subscriptions AS subscription
        SET daily_usage_usd = GREATEST(
                subscription.daily_usage_usd,
                entitlement.quota_used_usd
            ),
            daily_window_start = COALESCE(
                subscription.daily_window_start,
                entitlement.starts_at
            ),
            updated_at = NOW()
        FROM subscription_entitlements AS entitlement
        WHERE entitlement.user_id = subscription.user_id
          AND entitlement.group_id = subscription.group_id
          AND entitlement.status = 'active'
          AND subscription.status = 'active'
          AND subscription.expires_at <= subscription.starts_at + INTERVAL '1 day'
          AND entitlement.quota_used_usd >= 0
    $sql$;
END
$$;
