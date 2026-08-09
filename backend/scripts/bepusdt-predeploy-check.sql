-- Run against the sub2api PostgreSQL database before enabling BEpusdt.
-- The script is read-only and returns zero rows only when every invariant passes.

BEGIN TRANSACTION READ ONLY;

WITH required(filename) AS (
    VALUES
        ('092_payment_orders.sql'),
        ('096_payment_provider_instances.sql'),
        ('101_add_payment_mode.sql'),
        ('103_add_allow_user_refund.sql'),
        ('112_add_payment_order_provider_key_snapshot.sql'),
        ('117_add_payment_order_provider_snapshot.sql'),
        ('120_enforce_payment_orders_out_trade_no_unique_notx.sql'),
        ('224_payment_order_coupon_settlement.sql'),
        ('252_bepusdt_payment_contract.sql')
),
missing AS (
    SELECT required.filename
    FROM required
    LEFT JOIN schema_migrations applied USING (filename)
    WHERE applied.filename IS NULL
)
SELECT
    'missing_required_migration' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    COALESCE(string_agg(filename, ', ' ORDER BY filename), '') AS detail
FROM missing
HAVING COUNT(*) > 0;

WITH expected(table_name, constraint_name) AS (
    VALUES
        ('payment_provider_instances', 'payment_provider_instances_bepusdt_contract_check'),
        ('payment_orders', 'payment_orders_bepusdt_contract_check')
), missing_or_unvalidated AS (
    SELECT expected.table_name || '.' || expected.constraint_name AS constraint_name
    FROM expected
    CROSS JOIN pg_namespace target_ns
    LEFT JOIN pg_class rel
      ON rel.relname = expected.table_name
     AND rel.relnamespace = target_ns.oid
    LEFT JOIN pg_constraint c
      ON c.conrelid = rel.oid
     AND c.conname = expected.constraint_name
    WHERE target_ns.nspname = current_schema()
      AND (c.oid IS NULL
       OR c.convalidated IS NOT TRUE
      )
)
SELECT
    'missing_or_unvalidated_bepusdt_constraint' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    COALESCE(string_agg(constraint_name, ', ' ORDER BY constraint_name), '') AS detail
FROM missing_or_unvalidated
HAVING COUNT(*) > 0;

WITH expected(table_name, column_name, data_type, is_nullable, character_maximum_length, numeric_precision, numeric_scale) AS (
    VALUES
        ('payment_provider_instances', 'provider_key', 'character varying', 'NO', 30, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'config', 'text', 'NO', NULL::integer, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'supported_types', 'character varying', 'NO', 200, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'enabled', 'boolean', 'NO', NULL::integer, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'payment_mode', 'character varying', 'NO', 20, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'refund_enabled', 'boolean', 'NO', NULL::integer, NULL::integer, NULL::integer),
        ('payment_provider_instances', 'allow_user_refund', 'boolean', 'NO', NULL::integer, NULL::integer, NULL::integer),
        ('payment_orders', 'out_trade_no', 'character varying', 'NO', 64, NULL::integer, NULL::integer),
        ('payment_orders', 'payment_type', 'character varying', 'NO', 30, NULL::integer, NULL::integer),
        ('payment_orders', 'payment_trade_no', 'character varying', 'NO', 128, NULL::integer, NULL::integer),
        ('payment_orders', 'provider_instance_id', 'character varying', 'YES', 64, NULL::integer, NULL::integer),
        ('payment_orders', 'provider_key', 'character varying', 'YES', 30, NULL::integer, NULL::integer),
        ('payment_orders', 'provider_snapshot', 'jsonb', 'YES', NULL::integer, NULL::integer, NULL::integer),
        ('payment_orders', 'payment_currency', 'character varying', 'NO', 3, NULL::integer, NULL::integer),
        ('payment_orders', 'pay_amount', 'numeric', 'NO', NULL::integer, 20, 2),
        ('payment_orders', 'list_amount', 'numeric', 'NO', NULL::integer, 20, 2),
        ('payment_orders', 'gateway_base_amount', 'numeric', 'NO', NULL::integer, 20, 2),
        ('payment_orders', 'discount_amount', 'numeric', 'NO', NULL::integer, 20, 2),
        ('payment_orders', 'fee_amount', 'numeric', 'NO', NULL::integer, 20, 2),
        ('payment_orders', 'expires_at', 'timestamp with time zone', 'NO', NULL::integer, NULL::integer, NULL::integer),
        ('payment_orders', 'paid_at', 'timestamp with time zone', 'YES', NULL::integer, NULL::integer, NULL::integer)
),
mismatched AS (
    SELECT expected.table_name || '.' || expected.column_name AS column_name
    FROM expected
    LEFT JOIN information_schema.columns actual
      ON actual.table_schema = current_schema()
     AND actual.table_name = expected.table_name
     AND actual.column_name = expected.column_name
    WHERE actual.column_name IS NULL
       OR actual.data_type <> expected.data_type
       OR actual.is_nullable <> expected.is_nullable
       OR (expected.character_maximum_length IS NOT NULL AND actual.character_maximum_length <> expected.character_maximum_length)
       OR (expected.numeric_precision IS NOT NULL AND actual.numeric_precision <> expected.numeric_precision)
       OR (expected.numeric_scale IS NOT NULL AND actual.numeric_scale <> expected.numeric_scale)
)
SELECT
    'schema_column_mismatch' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    COALESCE(string_agg(column_name, ', ' ORDER BY column_name), '') AS detail
FROM mismatched
HAVING COUNT(*) > 0;

SELECT
    'out_trade_no_unique_index_missing' AS issue_code,
    1::bigint AS affected_rows,
    'payment_orders.out_trade_no requires a partial unique index' AS detail
WHERE NOT EXISTS (
    SELECT 1
    FROM pg_indexes
    WHERE schemaname = current_schema()
      AND tablename = 'payment_orders'
      AND indexdef ILIKE 'CREATE UNIQUE INDEX%'
      AND indexdef ILIKE '%(out_trade_no)%'
      AND indexdef ILIKE '%WHERE%'
);

WITH invalid AS (
    SELECT id
    FROM payment_provider_instances
    WHERE provider_key = 'bepusdt'
      AND (
          supported_types <> 'bepusdt'
          OR payment_mode <> ''
          OR refund_enabled
          OR allow_user_refund
          OR NOT CASE
              WHEN config ~ '^\s*\{' THEN
                  jsonb_typeof(config::jsonb) = 'object'
                  AND COALESCE(config::jsonb ->> 'apiBase', '') <> ''
                  AND COALESCE(config::jsonb ->> 'apiToken', '') <> ''
                  AND COALESCE(config::jsonb ->> 'notifyUrl', '') <> ''
                  AND COALESCE(config::jsonb ->> 'returnUrl', '') <> ''
                  AND config::jsonb ->> 'currency' = 'CNY'
              ELSE false
          END
      )
)
SELECT
    'invalid_bepusdt_provider_instance' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    'IDs: ' || string_agg(id::text, ', ' ORDER BY id) AS detail
FROM invalid
HAVING COUNT(*) > 0;

WITH invalid AS (
    SELECT orders.id
    FROM payment_orders orders
    LEFT JOIN payment_provider_instances instances
      ON instances.id::text = orders.provider_instance_id
    WHERE (orders.provider_key = 'bepusdt' OR orders.payment_type = 'bepusdt')
      AND (
          orders.provider_key IS DISTINCT FROM 'bepusdt'
          OR
          orders.payment_type <> 'bepusdt'
          OR orders.payment_currency <> 'CNY'
          OR orders.pay_amount <= 0
          OR instances.id IS NULL
          OR instances.provider_key <> 'bepusdt'
          OR orders.provider_snapshot IS NULL
          OR jsonb_typeof(orders.provider_snapshot) <> 'object'
          OR orders.provider_snapshot ->> 'schema_version' <> '2'
          OR orders.provider_snapshot ->> 'provider_key' <> 'bepusdt'
          OR orders.provider_snapshot ->> 'provider_instance_id' IS DISTINCT FROM orders.provider_instance_id
          OR orders.provider_snapshot ->> 'currency' <> 'CNY'
          OR orders.provider_snapshot::text ~* '"(api[_]?token|notify[_]?url|return[_]?url)"[[:space:]]*:'
      )
)
SELECT
    'invalid_bepusdt_order_binding' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    'IDs: ' || string_agg(id::text, ', ' ORDER BY id) AS detail
FROM invalid
HAVING COUNT(*) > 0;

WITH invalid AS (
    SELECT id
    FROM payment_orders
    WHERE (provider_key = 'bepusdt' OR payment_type = 'bepusdt')
      AND (
          gateway_base_amount <> list_amount - discount_amount
          OR pay_amount <> gateway_base_amount + fee_amount
          OR (status IN ('PAID', 'RECHARGING', 'COMPLETED', 'PARTIALLY_REFUNDED', 'REFUNDED') AND paid_at IS NULL)
          OR (status IN ('PAID', 'RECHARGING', 'COMPLETED', 'PARTIALLY_REFUNDED', 'REFUNDED') AND payment_trade_no = '')
      )
)
SELECT
    'invalid_bepusdt_order_accounting' AS issue_code,
    COUNT(*)::bigint AS affected_rows,
    'IDs: ' || string_agg(id::text, ', ' ORDER BY id) AS detail
FROM invalid
HAVING COUNT(*) > 0;

COMMIT;
