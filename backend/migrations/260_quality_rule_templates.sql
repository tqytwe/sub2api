-- Quality operations group rules. This filename intentionally matches upstream
-- and coexists with the fork's 260_model_catalog_media_capabilities.sql.
-- Enabled templates follow matching accounts on the runner's minute tick.
CREATE TABLE IF NOT EXISTS quality_rule_templates (
    id BIGSERIAL PRIMARY KEY,
    account_filter JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_id VARCHAR(100) NOT NULL DEFAULT '',
    cron_expression VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    max_results INT NOT NULL DEFAULT 100,
    pelican_config JSONB NOT NULL,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS quality_rule_template_accounts (
    template_id BIGINT NOT NULL REFERENCES quality_rule_templates(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    plan_id BIGINT REFERENCES scheduled_test_plans(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (template_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_quality_rule_template_accounts_plan
    ON quality_rule_template_accounts(plan_id);
