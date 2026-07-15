-- 187_model_catalog_group_scope.sql
-- NULL keeps legacy automatic group matching; an array restricts the model
-- to the explicitly selected groups in the authenticated pricing view.

ALTER TABLE site_model_catalog
    ADD COLUMN IF NOT EXISTS group_ids BIGINT[];

CREATE INDEX IF NOT EXISTS idx_site_model_catalog_group_ids
    ON site_model_catalog USING GIN (group_ids);
