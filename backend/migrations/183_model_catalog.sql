-- 183_model_catalog.sql
-- Site model catalog, pricing sync jobs, and discovery pool.

CREATE TABLE IF NOT EXISTS site_model_catalog (
    id                BIGSERIAL PRIMARY KEY,
    model_name        VARCHAR(128) NOT NULL,
    platform          VARCHAR(50)  NOT NULL DEFAULT '',
    display_name      VARCHAR(256),
    use_case          VARCHAR(32),
    sort_order        INT NOT NULL DEFAULT 0,
    visible_public    BOOLEAN NOT NULL DEFAULT FALSE,
    visible_auth      BOOLEAN NOT NULL DEFAULT TRUE,
    featured          BOOLEAN NOT NULL DEFAULT FALSE,
    input_price       DECIMAL(20,10),
    output_price      DECIMAL(20,10),
    cache_read_price  DECIMAL(20,10),
    cache_write_price DECIMAL(20,10),
    billing_mode      VARCHAR(20) DEFAULT 'token',
    source            VARCHAR(32) DEFAULT 'manual',
    source_updated_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (model_name, platform)
);

CREATE TABLE IF NOT EXISTS model_discoveries (
    id            BIGSERIAL PRIMARY KEY,
    model_name    VARCHAR(128) NOT NULL,
    platform      VARCHAR(50)  NOT NULL DEFAULT '',
    source        VARCHAR(32)  NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    status        VARCHAR(16) NOT NULL DEFAULT 'new',
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (model_name, platform, source)
);

CREATE TABLE IF NOT EXISTS model_sync_jobs (
    id            VARCHAR(64) PRIMARY KEY,
    kind          VARCHAR(32) NOT NULL DEFAULT 'pricing_refresh',
    status        VARCHAR(16) NOT NULL,
    result        JSONB,
    error         TEXT,
    started_at    TIMESTAMPTZ NOT NULL,
    completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_site_model_catalog_visible_public ON site_model_catalog (visible_public, sort_order);
CREATE INDEX IF NOT EXISTS idx_model_discoveries_status ON model_discoveries (status, discovered_at DESC);

-- Seed default marketing catalog (matches legacy PublicCatalogModels).
INSERT INTO site_model_catalog (model_name, platform, display_name, use_case, sort_order, visible_public, visible_auth, featured, source)
VALUES
    ('gpt-5.6-sol', 'openai', 'gpt-5.6-sol', 'reasoning', 10, TRUE, TRUE, TRUE, 'manual'),
    ('gpt-5.6-terra', 'openai', 'gpt-5.6-terra', 'code', 20, TRUE, TRUE, TRUE, 'manual'),
    ('gpt-5.5', 'openai', 'gpt-5.5', 'code', 30, TRUE, TRUE, FALSE, 'manual'),
    ('gpt-5-mini', 'openai', 'gpt-5-mini', 'chat', 40, TRUE, TRUE, FALSE, 'manual'),
    ('gpt-4.1', 'openai', 'gpt-4.1', 'code', 50, TRUE, TRUE, FALSE, 'manual'),
    ('o4-mini', 'openai', 'o4-mini', 'reasoning', 60, TRUE, TRUE, FALSE, 'manual'),
    ('claude-sonnet-4-6', 'anthropic', 'claude-sonnet-4-6', 'code', 70, TRUE, TRUE, TRUE, 'manual'),
    ('claude-opus-4-8', 'anthropic', 'claude-opus-4-8', 'reasoning', 80, TRUE, TRUE, TRUE, 'manual'),
    ('claude-haiku-4-5', 'anthropic', 'claude-haiku-4-5', 'chat', 90, TRUE, TRUE, FALSE, 'manual'),
    ('gemini-2.5-flash', 'gemini', 'gemini-2.5-flash', 'chat', 100, TRUE, TRUE, FALSE, 'manual'),
    ('gemini-3-flash', 'gemini', 'gemini-3-flash', 'reasoning', 110, TRUE, TRUE, FALSE, 'manual')
ON CONFLICT (model_name, platform) DO NOTHING;

-- Enable login pricing by default (pairs with public_models from migration 181).
INSERT INTO settings (key, value)
VALUES ('available_channels_enabled', 'true')
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
