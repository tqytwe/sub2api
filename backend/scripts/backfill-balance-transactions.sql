-- Backfill unified balance_transactions from legacy balance sources.
--
-- Usage:
--   psql "$DATABASE_URL" -v dry_run=true  -v batch_size=5000 -f backend/scripts/backfill-balance-transactions.sql
--   psql "$DATABASE_URL" -v dry_run=false -v batch_size=5000 -f backend/scripts/backfill-balance-transactions.sql
--
-- Notes:
--   - This script is idempotent through balance_transactions(user_id, idempotency_key).
--   - Historical balance_before/balance_after and frozen snapshots stay NULL unless the source
--     has a reliable snapshot. Do not infer them from current balance.
--   - Run repeatedly with a batch_size until inserted_rows is 0.

\set ON_ERROR_STOP on
\if :{?dry_run}
\else
  \set dry_run true
\endif
\if :{?batch_size}
\else
  \set batch_size 5000
\endif

BEGIN;

WITH refund_audits AS (
    SELECT
        pal.*,
        ((regexp_match(
            COALESCE(pal.detail, ''),
            '"balanceDeducted"[[:space:]]*:[[:space:]]*"?(-?[0-9]+([.][0-9]+)?)"?'
        ))[1])::numeric AS balance_deducted
    FROM payment_audit_logs pal
    WHERE pal.action = 'REFUND_SUCCESS'
),
candidates AS (
    SELECT
        po.user_id,
        po.amount::numeric(20,8) AS balance_delta,
        0::numeric(20,8) AS frozen_delta,
        'payment_recharge'::varchar(64) AS source_type,
        po.id::text::varchar(128) AS source_id,
        ('payment_order:' || po.id)::varchar(160) AS idempotency_key,
        'system'::varchar(32) AS actor_type,
        NULL::bigint AS actor_user_id,
        '订单充值'::text AS description,
        jsonb_build_object(
            'order_id', po.id,
            'out_trade_no', po.out_trade_no,
            'payment_type', po.payment_type,
            'pay_amount', po.pay_amount,
            'recharge_code', po.recharge_code,
            'status', po.status
        ) AS metadata,
        TRUE AS is_backfilled,
        'high'::varchar(24) AS confidence,
        COALESCE(po.completed_at, po.paid_at, po.updated_at, po.created_at) AS created_at
    FROM payment_orders po
    WHERE po.order_type = 'balance'
      AND po.status = 'COMPLETED'
      AND po.amount <> 0

    UNION ALL

    SELECT
        rc.used_by AS user_id,
        rc.value::numeric(20,8) AS balance_delta,
        0::numeric(20,8) AS frozen_delta,
        CASE WHEN rc.type = 'admin_balance' THEN 'admin_balance' ELSE 'balance' END::varchar(64) AS source_type,
        rc.id::text::varchar(128) AS source_id,
        ('redeem_code:' || rc.id)::varchar(160) AS idempotency_key,
        CASE WHEN rc.type = 'admin_balance' THEN 'admin' ELSE 'user' END::varchar(32) AS actor_type,
        rc.used_by AS actor_user_id,
        CASE
            WHEN rc.type = 'admin_balance' AND rc.value < 0 THEN '管理员扣减余额'
            WHEN rc.type = 'admin_balance' THEN '管理员增加余额'
            ELSE '兑换码充值'
        END::text AS description,
        jsonb_build_object(
            'redeem_code_id', rc.id,
            'code', rc.code,
            'redeem_type', rc.type,
            'notes', rc.notes
        ) AS metadata,
        TRUE AS is_backfilled,
        'high'::varchar(24) AS confidence,
        COALESCE(rc.used_at, rc.created_at) AS created_at
    FROM redeem_codes rc
    WHERE rc.used_by IS NOT NULL
      AND rc.status = 'used'
      AND rc.type IN ('balance', 'admin_balance')
      AND rc.value <> 0
      AND NOT EXISTS (
          SELECT 1
          FROM payment_orders po
          WHERE po.user_id = rc.used_by
            AND po.order_type = 'balance'
            AND po.status = 'COMPLETED'
            AND po.recharge_code = rc.code
      )

    UNION ALL

    SELECT
        ual.user_id,
        ual.amount::numeric(20,8),
        0::numeric(20,8),
        'affiliate_balance'::varchar(64),
        ual.id::text::varchar(128),
        ('affiliate_transfer:' || ual.id)::varchar(160),
        'user'::varchar(32),
        ual.user_id,
        '返利余额转入'::text,
        jsonb_build_object(
            'affiliate_ledger_id', ual.id,
            'action', ual.action,
            'source_order_id', ual.source_order_id,
            'balance_after', ual.balance_after,
            'aff_quota_after', ual.aff_quota_after,
            'aff_frozen_quota_after', ual.aff_frozen_quota_after,
            'aff_history_quota_after', ual.aff_history_quota_after
        ),
        TRUE,
        'high'::varchar(24),
        ual.created_at
    FROM user_affiliate_ledger ual
    WHERE ual.action = 'transfer'
      AND ual.amount <> 0

    UNION ALL

    SELECT
        prl.user_id,
        prl.amount::numeric(20,8),
        0::numeric(20,8),
        prl.source::varchar(64),
        prl.id::text::varchar(128),
        ('play_reward:' || prl.id)::varchar(160),
        'system'::varchar(32),
        NULL::bigint,
        CASE prl.source
            WHEN 'checkin' THEN '签到奖励'
            WHEN 'checkin_makeup' THEN '补签奖励'
            WHEN 'quiz' THEN '答题奖励'
            WHEN 'blindbox' THEN '盲盒净变动'
            WHEN 'arena_settlement' THEN '竞技场结算'
            WHEN 'arena_daily_settlement' THEN '日榜竞技场结算'
            WHEN 'team_shared_reward' THEN '组队共享奖励'
            ELSE '玩法奖励'
        END::text,
        COALESCE(prl.detail, '{}'::jsonb) ||
            jsonb_strip_nulls(jsonb_build_object(
                'play_reward_ledger_id', prl.id,
                'idempotency_key', prl.idempotency_key,
                'blindbox_open_id', b.id,
                'cost_amount', b.cost_amount,
                'reward_amount', b.reward_amount,
                'net_amount', CASE WHEN b.id IS NOT NULL THEN b.reward_amount - b.cost_amount ELSE NULL END
            )),
        TRUE,
        'high'::varchar(24),
        prl.created_at
    FROM play_reward_ledger prl
    LEFT JOIN play_blindbox_opens b
      ON b.user_id = prl.user_id
     AND b.idempotency_key = prl.idempotency_key
    WHERE prl.source <> 'team_affiliate_bonus'
      AND prl.amount <> 0

    UNION ALL

    SELECT
        b.user_id,
        (b.reward_amount - b.cost_amount)::numeric(20,8),
        0::numeric(20,8),
        'blindbox'::varchar(64),
        b.id::text::varchar(128),
        ('blindbox_open_orphan:' || b.id)::varchar(160),
        'system'::varchar(32),
        NULL::bigint,
        '盲盒净变动'::text,
        jsonb_build_object(
            'blindbox_open_id', b.id,
            'open_date', b.open_date,
            'cost_amount', b.cost_amount,
            'reward_amount', b.reward_amount,
            'net_amount', b.reward_amount - b.cost_amount,
            'pool_version', b.pool_version,
            'open_source', b.open_source,
            'missing_play_reward_ledger', true
        ),
        TRUE,
        'medium'::varchar(24),
        b.created_at
    FROM play_blindbox_opens b
    WHERE b.reward_amount - b.cost_amount <> 0
      AND NOT EXISTS (
          SELECT 1
          FROM play_reward_ledger prl
          WHERE prl.user_id = b.user_id
            AND prl.idempotency_key = b.idempotency_key
      )

    UNION ALL

    SELECT
        pcu.user_id,
        pcu.bonus_amount::numeric(20,8),
        0::numeric(20,8),
        'promo_bonus'::varchar(64),
        pcu.id::text::varchar(128),
        ('promo_code_usage:' || pcu.id)::varchar(160),
        'user'::varchar(32),
        pcu.user_id,
        '优惠码奖励'::text,
        jsonb_build_object(
            'promo_code_usage_id', pcu.id,
            'promo_code_id', pcu.promo_code_id,
            'code', pc.code
        ),
        TRUE,
        'high'::varchar(24),
        pcu.used_at
    FROM promo_code_usages pcu
    JOIN promo_codes pc ON pc.id = pcu.promo_code_id
    WHERE pcu.bonus_amount <> 0

    UNION ALL

    SELECT
        ul.user_id,
        (-ul.actual_cost)::numeric(20,8),
        0::numeric(20,8),
        'usage_charge'::varchar(64),
        ul.id::text::varchar(128),
        ('usage_log:' || ul.id)::varchar(160),
        'system'::varchar(32),
        NULL::bigint,
        'API 消耗扣费'::text,
        jsonb_build_object(
            'usage_log_id', ul.id,
            'request_id', ul.request_id,
            'api_key_id', ul.api_key_id,
            'model', ul.model,
            'requested_model', ul.requested_model,
            'upstream_model', ul.upstream_model,
            'billing_mode', ul.billing_mode,
            'actual_cost', ul.actual_cost,
            'total_cost', ul.total_cost
        ),
        TRUE,
        'high'::varchar(24),
        ul.created_at
    FROM usage_logs ul
    WHERE ul.billing_type = 0
      AND ul.actual_cost > 0

    UNION ALL

    SELECT
        po.user_id,
        (-COALESCE(ra.balance_deducted, po.refund_amount, 0))::numeric(20,8),
        0::numeric(20,8),
        'refund'::varchar(64),
        ra.id::text::varchar(128),
        ('payment_refund:' || ra.id)::varchar(160),
        COALESCE(NULLIF(ra.operator, ''), 'admin')::varchar(32),
        NULL::bigint,
        '退款扣回'::text,
        jsonb_build_object(
            'audit_log_id', ra.id,
            'order_id', po.id,
            'detail_raw', ra.detail,
            'balance_deducted', ra.balance_deducted,
            'refund_amount', po.refund_amount
        ),
        TRUE,
        'high'::varchar(24),
        ra.created_at
    FROM refund_audits ra
    JOIN payment_orders po ON po.id::text = ra.order_id
    WHERE po.order_type = 'balance'
      AND COALESCE(ra.balance_deducted, po.refund_amount, 0) <> 0
),
ordered AS (
    SELECT *
    FROM candidates
    WHERE user_id IS NOT NULL
      AND balance_delta <> 0
      AND NOT EXISTS (
          SELECT 1
          FROM balance_transactions bt
          WHERE bt.user_id = candidates.user_id
            AND bt.idempotency_key = candidates.idempotency_key
      )
    ORDER BY created_at, source_type, source_id
    LIMIT :batch_size
),
inserted AS (
    INSERT INTO balance_transactions (
        user_id,
        balance_delta,
        balance_before,
        balance_after,
        frozen_delta,
        frozen_before,
        frozen_after,
        source_type,
        source_id,
        idempotency_key,
        actor_type,
        actor_user_id,
        description,
        metadata,
        is_backfilled,
        confidence,
        created_at
    )
    SELECT
        user_id,
        balance_delta,
        NULL,
        NULL,
        frozen_delta,
        NULL,
        NULL,
        source_type,
        source_id,
        idempotency_key,
        actor_type,
        actor_user_id,
        description,
        metadata,
        is_backfilled,
        confidence,
        created_at
    FROM ordered
    ON CONFLICT (user_id, idempotency_key) DO NOTHING
    RETURNING 1
)
SELECT
    (SELECT COUNT(*) FROM ordered) AS candidate_rows,
    (SELECT COUNT(*) FROM inserted) AS inserted_rows,
    :'dry_run' AS dry_run;

\if :dry_run
ROLLBACK;
\else
COMMIT;
\endif
