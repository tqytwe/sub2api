-- Speed up site-catalog billing lookups by normalized model name.

CREATE INDEX IF NOT EXISTS idx_site_model_catalog_model_name_lower
    ON site_model_catalog (LOWER(model_name));
