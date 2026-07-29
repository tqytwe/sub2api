-- Daily-card holds are admission/audit markers. They may reserve zero dollars
-- while settlement writes authoritative usage to subscription_entitlements.

ALTER TABLE subscription_entitlement_holds
    DROP CONSTRAINT IF EXISTS subscription_entitlement_holds_amount_check;

ALTER TABLE subscription_entitlement_holds
    ADD CONSTRAINT subscription_entitlement_holds_amount_check
        CHECK (
            reserved_usd >= 0
            AND captured_usd >= 0
            AND (
                reserved_usd = 0
                OR captured_usd <= reserved_usd
            )
        );
