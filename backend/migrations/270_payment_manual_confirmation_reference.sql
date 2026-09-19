-- Manual payment confirmation uses the provider-scoped external transaction
-- reference as its only operator-entered financial evidence. Refuse rollout
-- when historical duplicates exist; never rewrite payment history here.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM payment_orders
        WHERE payment_trade_no <> ''
        GROUP BY
            CASE
                WHEN NULLIF(provider_instance_id, '') IS NOT NULL
                    THEN 'instance:' || provider_instance_id
                WHEN NULLIF(provider_key, '') IS NOT NULL
                    THEN 'provider:' || lower(provider_key)
                ELSE 'type:' || lower(payment_type)
            END,
            payment_trade_no
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate payment provider transaction references found; run the manual-payment-confirmation predeploy check';
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_orders_provider_trade_no_unique
    ON payment_orders (
        (
            CASE
                WHEN NULLIF(provider_instance_id, '') IS NOT NULL
                    THEN 'instance:' || provider_instance_id
                WHEN NULLIF(provider_key, '') IS NOT NULL
                    THEN 'provider:' || lower(provider_key)
                ELSE 'type:' || lower(payment_type)
            END
        ),
        payment_trade_no
    )
    WHERE payment_trade_no <> '';
