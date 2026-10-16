-- BPS settings and group/scheduling quarantine have independent ownership.
-- Keep one rule per scope, including paused rules, rather than one per account.
-- Ported from upstream v2.10.5 254_quality_bps_coexist.sql (renumbered to avoid
-- collision with fork-local 254_subscription_package_quotas.sql).
DROP INDEX IF EXISTS scheduled_test_quality_account_unique;
CREATE UNIQUE INDEX IF NOT EXISTS scheduled_test_quality_account_scope_unique
 ON scheduled_test_plans (account_id,
   (CASE WHEN pelican_config->'quality'->>'action' = 'enable_bps' THEN 'bps' ELSE 'quarantine' END))
 WHERE pelican_config->'quality' IS NOT NULL;
