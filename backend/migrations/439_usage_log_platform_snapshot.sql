-- Persist the concrete provider selected for each request. Historical rows remain
-- NULL because mutable accounts/groups cannot recover request-time identity.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS platform VARCHAR(50);
COMMENT ON COLUMN usage_logs.platform IS 'Concrete upstream platform captured at request time; NULL means unknown historical provider';
