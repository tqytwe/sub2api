-- Additive: legacy outflows remain readable with empty evidence.
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS external_reference VARCHAR(200),
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS operation_note VARCHAR(1000),
    ADD COLUMN IF NOT EXISTS actor_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_affiliate_ledger_user_action_created
    ON user_affiliate_ledger(user_id, action, created_at DESC, id DESC);
