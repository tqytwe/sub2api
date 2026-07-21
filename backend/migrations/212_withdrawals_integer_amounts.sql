-- CP5 hotfix: withdrawal amounts are whole-unit amounts.
-- Existing rows are not validated to avoid blocking deploys; new writes and
-- updates are constrained by PostgreSQL.

ALTER TABLE withdrawal_system_settings
    ADD CONSTRAINT chk_withdrawal_system_integer_amounts
    CHECK (
        minimum_amount > 0
        AND minimum_amount = TRUNC(minimum_amount)
        AND daily_limit_amount > 0
        AND daily_limit_amount = TRUNC(daily_limit_amount)
        AND double_review_threshold > 0
        AND double_review_threshold = TRUNC(double_review_threshold)
    ) NOT VALID;

ALTER TABLE user_withdrawal_settings
    ADD CONSTRAINT chk_user_withdrawal_integer_overrides
    CHECK (
        (
            minimum_amount_override IS NULL
            OR (minimum_amount_override > 0 AND minimum_amount_override = TRUNC(minimum_amount_override))
        )
        AND (
            daily_limit_amount_override IS NULL
            OR (daily_limit_amount_override > 0 AND daily_limit_amount_override = TRUNC(daily_limit_amount_override))
        )
    ) NOT VALID;

ALTER TABLE withdrawal_requests
    ADD CONSTRAINT chk_withdrawal_request_integer_amount
    CHECK (amount > 0 AND amount = TRUNC(amount)) NOT VALID;

ALTER TABLE withdrawal_requests
    ADD CONSTRAINT chk_withdrawal_paid_integer_amount
    CHECK (paid_amount IS NULL OR (paid_amount > 0 AND paid_amount = TRUNC(paid_amount))) NOT VALID;
