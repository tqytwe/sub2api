-- Track manual ownership per official display-price field.
-- These flags affect only model-plaza presentation data, never billing.

ALTER TABLE site_model_catalog
    ADD COLUMN IF NOT EXISTS official_input_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS official_output_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS official_cache_read_manual BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS official_cache_write_manual BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing record-level manual rows become field-level manual rows. Empty
-- fields remain sync-owned so a later refresh can fill them.
UPDATE site_model_catalog
SET official_input_manual = (official_source = 'manual' AND official_input_price IS NOT NULL),
    official_output_manual = (official_source = 'manual' AND official_output_price IS NOT NULL),
    official_cache_read_manual = (official_source = 'manual' AND official_cache_read_price IS NOT NULL),
    official_cache_write_manual = (official_source = 'manual' AND official_cache_write_price IS NOT NULL)
WHERE official_source = 'manual';
