-- Migration: 227_channel_monitor_account_soft_delete_unbind
-- Accounts use soft deletion, so the foreign key in migration 226 cannot run
-- its ON DELETE SET NULL action. Keep quota monitors, but remove their stale
-- account link as soon as an account becomes soft-deleted.

CREATE OR REPLACE FUNCTION clear_channel_monitor_account_on_soft_delete()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        UPDATE channel_monitors
        SET account_id = NULL
        WHERE account_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS accounts_channel_monitor_soft_delete_unbind ON accounts;
CREATE TRIGGER accounts_channel_monitor_soft_delete_unbind
AFTER UPDATE OF deleted_at ON accounts
FOR EACH ROW
EXECUTE FUNCTION clear_channel_monitor_account_on_soft_delete();

-- Repair any stale links that predate the trigger. This is idempotent and does
-- not delete monitor history or configuration.
UPDATE channel_monitors AS monitors
SET account_id = NULL
FROM accounts
WHERE accounts.deleted_at IS NOT NULL
  AND monitors.account_id = accounts.id;
