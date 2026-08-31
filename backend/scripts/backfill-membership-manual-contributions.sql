-- Run only after an operator has reviewed the candidate rows from the
-- read-only query below. This is intentionally separate from migrations.
-- It is idempotent on (source_type, external_ref) and never classifies gifts.
--
-- PREVIEW:
-- SELECT bt.id, bt.user_id, bt.balance_delta, bt.source_id, bt.created_at
-- FROM balance_transactions bt
-- WHERE bt.source_type = 'offline_recharge'
--   AND NULLIF(BTRIM(bt.source_id), '') IS NOT NULL
-- ORDER BY bt.created_at, bt.id;

BEGIN;

INSERT INTO play_membership_manual_contributions (
    user_id, balance_transaction_id, source_type, external_ref,
    paid_amount, refund_amount, net_amount, currency,
    qualification_state, qualification_source, qualification_reason,
    reviewed_by, reviewed_at, paid_at, rule_version
)
SELECT bt.user_id,
       bt.id,
       bt.source_type,
       BTRIM(bt.source_id),
       bt.balance_delta,
       0,
       bt.balance_delta,
       'CNY',
       'pending_review',
       'offline_recharge_backfill',
       'Requires operator verification of external payment evidence',
       NULL,
       NULL,
       bt.created_at,
       'v1'
FROM balance_transactions bt
WHERE bt.source_type = 'offline_recharge'
  AND bt.balance_delta > 0
  AND NULLIF(BTRIM(bt.source_id), '') IS NOT NULL
ON CONFLICT (source_type, external_ref) DO NOTHING;

COMMIT;
