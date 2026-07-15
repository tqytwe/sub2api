-- 185_model_catalog_official_prices.sql
-- Keep external official/reference prices separate from site display overrides.

ALTER TABLE site_model_catalog
    ADD COLUMN IF NOT EXISTS official_input_price DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS official_output_price DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS official_cache_read_price DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS official_cache_write_price DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS official_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS official_updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS price_multiplier DECIMAL(12,6);

CREATE INDEX IF NOT EXISTS idx_site_model_catalog_auth_sort
    ON site_model_catalog (visible_auth, sort_order, model_name);
