-- Keep the generic payment tables aligned with the dedicated BEpusdt adapter.
-- Invalid pre-existing rows stop the rollout instead of being silently changed.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM payment_provider_instances
        WHERE provider_key = 'bepusdt'
          AND (
              supported_types <> 'bepusdt'
              OR payment_mode <> ''
              OR refund_enabled <> false
              OR allow_user_refund <> false
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
    ) THEN
        RAISE EXCEPTION 'invalid BEpusdt provider instance contract; run the BEpusdt predeploy check';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM payment_orders
        WHERE (provider_key = 'bepusdt' OR payment_type = 'bepusdt')
          AND NOT (
              provider_key IS NOT DISTINCT FROM 'bepusdt'
              AND payment_type = 'bepusdt'
              AND provider_instance_id IS NOT NULL
              AND provider_instance_id ~ '^[1-9][0-9]*$'
              AND payment_currency = 'CNY'
              AND pay_amount > 0
              AND provider_snapshot IS NOT NULL
              AND jsonb_typeof(provider_snapshot) = 'object'
              AND provider_snapshot ->> 'schema_version' = '2'
              AND provider_snapshot ->> 'provider_key' = 'bepusdt'
              AND provider_snapshot ->> 'provider_instance_id' = provider_instance_id
              AND provider_snapshot ->> 'currency' = 'CNY'
              AND provider_snapshot::text !~* '"(api[_]?token|notify[_]?url|return[_]?url)"[[:space:]]*:'
          )
    ) THEN
        RAISE EXCEPTION 'invalid BEpusdt payment order contract; run the BEpusdt predeploy check';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'payment_provider_instances_bepusdt_contract_check'
          AND conrelid = 'payment_provider_instances'::regclass
    ) THEN
        ALTER TABLE payment_provider_instances
            ADD CONSTRAINT payment_provider_instances_bepusdt_contract_check
            CHECK (
                provider_key <> 'bepusdt'
                OR (
                    supported_types = 'bepusdt'
                    AND payment_mode = ''
                    AND refund_enabled = false
                    AND allow_user_refund = false
                    AND CASE
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
            ) NOT VALID;
    END IF;
END $$;

ALTER TABLE payment_provider_instances
    VALIDATE CONSTRAINT payment_provider_instances_bepusdt_contract_check;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'payment_orders_bepusdt_contract_check'
          AND conrelid = 'payment_orders'::regclass
    ) THEN
        ALTER TABLE payment_orders
            ADD CONSTRAINT payment_orders_bepusdt_contract_check
            CHECK (
                NOT (
                    provider_key IS NOT DISTINCT FROM 'bepusdt'
                    OR payment_type IS NOT DISTINCT FROM 'bepusdt'
                )
                OR (
                    provider_key IS NOT NULL
                    AND provider_key = 'bepusdt'
                    AND payment_type = 'bepusdt'
                    AND provider_instance_id IS NOT NULL
                    AND provider_instance_id ~ '^[1-9][0-9]*$'
                    AND payment_currency = 'CNY'
                    AND pay_amount > 0
                    AND provider_snapshot IS NOT NULL
                    AND jsonb_typeof(provider_snapshot) = 'object'
                    AND provider_snapshot ->> 'schema_version' = '2'
                    AND provider_snapshot ->> 'provider_key' = 'bepusdt'
                    AND provider_snapshot ->> 'provider_instance_id' = provider_instance_id
                    AND provider_snapshot ->> 'currency' = 'CNY'
                    AND provider_snapshot::text !~* '"(api[_]?token|notify[_]?url|return[_]?url)"[[:space:]]*:'
                )
            ) NOT VALID;
    END IF;
END $$;

ALTER TABLE payment_orders
    VALIDATE CONSTRAINT payment_orders_bepusdt_contract_check;
