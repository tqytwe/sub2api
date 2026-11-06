-- Existing makeup records remain unchanged; administrator adjustments append
-- actor and confirmed preview evidence under the existing immutable trigger.
ALTER TABLE play_checkin_reconciliations
    ADD COLUMN IF NOT EXISTS actor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS preview_digest VARCHAR(64);
ALTER TABLE play_checkin_reconciliations ADD CONSTRAINT play_checkin_repair_audit_complete
    CHECK ((actor_user_id IS NULL AND preview_digest IS NULL)
        OR (actor_user_id IS NOT NULL AND preview_digest ~ '^[a-f0-9]{64}$'));
