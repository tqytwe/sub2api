-- Daily cards are immutable, one-time quota entitlements. They must not reuse
-- calendar-day subscription windows or infer accounting policy from labels.

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS quota_mode VARCHAR(24) NOT NULL DEFAULT 'recurring';
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS quota_limit_usd NUMERIC(20,10);
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS duration_hours INTEGER;

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS subscription_snapshot JSONB;

CREATE TABLE IF NOT EXISTS subscription_entitlements (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    plan_id BIGINT REFERENCES subscription_plans(id) ON DELETE SET NULL,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    quota_mode VARCHAR(24) NOT NULL,
    quota_limit_usd NUMERIC(20,10) NOT NULL,
    quota_used_usd NUMERIC(20,10) NOT NULL DEFAULT 0,
    quota_reserved_usd NUMERIC(20,10) NOT NULL DEFAULT 0,
    duration_hours INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    starts_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    exhausted_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_entitlements_status_check
        CHECK (status IN ('pending', 'active', 'exhausted', 'expired', 'revoked')),
    CONSTRAINT subscription_entitlements_quota_mode_check
        CHECK (quota_mode IN ('one_time')),
    CONSTRAINT subscription_entitlements_quota_check
        CHECK (
            quota_limit_usd > 0
            AND quota_used_usd >= 0
            AND quota_reserved_usd >= 0
            AND quota_used_usd <= quota_limit_usd
            AND quota_used_usd + quota_reserved_usd <= quota_limit_usd
        ),
    CONSTRAINT subscription_entitlements_duration_check CHECK (duration_hours > 0),
    CONSTRAINT subscription_entitlements_active_time_check CHECK (
        status <> 'active' OR (
            starts_at IS NOT NULL
            AND expires_at IS NOT NULL
            AND activated_at IS NOT NULL
        )
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlements_order
    ON subscription_entitlements(payment_order_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlements_one_active
    ON subscription_entitlements(user_id, group_id)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_subscription_entitlements_pending_queue
    ON subscription_entitlements(user_id, group_id, created_at, id)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_subscription_entitlements_expiry
    ON subscription_entitlements(expires_at)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS subscription_entitlement_holds (
    id BIGSERIAL PRIMARY KEY,
    entitlement_id BIGINT NOT NULL REFERENCES subscription_entitlements(id) ON DELETE RESTRICT,
    request_id VARCHAR(200) NOT NULL,
    request_fingerprint VARCHAR(128) NOT NULL,
    reserved_usd NUMERIC(20,10) NOT NULL,
    captured_usd NUMERIC(20,10) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'reserved',
    expires_at TIMESTAMPTZ NOT NULL,
    captured_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_entitlement_holds_status_check
        CHECK (status IN ('reserved', 'captured', 'released')),
    CONSTRAINT subscription_entitlement_holds_amount_check
        CHECK (reserved_usd > 0 AND captured_usd >= 0 AND captured_usd <= reserved_usd)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlement_holds_request
    ON subscription_entitlement_holds(entitlement_id, request_id);
CREATE INDEX IF NOT EXISTS idx_subscription_entitlement_holds_stale
    ON subscription_entitlement_holds(expires_at)
    WHERE status = 'reserved';

-- Backfill only plans that are already explicit daily products. Names are not
-- used because they are mutable and ambiguous. Other plans remain recurring
-- until an administrator opts them in.
UPDATE subscription_plans AS sp
SET quota_mode = 'one_time',
    quota_limit_usd = g.daily_limit_usd,
    duration_hours = 24,
    updated_at = NOW()
FROM groups AS g
WHERE sp.group_id = g.id
  AND sp.storefront_category = 'daily'
  AND sp.validity_days = 1
  AND LOWER(sp.validity_unit) IN ('day', 'days')
  AND g.daily_limit_usd > 0
  AND sp.quota_mode = 'recurring'
  AND sp.quota_limit_usd IS NULL
  AND sp.duration_hours IS NULL;

-- Historical orders receive the same immutable snapshot used by new orders.
UPDATE payment_orders AS po
SET subscription_snapshot = jsonb_build_object(
        'schema_version', 1,
        'quota_mode', sp.quota_mode,
        'quota_limit_usd', sp.quota_limit_usd,
        'duration_hours', sp.duration_hours,
        'group_id', sp.group_id,
        'validity_days', sp.validity_days,
        'validity_unit', sp.validity_unit
    ),
    updated_at = NOW()
FROM subscription_plans AS sp
WHERE po.plan_id = sp.id
  AND po.order_type = 'subscription'
  AND po.status = 'COMPLETED'
  AND sp.quota_mode = 'one_time'
  AND (po.subscription_snapshot IS NULL OR po.subscription_snapshot = '{}'::jsonb);

-- Reconstruct stacked purchases in payment order. The first card starts at its
-- payment time; each later card starts after the prior 24-hour slot unless it
-- was purchased later. Pending rows deliberately keep start/expiry NULL so the
-- runtime can activate them at the exact earlier-of quota/time terminal event.
WITH RECURSIVE eligible_orders AS (
    SELECT
        po.id AS payment_order_id,
        po.user_id,
        sp.group_id,
        sp.id AS plan_id,
        sp.quota_limit_usd,
        sp.duration_hours,
        COALESCE(po.paid_at, po.completed_at, po.created_at) AS paid_at,
        ROW_NUMBER() OVER (
            PARTITION BY po.user_id, sp.group_id
            ORDER BY COALESCE(po.paid_at, po.completed_at, po.created_at), po.id
        ) AS queue_number
    FROM payment_orders AS po
    JOIN subscription_plans AS sp ON sp.id = po.plan_id
    WHERE po.order_type = 'subscription'
      AND po.status = 'COMPLETED'
      AND sp.quota_mode = 'one_time'
      AND sp.quota_limit_usd > 0
      AND sp.duration_hours > 0
      AND NOT EXISTS (
          SELECT 1 FROM subscription_entitlements AS existing
          WHERE existing.payment_order_id = po.id
      )
), timeline AS (
    SELECT
        eo.*,
        eo.paid_at AS starts_at
    FROM eligible_orders AS eo
    WHERE eo.queue_number = 1

    UNION ALL

    SELECT
        next_order.*,
        GREATEST(
            next_order.paid_at,
            previous.starts_at + make_interval(hours => previous.duration_hours)
        ) AS starts_at
    FROM timeline AS previous
    JOIN eligible_orders AS next_order
      ON next_order.user_id = previous.user_id
     AND next_order.group_id = previous.group_id
     AND next_order.queue_number = previous.queue_number + 1
), prepared AS (
    SELECT
        timeline.*,
        timeline.starts_at + make_interval(hours => timeline.duration_hours) AS ends_at,
        LEAST(
            timeline.quota_limit_usd,
            GREATEST(COALESCE(us.daily_usage_usd, 0), COALESCE(log_usage.used_usd, 0))
        ) AS preserved_used_usd
    FROM timeline
    LEFT JOIN LATERAL (
        SELECT active_sub.id, active_sub.daily_usage_usd
        FROM user_subscriptions AS active_sub
        WHERE active_sub.user_id = timeline.user_id
          AND active_sub.group_id = timeline.group_id
          AND active_sub.status = 'active'
          AND active_sub.deleted_at IS NULL
        ORDER BY active_sub.updated_at DESC, active_sub.id DESC
        LIMIT 1
    ) AS us ON TRUE
    LEFT JOIN LATERAL (
        SELECT SUM(
            CASE
                WHEN usage.billed_cost > 0 THEN usage.billed_cost
                ELSE usage.actual_cost
            END
        ) AS used_usd
        FROM usage_logs AS usage
        WHERE usage.subscription_id = us.id
          AND usage.created_at >= timeline.starts_at
          AND usage.created_at < LEAST(
              timeline.starts_at + make_interval(hours => timeline.duration_hours),
              NOW()
          )
    ) AS log_usage ON TRUE
)
INSERT INTO subscription_entitlements (
    user_id, group_id, plan_id, payment_order_id, quota_mode,
    quota_limit_usd, quota_used_usd, quota_reserved_usd, duration_hours,
    status, starts_at, expires_at, activated_at, exhausted_at, ended_at,
    created_at, updated_at
)
SELECT
    prepared.user_id,
    prepared.group_id,
    prepared.plan_id,
    prepared.payment_order_id,
    'one_time',
    prepared.quota_limit_usd,
    CASE
        WHEN prepared.starts_at <= NOW() AND prepared.ends_at > NOW()
            THEN prepared.preserved_used_usd
        ELSE 0
    END,
    0,
    prepared.duration_hours,
    CASE
        WHEN prepared.ends_at <= NOW() THEN 'expired'
        WHEN prepared.starts_at <= NOW() AND prepared.preserved_used_usd >= prepared.quota_limit_usd THEN 'exhausted'
        WHEN prepared.starts_at <= NOW() THEN 'active'
        ELSE 'pending'
    END,
    CASE WHEN prepared.starts_at <= NOW() THEN prepared.starts_at ELSE NULL END,
    CASE WHEN prepared.starts_at <= NOW() THEN prepared.ends_at ELSE NULL END,
    CASE WHEN prepared.starts_at <= NOW() THEN prepared.starts_at ELSE NULL END,
    CASE
        WHEN prepared.starts_at <= NOW() AND prepared.ends_at > NOW()
         AND prepared.preserved_used_usd >= prepared.quota_limit_usd THEN NOW()
        ELSE NULL
    END,
    CASE
        WHEN prepared.ends_at <= NOW() THEN prepared.ends_at
        WHEN prepared.starts_at <= NOW() AND prepared.preserved_used_usd >= prepared.quota_limit_usd THEN NOW()
        ELSE NULL
    END,
    prepared.paid_at,
    NOW()
FROM prepared
ON CONFLICT (payment_order_id) DO NOTHING;

-- If the reconstructed current card was already exhausted, immediately start
-- the first queued card instead of waiting for the old parent subscription.
WITH next_pending AS (
    SELECT DISTINCT ON (pending.user_id, pending.group_id)
        pending.id,
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
WHERE entitlement.id = next_pending.id;
