-- Explicit, one-time retrofit for the two subscription orders that were paid
-- before package caps were configured.
--
-- The order IDs and expected users/plan are intentionally hard-coded. This
-- prevents a broad or accidental historical rewrite. The current plan row is
-- only used as a guarded assertion; its values are copied from this explicit
-- policy, never discovered dynamically.
--
-- Usage (dry-run is the default):
--   psql "$DATABASE_URL" -v dry_run=true  -f backend/scripts/retrofit-package-entitlements.sql
--   psql "$DATABASE_URL" -v dry_run=false -f backend/scripts/retrofit-package-entitlements.sql
--
-- The production database has a unique payment_audit_logs(order_id, action)
-- index (created by migration 131). The entitlement table also has a unique
-- payment_order_id constraint, so rerunning the script is idempotent.

\set ON_ERROR_STOP on
\if :{?dry_run}
\else
  \set dry_run true
\endif

BEGIN;
SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;

CREATE TEMP TABLE retrofit_expected (
    order_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    plan_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    request_limit BIGINT NOT NULL,
    amount_limit_usd NUMERIC(20,10) NOT NULL,
    token_limit BIGINT NOT NULL,
    validity_days INTEGER NOT NULL
) ON COMMIT DROP;

INSERT INTO retrofit_expected VALUES
    (331, 451, 21, 62, 44, 12000, 700, 1000000000, 30),
    (332, 767, 21, 62, 45, 12000, 700, 1000000000, 30);

CREATE TEMP TABLE retrofit_candidates ON COMMIT DROP AS
SELECT
    e.order_id,
    e.user_id,
    e.plan_id,
    e.group_id,
    e.subscription_id,
    e.request_limit,
    e.amount_limit_usd,
    e.token_limit,
    e.validity_days,
    po.status AS order_status,
    po.order_type,
    po.paid_at,
    po.completed_at,
    po.subscription_snapshot,
    sp.request_limit AS current_request_limit,
    sp.amount_limit_usd AS current_amount_limit_usd,
    sp.token_limit AS current_token_limit,
    sp.validity_days AS current_validity_days,
    sp.validity_unit AS current_validity_unit,
    us.starts_at,
    us.expires_at,
    us.status AS subscription_status,
    COALESCE(usage.request_used, 0)::BIGINT AS request_used,
    COALESCE(usage.amount_used_usd, 0)::NUMERIC(20,10) AS amount_used_usd,
    COALESCE(usage.token_used, 0)::BIGINT AS token_used,
    pe.id AS existing_entitlement_id,
    pe.request_limit AS existing_request_limit,
    pe.amount_limit_usd AS existing_amount_limit_usd,
    pe.token_limit AS existing_token_limit,
    pe.user_id AS existing_user_id,
    pe.group_id AS existing_group_id,
    pe.starts_at AS existing_starts_at,
    pe.expires_at AS existing_expires_at
FROM retrofit_expected e
LEFT JOIN payment_orders po ON po.id = e.order_id
LEFT JOIN subscription_plans sp ON sp.id = e.plan_id
LEFT JOIN user_subscriptions us
       ON us.id = e.subscription_id
      AND us.user_id = e.user_id
      AND us.group_id = e.group_id
      AND us.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT
        COUNT(*)::BIGINT AS request_used,
        COALESCE(SUM(COALESCE(ul.billed_cost, ul.total_cost, 0)), 0)::NUMERIC(20,10) AS amount_used_usd,
        COALESCE(SUM(
            COALESCE(ul.input_tokens, 0)::BIGINT
          + COALESCE(ul.output_tokens, 0)::BIGINT
          + COALESCE(ul.cache_creation_tokens, 0)::BIGINT
          + COALESCE(ul.cache_read_tokens, 0)::BIGINT
        ), 0)::BIGINT AS token_used
    FROM usage_logs ul
    WHERE ul.user_id = e.user_id
      AND ul.group_id = e.group_id
      AND us.starts_at IS NOT NULL
      AND ul.created_at >= us.starts_at
      AND ul.created_at < LEAST(us.expires_at, NOW())
) usage ON TRUE
LEFT JOIN subscription_package_entitlements pe ON pe.payment_order_id = e.order_id;

-- Every guard is evaluated before any production row can be inserted.
DO $$
DECLARE
    bad_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO bad_count FROM retrofit_candidates;
    IF bad_count <> 2 THEN
        RAISE EXCEPTION 'retrofit expected exactly 2 candidate rows, got %', bad_count;
    END IF;

    IF EXISTS (
        SELECT 1 FROM retrofit_candidates
        WHERE existing_entitlement_id IS NULL
          AND (
              order_status IS DISTINCT FROM 'COMPLETED'
           OR order_type IS DISTINCT FROM 'subscription'
           OR current_request_limit IS DISTINCT FROM request_limit
           OR current_amount_limit_usd IS DISTINCT FROM amount_limit_usd
           OR current_token_limit IS DISTINCT FROM token_limit
           OR current_validity_days IS DISTINCT FROM validity_days
           OR current_validity_unit IS DISTINCT FROM 'days'
           OR starts_at IS NULL
           OR expires_at IS NULL
           OR subscription_status IS DISTINCT FROM 'active'
          )
    ) THEN
        RAISE EXCEPTION 'retrofit order, plan, or subscription guard failed';
    END IF;

    IF EXISTS (
        SELECT 1 FROM retrofit_candidates
        WHERE existing_entitlement_id IS NOT NULL
          AND (
              existing_request_limit IS DISTINCT FROM request_limit
           OR existing_amount_limit_usd IS DISTINCT FROM amount_limit_usd
           OR existing_token_limit IS DISTINCT FROM token_limit
           OR existing_user_id IS DISTINCT FROM user_id
           OR existing_group_id IS DISTINCT FROM group_id
           OR existing_starts_at IS DISTINCT FROM starts_at
           OR existing_expires_at IS DISTINCT FROM expires_at
          )
    ) THEN
        RAISE EXCEPTION 'existing entitlement does not match the approved retrofit policy';
    END IF;
END $$;

SELECT
    order_id,
    user_id,
    plan_id,
    group_id,
    request_limit,
    amount_limit_usd,
    token_limit,
    request_used,
    amount_used_usd,
    token_used,
    CASE
        WHEN expires_at <= NOW() THEN 'expired'
        WHEN request_used >= request_limit THEN 'exhausted'
        WHEN amount_used_usd >= amount_limit_usd THEN 'exhausted'
        WHEN token_used >= token_limit THEN 'exhausted'
        ELSE 'active'
    END AS resulting_status,
    existing_entitlement_id,
    CASE WHEN existing_entitlement_id IS NULL THEN 'insert' ELSE 'skip_existing' END AS action
FROM retrofit_candidates
ORDER BY order_id;

\if :dry_run
    ROLLBACK;
    SELECT 'dry-run: no production rows changed' AS result;
\else
    INSERT INTO subscription_package_entitlements (
        payment_order_id,
        user_id,
        group_id,
        starts_at,
        expires_at,
        status,
        exhausted_reason,
        request_limit,
        request_used,
        amount_limit_usd,
        amount_used_usd,
        token_limit,
        token_used,
        plan_snapshot
    )
    SELECT
        c.order_id,
        c.user_id,
        c.group_id,
        c.starts_at,
        c.expires_at,
        CASE
            WHEN c.expires_at <= NOW() THEN 'expired'
            WHEN c.request_used >= c.request_limit THEN 'exhausted'
            WHEN c.amount_used_usd >= c.amount_limit_usd THEN 'exhausted'
            WHEN c.token_used >= c.token_limit THEN 'exhausted'
            ELSE 'active'
        END,
        CASE
            WHEN c.expires_at <= NOW() THEN NULL
            WHEN c.request_used >= c.request_limit THEN 'request'
            WHEN c.amount_used_usd >= c.amount_limit_usd THEN 'amount'
            WHEN c.token_used >= c.token_limit THEN 'token'
            ELSE NULL
        END,
        c.request_limit,
        c.request_used,
        c.amount_limit_usd,
        c.amount_used_usd,
        c.token_limit,
        c.token_used,
        jsonb_build_object(
            'schema_version', 1,
            'retrofit', true,
            'retrofit_reason', 'explicit_user_authorization',
            'order_id', c.order_id,
            'plan_id', c.plan_id,
            'group_id', c.group_id,
            'validity_days', c.validity_days,
            'request_limit', c.request_limit,
            'amount_limit_usd', c.amount_limit_usd,
            'token_limit', c.token_limit
        )
    FROM retrofit_candidates c
    WHERE c.existing_entitlement_id IS NULL
    ON CONFLICT (payment_order_id) DO NOTHING;

    INSERT INTO payment_audit_logs (order_id, action, detail, operator)
    SELECT
        c.order_id::TEXT,
        'PACKAGE_QUOTA_RETROFIT',
        jsonb_build_object(
            'policy_version', 'package-quota-retrofit-v1',
            'user_id', c.user_id,
            'plan_id', c.plan_id,
            'group_id', c.group_id,
            'request_limit', c.request_limit,
            'amount_limit_usd', c.amount_limit_usd,
            'token_limit', c.token_limit,
            'request_used', c.request_used,
            'amount_used_usd', c.amount_used_usd,
            'token_used', c.token_used
        )::TEXT,
        'system:package-quota-retrofit'
    FROM retrofit_candidates c
    ON CONFLICT (order_id, action) DO NOTHING;

    COMMIT;
    SELECT 'execute: package entitlements and audit logs committed' AS result;
\endif
